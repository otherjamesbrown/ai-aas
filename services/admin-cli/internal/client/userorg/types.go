// Package userorg provides types for user-org-service API client.
//
// Purpose:
//
//	Define request and response types for user-org-service API operations.
//
// Requirements Reference:
//   - specs/009-admin-cli/plan.md#client/userorg/types
//
package userorg

// BootstrapRequest represents a bootstrap operation request.
type BootstrapRequest struct {
	Email       string `json:"email"`
	DisplayName string `json:"displayName,omitempty"`
	Password    string `json:"password,omitempty"` // Optional - service may generate
	OrgName     string `json:"orgName,omitempty"`  // Organization name for first admin
	OrgSlug     string `json:"orgSlug,omitempty"`  // Organization slug for first admin
}

// BootstrapResponse represents a bootstrap operation response.
type BootstrapResponse struct {
	AdminID string `json:"adminId"`
	OrgID   string `json:"orgId"`
	APIKey  string `json:"apiKey"`
	Email   string `json:"email"`
	OrgName string `json:"orgName,omitempty"`
}

// CreateOrgRequest represents organization creation request.
type CreateOrgRequest struct {
	Name              string                 `json:"name"`
	Slug              string                 `json:"slug"`
	BillingOwnerEmail string                 `json:"billingOwnerEmail,omitempty"`
	Declarative       *DeclarativeConfig     `json:"declarative,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

// DeclarativeConfig represents declarative GitOps configuration.
type DeclarativeConfig struct {
	Enabled bool   `json:"enabled"`
	RepoURL string `json:"repoUrl,omitempty"`
	Branch  string `json:"branch,omitempty"`
}

// OrganizationResponse represents an organization in API responses.
type OrganizationResponse struct {
	OrgID     string                 `json:"orgId"`
	Name      string                 `json:"name"`
	Slug      string                 `json:"slug"`
	Status    string                 `json:"status"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt string                 `json:"createdAt"`
	UpdatedAt string                 `json:"updatedAt"`
}

// InviteUserRequest represents user invite request.
type InviteUserRequest struct {
	Email          string   `json:"email"`
	Roles          []string `json:"roles,omitempty"`
	ExpiresInHours int      `json:"expiresInHours,omitempty"`
}

// InviteResponse represents an invite in API responses.
type InviteResponse struct {
	InviteID  string `json:"inviteId"`
	Email     string `json:"email"`
	Status    string `json:"status"`
	ExpiresAt string `json:"expiresAt"`
}

// UserResponse represents a user in API responses.
type UserResponse struct {
	UserID      string                 `json:"userId"`
	OrgID       string                 `json:"orgId"`
	Email       string                 `json:"email"`
	DisplayName string                 `json:"displayName"`
	Status      string                 `json:"status"`
	MFAEnrolled bool                   `json:"mfaEnrolled"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   string                 `json:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

// IssueAPIKeyRequest represents API key creation request.
type IssueAPIKeyRequest struct {
	Scopes        []string               `json:"scopes,omitempty"`
	ExpiresInDays *int                   `json:"expiresInDays,omitempty"`
	Annotations   map[string]interface{} `json:"annotations,omitempty"`
}

// IssuedAPIKeyResponse represents an issued API key (secret shown once).
type IssuedAPIKeyResponse struct {
	APIKeyID    string `json:"apiKeyId"`
	Secret      string `json:"secret"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
}

// RotateAPIKeyResponse represents rotated API key response.
type RotateAPIKeyResponse struct {
	APIKeyID    string `json:"apiKeyId"`
	Secret      string `json:"secret"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
}

// UpdateOrgRequest represents organization update request.
type UpdateOrgRequest struct {
	DisplayName    *string                `json:"displayName,omitempty"`
	Status         *string                `json:"status,omitempty"`
	BudgetPolicyID *string                `json:"budgetPolicyId,omitempty"`
	Declarative    *DeclarativeConfig     `json:"declarative,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

// UpdateUserRequest represents user update request.
type UpdateUserRequest struct {
	DisplayName *string                `json:"displayName,omitempty"`
	Status      *string                `json:"status,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// APIKeyResponse represents an API key in API responses.
type APIKeyResponse struct {
	APIKeyID    string                 `json:"apiKeyId"`
	UserID      string                 `json:"userId,omitempty"`
	Fingerprint string                 `json:"fingerprint"`
	Status      string                 `json:"status"`
	Scopes      []string               `json:"scopes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   string                 `json:"createdAt"`
	ExpiresAt   string                 `json:"expiresAt,omitempty"`
}

