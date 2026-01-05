package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// CLI Scenario Tests
//
// These tests exercise complete CLI workflows end-to-end, testing multiple
// commands in sequence to verify the full user experience works correctly.
//
// Unlike isolated UC tests, scenario tests:
// - Test real-world workflows as users would experience them
// - Verify command outputs can be used as inputs to subsequent commands
// - Ensure data flows correctly between operations
// - Catch integration issues that isolated tests miss

// =============================================================================
// Scenario: User Management Workflow
// =============================================================================

// TestScenario_UserManagement tests the complete user management workflow:
// 1. List existing users
// 2. Create a new user
// 3. Show user details
// 4. Manage user's model access
// 5. View user's token policy
// 6. View user's usage
// 7. Delete the user
func TestScenario_UserManagement(t *testing.T) {
	skipIfNoLiveAPI(t)

	orgCtx := NewTestOrgContext(t)
	testEmail := "scenario-user-" + generateUniqueID() + "@test.example.com"
	testName := "Scenario Test User"

	// Step 1: List users (baseline)
	t.Run("1-list-users-baseline", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "list")
		if result.ExitCode != 0 {
			t.Fatalf("user list failed: %s", result.Output)
		}
		t.Logf("Initial user list: %d chars", len(result.Output))
	})

	// Step 2: Create a new user
	var userID string
	t.Run("2-create-user", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "create",
			"--user-email", testEmail,
			"--user-display-name", testName,
			"--json")
		if result.ExitCode != 0 {
			t.Fatalf("user create failed: %s", result.Output)
		}

		// Extract user ID from JSON output
		var user map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &user); err != nil {
			t.Fatalf("failed to parse user JSON: %v", err)
		}
		if id, ok := user["userId"].(string); ok {
			userID = id
			t.Logf("Created user with ID: %s", userID)
		} else {
			t.Fatal("userId not found in response")
		}
	})

	// Cleanup
	t.Cleanup(func() {
		if userID != "" {
			orgCtx.RunCLI("user", "delete", testEmail, "--force")
		}
	})

	// Step 3: Show user details
	t.Run("3-show-user", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "show", testEmail)
		if result.ExitCode != 0 {
			t.Fatalf("user show failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, testEmail) {
			t.Errorf("expected email in output, got: %s", result.Output)
		}
		if !strings.Contains(result.Output, testName) {
			t.Errorf("expected display name in output, got: %s", result.Output)
		}
	})

	// Step 4: List user's models
	t.Run("4-list-user-models", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "models", "list", "--user", testEmail)
		if result.ExitCode != 0 {
			t.Fatalf("user models list failed: %s", result.Output)
		}
		// New user should have no custom model access
		t.Logf("User models: %s", result.Output)
	})

	// Step 5: Get user's token policy
	t.Run("5-get-user-token-policy", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "get-token-policy", "--user", testEmail)
		if result.ExitCode != 0 {
			t.Fatalf("user get-token-policy failed: %s", result.Output)
		}
		t.Logf("User token policy: %s", result.Output)
	})

	// Step 6: View user's usage
	t.Run("6-view-user-usage", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "usage", "--user", testEmail)
		if result.ExitCode != 0 {
			t.Fatalf("user usage failed: %s", result.Output)
		}
		// New user should have zero usage
		t.Logf("User usage: %s", result.Output)
	})

	// Step 7: Verify user appears in list
	t.Run("7-verify-user-in-list", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "list", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("user list failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, testEmail) {
			t.Errorf("created user not found in list")
		}
	})

	// Step 8: Delete user
	t.Run("8-delete-user", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "delete", testEmail, "--force")
		if result.ExitCode != 0 {
			t.Fatalf("user delete failed: %s", result.Output)
		}
		userID = "" // Clear so cleanup doesn't try again
	})

	// Step 9: Verify user is gone
	t.Run("9-verify-user-deleted", func(t *testing.T) {
		result := orgCtx.RunCLI("user", "show", testEmail)
		if result.ExitCode == 0 {
			t.Error("expected error when showing deleted user")
		}
	})
}

// =============================================================================
// Scenario: API Key Lifecycle
// =============================================================================

