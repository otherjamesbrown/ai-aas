// Package config provides configuration loading and watching for routing policies.
//
// Purpose:
//   This package implements configuration loading from Config Service with watch
//   support for real-time routing policy updates. It provides graceful fallback
//   to cached configuration when Config Service is unavailable.
//
// Dependencies:
//   - internal/config/cache: BoltDB cache for configuration persistence
//   - Config Service: etcd or similar for distributed configuration
//
// Key Responsibilities:
//   - Load initial configuration from Config Service or cache
//   - Watch for routing policy updates
//   - Handle connection failures gracefully
//   - Provide configuration interface for routing engine
//
// Requirements Reference:
//   - specs/006-api-router-service/spec.md#FR-003 (Routing engine)
//   - specs/006-api-router-service/spec.md#FR-009 (Configurable routing policies)
//
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"

	"github.com/otherjamesbrown/ai-aas/services/api-router-service/internal/adapter/triton"
)

// Loader manages configuration loading and watching from Config Service.
type Loader struct {
	endpoint     string
	watchEnabled bool
	cache        *Cache
	client       *clientv3.Client
	logger       *zap.Logger
	watchCtx     context.Context
	watchCancel  context.CancelFunc
}

const (
	// etcdKeyPrefix is the prefix for all routing policy keys in etcd
	etcdKeyPrefix = "/api-router/policies"
	// etcdGlobalOrgID is the organization ID used for global policies
	etcdGlobalOrgID = "*"
)

// RoutingPolicy represents a routing policy configuration.
type RoutingPolicy struct {
	PolicyID          string
	OrganizationID    string // "*" for global
	Model             string // Full internal model path (e.g., "unsloth/gpt-oss-20b")
	ExternalName      string // Name exposed in OpenAI API (e.g., "gpt-oss-20b")
	Backends          []BackendWeight
	FailoverThreshold int
	DegradedBackends  []string
	BackendType       string // "openai" (default) | "triton" - protocol for backend communication
	Tokenizer         string // tiktoken encoding name (e.g., cl100k_base, llama3) for token counting
	UpdatedAt         time.Time
	Version           int64
}

// BackendWeight defines a backend with its routing weight.
type BackendWeight struct {
	BackendID string
	Weight    int // Percentage (0-100)
	AllowList []string
	DenyList  []string
}

// NewLoader creates a new configuration loader.
// If logger is nil, a no-op logger will be used.
func NewLoader(endpoint string, watchEnabled bool, cache *Cache, logger *zap.Logger) *Loader {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Loader{
		endpoint:     endpoint,
		watchEnabled: watchEnabled,
		cache:        cache,
		logger:       logger,
	}
}

