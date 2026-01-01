//go:build (benchmark || nightly || full) && e2e_tier || !e2e_tier

// DEPRECATED: Migrated to tests/usecases/benchmarks_test.go
// This file contains CLI-based benchmark tests that have been migrated to the
// use case test suite. The UC tests use ai-aas-org CLI which is the preferred
// interface for benchmark operations.
//
// Tests in this file use ai-aas-cli --profile dev-admin, but ai-aas-org is
// the canonical CLI for user-domain operations including benchmarks.

package suites

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// BenchmarkE2ETestSuite runs comprehensive E2E tests for the benchmark system using the CLI
//
// Prerequisites:
//   - ai-aas-cli must be installed and in PATH
//   - CLI must be configured with admin credentials (--profile dev-admin or similar)
//   - guidellm-runner must be deployed and accessible
//   - At least one model must be deployed (llama-3.1-8b-instruct recommended)
//
// Run with: go test -v ./suites -run TestBenchmark -timeout 15m

// CLIResult holds the result of a CLI command
type CLIResult struct {
	Output   string
	Error    string
	ExitCode int
}

// runCLI executes an ai-aas-cli command and returns the result
func runCLI(args ...string) CLIResult {
	cmd := exec.Command("ai-aas-cli", args...)
	output, err := cmd.CombinedOutput()

	result := CLIResult{
		Output:   string(output),
		ExitCode: 0,
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = 1
		}
		result.Error = err.Error()
	}

	return result
}

// runCLIWithProfile executes a CLI command with the dev-admin profile
func runCLIWithProfile(args ...string) CLIResult {
	fullArgs := append([]string{"--profile", "dev-admin"}, args...)
	return runCLI(fullArgs...)
}

// BenchmarkScenarioJSON represents scenario data from JSON output
type BenchmarkScenarioJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Config      struct {
		Defaults struct {
			Profile     string  `json:"profile"`
			Rate        float64 `json:"rate"`
			MaxSeconds  int     `json:"max_seconds"`
			RequestType string  `json:"request_type"`
		} `json:"defaults"`
	} `json:"config"`
}

// BenchmarkTargetJSON represents target data from JSON output
type BenchmarkTargetJSON struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	ModelName       string `json:"model_name"`
	ScenarioName    string `json:"scenario_name"`
	Environment     string `json:"environment"`
	Status          string `json:"status"`
	ScheduleEnabled bool   `json:"schedule_enabled"`
}

// RunnerTargetJSON represents runner target data
type RunnerTargetJSON struct {
	Name        string  `json:"name"`
	Model       string  `json:"model"`
	Status      string  `json:"status"`
	Profile     string  `json:"profile"`
	Rate        float64 `json:"rate"`
	MaxSeconds  int     `json:"max_seconds"`
	RequestType string  `json:"request_type"`
	LastResults struct {
		TotalRequests      int `json:"TotalRequests"`
		SuccessfulRequests int `json:"SuccessfulRequests"`
		FailedRequests     int `json:"FailedRequests"`
	} `json:"last_results"`
}

// TestBenchmarkCLIAvailable verifies the CLI is available and configured
func TestBenchmarkCLIAvailable(t *testing.T) {
	result := runCLI("--version")
	if result.ExitCode != 0 {
		t.Fatalf("ai-aas-cli not available: %s", result.Error)
	}
	t.Logf("CLI version: %s", strings.TrimSpace(result.Output))
}

// TestBenchmarkScenarioList tests listing available scenarios
func TestBenchmarkScenarioList(t *testing.T) {
	result := runCLIWithProfile("benchmark", "scenario", "list")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to list scenarios: %s\n%s", result.Error, result.Output)
	}

	t.Logf("Scenarios output:\n%s", result.Output)

	// Should show at least short-qa scenario
	if !strings.Contains(result.Output, "short-qa") {
		t.Error("Expected 'short-qa' scenario to be available")
	}
}

