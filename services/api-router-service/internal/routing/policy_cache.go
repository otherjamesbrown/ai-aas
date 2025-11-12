// Package routing provides routing policy cache with Config Service watch updates.
//
// Purpose:
//   This package provides a routing-focused interface to the configuration loader,
//   caching routing policies and handling watch updates for real-time policy changes.
//
// Key Responsibilities:
//   - Cache routing policies for fast lookups
//   - Handle Config Service watch updates
//   - Provide routing-specific policy queries
//   - Invalidate cache on policy updates
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#US-003 (Intelligent routing and fallback)
//   - specs/006-api-router-service/spec.md#FR-009 (Configurable routing policies)
//
package routing

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/config"
)

// PolicyCache provides a routing-focused cache for routing policies.
type PolicyCache struct {
	loader      *config.Loader
	logger      *zap.Logger
	policies    map[string]*config.RoutingPolicy
	mu          sync.RWMutex
	lastUpdated time.Time
}

// NewPolicyCache creates a new routing policy cache.
func NewPolicyCache(loader *config.Loader, logger *zap.Logger) *PolicyCache {
	return &PolicyCache{
		loader:   loader,
		logger:   logger,
		policies: make(map[string]*config.RoutingPolicy),
	}
}

// GetPolicy retrieves a routing policy for the given organization and model.
// First checks cache, then falls back to config loader.
func (c *PolicyCache) GetPolicy(organizationID, model string) (*config.RoutingPolicy, error) {
	// Try cache first
	cacheKey := c.cacheKey(organizationID, model)
	c.mu.RLock()
	if policy, exists := c.policies[cacheKey]; exists {
		c.mu.RUnlock()
		return policy, nil
	}
	c.mu.RUnlock()

	// Cache miss - load from config loader
	policy, err := c.loader.GetPolicy(organizationID, model)
	if err != nil {
		return nil, err
	}

	// Store in cache
	c.mu.Lock()
	c.policies[cacheKey] = policy
	c.lastUpdated = time.Now()
	c.mu.Unlock()

	return policy, nil
}

// UpdatePolicy updates a policy in the cache.
func (c *PolicyCache) UpdatePolicy(policy *config.RoutingPolicy) {
	if policy == nil {
		return
	}

	cacheKey := c.cacheKey(policy.OrganizationID, policy.Model)
	c.mu.Lock()
	c.policies[cacheKey] = policy
	c.lastUpdated = time.Now()
	c.mu.Unlock()

	c.logger.Info("routing policy updated in cache",
		zap.String("policy_id", policy.PolicyID),
		zap.String("organization_id", policy.OrganizationID),
		zap.String("model", policy.Model),
	)
}

// InvalidatePolicy removes a policy from the cache.
func (c *PolicyCache) InvalidatePolicy(organizationID, model string) {
	cacheKey := c.cacheKey(organizationID, model)
	c.mu.Lock()
	delete(c.policies, cacheKey)
	c.mu.Unlock()

	c.logger.Info("routing policy invalidated",
		zap.String("organization_id", organizationID),
		zap.String("model", model),
	)
}

// InvalidateAll clears all cached policies.
func (c *PolicyCache) InvalidateAll() {
	c.mu.Lock()
	c.policies = make(map[string]*config.RoutingPolicy)
	c.mu.Unlock()

	c.logger.Info("all routing policies invalidated")
}

// StartWatch starts watching for policy updates from Config Service.
// This should be called after the config loader's Watch is started.
func (c *PolicyCache) StartWatch(ctx context.Context) error {
	// The actual watch is handled by the config loader
	// This method can be used to set up additional routing-specific watch handlers
	// For now, we'll just log that watch is active
	c.logger.Info("routing policy cache watch started")
	return nil
}

// GetCacheStats returns cache statistics.
func (c *PolicyCache) GetCacheStats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return map[string]interface{}{
		"policy_count": len(c.policies),
		"last_updated": c.lastUpdated,
	}
}

// cacheKey generates a cache key for an organization and model.
func (c *PolicyCache) cacheKey(organizationID, model string) string {
	return organizationID + ":" + model
}

// HandlePolicyUpdate handles a policy update from the config loader watch.
// This should be called when the config loader receives a watch event.
func (c *PolicyCache) HandlePolicyUpdate(policy *config.RoutingPolicy) {
	c.UpdatePolicy(policy)
}

// HandlePolicyDelete handles a policy deletion from the config loader watch.
func (c *PolicyCache) HandlePolicyDelete(organizationID, model string) {
	c.InvalidatePolicy(organizationID, model)
}

