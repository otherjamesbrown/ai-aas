// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//
//	These tests validate HTTP-level integration for streaming text completions.
//	They verify that the Stream field is correctly parsed through the full HTTP
//	request pipeline including all middleware (BodyBufferMiddleware, etc.).
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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

// mockStreamingBackendServer creates a mock backend that supports streaming completions.
// It returns Server-Sent Events (SSE) format for streaming requests.
func mockStreamingBackendServer(t *testing.T) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Parse the request to check if streaming is enabled
		var req public.OpenAICompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// If streaming is requested, return SSE format
		if req.Stream {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.WriteHeader(http.StatusOK)

			// Send streaming chunks
			chunks := []string{"Hello", " ", "world", "!"}
			for i, chunk := range chunks {
				// OpenAI streaming format: data: {json}\n\n
				chunkData := map[string]interface{}{
					"id":      "cmpl-test-123",
					"object":  "text_completion",
					"created": time.Now().Unix(),
					"model":   req.Model,
					"choices": []map[string]interface{}{
						{
							"text":          chunk,
							"index":         0,
							"finish_reason": nil,
						},
					},
				}
				if i == len(chunks)-1 {
					chunkData["choices"].([]map[string]interface{})[0]["finish_reason"] = "stop"
				}

				jsonData, _ := json.Marshal(chunkData)
				_, _ = w.Write([]byte("data: "))
				_, _ = w.Write(jsonData)
				_, _ = w.Write([]byte("\n\n"))

				// Flush immediately
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}
			}

			// Send [DONE] marker
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			return
		}

		// Non-streaming response
		response := public.OpenAICompletionResponse{
			ID:      "cmpl-test-123",
			Object:  "text_completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []public.OpenAICompletionChoice{
				{
					Text:         "Hello world!",
					Index:        0,
					FinishReason: "stop",
				},
			},
			Usage: public.OpenAIUsage{
				PromptTokens:     len(req.Prompt) / 4,
				CompletionTokens: 3,
				TotalTokens:      len(req.Prompt)/4 + 3,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(handler)
}

// TestStreamingTextCompletions_HTTPIntegration tests streaming text completions
// through the full HTTP pipeline with all middleware enabled.
// This is a regression test for aas-0ig0 / aas-wbxv.
func TestStreamingTextCompletions_HTTPIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup mock streaming backend server
	mockBackend := mockStreamingBackendServer(t)
	defer mockBackend.Close()

	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Seed a test routing policy with the mock backend URI
	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-streaming-1",
		OrganizationID: "*", // Global policy
		Model:          "gpt-4",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-streaming-backend-1",
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

	backendClient := routing.NewBackendClient(logger, 5*time.Second)

	// Create a minimal config for backend registry
	testCfg := &config.Config{
		BackendEndpoints: "", // Empty, we'll use SetBackendURI for testing
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)

	// Configure handler to use mock backend URI
	handler.SetBackendURI("mock-streaming-backend-1", mockBackend.URL+"/v1/completions")

	// Create router and register routes with ALL middleware enabled
	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Test cases for different request formats
	testCases := []struct {
		name        string
		requestBody map[string]interface{}
		contentType string
		expectSSE   bool
	}{
		{
			name: "streaming enabled - JSON body",
			requestBody: map[string]interface{}{
				"model":      "gpt-4",
				"prompt":     "Say hello",
				"stream":     true,
				"max_tokens": 100,
			},
			contentType: "application/json",
			expectSSE:   true,
		},
		{
			name: "streaming disabled - JSON body",
			requestBody: map[string]interface{}{
				"model":      "gpt-4",
				"prompt":     "Say hello",
				"stream":     false,
				"max_tokens": 100,
			},
			contentType: "application/json",
			expectSSE:   false,
		},
		{
			name: "streaming omitted (defaults to false) - JSON body",
			requestBody: map[string]interface{}{
				"model":      "gpt-4",
				"prompt":     "Say hello",
				"max_tokens": 100,
			},
			contentType: "application/json",
			expectSSE:   false,
		},
		{
			name: "streaming enabled - with charset in content-type",
			requestBody: map[string]interface{}{
				"model":      "gpt-4",
				"prompt":     "Say hello",
				"stream":     true,
				"max_tokens": 100,
			},
			contentType: "application/json; charset=utf-8",
			expectSSE:   true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal request body
			jsonBody, err := json.Marshal(tc.requestBody)
			if err != nil {
				t.Fatalf("failed to marshal request: %v", err)
			}

			// Create HTTP request
			req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", tc.contentType)
			req.Header.Set("X-API-Key", "dev-test-key")

			// Record response
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Validate response
			if w.Code != http.StatusOK {
				t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
				return
			}

			// Check if response format matches expectation
			responseContentType := w.Header().Get("Content-Type")
			if tc.expectSSE {
				// Should return SSE (text/event-stream)
				if !strings.Contains(responseContentType, "text/event-stream") {
					t.Errorf("expected SSE response (text/event-stream), got Content-Type: %s", responseContentType)
				}

				// Verify SSE format
				body := w.Body.String()
				if !strings.Contains(body, "data: ") {
					t.Errorf("expected SSE format with 'data:' prefix, got: %s", body)
				}
				if !strings.Contains(body, "[DONE]") {
					t.Errorf("expected SSE to end with [DONE] marker, got: %s", body)
				}
			} else {
				// Should return JSON
				if !strings.Contains(responseContentType, "application/json") {
					t.Errorf("expected JSON response, got Content-Type: %s", responseContentType)
				}

				// Verify JSON structure
				var response public.OpenAICompletionResponse
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
				}

				if response.ID == "" {
					t.Error("expected response to have an ID")
				}
				if len(response.Choices) != 1 {
					t.Errorf("expected 1 choice, got %d", len(response.Choices))
				}
			}

			// Verify routing headers are set
			if backendID := w.Header().Get("X-Routing-Backend"); backendID == "" {
				t.Error("expected X-Routing-Backend header")
			}
			if decision := w.Header().Get("X-Routing-Decision"); decision == "" {
				t.Error("expected X-Routing-Decision header")
			}
		})
	}
}

