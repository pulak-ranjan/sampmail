package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/pulak-ranjan/sampmail/internal/models"
)

// KumoClient provides safe access to KumoMTA HTTP API
// This replaces dangerous filesystem access to /var/spool/kumomta
type KumoClient struct {
	baseURL    string
	httpClient *http.Client
}

// Singleton instance
var kumoClient *KumoClient

// GetKumoClient returns the singleton KumoMTA client
func GetKumoClient() *KumoClient {
	if kumoClient == nil {
		cfg := config.Get()
		kumoClient = NewKumoClient(cfg.KumoAPIURL)
	}
	return kumoClient
}

// NewKumoClient creates a new KumoMTA API client
func NewKumoClient(baseURL string) *KumoClient {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8000"
	}
	return &KumoClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// KumoMetricsResponse represents the JSON metrics from KumoMTA
type KumoMetricsResponse struct {
	// Prometheus-style metrics as key-value pairs
	Metrics map[string]interface{} `json:"metrics"`
}

// GetQueueStats fetches queue statistics from KumoMTA HTTP API
// Uses /metrics.json endpoint instead of reading filesystem
func (k *KumoClient) GetQueueStats() (*QueueStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// First try /metrics.json for structured data
	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics.json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		logger.Warn("KumoMTA API unavailable, returning empty stats", "error", err)
		// Return empty stats instead of error (graceful degradation)
		return &QueueStats{}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Fall back to parsing Prometheus text format
		return k.parsePrometheusMetrics()
	}

	var metricsResp map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&metricsResp); err != nil {
		logger.Warn("Failed to parse KumoMTA metrics JSON", "error", err)
		return &QueueStats{}, nil
	}

	return k.extractQueueStats(metricsResp), nil
}

// parsePrometheusMetrics fetches and parses /metrics in Prometheus format
func (k *KumoClient) parsePrometheusMetrics() (*QueueStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics", nil)
	if err != nil {
		return nil, err
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return &QueueStats{}, nil
	}
	defer resp.Body.Close()

	// Parse Prometheus text format
	// Example: scheduled_count{queue="default"} 42
	stats := &QueueStats{}

	var body strings.Builder
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			body.Write(buf[:n])
		}
		if err != nil {
			break
		}
	}

	// Parse key metrics using regex
	scheduledRe := regexp.MustCompile(`scheduled_count\{[^}]*\}\s+(\d+)`)
	if matches := scheduledRe.FindAllStringSubmatch(body.String(), -1); len(matches) > 0 {
		for _, m := range matches {
			if val, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				stats.Total += int(val)
				stats.Queued += int(val)
			}
		}
	}

	// Parse deferred count
	deferredRe := regexp.MustCompile(`ready_count\{[^}]*\}\s+(\d+)`)
	if matches := deferredRe.FindAllStringSubmatch(body.String(), -1); len(matches) > 0 {
		for _, m := range matches {
			if val, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				stats.Deferred += int(val)
			}
		}
	}

	return stats, nil
}

// extractQueueStats extracts queue statistics from JSON metrics
func (k *KumoClient) extractQueueStats(metrics map[string]interface{}) *QueueStats {
	stats := &QueueStats{}

	// Navigate the metrics structure
	// KumoMTA returns metrics in various formats depending on version
	for key, value := range metrics {
		switch {
		case strings.Contains(key, "scheduled_count"):
			if v, ok := toInt64(value); ok {
				stats.Total += int(v)
				stats.Queued += int(v)
			}
		case strings.Contains(key, "ready_count"):
			if v, ok := toInt64(value); ok {
				stats.Deferred += int(v)
			}
		case strings.Contains(key, "total_messages_received"):
			// Could be used for additional stats
		}
	}

	return stats
}

// GetQueueMessages fetches queue message list via KumoMTA API
// Note: KumoMTA doesn't expose individual message details via HTTP by default
// We return what we can from metrics, or empty list
func (k *KumoClient) GetQueueMessages(limit int) ([]models.QueueMessage, error) {
	// KumoMTA's HTTP API doesn't expose individual messages by default
	// This would require custom Lua endpoints in KumoMTA
	// For now, return empty list with a note
	logger.Debug("GetQueueMessages: KumoMTA HTTP API doesn't expose individual messages")
	return []models.QueueMessage{}, nil
}

// FlushQueue triggers queue processing via KumoMTA HTTP API
func (k *KumoClient) FlushQueue() error {
	// KumoMTA uses HTTP API for injection but doesn't have a standard "flush" endpoint
	// The queue processes automatically based on retry intervals
	logger.Info("FlushQueue called - KumoMTA processes queue automatically")
	return nil
}

// DeleteMessage removes a message from the queue
// Note: This requires custom Lua endpoint in KumoMTA
func (k *KumoClient) DeleteMessage(id string) error {
	// Would need to call a custom Lua endpoint exposed via kumo.http
	logger.Warn("DeleteMessage: Not supported via standard KumoMTA API", "id", id)
	return fmt.Errorf("delete not supported via KumoMTA HTTP API")
}

// HealthCheck verifies KumoMTA is responding
func (k *KumoClient) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", k.baseURL+"/metrics", nil)
	if err != nil {
		return err
	}

	resp, err := k.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("KumoMTA unreachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("KumoMTA returned status %d", resp.StatusCode)
	}

	return nil
}

// Helper function to convert interface{} to int64
func toInt64(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case float64:
		return int64(t), true
	case int64:
		return t, true
	case int:
		return int64(t), true
	case string:
		if i, err := strconv.ParseInt(t, 10, 64); err == nil {
			return i, true
		}
	}
	return 0, false
}
