package domain

import (
	"time"
)

// ValidationError represents a field-level validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// RoutingPolicy represents a routing policy for model traffic distribution
type RoutingPolicy struct {
	PolicyID          string                 `json:"policy_id"`
	OrganizationID    string                 `json:"organization_id"`
	Model             string                 `json:"model"`
	ExternalName      string                 `json:"external_name,omitempty"` // Name exposed in OpenAI API
	Backends          []Backend              `json:"backends"`
	FallbackBackends  []Backend              `json:"fallback_backends,omitempty"`
	FailoverThreshold int                    `json:"failover_threshold"`
	Enabled           bool                   `json:"enabled"`
	Version           int                    `json:"version"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	BackendType       string                 `json:"backend_type,omitempty"`  // "openai" (default) | "triton"
	Tokenizer         string                 `json:"tokenizer,omitempty"`     // tiktoken encoding name (e.g., cl100k_base, llama3)
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
	CreatedBy         *string                `json:"created_by,omitempty"`
	UpdatedBy         *string                `json:"updated_by,omitempty"`
	DeletedAt         *time.Time             `json:"deleted_at,omitempty"`
}

// Backend represents a backend in a routing policy
type Backend struct {
	BackendID    string  `json:"backend_id"`
	Weight       int     `json:"weight"`
	HealthStatus *string `json:"health_status,omitempty"`
	Endpoint     *string `json:"endpoint,omitempty"`
}

// PolicyCreate represents a request to create a routing policy
type PolicyCreate struct {
	PolicyID          string                 `json:"policy_id,omitempty"`
	OrganizationID    string                 `json:"organization_id,omitempty"`
	Model             string                 `json:"model"`
	Backends          []Backend              `json:"backends"`
	FallbackBackends  []Backend              `json:"fallback_backends,omitempty"`
	FailoverThreshold int                    `json:"failover_threshold,omitempty"`
	Enabled           *bool                  `json:"enabled,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	BackendType       string                 `json:"backend_type,omitempty"` // "openai" (default) | "triton"
	Tokenizer         string                 `json:"tokenizer,omitempty"`    // tiktoken encoding name
}

// PolicyUpdate represents a partial policy update
type PolicyUpdate struct {
	Backends          []Backend              `json:"backends,omitempty"`
	FallbackBackends  []Backend              `json:"fallback_backends,omitempty"`
	FailoverThreshold *int                   `json:"failover_threshold,omitempty"`
	Enabled           *bool                  `json:"enabled,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	BackendType       *string                `json:"backend_type,omitempty"` // "openai" (default) | "triton"
	Tokenizer         *string                `json:"tokenizer,omitempty"`    // tiktoken encoding name
}

// PolicyListParams represents query parameters for listing policies
type PolicyListParams struct {
	Model          string
	OrganizationID string
	Enabled        *bool
	BackendID      string
	Limit          int
	Offset         int
}

// PolicyListResponse represents a paginated list of policies
type PolicyListResponse struct {
	Policies   []RoutingPolicy `json:"policies"`
	Pagination Pagination      `json:"pagination"`
}

// PolicySyncResponse represents the response for the sync endpoint
type PolicySyncResponse struct {
	Policies     []PolicySyncItem `json:"policies"`
	SyncMetadata SyncMetadata     `json:"sync_metadata"`
}

// PolicySyncItem represents a policy in the sync response
type PolicySyncItem struct {
	RoutingPolicy
	Deleted bool `json:"deleted,omitempty"`
}

// SyncMetadata contains sync-related metadata
type SyncMetadata struct {
	ServerTime            time.Time `json:"server_time"`
	MaxVersion            int       `json:"max_version"`
	NextSyncRecommendedAt time.Time `json:"next_sync_recommended_at"`
}

// PolicyValidationResponse represents the response from policy validation
type PolicyValidationResponse struct {
	Valid           bool              `json:"valid"`
	Errors          []ValidationError `json:"errors,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
	EstimatedImpact *EstimatedImpact  `json:"estimated_impact,omitempty"`
}

// EstimatedImpact contains impact estimation for a policy
type EstimatedImpact struct {
	AffectedOrganizations    []string `json:"affected_organizations,omitempty"`
	EstimatedRequestsPerHour int      `json:"estimated_requests_per_hour,omitempty"`
}

// PolicyActivationResponse represents the response from activate/deactivate
type PolicyActivationResponse struct {
	PolicyID  string    `json:"policy_id"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updated_at"`
	Message   string    `json:"message"`
}

// ValidateBackendWeights checks if backend weights sum to 100
func ValidateBackendWeights(backends []Backend) bool {
	if len(backends) == 0 {
		return false
	}
	sum := 0
	for _, b := range backends {
		sum += b.Weight
	}
	return sum == 100
}