// TestStreamingTextCompletions_BodyBuffering tests that BodyBufferMiddleware
// correctly handles streaming requests without breaking the Stream field.
// This is a focused test for the middleware interaction discovered in aas-0ig0.
func TestStreamingTextCompletions_BodyBuffering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup mock streaming backend
	mockBackend := mockStreamingBackendServer(t)
	defer mockBackend.Close()

	// Setup minimal handler with only BodyBufferMiddleware
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Seed routing policy
	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-buffering-1",
		OrganizationID: "*",
		Model:          "gpt-4",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-buffering-backend-1",
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

	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	testCfg := &config.Config{BackendEndpoints: ""}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)
	handler.SetBackendURI("mock-buffering-backend-1", mockBackend.URL+"/v1/completions")

	// Create router with BodyBufferMiddleware ENABLED
	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024)) // Enable buffering
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Request with stream:true
	requestBody := map[string]interface{}{
		"model":      "gpt-4",
		"prompt":     "Say hello",
		"stream":     true,
		"max_tokens": 100,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify streaming response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	responseContentType := w.Header().Get("Content-Type")
	if !strings.Contains(responseContentType, "text/event-stream") {
		t.Errorf("expected SSE response with BodyBufferMiddleware enabled, got Content-Type: %s", responseContentType)
	}

	body := w.Body.String()
	if !strings.Contains(body, "data: ") {
		t.Errorf("expected SSE format, got: %s", body)
	}
}

// TestStreamingTextCompletions_WithoutBuffering tests streaming without BodyBufferMiddleware.
// This serves as a control to verify the handler works correctly when middleware is disabled.
func TestStreamingTextCompletions_WithoutBuffering(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup mock streaming backend
	mockBackend := mockStreamingBackendServer(t)
	defer mockBackend.Close()

	// Setup minimal handler WITHOUT BodyBufferMiddleware
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Seed routing policy
	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-no-buffering-1",
		OrganizationID: "*",
		Model:          "gpt-4",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-no-buffering-backend-1",
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

	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	testCfg := &config.Config{BackendEndpoints: ""}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)
	handler.SetBackendURI("mock-no-buffering-backend-1", mockBackend.URL+"/v1/completions")

	// Create router WITHOUT BodyBufferMiddleware
	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	// NOTE: BodyBufferMiddleware is NOT used here
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Request with stream:true
	requestBody := map[string]interface{}{
		"model":      "gpt-4",
		"prompt":     "Say hello",
		"stream":     true,
		"max_tokens": 100,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify streaming response works without middleware too
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	responseContentType := w.Header().Get("Content-Type")
	if !strings.Contains(responseContentType, "text/event-stream") {
		t.Errorf("expected SSE response without BodyBufferMiddleware, got Content-Type: %s", responseContentType)
	}
}