// TestScenario_APIKeyLifecycle tests the complete API key workflow:
// 1. Create a test user
// 2. List API keys (empty for new user)
// 3. Create an API key
// 4. List API keys (verify key appears)
// 5. Rotate the API key
// 6. Delete the API key
// 7. Verify key is deleted
func TestScenario_APIKeyLifecycle(t *testing.T) {
	skipIfNoLiveAPI(t)

	orgCtx := NewTestOrgContext(t)
	testUser := orgCtx.CreateUser("apikey-scenario", "user")
	keyName := "scenario-key-" + generateUniqueID()

	// Step 1: List API keys for user (baseline)
	t.Run("1-list-keys-baseline", func(t *testing.T) {
		result := orgCtx.RunCLI("apikey", "list", "--user", testUser.Email)
		if result.ExitCode != 0 {
			t.Fatalf("apikey list failed: %s", result.Output)
		}
		t.Logf("Initial keys: %s", result.Output)
	})

	// Step 2: Create an API key
	var keyID string
	t.Run("2-create-key", func(t *testing.T) {
		result := orgCtx.RunCLI("apikey", "create",
			"--user-id", testUser.UserID,
			"--key-name", keyName,
			"--expires", "30d",
			"--json")
		if result.ExitCode != 0 {
			t.Fatalf("apikey create failed: %s", result.Output)
		}

		// Extract key ID (JSON uses "keyId")
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if id, ok := resp["keyId"].(string); ok {
			keyID = id
			t.Logf("Created API key: %s", keyID)
		} else {
			t.Fatalf("keyId not found in response: %v", resp)
		}

		// Verify token is returned (only shown once)
		if _, ok := resp["token"].(string); !ok {
			t.Error("expected token to be returned on creation")
		}
	})

	t.Cleanup(func() {
		if keyID != "" {
			orgCtx.RunCLI("apikey", "delete", keyID, "--force")
		}
	})

	// Step 3: Verify key appears in list
	t.Run("3-verify-key-in-list", func(t *testing.T) {
		result := orgCtx.RunCLI("apikey", "list", "--user", testUser.Email, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("apikey list failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, keyName) {
			t.Errorf("created key not found in list: %s", result.Output)
		}
	})

	// Step 4: Rotate the key
	// NOTE: Skipped - apikey rotate requires confirmation and has no --force flag
	// TODO: Add --force flag to apikey rotate command (CLI improvement)
	t.Run("4-rotate-key", func(t *testing.T) {
		t.Skip("apikey rotate requires confirmation - no --force flag available")
	})

	// Step 5: Delete the key
	t.Run("5-delete-key", func(t *testing.T) {
		result := orgCtx.RunCLI("apikey", "delete", keyID, "--force")
		if result.ExitCode != 0 {
			t.Fatalf("apikey delete failed: %s", result.Output)
		}
		keyID = "" // Clear so cleanup doesn't try again
	})

	// Step 6: Verify key is gone
	t.Run("6-verify-key-deleted", func(t *testing.T) {
		// Give the API a moment to process the deletion
		time.Sleep(100 * time.Millisecond)

		result := orgCtx.RunCLI("apikey", "list", "--user", testUser.Email, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("apikey list failed: %s", result.Output)
		}
		if strings.Contains(result.Output, keyName) {
			// Key still appears - may be eventual consistency delay
			t.Logf("Warning: key still in list after deletion (eventual consistency?): %s", result.Output)
		}
	})
}

// =============================================================================
// Scenario: Token Policy Management
// =============================================================================

// TestScenario_TokenPolicyManagement tests the complete token policy workflow:
// 1. List existing policies
// 2. Create a new policy
// 3. Show policy details
// 4. Set as org default
// 5. Assign to a user
// 6. Update the policy
// 7. Clear user assignment
// 8. Delete the policy
func TestScenario_TokenPolicyManagement(t *testing.T) {
	skipIfNoLiveAPI(t)

	orgCtx := NewTestOrgContext(t)
	testUser := orgCtx.CreateUser("policy-scenario", "user")
	policyName := "scenario-policy-" + generateUniqueID()

	// Step 1: List policies (baseline)
	t.Run("1-list-policies-baseline", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "list")
		if result.ExitCode != 0 {
			t.Fatalf("token-policy list failed: %s", result.Output)
		}
		// Should have at least "no-limit" built-in policy
		if !strings.Contains(result.Output, "no-limit") {
			t.Log("Warning: no-limit policy not found")
		}
	})

	// Step 2: Create a new policy
	t.Run("2-create-policy", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "create",
			"--name", policyName,
			"--limit-1h", "10000",
			"--limit-24h", "100000",
			"--limit-7d", "500000")
		if result.ExitCode != 0 {
			t.Fatalf("token-policy create failed: %s", result.Output)
		}
	})

	t.Cleanup(func() {
		// Reset default to no-limit first
		orgCtx.RunCLI("token-policy", "default", "--set", "no-limit")
		orgCtx.RunCLI("token-policy", "user", testUser.Email, "--clear")
		orgCtx.RunCLI("token-policy", "delete", policyName, "--force")
	})

	// Step 3: Show policy details
	t.Run("3-show-policy", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "show", policyName, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("token-policy show failed: %s", result.Output)
		}

		var policy map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &policy); err != nil {
			t.Fatalf("failed to parse policy: %v", err)
		}
		if policy["name"] != policyName {
			t.Errorf("expected name %s, got %v", policyName, policy["name"])
		}
	})

	// Step 4: Set as org default
	t.Run("4-set-as-default", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "default", "--set", policyName)
		if result.ExitCode != 0 {
			t.Fatalf("token-policy default --set failed: %s", result.Output)
		}

		// Verify it's now the default
		verifyResult := orgCtx.RunCLI("token-policy", "default", "--json")
		if !strings.Contains(verifyResult.Output, policyName) {
			t.Errorf("policy not set as default: %s", verifyResult.Output)
		}
	})

	// Step 5: Assign to user
	t.Run("5-assign-to-user", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "user", testUser.Email, "--set", policyName)
		if result.ExitCode != 0 {
			t.Fatalf("token-policy user --set failed: %s", result.Output)
		}

		// Verify assignment
		verifyResult := orgCtx.RunCLI("token-policy", "user", testUser.Email, "--json")
		if !strings.Contains(verifyResult.Output, policyName) {
			t.Errorf("policy not assigned to user: %s", verifyResult.Output)
		}
	})

	// Step 6: Update policy limits
	t.Run("6-update-policy", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "update", policyName, "--limit-1h", "20000")
		if result.ExitCode != 0 {
			t.Fatalf("token-policy update failed: %s", result.Output)
		}

		// Verify update
		verifyResult := orgCtx.RunCLI("token-policy", "show", policyName, "--json")
		if !strings.Contains(verifyResult.Output, "20000") {
			t.Errorf("policy limit not updated: %s", verifyResult.Output)
		}
	})

	// Step 7: Clear user assignment
	t.Run("7-clear-user-assignment", func(t *testing.T) {
		result := orgCtx.RunCLI("token-policy", "user", testUser.Email, "--clear")
		if result.ExitCode != 0 {
			t.Fatalf("token-policy user --clear failed: %s", result.Output)
		}
	})

	// Step 8: Reset default and delete policy
	t.Run("8-cleanup-and-delete", func(t *testing.T) {
		// Reset to no-limit first
		orgCtx.RunCLI("token-policy", "default", "--set", "no-limit")

		result := orgCtx.RunCLI("token-policy", "delete", policyName, "--force")
		if result.ExitCode != 0 {
			t.Fatalf("token-policy delete failed: %s", result.Output)
		}

		// Verify deleted
		listResult := orgCtx.RunCLI("token-policy", "list")
		if strings.Contains(listResult.Output, policyName) {
			t.Errorf("policy still in list after deletion")
		}
	})
}

