// Package registry provides a client for model registry operations via the Admin API.
package registry

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// Client provides model registry operations
type Client struct {
	api *api.Client
}

// NewClient creates a new registry client
func NewClient(apiClient *api.Client) *Client {
	return &Client{api: apiClient}
}

// Model represents a registered model
type Model struct {
	ID                     string                 `json:"id"`
	Name                   string                 `json:"name"`
	HFModelID              string                 `json:"hf_model_id"`
	HFRevision             string                 `json:"hf_revision"`
	RequiresAuth           bool                   `json:"requires_auth"`
	IsGated                bool                   `json:"is_gated"`
	LicenseType            string                 `json:"license_type,omitempty"`
	LicenseURL             string                 `json:"license_url,omitempty"`
	LicenseAcceptedAt      *time.Time             `json:"license_accepted_at,omitempty"`
	LicenseAcceptedBy      string                 `json:"license_accepted_by,omitempty"`
	RecommendedGPUMemoryGB int                    `json:"recommended_gpu_memory_gb,omitempty"`
	RecommendedCPUMemoryGB int                    `json:"recommended_cpu_memory_gb,omitempty"`
	PinnedVersion          string                 `json:"pinned_version,omitempty"`
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt              time.Time              `json:"created_at"`
	UpdatedAt              time.Time              `json:"updated_at"`
	CacheStatus            string                 `json:"cache_status,omitempty"`
	DeploymentStatus       string                 `json:"deployment_status,omitempty"`
}

// ListOptions configures model listing
type ListOptions struct {
	Cached      bool
	Deployed    bool
	Orphaned    bool
	Environment string
}

// List returns all registered models
func (c *Client) List(ctx context.Context, opts ListOptions) ([]Model, error) {
	path := "/v1/models"
	params := make([]string, 0)
	
	if opts.Cached {
		params = append(params, "cached=true")
	}
	if opts.Deployed {
		params = append(params, "deployed=true")
	}
	if opts.Orphaned {
		params = append(params, "orphaned=true")
	}
	if opts.Environment != "" {
		params = append(params, "environment="+opts.Environment)
	}
	
	if len(params) > 0 {
		path += "?"
		for i, p := range params {
			if i > 0 {
				path += "&"
			}
			path += p
		}
	}

	var models []Model
	if err := c.api.Get(ctx, path, &models); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}

	return models, nil
}

// Get retrieves a model by name
func (c *Client) Get(ctx context.Context, name string) (*Model, error) {
	var model Model
	if err := c.api.Get(ctx, "/v1/models/"+name, &model); err != nil {
		return nil, fmt.Errorf("get model %s: %w", name, err)
	}
	return &model, nil
}

// AddRequest contains parameters for adding a model
type AddRequest struct {
	HFModelID     string `json:"hf_model_id"`
	Name          string `json:"name"`
	RequiresAuth  bool   `json:"requires_auth"`
	LicenseType   string `json:"license_type,omitempty"`
	AcceptLicense bool   `json:"accept_license"`
	GPUMemoryGB   int    `json:"gpu_memory_gb,omitempty"`
	CPUMemoryGB   int    `json:"cpu_memory_gb,omitempty"`
}

// Add registers a new model
func (c *Client) Add(ctx context.Context, req AddRequest) (*Model, error) {
	var model Model
	if err := c.api.Post(ctx, "/v1/models", req, &model); err != nil {
		return nil, fmt.Errorf("add model: %w", err)
	}
	return &model, nil
}

// Remove deletes a model from the registry
func (c *Client) Remove(ctx context.Context, name string, force bool) error {
	path := "/v1/models/" + name
	if force {
		path += "?force=true"
	}
	if err := c.api.Delete(ctx, path); err != nil {
		return fmt.Errorf("remove model %s: %w", name, err)
	}
	return nil
}

// ModelStatus contains status information for a model
type ModelStatus struct {
	Name         string            `json:"name"`
	Registry     StatusCheck       `json:"registry"`
	Cache        StatusCheck       `json:"cache"`
	Deployments  []DeploymentInfo  `json:"deployments,omitempty"`
}

// StatusCheck represents a single status check result
type StatusCheck struct {
	Status  string `json:"status"` // ok, warning, error, unknown
	Message string `json:"message,omitempty"`
}

// DeploymentInfo contains deployment-specific status
type DeploymentInfo struct {
	Environment string `json:"environment"`
	Status      string `json:"status"`
	Replicas    string `json:"replicas"` // "2/2" format
	Endpoint    string `json:"endpoint,omitempty"`
}

// Status retrieves the status of a model
func (c *Client) Status(ctx context.Context, name string, environment string) (*ModelStatus, error) {
	path := "/v1/models/" + name + "/status"
	if environment != "" {
		path += "?environment=" + environment
	}

	var status ModelStatus
	if err := c.api.Get(ctx, path, &status); err != nil {
		return nil, fmt.Errorf("get model status: %w", err)
	}

	return &status, nil
}

