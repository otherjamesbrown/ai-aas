package domain

import (
	"time"

	"github.com/google/uuid"
)

// Model represents a registered model deployment
type Model struct {
	ModelID               uuid.UUID              `json:"model_id"`
	ModelName             string                 `json:"model_name"`
	DeploymentEndpoint    *string                `json:"deployment_endpoint,omitempty"`
	DeploymentEnvironment string                 `json:"deployment_environment"`
	DeploymentNamespace   string                 `json:"deployment_namespace"`
	DeploymentStatus      string                 `json:"deployment_status"`
	DeploymentTarget      string                 `json:"deployment_target,omitempty"`
	Revision              int                    `json:"revision"`
	CostPer1kTokens       *float64               `json:"cost_per_1k_tokens,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
}

// ModelRegistration represents a request to register a model
type ModelRegistration struct {
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
	PolicyConfig          *PolicyConfig          `json:"policy_config,omitempty"`
}

// PolicyConfig for auto-creating routing policy
type PolicyConfig struct {
	OrganizationID    string                 `json:"organization_id,omitempty"`
	Weight            int                    `json:"weight,omitempty"`
	FailoverThreshold int                    `json:"failover_threshold,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// ModelUpdate represents a partial model update
type ModelUpdate struct {
	DeploymentEndpoint *string                `json:"deployment_endpoint,omitempty"`
	DeploymentStatus   *string                `json:"deployment_status,omitempty"`
	DeploymentTarget   *string                `json:"deployment_target,omitempty"`
	Revision           *int                   `json:"revision,omitempty"`
	CostPer1kTokens    *float64               `json:"cost_per_1k_tokens,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

// ModelListParams represents query parameters for listing models
type ModelListParams struct {
	Environment string
	Status      string
	Limit       int
	Offset      int
}

// ModelListResponse represents a paginated list of models
type ModelListResponse struct {
	Models     []Model    `json:"models"`
	Pagination Pagination `json:"pagination"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Total      int  `json:"total"`
	Limit      int  `json:"limit"`
	Offset     int  `json:"offset"`
	NextOffset *int `json:"next_offset"`
}

// Valid deployment statuses
var ValidDeploymentStatuses = []string{"ready", "degraded", "offline", "pending"}

// Valid deployment environments
var ValidDeploymentEnvironments = []string{"development", "staging", "production"}

// IsValidStatus checks if a status is valid
func IsValidStatus(status string) bool {
	for _, s := range ValidDeploymentStatuses {
		if s == status {
			return true
		}
	}
	return false
}

// IsValidEnvironment checks if an environment is valid
func IsValidEnvironment(env string) bool {
	for _, e := range ValidDeploymentEnvironments {
		if e == env {
			return true
		}
	}
	return false
}

