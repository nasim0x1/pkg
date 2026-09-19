package middleware

import (
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/nasim0x1/pkg/response"
)

type clientBucket struct {
	tokens     float64
	lastRefill time.Time
}

// RateLimiter implements a sliding-window token bucket rate limiter
type RateLimiter struct {
	mu         sync.Mutex
	clients    map[string]*clientBucket
	rate       float64 // tokens added per second
	burst      float64 // max burst capacity
	cleanupDur time.Duration
}

func NewRateLimiter(rate, burst float64) *RateLimiter {
	rl := &RateLimiter{
		clients:    make(map[string]*clientBucket),
		rate:       rate,
		burst:      burst,
		cleanupDur: 3 * time.Minute,
	}

	go rl.cleanupRoutine()
	return rl
}

func (rl *RateLimiter) cleanupRoutine() {
	ticker := time.NewTicker(rl.cleanupDur)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, bucket := range rl.clients {
			if now.Sub(bucket.lastRefill) > rl.cleanupDur {
				delete(rl.clients, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(clientKey string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	bucket, exists := rl.clients[clientKey]
	if !exists {
		rl.clients[clientKey] = &clientBucket{
			tokens:     rl.burst - 1,
			lastRefill: now,
		}
		return true
	}

	// Refill tokens
	elapsed := now.Sub(bucket.lastRefill).Seconds()
	bucket.tokens += elapsed * rl.rate
	if bucket.tokens > rl.burst {
		bucket.tokens = rl.burst
	}
	bucket.lastRefill = now

	if bucket.tokens >= 1 {
		bucket.tokens--
		return true
	}

	return false
}

// RateLimit returns a middleware that limits requests based on client IP
func RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := getClientIP(r)
			if !limiter.Allow(ip) {
				w.Header().Set("Retry-After", "1")
				response.TooManyRequests(w, "rate limit exceeded, please retry shortly")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		return xff
	}
	xri := r.Header.Get("X-Real-IP")
	if xri != "" {
		return xri
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