// CacheEntry represents a cached model version
type CacheEntry struct {
	ID                string     `json:"id"`
	ModelID           string     `json:"model_id"`
	Version           string     `json:"version"`
	HFRevision        string     `json:"hf_revision,omitempty"`
	ObjectStoragePath string     `json:"object_storage_path"`
	SizeBytes         int64      `json:"size_bytes,omitempty"`
	FileCount         int        `json:"file_count,omitempty"`
	ChecksumSHA256    string     `json:"checksum_sha256,omitempty"`
	Status            string     `json:"status"`
	CachedAt          time.Time  `json:"cached_at"`
	VerifiedAt        *time.Time `json:"verified_at,omitempty"`
}

// GetCache retrieves cache information for a model
func (c *Client) GetCache(ctx context.Context, name string) ([]CacheEntry, error) {
	var entries []CacheEntry
	if err := c.api.Get(ctx, "/v1/models/"+name+"/cache", &entries); err != nil {
		return nil, fmt.Errorf("get cache for %s: %w", name, err)
	}
	return entries, nil
}

// PinRequest contains parameters for pinning a model version
type PinRequest struct {
	Version string `json:"version"`
}

// Pin pins a model to a specific version
func (c *Client) Pin(ctx context.Context, name string, version string) error {
	req := PinRequest{Version: version}
	if err := c.api.Post(ctx, "/v1/models/"+name+"/pin", req, nil); err != nil {
		return fmt.Errorf("pin model %s: %w", name, err)
	}
	return nil
}

// Unpin removes the version pin from a model
func (c *Client) Unpin(ctx context.Context, name string) error {
	if err := c.api.Post(ctx, "/v1/models/"+name+"/unpin", nil, nil); err != nil {
		return fmt.Errorf("unpin model %s: %w", name, err)
	}
	return nil
}

// Alias represents a model alias
type Alias struct {
	ID          string    `json:"id"`
	AliasName   string    `json:"alias_name"`
	ModelID     string    `json:"model_id"`
	ModelName   string    `json:"model_name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AliasCreateRequest contains parameters for creating an alias
type AliasCreateRequest struct {
	AliasName   string `json:"alias_name"`
	ModelName   string `json:"model_name"`
	Description string `json:"description,omitempty"`
}

// AliasUpdateRequest contains parameters for updating an alias
type AliasUpdateRequest struct {
	ModelName   string `json:"model_name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListAliases returns all model aliases
func (c *Client) ListAliases(ctx context.Context) ([]Alias, error) {
	var aliases []Alias
	if err := c.api.Get(ctx, "/v1/models/aliases", &aliases); err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	return aliases, nil
}

// GetAlias retrieves a specific alias
func (c *Client) GetAlias(ctx context.Context, aliasName string) (*Alias, error) {
	var alias Alias
	if err := c.api.Get(ctx, "/v1/models/aliases/"+aliasName, &alias); err != nil {
		return nil, fmt.Errorf("get alias %s: %w", aliasName, err)
	}
	return &alias, nil
}

// CreateAlias creates a new model alias
func (c *Client) CreateAlias(ctx context.Context, req AliasCreateRequest) (*Alias, error) {
	var alias Alias
	if err := c.api.Post(ctx, "/v1/models/aliases", req, &alias); err != nil {
		return nil, fmt.Errorf("create alias: %w", err)
	}
	return &alias, nil
}

// UpdateAlias updates an existing alias
func (c *Client) UpdateAlias(ctx context.Context, aliasName string, req AliasUpdateRequest) (*Alias, error) {
	var alias Alias
	if err := c.api.Put(ctx, "/v1/models/aliases/"+aliasName, req, &alias); err != nil {
		return nil, fmt.Errorf("update alias %s: %w", aliasName, err)
	}
	return &alias, nil
}

// DeleteAlias deletes an alias
func (c *Client) DeleteAlias(ctx context.Context, aliasName string) error {
	if err := c.api.Delete(ctx, "/v1/models/aliases/"+aliasName); err != nil {
		return fmt.Errorf("delete alias %s: %w", aliasName, err)
	}
	return nil
}

// StateHistoryEntry represents a model state change event
type StateHistoryEntry struct {
	ID          string     `json:"id"`
	Action      string     `json:"action"`
	PerformedBy string     `json:"performed_by"`
	Reason      string     `json:"reason,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	ExecutedAt  time.Time  `json:"executed_at"`
	Environment string     `json:"environment"`
}

// GetHistory retrieves state history for a model
func (c *Client) GetHistory(ctx context.Context, name string, environment string, limit int) ([]StateHistoryEntry, error) {
	path := fmt.Sprintf("/v1/models/%s/history?limit=%d", name, limit)
	if environment != "" {
		path += "&environment=" + environment
	}

	var entries []StateHistoryEntry
	if err := c.api.Get(ctx, path, &entries); err != nil {
		return nil, fmt.Errorf("get history for %s: %w", name, err)
	}
	return entries, nil
}

// RecordStateRequest contains parameters for recording a state change
type RecordStateRequest struct {
	Environment string `json:"environment"`
	Action      string `json:"action"`
	Reason      string `json:"reason,omitempty"`
}

// RecordState records a state change for a model
func (c *Client) RecordState(ctx context.Context, name string, req RecordStateRequest) error {
	if err := c.api.Post(ctx, "/v1/models/"+name+"/history", req, nil); err != nil {
		return fmt.Errorf("record state for %s: %w", name, err)
	}
	return nil
}

