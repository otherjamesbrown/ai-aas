// Package deploymentregistry provides an API client for the model deployment registry.
//
// This client communicates with the Admin API's /v1/registry/models endpoints
// for managing model deployments in the routing layer.
package deploymentregistry

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/otherjamesbrown/ai-aas/services/ai-aas-cli/internal/api"
)

// Client provides operations for the model deployment registry.
type Client struct {
	api *api.Client
}

// NewClient creates a new deployment registry client.
func NewClient(apiClient *api.Client) *Client {
	return &Client{api: apiClient}
}

// Model represents a registered model deployment in the registry.
type Model struct {
	ModelID               uuid.UUID              `json:"model_id"`
	ModelName             string                 `json:"model_name"`
	DeploymentEndpoint    string                 `json:"deployment_endpoint"`
	DeploymentEnvironment string                 `json:"deployment_environment"`
	DeploymentNamespace   string                 `json:"deployment_namespace"`
	DeploymentStatus      string                 `json:"deployment_status"`
	DeploymentTarget      string                 `json:"deployment_target,omitempty"`
	Revision              int                    `json:"revision"`
	CostPer1kTokens       *float64               `json:"cost_per_1k_tokens,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	LastHealthCheckAt     *time.Time             `json:"last_health_check_at,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// RegisterRequest represents a request to register a model deployment.
type RegisterRequest struct {
	ModelName             string                 `json:"model_name"`
	DeploymentEndpoint    string                 `json:"deployment_endpoint"`
	DeploymentEnvironment string                 `json:"deployment_environment"`
	DeploymentNamespace   string                 `json:"deployment_namespace"`
	DeploymentStatus      string                 `json:"deployment_status,omitempty"`
	DeploymentTarget      string                 `json:"deployment_target,omitempty"`
	Revision              int                    `json:"revision,omitempty"`
	CostPer1kTokens       *float64               `json:"cost_per_1k_tokens,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	AutoCreatePolicy      bool                   `json:"auto_create_policy,omitempty"`
}

// UpdateRequest represents a partial model update.
type UpdateRequest struct {
	DeploymentEndpoint *string                `json:"deployment_endpoint,omitempty"`
	DeploymentStatus   *string                `json:"deployment_status,omitempty"`
	DeploymentTarget   *string                `json:"deployment_target,omitempty"`
	Revision           *int                   `json:"revision,omitempty"`
	CostPer1kTokens    *float64               `json:"cost_per_1k_tokens,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// ListParams configures model listing.
type ListParams struct {
	Environment string
	Status      string
	Limit       int
	Offset      int
}

// Pagination represents pagination metadata in responses.
type Pagination struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset,omitempty"`
}

// ListResponse represents a paginated list of models.
type ListResponse struct {
	Models     []Model    `json:"models"`
	Pagination Pagination `json:"pagination"`
}

// List retrieves registered models with optional filtering.
func (c *Client) List(ctx context.Context, params ListParams) (*ListResponse, error) {
	query := url.Values{}
	if params.Environment != "" {
		query.Set("environment", params.Environment)
	}
	if params.Status != "" {
		query.Set("status", params.Status)
	}
	if params.Limit > 0 {
		query.Set("limit", strconv.Itoa(params.Limit))
	}
	if params.Offset > 0 {
		query.Set("offset", strconv.Itoa(params.Offset))
	}

	path := "/v1/registry/models"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}

	var resp ListResponse
	if err := c.api.Get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("list registry models: %w", err)
	}

	return &resp, nil
}

// Get retrieves a single model by name and environment.
func (c *Client) Get(ctx context.Context, modelName, environment string) (*Model, error) {
	query := url.Values{}
	query.Set("environment", environment)

	path := fmt.Sprintf("/v1/registry/models/%s?%s", url.PathEscape(modelName), query.Encode())

	var model Model
	if err := c.api.Get(ctx, path, &model); err != nil {
		return nil, fmt.Errorf("get model %s: %w", modelName, err)
	}

	return &model, nil
}

// Register creates or updates a model in the deployment registry.
func (c *Client) Register(ctx context.Context, req RegisterRequest) (*Model, error) {
	var model Model
	if err := c.api.Post(ctx, "/v1/registry/models", req, &model); err != nil {
		return nil, fmt.Errorf("register model: %w", err)
	}
	return &model, nil
}

// Update updates a model's properties (partial update via PATCH).
func (c *Client) Update(ctx context.Context, modelName, environment string, req UpdateRequest) (*Model, error) {
	query := url.Values{}
	query.Set("environment", environment)

	path := fmt.Sprintf("/v1/registry/models/%s?%s", url.PathEscape(modelName), query.Encode())

	var model Model
	if err := c.api.Request(ctx, "PATCH", path, req, &model); err != nil {
		return nil, fmt.Errorf("update model %s: %w", modelName, err)
	}

	return &model, nil
}

// UpdateStatus is a convenience method to update only the deployment status.
func (c *Client) UpdateStatus(ctx context.Context, modelName, environment, status string) (*Model, error) {
	return c.Update(ctx, modelName, environment, UpdateRequest{
		DeploymentStatus: &status,
	})
}

// Delete removes a model from the deployment registry.
func (c *Client) Delete(ctx context.Context, modelName, environment string) error {
	query := url.Values{}
	query.Set("environment", environment)

	path := fmt.Sprintf("/v1/registry/models/%s?%s", url.PathEscape(modelName), query.Encode())

	if err := c.api.Delete(ctx, path); err != nil {
		return fmt.Errorf("delete model %s: %w", modelName, err)
	}

	return nil
}

// Valid deployment statuses
const (
	StatusReady    = "ready"
	StatusDegraded = "degraded"
	StatusOffline  = "offline"
	StatusPending  = "pending"
	StatusDisabled = "disabled"
)

// Valid deployment environments
const (
	EnvDevelopment = "development"
	EnvStaging     = "staging"
	EnvProduction  = "production"
)
