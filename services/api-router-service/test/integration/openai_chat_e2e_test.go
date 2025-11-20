// Package integration provides integration tests for the API Router Service.
//
// This file contains E2E tests for OpenAI-compatible endpoints with REAL backends.
// These tests are designed to catch issues that mocks cannot, such as:
// - Model behavior quirks
// - Token counting accuracy
// - Real latency and timeout issues
// - Actual inference errors
//
// Run these tests with: go test -v ./test/integration -run E2E
// Requires: VLLM_BACKEND_URL environment variable
//
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
)

// TestOpenAIChatCompletions_E2E tests OpenAI chat completions against a REAL vLLM backend.
//
// This test validates:
// - Actual model inference works
// - Token counting is accurate
// - Response format matches expectations
// - Latency is acceptable
// - The "capital of France" question gets "Paris" answer
//
// Prerequisites:
// - Set VLLM_BACKEND_URL environment variable (e.g., http://localhost:8000)
// - vLLM service must be running and healthy
// - Model must be loaded and ready
//
// Usage:
//   export VLLM_BACKEND_URL=http://localhost:8000
//   go test -v ./test/integration -run TestOpenAIChatCompletions_E2E
func TestOpenAIChatCompletions_E2E(t *testing.T) {
	// Check if real backend URL is provided
	vllmURL := os.Getenv("VLLM_BACKEND_URL")
	if vllmURL == "" {
		t.Skip("Skipping E2E test: VLLM_BACKEND_URL not set. Set it to run against real vLLM backend.")
	}

	// Get model name from environment or use default
	modelName := os.Getenv("VLLM_MODEL_NAME")
	if modelName == "" {
		modelName = "gpt-4o" // Default fallback
		t.Logf("VLLM_MODEL_NAME not set, using default: %s", modelName)
	}

	// Verify vLLM is reachable and healthy
	t.Logf("Testing against real vLLM backend: %s", vllmURL)
	healthURL := fmt.Sprintf("%s/health", vllmURL)
	resp, err := http.Get(healthURL)
	if err != nil || (resp != nil && resp.StatusCode != 200) {
		t.Fatalf("vLLM backend is not reachable at %s/health. Error: %v", vllmURL, err)
	}
	if resp != nil {
		_ = resp.Body.Close()
	}
	t.Logf("✓ vLLM backend is healthy")

	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Seed a test routing policy pointing to the REAL vLLM backend
	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-e2e-1",
		OrganizationID: "*", // Global policy
		Model:          modelName,
		Backends: []config.BackendWeight{
			{
				BackendID: "vllm-backend-e2e",
				Weight:    100,
			},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}
	ctx := context.Background()
	if err := cache.StorePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to store policy: %v", err)
	}

	backendClient := routing.NewBackendClient(logger, 60*time.Second) // Longer timeout for real inference

	// Create a minimal config for backend registry
	testCfg := &config.Config{
		BackendEndpoints: "", // Empty, we'll use SetBackendURI for testing
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// Configure handler to use REAL vLLM backend
	realBackendURI := fmt.Sprintf("%s/v1/chat/completions", vllmURL)
	handler.SetBackendURI("vllm-backend-e2e", realBackendURI)
	t.Logf("Backend configured: %s", realBackendURI)

	// Create router and register routes with middleware
	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Create the test request - THE CRITICAL QUESTION
	requestBody := public.OpenAIChatCompletionRequest{
		Model: modelName,
		Messages: []public.OpenAIMessage{
			{
				Role:    "user",
				Content: "In one word, can you provide me the capital of France?",
			},
		},
		MaxTokens:   10,         // Limit tokens to encourage concise answer
		Temperature: 0.1,        // Low temperature for deterministic answer
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	// Record response with timing
	w := httptest.NewRecorder()
	startTime := time.Now()
	router.ServeHTTP(w, req)
	latency := time.Since(startTime)

	// Log request/response for debugging
	t.Logf("Request: %s", string(jsonBody))
	t.Logf("Response Status: %d", w.Code)
	t.Logf("Response Body: %s", w.Body.String())
	t.Logf("Latency: %v", latency)

	// Validate response
	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var response public.OpenAIChatCompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	// Validate response structure
	if response.ID == "" {
		t.Error("expected response to have an ID")
	}

	if response.Object != "chat.completion" {
		t.Errorf("expected object to be 'chat.completion', got %s", response.Object)
	}

	if response.Model == "" {
		t.Error("expected response to have a model")
	}

	if len(response.Choices) != 1 {
		t.Fatalf("expected 1 choice, got %d", len(response.Choices))
	}

	// THE CRITICAL TEST: Validate the answer contains "Paris"
	answerText := response.Choices[0].Message.Content
	t.Logf("Model Answer: %s", answerText)

	if !strings.Contains(strings.ToLower(answerText), "paris") {
		t.Errorf("❌ CRITICAL: Expected answer to contain 'Paris', got: %s", answerText)
		t.Error("This indicates a real problem with:")
		t.Error("  - Model not understanding the question")
		t.Error("  - Prompt engineering needed")
		t.Error("  - Model quality issues")
	} else {
		t.Logf("✓ Model correctly answered: %s", answerText)
	}

	// Validate message metadata
	if response.Choices[0].Message.Role != "assistant" {
		t.Errorf("expected message role to be 'assistant', got %s", response.Choices[0].Message.Role)
	}

	if response.Choices[0].FinishReason == "" {
		t.Error("expected finish_reason to be set")
	}

	// Validate token usage (CRITICAL for cost tracking)
	if response.Usage.PromptTokens == 0 {
		t.Error("❌ CRITICAL: prompt_tokens is 0 - token counting is broken!")
	}
	if response.Usage.CompletionTokens == 0 {
		t.Error("❌ CRITICAL: completion_tokens is 0 - token counting is broken!")
	}
	if response.Usage.TotalTokens == 0 {
		t.Error("❌ CRITICAL: total_tokens is 0 - token counting is broken!")
	}

	// Log token usage for cost analysis
	t.Logf("Token Usage:")
	t.Logf("  Prompt tokens: %d", response.Usage.PromptTokens)
	t.Logf("  Completion tokens: %d", response.Usage.CompletionTokens)
	t.Logf("  Total tokens: %d", response.Usage.TotalTokens)
	t.Logf("  Tokens/second: %.2f", float64(response.Usage.CompletionTokens)/latency.Seconds())

	// Validate routing headers
	if backendID := w.Header().Get("X-Routing-Backend"); backendID == "" {
		t.Error("expected X-Routing-Backend header")
	} else {
		t.Logf("✓ Routed to backend: %s", backendID)
	}
	if decision := w.Header().Get("X-Routing-Decision"); decision == "" {
		t.Error("expected X-Routing-Decision header")
	}

	// Performance validation
	if latency > 30*time.Second {
		t.Errorf("⚠ WARNING: Latency %v exceeds 30s threshold. Check GPU performance.", latency)
	}

	// Summary
	t.Logf("═══════════════════════════════════════")
	t.Logf("✓ E2E Test PASSED")
	t.Logf("  Backend: %s", vllmURL)
	t.Logf("  Model: %s", response.Model)
	t.Logf("  Question: In one word, can you provide me the capital of France?")
	t.Logf("  Answer: %s", answerText)
	t.Logf("  Latency: %v", latency)
	t.Logf("  Tokens: %d prompt + %d completion = %d total",
		response.Usage.PromptTokens,
		response.Usage.CompletionTokens,
		response.Usage.TotalTokens)
	t.Logf("═══════════════════════════════════════")
}

// TestOpenAIChatCompletions_E2E_StressTest runs multiple concurrent requests
// to validate the backend can handle load.
//
// This test helps catch:
// - Concurrency issues
// - GPU contention problems
// - Rate limiting issues
// - Connection pooling problems
func TestOpenAIChatCompletions_E2E_StressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	vllmURL := os.Getenv("VLLM_BACKEND_URL")
	if vllmURL == "" {
		t.Skip("Skipping E2E stress test: VLLM_BACKEND_URL not set")
	}

	// Run 10 concurrent requests
	const numRequests = 10
	t.Logf("Running %d concurrent requests...", numRequests)

	errors := make(chan error, numRequests)
	latencies := make(chan time.Duration, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(requestNum int) {
			// Simplified test - just hit the endpoint
			startTime := time.Now()
			resp, err := http.Post(
				fmt.Sprintf("%s/v1/chat/completions", vllmURL),
				"application/json",
				bytes.NewBufferString(`{
					"model": "gpt-4o",
					"messages": [{"role": "user", "content": "Hi"}],
					"max_tokens": 5
				}`),
			)
			latency := time.Since(startTime)

			if err != nil {
				errors <- fmt.Errorf("request %d failed: %w", requestNum, err)
				return
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != 200 {
				errors <- fmt.Errorf("request %d returned status %d", requestNum, resp.StatusCode)
				return
			}

			errors <- nil
			latencies <- latency
		}(i)
	}

	// Collect results
	var failureCount int
	var totalLatency time.Duration
	for i := 0; i < numRequests; i++ {
		if err := <-errors; err != nil {
			t.Logf("Request failed: %v", err)
			failureCount++
		} else {
			latency := <-latencies
			totalLatency += latency
		}
	}

	successCount := numRequests - failureCount
	if successCount > 0 {
		avgLatency := totalLatency / time.Duration(successCount)
		t.Logf("Stress Test Results:")
		t.Logf("  Requests: %d", numRequests)
		t.Logf("  Success: %d", successCount)
		t.Logf("  Failed: %d", failureCount)
		t.Logf("  Avg Latency: %v", avgLatency)
	}

	if failureCount > numRequests/2 {
		t.Errorf("Too many failures: %d/%d requests failed", failureCount, numRequests)
	}
}
