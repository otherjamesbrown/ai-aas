// Package client provides API clients for Admin API resources.
package client

import (
	"context"
	"fmt"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// DeploymentClient provides operations for model deployments
type DeploymentClient struct {
	api *api.Client
}

// NewDeploymentClient creates a new deployment client
func NewDeploymentClient(apiClient *api.Client) *DeploymentClient {
	return &DeploymentClient{api: apiClient}
}

// Deployment represents a model deployment
type Deployment struct {
	ID                   string     `json:"id"`
	ModelID              string     `json:"model_id"`
	ModelName            string     `json:"model_name,omitempty"`
	ExternalName         string     `json:"external_name,omitempty"`
	CacheID              string     `json:"cache_id,omitempty"`
	Environment          string     `json:"environment"`
	Namespace            string     `json:"namespace"`
	InferenceServiceName string     `json:"inferenceservice_name,omitempty"`
	Endpoint             string     `json:"endpoint,omitempty"`
	Enabled              bool       `json:"enabled"`
	Status               string     `json:"status"`
	StatusChangedAt      *time.Time `json:"status_changed_at,omitempty"`
	ReplicasDesired      int        `json:"replicas_desired"`
	ReplicasReady        int        `json:"replicas_ready"`
	GPUCount             int        `json:"gpu_count"`
	MemoryGB             int        `json:"memory_gb,omitempty"`
	LastHealthCheckAt    *time.Time `json:"last_health_check_at,omitempty"`
	LastHealthStatus     string     `json:"last_health_status,omitempty"`
	LastEnabledAt        *time.Time `json:"last_enabled_at,omitempty"`
	LastEnabledBy        string     `json:"last_enabled_by,omitempty"`
	LastDisabledAt       *time.Time `json:"last_disabled_at,omitempty"`
	LastDisabledBy       string     `json:"last_disabled_by,omitempty"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// ListOptions configures deployment listing
type ListOptions struct {
	Environment string
	ModelName   string
	Status      string
}

// List returns deployments matching the given options
func (c *DeploymentClient) List(ctx context.Context, opts ListOptions) ([]Deployment, error) {
	path := "/v1/deployments"
	params := make([]string, 0)

	if opts.Environment != "" {
		params = append(params, "environment="+opts.Environment)
	}
	if opts.ModelName != "" {
		params = append(params, "model_name="+opts.ModelName)
	}
	if opts.Status != "" {
		params = append(params, "status="+opts.Status)
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

	var deployments []Deployment
	if err := c.api.Get(ctx, path, &deployments); err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}

	return deployments, nil
}

// Get retrieves a deployment by model name and environment
func (c *DeploymentClient) Get(ctx context.Context, modelName, environment string) (*Deployment, error) {
	var deployment Deployment
	path := fmt.Sprintf("/v1/deployments/%s/%s", modelName, environment)
	if err := c.api.Get(ctx, path, &deployment); err != nil {
		return nil, fmt.Errorf("get deployment %s/%s: %w", modelName, environment, err)
	}
	return &deployment, nil
}
