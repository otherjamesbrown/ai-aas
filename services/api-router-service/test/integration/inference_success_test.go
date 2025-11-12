// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//   These tests validate end-to-end inference routing functionality, including
//   authentication, request forwarding, and response handling.
//
// Key Responsibilities:
//   - Test happy-path inference request flow
//   - Validate authentication and authorization
//   - Verify backend routing and response handling
//   - Test error scenarios
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
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
)

// mockBackendServer creates a mock backend HTTP server for testing.
func mockBackendServer(t *testing.T) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if r.URL.Path != "/v1/completions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Parse request
		var req struct {
			Prompt     string                 `json:"prompt"`
			Parameters map[string]interface{} `json:"parameters,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		// Generate mock response
		response := map[string]interface{}{
			"text":        req.Prompt + " [mock response]",
			"tokens_used": len(req.Prompt) + 10,
			"metadata":    map[string]interface{}{"model": "mock-model"},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(handler)
}

// TestInferenceSuccess tests the happy-path inference request flow.
func TestInferenceSuccess(t *testing.T) {
	// Setup mock backend server
	mockBackend := mockBackendServer(t)
	defer mockBackend.Close()

	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)

	// Seed a test routing policy with the mock backend URI
	policy := &config.RoutingPolicy{
		PolicyID:       "test-policy-1",
		OrganizationID: "*", // Global policy
		Model:          "gpt-4o",
		Backends: []config.BackendWeight{
			{
				BackendID: "mock-backend-1",
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

	// Configure handler to use mock backend URI
	handler.SetBackendURI("mock-backend-1", mockBackend.URL+"/v1/completions")

	// Create router and register routes
	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Create a test request
	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"model":      "gpt-4o",
		"payload":    "Hello, world!",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
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

	var response public.InferenceResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	// Validate response fields
	if response.RequestID != requestBody["request_id"] {
		t.Errorf("expected request_id %s, got %s", requestBody["request_id"], response.RequestID)
	}

	if response.Output == nil {
		t.Error("expected output field in response")
	} else {
		text, ok := response.Output["text"].(string)
		if !ok || text == "" {
			t.Error("expected output.text to be a non-empty string")
		}
	}

	if response.Usage == nil {
		t.Error("expected usage field in response")
	} else {
		if response.Usage.TokensInput == 0 {
			t.Error("expected tokens_input to be set")
		}
		if response.Usage.TokensOutput == 0 {
			t.Error("expected tokens_output to be set")
		}
		if response.Usage.LatencyMS == 0 {
			t.Error("expected latency_ms to be set")
		}
	}

	// Validate routing headers
	if backendID := w.Header().Get("X-Routing-Backend"); backendID == "" {
		t.Error("expected X-Routing-Backend header")
	}
	if decision := w.Header().Get("X-Routing-Decision"); decision == "" {
		t.Error("expected X-Routing-Decision header")
	}
}

// TestInferenceAuthFailure tests authentication failure scenarios.
func TestInferenceAuthFailure(t *testing.T) {
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, _ := config.NewCache(":memory:")
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	// Create a minimal config for backend registry
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Test without API key
	requestBody := map[string]interface{}{
		"request_id": "550e8400-e29b-41d4-a716-446655440001",
		"model":      "gpt-4o",
		"payload":    "Hello",
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	// No X-API-Key header

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d. Body: %s", w.Code, w.Body.String())
	}
}

// TestInferenceValidationError tests request validation errors.
func TestInferenceValidationError(t *testing.T) {
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, _ := config.NewCache(":memory:")
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)
	backendClient := routing.NewBackendClient(logger, 5*time.Second)
	
	// Create a minimal config for backend registry
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	router := chi.NewRouter()
	handler.RegisterRoutes(router)

	// Test with missing required fields
	testCases := []struct {
		name    string
		request map[string]interface{}
	}{
		{
			name: "missing request_id",
			request: map[string]interface{}{
				"model":   "gpt-4o",
				"payload": "Hello",
			},
		},
		{
			name: "missing model",
			request: map[string]interface{}{
				"request_id": "550e8400-e29b-41d4-a716-446655440002",
				"payload":    "Hello",
			},
		},
		{
			name: "missing payload",
			request: map[string]interface{}{
				"request_id": "550e8400-e29b-41d4-a716-446655440003",
				"model":      "gpt-4o",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			jsonBody, _ := json.Marshal(tc.request)
			req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Key", "dev-test-key")

			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusBadRequest {
				t.Errorf("expected status 400, got %d. Body: %s", w.Code, w.Body.String())
			}
		})
	}
}

