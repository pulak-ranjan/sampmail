package core

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/logger"
	"github.com/redis/go-redis/v9"
)

// =====================================
// PER-DOMAIN RATE LIMITING
// =====================================
// Implements token bucket rate limiting per destination domain
// to comply with provider-specific limits (Gmail, Outlook, Yahoo, etc.)

// DomainLimit defines rate limits for a specific domain
type DomainLimit struct {
	Domain         string    `json:"domain"`
	MaxPerMinute   int       `json:"max_per_minute"`
	MaxPerHour     int       `json:"max_per_hour"`
	MaxPerDay      int       `json:"max_per_day"`
	WarmupFactor   float64   `json:"warmup_factor"` // 0.0 - 1.0 multiplier
	LastUpdated    time.Time `json:"last_updated"`
}

// ProviderLimits defines default limits for major email providers
var ProviderLimits = map[string]DomainLimit{
	// Gmail limits (very strict for new senders)
	"gmail.com": {
		Domain:       "gmail.com",
		MaxPerMinute: 20,
		MaxPerHour:   500,
		MaxPerDay:    5000,
	},
	"googlemail.com": {
		Domain:       "googlemail.com",
		MaxPerMinute: 20,
		MaxPerHour:   500,
		MaxPerDay:    5000,
	},
	
	// Microsoft/Outlook limits
	"outlook.com": {
		Domain:       "outlook.com",
		MaxPerMinute: 30,
		MaxPerHour:   100,
		MaxPerDay:    2000,
	},
	"hotmail.com": {
		Domain:       "hotmail.com",
		MaxPerMinute: 30,
		MaxPerHour:   100,
		MaxPerDay:    2000,
	},
	"live.com": {
		Domain:       "live.com",
		MaxPerMinute: 30,
		MaxPerHour:   100,
		MaxPerDay:    2000,
	},
	"msn.com": {
		Domain:       "msn.com",
		MaxPerMinute: 30,
		MaxPerHour:   100,
		MaxPerDay:    2000,
	},
	
	// Yahoo limits
	"yahoo.com": {
		Domain:       "yahoo.com",
		MaxPerMinute: 25,
		MaxPerHour:   200,
		MaxPerDay:    5000,
	},
	"yahoo.co.in": {
		Domain:       "yahoo.co.in",
		MaxPerMinute: 25,
		MaxPerHour:   200,
		MaxPerDay:    5000,
	},
	"yahoo.co.uk": {
		Domain:       "yahoo.co.uk",
		MaxPerMinute: 25,
		MaxPerHour:   200,
		MaxPerDay:    5000,
	},
	"aol.com": {
		Domain:       "aol.com",
		MaxPerMinute: 25,
		MaxPerHour:   200,
		MaxPerDay:    5000,
	},
	
	// Apple
	"icloud.com": {
		Domain:       "icloud.com",
		MaxPerMinute: 20,
		MaxPerHour:   200,
		MaxPerDay:    2000,
	},
	"me.com": {
		Domain:       "me.com",
		MaxPerMinute: 20,
		MaxPerHour:   200,
		MaxPerDay:    2000,
	},
	
	// Default for unknown domains
	"_default": {
		Domain:       "_default",
		MaxPerMinute: 100,
		MaxPerHour:   1000,
		MaxPerDay:    10000,
	},
}

// DomainLimiter manages rate limiting per domain
type DomainLimiter struct {
	redis   *redis.Client
	limits  map[string]*DomainLimit
	mu      sync.RWMutex
}

// NewDomainLimiter creates a new domain rate limiter
func NewDomainLimiter(rdb *redis.Client) *DomainLimiter {
	dl := &DomainLimiter{
		redis:  rdb,
		limits: make(map[string]*DomainLimit),
	}
	
	// Initialize with provider limits
	for domain, limit := range ProviderLimits {
		limitCopy := limit
		dl.limits[domain] = &limitCopy
	}
	
	return dl
}

// Allow checks if a send to the given domain is allowed
func (dl *DomainLimiter) Allow(ctx context.Context, domain string) (bool, time.Duration, error) {
	if dl == nil {
		return true, 0, nil // No limiter, allow all
	}
	
	// Normalize domain
	domain = strings.ToLower(strings.TrimSpace(domain))
	
	// Get limit for domain
	limit := dl.getLimit(domain)
	
	// Apply warmup factor
	effectiveLimit := dl.applyWarmupFactor(limit)
	
	// Check Redis counters
	return dl.checkLimit(ctx, domain, effectiveLimit)
}

// getLimit returns the limit for a domain
func (dl *DomainLimiter) getLimit(domain string) *DomainLimit {
	dl.mu.RLock()
	defer dl.mu.RUnlock()
	
	// Check if we have a specific limit
	if limit, ok := dl.limits[domain]; ok {
		return limit
	}
	
	// Check for parent domain (e.g., mail.google.com -> google.com)
	parts := strings.Split(domain, ".")
	if len(parts) > 2 {
		parentDomain := strings.Join(parts[len(parts)-2:], ".")
		if limit, ok := dl.limits[parentDomain]; ok {
			return limit
		}
	}
	
	// Return default limit
	if limit, ok := dl.limits["_default"]; ok {
		return limit
	}
	
	// Fallback
	return &DomainLimit{
		Domain:       domain,
		MaxPerMinute: 100,
		MaxPerHour:   1000,
		MaxPerDay:    10000,
	}
}

