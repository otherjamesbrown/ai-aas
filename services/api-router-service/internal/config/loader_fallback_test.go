package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// setupTestCacheForFallback creates a temporary cache for fallback testing.
func setupTestCacheForFallback(t *testing.T) *Cache {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test-fallback-cache.db")
	cache, err := NewCache(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = cache.Close()
		_ = os.Remove(dbPath)
	})
	return cache
}

// mockAdminAPIClient implements AdminAPIClient for testing.
type mockAdminAPIClient struct {
	models map[string]*ModelDeploymentInfo
	err    error
}

func (m *mockAdminAPIClient) GetModelDeployment(ctx context.Context, modelName string) (*ModelDeploymentInfo, error) {
	if m.err != nil {
		return nil, m.err
	}
	model, ok := m.models[modelName]
	if !ok {
		return nil, nil
	}
	return model, nil
}

func TestGetPolicyWithFallback_OrgPolicy(t *testing.T) {
	// Setup: Create real cache with test database
	cache := setupTestCacheForFallback(t)

	// Store org-specific policy
	err := cache.StorePolicy(context.Background(), &RoutingPolicy{
		PolicyID:       "policy-org-123-gpt4",
		OrganizationID: "org-123",
		Model:          "gpt-4",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
	})
	require.NoError(t, err)

	loader := NewLoader("", false, cache, zap.NewNop())
	adminClient := &mockAdminAPIClient{models: make(map[string]*ModelDeploymentInfo)}

	// Test: Org-specific policy should be returned first
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-123", "gpt-4", adminClient)

	require.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "org_policy", source)
	assert.Equal(t, "policy-org-123-gpt4", policy.PolicyID)
	assert.Equal(t, "org-123", policy.OrganizationID)
}

func TestGetPolicyWithFallback_GlobalPolicy(t *testing.T) {
	// Setup: No org-specific policy, but global policy exists
	cache := setupTestCacheForFallback(t)

	// Store global policy
	err := cache.StorePolicy(context.Background(), &RoutingPolicy{
		PolicyID:       "policy-global-gpt4",
		OrganizationID: "*",
		Model:          "gpt-4",
		Backends: []BackendWeight{
			{BackendID: "backend-global", Weight: 100},
		},
	})
	require.NoError(t, err)

	loader := NewLoader("", false, cache, zap.NewNop())
	adminClient := &mockAdminAPIClient{models: make(map[string]*ModelDeploymentInfo)}

	// Test: Global policy should be returned when no org policy exists
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-456", "gpt-4", adminClient)

	require.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "global_policy", source)
	assert.Equal(t, "policy-global-gpt4", policy.PolicyID)
	assert.Equal(t, "*", policy.OrganizationID)
}

func TestGetPolicyWithFallback_RegistryDiscovery(t *testing.T) {
	// Setup: No policies, but model is deployed
	cache := setupTestCacheForFallback(t)

	ready := "ready"
	adminClient := &mockAdminAPIClient{
		models: map[string]*ModelDeploymentInfo{
			"llama-7b": {
				Name:             "llama-7b",
				DeploymentStatus: &ready,
				InferenceService: "llama-7b-vllm",
			},
		},
	}

	loader := NewLoader("", false, cache, zap.NewNop())

	// Test: Ephemeral policy should be created from registry discovery
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-789", "llama-7b", adminClient)

	require.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "registry_discovery", source)
	assert.Equal(t, "ephemeral-llama-7b", policy.PolicyID)
	assert.Equal(t, "*", policy.OrganizationID) // Ephemeral policies are global
	assert.Len(t, policy.Backends, 1)
	assert.Equal(t, "llama-7b-vllm", policy.Backends[0].BackendID)
	assert.Equal(t, 100, policy.Backends[0].Weight)
}

func TestGetPolicyWithFallback_ModelNotReady(t *testing.T) {
	// Setup: Model exists but not ready
	cache := setupTestCacheForFallback(t)

	pending := "pending"
	adminClient := &mockAdminAPIClient{
		models: map[string]*ModelDeploymentInfo{
			"llama-7b": {
				Name:             "llama-7b",
				DeploymentStatus: &pending,
				InferenceService: "llama-7b-vllm",
			},
		},
	}

	loader := NewLoader("", false, cache, zap.NewNop())

	// Test: Should return error indicating model is not ready
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-xyz", "llama-7b", adminClient)

	require.Error(t, err)
	assert.Nil(t, policy)
	assert.Empty(t, source)
	assert.Contains(t, err.Error(), "not ready")
	assert.Contains(t, err.Error(), "pending")
}

