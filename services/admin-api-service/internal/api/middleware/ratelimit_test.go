package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewRateLimiter_BackwardCompatibility(t *testing.T) {
	// Test that NewRateLimiter still works for backward compatibility
	rl := NewRateLimiter(100)
	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}
	defer rl.Stop()

	// Both traffic classes should have the same rate
	if rl.dataRate != rl.controlRate {
		t.Errorf("expected equal rates for backward compatibility, got data=%v control=%v",
			rl.dataRate, rl.controlRate)
	}
}

func TestNewRateLimiterWithConfig(t *testing.T) {
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    100,
		ControlPlaneRequestsPerMin: 500,
	}

	rl := NewRateLimiterWithConfig(cfg)
	if rl == nil {
		t.Fatal("expected non-nil rate limiter")
	}
	defer rl.Stop()

	// Verify rates are calculated correctly
	expectedDataRate := 100.0 / 60.0
	expectedControlRate := 500.0 / 60.0

	if float64(rl.dataRate) != expectedDataRate {
		t.Errorf("expected data rate %v, got %v", expectedDataRate, rl.dataRate)
	}

	if float64(rl.controlRate) != expectedControlRate {
		t.Errorf("expected control rate %v, got %v", expectedControlRate, rl.controlRate)
	}

	// Verify burst is 10% of rate
	if rl.dataBurst != 10 {
		t.Errorf("expected data burst 10, got %v", rl.dataBurst)
	}

	if rl.controlBurst != 50 {
		t.Errorf("expected control burst 50, got %v", rl.controlBurst)
	}
}

func TestClassifyTraffic(t *testing.T) {
	tests := []struct {
		name          string
		headers       map[string]string
		expectedClass TrafficClass
	}{
		{
			name: "X-Client-Type: operator",
			headers: map[string]string{
				"X-Client-Type": "operator",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "X-Client-Type: control-plane",
			headers: map[string]string{
				"X-Client-Type": "control-plane",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "X-Client-Type: OPERATOR (case insensitive)",
			headers: map[string]string{
				"X-Client-Type": "OPERATOR",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "User-Agent: ai-model-operator",
			headers: map[string]string{
				"User-Agent": "ai-model-operator/v1.0.0",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "User-Agent: kubernetes-operator",
			headers: map[string]string{
				"User-Agent": "kubernetes-operator/1.0",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "User-Agent: AI-Model-Operator (case insensitive)",
			headers: map[string]string{
				"User-Agent": "AI-Model-Operator/1.0",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "Regular user agent",
			headers: map[string]string{
				"User-Agent": "curl/7.68.0",
			},
			expectedClass: TrafficClassDataPlane,
		},
		{
			name:          "No identifying headers",
			headers:       map[string]string{},
			expectedClass: TrafficClassDataPlane,
		},
		{
			name: "X-Client-Type takes precedence over User-Agent when set to operator",
			headers: map[string]string{
				"X-Client-Type": "operator",
				"User-Agent":    "curl/7.68.0",
			},
			expectedClass: TrafficClassControlPlane,
		},
		{
			name: "User-Agent fallback when X-Client-Type not operator",
			headers: map[string]string{
				"X-Client-Type": "user",
				"User-Agent":    "ai-model-operator/1.0",
			},
			expectedClass: TrafficClassControlPlane,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			class := classifyTraffic(req)
			if class != tt.expectedClass {
				t.Errorf("expected traffic class %v, got %v", tt.expectedClass, class)
			}
		})
	}
}

func TestRateLimiter_DifferentLimitsPerClass(t *testing.T) {
	// Create rate limiter with different limits for testing
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    60,  // 1 per second, burst=6
		ControlPlaneRequestsPerMin: 300, // 5 per second, burst=30
	}

	rl := NewRateLimiterWithConfig(cfg)
	defer rl.Stop()

	// Data plane should be limited more strictly
	dataLimiter := rl.getLimiter("test-key-data", TrafficClassDataPlane)
	controlLimiter := rl.getLimiter("test-key-control", TrafficClassControlPlane)

	// Control plane should allow more requests in burst
	controlAllowed := 0
	for i := 0; i < 35; i++ {
		if controlLimiter.Allow() {
			controlAllowed++
		}
	}

	dataAllowed := 0
	for i := 0; i < 35; i++ {
		if dataLimiter.Allow() {
			dataAllowed++
		}
	}

	// Control plane should have allowed more requests due to higher burst
	if controlAllowed <= dataAllowed {
		t.Errorf("expected control plane to allow more requests than data plane, got control=%d data=%d",
			controlAllowed, dataAllowed)
	}

	// Control plane should allow significantly more (at least 3x)
	if controlAllowed < dataAllowed*3 {
		t.Errorf("expected control plane to allow at least 3x more requests, got control=%d data=%d",
			controlAllowed, dataAllowed)
	}
}

func TestRateLimiter_IsolationBetweenClasses(t *testing.T) {
	// Verify that exhausting one traffic class doesn't affect the other
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    6,  // Very low for testing
		ControlPlaneRequestsPerMin: 60, // Higher limit
	}

	rl := NewRateLimiterWithConfig(cfg)
	defer rl.Stop()

	// Exhaust data plane limit
	dataLimiter := rl.getLimiter("test-key", TrafficClassDataPlane)
	for i := 0; i < 10; i++ {
		dataLimiter.Allow()
	}

	// Data plane should be exhausted
	if dataLimiter.Allow() {
		t.Error("expected data plane to be rate limited")
	}

	// Control plane should still work
	controlLimiter := rl.getLimiter("test-key", TrafficClassControlPlane)
	if !controlLimiter.Allow() {
		t.Error("expected control plane to still allow requests despite data plane being limited")
	}
}

func TestRateLimit_Middleware(t *testing.T) {
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    60,
		ControlPlaneRequestsPerMin: 300,
	}

	rl := NewRateLimiterWithConfig(cfg)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name          string
		headers       map[string]string
		apiKeyID      string
		expectAllowed bool
	}{
		{
			name: "Data plane request with API key",
			headers: map[string]string{
				"User-Agent": "curl/7.68.0",
			},
			apiKeyID:      "user-key-123",
			expectAllowed: true,
		},
		{
			name: "Control plane request with API key",
			headers: map[string]string{
				"X-Client-Type": "operator",
			},
			apiKeyID:      "operator-key-456",
			expectAllowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Add API key ID to context
			ctx := context.WithValue(req.Context(), APIKeyContextKey, tt.apiKeyID)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if tt.expectAllowed && rr.Code != http.StatusOK {
				t.Errorf("expected request to be allowed (200), got %d", rr.Code)
			}
		})
	}
}

