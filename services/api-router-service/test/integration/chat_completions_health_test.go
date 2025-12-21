package integration

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/auth"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// TestChatCompletionsHealthEndpoint verifies that the /v1/chat/completions/health endpoint
// works without authentication (for guidellm validation).
func TestChatCompletionsHealthEndpoint(t *testing.T) {
	// Setup minimal handler (no auth required for health endpoint)
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 0)
	backendRegistry := config.NewBackendRegistry(&config.Config{})

	publicHandler := public.NewHandler(
		logger,
		authenticator,
		nil, // config loader not needed for health endpoint
		nil, // backend client not needed for health endpoint
		backendRegistry,
		nil, // routing engine not needed for health endpoint
		nil, // routing metrics not needed for health endpoint
		nil, // token metrics not needed for health endpoint
		nil, // usage hook not needed for health endpoint
		"",  // admin API endpoint not needed for health endpoint
		"",  // admin API key not needed for health endpoint
	)

	// Create router and register health endpoint
	router := chi.NewRouter()
	router.Get("/v1/chat/completions/health", publicHandler.HandleChatCompletionsHealth)

	// Create test server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Test that health endpoint is accessible without authentication
	req, err := http.NewRequest("GET", ts.URL+"/v1/chat/completions/health", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// Explicitly do NOT add authentication header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Should return 200 OK even without authentication
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", resp.StatusCode)
	}

	// Verify response body
	var response map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify expected fields
	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", response["status"])
	}
	if response["service"] != "api-router-service" {
		t.Errorf("expected service 'api-router-service', got %v", response["service"])
	}
	if response["endpoint"] != "/v1/chat/completions" {
		t.Errorf("expected endpoint '/v1/chat/completions', got %v", response["endpoint"])
	}
	if response["timestamp"] == nil {
		t.Error("expected timestamp field to be present")
	}
}

// TestChatCompletionsHealthNoAuthRequired verifies that the health endpoint
// does not require authentication, unlike the actual chat completions endpoint.
func TestChatCompletionsHealthNoAuthRequired(t *testing.T) {
	logger := zap.NewNop()
	authenticator := auth.NewAuthenticator(logger, "", 0)
	backendRegistry := config.NewBackendRegistry(&config.Config{})

	publicHandler := public.NewHandler(
		logger,
		authenticator,
		nil,
		nil,
		backendRegistry,
		nil,
		nil,
		nil, // token metrics not needed for health endpoint
		nil,
		"", // admin API endpoint not needed for health endpoint
		"", // admin API key not needed for health endpoint
	)

	// Create router similar to main.go architecture
	router := chi.NewRouter()

	// Register unauthenticated health endpoint on main router
	router.Get("/v1/chat/completions/health", publicHandler.HandleChatCompletionsHealth)

	// Create sub-router with auth middleware
	appRouter := chi.NewRouter()
	appRouter.Use(public.AuthContextMiddleware(authenticator, logger, nil))
	publicHandler.RegisterRoutes(appRouter)

	// Mount authenticated routes
	router.Mount("/", appRouter)

	// Create test server
	ts := httptest.NewServer(router)
	defer ts.Close()

	// Test authenticated endpoint requires auth
	req, err := http.NewRequest("POST", ts.URL+"/v1/chat/completions", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// No auth header
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Should return 401 Unauthorized
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized for chat completions without auth, got %d", resp.StatusCode)
	}

	// Test health endpoint does NOT require auth
	req, err = http.NewRequest("GET", ts.URL+"/v1/chat/completions/health", nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	// No auth header
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("failed to send request: %v", err)
	}
	defer resp.Body.Close()

	// Should return 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200 OK for health endpoint without auth, got %d", resp.StatusCode)
	}
}
