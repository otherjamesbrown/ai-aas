//go:build e2e
// +build e2e

// Package e2e provides end-to-end tests for the ai-aas-org benchmark commands.
//
// These tests run against a live development environment and exercise
// the complete benchmark workflow. The tests create their own test
// organizations using ai-aas-cli, then test the ai-aas-org benchmark
// commands using org API keys.
//
// Prerequisites:
//   - Development environment running (Kubernetes cluster, services deployed)
//   - Master API key configured (via AI_AAS_API_KEY)
//   - ai-aas-cli and ai-aas-org binaries built
//
// Run with: go test -v -tags=e2e ./tests/e2e/...
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

// TestConfig holds e2e test configuration from environment variables
type TestConfig struct {
	MasterAPIKey    string
	UserOrgEndpoint string
	AdminEndpoint   string
	TLSInsecure     bool
}

// OrgState holds state for a test organization
type OrgState struct {
	OrgID     string
	OrgSlug   string
	OrgName   string
	UserID    string
	UserEmail string
	APIKeyID  string
	APISecret string
}

// BenchmarkState holds state for benchmark tests
type BenchmarkState struct {
	ScenarioName string
	TargetID     string
	TargetName   string
	RunID        string
}

// loadTestConfig loads test configuration from environment
func loadTestConfig(t *testing.T) *TestConfig {
	t.Helper()

	masterAPIKey := os.Getenv("AI_AAS_API_KEY")
	if masterAPIKey == "" {
		t.Skip("AI_AAS_API_KEY not set - skipping e2e tests. Run: source tests/e2e/env.sh")
	}

	userOrgEndpoint := os.Getenv("AI_AAS_USER_ORG_ENDPOINT")
	if userOrgEndpoint == "" {
		userOrgEndpoint = "https://user-org.dev.otherjamesbrown.com"
	}

	adminEndpoint := os.Getenv("AI_AAS_ADMIN_ENDPOINT")
	if adminEndpoint == "" {
		adminEndpoint = "https://admin-api.dev.otherjamesbrown.com"
	}

	tlsInsecure := os.Getenv("AI_AAS_TLS_INSECURE") == "true"

	return &TestConfig{
		MasterAPIKey:    masterAPIKey,
		UserOrgEndpoint: userOrgEndpoint,
		AdminEndpoint:   adminEndpoint,
		TLSInsecure:     tlsInsecure,
	}
}

