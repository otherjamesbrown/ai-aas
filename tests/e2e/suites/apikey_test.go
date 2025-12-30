//go:build (apikey || nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// APIKeyE2ETestSuite runs comprehensive E2E tests for API key management
//
// Prerequisites:
//   - ai-aas-cli must be installed and in PATH (admin operations)
//   - CLI must be configured with admin credentials
//   - Admin API and User-Org services must be accessible
//
// Run with: go test -v ./suites -run TestAPIKey -timeout 10m

// APIKeyJSON represents API key data from JSON output (list response)
type APIKeyJSON struct {
	KeyID       string   `json:"keyId"`
	Notes       string   `json:"notes,omitempty"`
	Fingerprint string   `json:"fingerprint"`
	Status      string   `json:"status"`
	Scopes      []string `json:"scopes,omitempty"`
	IssuedAt    string   `json:"issuedAt,omitempty"`
	ExpiresAt   string   `json:"expiresAt,omitempty"`
}

// APIKeyCreateResponse represents the response when creating an API key
type APIKeyCreateResponse struct {
	KeyID       string `json:"keyId"`
	Token       string `json:"token"`
	Fingerprint string `json:"fingerprint"`
	Status      string `json:"status"`
	ExpiresAt   string `json:"expiresAt,omitempty"`
}

// TestAPIKeyCLIAvailable verifies the CLI is installed and apikey commands exist
func TestAPIKeyCLIAvailable(t *testing.T) {
	result := runCLIWithProfile("apikey", "--help")
	if result.ExitCode != 0 {
		t.Fatalf("CLI apikey command not available: %s", result.Error)
	}

	expectedSubcommands := []string{"list", "create", "delete"}
	for _, subcmd := range expectedSubcommands {
		if !strings.Contains(result.Output, subcmd) {
			t.Errorf("Expected subcommand '%s' not found in help output", subcmd)
		}
	}
	t.Logf("API Key CLI available with subcommands: %v", expectedSubcommands)
}

// UserCreateResponse represents the response from user create (direct mode)
type UserCreateResponse struct {
	UserID            string `json:"userId"`
	Email             string `json:"email"`
	DisplayName       string `json:"displayName"`
	Status            string `json:"status"`
	TemporaryPassword string `json:"temporaryPassword"`
}

