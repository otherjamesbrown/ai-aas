package config

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap/zaptest"
)

// getEtcdEndpoint returns the etcd endpoint for testing.
// If ETCD_ENDPOINT env var is set, uses that. Otherwise returns empty string
// to test fallback behavior.
func getEtcdEndpoint() string {
	endpoint := os.Getenv("ETCD_ENDPOINT")
	if endpoint == "" {
		// Default to localhost:2379 for local testing
		endpoint = "localhost:2379"
	}
	return endpoint
}

// setupTestCache creates a temporary cache for testing.
func setupTestCache(t *testing.T) *Cache {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test-cache.db")
	cache, err := NewCache(cachePath)
	if err != nil {
		t.Fatalf("failed to create test cache: %v", err)
	}
	return cache
}

// setupTestEtcdClient creates an etcd client for testing.
// Returns nil if etcd is not available (tests will verify fallback behavior).
func setupTestEtcdClient(t *testing.T, endpoint string) *clientv3.Client {
	cfg := clientv3.Config{
		Endpoints:   []string{endpoint},
		DialTimeout: 2 * time.Second,
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		t.Logf("etcd not available at %s: %v (testing fallback behavior)", endpoint, err)
		return nil
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = client.Status(ctx, endpoint)
	if err != nil {
		_ = client.Close()
		t.Logf("etcd not available at %s: %v (testing fallback behavior)", endpoint, err)
		return nil
	}

	return client
}

// cleanupEtcd removes test keys from etcd.
func cleanupEtcd(t *testing.T, client *clientv3.Client) {
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := client.Delete(ctx, etcdKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		t.Logf("failed to cleanup etcd: %v", err)
	}
}

// putPolicyToEtcd stores a policy in etcd for testing.
func putPolicyToEtcd(t *testing.T, client *clientv3.Client, policy *RoutingPolicy) {
	if client == nil {
		t.Skip("etcd not available")
	}

	key := etcdPolicyKey(policy.OrganizationID, policy.Model)
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatalf("failed to marshal policy: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err = client.Put(ctx, key, string(data))
	if err != nil {
		t.Fatalf("failed to put policy to etcd: %v", err)
	}
}

func TestLoader_Load_FromCache(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader("invalid-endpoint:2379", false, cache, logger)

	// Store a policy in cache
	policy := &RoutingPolicy{
		PolicyID:       "test-policy-1",
		OrganizationID: "org-123",
		Model:          "gpt-4o",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}

	ctx := context.Background()
	if err := cache.StorePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to store policy: %v", err)
	}

	// Load should succeed from cache even if etcd is unavailable
	err := loader.Load(ctx)
	if err != nil {
		t.Errorf("Load() failed: %v", err)
	}
}

func TestLoader_Load_FromEtcd(t *testing.T) {
	endpoint := getEtcdEndpoint()
	client := setupTestEtcdClient(t, endpoint)
	if client == nil {
		t.Skip("etcd not available, skipping etcd integration test")
	}
	defer func() { _ = client.Close() }()
	defer cleanupEtcd(t, client)

	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader(endpoint, false, cache, logger)

	// Store a policy in etcd
	policy := &RoutingPolicy{
		PolicyID:       "test-policy-etcd",
		OrganizationID: "*",
		Model:          "gpt-4o",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 70},
			{BackendID: "backend-2", Weight: 30},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}

	putPolicyToEtcd(t, client, policy)

	// Load should fetch from etcd and store in cache
	ctx := context.Background()
	err := loader.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Verify policy was stored in cache
	cachedPolicy, err := cache.GetPolicy("*", "gpt-4o")
	if err != nil {
		t.Fatalf("failed to get policy from cache: %v", err)
	}

	if cachedPolicy.PolicyID != policy.PolicyID {
		t.Errorf("expected policy ID %s, got %s", policy.PolicyID, cachedPolicy.PolicyID)
	}
	if len(cachedPolicy.Backends) != 2 {
		t.Errorf("expected 2 backends, got %d", len(cachedPolicy.Backends))
	}
}

func TestLoader_GetPolicy_CacheHit(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader("invalid-endpoint:2379", false, cache, logger)

	// Store policy in cache
	policy := &RoutingPolicy{
		PolicyID:       "test-policy-cache",
		OrganizationID: "org-456",
		Model:          "gpt-4o",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}

	ctx := context.Background()
	if err := cache.StorePolicy(ctx, policy); err != nil {
		t.Fatalf("failed to store policy: %v", err)
	}

	// GetPolicy should return from cache
	retrievedPolicy, err := loader.GetPolicy("org-456", "gpt-4o")
	if err != nil {
		t.Fatalf("GetPolicy() failed: %v", err)
	}

	if retrievedPolicy.PolicyID != policy.PolicyID {
		t.Errorf("expected policy ID %s, got %s", policy.PolicyID, retrievedPolicy.PolicyID)
	}
}

func TestLoader_GetPolicy_GlobalFallback(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader("invalid-endpoint:2379", false, cache, logger)

	// Store global policy in cache
	globalPolicy := &RoutingPolicy{
		PolicyID:       "global-policy",
		OrganizationID: "*",
		Model:          "gpt-4o",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}

	ctx := context.Background()
	if err := cache.StorePolicy(ctx, globalPolicy); err != nil {
		t.Fatalf("failed to store global policy: %v", err)
	}

	// GetPolicy for org-specific should fallback to global
	retrievedPolicy, err := loader.GetPolicy("org-789", "gpt-4o")
	if err != nil {
		t.Fatalf("GetPolicy() failed: %v", err)
	}

	if retrievedPolicy.PolicyID != globalPolicy.PolicyID {
		t.Errorf("expected global policy ID %s, got %s", globalPolicy.PolicyID, retrievedPolicy.PolicyID)
	}
}

