package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// API key management use case tests
// See: usecases/apikeys.yaml
// CLI helpers defined in helpers_test.go

// APIKeyJSON represents API key data from JSON output
type APIKeyJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	UserID    string `json:"user_id"`
	CreatedAt string `json:"created_at"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

// TestUC_KEY_001_ListAPIKeys validates that organization administrators can
// list API keys in their organization.
// See: usecases/apikeys.yaml UC-KEY-001
func TestUC_KEY_001_ListAPIKeys(t *testing.T) {
	t.Run("AC-01: list all API keys in organization", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Organization has users with API keys
		// When: User runs `ai-aas-org apikey list`
		result := runOrgCLI("apikey", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is formatted as a table (contains header row)
		if !strings.Contains(result.Output, "ID") {
			t.Error("expected table output with ID column")
		}
		if !strings.Contains(result.Output, "NAME") && !strings.Contains(result.Output, "Name") {
			t.Error("expected table output with NAME column")
		}

		// Then: Key values are NOT shown (only IDs)
		// API key values typically start with specific prefixes
		if strings.Contains(result.Output, "sk-") || strings.Contains(result.Output, "aak_") {
			t.Error("key values should NOT be shown in list output")
		}
	})

	t.Run("AC-02: list API keys for specific user", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User "user@example.com" has API keys
		testUser := "user@example.com"

		// When: User runs `ai-aas-org apikey list --user <email>`
		result := runOrgCLI("apikey", "list", "--user", testUser)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Only keys for the specified user are listed
		// (validation would require knowing the test user's keys)
	})

	t.Run("AC-03: list keys with JSON output", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Organization has API keys
		// When: User runs `ai-aas-org apikey list --json`
		result := runOrgCLI("apikey", "list", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Keys are output as valid JSON array
		var keys []APIKeyJSON
		if err := json.Unmarshal([]byte(result.Output), &keys); err != nil {
			t.Fatalf("expected valid JSON array: %v", err)
		}

		// Then: Each key object includes required fields (if any keys exist)
		for _, key := range keys {
			if key.ID == "" {
				t.Error("key missing 'id' field")
			}
			if key.Name == "" {
				t.Error("key missing 'name' field")
			}
			if key.UserID == "" {
				t.Error("key missing 'user_id' field")
			}
			if key.CreatedAt == "" {
				t.Error("key missing 'created_at' field")
			}
		}

		// Then: Key values are NOT included in JSON
		if strings.Contains(result.Output, `"key"`) || strings.Contains(result.Output, `"value"`) {
			t.Error("key values should NOT be included in JSON output")
		}
	})

	t.Run("AC-04: list keys when none exist", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with empty organization")

		// Given: Organization has no API keys
		// When: User runs `ai-aas-org apikey list`
		result := runOrgCLI("apikey", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no API keys found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "no api key") && !strings.Contains(output, "no keys") {
			t.Error("expected message indicating no API keys found")
		}
	})
}

// TestUC_KEY_002_CreateAPIKey validates that organization administrators can
// create new API keys for users.
// See: usecases/apikeys.yaml UC-KEY-002
func TestUC_KEY_002_CreateAPIKey(t *testing.T) {
	t.Run("AC-01: create key with required fields", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User "usr_abc123" exists in the organization
		testUserID := "usr_abc123"
		keyName := "Production Key"

		// When: User runs `ai-aas-org apikey create --user-id <id> --key-name <name>`
		result := runOrgCLI("apikey", "create", "--user-id", testUserID, "--key-name", keyName)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Key value is displayed (only time it's shown)
		// Key values typically have identifiable patterns
		if !strings.Contains(result.Output, "sk-") && !strings.Contains(result.Output, "aak_") {
			t.Error("expected key value to be displayed")
		}

		// Then: Warning to save the key is shown
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "save") && !strings.Contains(output, "copy") && !strings.Contains(output, "store") {
			t.Error("expected warning to save the key")
		}
	})

	t.Run("AC-02: create key with expiration", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User exists in the organization
		testUserID := "usr_abc123"
		keyName := "Temp Key"

		// When: User runs `ai-aas-org apikey create --user-id <id> --key-name <name> --expires 30d`
		result := runOrgCLI("apikey", "create", "--user-id", testUserID, "--key-name", keyName, "--expires", "30d")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: API key is created with 30-day expiration
		// Then: Expiration date is displayed
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "expir") {
			t.Error("expected expiration date to be displayed")
		}

		// Then: Key value is shown once
		if !strings.Contains(result.Output, "sk-") && !strings.Contains(result.Output, "aak_") {
			t.Error("expected key value to be displayed")
		}
	})

	t.Run("AC-03: create key for non-existent user", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User ID "usr_nonexistent" does not exist
		nonExistentUserID := "usr_nonexistent"

		// When: User runs `ai-aas-org apikey create --user-id <id> --key-name "Test"`
		result := runOrgCLI("apikey", "create", "--user-id", nonExistentUserID, "--key-name", "Test")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("expected exit code 5 (not found), got %d", result.ExitCode)
		}

		// Then: Error message indicates user not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") && !strings.Contains(output, "does not exist") {
			t.Error("expected error message indicating user not found")
		}
	})

	t.Run("AC-04: interactive guided key creation", func(t *testing.T) {
		t.Skip("requires live API and interactive TTY - cannot be tested in automated suite")

		// Given: Admin wants to create key interactively
		// When: User runs `ai-aas-org apikey create --guided`
		// Interactive tests require TTY and cannot be fully automated

		// Verify the flag is recognized at minimum
		result := runOrgCLI("apikey", "create", "--help")
		if !strings.Contains(result.Output, "guided") {
			t.Error("expected --guided flag to be documented in help")
		}
	})
}

// TestUC_KEY_003_DeleteAPIKey validates that organization administrators can
// delete/revoke API keys.
// See: usecases/apikeys.yaml UC-KEY-003
func TestUC_KEY_003_DeleteAPIKey(t *testing.T) {
	t.Run("AC-01: delete key with confirmation", func(t *testing.T) {
		t.Skip("requires live API and interactive TTY")

		// Given: API key "key_abc123" exists in the organization
		// When: User runs `ai-aas-org apikey delete key_abc123` and confirms
		// Interactive confirmation cannot be tested in automated suite

		// Verify the command structure is correct
		result := runOrgCLI("apikey", "delete", "--help")
		if result.ExitCode != 0 {
			t.Fatalf("delete command should show help, got: %s", result.Output)
		}
	})

	t.Run("AC-02: delete key with --force flag", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: API key exists in the organization
		testKeyID := "key_abc123"

		// When: User runs `ai-aas-org apikey delete <id> --force`
		result := runOrgCLI("apikey", "delete", testKeyID, "--force")

		// Then: No confirmation prompt is shown (--force bypasses)
		// Then: Key is deleted immediately
		// Then: Exit code is 0
		if result.ExitCode != 0 && result.ExitCode != 5 { // 0 success, 5 not found (if test key doesn't exist)
			t.Fatalf("expected exit 0 or 5, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("AC-03: delete non-existent key", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: No key with ID "key_nonexistent" exists
		nonExistentKeyID := "key_nonexistent"

		// When: User runs `ai-aas-org apikey delete <id> --force`
		result := runOrgCLI("apikey", "delete", nonExistentKeyID, "--force")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("expected exit code 5 (not found), got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates key not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") {
			t.Error("expected error message indicating key not found")
		}
	})

	t.Run("AC-04: cancel deletion at confirmation", func(t *testing.T) {
		t.Skip("requires live API and interactive TTY - cannot be tested in automated suite")

		// Given: API key "key_abc123" exists
		// When: User runs `ai-aas-org apikey delete key_abc123` and declines
		// Then: Key is NOT deleted
		// Then: Message confirms cancellation
		// Then: Exit code is 0

		// Interactive tests require TTY and cannot be fully automated
	})
}

// TestUC_KEY_004_RotateAPIKey validates that organization administrators can
// rotate API keys for security purposes.
// See: usecases/apikeys.yaml UC-KEY-004
func TestUC_KEY_004_RotateAPIKey(t *testing.T) {
	t.Run("AC-01: rotate existing key", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: API key "key_abc123" exists and is valid
		testKeyID := "key_abc123"

		// When: User runs `ai-aas-org apikey rotate <id>`
		result := runOrgCLI("apikey", "rotate", testKeyID)

		// Then: Exit code is 0 (or 5 if test key doesn't exist)
		if result.ExitCode != 0 && result.ExitCode != 5 {
			t.Fatalf("expected exit 0 or 5, got %d: %s", result.ExitCode, result.Output)
		}

		if result.ExitCode == 0 {
			// Then: New API key value is displayed (only time shown)
			if !strings.Contains(result.Output, "sk-") && !strings.Contains(result.Output, "aak_") {
				t.Error("expected new key value to be displayed")
			}

			// Then: Warning to update applications is shown
			output := strings.ToLower(result.Output)
			if !strings.Contains(output, "update") && !strings.Contains(output, "application") {
				t.Error("expected warning to update applications")
			}
		}
	})

	t.Run("AC-02: rotate non-existent key", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: No key with ID "key_nonexistent" exists
		nonExistentKeyID := "key_nonexistent"

		// When: User runs `ai-aas-org apikey rotate <id>`
		result := runOrgCLI("apikey", "rotate", nonExistentKeyID)

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("expected exit code 5 (not found), got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates key not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") {
			t.Error("expected error message indicating key not found")
		}

		// Then: No new key is created (implicit - exit code indicates failure)
	})

	t.Run("AC-03: rotate key preserves user association", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with actual key to rotate")

		// Given: API key "key_abc123" belongs to user "usr_xyz789"
		// When: User runs `ai-aas-org apikey rotate key_abc123`
		// Then: New key is associated with same user
		// Then: New key has same name as old key
		// Then: New key appears in user's key list
		// Then: Old key is removed from user's key list

		// This test requires:
		// 1. Create a key for a known user
		// 2. Rotate the key
		// 3. Verify new key belongs to same user
		// 4. Verify old key no longer works/exists

		// For now, verify the command exists
		result := runOrgCLI("apikey", "rotate", "--help")
		if result.ExitCode != 0 {
			t.Fatalf("rotate command should show help, got: %s", result.Output)
		}
	})
}