func TestGetPolicyWithFallback_ModelNotDeployed(t *testing.T) {
	// Setup: No policies and model not in registry
	cache := setupTestCacheForFallback(t)

	adminClient := &mockAdminAPIClient{models: make(map[string]*ModelDeploymentInfo)}

	loader := NewLoader("", false, cache, zap.NewNop())

	// Test: Should return error indicating model is not deployed
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-abc", "unknown-model", adminClient)

	require.Error(t, err)
	assert.Nil(t, policy)
	assert.Empty(t, source)
	assert.Contains(t, err.Error(), "not deployed or not accessible")
}

func TestGetPolicyWithFallback_OrgPolicyPreferredOverGlobal(t *testing.T) {
	// Setup: Both org-specific and global policies exist
	cache := setupTestCacheForFallback(t)

	// Store org-specific policy
	err := cache.StorePolicy(context.Background(), &RoutingPolicy{
		PolicyID:       "policy-org-999-gpt4",
		OrganizationID: "org-999",
		Model:          "gpt-4",
		Backends: []BackendWeight{
			{BackendID: "backend-org", Weight: 100},
		},
	})
	require.NoError(t, err)

	// Store global policy
	err = cache.StorePolicy(context.Background(), &RoutingPolicy{
		PolicyID:       "policy-global-gpt4",
		OrganizationID: "*",
		Model:          "gpt-4",
		Backends: []BackendWeight{
			{BackendID: "backend-global", Weight: 100},
		},
	})
	require.NoError(t, err)

	loader := NewLoader("", false, cache, zap.NewNop())
	adminClient := &mockAdminAPIClient{models: make(map[string]*ModelDeploymentInfo)}

	// Test: Org policy should be preferred over global
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-999", "gpt-4", adminClient)

	require.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "org_policy", source)
	assert.Equal(t, "policy-org-999-gpt4", policy.PolicyID)
	assert.Equal(t, "backend-org", policy.Backends[0].BackendID)
}

func TestGetPolicyWithFallback_GlobalPolicyPreferredOverRegistry(t *testing.T) {
	// Setup: Global policy and deployed model exist
	cache := setupTestCacheForFallback(t)

	// Store global policy
	err := cache.StorePolicy(context.Background(), &RoutingPolicy{
		PolicyID:       "policy-global-llama",
		OrganizationID: "*",
		Model:          "llama-7b",
		Backends: []BackendWeight{
			{BackendID: "backend-global", Weight: 100},
		},
	})
	require.NoError(t, err)

	ready := "ready"
	adminClient := &mockAdminAPIClient{
		models: map[string]*ModelDeploymentInfo{
			"llama-7b": {
				Name:             "llama-7b",
				DeploymentStatus: &ready,
				InferenceService: "llama-7b-vllm",
			},
		},
	}

	loader := NewLoader("", false, cache, zap.NewNop())

	// Test: Global policy should be preferred over registry discovery
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-111", "llama-7b", adminClient)

	require.NoError(t, err)
	assert.NotNil(t, policy)
	assert.Equal(t, "global_policy", source)
	assert.Equal(t, "policy-global-llama", policy.PolicyID)
	assert.Equal(t, "backend-global", policy.Backends[0].BackendID)
}

func TestGetPolicyWithFallback_NoAdminAPIClient(t *testing.T) {
	// Setup: No policies and no admin API client
	cache := setupTestCacheForFallback(t)

	loader := NewLoader("", false, cache, zap.NewNop())

	// Test: Should return error (no fallback to registry without client)
	policy, source, err := loader.GetPolicyWithFallback(context.Background(), "org-222", "some-model", nil)

	require.Error(t, err)
	assert.Nil(t, policy)
	assert.Empty(t, source)
	assert.Contains(t, err.Error(), "not deployed or not accessible")
}