func TestLoader_GetPolicy_FromEtcd(t *testing.T) {
	endpoint := getEtcdEndpoint()
	client := setupTestEtcdClient(t, endpoint)
	if client == nil {
		t.Skip("etcd not available, skipping etcd integration test")
	}
	defer func() { _ = client.Close() }()
	defer cleanupEtcd(t, client)

	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader(endpoint, false, cache, logger)

	// Store policy in etcd (not in cache)
	policy := &RoutingPolicy{
		PolicyID:       "test-policy-etcd-lookup",
		OrganizationID: "org-etcd",
		Model:          "gpt-4o",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}

	putPolicyToEtcd(t, client, policy)

	// Connect loader to etcd
	ctx := context.Background()
	if err := loader.connect(ctx); err != nil {
		t.Fatalf("failed to connect to etcd: %v", err)
	}

	// GetPolicy should fetch from etcd (cache miss)
	retrievedPolicy, err := loader.GetPolicy("org-etcd", "gpt-4o")
	if err != nil {
		t.Fatalf("GetPolicy() failed: %v", err)
	}

	if retrievedPolicy.PolicyID != policy.PolicyID {
		t.Errorf("expected policy ID %s, got %s", policy.PolicyID, retrievedPolicy.PolicyID)
	}

	// Verify policy was cached
	cachedPolicy, err := cache.GetPolicy("org-etcd", "gpt-4o")
	if err != nil {
		t.Fatalf("policy should have been cached: %v", err)
	}
	if cachedPolicy.PolicyID != policy.PolicyID {
		t.Errorf("cached policy ID mismatch: expected %s, got %s", policy.PolicyID, cachedPolicy.PolicyID)
	}
}

func TestLoader_Watch_UpdatesCache(t *testing.T) {
	endpoint := getEtcdEndpoint()
	client := setupTestEtcdClient(t, endpoint)
	if client == nil {
		t.Skip("etcd not available, skipping watch test")
	}
	defer func() { _ = client.Close() }()
	defer cleanupEtcd(t, client)

	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader(endpoint, true, cache, logger)

	ctx := context.Background()

	// Connect and start watch
	if err := loader.connect(ctx); err != nil {
		t.Fatalf("failed to connect to etcd: %v", err)
	}

	if err := loader.Watch(ctx); err != nil {
		t.Fatalf("Watch() failed: %v", err)
	}

	// Give watch time to start
	time.Sleep(100 * time.Millisecond)

	// Put a new policy in etcd
	policy := &RoutingPolicy{
		PolicyID:       "test-policy-watch",
		OrganizationID: "org-watch",
		Model:          "gpt-4o",
		Backends: []BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
		FailoverThreshold: 3,
		UpdatedAt:         time.Now(),
		Version:           1,
	}

	putPolicyToEtcd(t, client, policy)

	// Wait for watch to process
	time.Sleep(500 * time.Millisecond)

	// Verify policy was cached by watch
	cachedPolicy, err := cache.GetPolicy("org-watch", "gpt-4o")
	if err != nil {
		t.Fatalf("policy should have been cached by watch: %v", err)
	}

	if cachedPolicy.PolicyID != policy.PolicyID {
		t.Errorf("expected policy ID %s, got %s", policy.PolicyID, cachedPolicy.PolicyID)
	}

	// Stop watch
	loader.Stop()
}

func TestLoader_Stop_ClosesConnection(t *testing.T) {
	endpoint := getEtcdEndpoint()
	client := setupTestEtcdClient(t, endpoint)
	if client == nil {
		t.Skip("etcd not available, skipping stop test")
	}
	defer func() { _ = client.Close() }()
	defer cleanupEtcd(t, client)

	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader(endpoint, true, cache, logger)

	ctx := context.Background()
	if err := loader.connect(ctx); err != nil {
		t.Fatalf("failed to connect to etcd: %v", err)
	}

	if err := loader.Watch(ctx); err != nil {
		t.Fatalf("Watch() failed: %v", err)
	}

	// Stop should close connection
	loader.Stop()

	// Verify connection is closed (client should be nil)
	if loader.client != nil {
		t.Error("client should be nil after Stop()")
	}
}

func TestLoader_Load_NoSourcesAvailable(t *testing.T) {
	cache := setupTestCache(t)
	defer func() { _ = cache.Close() }()

	logger := zaptest.NewLogger(t)
	loader := NewLoader("invalid-endpoint:2379", false, cache, logger)

	// Don't store any policies in cache
	ctx := context.Background()
	err := loader.Load(ctx)

	// Should return error when neither etcd nor cache have policies
	if err == nil {
		t.Error("Load() should fail when no policies available")
	}
}

func TestEtcdPolicyKey(t *testing.T) {
	tests := []struct {
		name           string
		organizationID string
		model          string
		expected       string
	}{
		{
			name:           "global policy",
			organizationID: "*",
			model:          "gpt-4o",
			expected:       "/api-router/policies/*/gpt-4o",
		},
		{
			name:           "org-specific policy",
			organizationID: "123e4567-e89b-12d3-a456-426614174000",
			model:          "gpt-4o",
			expected:       "/api-router/policies/123e4567-e89b-12d3-a456-426614174000/gpt-4o",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := etcdPolicyKey(tt.organizationID, tt.model)
			if result != tt.expected {
				t.Errorf("etcdPolicyKey(%q, %q) = %q, want %q", tt.organizationID, tt.model, result, tt.expected)
			}
		})
	}
}

