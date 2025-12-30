package custom

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/pulak-ranjan/sampmail/internal/config"
	"golang.org/x/time/rate"
)

type RateLimiter struct {
	visitors map[string]*visitor
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

func NewRateLimiter(r rate.Limit, b int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		rate:     r,
		burst:    b,
	}
	go rl.cleanupVisitors()
	return rl
}

func (rl *RateLimiter) getVisitor(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, exists := rl.visitors[ip]
	if !exists {
		limiter := rate.NewLimiter(rl.rate, rl.burst)
		rl.visitors[ip] = &visitor{limiter, time.Now()}
		return limiter
	}
	v.lastSeen = time.Now()
	return v.limiter
}

func (rl *RateLimiter) cleanupVisitors() {
	for {
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 3*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Limit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the real client IP, respecting proxy headers if configured
		ip := GetRealIP(r)

		limiter := rl.getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetRealIP extracts the real client IP from the request
// It respects X-Forwarded-For and X-Real-IP headers when behind trusted proxies
func GetRealIP(r *http.Request) string {
	cfg := config.Get()

	// Get the direct connection IP
	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	// If proxy trust is not enabled, just return the direct IP
	if !cfg.TrustProxy {
		return remoteIP
	}

	// Check if the direct connection is from a trusted proxy
	if !cfg.IsTrustedProxy(remoteIP) {
		return remoteIP
	}

	// Try X-Forwarded-For first (standard header)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range parts {
			ip := strings.TrimSpace(part)
			if parsedIP := net.ParseIP(ip); parsedIP != nil {
				if !parsedIP.IsPrivate() && !parsedIP.IsLoopback() {
					return ip
				}
			}
		}
	}

	// Try X-Real-IP (common with nginx)
	if xrip := r.Header.Get("X-Real-IP"); xrip != "" {
		ip := strings.TrimSpace(xrip)
		if parsedIP := net.ParseIP(ip); parsedIP != nil {
			if !parsedIP.IsPrivate() && !parsedIP.IsLoopback() {
				return ip
			}
		}
	}

	return remoteIP
}

// Specific limiters for different endpoints
var (
	AuthLimiter    = NewRateLimiter(rate.Every(time.Second), 5)
	GeneralLimiter = NewRateLimiter(rate.Every(100*time.Millisecond), 100)
	VerifyLimiter  = NewRateLimiter(rate.Every(time.Second), 2)
)