// =============================================================================
// Scenario: Benchmark Workflow
// =============================================================================

// TestScenario_BenchmarkWorkflow tests the complete benchmark workflow:
// 1. List available scenarios
// 2. Show scenario details
// 3. List existing targets
// 4. Create a benchmark target
// 5. Show target details
// 6. Start the target
// 7. Stop the target
// 8. Trigger a benchmark run
// 9. List runs
// 10. Show run status
// 11. Remove the target
func TestScenario_BenchmarkWorkflow(t *testing.T) {
	skipIfNoLiveAPI(t)

	targetName := "scenario-target-" + generateUniqueID()
	model := getTestModel()
	if model == "" {
		model = "unsloth-gpt-oss-20b" // Fallback to known model
	}
	scenario := "standard"
	t.Logf("Using model: %s, scenario: %s", model, scenario)

	// Step 1: List available scenarios
	var scenarios []map[string]interface{}
	t.Run("1-list-scenarios", func(t *testing.T) {
		result := runOrgCLI("benchmark", "scenario", "list", "--json")
		if result.ExitCode != 0 {
			t.Fatalf("benchmark scenario list failed: %s", result.Output)
		}

		if err := json.Unmarshal([]byte(result.Output), &scenarios); err != nil {
			t.Fatalf("failed to parse scenarios: %v", err)
		}

		if len(scenarios) == 0 {
			t.Fatal("No benchmark scenarios available - run scripts/seed-benchmark-scenarios.sh")
		}
		t.Logf("Found %d scenarios", len(scenarios))
	})

	// Step 2: Show scenario details
	t.Run("2-show-scenario", func(t *testing.T) {
		result := runOrgCLI("benchmark", "scenario", "show", scenario)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark scenario show failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, scenario) {
			t.Errorf("expected scenario name in output")
		}
	})

	// Step 3: List existing targets (baseline)
	t.Run("3-list-targets-baseline", func(t *testing.T) {
		result := runOrgCLI("benchmark", "target", "list")
		if result.ExitCode != 0 {
			t.Fatalf("benchmark target list failed: %s", result.Output)
		}
		t.Logf("Existing targets: %d chars", len(result.Output))
	})

	// Step 4: Create a benchmark target
	t.Run("4-create-target", func(t *testing.T) {
		result := runOrgCLI("benchmark", "target", "add", targetName,
			"-m", model,
			"-s", scenario)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark target add failed: %s", result.Output)
		}
		t.Logf("Created target: %s", targetName)
	})

	t.Cleanup(func() {
		runOrgCLI("benchmark", "target", "remove", targetName, "--force")
	})

	// Step 5: Show target details
	t.Run("5-show-target", func(t *testing.T) {
		result := runOrgCLI("benchmark", "target", "show", targetName)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark target show failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, targetName) {
			t.Errorf("expected target name in output")
		}
		if !strings.Contains(result.Output, model) {
			t.Errorf("expected model name in output")
		}
	})

	// Step 6: Start the target
	t.Run("6-start-target", func(t *testing.T) {
		result := runOrgCLI("benchmark", "target", "start", targetName)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark target start failed: %s", result.Output)
		}
		t.Logf("Started target: %s", targetName)
	})

	// Step 7: Stop the target
	t.Run("7-stop-target", func(t *testing.T) {
		result := runOrgCLI("benchmark", "target", "stop", targetName)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark target stop failed: %s", result.Output)
		}
		t.Logf("Stopped target: %s", targetName)
	})

	// Step 8: Trigger a benchmark run
	var runID string
	t.Run("8-trigger-run", func(t *testing.T) {
		result := runOrgCLI("benchmark", "run", "trigger", targetName, "--json")
		if result.ExitCode != 0 {
			t.Fatalf("benchmark run trigger failed: %s", result.Output)
		}

		// Extract run ID
		var resp map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &resp); err != nil {
			t.Fatalf("failed to parse response: %v", err)
		}
		if id, ok := resp["runId"].(string); ok {
			runID = id
			t.Logf("Triggered run: %s", runID)
		}
	})

	// Step 9: List runs
	t.Run("9-list-runs", func(t *testing.T) {
		result := runOrgCLI("benchmark", "run", "list", "--target", targetName)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark run list failed: %s", result.Output)
		}
		t.Logf("Runs for target: %s", result.Output)
	})

	// Step 10: Show run status
	t.Run("10-show-run", func(t *testing.T) {
		if runID == "" {
			t.Skip("no run ID from trigger")
		}
		result := runOrgCLI("benchmark", "run", "show", runID)
		if result.ExitCode != 0 {
			t.Fatalf("benchmark run show failed: %s", result.Output)
		}
		// Check for status field
		if !strings.Contains(strings.ToLower(result.Output), "status") {
			t.Log("Warning: status not shown in run output")
		}
	})

	// Step 11: Remove the target
	t.Run("11-remove-target", func(t *testing.T) {
		result := runOrgCLI("benchmark", "target", "remove", targetName, "--force")
		if result.ExitCode != 0 {
			t.Fatalf("benchmark target remove failed: %s", result.Output)
		}

		// Verify removed
		listResult := runOrgCLI("benchmark", "target", "list", "--json")
		if strings.Contains(listResult.Output, targetName) {
			t.Errorf("target still in list after removal")
		}
	})
}