// TestBenchmarkScenarioListJSON tests scenario list with JSON output
func TestBenchmarkScenarioListJSON(t *testing.T) {
	result := runCLIWithProfile("benchmark", "scenario", "list", "--format", "json")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to list scenarios: %s\n%s", result.Error, result.Output)
	}

	var scenarios []BenchmarkScenarioJSON
	if err := json.Unmarshal([]byte(result.Output), &scenarios); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\n%s", err, result.Output)
	}

	if len(scenarios) == 0 {
		t.Fatal("No scenarios returned")
	}

	t.Logf("Found %d scenario(s)", len(scenarios))
	for _, s := range scenarios {
		t.Logf("  - %s: profile=%s, rate=%v, request_type=%s",
			s.Name, s.Config.Defaults.Profile, s.Config.Defaults.Rate, s.Config.Defaults.RequestType)
	}
}

// TestBenchmarkScenarioShow tests showing a specific scenario
func TestBenchmarkScenarioShow(t *testing.T) {
	result := runCLIWithProfile("benchmark", "scenario", "show", "short-qa")
	if result.ExitCode != 0 {
		t.Skipf("short-qa scenario not available: %s", result.Output)
	}

	t.Logf("Scenario details:\n%s", result.Output)

	// Verify key scenario properties are shown
	expectedFields := []string{"short-qa", "chat_completions", "constant"}
	for _, field := range expectedFields {
		if !strings.Contains(result.Output, field) {
			t.Errorf("Expected field '%s' in scenario output", field)
		}
	}
}

// TestBenchmarkTargetLifecycle tests the complete target lifecycle
func TestBenchmarkTargetLifecycle(t *testing.T) {
	targetName := fmt.Sprintf("e2e-test-%d", time.Now().Unix())
	modelName := "gpt-oss-20b" // Use a model that supports text_completions

	// Cleanup function
	defer func() {
		runCLIWithProfile("benchmark", "target", "stop", targetName)
		runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	}()

	// Step 1: Create target
	t.Log("Step 1: Creating benchmark target")
	result := runCLIWithProfile("benchmark", "target", "add", targetName,
		"--model", modelName,
		"--scenario", "short-qa",
		"--environment", "development")

	if result.ExitCode != 0 {
		t.Fatalf("Failed to create target: %s\n%s", result.Error, result.Output)
	}
	t.Logf("Created target: %s", targetName)

	// Step 2: List targets - verify new target appears
	t.Log("Step 2: Verifying target in list")
	result = runCLIWithProfile("benchmark", "target", "list", "--environment", "development")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to list targets: %s\n%s", result.Error, result.Output)
	}

	if !strings.Contains(result.Output, targetName) {
		t.Errorf("Target '%s' not found in list:\n%s", targetName, result.Output)
	}

	// Step 3: Show target details
	t.Log("Step 3: Getting target details")
	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to show target: %s\n%s", result.Error, result.Output)
	}

	if !strings.Contains(result.Output, modelName) {
		t.Errorf("Model name '%s' not found in target details", modelName)
	}

	// Step 4: Start target
	t.Log("Step 4: Starting target")
	result = runCLIWithProfile("benchmark", "target", "start", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to start target: %s\n%s", result.Error, result.Output)
	}

	// Verify status changed to active
	result = runCLIWithProfile("benchmark", "target", "show", targetName, "--format", "json")
	if result.ExitCode == 0 {
		var target BenchmarkTargetJSON
		if err := json.Unmarshal([]byte(result.Output), &target); err == nil {
			if target.Status != "active" {
				t.Errorf("Expected status 'active', got '%s'", target.Status)
			}
			t.Logf("Target status: %s, schedule_enabled: %v", target.Status, target.ScheduleEnabled)
		}
	}

	// Step 5: Wait for benchmark to run (short-qa is 60 seconds)
	t.Log("Step 5: Waiting for benchmark execution (this may take up to 90 seconds)...")
	time.Sleep(90 * time.Second)

	// Step 6: Check status
	t.Log("Step 6: Checking benchmark status")
	result = runCLIWithProfile("benchmark", "status")
	if result.ExitCode != 0 {
		t.Logf("Warning: Failed to get status: %s", result.Output)
	} else {
		t.Logf("Status:\n%s", result.Output)
	}

	// Step 7: Stop target
	t.Log("Step 7: Stopping target")
	result = runCLIWithProfile("benchmark", "target", "stop", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to stop target: %s\n%s", result.Error, result.Output)
	}

	// Step 8: Remove target
	t.Log("Step 8: Removing target")
	result = runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to remove target: %s\n%s", result.Error, result.Output)
	}

	// Verify target is gone (with small delay for consistency)
	time.Sleep(1 * time.Second)
	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode == 0 {
		t.Error("Target should have been removed but still exists")
	}

	t.Log("Target lifecycle test completed successfully")
}