// applyWarmupFactor adjusts limits based on warmup progress
func (dl *DomainLimiter) applyWarmupFactor(limit *DomainLimit) *DomainLimit {
	if limit.WarmupFactor <= 0 {
		limit.WarmupFactor = 1.0
	}
	
	adjusted := *limit
	adjusted.MaxPerMinute = int(float64(adjusted.MaxPerMinute) * limit.WarmupFactor)
	adjusted.MaxPerHour = int(float64(adjusted.MaxPerHour) * limit.WarmupFactor)
	adjusted.MaxPerDay = int(float64(adjusted.MaxPerDay) * limit.WarmupFactor)
	
	// Ensure minimum limits
	if adjusted.MaxPerMinute < 1 {
		adjusted.MaxPerMinute = 1
	}
	if adjusted.MaxPerHour < 10 {
		adjusted.MaxPerHour = 10
	}
	if adjusted.MaxPerDay < 100 {
		adjusted.MaxPerDay = 100
	}
	
	return &adjusted
}

// checkLimit checks if the current count is within limits
func (dl *DomainLimiter) checkLimit(ctx context.Context, domain string, limit *DomainLimit) (bool, time.Duration, error) {
	if dl.redis == nil {
		return true, 0, nil // No Redis, allow all
	}
	
	now := time.Now()
	
	// Check minute limit
	minuteKey := fmt.Sprintf("ratelimit:%s:minute:%d", domain, now.Unix()/60)
	minuteCount, err := dl.redis.Incr(ctx, minuteKey).Result()
	if err != nil {
		return true, 0, err // Redis error, allow
	}
	dl.redis.Expire(ctx, minuteKey, 2*time.Minute)
	
	if int(minuteCount) > limit.MaxPerMinute {
		waitTime := time.Until(time.Unix((now.Unix()/60+1)*60, 0))
		return false, waitTime, nil
	}
	
	// Check hour limit
	hourKey := fmt.Sprintf("ratelimit:%s:hour:%d", domain, now.Unix()/3600)
	hourCount, err := dl.redis.Incr(ctx, hourKey).Result()
	if err != nil {
		return true, 0, err
	}
	dl.redis.Expire(ctx, hourKey, 2*time.Hour)
	
	if int(hourCount) > limit.MaxPerHour {
		waitTime := time.Until(time.Unix((now.Unix()/3600+1)*3600, 0))
		return false, waitTime, nil
	}
	
	// Check day limit
	dayKey := fmt.Sprintf("ratelimit:%s:day:%s", domain, now.Format("2006-01-02"))
	dayCount, err := dl.redis.Incr(ctx, dayKey).Result()
	if err != nil {
		return true, 0, err
	}
	dl.redis.Expire(ctx, dayKey, 48*time.Hour)
	
	if int(dayCount) > limit.MaxPerDay {
		// Wait until midnight
		tomorrow := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, now.Location())
		waitTime := time.Until(tomorrow)
		return false, waitTime, nil
	}
	
	return true, 0, nil
}

// SetWarmupFactor sets the warmup factor for a domain
func (dl *DomainLimiter) SetWarmupFactor(domain string, factor float64) {
	if dl == nil {
		return
	}
	
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	// Clamp factor between 0.01 and 1.0
	if factor < 0.01 {
		factor = 0.01
	}
	if factor > 1.0 {
		factor = 1.0
	}
	
	// Get or create limit
	limit := dl.getLimit(domain)
	limit.WarmupFactor = factor
	limit.LastUpdated = time.Now()
	
	// Store in map
	dl.limits[domain] = limit
	
	logger.WithComponent("domain_limiter").Info("warmup factor set",
		"domain", domain,
		"factor", factor,
		"effective_minute_limit", int(float64(limit.MaxPerMinute)*factor))
}

// SetCustomLimit sets a custom limit for a domain
func (dl *DomainLimiter) SetCustomLimit(domain string, limit DomainLimit) {
	if dl == nil {
		return
	}
	
	dl.mu.Lock()
	defer dl.mu.Unlock()
	
	limit.Domain = domain
	limit.LastUpdated = time.Now()
	dl.limits[strings.ToLower(domain)] = &limit
	
	logger.WithComponent("domain_limiter").Info("custom limit set",
		"domain", domain,
		"max_per_minute", limit.MaxPerMinute,
		"max_per_hour", limit.MaxPerHour,
		"max_per_day", limit.MaxPerDay)
}