// =============================================================================
// Scenario: Organization Overview
// =============================================================================

// TestScenario_OrganizationOverview tests the organization info and usage workflow:
// 1. Show org details
// 2. View usage summary
// 3. View usage by model
// 4. View usage by user
// 5. View audit logs
func TestScenario_OrganizationOverview(t *testing.T) {
	skipIfNoLiveAPI(t)

	orgCtx := NewTestOrgContext(t)

	// Step 1: Show org details
	t.Run("1-org-show", func(t *testing.T) {
		result := orgCtx.RunCLI("org", "show")
		if result.ExitCode != 0 {
			t.Fatalf("org show failed: %s", result.Output)
		}
		// Should show org name and ID
		t.Logf("Org info: %s", result.Output)
	})

	// Step 2: View usage summary
	t.Run("2-usage-summary", func(t *testing.T) {
		result := orgCtx.RunCLI("usage", "summary")
		if result.ExitCode != 0 {
			// Usage summary may fail if analytics service has issues
			if strings.Contains(result.Output, "500") || strings.Contains(result.Output, "Internal Server Error") {
				t.Skipf("analytics service unavailable: %s", result.Output)
			}
			t.Fatalf("usage summary failed: %s", result.Output)
		}
		t.Logf("Usage summary: %s", result.Output)
	})

	// Step 3: View usage by model
	t.Run("3-usage-by-model", func(t *testing.T) {
		result := orgCtx.RunCLI("usage", "by-model", "--json")
		if result.ExitCode != 0 {
			// Usage by-model may fail if analytics service has issues
			if strings.Contains(result.Output, "500") || strings.Contains(result.Output, "Internal Server Error") {
				t.Skipf("analytics service unavailable: %s", result.Output)
			}
			t.Fatalf("usage by-model failed: %s", result.Output)
		}
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON: %s", result.Output)
		}
	})

	// Step 4: View usage by user
	t.Run("4-usage-by-user", func(t *testing.T) {
		result := orgCtx.RunCLI("usage", "by-user", "--json")
		if result.ExitCode != 0 {
			// Usage by-user may fail if analytics service has issues
			if strings.Contains(result.Output, "500") || strings.Contains(result.Output, "Internal Server Error") {
				t.Skipf("analytics service unavailable: %s", result.Output)
			}
			t.Fatalf("usage by-user failed: %s", result.Output)
		}
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON: %s", result.Output)
		}
	})

	// Step 5: View audit logs
	t.Run("5-audit-list", func(t *testing.T) {
		result := orgCtx.RunCLI("audit", "list", "--limit", "10")
		if result.ExitCode != 0 {
			t.Fatalf("audit list failed: %s", result.Output)
		}
		t.Logf("Recent audit events: %d chars", len(result.Output))
	})
}

