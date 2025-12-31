//go:build (admin || nightly || full) && e2e_tier || !e2e_tier

package suites

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

// TestMain validates environment before running tests
func TestMain(m *testing.M) {
	// Validate required environment variables in CI/development environments
	// In CI mode, we need ADMIN_API_KEY for tests requiring cross-org admin access
	testEnv := os.Getenv("TEST_ENV")

	if testEnv == "ci" || testEnv == "development" {
		adminAPIKey := os.Getenv("ADMIN_API_KEY")

		if adminAPIKey == "" {
			if testEnv == "ci" {
				// In CI, this is a hard failure - tests cannot proceed without admin key
				fmt.Fprintln(os.Stderr, "FATAL: ADMIN_API_KEY environment variable must be set for CI tests")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "The ADMIN_API_KEY is required for user/org tests that need cross-org admin access.")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "To fix this:")
				fmt.Fprintln(os.Stderr, "  1. Set the ADMIN_API_KEY environment variable:")
				fmt.Fprintln(os.Stderr, "     export ADMIN_API_KEY=<your-admin-api-key>")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "  2. OR source the admin key file:")
				fmt.Fprintln(os.Stderr, "     source tests/e2e/.admin-key.env")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "  3. OR run the setup script:")
				fmt.Fprintln(os.Stderr, "     ./tests/e2e/scripts/setup-test-env.sh")
				os.Exit(1)
			} else {
				// In development, warn but allow tests to proceed (they may use profile credentials)
				fmt.Fprintln(os.Stderr, "WARNING: ADMIN_API_KEY not set. Tests requiring admin access may fail.")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "To fix this:")
				fmt.Fprintln(os.Stderr, "  1. Set the ADMIN_API_KEY environment variable:")
				fmt.Fprintln(os.Stderr, "     export ADMIN_API_KEY=<your-admin-api-key>")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "  2. OR source the admin key file:")
				fmt.Fprintln(os.Stderr, "     source tests/e2e/.admin-key.env")
				fmt.Fprintln(os.Stderr, "")
			}
		}
	}

	// Run tests
	os.Exit(m.Run())
}

// UserE2ETestSuite runs comprehensive E2E tests for user management using the CLI
//
// Prerequisites:
//   - ai-aas-cli must be installed and in PATH
//   - CLI must be configured with admin credentials (--profile dev-admin)
//   - Admin API service must be accessible
//
// Run with: go test -v ./suites -run TestUser -timeout 10m

