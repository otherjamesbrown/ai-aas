// Package config provides configuration loading with Admin API sync support.
//
// Purpose:
//   This file implements policy synchronization from the Admin API Service.
//   When etcd is not configured, the API Router can fetch routing policies
//   directly from the Admin API's /v1/routing/policies/sync endpoint.
//
// Key Responsibilities:
//   - Fetch routing policies from Admin API
//   - Convert Admin API policies to internal RoutingPolicy format
//   - Periodic background sync
//   - Cache fetched policies in BoltDB
//
package config

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// AdminSyncClient fetches routing policies from Admin API Service.
type AdminSyncClient struct {
	endpoint     string
	apiKey       string
	httpClient   *http.Client
	cache        *Cache
	logger       *zap.Logger
	syncInterval time.Duration
	lastVersion  int
	stopCh       chan struct{}
}

// AdminSyncConfig configures the Admin API sync client.
type AdminSyncConfig struct {
	Endpoint     string        // Admin API endpoint (e.g., "https://admin-api.dev.otherjamesbrown.com")
	APIKey       string        // Admin API key for authentication
	SyncInterval time.Duration // How often to sync (default: 30s)
}

// AdminPolicySyncResponse matches the Admin API /v1/routing/policies/sync response.
type AdminPolicySyncResponse struct {
	Policies     []AdminPolicy     `json:"policies"`
	SyncMetadata AdminSyncMetadata `json:"sync_metadata"`
}

// AdminPolicy represents a routing policy from Admin API.
type AdminPolicy struct {
	PolicyID          string         `json:"policy_id"`
	OrganizationID    string         `json:"organization_id"`
	Model             string         `json:"model"`
	ExternalName      string         `json:"external_name,omitempty"`
	Backends          []AdminBackend `json:"backends"`
	FallbackBackends  []AdminBackend `json:"fallback_backends,omitempty"`
	FailoverThreshold int            `json:"failover_threshold"`
	Enabled           bool           `json:"enabled"`
	Version           int            `json:"version"`
	Deleted           bool           `json:"deleted,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

// AdminBackend represents a backend in Admin API response.
type AdminBackend struct {
	BackendID string `json:"backend_id"`
	Weight    int    `json:"weight"`
}

// AdminSyncMetadata contains sync metadata from Admin API.
type AdminSyncMetadata struct {
	ServerTime            time.Time `json:"server_time"`
	MaxVersion            int       `json:"max_version"`
	NextSyncRecommendedAt time.Time `json:"next_sync_recommended_at"`
}

// NewAdminSyncClient creates a new Admin API sync client.
func NewAdminSyncClient(cfg AdminSyncConfig, cache *Cache, logger *zap.Logger) *AdminSyncClient {
	if cfg.SyncInterval == 0 {
		cfg.SyncInterval = 30 * time.Second
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &AdminSyncClient{
		endpoint:     cfg.Endpoint,
		apiKey:       cfg.APIKey,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		cache:        cache,
		logger:       logger,
		syncInterval: cfg.SyncInterval,
		stopCh:       make(chan struct{}),
	}
}

// Start begins periodic policy sync from Admin API.
func (c *AdminSyncClient) Start(ctx context.Context) error {
	if c.endpoint == "" {
		c.logger.Info("admin sync disabled: no endpoint configured")
		return nil
	}

	// Initial sync
	if err := c.Sync(ctx); err != nil {
		c.logger.Warn("initial admin sync failed", zap.Error(err))
		// Don't fail startup - will retry
	}

	// Start background sync
	go c.runSyncLoop(ctx)

	c.logger.Info("admin sync started",
		zap.String("endpoint", c.endpoint),
		zap.Duration("interval", c.syncInterval),
	)
	return nil
}

// Stop stops the background sync.
func (c *AdminSyncClient) Stop() {
	close(c.stopCh)
}

// runSyncLoop runs periodic sync in the background.
func (c *AdminSyncClient) runSyncLoop(ctx context.Context) {
	ticker := time.NewTicker(c.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			if err := c.Sync(ctx); err != nil {
				c.logger.Warn("admin sync failed", zap.Error(err))
			}
		}
	}
}

// Sync fetches policies from Admin API and stores them in cache.
func (c *AdminSyncClient) Sync(ctx context.Context) error {
	url := fmt.Sprintf("%s/v1/routing/policies/sync?since_version=%d", c.endpoint, c.lastVersion)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin api returned %d: %s", resp.StatusCode, string(body))
	}

	var syncResp AdminPolicySyncResponse
	if err := json.NewDecoder(resp.Body).Decode(&syncResp); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	// Process policies
	updated := 0
	deleted := 0
	for _, adminPolicy := range syncResp.Policies {
		if adminPolicy.Deleted {
			// Handle deletion - currently cache doesn't support delete,
			// but we can store a disabled policy
			deleted++
			continue
		}

		if !adminPolicy.Enabled {
			// Skip disabled policies
			continue
		}

		// Convert to internal format
		policy := c.convertPolicy(&adminPolicy)
		if err := c.cache.StorePolicy(ctx, policy); err != nil {
			c.logger.Warn("failed to store policy",
				zap.String("policy_id", adminPolicy.PolicyID),
				zap.Error(err),
			)
			continue
		}
		updated++
	}

	// Update version watermark
	if syncResp.SyncMetadata.MaxVersion > c.lastVersion {
		c.lastVersion = syncResp.SyncMetadata.MaxVersion
	}

	if updated > 0 || deleted > 0 {
		c.logger.Info("admin sync completed",
			zap.Int("updated", updated),
			zap.Int("deleted", deleted),
			zap.Int("last_version", c.lastVersion),
		)
	}

	return nil
}

// convertPolicy converts an Admin API policy to internal RoutingPolicy format.
func (c *AdminSyncClient) convertPolicy(admin *AdminPolicy) *RoutingPolicy {
	backends := make([]BackendWeight, len(admin.Backends))
	for i, b := range admin.Backends {
		backends[i] = BackendWeight{
			BackendID: b.BackendID,
			Weight:    b.Weight,
		}
	}

	return &RoutingPolicy{
		PolicyID:          admin.PolicyID,
		OrganizationID:    admin.OrganizationID,
		Model:             admin.Model,
		ExternalName:      admin.ExternalName,
		Backends:          backends,
		FailoverThreshold: admin.FailoverThreshold,
		UpdatedAt:         admin.UpdatedAt,
		Version:           int64(admin.Version),
	}
}

// Health checks the Admin API sync health.
func (c *AdminSyncClient) Health(ctx context.Context) error {
	if c.endpoint == "" {
		return nil // Not configured, not an error
	}

	url := fmt.Sprintf("%s/healthz", c.endpoint)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin api health check returned %d", resp.StatusCode)
	}

	return nil
}
