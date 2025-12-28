// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//
//	These tests validate Triton backend integration with protocol translation.
//	Implements spec032 - Triton API Support.
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

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
)

// mockTritonServer creates a mock Triton V2 inference server for testing.
func mockTritonServer(t *testing.T, expectedModel string) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		// Check for Triton V2 inference endpoint pattern: /v2/models/{model}/infer
		expectedPath := "/v2/models/" + expectedModel + "/infer"
		if r.URL.Path != expectedPath {
			t.Logf("unexpected path: got %s, expected %s", r.URL.Path, expectedPath)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Parse Triton V2 request
		var tritonReq triton.InferRequest
		if err := json.NewDecoder(r.Body).Decode(&tritonReq); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Extract prompt from inputs
		var prompt string
		for _, input := range tritonReq.Inputs {
			if input.Name == "text_input" && len(input.Data) > 0 {
				if s, ok := input.Data[0].(string); ok {
					prompt = s
				}
			}
		}

		// Generate mock response
		responseText := "This is a mock Triton response."
		if strings.Contains(strings.ToLower(prompt), "capital of france") {
			responseText = "Paris"
		}

		// Build Triton V2 response
		// Note: ModelName should match what was requested (expectedModel)
		response := triton.InferResponse{
			ID:        tritonReq.ID,
			ModelName: expectedModel,
			Outputs: []triton.InferOutputTensor{
				{
					Name:     "text_output",
					Shape:    []int64{1},
					Datatype: "BYTES",
					Data:     []interface{}{responseText},
				},
			},
			Parameters: map[string]interface{}{
				"finish_reason": "stop",
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(handler)
}

// mockTritonErrorServer creates a mock Triton server that returns errors.
func mockTritonErrorServer(t *testing.T, statusCode int, errorMessage string) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": errorMessage})
	})

	return httptest.NewServer(handler)
}

// TestTritonBackendChatCompletions tests the Triton backend integration.
func TestTritonBackendChatCompletions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	modelName := "meta-llama/Llama-3.1-8B-Instruct"

	// Setup mock Triton server
	// TRT-LLM always uses "ensemble" as the model name in the Triton API
	mockBackend := mockTritonServer(t, "ensemble")
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

	// Seed a test routing policy with Triton backend type
	policy := &config.RoutingPolicy{
		PolicyID:       "test-triton-policy-1",
		OrganizationID: "*", // Global policy
		Model:          modelName,
		ExternalName:   "llama-3.1-8b",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-triton-backend-1",
				Weight:    100,
			},
		},
		BackendType:       "triton",
		Tokenizer:         "cl100k_base",
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}
	ctx := context.Background()
	if err := cache.StorePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to store policy: %v", err)
	}

	backendClient := routing.NewBackendClient(logger, 5*time.Second)

	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)

	// Configure handler to use mock Triton backend URI
	// Note: the handler will build /v2/models/{model}/infer path from this base
	handler.SetBackendURI("mock-triton-backend-1", mockBackend.URL)

	// Create router and register routes with middleware
	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Create a test request
	requestBody := public.OpenAIChatCompletionRequest{
		Model: modelName,
		Messages: []public.OpenAIMessage{
			{
				Role:    "user",
				Content: "What is the capital of France?",
			},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	// Record response
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Parse response as OpenAI format (not Triton - translation should have happened)
	var response struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	// Validate response fields
	if response.ID == "" {
		t.Error("expected response to have an ID")
	}

	if response.Object != "chat.completion" {
		t.Errorf("expected object to be 'chat.completion', got %s", response.Object)
	}

	if response.Model != modelName {
		t.Errorf("expected model to be %q, got %s", modelName, response.Model)
	}

	if len(response.Choices) != 1 {
		t.Errorf("expected 1 choice, got %d", len(response.Choices))
		return
	}

	// Validate the answer contains "Paris"
	answerText := response.Choices[0].Message.Content
	if !strings.Contains(answerText, "Paris") {
		t.Errorf("expected answer to contain 'Paris', got: %s", answerText)
	}

	if response.Choices[0].Message.Role != "assistant" {
		t.Errorf("expected message role to be 'assistant', got %s", response.Choices[0].Message.Role)
	}

	if response.Choices[0].FinishReason != "stop" {
		t.Errorf("expected finish_reason to be 'stop', got %s", response.Choices[0].FinishReason)
	}

	// Validate token counts (should be calculated by tiktoken)
	if response.Usage.PromptTokens == 0 {
		t.Error("expected prompt_tokens to be set (calculated by tiktoken)")
	}
	if response.Usage.CompletionTokens == 0 {
		t.Error("expected completion_tokens to be set (calculated by tiktoken)")
	}
	if response.Usage.TotalTokens != response.Usage.PromptTokens+response.Usage.CompletionTokens {
		t.Error("expected total_tokens to equal prompt_tokens + completion_tokens")
	}

	// Validate routing headers
	if backendID := w.Header().Get("X-Routing-Backend"); backendID == "" {
		t.Error("expected X-Routing-Backend header")
	}
	if decision := w.Header().Get("X-Routing-Decision"); decision == "" {
		t.Error("expected X-Routing-Decision header")
	}
	if backendType := w.Header().Get("X-Backend-Type"); backendType != "triton" {
		t.Errorf("expected X-Backend-Type header to be 'triton', got %q", backendType)
	}
}

// TestTritonBackendStreamingNotSupported tests that streaming is rejected for Triton backends.
func TestTritonBackendStreamingNotSupported(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	modelName := "meta-llama/Llama-3.1-8B-Instruct"

	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Seed a test routing policy with Triton backend type
	policy := &config.RoutingPolicy{
		PolicyID:       "test-triton-policy-streaming",
		OrganizationID: "*",
		Model:          modelName,
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-triton-backend-2",
				Weight:    100,
			},
		},
		BackendType:       "triton",
		Tokenizer:         "cl100k_base",
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}
	ctx := context.Background()
	if err := cache.StorePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to store policy: %v", err)
	}

	backendClient := routing.NewBackendClient(logger, 5*time.Second)

	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)

	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Create a streaming request
	requestBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
		"stream": true,
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Streaming should be rejected with a 400 error
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for streaming request, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Check that error message mentions streaming
	if !strings.Contains(w.Body.String(), "streaming") {
		t.Errorf("expected error message to mention streaming, got: %s", w.Body.String())
	}
}