// TestStreamingTextCompletions_LargePayload tests streaming with a large request body
// to verify that BodyBufferMiddleware handles size limits correctly.
func TestStreamingTextCompletions_LargePayload(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup mock streaming backend
	mockBackend := mockStreamingBackendServer(t)
	defer mockBackend.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-large-1",
		OrganizationID: "*",
		Model:          "gpt-4",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-large-backend-1",
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

	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	testCfg := &config.Config{BackendEndpoints: ""}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)
	handler.SetBackendURI("mock-large-backend-1", mockBackend.URL+"/v1/completions")

	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024)) // 64KB limit
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Create a large prompt (but within the 64KB limit)
	largePrompt := strings.Repeat("A", 10000) // 10KB prompt

	requestBody := map[string]interface{}{
		"model":      "gpt-4",
		"prompt":     largePrompt,
		"stream":     true,
		"max_tokens": 100,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify streaming works with large payloads
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	responseContentType := w.Header().Get("Content-Type")
	if !strings.Contains(responseContentType, "text/event-stream") {
		t.Errorf("expected SSE response with large payload, got Content-Type: %s", responseContentType)
	}
}

// TestStreamingTextCompletions_RealBodyRead tests that the handler actually reads
// the request body correctly after BodyBufferMiddleware has buffered it.
// This is a regression test for the specific issue where json.NewDecoder(r.Body)
// was reading from a stale body reference.
func TestStreamingTextCompletions_RealBodyRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a custom handler that verifies the body is read correctly
	streamFieldCaptured := false
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate what HandleOpenAICompletions does
		var openAIReq public.OpenAICompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&openAIReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("decode failed: " + err.Error()))
			return
		}

		// Capture the stream field value
		streamFieldCaptured = openAIReq.Stream

		// Return success
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	// Wrap with BodyBufferMiddleware
	middleware := public.BodyBufferMiddleware(64 * 1024)
	handler := middleware(testHandler)

	// Create request with stream:true
	requestBody := map[string]interface{}{
		"model":  "gpt-4",
		"prompt": "test",
		"stream": true,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify the handler successfully decoded the body
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Verify the stream field was captured correctly
	if !streamFieldCaptured {
		t.Error("Stream field was not captured correctly after BodyBufferMiddleware - this is the bug!")
	}
}

// TestNonStreamingTextCompletions_HTTPIntegration is a control test to verify
// non-streaming completions work correctly through the full HTTP pipeline.
func TestNonStreamingTextCompletions_HTTPIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup mock backend
	mockBackend := mockStreamingBackendServer(t)
	defer mockBackend.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-non-streaming-1",
		OrganizationID: "*",
		Model:          "gpt-4",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-non-streaming-backend-1",
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

	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	testCfg := &config.Config{BackendEndpoints: ""}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)
	handler.SetBackendURI("mock-non-streaming-backend-1", mockBackend.URL+"/v1/completions")

	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Non-streaming request
	requestBody := map[string]interface{}{
		"model":      "gpt-4",
		"prompt":     "Say hello",
		"stream":     false,
		"max_tokens": 100,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify JSON response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	responseContentType := w.Header().Get("Content-Type")
	if !strings.Contains(responseContentType, "application/json") {
		t.Errorf("expected JSON response for non-streaming request, got Content-Type: %s", responseContentType)
	}

	var response public.OpenAICompletionResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if response.ID == "" {
		t.Error("expected response to have an ID")
	}
	if len(response.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Text != "Hello world!" {
		t.Errorf("expected text 'Hello world!', got: %s", response.Choices[0].Text)
	}
}
