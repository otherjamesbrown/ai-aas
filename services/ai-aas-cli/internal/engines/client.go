// Package engines provides a client for inference engine operations via the Admin API.
package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// Client provides inference engine operations
type Client struct {
	api *api.Client
}

// NewClient creates a new engines client
func NewClient(apiClient *api.Client) *Client {
	return &Client{api: apiClient}
}

// Engine represents an inference engine type
type Engine struct {
	ID                    string    `json:"id"`
	Name                  string    `json:"name"`
	DisplayName           string    `json:"display_name"`
	Description           string    `json:"description,omitempty"`
	DefaultVersion        string    `json:"default_version"`
	ProtocolVersions      []string  `json:"protocol_versions"`
	SupportedModelFormats []string  `json:"supported_model_formats"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// EngineVersion represents a specific version of an engine
type EngineVersion struct {
	ID             string    `json:"id"`
	EngineID       string    `json:"engine_id"`
	Version        string    `json:"version"`
	ContainerImage string    `json:"container_image"`
	IsDefault      bool      `json:"is_default"`
	IsDeprecated   bool      `json:"is_deprecated"`
	MinGPUMemoryGB *int      `json:"min_gpu_memory_gb,omitempty"`
	ReleaseNotes   string    `json:"release_notes,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// EngineConfig represents a named resource profile for an engine
type EngineConfig struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	EngineID              string          `json:"engine_id"`
	EngineName            string          `json:"engine_name"`
	VersionID             string          `json:"version_id,omitempty"`
	Version               string          `json:"version,omitempty"`
	Description           string          `json:"description,omitempty"`
	GPUCount              int             `json:"gpu_count"`
	GPUType               string          `json:"gpu_type,omitempty"`
	MemoryGB              int             `json:"memory_gb"`
	CPUCores              int             `json:"cpu_cores"`
	EngineArgs            json.RawMessage `json:"engine_args"`
	MinReplicas           int             `json:"min_replicas"`
	MaxReplicas           int             `json:"max_replicas"`
	StartupTimeoutSeconds int             `json:"startup_timeout_seconds"`
	IsDefault             bool            `json:"is_default"`
	CreatedAt             time.Time       `json:"created_at"`
	UpdatedAt             time.Time       `json:"updated_at"`
}

// ListEngines returns all available engines
func (c *Client) ListEngines(ctx context.Context) ([]Engine, error) {
	var resp struct {
		Engines []Engine `json:"engines"`
	}

	if err := c.api.Get(ctx, "/v1/engines", &resp); err != nil {
		return nil, fmt.Errorf("list engines: %w", err)
	}

	return resp.Engines, nil
}

// GetEngine returns an engine by name
func (c *Client) GetEngine(ctx context.Context, name string) (*Engine, error) {
	var engine Engine

	if err := c.api.Get(ctx, "/v1/engines/"+name, &engine); err != nil {
		return nil, fmt.Errorf("get engine: %w", err)
	}

	return &engine, nil
}

// ListVersions returns all versions for an engine
func (c *Client) ListVersions(ctx context.Context, engineName string) ([]EngineVersion, error) {
	var resp struct {
		Versions []EngineVersion `json:"versions"`
	}

	if err := c.api.Get(ctx, "/v1/engines/"+engineName+"/versions", &resp); err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}

	return resp.Versions, nil
}