// GetStats returns current rate limit stats for a domain
func (dl *DomainLimiter) GetStats(ctx context.Context, domain string) (*DomainRateStats, error) {
	if dl == nil || dl.redis == nil {
		return nil, nil
	}
	
	now := time.Now()
	stats := &DomainRateStats{
		Domain: domain,
	}
	
	// Get current counts
	minuteKey := fmt.Sprintf("ratelimit:%s:minute:%d", domain, now.Unix()/60)
	if count, err := dl.redis.Get(ctx, minuteKey).Result(); err == nil {
		stats.CurrentMinute, _ = strconv.Atoi(count)
	}
	
	hourKey := fmt.Sprintf("ratelimit:%s:hour:%d", domain, now.Unix()/3600)
	if count, err := dl.redis.Get(ctx, hourKey).Result(); err == nil {
		stats.CurrentHour, _ = strconv.Atoi(count)
	}
	
	dayKey := fmt.Sprintf("ratelimit:%s:day:%s", domain, now.Format("2006-01-02"))
	if count, err := dl.redis.Get(ctx, dayKey).Result(); err == nil {
		stats.CurrentDay, _ = strconv.Atoi(count)
	}
	
	// Get limits
	limit := dl.getLimit(domain)
	stats.Limit = limit
	
	return stats, nil
}

// DomainRateStats holds current rate limit statistics
type DomainRateStats struct {
	Domain        string       `json:"domain"`
	CurrentMinute int          `json:"current_minute"`
	CurrentHour   int          `json:"current_hour"`
	CurrentDay    int          `json:"current_day"`
	Limit         *DomainLimit `json:"limit"`
}

// WaitForLimit waits until a send to the domain is allowed
func (dl *DomainLimiter) WaitForLimit(ctx context.Context, domain string) error {
	for {
		allowed, waitTime, err := dl.Allow(ctx, domain)
		if err != nil {
			return err
		}
		
		if allowed {
			return nil
		}
		
		// Wait for the specified time or until context is done
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Try again
		}
	}
}

// =====================================
// SENDER-DOMAIN RATE LIMITING
// =====================================

// SenderDomainLimiter limits based on sender+destination domain combination
// This is important for IP/domain reputation management
type SenderDomainLimiter struct {
	redis *redis.Client
	dl    *DomainLimiter
	mu    sync.RWMutex
}

// NewSenderDomainLimiter creates a new sender-domain limiter
func NewSenderDomainLimiter(rdb *redis.Client) *SenderDomainLimiter {
	return &SenderDomainLimiter{
		redis: rdb,
		dl:    NewDomainLimiter(rdb),
	}
}

// Allow checks if a sender can send to a destination domain
func (sdl *SenderDomainLimiter) Allow(ctx context.Context, senderDomain, destDomain string) (bool, time.Duration, error) {
	// Check destination domain limit
	return sdl.dl.Allow(ctx, destDomain)
}

// RecordSend records a send for rate limiting purposes
func (sdl *SenderDomainLimiter) RecordSend(ctx context.Context, senderDomain, destDomain string) error {
	if sdl == nil || sdl.redis == nil {
		return nil
	}
	
	now := time.Now()
	
	// Increment counters for sender+dest combination
	minuteKey := fmt.Sprintf("ratelimit:%s->%s:minute:%d", senderDomain, destDomain, now.Unix()/60)
	sdl.redis.Incr(ctx, minuteKey).Result()
	sdl.redis.Expire(ctx, minuteKey, 2*time.Minute)
	
	hourKey := fmt.Sprintf("ratelimit:%s->%s:hour:%d", senderDomain, destDomain, now.Unix()/3600)
	sdl.redis.Incr(ctx, hourKey).Result()
	sdl.redis.Expire(ctx, hourKey, 2*time.Hour)
	
	dayKey := fmt.Sprintf("ratelimit:%s->%s:day:%s", senderDomain, destDomain, now.Format("2006-01-02"))
	sdl.redis.Incr(ctx, dayKey).Result()
	sdl.redis.Expire(ctx, dayKey, 48*time.Hour)
	
	return nil
}

// =====================================
// GLOBAL DOMAIN LIMITER
// =====================================

var globalDomainLimiter *DomainLimiter

// InitDomainLimiter initializes the global domain limiter
func InitDomainLimiter(rdb *redis.Client) {
	globalDomainLimiter = NewDomainLimiter(rdb)
}

// GetDomainLimiter returns the global domain limiter
func GetDomainLimiter() *DomainLimiter {
	if globalDomainLimiter == nil {
		globalDomainLimiter = NewDomainLimiter(nil)
	}
	return globalDomainLimiter
}

// CheckDomainRateLimit checks if sending to a domain is allowed
func CheckDomainRateLimit(ctx context.Context, domain string) (bool, time.Duration, error) {
	return GetDomainLimiter().Allow(ctx, domain)
}

// =====================================
// HELPER FUNCTIONS
// =====================================

// ExtractDomain extracts the domain from an email address
func ExtractDomain(email string) string {
	email = strings.ToLower(strings.TrimSpace(email))
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}

// IsMajorProvider checks if a domain is a major email provider
func IsMajorProvider(domain string) bool {
	domain = strings.ToLower(domain)
	_, isMajor := ProviderLimits[domain]
	return isMajor
}

// GetProviderLimit returns the limit for a provider
func GetProviderLimit(domain string) DomainLimit {
	if limit, ok := ProviderLimits[strings.ToLower(domain)]; ok {
		return limit
	}
	return ProviderLimits["_default"]
}
