package middleware

import (
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// limiterEntry wraps a rate limiter with last access time for cleanup
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiter implements per-key rate limiting with automatic cleanup
type RateLimiter struct {
	limiters map[string]*limiterEntry
	mu       sync.RWMutex
	rate     rate.Limit
	burst    int
	stopCh   chan struct{}
}

// NewRateLimiter creates a new rate limiter with automatic cleanup
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		rate:     rate.Limit(float64(requestsPerMinute) / 60.0),
		burst:    requestsPerMinute / 10, // Allow burst of 10% of rate
		stopCh:   make(chan struct{}),
	}

	// Start background cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// cleanupLoop periodically removes stale limiters
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.Cleanup(10 * time.Minute)
		case <-rl.stopCh:
			return
		}
	}
}

// Stop stops the cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

// getLimiter returns the rate limiter for a given key
func (rl *RateLimiter) getLimiter(key string) *rate.Limiter {
	now := time.Now()

	rl.mu.RLock()
	entry, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		// Update last access time
		rl.mu.Lock()
		entry.lastAccess = now
		rl.mu.Unlock()
		return entry.limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if entry, exists = rl.limiters[key]; exists {
		entry.lastAccess = now
		return entry.limiter
	}

	limiter := rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[key] = &limiterEntry{
		limiter:    limiter,
		lastAccess: now,
	}
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

// Cleanup removes limiters that haven't been accessed within maxAge
func (rl *RateLimiter) Cleanup(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for key, entry := range rl.limiters {
		if entry.lastAccess.Before(cutoff) {
			delete(rl.limiters, key)
		}
	}
}

func writeTooManyRequests(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Retry-After", "60")
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"type":"https://docs.ai-aas.local/errors/rate-limit","title":"Too Many Requests","status":429,"detail":"Rate limit exceeded. Please retry after 60 seconds."}`))
}
