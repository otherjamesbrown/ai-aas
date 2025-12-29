package routing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

func TestNewEngine(t *testing.T) {
	logger := zaptest.NewLogger(t)
	healthMonitor := NewHealthMonitor(NewBackendClient(logger, 5*time.Second), logger, 10*time.Second)
	backendRegistry := &config.BackendRegistry{}
	timeout := 5 * time.Second

	engine := NewEngine(healthMonitor, backendRegistry, timeout, logger)

	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}

	if engine.healthMonitor != healthMonitor {
		t.Error("healthMonitor not set correctly")
	}

	if engine.backendRegistry != backendRegistry {
		t.Error("backendRegistry not set correctly")
	}

	if engine.defaultTimeout != timeout {
		t.Errorf("defaultTimeout = %v, want %v", engine.defaultTimeout, timeout)
	}

	if engine.logger == nil {
		t.Error("logger is nil")
	}

	if engine.decisions == nil {
		t.Error("decisions slice is nil")
	}
}

func TestEngine_SetModelRegistry(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewEngine(nil, nil, 5*time.Second, logger)

	registry := &Registry{}
	engine.SetModelRegistry(registry)

	if engine.modelRegistry != registry {
		t.Error("modelRegistry not set correctly")
	}
}

func TestEngine_SelectBackend(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name             string
		policy           *config.RoutingPolicy
		setupBackends    func() *config.BackendRegistry
		wantErr          bool
		errContains      string
		wantBackendID    string
		wantDecisionType string
	}{
		{
			name:        "nil policy",
			policy:      nil,
			wantErr:     true,
			errContains: "no backends configured",
		},
		{
			name: "empty backends in policy",
			policy: &config.RoutingPolicy{
				PolicyID:       "test-policy",
				OrganizationID: "org-1",
				Model:          "test-model",
				Backends:       []config.BackendWeight{},
			},
			wantErr:     true,
			errContains: "no backends configured",
		},
		{
			name: "single backend selection",
			policy: &config.RoutingPolicy{
				PolicyID:       "test-policy",
				OrganizationID: "org-1",
				Model:          "test-model",
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 100},
				},
			},
			setupBackends: func() *config.BackendRegistry {
				cfg := &config.Config{
					BackendEndpoints:      "backend-1:http://localhost:8001",
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg)
			},
			wantErr:          false,
			wantBackendID:    "backend-1",
			wantDecisionType: "WEIGHTED",
		},
		{
			name: "multiple backends with weights",
			policy: &config.RoutingPolicy{
				PolicyID:       "test-policy",
				OrganizationID: "org-1",
				Model:          "test-model",
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 50},
					{BackendID: "backend-2", Weight: 50},
				},
			},
			setupBackends: func() *config.BackendRegistry {
				cfg := &config.Config{
					BackendEndpoints:      "backend-1:http://localhost:8001,backend-2:http://localhost:8002",
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg)
			},
			wantErr:          false,
			wantDecisionType: "WEIGHTED",
		},
		{
			name: "excludes degraded backends",
			policy: &config.RoutingPolicy{
				PolicyID:       "test-policy",
				OrganizationID: "org-1",
				Model:          "test-model",
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 50},
					{BackendID: "backend-2", Weight: 50},
				},
				DegradedBackends: []string{"backend-1"},
			},
			setupBackends: func() *config.BackendRegistry {
				cfg := &config.Config{
					BackendEndpoints:      "backend-1:http://localhost:8001,backend-2:http://localhost:8002",
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg)
			},
			wantErr: false,
			// Don't check specific backend ID since weighted selection is random
			// Just verify we got a backend
		},
		{
			name: "backend not found in registry",
			policy: &config.RoutingPolicy{
				PolicyID:       "test-policy",
				OrganizationID: "org-1",
				Model:          "test-model",
				Backends: []config.BackendWeight{
					{BackendID: "nonexistent-backend", Weight: 100},
				},
			},
			setupBackends: func() *config.BackendRegistry {
				cfg := &config.Config{
					BackendEndpoints:      "backend-1:http://localhost:8001",
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg)
			},
			wantErr:     true,
			errContains: "backend not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var backendRegistry *config.BackendRegistry
			if tt.setupBackends != nil {
				backendRegistry = tt.setupBackends()
			}

			healthMonitor := NewHealthMonitor(NewBackendClient(logger, 5*time.Second), logger, 10*time.Second)
			engine := NewEngine(healthMonitor, backendRegistry, 5*time.Second, logger)

			ctx := context.Background()
			backend, decision, err := engine.SelectBackend(ctx, tt.policy)

			if tt.wantErr {
				if err == nil {
					t.Errorf("SelectBackend() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("SelectBackend() unexpected error: %v", err)
				return
			}

			if backend == nil {
				t.Error("backend is nil")
				return
			}

			if decision == nil {
				t.Error("decision is nil")
				return
			}

			if tt.wantBackendID != "" && backend.ID != tt.wantBackendID {
				t.Errorf("backend.ID = %s, want %s", backend.ID, tt.wantBackendID)
			}

			if tt.wantDecisionType != "" && decision.DecisionType != tt.wantDecisionType {
				t.Errorf("decision.DecisionType = %s, want %s", decision.DecisionType, tt.wantDecisionType)
			}
		})
	}
}