// connect establishes a connection to etcd.
func (l *Loader) connect(ctx context.Context) error {
	if l.client != nil {
		return nil // Already connected
	}

	cfg := clientv3.Config{
		Endpoints:   []string{l.endpoint},
		DialTimeout: 5 * time.Second,
	}

	client, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("connect to etcd: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err = client.Status(ctx, l.endpoint)
	if err != nil {
		_ = client.Close()
		return fmt.Errorf("etcd status check failed: %w", err)
	}

	l.client = client
	l.logger.Info("connected to etcd", zap.String("endpoint", l.endpoint))
	return nil
}

// close closes the etcd client connection.
func (l *Loader) close() error {
	if l.client != nil {
		err := l.client.Close()
		l.client = nil
		return err
	}
	return nil
}

// Load initializes configuration from Config Service or cache.
// Returns an error if configuration cannot be loaded from either source.
func (l *Loader) Load(ctx context.Context) error {
	// Try to connect to etcd and load policies
	if err := l.connect(ctx); err != nil {
		l.logger.Warn("failed to connect to etcd, falling back to cache", zap.Error(err))
		// Fall through to cache loading
	} else {
		// Load policies from etcd
		policies, err := l.loadPoliciesFromEtcd(ctx)
		if err == nil && len(policies) > 0 {
			// Store policies in cache
			for _, policy := range policies {
				if err := l.cache.StorePolicy(ctx, policy); err != nil {
					l.logger.Warn("failed to store policy in cache", zap.Error(err), zap.String("policy_id", policy.PolicyID))
				}
			}
			l.logger.Info("loaded policies from etcd", zap.Int("count", len(policies)))
			return nil
		}
		if err != nil {
			l.logger.Warn("failed to load policies from etcd", zap.Error(err))
		}
	}

	// Fallback to cache
	if l.cache != nil {
		policies, err := l.cache.LoadPolicies(ctx)
		if err == nil && len(policies) > 0 {
			l.logger.Info("loaded policies from cache", zap.Int("count", len(policies)))
			return nil
		}
		if err != nil {
			l.logger.Warn("failed to load policies from cache", zap.Error(err))
		}
	}

	return fmt.Errorf("config loader: unable to load configuration from etcd or cache")
}

// loadPoliciesFromEtcd loads all routing policies from etcd.
func (l *Loader) loadPoliciesFromEtcd(ctx context.Context) ([]*RoutingPolicy, error) {
	if l.client == nil {
		return nil, fmt.Errorf("etcd client not connected")
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Get all policies under the prefix
	resp, err := l.client.Get(ctx, etcdKeyPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd get: %w", err)
	}

	var policies []*RoutingPolicy
	for _, kv := range resp.Kvs {
		var policy RoutingPolicy
		if err := json.Unmarshal(kv.Value, &policy); err != nil {
			l.logger.Warn("failed to unmarshal policy", zap.Error(err), zap.String("key", string(kv.Key)))
			continue
		}
		policies = append(policies, &policy)
	}

	return policies, nil
}

// Watch starts watching for configuration updates from Config Service.
// Updates are written to cache and can be retrieved via GetPolicy.
func (l *Loader) Watch(ctx context.Context) error {
	if !l.watchEnabled {
		return nil // Watch disabled
	}

	// Ensure etcd connection
	if err := l.connect(ctx); err != nil {
		l.logger.Warn("failed to connect to etcd for watch, watch disabled", zap.Error(err))
		return nil // Don't fail startup if watch can't connect
	}

	l.watchCtx, l.watchCancel = context.WithCancel(ctx)

	// Start etcd watch stream
	go func() {
		defer l.logger.Info("config watch stopped")

		watchChan := l.client.Watch(l.watchCtx, etcdKeyPrefix, clientv3.WithPrefix())
		for {
			select {
			case <-l.watchCtx.Done():
				return
			case watchResp := <-watchChan:
				if watchResp.Err() != nil {
					l.logger.Error("etcd watch error", zap.Error(watchResp.Err()))
					// Try to reconnect after a delay
					time.Sleep(5 * time.Second)
					if err := l.connect(l.watchCtx); err != nil {
						l.logger.Error("failed to reconnect to etcd", zap.Error(err))
					} else {
						// Restart watch
						watchChan = l.client.Watch(l.watchCtx, etcdKeyPrefix, clientv3.WithPrefix())
					}
					continue
				}

				for _, event := range watchResp.Events {
					if err := l.handleWatchEvent(l.watchCtx, event); err != nil {
						l.logger.Error("failed to handle watch event", zap.Error(err))
					}
				}
			}
		}
	}()

	l.logger.Info("started config watch", zap.String("prefix", etcdKeyPrefix))
	return nil
}

// handleWatchEvent processes a single etcd watch event.
func (l *Loader) handleWatchEvent(ctx context.Context, event *clientv3.Event) error {
	switch event.Type {
	case clientv3.EventTypePut:
		// Policy created or updated
		var policy RoutingPolicy
		if err := json.Unmarshal(event.Kv.Value, &policy); err != nil {
			return fmt.Errorf("unmarshal policy: %w", err)
		}
		if err := l.cache.StorePolicy(ctx, &policy); err != nil {
			return fmt.Errorf("store policy in cache: %w", err)
		}
		l.logger.Info("policy updated", zap.String("key", string(event.Kv.Key)), zap.String("policy_id", policy.PolicyID))

	case clientv3.EventTypeDelete:
		// Policy deleted - extract org and model from key to invalidate cache
		key := string(event.Kv.Key)
		parts := strings.Split(strings.TrimPrefix(key, etcdKeyPrefix+"/"), "/")
		if len(parts) >= 2 {
			orgID := parts[0]
			model := parts[1]
			// Note: Cache doesn't have a delete method, but GetPolicy will return nil
			// if the key doesn't exist. For now, we'll just log the deletion.
			l.logger.Info("policy deleted", zap.String("key", key), zap.String("org_id", orgID), zap.String("model", model))
		}

	default:
		l.logger.Warn("unknown watch event type", zap.String("type", event.Type.String()))
	}

	return nil
}

// GetPolicy retrieves a routing policy for the given organization and model.
// Returns nil if no policy is found.
// First checks cache, then etcd if cache miss and etcd is available.
//
// Model name resolution:
//   - First tries exact match (e.g., "unsloth/gpt-oss-20b")
//   - If no "/" in model name, tries external_name lookup (explicit mapping from OpenAI API name)
//   - Falls back to alias lookup by matching models ending with "/<model>"
//     (e.g., "gpt-oss-20b" matches "unsloth/gpt-oss-20b")
func (l *Loader) GetPolicy(organizationID, model string) (*RoutingPolicy, error) {
	// First check cache with exact match
	if l.cache != nil {
		policy, err := l.cache.GetPolicy(organizationID, model)
		if err == nil && policy != nil {
			return policy, nil
		}
		// Try global policy if org-specific not found
		if organizationID != etcdGlobalOrgID {
			policy, err := l.cache.GetPolicy(etcdGlobalOrgID, model)
			if err == nil && policy != nil {
				return policy, nil
			}
		}

		// External name lookup: if model has no "/", try finding a policy by external_name
		if !strings.Contains(model, "/") {
			// Try external name match first (explicit configuration)
			policy, err := l.cache.GetPolicyByExternalName(organizationID, model)
			if err == nil && policy != nil {
				return policy, nil
			}
			if organizationID != etcdGlobalOrgID {
				policy, err := l.cache.GetPolicyByExternalName(etcdGlobalOrgID, model)
				if err == nil && policy != nil {
					return policy, nil
				}
			}

			// Fall back to alias lookup (suffix matching for backwards compatibility)
			policy, err = l.cache.GetPolicyByAlias(organizationID, model)
			if err == nil && policy != nil {
				return policy, nil
			}
			if organizationID != etcdGlobalOrgID {
				policy, err := l.cache.GetPolicyByAlias(etcdGlobalOrgID, model)
				if err == nil && policy != nil {
					return policy, nil
				}
			}
		}
	}

	// Cache miss - try etcd if available
	if l.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		// Try org-specific policy
		policy, err := l.getPolicyFromEtcd(ctx, organizationID, model)
		if err == nil && policy != nil {
			// Store in cache for future lookups
			if l.cache != nil {
				_ = l.cache.StorePolicy(ctx, policy)
			}
			return policy, nil
		}

		// Try global policy if org-specific not found
		if organizationID != etcdGlobalOrgID {
			policy, err := l.getPolicyFromEtcd(ctx, etcdGlobalOrgID, model)
			if err == nil && policy != nil {
				// Store in cache for future lookups
				if l.cache != nil {
					_ = l.cache.StorePolicy(ctx, policy)
				}
				return policy, nil
			}
		}

		// Alias lookup from etcd if model has no "/"
		if !strings.Contains(model, "/") {
			policy, err := l.getPolicyByAliasFromEtcd(ctx, organizationID, model)
			if err == nil && policy != nil {
				if l.cache != nil {
					_ = l.cache.StorePolicy(ctx, policy)
				}
				return policy, nil
			}
			if organizationID != etcdGlobalOrgID {
				policy, err := l.getPolicyByAliasFromEtcd(ctx, etcdGlobalOrgID, model)
				if err == nil && policy != nil {
					if l.cache != nil {
						_ = l.cache.StorePolicy(ctx, policy)
					}
					return policy, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("policy not found for org=%s model=%s", organizationID, model)
}

// getPolicyFromEtcd retrieves a single policy from etcd.
func (l *Loader) getPolicyFromEtcd(ctx context.Context, organizationID, model string) (*RoutingPolicy, error) {
	key := etcdPolicyKey(organizationID, model)

	resp, err := l.client.Get(ctx, key)
	if err != nil {
		return nil, fmt.Errorf("etcd get: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("policy not found")
	}

	var policy RoutingPolicy
	if err := json.Unmarshal(resp.Kvs[0].Value, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy: %w", err)
	}

	return &policy, nil
}

// etcdPolicyKey generates an etcd key for a routing policy.
func etcdPolicyKey(organizationID, model string) string {
	return fmt.Sprintf("%s/%s/%s", etcdKeyPrefix, organizationID, model)
}

// getPolicyByAliasFromEtcd searches for a policy by alias (model name suffix).
// For example, alias "gpt-oss-20b" matches policy with model "unsloth/gpt-oss-20b".
func (l *Loader) getPolicyByAliasFromEtcd(ctx context.Context, organizationID, alias string) (*RoutingPolicy, error) {
	// List all policies for this org and find one matching the alias
	prefix := fmt.Sprintf("%s/%s/", etcdKeyPrefix, organizationID)
	suffix := "/" + alias

	resp, err := l.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd prefix get: %w", err)
	}

	for _, kv := range resp.Kvs {
		var policy RoutingPolicy
		if err := json.Unmarshal(kv.Value, &policy); err != nil {
			continue
		}
		// Check if the model name ends with the alias (e.g., "unsloth/gpt-oss-20b" ends with "/gpt-oss-20b")
		if strings.HasSuffix(policy.Model, suffix) {
			return &policy, nil
		}
	}

	return nil, fmt.Errorf("no policy found with alias %s", alias)
}

// Stop stops watching for configuration updates and closes the etcd connection.
func (l *Loader) Stop() {
	if l.watchCancel != nil {
		l.watchCancel()
	}
	if err := l.close(); err != nil {
		if l.logger != nil {
			l.logger.Warn("error closing etcd client", zap.Error(err))
		}
	}
}

// Health checks the health of the Config Service (etcd) connection.
// Returns nil if healthy, error if unhealthy.
// This method attempts to connect if not already connected, but does not fail
// if etcd is unavailable (graceful degradation with cache fallback).
func (l *Loader) Health(ctx context.Context) error {
	// If endpoint is empty, Config Service is not configured
	if l.endpoint == "" {
		return nil // Not an error - just not configured
	}

	// Try to connect if not already connected
	if l.client == nil {
		if err := l.connect(ctx); err != nil {
			// Connection failed - but cache might still be available
			// This is not a critical failure for readiness
			return fmt.Errorf("etcd connection failed: %w", err)
		}
	}

	// Check connection status
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	_, err := l.client.Status(ctx, l.endpoint)
	if err != nil {
		// Connection lost - try to reconnect
		_ = l.close()
		return fmt.Errorf("etcd status check failed: %w", err)
	}

	return nil
}

// ValidateTritonPolicy validates a routing policy configured for Triton backend.
// This function performs the following validations:
//   - Tokenizer encoding must be valid (strict - returns error if invalid)
//   - Backend health is checked (warning only - logs if unreachable)
//
// This implements spec032 FR-5 (Configuration Validation).
func (l *Loader) ValidateTritonPolicy(ctx context.Context, policy *RoutingPolicy, backendEndpoint string) error {
	if policy.BackendType != "triton" {
		return nil // No validation needed for non-triton backends
	}

	// 1. Validate tokenizer encoding exists (strict - fail if invalid)
	if policy.Tokenizer == "" {
		l.logger.Error("triton policy missing tokenizer",
			zap.String("policy_id", policy.PolicyID),
			zap.String("model", policy.Model),
		)
		return fmt.Errorf("tokenizer encoding is required for triton backend type")
	}

	// Try to create a tokenizer with this encoding
	_, err := triton.NewTokenizer(policy.Tokenizer)
	if err != nil {
		l.logger.Error("triton policy has invalid tokenizer encoding",
			zap.String("policy_id", policy.PolicyID),
			zap.String("model", policy.Model),
			zap.String("tokenizer", policy.Tokenizer),
			zap.Error(err),
		)
		return fmt.Errorf("invalid tokenizer encoding %q: %w", policy.Tokenizer, err)
	}

	l.logger.Debug("triton policy tokenizer validation passed",
		zap.String("policy_id", policy.PolicyID),
		zap.String("tokenizer", policy.Tokenizer),
	)

	// 2. Validate backend health (warning only - logs if unreachable)
	// Only check if a backend endpoint is provided
	if backendEndpoint != "" {
		l.checkTritonBackendHealth(ctx, policy, backendEndpoint)
	}

	return nil
}

// checkTritonBackendHealth checks if a Triton backend is reachable.
// This is a warning-only check - it logs issues but doesn't fail policy sync.
func (l *Loader) checkTritonBackendHealth(ctx context.Context, policy *RoutingPolicy, backendEndpoint string) {
	// Triton V2 health endpoint: /v2/health/ready
	healthURL := strings.TrimSuffix(backendEndpoint, "/") + "/v2/health/ready"

	client := &http.Client{Timeout: 3 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		l.logger.Warn("failed to create triton health check request",
			zap.String("policy_id", policy.PolicyID),
			zap.String("endpoint", backendEndpoint),
			zap.Error(err),
		)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		l.logger.Warn("triton backend unreachable",
			zap.String("policy_id", policy.PolicyID),
			zap.String("model", policy.Model),
			zap.String("endpoint", backendEndpoint),
			zap.Error(err),
		)
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		l.logger.Warn("triton backend unhealthy",
			zap.String("policy_id", policy.PolicyID),
			zap.String("model", policy.Model),
			zap.String("endpoint", backendEndpoint),
			zap.Int("status", resp.StatusCode),
		)
		return
	}

	l.logger.Debug("triton backend health check passed",
		zap.String("policy_id", policy.PolicyID),
		zap.String("endpoint", backendEndpoint),
	)
}