// runAdminCLI executes the ai-aas-cli (admin CLI) with master API key
func runAdminCLI(t *testing.T, cfg *TestConfig, args ...string) (string, error) {
	t.Helper()

	cliBinary := findAdminCLIBinary(t)

	cmdArgs := append(args,
		"--user-org-endpoint", cfg.UserOrgEndpoint,
		"--api-key", cfg.MasterAPIKey,
		"--format", "json",
	)

	cmd := exec.Command(cliBinary, cmdArgs...)
	cmd.Env = append(os.Environ(),
		"AI_AAS_TLS_INSECURE="+fmt.Sprintf("%v", cfg.TLSInsecure),
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	t.Logf("Running admin CLI: %s %s", cliBinary, strings.Join(args, " "))

	err := cmd.Run()
	output := stdout.String()

	if err != nil {
		t.Logf("Admin CLI error: %v\nStderr: %s\nStdout: %s", err, stderr.String(), output)
		return output, fmt.Errorf("cli error: %w, stderr: %s", err, stderr.String())
	}

	return output, nil
}

// runOrgCLI executes the ai-aas-org CLI with an org API key
func runOrgCLI(t *testing.T, cfg *TestConfig, orgAPIKey string, args ...string) (string, error) {
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
	output := stdout.String()

	if err != nil {
		t.Logf("Org CLI error: %v\nStderr: %s\nStdout: %s", err, stderr.String(), output)
		return output, fmt.Errorf("cli error: %w, stderr: %s", err, stderr.String())
	}

	return output, nil
}

// findAdminCLIBinary locates the ai-aas-cli binary
func findAdminCLIBinary(t *testing.T) string {
	t.Helper()

	locations := []string{
		"./ai-aas-cli",
		"../../ai-aas-cli",
		"../../../ai-aas-cli/ai-aas-cli",
		"../../../../bin/ai-aas-cli",
		os.Getenv("AI_AAS_CLI_PATH"),
	}

	for _, loc := range locations {
		if loc == "" {
			continue
		}
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	path, err := exec.LookPath("ai-aas-cli")
	if err == nil {
		return path
	}

	t.Fatal("ai-aas-cli binary not found. Build with: go build -o ai-aas-cli ./services/ai-aas-cli/cmd/ai-aas-cli")
	return ""
}

// findOrgCLIBinary locates the ai-aas-org binary
func findOrgCLIBinary(t *testing.T) string {
	t.Helper()

	locations := []string{
		"./ai-aas-org",
		"../../ai-aas-org",
		"../../../ai-aas-org/ai-aas-org",
		"../../../../bin/ai-aas-org",
		os.Getenv("AI_AAS_ORG_CLI_PATH"),
	}

	for _, loc := range locations {
		if loc == "" {
			continue
		}
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	path, err := exec.LookPath("ai-aas-org")
	if err == nil {
		return path
	}

	t.Fatal("ai-aas-org binary not found. Build with: go build -o ai-aas-org ./services/ai-aas-org/cmd/ai-aas-org")
	return ""
}

// createTestOrg creates a test organization with user and API key
func createTestOrg(t *testing.T, cfg *TestConfig, suffix string) *OrgState {
	t.Helper()

	timestamp := time.Now().Unix()
	state := &OrgState{
		OrgSlug:   fmt.Sprintf("e2e-test-%s-%d", suffix, timestamp),
		OrgName:   fmt.Sprintf("E2E-Test Benchmark %s %d", suffix, timestamp),
		UserEmail: fmt.Sprintf("e2e-test-%s-%d@test.ai-aas.dev", suffix, timestamp),
	}

	// Create organization
	output, err := runAdminCLI(t, cfg,
		"org", "create",
		"--name", state.OrgName,
		"--slug", state.OrgSlug,
	)
	require.NoError(t, err, "Failed to create organization")

	var orgResult map[string]interface{}
	err = json.Unmarshal([]byte(output), &orgResult)
	require.NoError(t, err, "Failed to parse org create response")

	state.OrgID = getString(orgResult, "orgId")
	require.NotEmpty(t, state.OrgID, "Organization ID should not be empty")
	t.Logf("Created org: ID=%s, Slug=%s, Name=%s", state.OrgID, state.OrgSlug, state.OrgName)

	// Create user
	output, err = runAdminCLI(t, cfg,
		"user", "create",
		"--org-id", state.OrgSlug,
		"--email", state.UserEmail,
		"--roles", "admin",
	)
	require.NoError(t, err, "Failed to create user")

	var userResult map[string]interface{}
	err = json.Unmarshal([]byte(output), &userResult)
	require.NoError(t, err, "Failed to parse user create response")

	state.UserID = getString(userResult, "userId")
	require.NotEmpty(t, state.UserID, "User ID should not be empty")
	t.Logf("Created user: ID=%s, Email=%s", state.UserID, state.UserEmail)

	// Activate user
	_, err = runAdminCLI(t, cfg,
		"user", "update",
		"--org-id", state.OrgSlug,
		"--user-id", state.UserID,
		"--status", "active",
	)
	require.NoError(t, err, "Failed to activate user")

	// Create API key with benchmark scopes
	output, err = runAdminCLI(t, cfg,
		"apikey", "create",
		"--org-id", state.OrgSlug,
		"--user-id", state.UserID,
		"--scopes", "benchmarks:read,benchmarks:write,models:read",
	)
	require.NoError(t, err, "Failed to create API key")

	var keyResult map[string]interface{}
	err = json.Unmarshal([]byte(output), &keyResult)
	require.NoError(t, err, "Failed to parse API key create response")

	state.APIKeyID = getString(keyResult, "apiKeyId")
	state.APISecret = getString(keyResult, "secret")
	require.NotEmpty(t, state.APIKeyID, "API Key ID should not be empty")
	require.NotEmpty(t, state.APISecret, "API Secret should not be empty")
	t.Logf("Created API key: ID=%s", state.APIKeyID)

	return state
}

// cleanupTestOrg deletes a test organization and all its resources
func cleanupTestOrg(t *testing.T, cfg *TestConfig, state *OrgState) {
	if state == nil || state.OrgSlug == "" {
		return
	}

	t.Logf("Cleaning up test org: %s (%s)", state.OrgName, state.OrgSlug)

	_, err := runAdminCLI(t, cfg,
		"org", "delete",
		"--org-id", state.OrgSlug,
		"--force",
	)

	if err != nil {
		t.Logf("Warning: Failed to cleanup org %s: %v", state.OrgSlug, err)
	} else {
		t.Logf("Successfully cleaned up org: %s", state.OrgSlug)
	}
}

// TestBenchmarkWorkflow tests the complete benchmark workflow:
// 1. Create a test organization with API key
// 2. List scenarios (available to all orgs)
// 3. Create a target
// 4. Show target details
// 5. Start the target
// 6. Trigger a run
// 7. Show run details
// 8. Stop the target
// 9. Remove the target
// 10. Cleanup test organization
func TestBenchmarkWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	cfg := loadTestConfig(t)

	// Create test organization
	orgState := createTestOrg(t, cfg, "workflow")
	t.Cleanup(func() {
		cleanupTestOrg(t, cfg, orgState)
	})

	benchState := &BenchmarkState{}
	timestamp := time.Now().Unix()
	benchState.TargetName = fmt.Sprintf("e2e-target-%d", timestamp)

	// Cleanup benchmark target if test fails partway through
	t.Cleanup(func() {
		if benchState.TargetID != "" {
			runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "target", "remove", benchState.TargetName, "--force")
		}
	})

	// Run test steps in order
	t.Run("Step1_ListScenarios", func(t *testing.T) {
		testListScenarios(t, cfg, orgState, benchState)
	})

	if benchState.ScenarioName == "" {
		t.Skip("No scenarios available - skipping remaining tests")
	}

	t.Run("Step2_CreateTarget", func(t *testing.T) {
		testCreateTarget(t, cfg, orgState, benchState)
	})

	t.Run("Step3_ShowTarget", func(t *testing.T) {
		testShowTarget(t, cfg, orgState, benchState)
	})

	t.Run("Step4_ListTargets", func(t *testing.T) {
		testListTargets(t, cfg, orgState, benchState)
	})

	t.Run("Step5_StartTarget", func(t *testing.T) {
		testStartTarget(t, cfg, orgState, benchState)
	})

	t.Run("Step6_TriggerRun", func(t *testing.T) {
		testTriggerRun(t, cfg, orgState, benchState)
	})

	t.Run("Step7_ShowRun", func(t *testing.T) {
		testShowRun(t, cfg, orgState, benchState)
	})

	t.Run("Step8_ListRuns", func(t *testing.T) {
		testListRuns(t, cfg, orgState, benchState)
	})

	t.Run("Step9_StopTarget", func(t *testing.T) {
		testStopTarget(t, cfg, orgState, benchState)
	})

	t.Run("Step10_RemoveTarget", func(t *testing.T) {
		testRemoveTarget(t, cfg, orgState, benchState)
	})
}

// testListScenarios lists available benchmark scenarios
func testListScenarios(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "scenario", "list")
	require.NoError(t, err, "Failed to list scenarios")

	var scenarios []map[string]interface{}
	err = json.Unmarshal([]byte(output), &scenarios)
	require.NoError(t, err, "Failed to parse scenarios response")

	if len(scenarios) == 0 {
		t.Log("No scenarios available")
		return
	}

	benchState.ScenarioName = getString(scenarios[0], "name")
	require.NotEmpty(t, benchState.ScenarioName, "Scenario name should not be empty")

	t.Logf("Found %d scenarios, using: %s", len(scenarios), benchState.ScenarioName)

	// Test showing scenario details
	showOutput, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "scenario", "show", benchState.ScenarioName)
	require.NoError(t, err, "Failed to show scenario")

	var scenario map[string]interface{}
	err = json.Unmarshal([]byte(showOutput), &scenario)
	require.NoError(t, err, "Failed to parse scenario show response")
	assert.Equal(t, benchState.ScenarioName, getString(scenario, "name"), "Scenario name should match")
}