func TestEngine_RouteWithFailover(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name             string
		policy           *config.RoutingPolicy
		setupBackends    func() (*config.BackendRegistry, []*httptest.Server)
		wantErr          bool
		errContains      string
		wantAttempts     int
		wantDecisionType string
	}{
		{
			name:        "nil policy",
			policy:      nil,
			wantErr:     true,
			errContains: "no backends configured",
		},
		{
			name: "successful primary backend",
			policy: &config.RoutingPolicy{
				PolicyID:          "test-policy",
				OrganizationID:    "org-1",
				Model:             "test-model",
				FailoverThreshold: 3,
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 100},
				},
			},
			setupBackends: func() (*config.BackendRegistry, []*httptest.Server) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"text": "response", "tokens_used": 5}`))
				}))

				cfg := &config.Config{
					BackendEndpoints:      fmt.Sprintf("backend-1:%s", server.URL),
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg), []*httptest.Server{server}
			},
			wantErr:          false,
			wantAttempts:     1,
			wantDecisionType: "PRIMARY",
		},
		{
			name: "failover to second backend",
			policy: &config.RoutingPolicy{
				PolicyID:          "test-policy",
				OrganizationID:    "org-1",
				Model:             "test-model",
				FailoverThreshold: 3,
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 100},
					{BackendID: "backend-2", Weight: 50},
				},
			},
			setupBackends: func() (*config.BackendRegistry, []*httptest.Server) {
				// First backend fails
				server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))

				// Second backend succeeds
				server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"text": "response", "tokens_used": 5}`))
				}))

				cfg := &config.Config{
					BackendEndpoints:      fmt.Sprintf("backend-1:%s,backend-2:%s", server1.URL, server2.URL),
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg), []*httptest.Server{server1, server2}
			},
			wantErr:          false,
			wantAttempts:     2,
			wantDecisionType: "FAILOVER",
		},
		{
			name: "all backends fail",
			policy: &config.RoutingPolicy{
				PolicyID:          "test-policy",
				OrganizationID:    "org-1",
				Model:             "test-model",
				FailoverThreshold: 3,
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 100},
					{BackendID: "backend-2", Weight: 50},
				},
			},
			setupBackends: func() (*config.BackendRegistry, []*httptest.Server) {
				server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusInternalServerError)
				}))

				server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusServiceUnavailable)
				}))

				cfg := &config.Config{
					BackendEndpoints:      fmt.Sprintf("backend-1:%s,backend-2:%s", server1.URL, server2.URL),
					DefaultBackendTimeout: 5 * time.Second,
				}
				return config.NewBackendRegistry(cfg), []*httptest.Server{server1, server2}
			},
			wantErr:     true,
			errContains: "all backends failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var backendRegistry *config.BackendRegistry
			var servers []*httptest.Server

			if tt.setupBackends != nil {
				backendRegistry, servers = tt.setupBackends()
				defer func() {
					for _, s := range servers {
						s.Close()
					}
				}()
			}

			healthMonitor := NewHealthMonitor(NewBackendClient(logger, 5*time.Second), logger, 10*time.Second)
			engine := NewEngine(healthMonitor, backendRegistry, 5*time.Second, logger)
			client := NewBackendClient(logger, 5*time.Second)

			request := &BackendRequest{
				Prompt:    "Test prompt",
				MaxTokens: 100,
			}

			ctx := context.Background()
			resp, decision, err := engine.RouteWithFailover(ctx, tt.policy, request, client)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RouteWithFailover() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("RouteWithFailover() unexpected error: %v", err)
				return
			}

			if resp == nil {
				t.Error("response is nil")
				return
			}

			if decision == nil {
				t.Error("decision is nil")
				return
			}

			if tt.wantAttempts > 0 && decision.AttemptNumber != tt.wantAttempts {
				t.Errorf("decision.AttemptNumber = %d, want %d", decision.AttemptNumber, tt.wantAttempts)
			}

			if tt.wantDecisionType != "" && decision.DecisionType != tt.wantDecisionType {
				t.Errorf("decision.DecisionType = %s, want %s", decision.DecisionType, tt.wantDecisionType)
			}
		})
	}
}

