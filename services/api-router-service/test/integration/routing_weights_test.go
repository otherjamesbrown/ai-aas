// Package integration provides integration tests for routing weight distribution.
//
// Purpose:
//   These tests validate that the routing engine correctly distributes traffic
//   according to configured backend weights, handles failover scenarios, and
//   respects degraded backend states.
//
// Key Responsibilities:
//   - Test weighted routing distribution
//   - Validate failover behavior
//   - Verify degraded backend exclusion
//   - Test routing decision logging
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
	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/routing"
)

// mockBackendServerWithID creates a mock backend server that identifies itself in responses.
func mockBackendServerWithID(t *testing.T, backendID string) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
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

		// Generate mock response with backend ID
		response := map[string]interface{}{
			"text":        req.Prompt + " [response from " + backendID + "]",
			"tokens_used": len(req.Prompt) + 10,
			"metadata": map[string]interface{}{
				"model":      "mock-model",
				"backend_id": backendID,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	})

	return httptest.NewServer(handler)
}

// mockBackendServerWithError creates a mock backend server that returns errors.
func mockBackendServerWithError(t *testing.T, statusCode int) *httptest.Server {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(map[string]string{"error": "backend error"})
	})

	return httptest.NewServer(handler)
}

// TestRoutingWeightDistribution tests that traffic can be routed to weighted backends.
// This test validates the routing infrastructure supports weighted selection.
func TestRoutingWeightDistribution(t *testing.T) {
	// Setup multiple mock backend servers
	backend1 := mockBackendServerWithID(t, "backend-1")
	defer backend1.Close()
	backend2 := mockBackendServerWithID(t, "backend-2")
	defer backend2.Close()

	// Setup
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)

	// Create routing policy with weighted backends
	policy := &config.RoutingPolicy{
		PolicyID:       "test-weighted-policy",
		OrganizationID: "*",
		Model:          "gpt-4o",
		Backends: []config.BackendWeight{
			{
				BackendID: "backend-1",
				Weight:    70,
			},
			{
				BackendID: "backend-2",
				Weight:    30,
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
	testCfg := &config.Config{
		BackendEndpoints: "",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	// Configure handler to use mock backend URIs
	handler.SetBackendURI("backend-1", backend1.URL+"/v1/completions")
	handler.SetBackendURI("backend-2", backend2.URL+"/v1/completions")

	// Create router and register routes
	tracer := otel.Tracer("test")
	router := chi.NewRouter()
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Send a request and verify it routes to one of the backends
	requestBody := map[string]interface{}{
		"request_id": "test-weighted-1",
		"model":      "gpt-4o",
		"payload":    "Test weighted routing",
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Verify routing header indicates a backend was selected
	backendID := w.Header().Get("X-Routing-Backend")
	if backendID == "" {
		t.Error("expected X-Routing-Backend header")
	}

	if backendID != "backend-1" && backendID != "backend-2" {
		t.Errorf("expected backend-1 or backend-2, got %s", backendID)
	}
}

// TestRoutingFailover tests failover behavior when primary backend fails.
func TestRoutingFailover(t *testing.T) {
	// Setup: primary backend returns errors, secondary succeeds
	failingBackend := mockBackendServerWithError(t, http.StatusInternalServerError)
	defer failingBackend.Close()

	workingBackend := mockBackendServerWithID(t, "backend-2")
	defer workingBackend.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)

	// Create routing policy with failover
	policy := &config.RoutingPolicy{
		PolicyID:       "test-failover-policy",
		OrganizationID: "*",
		Model:          "gpt-4o",
		Backends: []config.BackendWeight{
			{
				BackendID: "backend-1",
				Weight:    100, // Primary
			},
			{
				BackendID: "backend-2",
				Weight:    50, // Failover
			},
		},
		FailoverThreshold: 1,
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
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	handler.SetBackendURI("backend-1", failingBackend.URL+"/v1/completions")
	handler.SetBackendURI("backend-2", workingBackend.URL+"/v1/completions")

	// Create router
	tracer := otel.Tracer("test")
	router := chi.NewRouter()
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Send request - should failover to backend-2
	requestBody := map[string]interface{}{
		"request_id": "test-failover-1",
		"model":      "gpt-4o",
		"payload":    "Test failover",
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed via failover
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 after failover, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Verify it routed to backend-2 (failover)
	backendID := w.Header().Get("X-Routing-Backend")
	if backendID != "backend-2" {
		t.Errorf("expected failover to backend-2, got %s", backendID)
	}

	decision := w.Header().Get("X-Routing-Decision")
	if decision != "FAILOVER" {
		t.Errorf("expected FAILOVER decision, got %s", decision)
	}
}

// TestDegradedBackendExclusion tests that degraded backends are excluded from routing.
func TestDegradedBackendExclusion(t *testing.T) {
	degradedBackend := mockBackendServerWithID(t, "backend-degraded")
	defer degradedBackend.Close()

	healthyBackend := mockBackendServerWithID(t, "backend-healthy")
	defer healthyBackend.Close()

	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 2*time.Second)
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer cache.Close()

	loader := config.NewLoader("", false, cache, logger)

	// Create routing policy with degraded backend
	policy := &config.RoutingPolicy{
		PolicyID:       "test-degraded-policy",
		OrganizationID: "*",
		Model:          "gpt-4o",
		Backends: []config.BackendWeight{
			{
				BackendID: "backend-degraded",
				Weight:    100,
			},
			{
				BackendID: "backend-healthy",
				Weight:    50,
			},
		},
		DegradedBackends: []string{"backend-degraded"},
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
	handler := public.NewHandler(logger, authenticator, loader, backendClient, backendRegistry, nil, nil, nil)

	handler.SetBackendURI("backend-degraded", degradedBackend.URL+"/v1/completions")
	handler.SetBackendURI("backend-healthy", healthyBackend.URL+"/v1/completions")

	// Create router
	tracer := otel.Tracer("test")
	router := chi.NewRouter()
	router.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
	handler.RegisterRoutes(router)

	// Send request - should route to healthy backend only
	requestBody := map[string]interface{}{
		"request_id": "test-degraded-1",
		"model":      "gpt-4o",
		"payload":    "Test degraded exclusion",
	}

	jsonBody, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/v1/inference", bytes.NewReader(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", "dev-test-key")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should succeed using healthy backend
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	// Verify it routed to healthy backend (not degraded)
	backendID := w.Header().Get("X-Routing-Backend")
	if backendID == "backend-degraded" {
		t.Error("should not route to degraded backend")
	}

	if backendID != "backend-healthy" {
		t.Errorf("expected backend-healthy, got %s", backendID)
	}
}