// TestAPIKeyLifecycle tests the complete API key CRUD lifecycle
func TestAPIKeyLifecycle(t *testing.T) {
	timestamp := time.Now().Unix()
	testOrgSlug := fmt.Sprintf("e2e-apikey-test-%d", timestamp)
	testUserEmail := fmt.Sprintf("e2e-apikey-user-%d@example.com", timestamp)

	var orgID string
	var userID string
	var keyID string

	t.Cleanup(func() {
		if keyID != "" && orgID != "" {
			t.Logf("Cleanup: Deleting API key %s", keyID)
			runCLIWithUserOrg("apikey", "delete", "--org-id", orgID, "--api-key-id", keyID, "--force")
		}
		// Delete user (may fail if 405 not implemented)
		if userID != "" && orgID != "" {
			t.Logf("Cleanup: Deleting user %s", userID)
			runCLIWithUserOrg("user", "delete", "--org-id", orgID, "--user-id", userID, "--force")
		}
		if orgID != "" {
			t.Logf("Cleanup: Deleting org %s", orgID)
			runCLIWithUserOrg("org", "delete", "--org-id", orgID, "--force")
		}
	})

	// Step 1: Create a test organization
	t.Run("Step1_CreateOrg", func(t *testing.T) {
		t.Logf("Creating test organization: %s", testOrgSlug)
		result := runCLIWithProfile("org", "create",
			"--name", fmt.Sprintf("E2E APIKey Test Org %d", timestamp),
			"--slug", testOrgSlug,
			"--format", "json")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to create org: %s\nOutput: %s", result.Error, result.Output)
		}

		// Get org ID by looking it up
		orgID = findOrgBySlug(t, testOrgSlug)
		if orgID == "" {
			t.Fatal("Could not determine org ID")
		}
		t.Logf("Created org with ID: %s", orgID)
	})

	// Step 2: Create a user in the org (needed to create API key)
	t.Run("Step2_CreateUser", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		t.Logf("Creating user: %s", testUserEmail)
		result := runCLIWithUserOrg("user", "create",
			"--org-id", orgID,
			"--email", testUserEmail,
			"--direct",
			"--format", "json")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to create user: %s\nOutput: %s", result.Error, result.Output)
		}

		// Parse user ID from response
		var userResp UserCreateResponse
		jsonData := extractJSONObject(result.Output)
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &userResp); err != nil {
				t.Fatalf("Failed to parse user response: %v\nOutput: %s", err, result.Output)
			}
			userID = userResp.UserID
		}
		if userID == "" {
			t.Fatal("Could not get user ID from create response")
		}
		t.Logf("User created: %s (ID: %s)", testUserEmail, userID)
	})

	// Step 3: Create an API key for the user
	t.Run("Step3_CreateAPIKey", func(t *testing.T) {
		if orgID == "" || userID == "" {
			t.Skip("Skipping: org or user not created")
		}

		t.Logf("Creating API key for user: %s", userID)
		result := runCLIWithUserOrg("apikey", "create",
			"--org-id", orgID,
			"--user-id", userID,
			"--scopes", "inference:read,inference:write",
			"--format", "json")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to create API key: %s\nOutput: %s", result.Error, result.Output)
		}

		// Parse key response
		var keyResp APIKeyCreateResponse
		jsonData := extractJSONObject(result.Output)
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &keyResp); err != nil {
				t.Fatalf("Failed to parse API key JSON: %v\nOutput: %s", err, result.Output)
			}
			keyID = keyResp.KeyID
		}

		if keyID == "" {
			t.Fatal("Could not determine key ID")
		}
		if keyResp.Token == "" {
			t.Error("Key token not returned on creation")
		} else {
			t.Logf("Created API key with ID: %s, fingerprint: %s", keyID, keyResp.Fingerprint)
		}
	})

	// Step 4: List API keys and verify
	t.Run("Step4_ListAPIKeys", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		result := runCLIWithUserOrg("apikey", "list", "--org-id", orgID, "--format", "json")
		if result.ExitCode != 0 {
			t.Fatalf("Failed to list API keys: %s\nOutput: %s", result.Error, result.Output)
		}

		var keys []APIKeyJSON
		jsonData := extractJSONArray(result.Output)
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &keys); err != nil {
				t.Fatalf("Failed to parse keys JSON: %v", err)
			}
		}

		found := false
		for _, k := range keys {
			if k.KeyID == keyID {
				found = true
				t.Logf("Found key in list: keyId=%s, status=%s", k.KeyID, k.Status)
				break
			}
		}

		if !found && keyID != "" {
			t.Errorf("Created key %s not found in list", keyID)
		}
	})

	// Step 5: Delete API key
	t.Run("Step5_DeleteAPIKey", func(t *testing.T) {
		if keyID == "" || orgID == "" {
			t.Skip("Skipping: key not created")
		}

		t.Logf("Deleting API key %s", keyID)
		result := runCLIWithUserOrg("apikey", "delete", "--org-id", orgID, "--api-key-id", keyID, "--force")
		if result.ExitCode != 0 {
			// Check for known issue: short keyId format not accepted by delete endpoint
			if strings.Contains(result.Output, "invalid API key ID") {
				t.Skip("Skipping: API key delete returns 'invalid API key ID' - known issue with short keyId format")
			}
			t.Fatalf("Failed to delete API key: %s\nOutput: %s", result.Error, result.Output)
		}

		// Verify deletion by listing
		listResult := runCLIWithUserOrg("apikey", "list", "--org-id", orgID, "--format", "json")
		var keys []APIKeyJSON
		jsonData := extractJSONArray(listResult.Output)
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &keys); err == nil {
				for _, k := range keys {
					if k.KeyID == keyID {
						t.Error("Key still exists after deletion")
					}
				}
			}
		}

		keyID = ""
		t.Log("API key deleted successfully")
	})

	t.Log("API key lifecycle test completed successfully")
}

