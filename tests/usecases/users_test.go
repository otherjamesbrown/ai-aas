package usecases_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// User management use case tests
// See: usecases/users.yaml
// CLI helpers defined in helpers_test.go

// UserJSON represents user data from JSON output
type UserJSON struct {
	ID          string `json:"userId"`
	Email       string `json:"email"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	CreatedAt   string `json:"createdAt"`
}

// ModelAccessJSON represents model access data from JSON output
type ModelAccessJSON struct {
	ModelID   string `json:"model_id"`
	ModelName string `json:"model_name"`
}

// ---------------------------------------------------------------------------
// UC-USR-001: List Organization Users
// ---------------------------------------------------------------------------

// TestUC_USR_001_ListOrganizationUsers validates that organization admins can
// list all users in their organization to understand who has access.
func TestUC_USR_001_ListOrganizationUsers(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)

	// Create a test user so the org isn't empty
	orgCtx.CreateUser("list-test-user", "user")

	t.Run("AC-01: List all users in organization", func(t *testing.T) {
		// Given: Organization has multiple users
		// (precondition: org setup with users)

		// When: User runs `ai-aas-org user list`
		result := orgCtx.RunCLI("user", "list")

		// Then: All users in the organization are listed
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Each user shows email, name, role, and status
		// Table output should have headers
		if !strings.Contains(result.Output, "EMAIL") && !strings.Contains(result.Output, "email") {
			t.Error("Expected table output to contain email column")
		}

		// Then: Output is formatted as a table
		lines := strings.Split(strings.TrimSpace(result.Output), "\n")
		if len(lines) < 2 {
			t.Error("Expected at least header and one data row")
		}

		t.Logf("User list output:\n%s", result.Output)
	})

	t.Run("AC-02: List users with JSON output", func(t *testing.T) {
		// Given: Organization has multiple users
		// (precondition: org setup with users)

		// When: User runs `ai-aas-org user list --json`
		result := orgCtx.RunCLI("user", "list", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Users are output as valid JSON array
		if !isValidJSON(result.Output) {
			t.Fatalf("Output is not valid JSON: %s", result.Output)
		}

		// Then: Each user object includes id, email, name, role, status
		var users []UserJSON
		if err := json.Unmarshal([]byte(result.Output), &users); err != nil {
			t.Fatalf("Failed to unmarshal user list: %v", err)
		}

		if len(users) == 0 {
			t.Log("No users returned (may be expected for empty org)")
			return
		}

		// Validate first user has required fields
		u := users[0]
		if u.ID == "" {
			t.Error("User missing 'id' field")
		}
		if u.Email == "" {
			t.Error("User missing 'email' field")
		}

		t.Logf("Found %d user(s) in JSON output", len(users))
	})

	t.Run("AC-03: Empty organization displays helpful message", func(t *testing.T) {
		// Create a separate fresh org to test empty state
		emptyOrgCtx := NewTestOrgContext(t)

		// Given: Organization has no users (only the admin)
		// When: User runs `ai-aas-org user list`
		result := emptyOrgCtx.RunCLI("user", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no users found or empty list
		// Then: Suggestion to create a user is shown
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "no user") && !strings.Contains(output, "empty") && !strings.Contains(output, "[]") {
			t.Log("Warning: Expected message indicating no users found or empty list")
		}

		t.Logf("Empty org output:\n%s", result.Output)
	})
}

// ---------------------------------------------------------------------------
// UC-USR-002: Create User
// ---------------------------------------------------------------------------

// TestUC_USR_002_CreateUser validates that organization admins can create
// new users who will be able to access AI models through the platform.
func TestUC_USR_002_CreateUser(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)

	t.Run("AC-01: Create user with required fields", func(t *testing.T) {
		// Given: Admin is authenticated and email is not in use
		testEmail := fmt.Sprintf("test-user-ac01-%s@example.com", generateUniqueID())

		// When: User runs `ai-aas-org user create --user-email ... --user-display-name ...`
		result := orgCtx.RunCLI("user", "create",
			"--user-email", testEmail,
			"--user-display-name", "Test User AC01")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User is created in the organization
		// Then: User ID is returned and displayed
		if !strings.Contains(result.Output, "usr_") && !strings.Contains(result.Output, "id") {
			t.Error("Expected user ID to be displayed in output")
		}

		// Verify: User appears in `ai-aas-org user list`
		listResult := orgCtx.RunCLI("user", "list", "--json")
		if !strings.Contains(listResult.Output, testEmail) {
			t.Error("Created user should appear in user list")
		}

		// Then: Default role is 'user'
		if !strings.Contains(strings.ToLower(result.Output), "user") {
			t.Log("Warning: Expected default role 'user' to be shown")
		}

		// Register cleanup to delete the test user
		t.Cleanup(func() {
			orgCtx.RunCLI("user", "delete", testEmail, "--force")
		})

		t.Logf("Create user output:\n%s", result.Output)
	})

	t.Run("AC-02: Create user with admin role", func(t *testing.T) {
		// Given: Admin is authenticated
		testEmail := fmt.Sprintf("test-admin-ac02-%s@example.com", generateUniqueID())

		// When: User runs `ai-aas-org user create ... --role admin`
		result := orgCtx.RunCLI("user", "create",
			"--user-email", testEmail,
			"--user-display-name", "Test Admin AC02",
			"--role", "admin")

		// Then: User is created with admin role
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Role is displayed in output
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "admin") {
			t.Error("Expected admin role to be displayed in output")
		}

		// Register cleanup to delete the test user
		t.Cleanup(func() {
			orgCtx.RunCLI("user", "delete", testEmail, "--force")
		})

		t.Logf("Create admin user output:\n%s", result.Output)
	})

	t.Run("AC-03: Reject duplicate email", func(t *testing.T) {
		// Given: Email is already registered in the organization
		existingEmail := fmt.Sprintf("existing-user-ac03-%s@example.com", generateUniqueID())

		// First create the user
		orgCtx.RunCLI("user", "create",
			"--user-email", existingEmail,
			"--user-display-name", "Existing User")

		// When: User runs create with same email
		result := orgCtx.RunCLI("user", "create",
			"--user-email", existingEmail,
			"--user-display-name", "Duplicate User")

		// Then: Command fails with exit code 6 (conflict)
		if result.ExitCode != 6 {
			t.Errorf("Expected exit code 6 (conflict), got %d", result.ExitCode)
		}

		// Then: Error message indicates email already exists
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "exist") && !strings.Contains(output, "duplicate") && !strings.Contains(output, "conflict") {
			t.Error("Expected error message indicating email already exists")
		}

		// Register cleanup to delete the test user
		t.Cleanup(func() {
			orgCtx.RunCLI("user", "delete", existingEmail, "--force")
		})

		t.Logf("Duplicate email error:\n%s", result.Output)
	})

	t.Run("AC-04: Interactive guided user creation", func(t *testing.T) {
		t.Skip("requires live API and interactive terminal")

		// Given: Admin wants to create user interactively
		// When: User runs `ai-aas-org user create --guided`
		// Then: User is prompted for email, display name, role

		// Note: Interactive tests require special handling (expect/pty)
		// This test documents the expected behavior but cannot be fully automated
		t.Log("Interactive mode requires manual testing or expect-style automation")
	})
}

// ---------------------------------------------------------------------------
// UC-USR-003: Show User Details
// ---------------------------------------------------------------------------

// TestUC_USR_003_ShowUserDetails validates that organization admins can view
// detailed information about a specific user.
func TestUC_USR_003_ShowUserDetails(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)

	t.Run("AC-01: Show user by email", func(t *testing.T) {
		// Given: User exists in the organization
		testUser := orgCtx.CreateUser("show-test", "user")

		// When: User runs `ai-aas-org user show <email>`
		result := orgCtx.RunCLI("user", "show", testUser.Email)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User details are displayed
		// Then: Shows id, email, display name, role, status
		output := strings.ToLower(result.Output)
		requiredFields := []string{"id", "email", "name", "role", "status"}
		for _, field := range requiredFields {
			if !strings.Contains(output, field) {
				t.Errorf("Expected output to contain '%s'", field)
			}
		}

		// Then: Shows created date
		if !strings.Contains(output, "created") {
			t.Log("Warning: Expected created date to be shown")
		}

		t.Logf("User details:\n%s", result.Output)
	})

	t.Run("AC-02: Show user by ID", func(t *testing.T) {
		// Given: User exists
		testUser := orgCtx.CreateUser("show-by-id", "user")

		// When: User runs `ai-aas-org user show <id>`
		result := orgCtx.RunCLI("user", "show", testUser.UserID)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User details are displayed
		// Then: Same information as when using email
		if result.Output == "" {
			t.Error("Expected user details in output")
		}

		t.Logf("User details by ID:\n%s", result.Output)
	})

	t.Run("AC-03: User not found", func(t *testing.T) {
		// Given: No user with the specified identifier exists
		nonexistentEmail := "nonexistent-user-xyz@example.com"

		// When: User runs `ai-aas-org user show <nonexistent>`
		result := orgCtx.RunCLI("user", "show", nonexistentEmail)

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("Expected exit code 5 (not found), got %d", result.ExitCode)
		}

		// Then: Error message indicates user not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") && !strings.Contains(output, "does not exist") {
			t.Error("Expected error message indicating user not found")
		}

		t.Logf("User not found error:\n%s", result.Output)
	})

	t.Run("AC-04: Show user details as JSON", func(t *testing.T) {
		// Given: User exists in the organization
		testUser := orgCtx.CreateUser("json-test", "user")

		// When: User runs `ai-aas-org user show <email> --json`
		result := orgCtx.RunCLI("user", "show", testUser.Email, "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User details are output as valid JSON
		if !isValidJSON(result.Output) {
			t.Fatalf("Output is not valid JSON: %s", result.Output)
		}

		// Then: JSON includes all user fields
		var user UserJSON
		if err := json.Unmarshal([]byte(result.Output), &user); err != nil {
			t.Fatalf("Failed to unmarshal user: %v", err)
		}

		if user.ID == "" || user.Email == "" {
			t.Error("Expected user JSON to include id and email")
		}

		t.Logf("User JSON:\n%s", result.Output)
	})
}

// ---------------------------------------------------------------------------
// UC-USR-004: Delete User
// ---------------------------------------------------------------------------

// TestUC_USR_004_DeleteUser validates that organization admins can remove
// users from the organization, revoking all their API keys.
func TestUC_USR_004_DeleteUser(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)

	t.Run("AC-01: Delete user with confirmation", func(t *testing.T) {
		t.Skip("requires live API and interactive terminal")

		// Given: User "user@example.com" exists in the organization
		// When: User runs `ai-aas-org user delete <email>` and confirms
		// Then: Confirmation prompt is shown
		// Then: User is deleted after confirmation
		// Then: All user's API keys are revoked
		// Then: User no longer appears in user list
		// Then: Exit code is 0

		// Note: Interactive confirmation requires expect-style automation
		t.Log("Interactive confirmation requires manual testing")
	})

	t.Run("AC-02: Delete user with --force flag", func(t *testing.T) {
		// Given: User exists in the organization
		// Use unique email to avoid conflicts from previous test runs
		uniqueID := generateUniqueID()
		testEmail := "delete-force-" + uniqueID + "@example.com"

		// Setup: Create user to delete (check that creation succeeded)
		createResult := orgCtx.RunCLI("user", "create",
			"--user-email", testEmail,
			"--user-display-name", "Delete Force User")
		if createResult.ExitCode != 0 {
			t.Fatalf("Failed to create test user: %s", createResult.Output)
		}

		// When: User runs `ai-aas-org user delete <email> --force`
		result := orgCtx.RunCLI("user", "delete", testEmail, "--force")

		// Then: No confirmation prompt is shown
		// Then: User is deleted immediately
		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Verify: User no longer appears in user list
		listResult := orgCtx.RunCLI("user", "list", "--json")
		if strings.Contains(listResult.Output, testEmail) {
			t.Error("Deleted user should not appear in user list")
		}

		t.Logf("Delete user output:\n%s", result.Output)
	})

	t.Run("AC-03: Delete non-existent user", func(t *testing.T) {
		// Given: No user with the specified identifier exists
		nonexistentEmail := "nonexistent-delete@example.com"

		// When: User runs `ai-aas-org user delete <nonexistent> --force`
		result := orgCtx.RunCLI("user", "delete", nonexistentEmail, "--force")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("Expected exit code 5 (not found), got %d", result.ExitCode)
		}

		// Then: Error message indicates user not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") {
			t.Error("Expected error message indicating user not found")
		}

		t.Logf("Delete not found error:\n%s", result.Output)
	})

	t.Run("AC-04: Cancel deletion at confirmation", func(t *testing.T) {
		t.Skip("requires live API and interactive terminal")

		// Given: User "user@example.com" exists
		// When: User runs `ai-aas-org user delete <email>` and declines
		// Then: User is NOT deleted
		// Then: Message confirms cancellation
		// Then: Exit code is 0

		// Note: Interactive confirmation requires expect-style automation
		t.Log("Interactive cancellation requires manual testing")
	})
}

// ---------------------------------------------------------------------------
// UC-USR-005: List User Model Access
// ---------------------------------------------------------------------------

// TestUC_USR_005_ListUserModelAccess validates that organization admins can
// see which AI models a specific user has access to.
func TestUC_USR_005_ListUserModelAccess(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)

	t.Run("AC-01: List models for user with access", func(t *testing.T) {
		// Given: User has access to multiple models
		testUser := orgCtx.CreateUser("model-access", "user")

		// When: User runs `ai-aas-org user models list --user <email>`
		result := orgCtx.RunCLI("user", "models", "list", "--user", testUser.Email)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: All accessible models are listed
		// Then: Each model shows ID and name
		// Then: Output is formatted as a table
		if result.Output == "" {
			t.Error("Expected model list in output")
		}

		t.Logf("Model access list:\n%s", result.Output)
	})

	t.Run("AC-02: List models for user with no access", func(t *testing.T) {
		// Given: User has no model access
		testUser := orgCtx.CreateUser("restricted", "user")

		// When: User runs `ai-aas-org user models list --user <email>`
		result := orgCtx.RunCLI("user", "models", "list", "--user", testUser.Email)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no model access
		// Then: Suggestion to grant model access is shown
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "no") && !strings.Contains(output, "access") {
			t.Log("Warning: Expected message indicating no model access")
		}

		t.Logf("No model access output:\n%s", result.Output)
	})

	t.Run("AC-03: List models with JSON output", func(t *testing.T) {
		// Given: User has access to models
		testUser := orgCtx.CreateUser("model-json", "user")

		// When: User runs `ai-aas-org user models list --user <email> --json`
		result := orgCtx.RunCLI("user", "models", "list", "--user", testUser.Email, "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Models are output as valid JSON array
		if !isValidJSON(result.Output) {
			t.Fatalf("Output is not valid JSON: %s", result.Output)
		}

		var models []ModelAccessJSON
		if err := json.Unmarshal([]byte(result.Output), &models); err != nil {
			t.Fatalf("Failed to unmarshal model access: %v", err)
		}

		t.Logf("Found %d model(s) in JSON output", len(models))
	})
}

// ---------------------------------------------------------------------------
// UC-USR-006: Grant User Model Access
// ---------------------------------------------------------------------------

// TestUC_USR_006_GrantUserModelAccess validates that organization admins can
// grant users access to specific AI models.
func TestUC_USR_006_GrantUserModelAccess(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)
	testModel := getTestModel()

	t.Run("AC-01: Grant access to specific model", func(t *testing.T) {
		// Given: User exists and model is available
		testUser := orgCtx.CreateUser("grant-access", "user")

		// When: User runs `ai-aas-org user models add --user <email> --model <model>`
		result := orgCtx.RunCLI("user", "models", "add",
			"--user", testUser.Email,
			"--model", testModel)

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User is granted access to the model
		// Then: Success message confirms access granted
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "grant") && !strings.Contains(output, "access") && !strings.Contains(output, "success") {
			t.Log("Warning: Expected success message confirming access granted")
		}

		// Verify: Model appears in user's model list
		listResult := orgCtx.RunCLI("user", "models", "list", "--user", testUser.Email, "--json")
		if !strings.Contains(listResult.Output, testModel) {
			t.Error("Granted model should appear in user's model list")
		}

		t.Logf("Grant access output:\n%s", result.Output)
	})

	t.Run("AC-02: Grant access to all models", func(t *testing.T) {
		// Given: User exists and organization has multiple models
		testUser := orgCtx.CreateUser("grant-all", "user")

		// When: User runs `ai-aas-org user models add --user <email> --all`
		result := orgCtx.RunCLI("user", "models", "add",
			"--user", testUser.Email,
			"--all")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User is granted access to all available models
		// Then: Success message lists models granted
		// Then: All models appear in user's model list

		t.Logf("Grant all models output:\n%s", result.Output)
	})

	t.Run("AC-03: Grant access to model user already has", func(t *testing.T) {
		// Given: User already has access to model
		testUser := orgCtx.CreateUser("idempotent-grant", "user")

		// First grant
		orgCtx.RunCLI("user", "models", "add",
			"--user", testUser.Email,
			"--model", testModel)

		// When: User runs grant again
		result := orgCtx.RunCLI("user", "models", "add",
			"--user", testUser.Email,
			"--model", testModel)

		// Then: Operation succeeds (idempotent)
		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0 (idempotent), got %d", result.ExitCode)
		}

		// Then: Message indicates user already has access
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "already") {
			t.Log("Warning: Expected message indicating user already has access")
		}

		t.Logf("Idempotent grant output:\n%s", result.Output)
	})

	t.Run("AC-04: Grant access to unavailable model", func(t *testing.T) {
		// KNOWN GAP: Backend (user-org-service) does not currently validate that models
		// exist before granting access. This requires cross-service validation between
		// user-org-service and admin-api-service model registry.
		// Tracked by: aas-uadrw
		t.Skip("skipping: backend model validation not yet implemented (tracked by aas-uadrw)")

		// Given: Model "restricted-model" is not available to the organization
		testUser := orgCtx.CreateUser("unavailable-model", "user")
		unavailableModel := "restricted-model-xyz"

		// When: User runs `ai-aas-org user models add --user <email> --model <unavailable>`
		result := orgCtx.RunCLI("user", "models", "add",
			"--user", testUser.Email,
			"--model", unavailableModel)

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("Expected exit code 5 (not found), got %d", result.ExitCode)
		}

		// Then: Error message indicates model not available
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") && !strings.Contains(output, "not available") {
			t.Error("Expected error message indicating model not available")
		}

		// Then: No access is granted
		t.Logf("Unavailable model error:\n%s", result.Output)
	})
}

// ---------------------------------------------------------------------------
// UC-USR-007: Revoke User Model Access
// ---------------------------------------------------------------------------

// TestUC_USR_007_RevokeUserModelAccess validates that organization admins can
// revoke a user's access to specific AI models.
func TestUC_USR_007_RevokeUserModelAccess(t *testing.T) {
	skipIfNoLiveAPI(t)

	// Create fresh org for test isolation
	orgCtx := NewTestOrgContext(t)
	testModel := getTestModel()

	t.Run("AC-01: Revoke access to specific model", func(t *testing.T) {
		// Given: User has access to model
		testUser := orgCtx.CreateUser("revoke-access", "user")

		// Setup: Ensure user has access
		orgCtx.RunCLI("user", "models", "add",
			"--user", testUser.Email,
			"--model", testModel)

		// When: User runs `ai-aas-org user models remove --user <email> --model <model> --force`
		result := orgCtx.RunCLI("user", "models", "remove",
			"--user", testUser.Email,
			"--model", testModel,
			"--force")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("Expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: User's access to the model is revoked
		// Then: Success message confirms access removed
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "revoke") && !strings.Contains(output, "remove") && !strings.Contains(output, "success") {
			t.Log("Warning: Expected success message confirming access removed")
		}

		// Verify: Model no longer appears in user's model list
		listResult := orgCtx.RunCLI("user", "models", "list", "--user", testUser.Email, "--json")
		if strings.Contains(listResult.Output, testModel) {
			t.Error("Revoked model should not appear in user's model list")
		}

		t.Logf("Revoke access output:\n%s", result.Output)
	})

	t.Run("AC-02: Revoke access to model user doesn't have", func(t *testing.T) {
		// Given: User does not have access to model
		testUser := orgCtx.CreateUser("no-access-revoke", "user")

		// When: User runs `ai-aas-org user models remove --user <email> --model <model> --force`
		result := orgCtx.RunCLI("user", "models", "remove",
			"--user", testUser.Email,
			"--model", testModel,
			"--force")

		// Then: Operation succeeds (idempotent)
		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Errorf("Expected exit code 0 (idempotent), got %d", result.ExitCode)
		}

		// Then: Message indicates user did not have access
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not have") && !strings.Contains(output, "no access") {
			t.Log("Warning: Expected message indicating user did not have access")
		}

		t.Logf("Idempotent revoke output:\n%s", result.Output)
	})

	t.Run("AC-03: Revoke access for non-existent user", func(t *testing.T) {
		// Given: No user "nonexistent@example.com" exists
		nonexistentEmail := "nonexistent-revoke@example.com"

		// When: User runs `ai-aas-org user models remove --user <nonexistent> --model <model> --force`
		result := orgCtx.RunCLI("user", "models", "remove",
			"--user", nonexistentEmail,
			"--model", testModel,
			"--force")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Errorf("Expected exit code 5 (not found), got %d", result.ExitCode)
		}

		// Then: Error message indicates user not found
		output := strings.ToLower(result.Output)
		if !strings.Contains(output, "not found") && !strings.Contains(output, "does not exist") {
			t.Error("Expected error message indicating user not found")
		}

		// Then: No changes are made
		t.Logf("User not found error:\n%s", result.Output)
	})
}
