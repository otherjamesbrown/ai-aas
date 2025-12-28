package routing

import (
	"testing"
	"time"

	"go.uber.org/zap/zaptest"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

func TestNewPolicyCache(t *testing.T) {
	logger := zaptest.NewLogger(t)

	// Create a minimal loader for testing
	loader := config.NewLoader("", false, nil, logger)

	cache := NewPolicyCache(loader, logger)

	if cache == nil {
		t.Fatal("NewPolicyCache returned nil")
	}

	if cache.loader == nil {
		t.Error("loader not set correctly")
	}

	if cache.logger == nil {
		t.Error("logger is nil")
	}

	if cache.policies == nil {
		t.Error("policies map is nil")
	}
}

func TestPolicyCache_GetPolicy_CacheHit(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	// Pre-populate cache
	policy := &config.RoutingPolicy{
		PolicyID:       "policy-1",
		OrganizationID: "org-1",
		Model:          "model-1",
	}
	cache.UpdatePolicy(policy)

	// Get from cache
	cached, err := cache.GetPolicy("org-1", "model-1")
	if err != nil {
		t.Errorf("GetPolicy() unexpected error: %v", err)
		return
	}

	if cached == nil {
		t.Error("GetPolicy() returned nil")
		return
	}

	if cached.PolicyID != "policy-1" {
		t.Errorf("policy.PolicyID = %s, want policy-1", cached.PolicyID)
	}
}

func TestPolicyCache_UpdatePolicy(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	policy := &config.RoutingPolicy{
		PolicyID:       "policy-1",
		OrganizationID: "org-1",
		Model:          "model-1",
		Backends: []config.BackendWeight{
			{BackendID: "backend-1", Weight: 100},
		},
	}

	cache.UpdatePolicy(policy)

	// Verify policy was cached
	cached, err := cache.GetPolicy("org-1", "model-1")
	if err != nil {
		t.Errorf("GetPolicy() unexpected error: %v", err)
		return
	}

	if cached.PolicyID != "policy-1" {
		t.Errorf("cached policy.PolicyID = %s, want policy-1", cached.PolicyID)
	}

	// Update with new policy
	updatedPolicy := &config.RoutingPolicy{
		PolicyID:       "policy-2",
		OrganizationID: "org-1",
		Model:          "model-1",
		Backends: []config.BackendWeight{
			{BackendID: "backend-2", Weight: 100},
		},
	}

	cache.UpdatePolicy(updatedPolicy)

	// Verify cache was updated
	cached, err = cache.GetPolicy("org-1", "model-1")
	if err != nil {
		t.Errorf("GetPolicy() unexpected error: %v", err)
		return
	}

	if cached.PolicyID != "policy-2" {
		t.Errorf("cached policy.PolicyID = %s, want policy-2 after update", cached.PolicyID)
	}
}

func TestPolicyCache_UpdatePolicy_NilPolicy(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	// Should not panic
	cache.UpdatePolicy(nil)
}

func TestPolicyCache_InvalidatePolicy(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	policy := &config.RoutingPolicy{
		PolicyID:       "policy-1",
		OrganizationID: "org-1",
		Model:          "model-1",
	}

	cache.UpdatePolicy(policy)

	// Verify policy is cached
	_, err := cache.GetPolicy("org-1", "model-1")
	if err != nil {
		t.Errorf("GetPolicy() unexpected error before invalidation: %v", err)
	}

	// Invalidate
	cache.InvalidatePolicy("org-1", "model-1")

	// Verify policy was removed from cache
	cache.mu.RLock()
	_, exists := cache.policies[cache.cacheKey("org-1", "model-1")]
	cache.mu.RUnlock()

	if exists {
		t.Error("policy still in cache after invalidation")
	}
}

func TestPolicyCache_InvalidateAll(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	// Add multiple policies
	policies := []*config.RoutingPolicy{
		{PolicyID: "policy-1", OrganizationID: "org-1", Model: "model-1"},
		{PolicyID: "policy-2", OrganizationID: "org-1", Model: "model-2"},
		{PolicyID: "policy-3", OrganizationID: "org-2", Model: "model-1"},
	}

	for _, p := range policies {
		cache.UpdatePolicy(p)
	}

	// Verify all are cached
	cache.mu.RLock()
	count := len(cache.policies)
	cache.mu.RUnlock()

	if count != 3 {
		t.Errorf("cache has %d policies, want 3", count)
	}

	// Invalidate all
	cache.InvalidateAll()

	// Verify cache is empty
	cache.mu.RLock()
	count = len(cache.policies)
	cache.mu.RUnlock()

	if count != 0 {
		t.Errorf("cache has %d policies after InvalidateAll, want 0", count)
	}
}

func TestPolicyCache_GetCacheStats(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	// Initial stats
	stats := cache.GetCacheStats()
	if stats["policy_count"] != 0 {
		t.Errorf("initial policy_count = %v, want 0", stats["policy_count"])
	}

	// Add policies
	policies := []*config.RoutingPolicy{
		{PolicyID: "policy-1", OrganizationID: "org-1", Model: "model-1"},
		{PolicyID: "policy-2", OrganizationID: "org-1", Model: "model-2"},
	}

	for _, p := range policies {
		cache.UpdatePolicy(p)
	}

	// Check stats
	stats = cache.GetCacheStats()
	if stats["policy_count"] != 2 {
		t.Errorf("policy_count = %v, want 2", stats["policy_count"])
	}

	lastUpdated, ok := stats["last_updated"].(time.Time)
	if !ok {
		t.Error("last_updated is not a time.Time")
	}

	if lastUpdated.IsZero() {
		t.Error("last_updated is zero")
	}
}

func TestPolicyCache_CacheKey(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	tests := []struct {
		name           string
		organizationID string
		model          string
		wantKey        string
	}{
		{
			name:           "simple key",
			organizationID: "org-1",
			model:          "model-1",
			wantKey:        "org-1:model-1",
		},
		{
			name:           "global organization",
			organizationID: "*",
			model:          "model-1",
			wantKey:        "*:model-1",
		},
		{
			name:           "model with slashes",
			organizationID: "org-1",
			model:          "meta-llama/Llama-3.1-8B",
			wantKey:        "org-1:meta-llama/Llama-3.1-8B",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key := cache.cacheKey(tt.organizationID, tt.model)
			if key != tt.wantKey {
				t.Errorf("cacheKey() = %s, want %s", key, tt.wantKey)
			}
		})
	}
}

