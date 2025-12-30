package custom

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/time/rate"

	"github.com/pulak-ranjan/sampmail/internal/config"
)

// RedisRateLimiter provides distributed rate limiting using Redis
type RedisRateLimiter struct {
	client     *redis.Client
	keyPrefix  string
	rate       int           // requests per window
	window     time.Duration // sliding window size
	enabled    bool
}

// RedisConfig holds Redis connection settings
type RedisConfig struct {
	Addr         string
	Password     string
	DB           int
	PoolSize     int
	MinIdleConns int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// DefaultRedisConfig returns production-ready defaults
func DefaultRedisConfig() *RedisConfig {
	return &RedisConfig{
		Addr:         "localhost:6379",
		DB:           0,
		PoolSize:     100,
		MinIdleConns: 10,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	}
}

// NewRedisRateLimiter creates a new Redis-backed rate limiter
func NewRedisRateLimiter(cfg *RedisConfig, keyPrefix string, rate int, window time.Duration) (*RedisRateLimiter, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return &RedisRateLimiter{
			keyPrefix: keyPrefix,
			rate:      rate,
			window:    window,
			enabled:   false, // Fallback to in-memory if Redis unavailable
		}, nil
	}

	return &RedisRateLimiter{
		client:    client,
		keyPrefix: keyPrefix,
		rate:      rate,
		window:    window,
		enabled:   true,
	}, nil
}

// slidingWindowScript is a Lua script for atomic sliding window rate limiting
const slidingWindowScript = `
local key = KEYS[1]
local now = tonumber(ARGV[1])
local window = tonumber(ARGV[2])
local limit = tonumber(ARGV[3])

-- Remove old entries outside the window
redis.call('ZREMRANGEBYSCORE', key, 0, now - window)

-- Count current requests in window
local count = redis.call('ZCARD', key)

if count < limit then
    -- Add new request
    redis.call('ZADD', key, now, now .. '-' .. math.random())
    redis.call('EXPIRE', key, window / 1000)
    return 1
else
    return 0
end
`

// Allow checks if a request should be allowed
func (rl *RedisRateLimiter) Allow(ctx context.Context, identifier string) (bool, error) {
	if !rl.enabled || rl.client == nil {
		return true, nil // Fail open if Redis unavailable
	}

	key := fmt.Sprintf("%s:%s", rl.keyPrefix, identifier)
	now := time.Now().UnixMilli()

	result, err := rl.client.Eval(ctx, slidingWindowScript, []string{key}, now, rl.window.Milliseconds(), rl.rate).Int()
	if err != nil {
		// Fail open on Redis errors
		return true, err
	}

	return result == 1, nil
}

// Limit returns an HTTP middleware for rate limiting
func (rl *RedisRateLimiter) Limit(next http.Handler) http.Handler {
	// If Redis is not available, fall back to in-memory limiter
	if !rl.enabled {
		return NewRateLimiter(
			rate.Limit(RateFromDuration(rl.window/time.Duration(rl.rate))),
			rl.rate,
		).Limit(next)
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetRealIP(r)

		ctx, cancel := context.WithTimeout(r.Context(), 100*time.Millisecond)
		defer cancel()

		allowed, err := rl.Allow(ctx, ip)
		if err != nil {
			// Log error but allow request (fail open)
			allowed = true
		}

		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(rl.window.Seconds())))
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", rl.rate))
			w.Header().Set("X-RateLimit-Remaining", "0")
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// RateFromDuration converts a duration to a rate.Limit compatible value
func RateFromDuration(d time.Duration) float64 {
	return float64(time.Second) / float64(d)
}

// Close closes the Redis connection
func (rl *RedisRateLimiter) Close() error {
	if rl.client != nil {
		return rl.client.Close()
	}
	return nil
}

// Global Redis rate limiters (initialized in main)
var (
	RedisAuthLimiter    *RedisRateLimiter
	RedisGeneralLimiter *RedisRateLimiter
	RedisVerifyLimiter  *RedisRateLimiter
)

// InitRedisRateLimiters initializes Redis-based rate limiters
func InitRedisRateLimiters(cfg *RedisConfig) error {
	var err error

	RedisAuthLimiter, err = NewRedisRateLimiter(cfg, "ratelimit:auth", 5, time.Minute)
	if err != nil {
		return err
	}

	RedisGeneralLimiter, err = NewRedisRateLimiter(cfg, "ratelimit:general", 1000, time.Minute)
	if err != nil {
		return err
	}

	RedisVerifyLimiter, err = NewRedisRateLimiter(cfg, "ratelimit:verify", 10, time.Minute)
	if err != nil {
		return err
	}

	return nil
}

// GetRateLimiter returns either Redis or in-memory rate limiter based on availability
func GetRateLimiter(name string) interface{ Limit(http.Handler) http.Handler } {
	cfg := config.Get()

	// Use Redis if configured
	if cfg.RedisAddr != "" {
		switch name {
		case "auth":
			if RedisAuthLimiter != nil && RedisAuthLimiter.enabled {
				return RedisAuthLimiter
			}
		case "general":
			if RedisGeneralLimiter != nil && RedisGeneralLimiter.enabled {
				return RedisGeneralLimiter
			}
		case "verify":
			if RedisVerifyLimiter != nil && RedisVerifyLimiter.enabled {
				return RedisVerifyLimiter
			}
		}
	}

	// Fall back to in-memory
	switch name {
	case "auth":
		return AuthLimiter
	case "general":
		return GeneralLimiter
	case "verify":
		return VerifyLimiter
	default:
		return GeneralLimiter
	}
}
