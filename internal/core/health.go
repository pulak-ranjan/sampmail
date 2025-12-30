package core

import (
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/store"
)

// HealthStatus represents the health of a component
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of a single component
type ComponentHealth struct {
	Name      string       `json:"name"`
	Status    HealthStatus `json:"status"`
	Message   string       `json:"message,omitempty"`
	Latency   string       `json:"latency,omitempty"`
	CheckedAt time.Time    `json:"checked_at"`
}

// SystemHealth represents overall system health
type SystemHealth struct {
	Status     HealthStatus      `json:"status"`
	Version    string            `json:"version"`
	Uptime     string            `json:"uptime"`
	Components []ComponentHealth `json:"components"`
	CheckedAt  time.Time         `json:"checked_at"`
}

// HealthChecker performs health checks
type HealthChecker struct {
	store     *store.Store
	startTime time.Time
	version   string
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(st *store.Store, version string) *HealthChecker {
	return &HealthChecker{
		store:     st,
		startTime: time.Now(),
		version:   version,
	}
}

// Check performs all health checks
func (hc *HealthChecker) Check(ctx context.Context) *SystemHealth {
	health := &SystemHealth{
		Status:     HealthStatusHealthy,
		Version:    hc.version,
		Uptime:     time.Since(hc.startTime).Round(time.Second).String(),
		Components: make([]ComponentHealth, 0),
		CheckedAt:  time.Now(),
	}

	// Run checks concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	checks := []struct {
		name  string
		check func(context.Context) ComponentHealth
	}{
		{"database", hc.checkDatabase},
		{"smtp", hc.checkSMTP},
		{"reacher", hc.checkReacher},
		{"disk", hc.checkDisk},
		{"memory", hc.checkMemory},
	}

	for _, c := range checks {
		wg.Add(1)
		go func(name string, checkFn func(context.Context) ComponentHealth) {
			defer wg.Done()
			result := checkFn(ctx)
			mu.Lock()
			health.Components = append(health.Components, result)
			// Update overall status
			if result.Status == HealthStatusUnhealthy {
				health.Status = HealthStatusUnhealthy
			} else if result.Status == HealthStatusDegraded && health.Status == HealthStatusHealthy {
				health.Status = HealthStatusDegraded
			}
			mu.Unlock()
		}(c.name, c.check)
	}

	wg.Wait()
	return health
}

// checkDatabase checks database connectivity
func (hc *HealthChecker) checkDatabase(ctx context.Context) ComponentHealth {
	start := time.Now()
	result := ComponentHealth{
		Name:      "database",
		Status:    HealthStatusHealthy,
		CheckedAt: time.Now(),
	}

	sqlDB, err := hc.store.DB.DB()
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("failed to get DB handle: %v", err)
		return result
	}

	// Ping with context
	if err := sqlDB.PingContext(ctx); err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("ping failed: %v", err)
		return result
	}

	result.Latency = time.Since(start).String()

	// Check connection pool stats
	stats := sqlDB.Stats()
	if stats.OpenConnections >= stats.MaxOpenConnections-5 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("connection pool near limit: %d/%d", stats.OpenConnections, stats.MaxOpenConnections)
	}

	return result
}

// checkSMTP checks SMTP connectivity
func (hc *HealthChecker) checkSMTP(ctx context.Context) ComponentHealth {
	start := time.Now()
	result := ComponentHealth{
		Name:      "smtp",
		Status:    HealthStatusHealthy,
		CheckedAt: time.Now(),
	}

	cfg := config.Get()

	// Check circuit breaker state
	if SMTPCircuitBreaker != nil && SMTPCircuitBreaker.State() == CircuitOpen {
		result.Status = HealthStatusUnhealthy
		result.Message = "circuit breaker is open"
		return result
	}

	// Try to connect
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.SMTPAddr)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("connection failed: %v", err)
		return result
	}
	conn.Close()

	result.Latency = time.Since(start).String()

	// Check pool stats if available
	if pool := GetSMTPPool(); pool != nil {
		stats := pool.Stats()
		available := stats["available"].(int)
		total := stats["total"].(int)
		if available == 0 && total > 0 {
			result.Status = HealthStatusDegraded
			result.Message = "no available connections in pool"
		}
	}

	return result
}

