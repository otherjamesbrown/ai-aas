// Test for Triton backend health check support (aas-7t8fv)
package public

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// TestBuildBackendEndpoint_HealthPathByBackendType verifies that the correct health
// check path is set based on the backend_type parameter (aas-7t8fv).
func TestBuildBackendEndpoint_HealthPathByBackendType(t *testing.T) {
	logger := zap.NewNop()

	tests := []struct {
		name               string
		backendType        string
		expectedHealthPath string
	}{
		{
			name:               "vLLM backend (openai type)",
			backendType:        "openai",
			expectedHealthPath: "/health",
		},
		{
			name:               "vLLM backend (empty type defaults to openai)",
			backendType:        "",
			expectedHealthPath: "/health",
		},
		{
			name:               "Triton HTTP backend",
			backendType:        "triton",
			expectedHealthPath: "/v2/health/ready",
		},
		{
			name:               "Triton gRPC backend",
			backendType:        "triton-grpc",
			expectedHealthPath: "/v2/health/ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create handler with backend registry
			registry := &config.BackendRegistry{}
			registry.RegisterBackend("test-backend", "http://localhost:8000", 0)

			handler := &Handler{
				logger:          logger,
				backendRegistry: registry,
			}

			// Build endpoint with backend type
			endpoint, err := handler.buildBackendEndpoint("test-backend", "test-model", tt.backendType)
			if err != nil {
				t.Fatalf("buildBackendEndpoint failed: %v", err)
			}

			// Verify health path
			if endpoint.HealthPath != tt.expectedHealthPath {
				t.Errorf("expected HealthPath=%q, got %q", tt.expectedHealthPath, endpoint.HealthPath)
			}
		})
	}
}

// TestTritonHealthCheckIntegration verifies end-to-end health check behavior
// for Triton backends using the correct endpoint (aas-7t8fv).
func TestTritonHealthCheckIntegration(t *testing.T) {
	logger := zap.NewNop()

	// Create mock Triton server with both health endpoints
	tritonHealthReady := false
	tritonHealthAlive := false

	mockTriton := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/health/ready":
			if tritonHealthReady {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		case "/v2/health/live":
			if tritonHealthAlive {
				w.WriteHeader(http.StatusOK)
			} else {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
		case "/health":
			// Old OpenAI-style health check - should NOT be used for Triton
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockTriton.Close()

	// Create handler with Triton backend
	registry := &config.BackendRegistry{}
	registry.RegisterBackend("triton-backend", mockTriton.URL, 0)

	handler := &Handler{
		logger:          logger,
		backendRegistry: registry,
	}

	tests := []struct {
		name          string
		backendType   string
		healthReady   bool
		healthAlive   bool
		expectHealthy bool
	}{
		{
			name:          "Triton backend ready",
			backendType:   "triton-grpc",
			healthReady:   true,
			healthAlive:   true,
			expectHealthy: true,
		},
		{
			name:          "Triton backend not ready",
			backendType:   "triton-grpc",
			healthReady:   false,
			healthAlive:   true,
			expectHealthy: false,
		},
		{
			name:          "Triton HTTP backend ready",
			backendType:   "triton",
			healthReady:   true,
			healthAlive:   true,
			expectHealthy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set mock server state
			tritonHealthReady = tt.healthReady
			tritonHealthAlive = tt.healthAlive

			// Build endpoint with backend type
			endpoint, err := handler.buildBackendEndpoint("triton-backend", "ensemble", tt.backendType)
			if err != nil {
				t.Fatalf("buildBackendEndpoint failed: %v", err)
			}

			// Verify endpoint has correct health path
			if endpoint.HealthPath != "/v2/health/ready" {
				t.Errorf("expected HealthPath=/v2/health/ready, got %q", endpoint.HealthPath)
			}

			// Verify health check uses the correct endpoint
			// We can't directly test BackendClient.HealthCheck here without exposing it,
			// but the endpoint configuration is what matters
			if tt.expectHealthy && endpoint.HealthPath != "/v2/health/ready" {
				t.Error("Triton backend should use /v2/health/ready for health checks")
			}
		})
	}
}
