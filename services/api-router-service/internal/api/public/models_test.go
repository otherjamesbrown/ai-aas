package public

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestHandleModels_FiltersByDeploymentStatus verifies that /v1/models only returns
// models with deployment_status == "ready", filtering out pending, deploying, failed, etc.
func TestHandleModels_FiltersByDeploymentStatus(t *testing.T) {
	// Helper function to create string pointer
	strPtr := func(s string) *string { return &s }

	tests := []struct {
		name           string
		adminModels    []AdminAPIModelResponse
		expectedModels []string // Model IDs that should appear in response
	}{
		{
			name: "only ready models appear",
			adminModels: []AdminAPIModelResponse{
				{
					Name:             "model-ready",
					ExternalName:     "gpt-ready",
					DeploymentStatus: strPtr("ready"),
				},
				{
					Name:             "model-deploying",
					DeploymentStatus: strPtr("deploying"),
				},
				{
					Name:             "model-pending",
					DeploymentStatus: strPtr("pending"),
				},
				{
					Name:             "model-failed",
					DeploymentStatus: strPtr("failed"),
				},
			},
			expectedModels: []string{"gpt-ready"},
		},
		{
			name: "nil deployment status is filtered out",
			adminModels: []AdminAPIModelResponse{
				{
					Name:             "model-no-deployment",
					DeploymentStatus: nil,
				},
				{
					Name:             "model-ready",
					ExternalName:     "gpt-ready",
					DeploymentStatus: strPtr("ready"),
				},
			},
			expectedModels: []string{"gpt-ready"},
		},
		{
			name: "disabled and terminated models filtered",
			adminModels: []AdminAPIModelResponse{
				{
					Name:             "model-disabled",
					DeploymentStatus: strPtr("disabled"),
				},
				{
					Name:             "model-terminated",
					DeploymentStatus: strPtr("terminated"),
				},
				{
					Name:             "model-ready",
					DeploymentStatus: strPtr("ready"),
				},
			},
			expectedModels: []string{"model-ready"},
		},
		{
			name: "multiple ready models",
			adminModels: []AdminAPIModelResponse{
				{
					Name:             "model1",
					ExternalName:     "gpt-1",
					DeploymentStatus: strPtr("ready"),
				},
				{
					Name:             "model2",
					ExternalName:     "gpt-2",
					DeploymentStatus: strPtr("ready"),
				},
				{
					Name:             "model3",
					DeploymentStatus: strPtr("deploying"),
				},
			},
			expectedModels: []string{"gpt-1", "gpt-2"},
		},
		{
			name: "uses internal name when external_name not set",
			adminModels: []AdminAPIModelResponse{
				{
					Name:             "internal-name",
					ExternalName:     "", // Empty external name
					DeploymentStatus: strPtr("ready"),
				},
			},
			expectedModels: []string{"internal-name"},
		},
		{
			name:           "empty list when no ready models",
			adminModels:    []AdminAPIModelResponse{},
			expectedModels: []string{},
		},
		{
			name: "all non-ready statuses filtered",
			adminModels: []AdminAPIModelResponse{
				{
					Name:             "model-pending",
					DeploymentStatus: strPtr("pending"),
				},
				{
					Name:             "model-deploying",
					DeploymentStatus: strPtr("deploying"),
				},
				{
					Name:             "model-failed",
					DeploymentStatus: strPtr("failed"),
				},
			},
			expectedModels: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test handler with mocked cache
			logger := zap.NewNop()
			handler := &Handler{
				logger:         logger,
				modelsCache:    tt.adminModels,
				modelsCacheTime: time.Now(),
				modelsCacheTTL: time.Hour, // Long TTL so cache is used
			}

			// Create test request
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			rec := httptest.NewRecorder()

			// Execute handler
			handler.HandleModels(rec, req)

			// Verify response
			assert.Equal(t, http.StatusOK, rec.Code)

			var response ModelsResponse
			err := json.NewDecoder(rec.Body).Decode(&response)
			require.NoError(t, err)

			assert.Equal(t, "list", response.Object)
			assert.Len(t, response.Data, len(tt.expectedModels))

			// Verify model IDs match expected
			actualIDs := make([]string, len(response.Data))
			for i, model := range response.Data {
				actualIDs[i] = model.ID
			}
			assert.ElementsMatch(t, tt.expectedModels, actualIDs)

			// Verify all returned models have correct format
			for _, model := range response.Data {
				assert.Equal(t, "model", model.Object)
				assert.Equal(t, "ai-aas", model.OwnedBy)
				assert.NotZero(t, model.Created)
			}
		})
	}
}

