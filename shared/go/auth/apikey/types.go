package apikey

// AuthenticatedContext contains authentication and authorization context
// extracted from a validated API key.
type AuthenticatedContext struct {
	// APIKeyID is the unique identifier for the API key.
	APIKeyID string `json:"apiKeyId"`

	// OrganizationID is the organization that owns this API key.
	OrganizationID string `json:"organizationId"`

	// PrincipalID is the user or service account ID associated with this key.
	PrincipalID string `json:"principalId"`

	// PrincipalType is either "user" or "service_account".
	PrincipalType string `json:"principalType"`

	// Scopes are the permission scopes granted to this key.
	Scopes []string `json:"scopes"`

	// ModelAccessMode is "restricted" or "auto_grant" (Spec 022).
	ModelAccessMode string `json:"modelAccessMode,omitempty"`

	// GrantedModels is the list of model names accessible when mode is "restricted".
	GrantedModels []string `json:"grantedModels,omitempty"`

	// HMACSecret is the derived secret for request signing verification.
	// This is only populated when HMAC verification is enabled.
	HMACSecret string `json:"-"` // Omit from JSON serialization for security
}

// Principal types
const (
	PrincipalTypeUser           = "user"
	PrincipalTypeServiceAccount = "service_account"
)

// Model access modes (Spec 022)
const (
	ModelAccessModeAutoGrant  = "auto_grant"
	ModelAccessModeRestricted = "restricted"
)

// CanAccessModel checks if the authenticated principal has access to a specific model.
// Returns true if:
// - ModelAccessMode is empty or "auto_grant" (all models allowed)
// - ModelAccessMode is "restricted" and the model is in GrantedModels
func (c *AuthenticatedContext) CanAccessModel(modelName string) bool {
	if c == nil {
		return false
	}

	// auto_grant or unset mode allows all models
	if c.ModelAccessMode == "" || c.ModelAccessMode == ModelAccessModeAutoGrant {
		return true
	}

	// restricted mode requires explicit grant
	if c.ModelAccessMode == ModelAccessModeRestricted {
		for _, grantedModel := range c.GrantedModels {
			if grantedModel == modelName {
				return true
			}
		}
		return false
	}

	// Unknown mode - default to deny (fail closed for security)
	return false
}

// HasScope checks if the context has a specific scope.
func (c *AuthenticatedContext) HasScope(scope string) bool {
	if c == nil {
		return false
	}
	for _, s := range c.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

// HasAnyScope checks if the context has any of the specified scopes.
func (c *AuthenticatedContext) HasAnyScope(scopes ...string) bool {
	if c == nil || len(scopes) == 0 {
		return false
	}

	scopesToCheck := make(map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		scopesToCheck[scope] = struct{}{}
	}

	for _, s := range c.Scopes {
		if _, ok := scopesToCheck[s]; ok {
			return true
		}
	}

	return false
}

// IsUser returns true if the principal type is "user".
func (c *AuthenticatedContext) IsUser() bool {
	return c != nil && c.PrincipalType == PrincipalTypeUser
}

// IsServiceAccount returns true if the principal type is "service_account".
func (c *AuthenticatedContext) IsServiceAccount() bool {
	return c != nil && c.PrincipalType == PrincipalTypeServiceAccount
}

// ValidateAPIKeyRequest represents the request to validate an API key.
type ValidateAPIKeyRequest struct {
	// APIKeySecret is the API key to validate.
	APIKeySecret string `json:"apiKeySecret"`

	// OrgID is an optional organization ID or slug for optimization.
	OrgID string `json:"orgId,omitempty"`
}

// ValidateAPIKeyResponse represents the response after validating an API key.
type ValidateAPIKeyResponse struct {
	// Valid indicates whether the API key is valid.
	Valid bool `json:"valid"`

	// APIKeyID is the unique identifier for the validated key.
	APIKeyID string `json:"apiKeyId,omitempty"`

	// OrganizationID is the organization that owns this API key.
	OrganizationID string `json:"organizationId,omitempty"`

	// PrincipalID is the user or service account ID associated with this key.
	PrincipalID string `json:"principalId,omitempty"`

	// PrincipalType is either "user" or "service_account".
	PrincipalType string `json:"principalType,omitempty"`

	// Scopes are the permission scopes granted to this key.
	Scopes []string `json:"scopes,omitempty"`

	// Status is the key status (e.g., "active", "revoked").
	Status string `json:"status,omitempty"`

	// ExpiresAt is the expiration time in RFC3339 format.
	ExpiresAt *string `json:"expiresAt,omitempty"`

	// Message provides additional context (e.g., why validation failed).
	Message string `json:"message,omitempty"`

	// ModelAccessMode is "restricted" or "auto_grant" (Spec 022).
	ModelAccessMode string `json:"modelAccessMode,omitempty"`

	// GrantedModels is the list of model names accessible when mode is "restricted".
	GrantedModels []string `json:"grantedModels,omitempty"`

	// HMACSecret is the derived secret for request signing verification.
	HMACSecret string `json:"hmacSecret,omitempty"`
}

// ToAuthenticatedContext converts a successful validation response to an AuthenticatedContext.
// Returns nil if the validation was not successful.
func (r *ValidateAPIKeyResponse) ToAuthenticatedContext() *AuthenticatedContext {
	if r == nil || !r.Valid {
		return nil
	}
	return &AuthenticatedContext{
		APIKeyID:        r.APIKeyID,
		OrganizationID:  r.OrganizationID,
		PrincipalID:     r.PrincipalID,
		PrincipalType:   r.PrincipalType,
		Scopes:          r.Scopes,
		ModelAccessMode: r.ModelAccessMode,
		GrantedModels:   r.GrantedModels,
		HMACSecret:      r.HMACSecret,
	}
}