func TestRateLimit_429Response(t *testing.T) {
	// Create rate limiter with very low limit
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    60,
		ControlPlaneRequestsPerMin: 60,
	}

	rl := NewRateLimiterWithConfig(cfg)
	defer rl.Stop()

	handler := RateLimit(rl)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name          string
		headers       map[string]string
		expectedClass string
	}{
		{
			name: "Data plane 429",
			headers: map[string]string{
				"User-Agent": "curl/7.68.0",
			},
			expectedClass: "data-plane",
		},
		{
			name: "Control plane 429",
			headers: map[string]string{
				"X-Client-Type": "operator",
			},
			expectedClass: "control-plane",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a unique key for each test to isolate rate limiters
			testKey := "test-key-" + tt.name

			// Exhaust the rate limit first
			for i := 0; i < 20; i++ {
				req := httptest.NewRequest(http.MethodGet, "/test", nil)
				for k, v := range tt.headers {
					req.Header.Set(k, v)
				}
				ctx := context.WithValue(req.Context(), APIKeyContextKey, testKey)
				req = req.WithContext(ctx)
				rr := httptest.NewRecorder()
				handler.ServeHTTP(rr, req)
			}

			// Now the next request should be rate limited
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			// Add API key ID to context
			ctx := context.WithValue(req.Context(), APIKeyContextKey, testKey)
			req = req.WithContext(ctx)

			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code != http.StatusTooManyRequests {
				t.Errorf("expected 429, got %d", rr.Code)
			}

			// Check that traffic class is included in response
			trafficClass := rr.Header().Get("X-RateLimit-Class")
			if trafficClass != tt.expectedClass {
				t.Errorf("expected X-RateLimit-Class=%s, got %s", tt.expectedClass, trafficClass)
			}

			// Check Retry-After header
			retryAfter := rr.Header().Get("Retry-After")
			if retryAfter != "60" {
				t.Errorf("expected Retry-After=60, got %s", retryAfter)
			}
		})
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    100,
		ControlPlaneRequestsPerMin: 500,
	}

	rl := NewRateLimiterWithConfig(cfg)
	defer rl.Stop()

	// Create some limiters
	rl.getLimiter("data-key-1", TrafficClassDataPlane)
	rl.getLimiter("data-key-2", TrafficClassDataPlane)
	rl.getLimiter("control-key-1", TrafficClassControlPlane)

	// Verify they exist
	if len(rl.dataLimiters) != 2 {
		t.Errorf("expected 2 data limiters, got %d", len(rl.dataLimiters))
	}
	if len(rl.controlLimiters) != 1 {
		t.Errorf("expected 1 control limiter, got %d", len(rl.controlLimiters))
	}

	// Wait a bit and run cleanup with very short max age
	time.Sleep(10 * time.Millisecond)
	rl.Cleanup(1 * time.Millisecond)

	// All limiters should be cleaned up
	if len(rl.dataLimiters) != 0 {
		t.Errorf("expected 0 data limiters after cleanup, got %d", len(rl.dataLimiters))
	}
	if len(rl.controlLimiters) != 0 {
		t.Errorf("expected 0 control limiters after cleanup, got %d", len(rl.controlLimiters))
	}
}

func TestRateLimiter_PerKeyIsolation(t *testing.T) {
	// Verify that different keys get independent rate limiters
	cfg := RateLimiterConfig{
		DataPlaneRequestsPerMin:    60, // 1 per second, burst=6
		ControlPlaneRequestsPerMin: 60,
	}

	rl := NewRateLimiterWithConfig(cfg)
	defer rl.Stop()

	// Exhaust limit for key1
	limiter1 := rl.getLimiter("key1", TrafficClassDataPlane)
	for i := 0; i < 10; i++ {
		limiter1.Allow()
	}

	// key1 should be limited (consumed burst)
	if limiter1.Allow() {
		t.Error("expected key1 to be rate limited after exhausting burst")
	}

	// key2 should still work (fresh limiter with full burst)
	limiter2 := rl.getLimiter("key2", TrafficClassDataPlane)
	if !limiter2.Allow() {
		t.Error("expected key2 to allow requests (independent from key1)")
	}
}