// testCreateTarget creates a benchmark target
func testCreateTarget(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.ScenarioName, "Scenario must be available first")

	modelName := "test-model"

	output, err := runOrgCLI(t, cfg, orgState.APISecret,
		"benchmark", "target", "add", benchState.TargetName,
		"--model", modelName,
		"--scenario", benchState.ScenarioName,
		"--environment", "development",
	)
	require.NoError(t, err, "Failed to create target")

	var target map[string]interface{}
	err = json.Unmarshal([]byte(output), &target)
	require.NoError(t, err, "Failed to parse target create response")

	benchState.TargetID = getString(target, "id")
	require.NotEmpty(t, benchState.TargetID, "Target ID should not be empty")

	t.Logf("Created target: ID=%s, Name=%s", benchState.TargetID, benchState.TargetName)
}

// testShowTarget shows target details
func testShowTarget(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.TargetName, "Target must be created first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "target", "show", benchState.TargetName)
	require.NoError(t, err, "Failed to show target")

	var target map[string]interface{}
	err = json.Unmarshal([]byte(output), &target)
	require.NoError(t, err, "Failed to parse target show response")

	assert.Equal(t, benchState.TargetID, getString(target, "id"), "Target ID should match")
	assert.Equal(t, benchState.TargetName, getString(target, "name"), "Target name should match")
}