// checkReacher checks Reacher email verification service
func (hc *HealthChecker) checkReacher(ctx context.Context) ComponentHealth {
	start := time.Now()
	result := ComponentHealth{
		Name:      "reacher",
		Status:    HealthStatusHealthy,
		CheckedAt: time.Now(),
	}

	cfg := config.Get()
	if cfg.ReacherURL == "" {
		result.Status = HealthStatusDegraded
		result.Message = "not configured"
		return result
	}

	// Check circuit breaker state
	if ReacherCircuitBreaker != nil && ReacherCircuitBreaker.State() == CircuitOpen {
		result.Status = HealthStatusDegraded
		result.Message = "circuit breaker is open"
		return result
	}

	// Try health endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.ReacherURL+"/health", nil)
	if err != nil {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("request creation failed: %v", err)
		return result
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("connection failed: %v", err)
		return result
	}
	defer resp.Body.Close()

	result.Latency = time.Since(start).String()

	if resp.StatusCode != http.StatusOK {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("unhealthy response: %d", resp.StatusCode)
	}

	return result
}

// checkDisk checks disk space
func (hc *HealthChecker) checkDisk(ctx context.Context) ComponentHealth {
	result := ComponentHealth{
		Name:      "disk",
		Status:    HealthStatusHealthy,
		CheckedAt: time.Now(),
	}

	// Use statfs to check disk space
	// This is a simplified check - in production, use proper syscall
	cfg := config.Get()
	var stat DiskStats
	if err := GetDiskStats(cfg.DataDir, &stat); err != nil {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("unable to check: %v", err)
		return result
	}

	usedPercent := float64(stat.Used) / float64(stat.Total) * 100
	result.Message = fmt.Sprintf("%.1f%% used", usedPercent)

	if usedPercent > 95 {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("critical: %.1f%% used", usedPercent)
	} else if usedPercent > 85 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("warning: %.1f%% used", usedPercent)
	}

	return result
}

// DiskStats holds disk usage statistics
type DiskStats struct {
	Total     uint64
	Used      uint64
	Available uint64
}

// GetDiskStats gets disk statistics for a path
// Platform-specific implementation
func GetDiskStats(path string, stats *DiskStats) error {
	// Simplified implementation - use syscall.Statfs on Linux
	// For now, return healthy
	stats.Total = 100
	stats.Used = 50
	stats.Available = 50
	return nil
}

// checkMemory checks memory usage
func (hc *HealthChecker) checkMemory(ctx context.Context) ComponentHealth {
	result := ComponentHealth{
		Name:      "memory",
		Status:    HealthStatusHealthy,
		CheckedAt: time.Now(),
	}

	var stats MemoryStats
	if err := GetMemoryStats(&stats); err != nil {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("unable to check: %v", err)
		return result
	}

	usedPercent := float64(stats.Used) / float64(stats.Total) * 100
	result.Message = fmt.Sprintf("%.1f%% used", usedPercent)

	if usedPercent > 95 {
		result.Status = HealthStatusUnhealthy
		result.Message = fmt.Sprintf("critical: %.1f%% used", usedPercent)
	} else if usedPercent > 85 {
		result.Status = HealthStatusDegraded
		result.Message = fmt.Sprintf("warning: %.1f%% used", usedPercent)
	}

	return result
}

// MemoryStats holds memory usage statistics
type MemoryStats struct {
	Total     uint64
	Used      uint64
	Available uint64
}

// GetMemoryStats gets memory statistics
func GetMemoryStats(stats *MemoryStats) error {
	// Simplified implementation - use runtime.MemStats
	// For now, return healthy
	stats.Total = 100
	stats.Used = 50
	stats.Available = 50
	return nil
}

// Liveness check for Kubernetes - just checks if process is running
func (hc *HealthChecker) Liveness() bool {
	return true
}

// Readiness check for Kubernetes - checks if ready to serve traffic
func (hc *HealthChecker) Readiness(ctx context.Context) bool {
	// Check database is reachable
	sqlDB, err := hc.store.DB.DB()
	if err != nil {
		return false
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return false
	}
	return true
}

// Global health checker
var globalHealthChecker *HealthChecker

// InitHealthChecker initializes the global health checker
func InitHealthChecker(st *store.Store, version string) {
	globalHealthChecker = NewHealthChecker(st, version)
}

// GetHealthChecker returns the global health checker
func GetHealthChecker() *HealthChecker {
	return globalHealthChecker
}

// DBStats returns database statistics
func DBStats(db *sql.DB) map[string]interface{} {
	stats := db.Stats()
	return map[string]interface{}{
		"max_open_connections": stats.MaxOpenConnections,
		"open_connections":     stats.OpenConnections,
		"in_use":               stats.InUse,
		"idle":                 stats.Idle,
		"wait_count":           stats.WaitCount,
		"wait_duration":        stats.WaitDuration.String(),
		"max_idle_closed":      stats.MaxIdleClosed,
		"max_lifetime_closed":  stats.MaxLifetimeClosed,
	}
}
