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
		ip := GetRealIP(r)

		limiter := rl.getVisitor(ip)
		if !limiter.Allow() {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// GetRealIP extracts the client IP from the request
func GetRealIP(r *http.Request) string {
	cfg := config.Get()

	remoteIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		remoteIP = r.RemoteAddr
	}

	if !cfg.TrustProxy {
		return remoteIP
	}

	if !cfg.IsTrustedProxy(remoteIP) {
		return remoteIP
	}

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

// Rate limiters for different endpoint types
var (
	AuthLimiter     = NewRateLimiter(rate.Every(2*time.Second), 3)
	GeneralLimiter  = NewRateLimiter(rate.Every(100*time.Millisecond), 100)
	VerifyLimiter   = NewRateLimiter(rate.Every(time.Second), 2)
	TrackingLimiter = NewRateLimiter(rate.Every(100*time.Millisecond), 20)
	ImportLimiter   = NewRateLimiter(rate.Every(5*time.Second), 2)
)
