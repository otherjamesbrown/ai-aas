package public

import (
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// TestBuildBackendEndpoint_NotConfigured tests that buildBackendEndpoint
// returns an error when backend is not configured (aas-6yyiv)
func TestBuildBackendEndpoint_NotConfigured(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tracer := otel.Tracer("test")

	handler := &Handler{
		logger:          logger,
		defaultTimeout:  time.Second * 30,
		errorBuilder:    api.NewErrorBuilder(tracer),
		backendURIs:     nil,             // No test overrides
		backendRegistry: nil,             // No registry configured
	}

	// Should return error when backend not found
	_, err := handler.buildBackendEndpoint("non-existent-backend", "test-model", "")
	if err == nil {
		t.Fatal("expected error when backend not configured, got nil")
	}

	expectedErrMsg := `backend "non-existent-backend" not found in BACKEND_ENDPOINTS configuration`
	if err.Error() != expectedErrMsg {
		t.Errorf("expected error message %q, got %q", expectedErrMsg, err.Error())
	}
}

// TestBuildBackendEndpoint_WithTestOverride tests that buildBackendEndpoint
// succeeds when backend is configured via test override (aas-6yyiv)
func TestBuildBackendEndpoint_WithTestOverride(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tracer := otel.Tracer("test")

	handler := &Handler{
		logger:         logger,
		defaultTimeout: time.Second * 30,
		errorBuilder:   api.NewErrorBuilder(tracer),
		backendURIs: map[string]string{
			"test-backend": "http://localhost:8000/v1/completions",
		},
		backendRegistry: nil,
	}

	// Should succeed when backend is in test overrides
	backend, err := handler.buildBackendEndpoint("test-backend", "test-model", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if backend.ID != "test-backend" {
		t.Errorf("expected backend ID %q, got %q", "test-backend", backend.ID)
	}

	if backend.URI != "http://localhost:8000/v1/completions" {
		t.Errorf("expected URI %q, got %q", "http://localhost:8000/v1/completions", backend.URI)
	}

	if backend.ModelVariant != "test-model" {
		t.Errorf("expected model variant %q, got %q", "test-model", backend.ModelVariant)
	}

	if backend.Timeout != time.Second*30 {
		t.Errorf("expected timeout %v, got %v", time.Second*30, backend.Timeout)
	}
}

// TestBuildBackendEndpoint_WithRegistry tests that buildBackendEndpoint
// succeeds when backend is configured via registry (aas-6yyiv)
func TestBuildBackendEndpoint_WithRegistry(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tracer := otel.Tracer("test")

	// Create a backend registry with configured endpoint
	cfg := &config.Config{
		BackendEndpoints:      "registry-backend:http://vllm-service:8000/v1/completions",
		DefaultBackendTimeout: time.Second * 60,
	}
	registry := config.NewBackendRegistry(cfg)

	handler := &Handler{
		logger:          logger,
		defaultTimeout:  time.Second * 30,
		errorBuilder:    api.NewErrorBuilder(tracer),
		backendURIs:     nil,
		backendRegistry: registry,
	}

	// Should succeed when backend is in registry
	backend, err := handler.buildBackendEndpoint("registry-backend", "test-model", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if backend.ID != "registry-backend" {
		t.Errorf("expected backend ID %q, got %q", "registry-backend", backend.ID)
	}

	if backend.URI != "http://vllm-service:8000/v1/completions" {
		t.Errorf("expected URI %q, got %q", "http://vllm-service:8000/v1/completions", backend.URI)
	}

	if backend.Timeout != time.Second*60 {
		t.Errorf("expected timeout %v, got %v", time.Second*60, backend.Timeout)
	}
}

// TestBuildBackendEndpoint_TestOverrideTakesPrecedence tests that test overrides
// take precedence over registry configuration (aas-6yyiv)
func TestBuildBackendEndpoint_TestOverrideTakesPrecedence(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	tracer := otel.Tracer("test")

	// Create a backend registry with configured endpoint
	cfg := &config.Config{
		BackendEndpoints:      "backend:http://registry-uri:8000/v1/completions",
		DefaultBackendTimeout: time.Second * 60,
	}
	registry := config.NewBackendRegistry(cfg)

	handler := &Handler{
		logger:         logger,
		defaultTimeout: time.Second * 30,
		errorBuilder:   api.NewErrorBuilder(tracer),
		backendURIs: map[string]string{
			"backend": "http://test-override:9000/v1/completions",
		},
		backendRegistry: registry,
	}

	// Test override should take precedence over registry
	backend, err := handler.buildBackendEndpoint("backend", "test-model", "")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Should use test override URI, not registry URI
	if backend.URI != "http://test-override:9000/v1/completions" {
		t.Errorf("expected test override URI, got %q", backend.URI)
	}

	// Should use default timeout (test overrides don't specify timeout)
	if backend.Timeout != time.Second*30 {
		t.Errorf("expected default timeout %v, got %v", time.Second*30, backend.Timeout)
	}
}