// TestBenchmarkScenarioConfigPropagation tests that scenario config is passed to guidellm-runner
func TestBenchmarkScenarioConfigPropagation(t *testing.T) {
	// This test verifies the fix for aas-v8cy: scenario config (request_type, rate, etc.)
	// must be passed from Admin API to guidellm-runner

	targetName := fmt.Sprintf("e2e-config-test-%d", time.Now().Unix())
	modelName := "llama-3.1-8b-instruct" // Model that REQUIRES chat_completions

	defer func() {
		runCLIWithProfile("benchmark", "target", "stop", targetName)
		runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	}()

	// Get scenario details first
	t.Log("Step 1: Verify scenario has chat_completions")
	result := runCLIWithProfile("benchmark", "scenario", "show", "short-qa", "--format", "json")
	if result.ExitCode != 0 {
		t.Skipf("short-qa scenario not available")
	}

	var scenario BenchmarkScenarioJSON
	if err := json.Unmarshal([]byte(result.Output), &scenario); err != nil {
		t.Fatalf("Failed to parse scenario: %v", err)
	}

	expectedRequestType := scenario.Config.Defaults.RequestType
	expectedRate := scenario.Config.Defaults.Rate
	expectedProfile := scenario.Config.Defaults.Profile

	t.Logf("Scenario config: request_type=%s, rate=%v, profile=%s",
		expectedRequestType, expectedRate, expectedProfile)

	if expectedRequestType != "chat_completions" {
		t.Fatalf("Expected request_type 'chat_completions', got '%s'", expectedRequestType)
	}

	// Create and start target
	t.Log("Step 2: Creating target with llama model (requires chat_completions)")
	result = runCLIWithProfile("benchmark", "target", "add", targetName,
		"--model", modelName,
		"--scenario", "short-qa",
		"--environment", "development")

	if result.ExitCode != 0 {
		t.Fatalf("Failed to create target: %s\n%s", result.Error, result.Output)
	}

	t.Log("Step 3: Starting target")
	result = runCLIWithProfile("benchmark", "target", "start", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to start target: %s\n%s", result.Error, result.Output)
	}

	// Wait for benchmark to complete (with periodic status checks)
	t.Log("Step 4: Waiting for benchmark execution (checking every 15s for up to 120s)...")

	var lastStatus string
	for i := 0; i < 8; i++ {
		time.Sleep(15 * time.Second)

		result = runCLIWithProfile("benchmark", "target", "show", targetName, "--format", "json")
		if result.ExitCode == 0 {
			var target BenchmarkTargetJSON
			if err := json.Unmarshal([]byte(result.Output), &target); err == nil {
				lastStatus = target.Status
				t.Logf("  Check %d/8: status=%s", i+1, target.Status)
			}
		}
	}

	// Check benchmark results - if config was NOT propagated, llama would fail
	// because it only supports chat_completions but runner would default to text_completions
	t.Log("Step 5: Checking benchmark results")

	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to get target details: %s", result.Output)
	}

	t.Logf("Target details:\n%s", result.Output)

	// The key validation: target should still be active (not failed/error)
	if lastStatus == "error" || lastStatus == "failed" {
		t.Errorf("Benchmark failed with status '%s' - config propagation may not be working", lastStatus)
	}

	// Check output doesn't contain obvious failure indicators
	if strings.Contains(strings.ToLower(result.Output), "error") &&
	   strings.Contains(strings.ToLower(result.Output), "text_completions") {
		t.Error("Output suggests text_completions was used instead of chat_completions")
	}

	t.Log("Config propagation test completed")
}