func TestEngine_GetRecentDecisions(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewEngine(nil, nil, 5*time.Second, logger)

	// Add some decisions
	for i := 0; i < 10; i++ {
		decision := &RoutingDecision{
			BackendID:     fmt.Sprintf("backend-%d", i),
			DecisionType:  "WEIGHTED",
			Reason:        "test",
			Timestamp:     time.Now(),
			AttemptNumber: 1,
		}
		engine.recordDecision(decision)
	}

	tests := []struct {
		name      string
		limit     int
		wantCount int
	}{
		{
			name:      "get last 5 decisions",
			limit:     5,
			wantCount: 5,
		},
		{
			name:      "get all decisions",
			limit:     0,
			wantCount: 10,
		},
		{
			name:      "limit exceeds decision count",
			limit:     20,
			wantCount: 10,
		},
		{
			name:      "negative limit",
			limit:     -1,
			wantCount: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decisions := engine.GetRecentDecisions(tt.limit)

			if len(decisions) != tt.wantCount {
				t.Errorf("GetRecentDecisions() returned %d decisions, want %d", len(decisions), tt.wantCount)
			}
		})
	}
}

func TestEngine_RecordDecision_CircularBuffer(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewEngine(nil, nil, 5*time.Second, logger)

	// Add 150 decisions (more than the 100 limit)
	for i := 0; i < 150; i++ {
		decision := &RoutingDecision{
			BackendID:     fmt.Sprintf("backend-%d", i),
			DecisionType:  "WEIGHTED",
			Reason:        "test",
			Timestamp:     time.Now(),
			AttemptNumber: 1,
		}
		engine.recordDecision(decision)
	}

	// Should only keep last 100
	decisions := engine.GetRecentDecisions(0)
	if len(decisions) != 100 {
		t.Errorf("decisions count = %d, want 100 (circular buffer limit)", len(decisions))
	}

	// Verify oldest decisions were removed
	firstDecision := decisions[0]
	if firstDecision.BackendID == "backend-0" {
		t.Error("circular buffer did not remove oldest decisions")
	}
}

func TestEngine_RouteToRegisteredModel(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name             string
		modelName        string
		setupRegistry    func() *Registry
		setupServer      func() *httptest.Server
		wantErr          bool
		errContains      string
		wantDecisionType string
	}{
		{
			name:      "registry not configured",
			modelName: "test-model",
			setupRegistry: func() *Registry {
				return nil
			},
			wantErr:     true,
			errContains: "registry not configured",
		},
		// Skipping "model not found in registry" test because it requires a working database connection
		// This would be better suited for integration tests
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var registry *Registry
			var server *httptest.Server

			if tt.setupRegistry != nil {
				registry = tt.setupRegistry()
			}

			if tt.setupServer != nil {
				server = tt.setupServer()
				defer server.Close()
			}

			engine := NewEngine(nil, nil, 5*time.Second, logger)
			engine.SetModelRegistry(registry)

			client := NewBackendClient(logger, 5*time.Second)
			request := &BackendRequest{
				Prompt: "Test prompt",
			}

			ctx := context.Background()
			_, decision, err := engine.RouteToRegisteredModel(ctx, tt.modelName, request, client)

			if tt.wantErr {
				if err == nil {
					t.Errorf("RouteToRegisteredModel() expected error, got nil")
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want to contain %s", err, tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("RouteToRegisteredModel() unexpected error: %v", err)
				return
			}

			if decision != nil && tt.wantDecisionType != "" && decision.DecisionType != tt.wantDecisionType {
				t.Errorf("decision.DecisionType = %s, want %s", decision.DecisionType, tt.wantDecisionType)
			}
		})
	}
}

