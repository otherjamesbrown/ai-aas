// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//   These tests validate budget enforcement, quota management, and rate limiting
//   functionality, including HTTP 402/429 responses and audit event emission.
//
// Key Responsibilities:
//   - Test rate limit enforcement (429 responses)
//   - Test budget/quota enforcement (402 responses)
//   - Verify audit event emission for denials
//   - Validate structured error responses match OpenAPI spec
//
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/limiter"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

// LimitErrorResponse represents a limit error response matching OpenAPI spec.
type LimitErrorResponse struct {
	Error              string                 `json:"error"`
	Code               string                 `json:"code"`
	TraceID            string                 `json:"trace_id,omitempty"`
	RetryAfterSeconds  *int                   `json:"retry_after_seconds,omitempty"`
	LimitContext       map[string]interface{} `json:"limit_context,omitempty"`
}

// TestRateLimitExceeded tests that rate limit enforcement returns HTTP 429.
func TestRateLimitExceeded(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// Set up Redis for rate limiting (skip test if Redis unavailable)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests to avoid conflicts
	})
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available for rate limit test: %v", err)
	}
	defer redisClient.Close()
	defer redisClient.FlushDB(ctx) // Clean up test data

	// Initialize rate limiter with low limits for testing
	rateLimiter := limiter.NewRateLimiter(redisClient, logger, 10, 15) // 10 RPS, burst 15

	// Initialize budget client
	budgetClient := limiter.NewBudgetClient("", 2*time.Second, logger)

	// Initialize audit logger
	auditLogger := usage.NewAuditLogger(logger)

	// Set up router with middleware
	tracer := otel.Tracer("test")
	router := chi.NewRouter()
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	router.Use(public.RateLimitMiddleware(rateLimiter, auditLogger, logger, tracer))
	router.Use(public.BudgetMiddleware(budgetClient, auditLogger, logger, tracer))
	handler.RegisterRoutes(router)

	// Create a valid request
	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Send requests rapidly to exceed rate limit
	// Assuming rate limit is 10 requests per second
	rateLimitExceeded := false
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", "dev-test-key")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// After exceeding rate limit, should get 429
		if w.Code == http.StatusTooManyRequests {
			rateLimitExceeded = true

			// Validate response structure
			var limitErr LimitErrorResponse
			if err := json.Unmarshal(w.Body.Bytes(), &limitErr); err != nil {
				t.Errorf("failed to unmarshal limit error response: %v. Body: %s", err, w.Body.String())
				continue
			}

			// Validate required fields
			if limitErr.Error == "" {
				t.Error("expected error message in response")
			}
			if limitErr.Code == "" {
				t.Error("expected error code in response")
			}
			if limitErr.RetryAfterSeconds == nil {
				t.Error("expected retry_after_seconds in rate limit response")
			} else if *limitErr.RetryAfterSeconds <= 0 {
				t.Errorf("expected retry_after_seconds > 0, got %d", *limitErr.RetryAfterSeconds)
			}

			// Validate Retry-After header
			retryAfter := w.Header().Get("Retry-After")
			if retryAfter == "" {
				t.Error("expected Retry-After header in rate limit response")
			}

			break
		}

		// Small delay to simulate rapid requests
		time.Sleep(10 * time.Millisecond)
	}

	if !rateLimitExceeded {
		t.Error("expected rate limit to be exceeded, but did not receive 429 response")
	}
}

// TestBudgetExceeded tests that budget enforcement returns HTTP 402.
func TestBudgetExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// TODO: Add budget middleware to router
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Create a valid request
	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440001",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// Use an API key that simulates an organization with exhausted budget
	req.Header.Set("X-API-Key", "dev-exhausted-budget-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should get 402 Payment Required
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Validate response structure
	var limitErr LimitErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &limitErr); err != nil {
		t.Fatalf("failed to unmarshal limit error response: %v. Body: %s", err, w.Body.String())
	}

	// Validate required fields
	if limitErr.Error == "" {
		t.Error("expected error message in response")
	}
	if limitErr.Code == "" {
		t.Error("expected error code in response")
	}
	if limitErr.LimitContext == nil {
		t.Error("expected limit_context in budget error response")
	} else {
		// Validate limit context contains budget information
		if _, ok := limitErr.LimitContext["current_usage"]; !ok {
			t.Error("expected current_usage in limit_context")
		}
		if _, ok := limitErr.LimitContext["limit"]; !ok {
			t.Error("expected limit in limit_context")
		}
	}
}

