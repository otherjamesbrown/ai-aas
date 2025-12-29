//go:build e2e
// +build e2e

// This test verifies that benchmark data is properly isolated between organizations.
// It creates TWO test organizations using ai-aas-cli, then verifies that:
//   - Org A can manage its own benchmark targets and runs
//   - Org B CANNOT access Org A's targets or runs
//   - Both orgs can read the same platform-defined scenarios
//
// Prerequisites:
//   - Development environment running
//   - Master API key configured (via AI_AAS_API_KEY)
//   - ai-aas-cli and ai-aas-org binaries built
//
// Run with: go test -v -tags=e2e ./tests/e2e/... -run TestCrossOrgIsolation
package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// runOrgCLIWithStderr executes the ai-aas-org CLI and returns both stdout and stderr
func runOrgCLIWithStderr(t *testing.T, cfg *TestConfig, orgAPIKey string, args ...string) (string, string, error) {
	t.Helper()

	cliBinary := findOrgCLIBinary(t)

	cmdArgs := append(args,
		"--api-endpoint", cfg.AdminEndpoint,
		"--api-key", orgAPIKey,
		"--json",
	)

	cmd := exec.Command(cliBinary, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"AI_AAS_TLS_INSECURE="+fmt.Sprintf("%v", cfg.TLSInsecure),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running org CLI: %s %s", cliBinary, strings.Join(args, " "))

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// TestCrossOrgIsolation verifies that org keys cannot access other orgs' benchmark data
func TestCrossOrgIsolation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	cfg := loadTestConfig(t)

	// Create two test organizations
	t.Log("Creating test organization A...")
	orgA := createTestOrg(t, cfg, "iso-orgA")
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, orgA)
	})

	t.Log("Creating test organization B...")
	orgB := createTestOrg(t, cfg, "iso-orgB")
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, orgB)
	})

	// First, check if there's a scenario available
	output, err := runOrgCLI(t, cfg, orgA.APISecret, "benchmark", "scenario", "list")
	require.NoError(t, err, "Failed to list scenarios")

	var scenarios []map[string]interface{}
	err = json.Unmarshal([]byte(output), &scenarios)
	require.NoError(t, err, "Failed to parse scenarios")

	if len(scenarios) == 0 {
		t.Skip("No scenarios available - skipping isolation tests")
	}
	scenarioName := getString(scenarios[0], "name")

	// Create a unique target name
	timestamp := time.Now().Unix()
	targetName := fmt.Sprintf("e2e-isolation-target-%d", timestamp)
	var targetID string

	// Cleanup function for the target
	t.Cleanup(func() {
		if targetID != "" {
			runOrgCLI(t, cfg, orgA.APISecret, "benchmark", "target", "remove", targetName, "--force")
		}
	})

	// Step 1: Create target with Org A
	t.Run("OrgA_CreateTarget", func(t *testing.T) {
		output, err := runOrgCLI(t, cfg, orgA.APISecret,
			"benchmark", "target", "add", targetName,
			"--model", "test-model",
			"--scenario", scenarioName,
			"--environment", "development",
		)
		require.NoError(t, err, "Org A failed to create target")

		var target map[string]interface{}
		err = json.Unmarshal([]byte(output), &target)
		require.NoError(t, err, "Failed to parse target create response")

		targetID = getString(target, "id")
		require.NotEmpty(t, targetID, "Target ID should not be empty")

		t.Logf("Org A (%s) created target: ID=%s, Name=%s", orgA.OrgName, targetID, targetName)
	})

	// Step 2: Verify Org B cannot see Org A's target in list
	t.Run("OrgB_CannotSeeTargetInList", func(t *testing.T) {
		output, err := runOrgCLI(t, cfg, orgB.APISecret, "benchmark", "target", "list")
		require.NoError(t, err, "Org B failed to list targets")

		var targets []map[string]interface{}
		err = json.Unmarshal([]byte(output), &targets)
		require.NoError(t, err, "Failed to parse targets list")

		// Org B should not see Org A's target
		for _, target := range targets {
			id := getString(target, "id")
			name := getString(target, "name")
			assert.NotEqual(t, targetID, id, "Org B should not see Org A's target ID")
			assert.NotEqual(t, targetName, name, "Org B should not see Org A's target name")
		}

		t.Logf("Verified: Org B's (%s) target list does not contain Org A's target", orgB.OrgName)
	})

	// Step 3: Verify Org B cannot access Org A's target directly
	t.Run("OrgB_CannotGetTarget", func(t *testing.T) {
		_, stderr, err := runOrgCLIWithStderr(t, cfg, orgB.APISecret, "benchmark", "target", "show", targetName)
		assert.Error(t, err, "Org B should not be able to get Org A's target")

		// Should get a forbidden or not found error
		assert.True(t,
			strings.Contains(stderr, "forbidden") ||
				strings.Contains(stderr, "not found") ||
				strings.Contains(stderr, "403") ||
				strings.Contains(stderr, "404"),
			"Error should indicate forbidden or not found, got: %s", stderr)

		t.Logf("Verified: Org B cannot access Org A's target directly")
	})

	// Step 4: Verify Org B cannot start Org A's target
	t.Run("OrgB_CannotStartTarget", func(t *testing.T) {
		_, stderr, err := runOrgCLIWithStderr(t, cfg, orgB.APISecret, "benchmark", "target", "start", targetName)
		assert.Error(t, err, "Org B should not be able to start Org A's target")

		assert.True(t,
			strings.Contains(stderr, "forbidden") ||
				strings.Contains(stderr, "not found") ||
				strings.Contains(stderr, "403") ||
				strings.Contains(stderr, "404"),
			"Error should indicate forbidden or not found")

		t.Logf("Verified: Org B cannot start Org A's target")
	})

	// Step 5: Verify Org B cannot trigger runs for Org A's target
	t.Run("OrgB_CannotTriggerRun", func(t *testing.T) {
		_, stderr, err := runOrgCLIWithStderr(t, cfg, orgB.APISecret, "benchmark", "run", "trigger", targetName)
		assert.Error(t, err, "Org B should not be able to trigger runs for Org A's target")

		assert.True(t,
			strings.Contains(stderr, "forbidden") ||
				strings.Contains(stderr, "not found") ||
				strings.Contains(stderr, "403") ||
				strings.Contains(stderr, "404"),
			"Error should indicate forbidden or not found")

		t.Logf("Verified: Org B cannot trigger runs for Org A's target")
	})

	// Step 6: Verify Org B cannot stop Org A's target
	t.Run("OrgB_CannotStopTarget", func(t *testing.T) {
		_, stderr, err := runOrgCLIWithStderr(t, cfg, orgB.APISecret, "benchmark", "target", "stop", targetName)
		assert.Error(t, err, "Org B should not be able to stop Org A's target")

		assert.True(t,
			strings.Contains(stderr, "forbidden") ||
				strings.Contains(stderr, "not found") ||
				strings.Contains(stderr, "403") ||
				strings.Contains(stderr, "404"),
			"Error should indicate forbidden or not found")

		t.Logf("Verified: Org B cannot stop Org A's target")
	})

	// Step 7: Verify Org B cannot delete Org A's target
	t.Run("OrgB_CannotDeleteTarget", func(t *testing.T) {
		_, stderr, err := runOrgCLIWithStderr(t, cfg, orgB.APISecret, "benchmark", "target", "remove", targetName, "--force")
		assert.Error(t, err, "Org B should not be able to delete Org A's target")

		assert.True(t,
			strings.Contains(stderr, "forbidden") ||
				strings.Contains(stderr, "not found") ||
				strings.Contains(stderr, "403") ||
				strings.Contains(stderr, "404"),
			"Error should indicate forbidden or not found")

		t.Logf("Verified: Org B cannot delete Org A's target")
	})

	// Step 8: Verify Org A can still access and manage their target
	t.Run("OrgA_CanStillAccessTarget", func(t *testing.T) {
		output, err := runOrgCLI(t, cfg, orgA.APISecret, "benchmark", "target", "show", targetName)
		require.NoError(t, err, "Org A should still be able to access their target")

		var target map[string]interface{}
		err = json.Unmarshal([]byte(output), &target)
		require.NoError(t, err, "Failed to parse target")

		assert.Equal(t, targetID, getString(target, "id"), "Target ID should match")

		t.Logf("Verified: Org A can still access their target")
	})

	// Step 9: Cleanup - Org A removes their target
	t.Run("OrgA_RemoveTarget", func(t *testing.T) {
		output, err := runOrgCLI(t, cfg, orgA.APISecret, "benchmark", "target", "remove", targetName, "--force")
		require.NoError(t, err, "Org A should be able to remove their target")

		var result map[string]interface{}
		err = json.Unmarshal([]byte(output), &result)
		require.NoError(t, err, "Failed to parse remove response")

		assert.Equal(t, "deleted", getString(result, "status"), "Status should be deleted")

		// Clear target ID so cleanup doesn't run again
		targetID = ""

		t.Logf("Org A successfully removed their target")
	})
}