// TestTritonBackendMissingTokenizer tests that tokenizer is required for Triton backends.
func TestTritonBackendMissingTokenizer(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	modelName := "test-model-no-tokenizer"

	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Seed a test routing policy with Triton backend but NO tokenizer
	policy := &config.RoutingPolicy{
		PolicyID:       "test-triton-policy-no-tokenizer",
		OrganizationID: "*",
		Model:          modelName,
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-triton-backend-3",
				Weight:    100,
			},
		},
		BackendType:       "triton",
		Tokenizer:         "", // Missing tokenizer!
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}
	ctx := context.Background()
	if err := cache.StorePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to store policy: %v", err)
	}

	backendClient := routing.NewBackendClient(logger, 5*time.Second)

	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil, nil, nil, "", "", 30*time.Second, 10*time.Second)

	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Create a request
	requestBody := map[string]interface{}{
		"model": modelName,
		"messages": []map[string]string{
			{"role": "user", "content": "Hello"},
		},
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail with validation error
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 for missing tokenizer, got %d. Body: %s", w.Code, w.Body.String())
	}

	// Check that error message mentions tokenizer
	if !strings.Contains(w.Body.String(), "tokenizer") {
		t.Errorf("expected error message to mention tokenizer, got: %s", w.Body.String())
	}
}

// TestTritonBackendErrorMapping tests that Triton errors are mapped to OpenAI format.
func TestTritonBackendErrorMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	testCases := []struct {
		name               string
		tritonStatus       int
		expectedHTTPStatus int
		expectedErrorType  string
	}{
		{
			name:               "service unavailable",
			tritonStatus:       http.StatusServiceUnavailable,
			expectedHTTPStatus: http.StatusServiceUnavailable,
			expectedErrorType:  "service_unavailable",
		},
		{
			name:               "bad request",
			tritonStatus:       http.StatusBadRequest,
			expectedHTTPStatus: http.StatusBadRequest,
			expectedErrorType:  "invalid_request_error",
		},
		{
			name:               "not found",
			tritonStatus:       http.StatusNotFound,
			expectedHTTPStatus: http.StatusNotFound,
			expectedErrorType:  "model_not_found",
		},
		{
			name:               "rate limited",
			tritonStatus:       http.StatusTooManyRequests,
			expectedHTTPStatus: http.StatusTooManyRequests,
			expectedErrorType:  "rate_limit_exceeded",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			modelName := "error-test-model"

			// Setup mock Triton server that returns errors
			mockBackend := mockTritonErrorServer(t, tc.tritonStatus, "Triton error: "+tc.name)
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

			policy := &config.RoutingPolicy{
				PolicyID:       "test-triton-error-" + tc.name,
				OrganizationID: "*",
				Model:          modelName,
				Backends: []config.BackendWeight{
					{
						BackendID: "mock-triton-error",
						Weight:    100,
					},
				},
				BackendType:       "triton",
				Tokenizer:         "cl100k_base",
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
			handler.SetBackendURI("mock-triton-error", mockBackend.URL)

			router := chi.NewRouter()
			tracer := otel.Tracer("test")
			router.Use(public.BodyBufferMiddleware(64 * 1024))
			router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
			handler.RegisterRoutes(router)

			requestBody := map[string]interface{}{
				"model": modelName,
				"messages": []map[string]string{
					{"role": "user", "content": "Hello"},
				},
			}

			jsonBody, _ := json.Marshal(requestBody)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "dev-test-key")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			// Validate HTTP status
			if w.Code != tc.expectedHTTPStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedHTTPStatus, w.Code, w.Body.String())
			}

			// Parse error response
			var errResp struct {
				Error struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to unmarshal error response: %v", err)
			}

			// Validate error type
			if errResp.Error.Type != tc.expectedErrorType {
				t.Errorf("expected error type %q, got %q", tc.expectedErrorType, errResp.Error.Type)
			}
		})
	}
}