// testListTargets lists all targets and verifies the created one appears
func testListTargets(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.TargetName, "Target must be created first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "target", "list")
	require.NoError(t, err, "Failed to list targets")

	var targets []map[string]interface{}
	err = json.Unmarshal([]byte(output), &targets)
	require.NoError(t, err, "Failed to parse targets list response")

	found := false
	for _, target := range targets {
		if getString(target, "id") == benchState.TargetID {
			found = true
			break
		}
	}
	assert.True(t, found, "Created target should appear in list")
}

// testStartTarget starts the benchmark target
func testStartTarget(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.TargetName, "Target must be created first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "target", "start", benchState.TargetName)
	require.NoError(t, err, "Failed to start target")

	var target map[string]interface{}
	err = json.Unmarshal([]byte(output), &target)
	require.NoError(t, err, "Failed to parse target start response")

	status := getString(target, "status")
	assert.Equal(t, "active", status, "Target status should be active after start")

	t.Logf("Started target: status=%s", status)
}

// testTriggerRun triggers a benchmark run
func testTriggerRun(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.TargetName, "Target must be created first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "run", "trigger", benchState.TargetName)
	require.NoError(t, err, "Failed to trigger run")

	var run map[string]interface{}
	err = json.Unmarshal([]byte(output), &run)
	require.NoError(t, err, "Failed to parse run trigger response")

	benchState.RunID = getString(run, "id")
	require.NotEmpty(t, benchState.RunID, "Run ID should not be empty")

	t.Logf("Triggered run: ID=%s", benchState.RunID)
}

// testShowRun shows run details
func testShowRun(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.RunID, "Run must be triggered first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "run", "show", benchState.RunID)
	require.NoError(t, err, "Failed to show run")

	var run map[string]interface{}
	err = json.Unmarshal([]byte(output), &run)
	require.NoError(t, err, "Failed to parse run show response")

	assert.Equal(t, benchState.RunID, getString(run, "id"), "Run ID should match")
	assert.Equal(t, benchState.TargetID, getString(run, "target_id"), "Target ID should match")
}

// testListRuns lists runs and verifies the triggered one appears
func testListRuns(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.RunID, "Run must be triggered first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "run", "list", "--target", benchState.TargetName)
	require.NoError(t, err, "Failed to list runs")

	var runs []map[string]interface{}
	err = json.Unmarshal([]byte(output), &runs)
	require.NoError(t, err, "Failed to parse runs list response")

	found := false
	for _, run := range runs {
		if getString(run, "id") == benchState.RunID {
			found = true
			break
		}
	}
	assert.True(t, found, "Triggered run should appear in list")
}

// testStopTarget stops the benchmark target
func testStopTarget(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.TargetName, "Target must be created first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "target", "stop", benchState.TargetName)
	require.NoError(t, err, "Failed to stop target")

	var target map[string]interface{}
	err = json.Unmarshal([]byte(output), &target)
	require.NoError(t, err, "Failed to parse target stop response")

	status := getString(target, "status")
	assert.Equal(t, "paused", status, "Target status should be paused after stop")

	t.Logf("Stopped target: status=%s", status)
}

// testRemoveTarget removes the benchmark target
func testRemoveTarget(t *testing.T, cfg *TestConfig, orgState *OrgState, benchState *BenchmarkState) {
	require.NotEmpty(t, benchState.TargetName, "Target must be created first")

	output, err := runOrgCLI(t, cfg, orgState.APISecret, "benchmark", "target", "remove", benchState.TargetName, "--force")
	require.NoError(t, err, "Failed to remove target")

	var result map[string]interface{}
	err = json.Unmarshal([]byte(output), &result)
	require.NoError(t, err, "Failed to parse remove response")

	assert.Equal(t, "deleted", getString(result, "status"), "Status should be deleted")

	benchState.TargetID = ""

	t.Logf("Removed target successfully")
}

// Helper functions

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
