package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// TrafficClass represents the type of traffic for rate limiting
type TrafficClass string

const (
	// TrafficClassControlPlane is for operator/system control plane traffic
	TrafficClassControlPlane TrafficClass = "control-plane"
	// TrafficClassDataPlane is for user API traffic
	TrafficClassDataPlane TrafficClass = "data-plane"
)

// limiterEntry wraps a rate limiter with last access time for cleanup
type limiterEntry struct {
	limiter    *rate.Limiter
	lastAccess time.Time
}

// RateLimiterConfig holds configuration for different traffic classes
type RateLimiterConfig struct {
	// DataPlaneRequestsPerMin is the rate limit for user API traffic
	DataPlaneRequestsPerMin int
	// ControlPlaneRequestsPerMin is the rate limit for operator/system traffic
	ControlPlaneRequestsPerMin int
}

// RateLimiter implements per-key rate limiting with automatic cleanup
// and traffic class differentiation
type RateLimiter struct {
	// Per-key limiters for data plane traffic
	dataLimiters map[string]*limiterEntry
	// Per-key limiters for control plane traffic
	controlLimiters map[string]*limiterEntry
	mu              sync.RWMutex

	// Rate and burst for data plane
	dataRate  rate.Limit
	dataBurst int

	// Rate and burst for control plane
	controlRate  rate.Limit
	controlBurst int

	stopCh chan struct{}
}

// NewRateLimiter creates a new rate limiter with automatic cleanup
// Deprecated: Use NewRateLimiterWithConfig for traffic class differentiation
func NewRateLimiter(requestsPerMinute int) *RateLimiter {
	return NewRateLimiterWithConfig(RateLimiterConfig{
		DataPlaneRequestsPerMin:    requestsPerMinute,
		ControlPlaneRequestsPerMin: requestsPerMinute,
	})
}

// calculateBurst calculates burst size ensuring at least 1
func calculateBurst(requestsPerMin int) int {
	burst := requestsPerMin / 10 // 10% of rate
	if burst < 1 {
		burst = 1 // Ensure at least 1 request can burst
	}
	return burst
}

// NewRateLimiterWithConfig creates a new rate limiter with traffic class differentiation
func NewRateLimiterWithConfig(cfg RateLimiterConfig) *RateLimiter {
	rl := &RateLimiter{
		dataLimiters:    make(map[string]*limiterEntry),
		controlLimiters: make(map[string]*limiterEntry),
		dataRate:        rate.Limit(float64(cfg.DataPlaneRequestsPerMin) / 60.0),
		dataBurst:       calculateBurst(cfg.DataPlaneRequestsPerMin),
		controlRate:     rate.Limit(float64(cfg.ControlPlaneRequestsPerMin) / 60.0),
		controlBurst:    calculateBurst(cfg.ControlPlaneRequestsPerMin),
		stopCh:          make(chan struct{}),
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

// getLimiter returns the rate limiter for a given key and traffic class
func (rl *RateLimiter) getLimiter(key string, trafficClass TrafficClass) *rate.Limiter {
	now := time.Now()

	// Select the appropriate limiter map based on traffic class
	limiters := rl.dataLimiters
	limiterRate := rl.dataRate
	limiterBurst := rl.dataBurst
	if trafficClass == TrafficClassControlPlane {
		limiters = rl.controlLimiters
		limiterRate = rl.controlRate
		limiterBurst = rl.controlBurst
	}

	rl.mu.RLock()
	entry, exists := limiters[key]
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
	if entry, exists = limiters[key]; exists {
		entry.lastAccess = now
		return entry.limiter
	}

	limiter := rate.NewLimiter(limiterRate, limiterBurst)
	limiters[key] = &limiterEntry{
		limiter:    limiter,
		lastAccess: now,
	}
	return limiter
}

// classifyTraffic determines the traffic class based on request headers
func classifyTraffic(r *http.Request) TrafficClass {
	// Check for X-Client-Type header (primary method)
	clientType := r.Header.Get("X-Client-Type")
	if strings.EqualFold(clientType, "operator") || strings.EqualFold(clientType, "control-plane") {
		return TrafficClassControlPlane
	}

	// Check User-Agent for operator identification (fallback)
	userAgent := r.Header.Get("User-Agent")
	if strings.Contains(strings.ToLower(userAgent), "ai-model-operator") ||
		strings.Contains(strings.ToLower(userAgent), "kubernetes-operator") {
		return TrafficClassControlPlane
	}

	// Default to data plane (user traffic)
	return TrafficClassDataPlane
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

			// Classify traffic to apply appropriate rate limit
			trafficClass := classifyTraffic(r)

			limiter := rl.getLimiter(key, trafficClass)
			if !limiter.Allow() {
				writeTooManyRequests(w, trafficClass)
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

	// Clean up both data plane and control plane limiters
	for key, entry := range rl.dataLimiters {
		if entry.lastAccess.Before(cutoff) {
			delete(rl.dataLimiters, key)
		}
	}

	for key, entry := range rl.controlLimiters {
		if entry.lastAccess.Before(cutoff) {
			delete(rl.controlLimiters, key)
		}
	}
}

func writeTooManyRequests(w http.ResponseWriter, trafficClass TrafficClass) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.Header().Set("Retry-After", "60")
	w.Header().Set("X-RateLimit-Class", string(trafficClass))
	w.WriteHeader(http.StatusTooManyRequests)
	w.Write([]byte(`{"type":"https://docs.otherjamesbrown.com/errors/rate-limit","title":"Too Many Requests","status":429,"detail":"Rate limit exceeded. Please retry after 60 seconds."}`))
}