// AddVersionRequest contains data for adding an engine version
type AddVersionRequest struct {
	Version        string `json:"version"`
	ContainerImage string `json:"container_image"`
	SetDefault     bool   `json:"set_default"`
	MinGPUMemoryGB *int   `json:"min_gpu_memory_gb,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
}

// AddVersion adds a new version for an engine
func (c *Client) AddVersion(ctx context.Context, engineName string, req AddVersionRequest) (*EngineVersion, error) {
	var version EngineVersion

	if err := c.api.Post(ctx, "/v1/engines/"+engineName+"/versions", req, &version); err != nil {
		return nil, fmt.Errorf("add version: %w", err)
	}

	return &version, nil
}

// SetDefaultVersion sets the default version for an engine
func (c *Client) SetDefaultVersion(ctx context.Context, engineName, version string) error {
	path := fmt.Sprintf("/v1/engines/%s/versions/%s/set-default", engineName, version)
	if err := c.api.Post(ctx, path, nil, nil); err != nil {
		return fmt.Errorf("set default version: %w", err)
	}
	return nil
}

// DeleteVersion removes a version from an engine
func (c *Client) DeleteVersion(ctx context.Context, engineName, version string) error {
	path := fmt.Sprintf("/v1/engines/%s/versions/%s", engineName, version)
	if err := c.api.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete version: %w", err)
	}
	return nil
}

// ListConfigs returns all engine configs
func (c *Client) ListConfigs(ctx context.Context) ([]EngineConfig, error) {
	var resp struct {
		Configs []EngineConfig `json:"configs"`
	}

	if err := c.api.Get(ctx, "/v1/engine-configs", &resp); err != nil {
		return nil, fmt.Errorf("list configs: %w", err)
	}

	return resp.Configs, nil
}

// GetConfig returns a config by name
func (c *Client) GetConfig(ctx context.Context, name string) (*EngineConfig, error) {
	var config EngineConfig

	if err := c.api.Get(ctx, "/v1/engine-configs/"+name, &config); err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}

	return &config, nil
}

// CreateConfigRequest contains data for creating an engine config
type CreateConfigRequest struct {
	Name                  string                 `json:"name"`
	EngineName            string                 `json:"engine"`
	Version               string                 `json:"version,omitempty"`
	Description           string                 `json:"description,omitempty"`
	GPUCount              int                    `json:"gpu_count"`
	GPUType               string                 `json:"gpu_type,omitempty"`
	MemoryGB              int                    `json:"memory_gb"`
	CPUCores              int                    `json:"cpu_cores"`
	EngineArgs            map[string]interface{} `json:"engine_args,omitempty"`
	MinReplicas           int                    `json:"min_replicas"`
	MaxReplicas           int                    `json:"max_replicas"`
	StartupTimeoutSeconds int                    `json:"startup_timeout_seconds"`
	IsDefault             bool                   `json:"is_default"`
}

// CreateConfig creates a new engine config
func (c *Client) CreateConfig(ctx context.Context, req CreateConfigRequest) (*EngineConfig, error) {
	var config EngineConfig

	if err := c.api.Post(ctx, "/v1/engine-configs", req, &config); err != nil {
		return nil, fmt.Errorf("create config: %w", err)
	}

	return &config, nil
}

// UpdateConfigRequest contains data for updating an engine config
type UpdateConfigRequest struct {
	Description           *string                `json:"description,omitempty"`
	GPUCount              *int                   `json:"gpu_count,omitempty"`
	GPUType               *string                `json:"gpu_type,omitempty"`
	MemoryGB              *int                   `json:"memory_gb,omitempty"`
	CPUCores              *int                   `json:"cpu_cores,omitempty"`
	EngineArgs            map[string]interface{} `json:"engine_args,omitempty"`
	MinReplicas           *int                   `json:"min_replicas,omitempty"`
	MaxReplicas           *int                   `json:"max_replicas,omitempty"`
	StartupTimeoutSeconds *int                   `json:"startup_timeout_seconds,omitempty"`
	IsDefault             *bool                  `json:"is_default,omitempty"`
}

// UpdateConfig updates an existing engine config
func (c *Client) UpdateConfig(ctx context.Context, name string, req UpdateConfigRequest) (*EngineConfig, error) {
	var config EngineConfig

	if err := c.api.Put(ctx, "/v1/engine-configs/"+name, req, &config); err != nil {
		return nil, fmt.Errorf("update config: %w", err)
	}

	return &config, nil
}

// DeleteConfig removes an engine config
func (c *Client) DeleteConfig(ctx context.Context, name string) error {
	if err := c.api.Delete(ctx, "/v1/engine-configs/"+name); err != nil {
		return fmt.Errorf("delete config: %w", err)
	}
	return nil
}
