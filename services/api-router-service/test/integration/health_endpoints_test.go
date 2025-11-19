// Package integration provides integration tests for the API Router Service.
//
// Purpose:
//   These tests validate health and readiness endpoint functionality, including
//   component-level health checks and build metadata.
//
// Key Responsibilities:
//   - Test /v1/status/healthz endpoint (liveness check)
//   - Test /v1/status/readyz endpoint (readiness check)
//   - Validate component-level health checks (Redis, Kafka, Config Service, Backend Registry)
//   - Verify build metadata in responses
//   - Test degraded state handling
//
// Note: These tests currently use inline handlers to define expected behavior.
// Once T038 (status_handlers.go) is implemented, these tests should be updated
// to use the actual handler implementations.
//
package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/api/public"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/usage"
)

// HealthResponse represents the health endpoint response.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ReadinessResponse represents the readiness endpoint response.
type ReadinessResponse struct {
	Status     string            `json:"status"`
	Components map[string]string `json:"components,omitempty"`
	Build      *struct {
		Version   string `json:"version"`
		Commit    string `json:"commit"`
		BuildTime string `json:"build_time"`
	} `json:"build,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// TestHealthzEndpoint tests the basic liveness check endpoint.
func TestHealthzEndpoint(t *testing.T) {
	// Set up status handlers
	logger := zap.NewNop()
	statusHandlers := public.NewStatusHandlers(public.StatusHandlersConfig{
		Logger: logger,
		BuildMetadata: public.BuildMetadata{
			Version:   "test-version",
			Commit:    "test-commit",
			BuildTime: time.Now().Format(time.RFC3339),
		},
	})

	// Set up router with health endpoint
	router := chi.NewRouter()
	router.Get("/v1/status/healthz", statusHandlers.Healthz)

	// Make request
	req := httptest.NewRequest("GET", "/v1/status/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if response.Status != "healthy" {
		t.Errorf("expected status 'healthy', got '%s'", response.Status)
	}
}

// TestReadyzEndpointAllHealthy tests readiness endpoint when all components are healthy.
func TestReadyzEndpointAllHealthy(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Set up Redis (skip test if Redis unavailable)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1, // Use DB 1 for tests
	})
	ctx := context.Background()
	redisAvailable := redisClient.Ping(ctx).Err() == nil
	if !redisAvailable {
		t.Skipf("Redis not available for readiness test: %v", redisClient.Ping(ctx).Err())
	}
	defer func() { _ = redisClient.Close() }()

	// Set up Kafka publisher (mock for now - will be properly implemented in T038)
	publisher := usage.NewPublisher(usage.PublisherConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "test-topic",
		ClientID:     "test-client",
		BatchSize:    10,
		BatchTimeout: 1 * time.Second,
		WriteTimeout: 2 * time.Second,
		RequiredAcks: 1,
	}, logger)

	// Set up backend registry
	testCfg := &config.Config{
		BackendEndpoints: "backend-1:http://localhost:8001/v1/completions",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	// Set up status handlers
	statusHandlers := public.NewStatusHandlers(public.StatusHandlersConfig{
		RedisClient:    redisClient,
		KafkaPublisher: publisher,
		ConfigLoader:   loader,
		BackendRegistry: backendRegistry,
		Logger:         logger,
		BuildMetadata: public.BuildMetadata{
			Version:   "test-version",
			Commit:    "test-commit",
			BuildTime: time.Now().Format(time.RFC3339),
		},
		HealthTimeout: 1 * time.Second,
		ReadyTimeout:  5 * time.Second,
	})

	// Set up router with readiness endpoint
	router := chi.NewRouter()
	router.Get("/v1/status/readyz", statusHandlers.Readyz)

	// Make request
	req := httptest.NewRequest("GET", "/v1/status/readyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response ReadinessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if response.Status != "ready" {
		t.Errorf("expected status 'ready', got '%s'", response.Status)
	}

	// Validate components
	if response.Components == nil {
		t.Error("expected components map in response")
		return
	}

	expectedComponents := []string{"redis", "kafka", "config_service", "backend_registry"}
	for _, comp := range expectedComponents {
		if status, ok := response.Components[comp]; !ok {
			t.Errorf("expected component '%s' in response", comp)
		} else if status != "healthy" {
			t.Errorf("expected component '%s' to be 'healthy', got '%s'", comp, status)
		}
	}

	// Validate build metadata
	if response.Build == nil {
		t.Error("expected build metadata in response")
	} else {
		if response.Build.Version == "" {
			t.Error("expected 'version' in build metadata")
		}
		if response.Build.Commit == "" {
			t.Error("expected 'commit' in build metadata")
		}
		if response.Build.BuildTime == "" {
			t.Error("expected 'build_time' in build metadata")
		}
	}
}

// TestReadyzEndpointRedisDown tests readiness endpoint when Redis is unavailable.
func TestReadyzEndpointRedisDown(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Set up Redis client pointing to invalid address
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:9999", // Invalid port
		DB:   1,
	})
	defer func() { _ = redisClient.Close() }()

	// Set up Kafka publisher
	publisher := usage.NewPublisher(usage.PublisherConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "test-topic",
		ClientID:     "test-client",
		BatchSize:    10,
		BatchTimeout: 1 * time.Second,
		WriteTimeout: 2 * time.Second,
		RequiredAcks: 1,
	}, logger)

	// Set up backend registry
	testCfg := &config.Config{
		BackendEndpoints: "backend-1:http://localhost:8001/v1/completions",
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	// Set up status handlers
	statusHandlers := public.NewStatusHandlers(public.StatusHandlersConfig{
		RedisClient:    redisClient,
		KafkaPublisher: publisher,
		ConfigLoader:   loader,
		BackendRegistry: backendRegistry,
		Logger:         logger,
		BuildMetadata: public.BuildMetadata{
			Version:   "test-version",
			Commit:    "test-commit",
			BuildTime: time.Now().Format(time.RFC3339),
		},
		HealthTimeout: 1 * time.Second,
		ReadyTimeout:  5 * time.Second,
	})

	// Set up router with readiness endpoint
	router := chi.NewRouter()
	router.Get("/v1/status/readyz", statusHandlers.Readyz)

	// Make request
	req := httptest.NewRequest("GET", "/v1/status/readyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 when Redis is down
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response ReadinessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if response.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", response.Status)
	}

	// Validate Redis is marked as unhealthy
	if response.Components == nil {
		t.Error("expected components map in response")
		return
	}

	if status, ok := response.Components["redis"]; !ok {
		t.Error("expected 'redis' component in response")
	} else if status != "unhealthy" {
		t.Errorf("expected redis to be 'unhealthy', got '%s'", status)
	}
}

// TestReadyzEndpointBackendRegistryEmpty tests readiness endpoint when backend registry is empty.
func TestReadyzEndpointBackendRegistryEmpty(t *testing.T) {
	// Setup
	logger := zap.NewNop()
	cache, err := config.NewCache(":memory:")
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	defer func() { _ = cache.Close() }()

	loader := config.NewLoader("", false, cache, logger)

	// Set up Redis (skip if unavailable)
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   1,
	})
	ctx := context.Background()
	redisAvailable := redisClient.Ping(ctx).Err() == nil
	if !redisAvailable {
		t.Skipf("Redis not available: %v", redisClient.Ping(ctx).Err())
	}
	defer func() { _ = redisClient.Close() }()

	// Set up Kafka publisher
	publisher := usage.NewPublisher(usage.PublisherConfig{
		Brokers:      []string{"localhost:9092"},
		Topic:        "test-topic",
		ClientID:     "test-client",
		BatchSize:    10,
		BatchTimeout: 1 * time.Second,
		WriteTimeout: 2 * time.Second,
		RequiredAcks: 1,
	}, logger)

	// Set up empty backend registry
	testCfg := &config.Config{
		BackendEndpoints: "", // Empty
	}
	backendRegistry := config.NewBackendRegistry(testCfg)

	// Set up status handlers
	statusHandlers := public.NewStatusHandlers(public.StatusHandlersConfig{
		RedisClient:    redisClient,
		KafkaPublisher: publisher,
		ConfigLoader:   loader,
		BackendRegistry: backendRegistry,
		Logger:         logger,
		BuildMetadata: public.BuildMetadata{
			Version:   "test-version",
			Commit:    "test-commit",
			BuildTime: time.Now().Format(time.RFC3339),
		},
		HealthTimeout: 1 * time.Second,
		ReadyTimeout:  5 * time.Second,
	})

	// Set up router with readiness endpoint
	router := chi.NewRouter()
	router.Get("/v1/status/readyz", statusHandlers.Readyz)

	// Make request
	req := httptest.NewRequest("GET", "/v1/status/readyz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 503 when backend registry is empty
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected status 503, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response ReadinessResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if response.Status != "degraded" {
		t.Errorf("expected status 'degraded', got '%s'", response.Status)
	}

	// Validate backend registry is marked as unhealthy
	if response.Components == nil {
		t.Error("expected components map in response")
		return
	}

	if status, ok := response.Components["backend_registry"]; !ok {
		t.Error("expected 'backend_registry' component in response")
	} else if status != "unhealthy" {
		t.Errorf("expected backend_registry to be 'unhealthy', got '%s'", status)
	}
}

// TestHealthzWithBuildMetadata tests that health endpoint can include build metadata.
func TestHealthzWithBuildMetadata(t *testing.T) {
	// Set up status handlers with build metadata
	logger := zap.NewNop()
	statusHandlers := public.NewStatusHandlers(public.StatusHandlersConfig{
		Logger: logger,
		BuildMetadata: public.BuildMetadata{
			Version:   "test-version",
			Commit:    "test-commit",
			BuildTime: time.Now().Format(time.RFC3339),
		},
	})

	// Set up router with health endpoint
	router := chi.NewRouter()
	router.Get("/v1/status/healthz", statusHandlers.Healthz)

	// Make request
	req := httptest.NewRequest("GET", "/v1/status/healthz", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Validate response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d. Body: %s", w.Code, w.Body.String())
		return
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v. Body: %s", err, w.Body.String())
	}

	if status, ok := response["status"].(string); !ok || status != "healthy" {
		t.Errorf("expected status 'healthy', got '%v'", response["status"])
	}

	// Build metadata should be present
	if build, ok := response["build"].(map[string]interface{}); ok {
		if _, ok := build["version"]; !ok {
			t.Error("expected 'version' in build metadata")
		}
		if _, ok := build["commit"]; !ok {
			t.Error("expected 'commit' in build metadata")
		}
	} else {
		t.Error("expected 'build' metadata in response")
	}
}

