// Package userorg provides types for user-org-service API client.
//
// Purpose:
//
//	Define request and response types for user-org-service API operations.
//
// Requirements Reference:
//   - specs/009-admin-cli/plan.md#client/userorg/types
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
	Notes         string                 `json:"notes,omitempty"` // Human-readable description
	Scopes        []string               `json:"scopes,omitempty"`
	ExpiresInDays *int                   `json:"expiresInDays,omitempty"`
	Annotations   map[string]interface{} `json:"annotations,omitempty"`
}

// IssuedAPIKeyResponse represents an issued API key (token shown once).
type IssuedAPIKeyResponse struct {
	KeyID       string `json:"keyId"`       // Short, unique identifier for the key
	Token       string `json:"token"`       // The secret token - shown only once
	Fingerprint string `json:"fingerprint"` // Hash of token for identification
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
}

// RotateAPIKeyResponse represents rotated API key response.
type RotateAPIKeyResponse struct {
	KeyID       string `json:"keyId"`       // Short, unique identifier for the key
	Token       string `json:"token"`       // The secret token - shown only once
	Fingerprint string `json:"fingerprint"` // Hash of token for identification
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
	KeyID       string                 `json:"keyId"`           // Short, unique identifier for the key
	Notes       string                 `json:"notes,omitempty"` // Human-readable description
	Fingerprint string                 `json:"fingerprint"`     // Hash of token for identification
	Status      string                 `json:"status"`
	Scopes      []string               `json:"scopes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt   string                 `json:"issuedAt"` // API returns issuedAt not createdAt
	ExpiresAt   string                 `json:"expiresAt,omitempty"`
}

// BudgetStatusResponse represents budget status for an organization.
type BudgetStatusResponse struct {
	OrgID             string `json:"orgId"`
	BudgetLimitCents  int64  `json:"budgetLimitCents"`
	CurrentUsageCents int64  `json:"currentUsageCents"`
	RemainingCents    int64  `json:"remainingCents"`
	Status            string `json:"status"` // "ok", "warning", "exceeded", "unknown"
	PeriodStart       string `json:"periodStart,omitempty"`
	PeriodEnd         string `json:"periodEnd,omitempty"`
}

// CreateUserRequest represents direct user creation request (bypasses invite flow).
type CreateUserRequest struct {
	Email          string   `json:"email"`
	DisplayName    string   `json:"displayName,omitempty"`
	Roles          []string `json:"roles,omitempty"`
	ForcePwdChange *bool    `json:"forcePwdChange,omitempty"` // If true, user must change password on first login
}

// CreateUserResponse represents direct user creation response with temporary password.
type CreateUserResponse struct {
	UserID            string `json:"userId"`
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	Status            string `json:"status"`
	TemporaryPassword string `json:"temporaryPassword"`
	CreatedAt         string `json:"createdAt"`
}

// BootstrapKeyRequest represents a request to create a bootstrap key.
type BootstrapKeyRequest struct {
	OrgID         string `json:"orgId"`
	ExpiresInDays int    `json:"expiresInDays,omitempty"` // Default 7 days
	Notes         string `json:"notes,omitempty"`
}

// BootstrapKeyResponse represents a bootstrap key in API responses.
type BootstrapKeyResponse struct {
	KeyID      string `json:"keyId"`
	OrgID      string `json:"orgId"`
	OrgName    string `json:"orgName,omitempty"`
	Status     string `json:"status"` // active, revoked, expired, redeemed
	Notes      string `json:"notes,omitempty"`
	CreatedAt  string `json:"createdAt"`
	ExpiresAt  string `json:"expiresAt"`
	RedeemedAt string `json:"redeemedAt,omitempty"`
	RedeemedBy string `json:"redeemedBy,omitempty"`
}

// BootstrapKeyCreatedResponse represents a newly created bootstrap key (token shown once).
type BootstrapKeyCreatedResponse struct {
	KeyID     string `json:"keyId"`
	Token     string `json:"token"` // bsk_xxxx - shown only once
	OrgID     string `json:"orgId"`
	OrgName   string `json:"orgName,omitempty"`
	ExpiresAt string `json:"expiresAt"`
}

// BootstrapKeyListResponse represents the response from listing bootstrap keys.
type BootstrapKeyListResponse struct {
	Keys []BootstrapKeyResponse `json:"keys"`
}

// InspectAPIKeyRequest represents an API key inspection request.
type InspectAPIKeyRequest struct {
	Key string `json:"key"` // The API key secret to inspect
}

// InspectAPIKeyResponse represents an API key inspection response.
type InspectAPIKeyResponse struct {
	KeyID         string   `json:"keyId"`
	OrgID         string   `json:"orgId"`
	OrgSlug       string   `json:"orgSlug"`
	PrincipalType string   `json:"principalType"` // "user" or "service_account"
	PrincipalID   string   `json:"principalId"`
	Status        string   `json:"status"` // "active", "revoked", "expired"
	Scopes        []string `json:"scopes"`
	CreatedAt     string   `json:"createdAt"`
	ExpiresAt     string   `json:"expiresAt,omitempty"`
	LastUsedAt    string   `json:"lastUsedAt,omitempty"`
}

// ---- Token Rate-Limit Policy Types ----

// TokenRateLimitPolicyResponse represents a token rate-limit policy in API responses.
type TokenRateLimitPolicyResponse struct {
	ID          string  `json:"id"`
	OrgID       *string `json:"org_id,omitempty"`
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
	IsBuiltin   bool    `json:"is_builtin"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

// TokenPolicyListResponse represents the response from listing token policies.
type TokenPolicyListResponse struct {
	Policies []TokenRateLimitPolicyResponse `json:"policies"`
}

// CreateTokenPolicyRequest represents a request to create a token rate-limit policy.
type CreateTokenPolicyRequest struct {
	Name        string  `json:"name"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
}

// UpdateTokenPolicyRequest represents a request to update a token rate-limit policy.
type UpdateTokenPolicyRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	Limit1h     *int64  `json:"limit_1h,omitempty"`
	Limit24h    *int64  `json:"limit_24h,omitempty"`
	Limit7d     *int64  `json:"limit_7d,omitempty"`
}

// SetTokenPolicyRequest represents a request to set a default or override policy.
type SetTokenPolicyRequest struct {
	PolicyID string `json:"policy_id"`
}

// EffectiveTokenPolicyResponse represents a user's effective token policy.
type EffectiveTokenPolicyResponse struct {
	Policy TokenRateLimitPolicyResponse `json:"policy"`
	Source string                       `json:"source"` // "override" or "inherited"
}

// TokenUsageResponse represents token usage in API responses.
type TokenUsageResponse struct {
	UserID     string                     `json:"user_id,omitempty"`
	OrgID      string                     `json:"org_id,omitempty"`
	PolicyID   string                     `json:"policy_id,omitempty"`
	PolicyName string                     `json:"policy_name"`
	Windows    []TokenUsageWindowResponse `json:"windows"`
}

// TokenUsageWindowResponse represents a usage window in API responses.
type TokenUsageWindowResponse struct {
	Window     string  `json:"window"` // "1h", "24h", "7d"
	Limit      int64   `json:"limit"`
	Used       int64   `json:"used"`
	Remaining  int64   `json:"remaining"`
	Percentage float64 `json:"percentage"`
	ResetsAt   string  `json:"resets_at"`
}