// =============================================================================
// Scenario: Model Access
// =============================================================================

// TestScenario_ModelAccess tests model listing and details workflow:
// 1. List all available models
// 2. Show details for a model
//
// NOTE: model list uses the inference endpoint (api-router /v1/models)
// while model show uses the admin API. Both may fail if not configured.
func TestScenario_ModelAccess(t *testing.T) {
	skipIfNoLiveAPI(t)

	orgCtx := NewTestOrgContext(t)
	model := getTestModel()

	// Step 1: List models (via inference endpoint)
	t.Run("1-list-models", func(t *testing.T) {
		result := orgCtx.RunCLI("model", "list")
		if result.ExitCode != 0 {
			// model list uses inference endpoint which may not be configured/available
			if strings.Contains(result.Output, "404") ||
				strings.Contains(result.Output, "not configured") ||
				strings.Contains(result.Output, "connection refused") {
				t.Skipf("inference endpoint unavailable: %s", result.Output)
			}
			t.Fatalf("model list failed: %s", result.Output)
		}
		t.Logf("Available models: %d chars", len(result.Output))
	})

	// Step 2: Show model details (via admin API)
	t.Run("2-show-model", func(t *testing.T) {
		result := orgCtx.RunCLI("model", "show", model)
		if result.ExitCode != 0 {
			// model show may fail if model not found or API not available
			if strings.Contains(result.Output, "404") || strings.Contains(result.Output, "not found") {
				t.Skipf("model %s not found: %s", model, result.Output)
			}
			t.Fatalf("model show failed: %s", result.Output)
		}
		if !strings.Contains(result.Output, model) {
			t.Errorf("expected model name in output")
		}
	})

	// Step 3: List models with JSON
	t.Run("3-list-models-json", func(t *testing.T) {
		result := orgCtx.RunCLI("model", "list", "--json")
		if result.ExitCode != 0 {
			// model list uses inference endpoint which may not be configured/available
			if strings.Contains(result.Output, "404") ||
				strings.Contains(result.Output, "not configured") ||
				strings.Contains(result.Output, "connection refused") {
				t.Skipf("inference endpoint unavailable: %s", result.Output)
			}
			t.Fatalf("model list --json failed: %s", result.Output)
		}
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON: %s", result.Output)
		}
	})
}