// TestHandleModels_FallbackOnError verifies that the handler returns an empty list
// when fetching from Admin API fails, rather than returning an error.
func TestHandleModels_FallbackOnError(t *testing.T) {
	logger := zap.NewNop()
	handler := &Handler{
		logger:            logger,
		adminAPIEndpoint:  "http://invalid-endpoint-that-does-not-exist",
		httpClient:        http.DefaultClient,
		modelsCacheTTL:    time.Hour,
		modelsCache:       nil, // No cache
		modelsCacheTime:   time.Time{}, // Zero time = expired
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	handler.HandleModels(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ModelsResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	assert.Equal(t, "list", response.Object)
	assert.Empty(t, response.Data) // Should return empty list on error
}

// TestHandleModels_CacheHit verifies that the cache is used when valid.
func TestHandleModels_CacheHit(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	logger := zap.NewNop()
	cachedModels := []AdminAPIModelResponse{
		{
			Name:             "cached-model",
			ExternalName:     "gpt-cached",
			DeploymentStatus: strPtr("ready"),
		},
	}

	handler := &Handler{
		logger:          logger,
		modelsCache:     cachedModels,
		modelsCacheTime: time.Now(),
		modelsCacheTTL:  time.Hour, // Cache is valid
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	handler.HandleModels(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	var response ModelsResponse
	err := json.NewDecoder(rec.Body).Decode(&response)
	require.NoError(t, err)

	require.Len(t, response.Data, 1)
	assert.Equal(t, "gpt-cached", response.Data[0].ID)
}

// TestFetchModelsFromAdminAPI tests the direct Admin API fetch function.
func TestFetchModelsFromAdminAPI(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("successful fetch", func(t *testing.T) {
		// Create mock Admin API server
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/v1/models", r.URL.Path)
			assert.Equal(t, "application/json", r.Header.Get("Accept"))

			models := []AdminAPIModelResponse{
				{
					Name:             "test-model",
					ExternalName:     "gpt-test",
					DeploymentStatus: strPtr("ready"),
				},
			}
			json.NewEncoder(w).Encode(models)
		}))
		defer mockServer.Close()

		logger := zap.NewNop()
		handler := &Handler{
			logger:           logger,
			adminAPIEndpoint: mockServer.URL,
			httpClient:       http.DefaultClient,
		}

		models, err := handler.fetchModelsFromAdminAPI(context.Background())
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "test-model", models[0].Name)
		assert.Equal(t, "gpt-test", models[0].ExternalName)
	})

	t.Run("no endpoint configured", func(t *testing.T) {
		logger := zap.NewNop()
		handler := &Handler{
			logger:           logger,
			adminAPIEndpoint: "",
		}

		_, err := handler.fetchModelsFromAdminAPI(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "admin API endpoint not configured")
	})

	t.Run("non-200 status code", func(t *testing.T) {
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}))
		defer mockServer.Close()

		logger := zap.NewNop()
		handler := &Handler{
			logger:           logger,
			adminAPIEndpoint: mockServer.URL,
			httpClient:       http.DefaultClient,
		}

		_, err := handler.fetchModelsFromAdminAPI(context.Background())
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "500")
	})
}

// TestGetModelsWithCache verifies the caching logic.
func TestGetModelsWithCache(t *testing.T) {
	strPtr := func(s string) *string { return &s }

	t.Run("cache hit - valid cache", func(t *testing.T) {
		logger := zap.NewNop()
		cachedModels := []AdminAPIModelResponse{
			{Name: "cached", DeploymentStatus: strPtr("ready")},
		}

		handler := &Handler{
			logger:          logger,
			modelsCache:     cachedModels,
			modelsCacheTime: time.Now(),
			modelsCacheTTL:  time.Hour,
		}

		models, err := handler.getModelsWithCache(context.Background())
		require.NoError(t, err)
		assert.Equal(t, cachedModels, models)
	})

	t.Run("cache miss - expired cache", func(t *testing.T) {
		// Create mock server
		mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fresh := []AdminAPIModelResponse{
				{Name: "fresh", DeploymentStatus: strPtr("ready")},
			}
			json.NewEncoder(w).Encode(fresh)
		}))
		defer mockServer.Close()

		logger := zap.NewNop()
		handler := &Handler{
			logger:           logger,
			adminAPIEndpoint: mockServer.URL,
			httpClient:       http.DefaultClient,
			modelsCache:      []AdminAPIModelResponse{{Name: "stale"}},
			modelsCacheTime:  time.Now().Add(-2 * time.Hour), // Expired
			modelsCacheTTL:   time.Hour,
		}

		models, err := handler.getModelsWithCache(context.Background())
		require.NoError(t, err)
		require.Len(t, models, 1)
		assert.Equal(t, "fresh", models[0].Name)
	})

	t.Run("returns stale cache on fetch error", func(t *testing.T) {
		logger := zap.NewNop()
		staleModels := []AdminAPIModelResponse{
			{Name: "stale", DeploymentStatus: strPtr("ready")},
		}

		handler := &Handler{
			logger:           logger,
			adminAPIEndpoint: "http://invalid",
			httpClient:       http.DefaultClient,
			modelsCache:      staleModels,
			modelsCacheTime:  time.Now().Add(-2 * time.Hour), // Expired
			modelsCacheTTL:   time.Hour,
		}

		models, err := handler.getModelsWithCache(context.Background())
		require.NoError(t, err) // Should not error, returns stale cache
		assert.Equal(t, staleModels, models)
	})
}