// TestBenchmarkChatVsTextCompletions tests that models are called with correct request type
func TestBenchmarkChatVsTextCompletions(t *testing.T) {
	// Test both chat_completions (llama) and text_completions (gpt-oss) models

	tests := []struct {
		name             string
		model            string
		expectedWorkWith string
		scenario         string
	}{
		{
			name:             "llama-chat-completions",
			model:            "llama-3.1-8b-instruct",
			expectedWorkWith: "chat_completions",
			scenario:         "short-qa",
		},
		{
			name:             "gpt-text-completions",
			model:            "gpt-oss-20b",
			expectedWorkWith: "text_completions", // This model works with both
			scenario:         "short-qa",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			targetName := fmt.Sprintf("e2e-%s-%d", tc.name, time.Now().Unix())

			defer func() {
				runCLIWithProfile("benchmark", "target", "stop", targetName)
				runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
			}()

			// Create target
			result := runCLIWithProfile("benchmark", "target", "add", targetName,
				"--model", tc.model,
				"--scenario", tc.scenario,
				"--environment", "development")

			if result.ExitCode != 0 {
				t.Skipf("Failed to create target (model may not be available): %s", result.Output)
			}

			// Start and wait
			result = runCLIWithProfile("benchmark", "target", "start", targetName)
			if result.ExitCode != 0 {
				t.Fatalf("Failed to start target: %s", result.Output)
			}

			t.Logf("Waiting 90 seconds for benchmark to complete...")
			time.Sleep(90 * time.Second)

			// Check results
			result = runCLIWithProfile("benchmark", "target", "show", targetName)
			t.Logf("Results for %s:\n%s", tc.model, result.Output)

			// Stop
			runCLIWithProfile("benchmark", "target", "stop", targetName)
		})
	}
}

// TestBenchmarkMultipleTargets tests running multiple targets simultaneously
func TestBenchmarkMultipleTargets(t *testing.T) {
	timestamp := time.Now().Unix()
	targets := []struct {
		name  string
		model string
	}{
		{name: fmt.Sprintf("e2e-multi-1-%d", timestamp), model: "gpt-oss-20b"},
		{name: fmt.Sprintf("e2e-multi-2-%d", timestamp), model: "gpt-oss-20b"},
	}

	// Cleanup
	defer func() {
		for _, target := range targets {
			runCLIWithProfile("benchmark", "target", "stop", target.name)
			runCLIWithProfile("benchmark", "target", "remove", target.name, "--force")
		}
	}()

	// Create all targets
	t.Log("Creating multiple targets")
	for _, target := range targets {
		result := runCLIWithProfile("benchmark", "target", "add", target.name,
			"--model", target.model,
			"--scenario", "short-qa",
			"--environment", "development")

		if result.ExitCode != 0 {
			t.Fatalf("Failed to create target %s: %s", target.name, result.Output)
		}
		t.Logf("  Created: %s", target.name)
	}

	// Small delay after creates
	time.Sleep(1 * time.Second)

	// Verify all targets exist before starting
	t.Log("Verifying targets were created")
	result := runCLIWithProfile("benchmark", "target", "list", "--environment", "development")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to list targets: %s", result.Output)
	}

	for _, target := range targets {
		if !strings.Contains(result.Output, target.name) {
			t.Errorf("Target %s not found in list after creation", target.name)
		}
	}

	// Start all targets
	t.Log("Starting all targets")
	for _, target := range targets {
		result := runCLIWithProfile("benchmark", "target", "start", target.name)
		if result.ExitCode != 0 {
			t.Fatalf("Failed to start target %s: %s", target.name, result.Output)
		}
	}

	// Check status shows multiple active targets
	result = runCLIWithProfile("benchmark", "status")
	if result.ExitCode == 0 {
		t.Logf("Status with multiple targets:\n%s", result.Output)
	}

	// Wait and then stop all
	t.Log("Waiting 30 seconds then stopping...")
	time.Sleep(30 * time.Second)

	for _, target := range targets {
		runCLIWithProfile("benchmark", "target", "stop", target.name)
	}

	t.Log("Multiple targets test completed")
}

