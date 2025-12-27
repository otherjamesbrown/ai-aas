package apikey

import (
	"testing"
)

func TestAuthenticatedContext_CanAccessModel(t *testing.T) {
	tests := []struct {
		name      string
		ctx       *AuthenticatedContext
		modelName string
		expected  bool
	}{
		{
			name:      "nil context",
			ctx:       nil,
			modelName: "gpt-4",
			expected:  false,
		},
		{
			name: "auto_grant mode allows all",
			ctx: &AuthenticatedContext{
				ModelAccessMode: ModelAccessModeAutoGrant,
			},
			modelName: "gpt-4",
			expected:  true,
		},
		{
			name: "empty mode allows all (default)",
			ctx: &AuthenticatedContext{
				ModelAccessMode: "",
			},
			modelName: "gpt-4",
			expected:  true,
		},
		{
			name: "restricted mode with granted model",
			ctx: &AuthenticatedContext{
				ModelAccessMode: ModelAccessModeRestricted,
				GrantedModels:   []string{"gpt-3.5-turbo", "gpt-4"},
			},
			modelName: "gpt-4",
			expected:  true,
		},
		{
			name: "restricted mode without granted model",
			ctx: &AuthenticatedContext{
				ModelAccessMode: ModelAccessModeRestricted,
				GrantedModels:   []string{"gpt-3.5-turbo"},
			},
			modelName: "gpt-4",
			expected:  false,
		},
		{
			name: "restricted mode with empty grants",
			ctx: &AuthenticatedContext{
				ModelAccessMode: ModelAccessModeRestricted,
				GrantedModels:   []string{},
			},
			modelName: "gpt-4",
			expected:  false,
		},
		{
			name: "unknown mode denies (fail closed)",
			ctx: &AuthenticatedContext{
				ModelAccessMode: "unknown_mode",
			},
			modelName: "gpt-4",
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.CanAccessModel(tt.modelName)
			if got != tt.expected {
				t.Errorf("CanAccessModel(%q) = %v, want %v", tt.modelName, got, tt.expected)
			}
		})
	}
}

func TestAuthenticatedContext_HasScope(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *AuthenticatedContext
		scope    string
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			scope:    "read",
			expected: false,
		},
		{
			name: "has scope",
			ctx: &AuthenticatedContext{
				Scopes: []string{"read", "write", "admin"},
			},
			scope:    "write",
			expected: true,
		},
		{
			name: "does not have scope",
			ctx: &AuthenticatedContext{
				Scopes: []string{"read"},
			},
			scope:    "write",
			expected: false,
		},
		{
			name: "empty scopes",
			ctx: &AuthenticatedContext{
				Scopes: []string{},
			},
			scope:    "read",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.HasScope(tt.scope)
			if got != tt.expected {
				t.Errorf("HasScope(%q) = %v, want %v", tt.scope, got, tt.expected)
			}
		})
	}
}

func TestAuthenticatedContext_HasAnyScope(t *testing.T) {
	ctx := &AuthenticatedContext{
		Scopes: []string{"read", "write"},
	}

	tests := []struct {
		name     string
		scopes   []string
		expected bool
	}{
		{
			name:     "has first scope",
			scopes:   []string{"read", "admin"},
			expected: true,
		},
		{
			name:     "has second scope",
			scopes:   []string{"admin", "write"},
			expected: true,
		},
		{
			name:     "has none",
			scopes:   []string{"admin", "super"},
			expected: false,
		},
		{
			name:     "empty input",
			scopes:   []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ctx.HasAnyScope(tt.scopes...)
			if got != tt.expected {
				t.Errorf("HasAnyScope(%v) = %v, want %v", tt.scopes, got, tt.expected)
			}
		})
	}
}

func TestAuthenticatedContext_IsUser(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *AuthenticatedContext
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: false,
		},
		{
			name: "is user",
			ctx: &AuthenticatedContext{
				PrincipalType: PrincipalTypeUser,
			},
			expected: true,
		},
		{
			name: "is service account",
			ctx: &AuthenticatedContext{
				PrincipalType: PrincipalTypeServiceAccount,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.IsUser()
			if got != tt.expected {
				t.Errorf("IsUser() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAuthenticatedContext_IsServiceAccount(t *testing.T) {
	tests := []struct {
		name     string
		ctx      *AuthenticatedContext
		expected bool
	}{
		{
			name:     "nil context",
			ctx:      nil,
			expected: false,
		},
		{
			name: "is service account",
			ctx: &AuthenticatedContext{
				PrincipalType: PrincipalTypeServiceAccount,
			},
			expected: true,
		},
		{
			name: "is user",
			ctx: &AuthenticatedContext{
				PrincipalType: PrincipalTypeUser,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.ctx.IsServiceAccount()
			if got != tt.expected {
				t.Errorf("IsServiceAccount() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidateAPIKeyResponse_ToAuthenticatedContext(t *testing.T) {
	tests := []struct {
		name     string
		response *ValidateAPIKeyResponse
		wantNil  bool
	}{
		{
			name:     "nil response",
			response: nil,
			wantNil:  true,
		},
		{
			name: "invalid response",
			response: &ValidateAPIKeyResponse{
				Valid: false,
			},
			wantNil: true,
		},
		{
			name: "valid response",
			response: &ValidateAPIKeyResponse{
				Valid:           true,
				APIKeyID:        "key-123",
				OrganizationID:  "org-456",
				PrincipalID:     "user-789",
				PrincipalType:   PrincipalTypeUser,
				Scopes:          []string{"read", "write"},
				ModelAccessMode: ModelAccessModeRestricted,
				GrantedModels:   []string{"gpt-4"},
			},
			wantNil: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.response.ToAuthenticatedContext()
			if tt.wantNil {
				if got != nil {
					t.Error("ToAuthenticatedContext() should return nil")
				}
				return
			}

			if got == nil {
				t.Fatal("ToAuthenticatedContext() should not return nil")
			}

			if got.APIKeyID != tt.response.APIKeyID {
				t.Errorf("APIKeyID = %q, want %q", got.APIKeyID, tt.response.APIKeyID)
			}
			if got.OrganizationID != tt.response.OrganizationID {
				t.Errorf("OrganizationID = %q, want %q", got.OrganizationID, tt.response.OrganizationID)
			}
			if got.PrincipalID != tt.response.PrincipalID {
				t.Errorf("PrincipalID = %q, want %q", got.PrincipalID, tt.response.PrincipalID)
			}
			if got.PrincipalType != tt.response.PrincipalType {
				t.Errorf("PrincipalType = %q, want %q", got.PrincipalType, tt.response.PrincipalType)
			}
			if got.ModelAccessMode != tt.response.ModelAccessMode {
				t.Errorf("ModelAccessMode = %q, want %q", got.ModelAccessMode, tt.response.ModelAccessMode)
			}
		})
	}
}

func TestTypeConstants(t *testing.T) {
	if PrincipalTypeUser != "user" {
		t.Errorf("PrincipalTypeUser = %q, want %q", PrincipalTypeUser, "user")
	}
	if PrincipalTypeServiceAccount != "service_account" {
		t.Errorf("PrincipalTypeServiceAccount = %q, want %q", PrincipalTypeServiceAccount, "service_account")
	}
	if ModelAccessModeAutoGrant != "auto_grant" {
		t.Errorf("ModelAccessModeAutoGrant = %q, want %q", ModelAccessModeAutoGrant, "auto_grant")
	}
	if ModelAccessModeRestricted != "restricted" {
		t.Errorf("ModelAccessModeRestricted = %q, want %q", ModelAccessModeRestricted, "restricted")
	}
}
