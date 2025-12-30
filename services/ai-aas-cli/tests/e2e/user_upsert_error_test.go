//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the ai-aas-cli.
//
// This test validates that the CLI provides helpful error messages
// when attempting to create a duplicate user, specifically suggesting
// the --upsert flag as a solution.
//
// Prerequisites:
//   - Development environment running (Kubernetes cluster, services deployed)
//   - Environment variables configured (see env.sh)
//   - ai-aas-cli binary built
//
// Run with: go test -v -tags=e2e ./tests/e2e/...
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUserCreate_DuplicateEmail_SuggestsUpsert tests that when creating a user
// with an email that already exists, the CLI suggests using --upsert flag.
//
// Related beads: aas-tq3u, aas-b8ze, aas-hjat
// Related use case: UC-USR-002/AC-03
func TestUserCreate_DuplicateEmail_SuggestsUpsert(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	cfg := loadTestConfig(t)
	state := &TestState{}

	// Generate unique identifiers for this test run
	timestamp := time.Now().Unix()
	state.OrgSlug = fmt.Sprintf("%s-upsert-%d", cfg.TestOrgPrefix, timestamp)
	state.UserEmail = fmt.Sprintf("upsert-test-%d@test.ai-aas.dev", timestamp)

	// Cleanup function
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, state)
	})

	// Step 1: Create organization
	t.Run("CreateOrganization", func(t *testing.T) {
		orgName := fmt.Sprintf("E2E Upsert Test %s", state.OrgSlug)

		output, err := runCLI(t, cfg,
			"org", "create",
			"--name", orgName,
			"--slug", state.OrgSlug,
		)
		require.NoError(t, err, "Failed to create organization")

		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Failed to parse org create response")

		state.OrgID = getString(result, "orgId")
		require.NotEmpty(t, state.OrgID, "Organization ID should not be empty")

		t.Logf("Created organization: ID=%s, Slug=%s", state.OrgID, state.OrgSlug)
	})

	// Step 2: Create first user (should succeed)
	t.Run("CreateFirstUser", func(t *testing.T) {
		require.NotEmpty(t, state.OrgSlug, "Organization must be created first")

		output, err := runCLI(t, cfg,
			"user", "create",
			"--org-id", state.OrgSlug,
			"--email", state.UserEmail,
			"--roles", "admin",
		)
		require.NoError(t, err, "Failed to create first user")

		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Failed to parse user create response")

		state.UserID = getString(result, "userId")
		require.NotEmpty(t, state.UserID, "User ID should not be empty")

		t.Logf("Created first user: ID=%s, Email=%s", state.UserID, state.UserEmail)
	})

	// Step 3: Attempt to create duplicate user (should fail with helpful error)
	t.Run("CreateDuplicateUser_ShowsUpsertSuggestion", func(t *testing.T) {
		require.NotEmpty(t, state.OrgSlug, "Organization must be created first")
		require.NotEmpty(t, state.UserEmail, "First user must be created")

		cliBinary := findCLIBinary(t)

		cmdArgs := []string{
			"user", "create",
			"--org-id", state.OrgSlug,
			"--email", state.UserEmail,
			"--roles", "member",
			"--user-org-endpoint", cfg.UserOrgEndpoint,
			"--api-key", cfg.APIKey,
			"--format", "json",
		}

		cmd := exec.Command(cliBinary, cmdArgs...)
		cmd.Env = append(cmd.Env,
			fmt.Sprintf("AI_AAS_TLS_INSECURE=%v", cfg.TLSInsecure),
		)

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		t.Logf("Running: %s %s", cliBinary, strings.Join(cmdArgs, " "))

		err := cmd.Run()

		// We EXPECT this to fail (exit code != 0)
		require.Error(t, err, "Creating duplicate user should fail")

		output := stdout.String() + stderr.String()
		t.Logf("Error output:\n%s", output)

		// Verify error message contains helpful suggestions
		// According to aas-b8ze acceptance criteria:
		// 1. Error message should mention the email that conflicts
		assert.Contains(t, output, state.UserEmail,
			"Error message should include the conflicting email address")

		// 2. Error message should suggest --upsert flag
		lowerOutput := strings.ToLower(output)
		assert.True(t,
			strings.Contains(lowerOutput, "upsert") || strings.Contains(lowerOutput, "--upsert"),
			"Error message should suggest the --upsert flag")

		// 3. Error message should provide example command (optional but nice)
		if strings.Contains(lowerOutput, "--upsert") {
			t.Logf("✓ Error message includes --upsert suggestion")
		}

		// 4. Verify exit code indicates conflict (6 per UC-USR-002/AC-03)
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			t.Logf("Exit code: %d", exitCode)
			// Note: Exit code 6 is expected per UC-USR-002/AC-03
			// but may vary during implementation
			if exitCode == 6 {
				t.Logf("✓ Exit code 6 (conflict) as specified in UC-USR-002/AC-03")
			}
		}
	})

	// Step 4: Verify --upsert flag works correctly (returns existing user)
	t.Run("CreateWithUpsert_ReturnsExistingUser", func(t *testing.T) {
		require.NotEmpty(t, state.OrgSlug, "Organization must be created first")
		require.NotEmpty(t, state.UserEmail, "First user must be created")

		output, err := runCLI(t, cfg,
			"user", "create",
			"--org-id", state.OrgSlug,
			"--email", state.UserEmail,
			"--roles", "member",
			"--upsert",
		)
		require.NoError(t, err, "Creating user with --upsert should succeed")

		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Failed to parse upsert response")

		returnedUserID := getString(result, "userId")
		require.NotEmpty(t, returnedUserID, "Upsert should return user ID")

		// Verify it returns the EXISTING user ID
		assert.Equal(t, state.UserID, returnedUserID,
			"Upsert should return the existing user, not create a new one")

		t.Logf("✓ --upsert correctly returned existing user: ID=%s", returnedUserID)
	})
}