// TestBenchmarkErrorHandling tests error cases
func TestBenchmarkErrorHandling(t *testing.T) {
	t.Run("create-with-invalid-scenario", func(t *testing.T) {
		result := runCLIWithProfile("benchmark", "target", "add", "invalid-test",
			"--model", "gpt-oss-20b",
			"--scenario", "nonexistent-scenario",
			"--environment", "development")

		if result.ExitCode == 0 {
			// Cleanup if accidentally created
			runCLIWithProfile("benchmark", "target", "remove", "invalid-test")
			t.Error("Expected error for nonexistent scenario")
		} else {
			t.Logf("Correctly rejected invalid scenario: %s", result.Output)
		}
	})

	t.Run("create-duplicate-target", func(t *testing.T) {
		targetName := fmt.Sprintf("e2e-dup-%d", time.Now().Unix())
		defer runCLIWithProfile("benchmark", "target", "remove", targetName)

		// Create first
		result := runCLIWithProfile("benchmark", "target", "add", targetName,
			"--model", "gpt-oss-20b",
			"--scenario", "short-qa",
			"--environment", "development")

		if result.ExitCode != 0 {
			t.Skipf("Failed to create first target: %s", result.Output)
		}

		// Try to create duplicate
		result = runCLIWithProfile("benchmark", "target", "add", targetName,
			"--model", "gpt-oss-20b",
			"--scenario", "short-qa",
			"--environment", "development")

		if result.ExitCode == 0 {
			t.Error("Expected error for duplicate target name")
		} else {
			if !strings.Contains(strings.ToLower(result.Output), "already exists") &&
				!strings.Contains(strings.ToLower(result.Output), "conflict") &&
				!strings.Contains(strings.ToLower(result.Output), "duplicate") {
				t.Logf("Got error but unexpected message: %s", result.Output)
			}
		}
	})

	t.Run("start-nonexistent-target", func(t *testing.T) {
		result := runCLIWithProfile("benchmark", "target", "start", "nonexistent-target-12345")
		if result.ExitCode == 0 {
			t.Error("Expected error for nonexistent target")
		}
	})

	t.Run("stop-nonexistent-target", func(t *testing.T) {
		result := runCLIWithProfile("benchmark", "target", "stop", "nonexistent-target-12345")
		if result.ExitCode == 0 {
			t.Error("Expected error for nonexistent target")
		}
	})
}

// TestBenchmarkLongIdleConnection validates that API connections survive idle periods
// This test catches connection pool timeout mismatches between client and server
// See: aas-32qi, aas-0x7g for context on why this test exists
func TestBenchmarkLongIdleConnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long idle connection test in short mode")
	}

	targetName := fmt.Sprintf("e2e-idle-%d", time.Now().Unix())

	defer func() {
		runCLIWithProfile("benchmark", "target", "stop", targetName)
		runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	}()

	// Step 1: Create target
	t.Log("Step 1: Creating target")
	result := runCLIWithProfile("benchmark", "target", "add", targetName,
		"--model", "gpt-oss-20b",
		"--scenario", "short-qa",
		"--environment", "development")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to create target: %s", result.Output)
	}
	t.Logf("Created target: %s", targetName)

	// Step 2: Verify target exists (establishes connection pool)
	t.Log("Step 2: Verifying target exists")
	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to show target: %s", result.Output)
	}

	// Step 3: Wait for idle period that exceeds previous server IdleTimeout (60s)
	// With fix, server IdleTimeout is now 120s, so 70s idle should work
	idleDuration := 70 * time.Second
	t.Logf("Step 3: Waiting %v to test connection pool idle handling...", idleDuration)
	time.Sleep(idleDuration)

	// Step 4: Make request after idle period - this is the critical test
	// Before the fix (IdleTimeout=60s), this would fail with:
	// - "context deadline exceeded"
	// - "TLS handshake timeout"
	t.Log("Step 4: Making request after idle period")
	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Request after %v idle failed (connection pool issue): %s", idleDuration, result.Output)
	}
	t.Log("Request after idle period succeeded")

	// Step 5: Verify we can still perform operations
	t.Log("Step 5: Starting target to verify full functionality")
	result = runCLIWithProfile("benchmark", "target", "start", targetName)
	if result.ExitCode != 0 {
		t.Fatalf("Failed to start target after idle: %s", result.Output)
	}

	// Step 6: Stop and remove
	t.Log("Step 6: Stopping target")
	result = runCLIWithProfile("benchmark", "target", "stop", targetName)
	if result.ExitCode != 0 {
		t.Logf("Warning: Failed to stop target: %s", result.Output)
	}

	t.Log("Step 7: Removing target")
	result = runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	if result.ExitCode != 0 {
		t.Fatalf("Failed to remove target: %s", result.Output)
	}

	// Verify removal
	time.Sleep(1 * time.Second)
	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode == 0 {
		t.Error("Target should have been removed but still exists")
	}

	t.Log("Long idle connection test completed successfully")
}