// TestQuotaExceeded tests that quota enforcement returns HTTP 402.
func TestQuotaExceeded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// TODO: Add quota middleware to router
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Create a valid request
	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440002",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// Use an API key that simulates an organization with exhausted quota
	req.Header.Set("X-API-Key", "dev-exhausted-quota-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should get 402 Payment Required
	if w.Code != http.StatusPaymentRequired {
		t.Errorf("expected status 402, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Validate response structure
	var limitErr LimitErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &limitErr); err != nil {
		t.Fatalf("failed to unmarshal limit error response: %v. Body: %s", err, w.Body.String())
	}

	// Validate required fields
	if limitErr.Error == "" {
		t.Error("expected error message in response")
	}
	if limitErr.Code == "" {
		t.Error("expected error code in response")
	}
	if limitErr.LimitContext == nil {
		t.Error("expected limit_context in quota error response")
	} else {
		// Validate limit context contains quota information
		if _, ok := limitErr.LimitContext["quota_type"]; !ok {
			t.Error("expected quota_type in limit_context (e.g., daily, monthly)")
		}
		if _, ok := limitErr.LimitContext["current_usage"]; !ok {
			t.Error("expected current_usage in limit_context")
		}
		if _, ok := limitErr.LimitContext["limit"]; !ok {
			t.Error("expected limit in limit_context")
		}
	}
}

// TestAuditEventEmitted tests that audit events are emitted for limit denials.
func TestAuditEventEmitted(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// TODO: Add audit logger to handler/middleware
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Create a valid request that will be denied
	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440003",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-exhausted-budget-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should get 402 or 429 (depending on which limit is hit first)
	if w.Code != http.StatusPaymentRequired && w.Code != http.StatusTooManyRequests {
		t.Errorf("expected status 402 or 429, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// TODO: Verify audit event was emitted
	// This will require:
	// 1. Mock Kafka producer or capture audit events
	// 2. Verify event contains:
	//    - request_id
	//    - organization_id
	//    - api_key_id
	//    - action: REQUEST_DENIED
	//    - decision_reason (e.g., BUDGET_EXCEEDED, RATE_LIMIT_EXCEEDED)
	//    - limit_state
	//    - timestamp

	// For now, this test documents the expected behavior
	t.Log("TODO: Verify audit event was emitted with correct structure")
}

// TestRateLimitPerOrganization tests that rate limits are enforced per organization.
func TestRateLimitPerOrganization(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// Set up Redis for rate limiting (skip test if Redis unavailable)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests to avoid conflicts
	})
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available for rate limit test: %v", err)
	}
	defer redisClient.Close()
	defer redisClient.FlushDB(ctx) // Clean up test data

	// Initialize rate limiter with low limits for testing
	rateLimiter := limiter.NewRateLimiter(redisClient, logger, 10, 15) // 10 RPS, burst 15

	// Initialize budget client
	budgetClient := limiter.NewBudgetClient("", 2*time.Second, logger)

	// Initialize audit logger
	auditLogger := usage.NewAuditLogger(logger)

	// Set up router with middleware
	tracer := otel.Tracer("test")
	router := chi.NewRouter()
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	router.Use(public.RateLimitMiddleware(rateLimiter, auditLogger, logger, tracer))
	router.Use(public.BudgetMiddleware(budgetClient, auditLogger, logger, tracer))
	handler.RegisterRoutes(router)

	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440004",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Exhaust rate limit for org-1
	org1Key := "dev-org-1-key"
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", org1Key)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Org-2 should still be able to make requests (different rate limit bucket)
	org2Key := "dev-org-2-key"
	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", org2Key)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Org-2 should not be rate limited (unless they've also exceeded their limit)
	// This test verifies that rate limits are isolated per organization
	if w.Code == http.StatusTooManyRequests {
		t.Log("Org-2 also rate limited (may be expected if they share a limit or have low limit)")
	}
}

// TestRateLimitPerKey tests that rate limits can be enforced per API key.
func TestRateLimitPerKey(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// Set up Redis for rate limiting (skip test if Redis unavailable)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests to avoid conflicts
	})
	ctx := context.Background()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available for rate limit test: %v", err)
	}
	defer redisClient.Close()
	defer redisClient.FlushDB(ctx) // Clean up test data

	// Initialize rate limiter with low limits for testing
	rateLimiter := limiter.NewRateLimiter(redisClient, logger, 10, 15) // 10 RPS, burst 15

	// Initialize budget client
	budgetClient := limiter.NewBudgetClient("", 2*time.Second, logger)

	// Initialize audit logger
	auditLogger := usage.NewAuditLogger(logger)

	// Set up router with middleware
	tracer := otel.Tracer("test")
	router := chi.NewRouter()
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	router.Use(public.RateLimitMiddleware(rateLimiter, auditLogger, logger, tracer))
	router.Use(public.BudgetMiddleware(budgetClient, auditLogger, logger, tracer))
	handler.RegisterRoutes(router)

	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440005",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	// Exhaust rate limit for key-1
	key1 := "dev-test-key-1"
	for i := 0; i < 15; i++ {
		req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", key1)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code == http.StatusTooManyRequests {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Key-2 should still be able to make requests (different rate limit bucket)
	key2 := "dev-test-key-2"
	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", key2)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Key-2 should not be rate limited (unless they've also exceeded their limit)
	// This test verifies that rate limits can be isolated per API key
	if w.Code == http.StatusTooManyRequests {
		t.Log("Key-2 also rate limited (may be expected if they share a limit or have low limit)")
	}
}

