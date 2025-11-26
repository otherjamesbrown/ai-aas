package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter implements per-key rate limiting
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:    requestsPerMinute / 10, // Allow burst of 10% of rate
	}
}

// getLimiter returns the rate limiter for a given key
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[key] = limiter
	return limiter
}

// RateLimit creates a rate limiting middleware
func RateLimit(rl *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get rate limit key (API key ID from auth middleware)
			key := GetAPIKeyID(r.Context())
			if key == "" {
				// Fall back to IP address for unauthenticated endpoints
				key = r.RemoteAddr
			}

			limiter := rl.getLimiter(key)
			if !limiter.Allow() {
				writeTooManyRequests(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Cleanup removes stale limiters (call periodically)
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	// In production, track last access time and remove stale entries
	// For simplicity, this is a no-op placeholder
}

func writeTooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"type":"https://docs.ai-aas.local/errors/rate-limit","title":"Too Many Requests","status":429,"detail":"Rate limit exceeded. Please retry after 60 seconds."}`))
}