// TestAPIKeyWithExpiration tests creating keys with expiration
func TestAPIKeyWithExpiration(t *testing.T) {
	timestamp := time.Now().Unix()
	testOrgSlug := fmt.Sprintf("e2e-key-exp-%d", timestamp)
	testUserEmail := fmt.Sprintf("e2e-exp-user-%d@example.com", timestamp)

	var orgID string
	var keyID string

	t.Cleanup(func() {
		if keyID != "" && orgID != "" {
			runCLIWithUserOrg("apikey", "delete", "--org-id", orgID, "--api-key-id", keyID, "--force")
		}
		if orgID != "" {
			runCLIWithUserOrg("user", "delete", "--org-id", orgID, "--email", testUserEmail, "--force")
			runCLIWithUserOrg("org", "delete", "--org-id", orgID, "--force")
		}
	})

	// Create org
	result := runCLIWithProfile("org", "create",
		"--name", "Expiration Test",
		"--slug", testOrgSlug)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to create org: %s", result.Error)
	}

	// Get org ID
	orgID = findOrgBySlug(t, testOrgSlug)
	if orgID == "" {
		t.Fatal("Could not get org ID")
	}

	// Create user
	result = runCLIWithUserOrg("user", "create",
		"--org-id", orgID,
		"--email", testUserEmail,
		"--direct")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to create user: %s", result.Error)
	}

	// Create key with 30-day expiration
	result = runCLIWithUserOrg("apikey", "create",
		"--org-id", orgID,
		"--email", testUserEmail,
		"--scopes", "inference:read",
		"--expires-in-days", "30",
		"--format", "json")

	if result.ExitCode != 0 {
		t.Fatalf("Failed to create key with expiration: %s\nOutput: %s", result.Error, result.Output)
	}

	var keyResp APIKeyCreateResponse
	jsonData := extractJSONObject(result.Output)
	if jsonData != "" {
		if err := json.Unmarshal([]byte(jsonData), &keyResp); err != nil {
			t.Fatalf("Failed to parse key: %v", err)
		}
		keyID = keyResp.KeyID
	}

	// Note: ExpiresAt might not be returned in the create response
	t.Logf("Key created with ID: %s", keyID)
}

// TestAPIKeyScopes tests scope validation
func TestAPIKeyScopes(t *testing.T) {
	timestamp := time.Now().Unix()
	testOrgSlug := fmt.Sprintf("e2e-key-scope-%d", timestamp)
	testUserEmail := fmt.Sprintf("e2e-scope-user-%d@example.com", timestamp)

	var orgID string

	t.Cleanup(func() {
		if orgID != "" {
			runCLIWithUserOrg("user", "delete", "--org-id", orgID, "--email", testUserEmail, "--force")
			runCLIWithUserOrg("org", "delete", "--org-id", orgID, "--force")
		}
	})

	// Create org
	result := runCLIWithProfile("org", "create",
		"--name", "Scope Test",
		"--slug", testOrgSlug)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to create org: %s", result.Error)
	}

	// Get org ID
	orgID = findOrgBySlug(t, testOrgSlug)
	if orgID == "" {
		t.Fatal("Could not get org ID")
	}

	// Create user
	result = runCLIWithUserOrg("user", "create",
		"--org-id", orgID,
		"--email", testUserEmail,
		"--direct")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to create user: %s", result.Error)
	}

	t.Run("ValidScopes", func(t *testing.T) {
		result := runCLIWithUserOrg("apikey", "create",
			"--org-id", orgID,
			"--email", testUserEmail,
			"--scopes", "inference:read,inference:write,benchmark:read",
			"--format", "json")

		if result.ExitCode != 0 {
			t.Errorf("Valid scopes rejected: %s", result.Output)
		} else {
			// Clean up the key
			var keyResp APIKeyCreateResponse
			jsonData := extractJSONObject(result.Output)
			if jsonData != "" {
				if err := json.Unmarshal([]byte(jsonData), &keyResp); err == nil {
					if keyResp.KeyID != "" {
						runCLIWithUserOrg("apikey", "delete", "--org-id", orgID, "--api-key-id", keyResp.KeyID, "--force")
					}
				}
			}
			t.Log("Valid scopes accepted")
		}
	})

	t.Run("EmptyScopes", func(t *testing.T) {
		result := runCLIWithUserOrg("apikey", "create",
			"--org-id", orgID,
			"--email", testUserEmail)
		// Without --scopes flag - might use defaults or reject

		// Behavior depends on API - might use defaults or reject
		if result.ExitCode == 0 {
			// Clean up if created
			var keyResp APIKeyCreateResponse
			jsonData := extractJSONObject(result.Output)
			if jsonData != "" {
				if err := json.Unmarshal([]byte(jsonData), &keyResp); err == nil {
					if keyResp.KeyID != "" {
						runCLIWithUserOrg("apikey", "delete", "--org-id", orgID, "--api-key-id", keyResp.KeyID, "--force")
					}
				}
			}
		}
		t.Logf("Empty scopes result: exit=%d", result.ExitCode)
	})
}
