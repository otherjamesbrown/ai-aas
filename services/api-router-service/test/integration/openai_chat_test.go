// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//   These tests validate OpenAI-compatible chat completions endpoint functionality.
//
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

// mockOpenAIBackendServer creates a mock OpenAI-compatible backend HTTP server for testing.
func mockOpenAIBackendServer(t *testing.T) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/chat/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Parse OpenAI chat completion request
		var req public.OpenAIChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Extract the question from the last message
		var userMessage string
		for _, msg := range req.Messages {
			if msg.Role == "user" {
				userMessage = msg.Content
			}
		}

		// Generate mock response based on the question
		var responseText string
		if strings.Contains(strings.ToLower(userMessage), "capital of france") {
			responseText = "Paris"
		} else {
			responseText = "I don't know the answer to that question."
		}

		// Generate OpenAI-compatible response
		response := public.OpenAIChatCompletionResponse{
			ID:      "chatcmpl-test-123",
			Object:  "chat.completion",
			Created: time.Now().Unix(),
			Model:   req.Model,
			Choices: []public.OpenAIChoice{
				{
					Index: 0,
					Message: public.OpenAIMessage{
						Role:    "assistant",
						Content: responseText,
					},
					FinishReason: "stop",
				},
			},
			Usage: public.OpenAIUsage{
				PromptTokens:     len(userMessage) / 4,  // Rough estimate
				CompletionTokens: len(responseText) / 4, // Rough estimate
				TotalTokens:      (len(userMessage) + len(responseText)) / 4,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(handler)
}

// TestOpenAIChatCompletions tests the OpenAI-compatible chat completions endpoint.
func TestOpenAIChatCompletions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Setup mock OpenAI backend server
	mockBackend := mockOpenAIBackendServer(t)
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
		PolicyID:       "test-policy-openai-1",
		OrganizationID: "*", // Global policy
		Model:          "gpt-4o",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-openai-backend-1",
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

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// Configure handler to use mock OpenAI backend URI
	handler.SetBackendURI("mock-openai-backend-1", mockBackend.URL+"/v1/chat/completions")

	// Create router and register routes with middleware
	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Create a test request with the specified question
	requestBody := public.OpenAIChatCompletionRequest{
		Model: "gpt-4o",
		Messages: []public.OpenAIMessage{
			{
				Role:    "user",
				Content: "in one word, can you provide me the Capital of France",
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

	var response public.OpenAIChatCompletionResponse
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

	if response.Model != "gpt-4o" {
		t.Errorf("expected model to be 'gpt-4o', got %s", response.Model)
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

	// Validate usage information
	if response.Usage.PromptTokens == 0 {
		t.Error("expected prompt_tokens to be set")
	}
	if response.Usage.CompletionTokens == 0 {
		t.Error("expected completion_tokens to be set")
	}
	if response.Usage.TotalTokens == 0 {
		t.Error("expected total_tokens to be set")
	}

	// Validate routing headers
	if backendID := w.Header().Get("X-Routing-Backend"); backendID == "" {
		t.Error("expected X-Routing-Backend header")
	}
	if decision := w.Header().Get("X-Routing-Decision"); decision == "" {
		t.Error("expected X-Routing-Decision header")
	}
}

// TestOpenAIChatCompletionsValidation tests request validation for OpenAI chat endpoint.
func TestOpenAIChatCompletionsValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, _ := config.NewCache(":memory:")
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)

	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	router := chi.NewRouter()
	tracer := otel.Tracer("test")
	router.Use(public.BodyBufferMiddleware(64 * 1024))
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Test cases for validation
	testCases := []struct {
		name           string
		request        map[string]interface{}
		expectedStatus int
	}{
		{
			name: "missing model",
			request: map[string]interface{}{
				"messages": []map[string]string{
					{"role": "user", "content": "Hello"},
				},
			},
			expectedStatus: http.StatusBadRequest,
		},
		{
			name: "empty messages array",
			request: map[string]interface{}{
				"model":    "gpt-4o",
				"messages": []map[string]string{},
			},
			expectedStatus: http.StatusBadRequest,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tc.request)
			req := httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "dev-test-key")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.expectedStatus {
				t.Errorf("expected status %d, got %d. Body: %s", tc.expectedStatus, w.Code, w.Body.String())
			}
		})
	}
}
