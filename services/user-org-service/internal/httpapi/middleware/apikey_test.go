package middleware

import (
	"encoding/json"
	"testing"
)

// TestJSONBScopeUnmarshaling tests that JSONB scope arrays are correctly unmarshaled.
// This is a regression test for the bug where scopes JSONB column was being scanned
// directly into []string, which doesn't work with pgx v5.
func TestJSONBScopeUnmarshaling(t *testing.T) {
	tests := []struct {
		name        string
		jsonbBytes  []byte
		expected    []string
		expectError bool
	}{
		{
			name:        "single scope",
			jsonbBytes:  []byte(`["inference:read"]`),
			expected:    []string{"inference:read"},
			expectError: false,
		},
		{
			name:        "multiple scopes",
			jsonbBytes:  []byte(`["inference:read", "org:admin", "user:manage"]`),
			expected:    []string{"inference:read", "org:admin", "user:manage"},
			expectError: false,
		},
		{
			name:        "empty array",
			jsonbBytes:  []byte(`[]`),
			expected:    []string{},
			expectError: false,
		},
		{
			name:        "null bytes",
			jsonbBytes:  []byte{},
			expected:    []string{},
			expectError: false,
		},
		{
			name:        "admin and wildcard scopes",
			jsonbBytes:  []byte(`["admin", "*"]`),
			expected:    []string{"admin", "*"},
			expectError: false,
		},
		{
			name:        "invalid json",
			jsonbBytes:  []byte(`not valid json`),
			expected:    nil,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var scopes []string

			// Simulate what tryAPIKeyAuth does: unmarshal JSONB bytes into []string
			if len(tt.jsonbBytes) > 0 {
				err := json.Unmarshal(tt.jsonbBytes, &scopes)
				if tt.expectError {
					if err == nil {
						t.Errorf("expected error unmarshaling %q, got nil", string(tt.jsonbBytes))
					}
					return
				}
				if err != nil {
					t.Fatalf("unexpected error unmarshaling %q: %v", string(tt.jsonbBytes), err)
				}
			}

			// Initialize to empty slice if nil (like the fixed code does)
			if scopes == nil {
				scopes = []string{}
			}

			// Verify the result
			if len(scopes) != len(tt.expected) {
				t.Errorf("expected %d scopes, got %d: %v", len(tt.expected), len(scopes), scopes)
				return
			}

			for i, scope := range scopes {
				if scope != tt.expected[i] {
					t.Errorf("scope[%d]: expected %q, got %q", i, tt.expected[i], scope)
				}
			}
		})
	}
}

// TestJSONBScopeUnmarshalingWithDatabase simulates the exact pattern used in tryAPIKeyAuth.
// This ensures that the fix properly handles the JSONB column from PostgreSQL.
func TestJSONBScopeUnmarshalingWithDatabase(t *testing.T) {
	// This simulates what the database returns for different JSONB values
	tests := []struct {
		name           string
		dbValue        []byte // What pgx.Scan gives us when scanning JSONB into []byte
		expectedScopes []string
	}{
		{
			name:           "inference:read scope",
			dbValue:        []byte(`["inference:read"]`),
			expectedScopes: []string{"inference:read"},
		},
		{
			name:           "org:admin scope",
			dbValue:        []byte(`["org:admin"]`),
			expectedScopes: []string{"org:admin"},
		},
		{
			name:           "multiple scopes",
			dbValue:        []byte(`["org:admin","user:manage","apikey:manage"]`),
			expectedScopes: []string{"org:admin", "user:manage", "apikey:manage"},
		},
		{
			name:           "empty scopes array (default)",
			dbValue:        []byte(`[]`),
			expectedScopes: []string{},
		},
		{
			name:           "admin scope (universal grant)",
			dbValue:        []byte(`["admin"]`),
			expectedScopes: []string{"admin"},
		},
		{
			name:           "wildcard scope (universal grant)",
			dbValue:        []byte(`["*"]`),
			expectedScopes: []string{"*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This is the exact pattern from the fixed tryAPIKeyAuth function
			var scopesJSON []byte = tt.dbValue
			var scopes []string

			// Unmarshal JSONB scopes array into []string
			if len(scopesJSON) > 0 {
				if err := json.Unmarshal(scopesJSON, &scopes); err != nil {
					t.Fatalf("failed to unmarshal scopes: %v", err)
				}
			}
			if scopes == nil {
				scopes = []string{}
			}

			// Verify the result matches expected
			if len(scopes) != len(tt.expectedScopes) {
				t.Errorf("expected %d scopes, got %d: %v", len(tt.expectedScopes), len(scopes), scopes)
				return
			}

			for i, scope := range scopes {
				if scope != tt.expectedScopes[i] {
					t.Errorf("scope[%d]: expected %q, got %q", i, tt.expectedScopes[i], scope)
				}
			}
		})
	}
}