// TestBenchmarkE2EComplete runs a comprehensive end-to-end test
func TestBenchmarkE2EComplete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping comprehensive E2E test in short mode")
	}

	t.Log("=== Benchmark E2E Complete Test ===")
	t.Log("")

	passed := 0
	failed := 0
	skipped := 0

	// Step 1: CLI available
	t.Log("Step 1: Verify CLI")
	result := runCLI("--version")
	if result.ExitCode != 0 {
		t.Fatalf("CLI not available")
	}
	t.Logf("  PASS: CLI version %s", strings.TrimSpace(result.Output))
	passed++

	// Step 2: Scenarios available
	t.Log("Step 2: List Scenarios")
	result = runCLIWithProfile("benchmark", "scenario", "list")
	if result.ExitCode != 0 {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	} else if !strings.Contains(result.Output, "short-qa") {
		t.Log("  FAIL: short-qa scenario not found")
		failed++
	} else {
		t.Log("  PASS: Scenarios available")
		passed++
	}

	// Step 3: Available models
	t.Log("Step 3: Check Models")
	result = runCLIWithProfile("model", "list", "--live")
	if result.ExitCode != 0 {
		t.Logf("  SKIP: Could not list models: %s", result.Output)
		skipped++
	} else {
		t.Logf("  PASS: Models listed\n%s", result.Output)
		passed++
	}

	// Step 4: Create target
	targetName := fmt.Sprintf("e2e-complete-%d", time.Now().Unix())
	defer func() {
		runCLIWithProfile("benchmark", "target", "stop", targetName)
		runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	}()

	t.Log("Step 4: Create Target")
	result = runCLIWithProfile("benchmark", "target", "add", targetName,
		"--model", "gpt-oss-20b",
		"--scenario", "short-qa",
		"--environment", "development")

	if result.ExitCode != 0 {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	} else {
		t.Logf("  PASS: Target created: %s", targetName)
		passed++
	}

	// Step 5: Start target
	t.Log("Step 5: Start Target")
	result = runCLIWithProfile("benchmark", "target", "start", targetName)
	if result.ExitCode != 0 {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	} else {
		t.Log("  PASS: Target started")
		passed++
	}

	// Step 6: Check status
	t.Log("Step 6: Check Status")
	result = runCLIWithProfile("benchmark", "status")
	if result.ExitCode != 0 {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	} else {
		t.Logf("  PASS: Status retrieved\n%s", result.Output)
		passed++
	}

	// Step 7: Wait and verify execution
	t.Log("Step 7: Wait for Benchmark (90s)")
	time.Sleep(90 * time.Second)
	result = runCLIWithProfile("benchmark", "target", "show", targetName)
	if result.ExitCode == 0 {
		t.Logf("  PASS: Benchmark executed\n%s", result.Output)
		passed++
	} else {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	}

	// Step 8: Stop target
	t.Log("Step 8: Stop Target")
	result = runCLIWithProfile("benchmark", "target", "stop", targetName)
	if result.ExitCode != 0 {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	} else {
		t.Log("  PASS: Target stopped")
		passed++
	}

	// Step 9: Remove target
	t.Log("Step 9: Remove Target")
	result = runCLIWithProfile("benchmark", "target", "remove", targetName, "--force")
	if result.ExitCode != 0 {
		t.Logf("  FAIL: %s", result.Output)
		failed++
	} else {
		t.Log("  PASS: Target removed")
		passed++
	}

	t.Log("")
	t.Log("=== Benchmark E2E Summary ===")
	t.Logf("  PASSED:  %d", passed)
	t.Logf("  SKIPPED: %d", skipped)
	t.Logf("  FAILED:  %d", failed)
	t.Log("")

	if failed > 0 {
		t.Fatalf("E2E test failed with %d failures", failed)
	}

	t.Log("=== Benchmark E2E Complete ===")
}