func TestPolicyCache_HandlePolicyUpdate(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	policy := &config.RoutingPolicy{
		PolicyID:       "policy-1",
		OrganizationID: "org-1",
		Model:          "model-1",
	}

	cache.HandlePolicyUpdate(policy)

	// Verify policy was cached
	cached, err := cache.GetPolicy("org-1", "model-1")
	if err != nil {
		t.Errorf("GetPolicy() unexpected error: %v", err)
		return
	}

	if cached.PolicyID != "policy-1" {
		t.Errorf("cached policy.PolicyID = %s, want policy-1", cached.PolicyID)
	}
}

func TestPolicyCache_HandlePolicyDelete(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	policy := &config.RoutingPolicy{
		PolicyID:       "policy-1",
		OrganizationID: "org-1",
		Model:          "model-1",
	}

	cache.UpdatePolicy(policy)

	// Delete policy
	cache.HandlePolicyDelete("org-1", "model-1")

	// Verify policy was removed
	cache.mu.RLock()
	_, exists := cache.policies[cache.cacheKey("org-1", "model-1")]
	cache.mu.RUnlock()

	if exists {
		t.Error("policy still in cache after delete")
	}
}

func TestPolicyCache_ThreadSafety(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	done := make(chan bool)

	// Concurrent writes
	for i := 0; i < 5; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				policy := &config.RoutingPolicy{
					PolicyID:       "policy-1",
					OrganizationID: "org-1",
					Model:          "model-1",
				}
				cache.UpdatePolicy(policy)
				cache.InvalidatePolicy("org-1", "model-1")
				time.Sleep(time.Millisecond)
			}
			done <- true
		}(i)
	}

	// Concurrent reads
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				_, _ = cache.GetPolicy("org-1", "model-1")
				_ = cache.GetCacheStats()
				time.Sleep(time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestPolicyCache_LastUpdatedTracking(t *testing.T) {
	logger := zaptest.NewLogger(t)
	loader := config.NewLoader("", false, nil, logger)
	cache := NewPolicyCache(loader, logger)

	// lastUpdated should be zero initially
	stats := cache.GetCacheStats()
	lastUpdated := stats["last_updated"].(time.Time)
	if !lastUpdated.IsZero() {
		t.Error("initial lastUpdated is not zero")
	}

	// Update a policy
	policy := &config.RoutingPolicy{
		PolicyID:       "policy-1",
		OrganizationID: "org-1",
		Model:          "model-1",
	}

	beforeUpdate := time.Now()
	time.Sleep(10 * time.Millisecond)
	cache.UpdatePolicy(policy)
	time.Sleep(10 * time.Millisecond)
	afterUpdate := time.Now()

	// lastUpdated should be set
	stats = cache.GetCacheStats()
	lastUpdated = stats["last_updated"].(time.Time)

	if lastUpdated.Before(beforeUpdate) || lastUpdated.After(afterUpdate) {
		t.Errorf("lastUpdated = %v, want between %v and %v", lastUpdated, beforeUpdate, afterUpdate)
	}
}