func TestRandomInt64(t *testing.T) {
	tests := []struct {
		name    string
		max     int64
		wantErr bool
	}{
		{
			name:    "valid positive max",
			max:     100,
			wantErr: false,
		},
		{
			name:    "max = 1",
			max:     1,
			wantErr: false,
		},
		{
			name:    "zero max",
			max:     0,
			wantErr: true,
		},
		{
			name:    "negative max",
			max:     -10,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := randomInt64(tt.max)

			if tt.wantErr {
				if err == nil {
					t.Errorf("randomInt64() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("randomInt64() unexpected error: %v", err)
				return
			}

			if result < 0 || result >= tt.max {
				t.Errorf("randomInt64() = %d, want in range [0, %d)", result, tt.max)
			}
		})
	}
}

func TestEngine_SelectWeightedBackend(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewEngine(nil, nil, 5*time.Second, logger)

	tests := []struct {
		name     string
		backends []config.BackendWeight
		want     string // backend ID or empty if nil expected
	}{
		{
			name:     "empty backends",
			backends: []config.BackendWeight{},
			want:     "",
		},
		{
			name: "single backend",
			backends: []config.BackendWeight{
				{BackendID: "backend-1", Weight: 100},
			},
			want: "backend-1",
		},
		{
			name: "all backends with zero weight",
			backends: []config.BackendWeight{
				{BackendID: "backend-1", Weight: 0},
				{BackendID: "backend-2", Weight: 0},
			},
			want: "backend-1", // Should return first backend
		},
		{
			name: "multiple backends with weights",
			backends: []config.BackendWeight{
				{BackendID: "backend-1", Weight: 50},
				{BackendID: "backend-2", Weight: 50},
			},
			want: "", // Can be either, don't test specific backend
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := engine.selectWeightedBackend(tt.backends)

			if tt.want == "" && len(tt.backends) == 0 {
				if result != nil {
					t.Errorf("selectWeightedBackend() = %v, want nil", result)
				}
				return
			}

			if tt.want != "" {
				if result == nil {
					t.Error("selectWeightedBackend() returned nil")
					return
				}
				if result.BackendID != tt.want {
					t.Errorf("selectWeightedBackend() = %s, want %s", result.BackendID, tt.want)
				}
			} else if len(tt.backends) > 0 && result == nil {
				t.Error("selectWeightedBackend() returned nil for non-empty backends")
			}
		})
	}
}

func TestEngine_SortBackendsByWeight(t *testing.T) {
	logger := zaptest.NewLogger(t)
	engine := NewEngine(nil, nil, 5*time.Second, logger)

	backends := []config.BackendWeight{
		{BackendID: "backend-1", Weight: 10},
		{BackendID: "backend-2", Weight: 100},
		{BackendID: "backend-3", Weight: 50},
	}

	engine.sortBackendsByWeight(backends)

	// Should be sorted in descending order
	if backends[0].Weight != 100 {
		t.Errorf("backends[0].Weight = %d, want 100", backends[0].Weight)
	}
	if backends[1].Weight != 50 {
		t.Errorf("backends[1].Weight = %d, want 50", backends[1].Weight)
	}
	if backends[2].Weight != 10 {
		t.Errorf("backends[2].Weight = %d, want 10", backends[2].Weight)
	}
}

func TestEngine_GetAvailableBackends(t *testing.T) {
	logger := zaptest.NewLogger(t)

	tests := []struct {
		name           string
		policy         *config.RoutingPolicy
		setupMonitor   func() *HealthMonitor
		wantCount      int
		wantBackendIDs []string
	}{
		{
			name: "no degraded backends",
			policy: &config.RoutingPolicy{
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 50},
					{BackendID: "backend-2", Weight: 50},
				},
			},
			wantCount:      2,
			wantBackendIDs: []string{"backend-1", "backend-2"},
		},
		{
			name: "one degraded backend in policy",
			policy: &config.RoutingPolicy{
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 50},
					{BackendID: "backend-2", Weight: 50},
				},
				DegradedBackends: []string{"backend-1"},
			},
			wantCount:      1,
			wantBackendIDs: []string{"backend-2"},
		},
		{
			name: "all backends degraded - fallback to all",
			policy: &config.RoutingPolicy{
				Backends: []config.BackendWeight{
					{BackendID: "backend-1", Weight: 50},
					{BackendID: "backend-2", Weight: 50},
				},
				DegradedBackends: []string{"backend-1", "backend-2"},
			},
			wantCount:      2,
			wantBackendIDs: []string{"backend-1", "backend-2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var healthMonitor *HealthMonitor
			if tt.setupMonitor != nil {
				healthMonitor = tt.setupMonitor()
			}

			engine := NewEngine(healthMonitor, nil, 5*time.Second, logger)
			available := engine.getAvailableBackends(tt.policy)

			if len(available) != tt.wantCount {
				t.Errorf("getAvailableBackends() returned %d backends, want %d", len(available), tt.wantCount)
			}

			if tt.wantBackendIDs != nil {
				for i, wantID := range tt.wantBackendIDs {
					if i < len(available) && available[i].BackendID != wantID {
						t.Errorf("available[%d].BackendID = %s, want %s", i, available[i].BackendID, wantID)
					}
				}
			}
		})
	}
}

// Mock Registry for testing RouteToRegisteredModel
type mockRegistry struct {
	lookupFunc func(ctx context.Context, modelName string) (*ModelRegistryEntry, error)
}

func (m *mockRegistry) LookupModel(ctx context.Context, modelName string) (*ModelRegistryEntry, error) {
	if m.lookupFunc != nil {
		return m.lookupFunc(ctx, modelName)
	}
	return nil, errors.New("model not found")
}

func (m *mockRegistry) LookupModelInEnvironment(ctx context.Context, modelName, environment string) (*ModelRegistryEntry, error) {
	return m.LookupModel(ctx, modelName)
}

func (m *mockRegistry) Close() error {
	return nil
}