// TestScenariosAccessibleToAll verifies that scenarios are readable by all org keys
func TestScenariosAccessibleToAll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	cfg := loadTestConfig(t)

	// Create two test organizations
	t.Log("Creating test organization A...")
	orgA := createTestOrg(t, cfg, "scen-orgA")
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, orgA)
	})

	t.Log("Creating test organization B...")
	orgB := createTestOrg(t, cfg, "scen-orgB")
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, orgB)
	})

	var scenariosA, scenariosB []map[string]interface{}

	// Both orgs should be able to list scenarios
	t.Run("OrgA_CanListScenarios", func(t *testing.T) {
		output, err := runOrgCLI(t, cfg, orgA.APISecret, "benchmark", "scenario", "list")
		require.NoError(t, err, "Org A should be able to list scenarios")

		err = json.Unmarshal([]byte(output), &scenariosA)
		require.NoError(t, err, "Failed to parse scenarios")

		t.Logf("Org A (%s) can see %d scenarios", orgA.OrgName, len(scenariosA))
	})

	t.Run("OrgB_CanListScenarios", func(t *testing.T) {
		output, err := runOrgCLI(t, cfg, orgB.APISecret, "benchmark", "scenario", "list")
		require.NoError(t, err, "Org B should be able to list scenarios")

		err = json.Unmarshal([]byte(output), &scenariosB)
		require.NoError(t, err, "Failed to parse scenarios")

		t.Logf("Org B (%s) can see %d scenarios", orgB.OrgName, len(scenariosB))
	})

	// Both orgs should see the same scenarios
	t.Run("BothOrgsSeeSameScenarios", func(t *testing.T) {
		assert.Equal(t, len(scenariosA), len(scenariosB), "Both orgs should see the same number of scenarios")

		// Check that scenario names match
		namesA := make(map[string]bool)
		for _, s := range scenariosA {
			namesA[getString(s, "name")] = true
		}

		for _, s := range scenariosB {
			name := getString(s, "name")
			assert.True(t, namesA[name], "Org B should see scenario %s that Org A sees", name)
		}

		t.Logf("Verified: Both orgs see the same %d scenarios", len(scenariosA))
	})
}