// UserJSON represents user data from JSON output
type UserJSON struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Status    string `json:"status"`
	OrgID     string `json:"org_id"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// OrgJSON represents organization data from JSON output
type OrgJSON struct {
	ID        string `json:"id"`
	OrgID     string `json:"orgId"` // Alternative field name in some responses
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// GetID returns the org ID from either field
func (o OrgJSON) GetID() string {
	if o.ID != "" {
		return o.ID
	}
	return o.OrgID
}

// extractJSONArray extracts a JSON array from CLI output that may contain warnings
func extractJSONArray(output string) string {
	start := strings.Index(output, "[")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(output, "]")
	if end <= start {
		return ""
	}
	return output[start : end+1]
}

// extractJSONObject extracts the last JSON object from CLI output
func extractJSONObject(output string) string {
	// Find last occurrence of a JSON object
	lastBrace := strings.LastIndex(output, "}")
	if lastBrace < 0 {
		return ""
	}
	// Find matching opening brace
	depth := 0
	for i := lastBrace; i >= 0; i-- {
		if output[i] == '}' {
			depth++
		} else if output[i] == '{' {
			depth--
			if depth == 0 {
				return output[i : lastBrace+1]
			}
		}
	}
	return ""
}

// findOrgBySlug looks up an org by slug from org list output
func findOrgBySlug(t *testing.T, slug string) string {
	listResult := runCLIWithProfile("org", "list", "--format", "json")
	jsonData := extractJSONArray(listResult.Output)
	if jsonData == "" {
		return ""
	}

	var orgs []OrgJSON
	if err := json.Unmarshal([]byte(jsonData), &orgs); err != nil {
		t.Logf("Warning: could not parse org list: %v", err)
		return ""
	}

	for _, o := range orgs {
		if o.Slug == slug {
			return o.GetID()
		}
	}
	return ""
}

// userOrgEndpoint returns the user-org-service endpoint for dev environment
const userOrgEndpoint = "https://user-org.dev.otherjamesbrown.com"

// runCLIWithUserOrg runs a CLI command with user-org-endpoint configured
// Uses ADMIN_API_KEY from environment if available to ensure cross-org access permissions
func runCLIWithUserOrg(args ...string) CLIResult {
	fullArgs := []string{"--profile", "dev-admin", "--user-org-endpoint", userOrgEndpoint}

	// For admin operations requiring cross-org access (apikey create --email, user operations),
	// override the profile's API key with the master admin key from environment
	// This ensures we have the necessary 'admin' scope
	adminAPIKey := os.Getenv("ADMIN_API_KEY")
	if adminAPIKey != "" {
		fullArgs = append(fullArgs, "--api-key", adminAPIKey)
	}

	fullArgs = append(fullArgs, args...)
	return runCLI(fullArgs...)
}

// TestUserCLIAvailable verifies the CLI is installed and user commands exist
func TestUserCLIAvailable(t *testing.T) {
	result := runCLIWithProfile("user", "--help")
	if result.ExitCode != 0 {
		t.Fatalf("CLI user command not available: %s", result.Error)
	}

	// Verify subcommands are listed (no 'show' - use list with filters)
	expectedSubcommands := []string{"list", "create", "update", "delete"}
	for _, subcmd := range expectedSubcommands {
		if !strings.Contains(result.Output, subcmd) {
			t.Errorf("Expected subcommand '%s' not found in help output", subcmd)
		}
	}
	t.Logf("User CLI available with subcommands: %v", expectedSubcommands)
}

// TestUserLifecycle tests the complete user CRUD lifecycle
func TestUserLifecycle(t *testing.T) {
	// Generate unique identifiers for this test run
	timestamp := time.Now().Unix()
	testOrgSlug := fmt.Sprintf("e2e-user-test-%d", timestamp)
	testUserEmail := fmt.Sprintf("e2e-test-%d@example.com", timestamp)

	var orgID string

	// Cleanup at the end
	t.Cleanup(func() {
		if orgID != "" {
			// Delete user first (by email) - may fail if user delete not implemented
			t.Logf("Cleanup: Deleting user %s", testUserEmail)
			runCLIWithUserOrg("user", "delete", "--org-id", orgID, "--email", testUserEmail, "--force")
			// Delete org
			t.Logf("Cleanup: Deleting org %s", orgID)
			runCLIWithProfile("org", "delete", "--org-id", orgID, "--force")
		}
	})

	// Step 1: Create a test organization
	t.Run("Step1_CreateOrg", func(t *testing.T) {
		t.Logf("Creating test organization: %s", testOrgSlug)
		result := runCLIWithProfile("org", "create",
			"--name", fmt.Sprintf("E2E User Test Org %d", timestamp),
			"--slug", testOrgSlug,
			"--format", "json")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to create org: %s\nOutput: %s", result.Error, result.Output)
		}

		// Get org ID by looking it up in the list
		orgID = findOrgBySlug(t, testOrgSlug)
		if orgID == "" {
			t.Fatal("Could not determine org ID after creation")
		}
		t.Logf("Created org with ID: %s", orgID)
	})

	// Step 2: Create a user in the org (direct mode - no email required)
	t.Run("Step2_CreateUser", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		t.Logf("Creating user: %s in org %s", testUserEmail, orgID)
		result := runCLIWithProfile("user", "create",
			"--org-id", orgID,
			"--email", testUserEmail,
			"--direct", // Direct creation, no invitation email
			"--format", "json")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to create user: %s\nOutput: %s", result.Error, result.Output)
		}

		t.Logf("User created: %s", testUserEmail)
	})

	// Step 3: List users and verify our user exists
	t.Run("Step3_ListUsers", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		result := runCLIWithUserOrg("user", "list", "--org-id", orgID, "--format", "json")
		if result.ExitCode != 0 {
			t.Fatalf("Failed to list users: %s\nOutput: %s", result.Error, result.Output)
		}

		var users []UserJSON
		jsonStart := strings.Index(result.Output, "[")
		if jsonStart >= 0 {
			if err := json.Unmarshal([]byte(result.Output[jsonStart:]), &users); err != nil {
				t.Fatalf("Failed to parse users JSON: %v\nOutput: %s", err, result.Output)
			}
		}

		found := false
		for _, u := range users {
			if u.Email == testUserEmail {
				found = true
				t.Logf("Found user in list: %s (role=%s, status=%s)", u.Email, u.Role, u.Status)
				break
			}
		}

		if !found {
			t.Errorf("Created user %s not found in user list", testUserEmail)
		}
	})

	// Step 4: Update user status
	t.Run("Step4_UpdateUser", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		t.Logf("Updating user %s status to suspended", testUserEmail)
		result := runCLIWithUserOrg("user", "update",
			"--org-id", orgID,
			"--email", testUserEmail,
			"--status", "suspended")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to update user: %s\nOutput: %s", result.Error, result.Output)
		}

		// Verify the update by listing
		listResult := runCLIWithUserOrg("user", "list", "--org-id", orgID, "--format", "json")
		var users []UserJSON
		jsonStart := strings.Index(listResult.Output, "[")
		if jsonStart >= 0 {
			if err := json.Unmarshal([]byte(listResult.Output[jsonStart:]), &users); err != nil {
				t.Fatalf("Failed to parse users: %v", err)
			}
		}

		for _, u := range users {
			if u.Email == testUserEmail {
				if u.Status != "suspended" {
					t.Errorf("Expected status 'suspended', got %s", u.Status)
				} else {
					t.Logf("User status updated to: %s", u.Status)
				}
				break
			}
		}
	})

	// Step 5: Reactivate user
	t.Run("Step5_ReactivateUser", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		result := runCLIWithUserOrg("user", "update",
			"--org-id", orgID,
			"--email", testUserEmail,
			"--status", "active")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to reactivate user: %s\nOutput: %s", result.Error, result.Output)
		}
		t.Log("User reactivated")
	})

	// Step 6: Delete user
	t.Run("Step6_DeleteUser", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		t.Logf("Deleting user %s", testUserEmail)
		result := runCLIWithUserOrg("user", "delete",
			"--org-id", orgID,
			"--email", testUserEmail,
			"--force")

		if result.ExitCode != 0 {
			// Check if the API returns 405 (method not allowed) - endpoint may not be deployed
			if strings.Contains(result.Output, "405") || strings.Contains(result.Output, "method not allowed") {
				t.Skip("Skipping: user DELETE endpoint not implemented (405 method not allowed)")
			}
			t.Fatalf("Failed to delete user: %s\nOutput: %s", result.Error, result.Output)
		}

		// Verify deletion by listing
		listResult := runCLIWithUserOrg("user", "list", "--org-id", orgID, "--format", "json")
		var users []UserJSON
		jsonStart := strings.Index(listResult.Output, "[")
		if jsonStart >= 0 {
			if err := json.Unmarshal([]byte(listResult.Output[jsonStart:]), &users); err == nil {
				for _, u := range users {
					if u.Email == testUserEmail {
						t.Error("User still exists after deletion")
					}
				}
			}
		}

		t.Log("User deleted successfully")
	})

	t.Log("User lifecycle test completed successfully")
}

// TestUserListFiltering tests user list filtering options
func TestUserListFiltering(t *testing.T) {
	result := runCLIWithProfile("user", "list", "--help")
	if result.ExitCode != 0 {
		t.Fatalf("user list --help failed: %s", result.Error)
	}

	// Verify filter flags exist
	expectedFlags := []string{"--org-id", "--format"}
	for _, flag := range expectedFlags {
		if !strings.Contains(result.Output, flag) {
			t.Errorf("Expected flag '%s' not found in help", flag)
		}
	}
	t.Logf("User list supports flags: %v", expectedFlags)
}

// TestUserCreateValidation tests input validation for user creation
func TestUserCreateValidation(t *testing.T) {
	// Note: Client-side validation is minimal; most validation happens server-side
	// These tests verify the CLI handles missing required flags

	t.Run("MissingEmail", func(t *testing.T) {
		result := runCLIWithProfile("user", "create",
			"--org-id", "test-org",
			"--direct")

		if result.ExitCode == 0 {
			t.Error("Expected error for missing email, but command succeeded")
		}
		t.Logf("Missing email correctly rejected")
	})

	t.Run("NonexistentOrg", func(t *testing.T) {
		// Server-side validation: org doesn't exist
		result := runCLIWithProfile("user", "create",
			"--org-id", "nonexistent-org-12345",
			"--email", "test@example.com",
			"--direct")

		if result.ExitCode == 0 {
			t.Error("Expected error for nonexistent org, but command succeeded")
		}
		t.Logf("Nonexistent org correctly rejected: exit code %d", result.ExitCode)
	})
}

// TestOrgLifecycle tests the complete organization CRUD lifecycle
func TestOrgLifecycle(t *testing.T) {
	timestamp := time.Now().Unix()
	testOrgSlug := fmt.Sprintf("e2e-org-test-%d", timestamp)
	testOrgName := fmt.Sprintf("E2E Org Test %d", timestamp)

	var orgID string

	t.Cleanup(func() {
		if orgID != "" {
			t.Logf("Cleanup: Deleting org %s", orgID)
			runCLIWithUserOrg("org", "delete", "--org-id", orgID, "--force")
		}
	})

	// Step 1: Create organization
	t.Run("Step1_CreateOrg", func(t *testing.T) {
		t.Logf("Creating organization: %s", testOrgSlug)
		result := runCLIWithProfile("org", "create",
			"--name", testOrgName,
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

	// Step 2: List orgs and verify
	t.Run("Step2_ListOrgs", func(t *testing.T) {
		result := runCLIWithProfile("org", "list", "--format", "json")
		if result.ExitCode != 0 {
			t.Fatalf("Failed to list orgs: %s", result.Error)
		}

		var orgs []OrgJSON
		jsonStart := strings.Index(result.Output, "[")
		if jsonStart >= 0 {
			if err := json.Unmarshal([]byte(result.Output[jsonStart:]), &orgs); err != nil {
				t.Fatalf("Failed to parse orgs: %v", err)
			}
		}

		found := false
		for _, o := range orgs {
			if o.Slug == testOrgSlug {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Created org %s not found in list", testOrgSlug)
		}
		t.Logf("Found %d orgs, test org present", len(orgs))
	})

	// Step 3: Verify org in list (no show command, use list and filter)
	t.Run("Step3_VerifyOrgDetails", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		result := runCLIWithProfile("org", "list", "--format", "json")
		if result.ExitCode != 0 {
			t.Fatalf("Failed to list orgs: %s\nOutput: %s", result.Error, result.Output)
		}

		jsonData := extractJSONArray(result.Output)
		var orgs []OrgJSON
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &orgs); err != nil {
				t.Fatalf("Failed to parse orgs: %v", err)
			}
		}

		var org OrgJSON
		for _, o := range orgs {
			if o.GetID() == orgID {
				org = o
				break
			}
		}

		if org.GetID() == "" {
			t.Fatalf("Org %s not found in list", orgID)
		}

		if org.Name != testOrgName {
			t.Errorf("Expected name %s, got %s", testOrgName, org.Name)
		}
		if org.Slug != testOrgSlug {
			t.Errorf("Expected slug %s, got %s", testOrgSlug, org.Slug)
		}
		t.Logf("Org details: name=%s, slug=%s, status=%s", org.Name, org.Slug, org.Status)
	})

	// Step 4: Update org
	t.Run("Step4_UpdateOrg", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		newName := testOrgName + " Updated"
		result := runCLIWithUserOrg("org", "update", "--org-id", orgID, "--display-name", newName)
		if result.ExitCode != 0 {
			t.Fatalf("Failed to update org: %s\nOutput: %s", result.Error, result.Output)
		}

		// Verify update by listing orgs
		listResult := runCLIWithProfile("org", "list", "--format", "json")
		jsonData := extractJSONArray(listResult.Output)
		var orgs []OrgJSON
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &orgs); err != nil {
				t.Fatalf("Failed to parse orgs: %v", err)
			}
		}

		var org OrgJSON
		for _, o := range orgs {
			if o.GetID() == orgID {
				org = o
				break
			}
		}

		if org.Name != newName {
			t.Errorf("Expected name '%s', got '%s'", newName, org.Name)
		}
		t.Logf("Org updated: name=%s", org.Name)
	})

	// Step 5: Delete org
	t.Run("Step5_DeleteOrg", func(t *testing.T) {
		if orgID == "" {
			t.Skip("Skipping: org not created")
		}

		t.Logf("Deleting org %s", orgID)
		result := runCLIWithUserOrg("org", "delete", "--org-id", orgID, "--force")
		if result.ExitCode != 0 {
			t.Fatalf("Failed to delete org: %s\nOutput: %s", result.Error, result.Output)
		}

		// Verify deletion by checking org list
		listResult := runCLIWithProfile("org", "list", "--format", "json")
		jsonData := extractJSONArray(listResult.Output)
		var orgs []OrgJSON
		if jsonData != "" {
			if err := json.Unmarshal([]byte(jsonData), &orgs); err == nil {
				for _, o := range orgs {
					if o.GetID() == orgID {
						t.Error("Org still exists after deletion")
					}
				}
			}
		}

		orgID = ""
		t.Log("Org deleted successfully")
	})

	t.Log("Org lifecycle test completed successfully")
}

// TestOrgDuplicateSlug tests that duplicate slugs are rejected
func TestOrgDuplicateSlug(t *testing.T) {
	timestamp := time.Now().Unix()
	testOrgSlug := fmt.Sprintf("e2e-dup-test-%d", timestamp)

	var orgID string

	t.Cleanup(func() {
		if orgID != "" {
			runCLIWithUserOrg("org", "delete", "--org-id", orgID, "--force")
		}
	})

	// Create first org
	result := runCLIWithProfile("org", "create",
		"--name", "First Org",
		"--slug", testOrgSlug,
		"--format", "json")

	if result.ExitCode != 0 {
		t.Fatalf("Failed to create first org: %s", result.Error)
	}

	// Get org ID for cleanup
	orgID = findOrgBySlug(t, testOrgSlug)
	if orgID == "" {
		t.Fatal("Could not determine org ID")
	}

	// Try to create second org with same slug
	result = runCLIWithProfile("org", "create",
		"--name", "Second Org",
		"--slug", testOrgSlug)

	if result.ExitCode == 0 {
		t.Error("Expected duplicate slug to be rejected, but command succeeded")
	} else {
		t.Logf("Duplicate slug correctly rejected: exit code %d", result.ExitCode)
	}
}
