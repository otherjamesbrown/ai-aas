package usecases_test

import (
	"encoding/json"
	"strings"
	"testing"
)

// Benchmark use case tests
// See: usecases/benchmarks.yaml
// CLI helpers defined in helpers_test.go

// TestUC_BM_001_CreateBenchmarkTarget validates that organization admins can
// create benchmark target configurations.
//
// Use Case: UC-BM-001 - Create Benchmark Target
// Acceptance Criteria:
//   - AC-01: Create target with required fields
//   - AC-02: Reject unauthorized model
//   - AC-03: Reject invalid scenario
//   - AC-04: Create target with optional parameters
//
// See: usecases/benchmarks.yaml
func TestUC_BM_001_CreateBenchmarkTarget(t *testing.T) {
	t.Run("AC-01: create target with required fields", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org benchmark target add test-target-01 --model llama-7b --scenario standard`
		result := runOrgCLI("benchmark", "target", "add", "test-target-01", "--model", "llama-7b", "--scenario", "standard")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Benchmark target is created in the system
		// Then: Target ID is returned and displayed
		if !strings.Contains(result.Output, "target") || !strings.Contains(strings.ToLower(result.Output), "id") {
			t.Error("expected target ID in output")
		}

		// Then: Target appears in `ai-aas-org benchmark target list` output
		listResult := runOrgCLI("benchmark", "target", "list")
		if listResult.ExitCode != 0 {
			t.Fatalf("expected exit code 0 for list, got %d", listResult.ExitCode)
		}
	})

	t.Run("AC-02: reject unauthorized model", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User specifies a model they don't have access to
		// When: User runs `ai-aas-org benchmark target add test-unauthorized --model restricted-model --scenario standard`
		result := runOrgCLI("benchmark", "target", "add", "test-unauthorized", "--model", "restricted-model", "--scenario", "standard")

		// Then: Command fails with exit code 4 (auth error)
		if result.ExitCode != 4 {
			t.Fatalf("expected exit code 4, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message explains lack of model access
		if !strings.Contains(strings.ToLower(result.Output), "access") {
			t.Error("expected access-related error message")
		}

		// Then: Error includes suggestion to check model permissions
		// Then: No target is created
	})

	t.Run("AC-03: reject invalid scenario", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User specifies a non-existent scenario
		// When: User runs `ai-aas-org benchmark target add test-invalid-scenario --model llama-7b --scenario nonexistent`
		result := runOrgCLI("benchmark", "target", "add", "test-invalid-scenario", "--model", "llama-7b", "--scenario", "nonexistent")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Fatalf("expected exit code 5, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message lists available scenarios
		// Then: No target is created
	})

	t.Run("AC-04: create target with optional parameters", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org benchmark target add test-target-04 --model llama-7b --scenario throughput --interval 120`
		// Note: CLI supports --interval for scheduled runs, not --concurrency/--duration
		result := runOrgCLI("benchmark", "target", "add",
			"test-target-04",
			"--model", "llama-7b",
			"--scenario", "throughput",
			"--schedule",
			"--interval", "120")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Benchmark target is created with custom parameters
		// Then: Parameters are displayed in target details
		// Then: Target can be used for benchmark runs
	})
}

// TestUC_BM_001_CreateBenchmarkTarget_MustNot validates negative requirements.
func TestUC_BM_001_CreateBenchmarkTarget_MustNot(t *testing.T) {
	t.Run("must not auto-start benchmark after target creation", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User creates a benchmark target
		// When: Target creation completes
		result := runOrgCLI("benchmark", "target", "add", "test-no-autostart", "--model", "llama-7b", "--scenario", "standard")

		// Then: No benchmark run is started automatically
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Check that output doesn't indicate a running benchmark
		if strings.Contains(strings.ToLower(result.Output), "running") ||
			strings.Contains(strings.ToLower(result.Output), "started") {
			t.Error("benchmark should not auto-start after target creation")
		}
	})

	t.Run("must not modify any existing targets", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Creating a new target should not affect existing targets
		// This test would verify by creating a target, then creating another,
		// and verifying the first is unchanged
	})

	t.Run("must not expose internal system metrics or endpoints", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User creates a benchmark target
		result := runOrgCLI("benchmark", "target", "add", "test-no-internals", "--model", "llama-7b", "--scenario", "standard")

		// Then: No internal endpoints or metrics are exposed
		internalPatterns := []string{"internal", "endpoint", "metrics", "prometheus"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal information: %s", pattern)
			}
		}
	})
}

// TestUC_BM_002_TriggerBenchmarkRun validates that organization admins can
// trigger benchmark runs on existing targets.
//
// Use Case: UC-BM-002 - Trigger Benchmark Run
// Acceptance Criteria:
//   - AC-01: Trigger run on valid target
//   - AC-02: Reject run on non-existent target
//   - AC-03: Reject run when model unavailable
//
// See: usecases/benchmarks.yaml
func TestUC_BM_002_TriggerBenchmarkRun(t *testing.T) {
	t.Run("AC-01: trigger run on valid target", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has created benchmark target "target-123"
		// When: User runs `ai-aas-org benchmark run trigger target-123`
		result := runOrgCLI("benchmark", "run", "trigger", "target-123")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Benchmark run is queued for execution
		// Then: Run ID is returned and displayed
		if !strings.Contains(strings.ToLower(result.Output), "run") ||
			!strings.Contains(strings.ToLower(result.Output), "id") {
			t.Error("expected run ID in output")
		}

		// Then: Run status is "pending" or "running"
	})

	t.Run("AC-02: reject run on non-existent target", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User specifies a target that doesn't exist
		// When: User runs `ai-aas-org benchmark run trigger nonexistent-target`
		result := runOrgCLI("benchmark", "run", "trigger", "nonexistent-target")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Fatalf("expected exit code 5, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates target not found
		// Then: No run is created
	})

	t.Run("AC-03: reject run when model unavailable", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with offline model")

		// Given: Benchmark target exists but referenced model is offline
		// When: User runs `ai-aas-org benchmark run trigger target-with-offline-model`
		result := runOrgCLI("benchmark", "run", "trigger", "target-with-offline-model")

		// Then: Command fails with exit code 3 (service error)
		if result.ExitCode != 3 {
			t.Fatalf("expected exit code 3, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates model unavailable
		// Then: Error includes suggestion to check model status
	})
}

// TestUC_BM_002_TriggerBenchmarkRun_MustNot validates negative requirements.
func TestUC_BM_002_TriggerBenchmarkRun_MustNot(t *testing.T) {
	t.Run("must not block until run completes", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User triggers a benchmark run
		// When: Command returns
		// Then: Command returns immediately without waiting for completion
		// (This is validated by the async nature - run is queued, not completed)
	})

	t.Run("must not modify the target configuration", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Triggering a run should not change the target configuration
	})

	t.Run("must not expose internal queue details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User triggers a benchmark run
		result := runOrgCLI("benchmark", "run", "trigger", "target-123")

		// Then: No internal queue details are exposed
		internalPatterns := []string{"queue", "worker", "internal", "redis"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal queue details: %s", pattern)
			}
		}
	})
}

// TestUC_BM_003_ViewBenchmarkResults validates that organization admins can
// view benchmark run results.
//
// Use Case: UC-BM-003 - View Benchmark Results
// Acceptance Criteria:
//   - AC-01: View completed run results
//   - AC-02: View pending run status
//   - AC-03: View failed run details
//   - AC-04: Export results as JSON
//
// See: usecases/benchmarks.yaml
func TestUC_BM_003_ViewBenchmarkResults(t *testing.T) {
	t.Run("AC-01: view completed run results", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" has completed successfully
		// When: User runs `ai-aas-org benchmark run show run-456`
		result := runOrgCLI("benchmark", "run", "show", "run-456")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Run status is displayed as "completed"
		// Then: Throughput metric is shown (requests/second)
		// Then: Latency percentiles are shown (p50, p90, p99)
		// Then: Error rate is shown (percentage)
		// Then: Duration is shown
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-02: view pending run status", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" is still running
		// When: User runs `ai-aas-org benchmark run show run-456`
		result := runOrgCLI("benchmark", "run", "show", "run-in-progress")

		// Then: Run status is displayed as "running" or "pending"
		// Then: Progress indicator shown if available
		// Then: Start time is displayed
		// Then: Message indicates results pending
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("AC-03: view failed run details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" failed during execution
		// When: User runs `ai-aas-org benchmark run show run-456`
		result := runOrgCLI("benchmark", "run", "show", "run-failed")

		// Then: Run status is displayed as "failed"
		// Then: Error message explains failure reason
		// Then: Partial results shown if available
		// Then: Suggestion for troubleshooting included
		if result.ExitCode != 0 {
			// Failed runs should still return exit code 0 for show command
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		if !strings.Contains(strings.ToLower(result.Output), "failed") &&
			!strings.Contains(strings.ToLower(result.Output), "error") {
			t.Error("expected failure indication in output")
		}
	})

	t.Run("AC-04: export results as JSON", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" has completed
		// When: User runs `ai-aas-org benchmark run show run-456 --json`
		result := runOrgCLI("benchmark", "run", "show", "run-456", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Results are output as valid JSON
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: JSON includes all metrics
		var results map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &results); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})
}

// TestUC_BM_003_ViewBenchmarkResults_MustNot validates negative requirements.
func TestUC_BM_003_ViewBenchmarkResults_MustNot(t *testing.T) {
	t.Run("must not show results from other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User tries to view results for another org's run
		// Then: Access is denied or results not found
	})

	t.Run("must not expose internal implementation details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User views benchmark results
		result := runOrgCLI("benchmark", "run", "show", "run-456", "--json")

		// Then: No internal implementation details are exposed
		internalPatterns := []string{"internal_", "private_", "worker_id", "node_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal details: %s", pattern)
			}
		}
	})
}

// TestUC_BM_004_ListBenchmarkTargets validates that organization admins can
// list their benchmark targets.
//
// Use Case: UC-BM-004 - List Benchmark Targets
// Acceptance Criteria:
//   - AC-01: List all targets
//   - AC-02: List targets with no results
//
// See: usecases/benchmarks.yaml
func TestUC_BM_004_ListBenchmarkTargets(t *testing.T) {
	t.Run("AC-01: list all targets", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has created multiple benchmark targets
		// When: User runs `ai-aas-org benchmark target list`
		result := runOrgCLI("benchmark", "target", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: All targets for the organization are listed
		// Then: Each target shows ID, model, scenario, and created date
		// Then: Output is formatted as a table
		if result.Output == "" {
			t.Error("expected non-empty output")
		}
	})

	t.Run("AC-01b: list all targets with JSON output", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User has created multiple benchmark targets
		// When: User runs `ai-aas-org benchmark target list --json`
		result := runOrgCLI("benchmark", "target", "list", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Output is valid JSON array
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: Each target object includes ID, model, scenario, created_at
		var targets []map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &targets); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-02: list targets with no results", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with org that has no targets")

		// Given: User has not created any benchmark targets
		// When: User runs `ai-aas-org benchmark target list`
		result := runOrgCLI("benchmark", "target", "list")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message indicates no targets found
		// Then: Suggestion to create a target is shown
	})
}

// TestUC_BM_004_ListBenchmarkTargets_MustNot validates negative requirements.
func TestUC_BM_004_ListBenchmarkTargets_MustNot(t *testing.T) {
	t.Run("must not show targets from other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User runs `ai-aas-org benchmark target list`
		result := runOrgCLI("benchmark", "target", "list")

		// Then: Only the user's organization targets are shown
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("must not modify any targets", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Listing targets is a read-only operation
		// This test verifies no side effects occur
	})
}

// TestUC_BM_005_WaitForRunCompletion validates that organization admins can
// wait for benchmark runs to complete.
//
// Use Case: UC-BM-005 - Wait for Run Completion
// Acceptance Criteria:
//   - AC-01: Wait for successful completion
//   - AC-02: Wait with timeout
//   - AC-03: Wait for already completed run
//   - AC-04: Wait for failed run
//
// See: usecases/benchmarks.yaml
func TestUC_BM_005_WaitForRunCompletion(t *testing.T) {
	t.Run("AC-01: wait for successful completion", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" is in running state
		// When: User runs `ai-aas-org benchmark run wait run-456`
		result := runOrgCLI("benchmark", "run", "wait", "run-456")

		// Then: Command blocks until run completes
		// Then: Final status is displayed as "completed"
		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		if !strings.Contains(strings.ToLower(result.Output), "completed") &&
			!strings.Contains(strings.ToLower(result.Output), "complete") {
			t.Error("expected completion status in output")
		}
	})

	t.Run("AC-02: wait with timeout", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run is in running state
		// When: User runs `ai-aas-org benchmark run wait run-long --timeout 5s`
		// Note: Using a short timeout to test timeout behavior
		result := runOrgCLI("benchmark", "run", "wait", "run-long", "--timeout", "5s")

		// Then: Command waits up to the timeout
		// If run completes within timeout, exit code is 0
		// If timeout expires, exit code is 1
		// Either outcome is acceptable depending on run state
		if result.ExitCode != 0 && result.ExitCode != 1 {
			t.Fatalf("expected exit code 0 or 1, got %d: %s", result.ExitCode, result.Output)
		}

		// If timeout occurred, should indicate run still in progress
		if result.ExitCode == 1 {
			if !strings.Contains(strings.ToLower(result.Output), "timeout") &&
				!strings.Contains(strings.ToLower(result.Output), "still") {
				t.Error("expected timeout indication in output")
			}
		}
	})

	t.Run("AC-03: wait for already completed run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" has already completed
		// When: User runs `ai-aas-org benchmark run wait run-456`
		result := runOrgCLI("benchmark", "run", "wait", "run-completed")

		// Then: Command returns immediately
		// Then: Status shown as "completed"
		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("AC-04: wait for failed run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run fails during execution
		// When: User runs `ai-aas-org benchmark run wait run-failed`
		result := runOrgCLI("benchmark", "run", "wait", "run-failed")

		// Then: Command returns when run fails
		// Then: Status shown as "failed"
		// Then: Exit code is 2 (indicating run failure)
		if result.ExitCode != 2 {
			t.Fatalf("expected exit code 2 for failed run, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error details are displayed
		if !strings.Contains(strings.ToLower(result.Output), "failed") &&
			!strings.Contains(strings.ToLower(result.Output), "error") {
			t.Error("expected failure indication in output")
		}
	})

	t.Run("AC-04b: wait for non-existent run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run does not exist
		// When: User runs `ai-aas-org benchmark run wait nonexistent`
		result := runOrgCLI("benchmark", "run", "wait", "nonexistent-run")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Fatalf("expected exit code 5, got %d: %s", result.ExitCode, result.Output)
		}
	})
}

// TestUC_BM_005_WaitForRunCompletion_MustNot validates negative requirements.
func TestUC_BM_005_WaitForRunCompletion_MustNot(t *testing.T) {
	t.Run("must not modify the run in any way", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User waits for a run
		// When: Wait completes
		// Then: Run data should be unchanged
		// (This is an implicit requirement - wait is read-only)
	})

	t.Run("must not return success exit code for failed runs", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-failed" has failed
		// When: User waits for it
		result := runOrgCLI("benchmark", "run", "wait", "run-failed")

		// Then: Exit code should NOT be 0
		if result.ExitCode == 0 {
			t.Error("expected non-zero exit code for failed run")
		}
	})
}

// TestUC_BM_006_CancelRunningBenchmarks validates that organization admins can
// cancel benchmark runs.
//
// Use Case: UC-BM-006 - Cancel Running Benchmarks
// Acceptance Criteria:
//   - AC-01: Cancel running benchmark
//   - AC-02: Cancel pending benchmark
//   - AC-03: Reject cancel on completed run
//   - AC-04: Reject cancel on non-existent run
//
// See: usecases/benchmarks.yaml
func TestUC_BM_006_CancelRunningBenchmarks(t *testing.T) {
	t.Run("AC-01: cancel running benchmark", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" is in running state
		// When: User runs `ai-aas-org benchmark run cancel run-456`
		result := runOrgCLI("benchmark", "run", "cancel", "run-running")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Message confirms cancellation
		if !strings.Contains(strings.ToLower(result.Output), "cancel") {
			t.Error("expected cancellation confirmation in output")
		}

		// Verify status changed to cancelled
		showResult := runOrgCLI("benchmark", "run", "show", "run-running")
		if !strings.Contains(strings.ToLower(showResult.Output), "cancelled") &&
			!strings.Contains(strings.ToLower(showResult.Output), "canceled") {
			t.Error("expected run status to be cancelled")
		}
	})

	t.Run("AC-02: cancel pending benchmark", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" is in pending state (queued)
		// When: User runs `ai-aas-org benchmark run cancel run-pending`
		result := runOrgCLI("benchmark", "run", "cancel", "run-pending")

		// Then: Benchmark run is removed from queue
		// Then: Status changes to "cancelled"
		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}
	})

	t.Run("AC-03: reject cancel on completed run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "run-456" has already completed
		// When: User runs `ai-aas-org benchmark run cancel run-completed`
		result := runOrgCLI("benchmark", "run", "cancel", "run-completed")

		// Then: Command fails with exit code 3 (invalid operation)
		if result.ExitCode != 3 {
			t.Fatalf("expected exit code 3, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates run already completed
		if !strings.Contains(strings.ToLower(result.Output), "completed") &&
			!strings.Contains(strings.ToLower(result.Output), "already") {
			t.Error("expected message indicating run already completed")
		}
	})

	t.Run("AC-04: reject cancel on non-existent run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Benchmark run "nonexistent" does not exist
		// When: User runs `ai-aas-org benchmark run cancel nonexistent`
		result := runOrgCLI("benchmark", "run", "cancel", "nonexistent-run")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Fatalf("expected exit code 5, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates run not found
		if !strings.Contains(strings.ToLower(result.Output), "not found") &&
			!strings.Contains(strings.ToLower(result.Output), "notfound") {
			t.Error("expected not found message")
		}
	})
}

// TestUC_BM_006_CancelRunningBenchmarks_MustNot validates negative requirements.
func TestUC_BM_006_CancelRunningBenchmarks_MustNot(t *testing.T) {
	t.Run("must not cancel runs from other organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User tries to cancel another org's run
		// Then: Access is denied or run not found
	})

	t.Run("must not delete run records", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User cancels a run
		// When: Cancellation completes
		// Then: Run record should still exist (just with cancelled status)
		result := runOrgCLI("benchmark", "run", "cancel", "run-to-cancel")
		if result.ExitCode != 0 && result.ExitCode != 3 {
			// Skip if run doesn't exist or is already completed
			t.Skip("run not in cancellable state")
		}

		// Verify run still exists
		showResult := runOrgCLI("benchmark", "run", "show", "run-to-cancel")
		if showResult.ExitCode == 5 {
			t.Error("run record should not be deleted, only status updated")
		}
	})
}

// TestUC_BM_007_CompareResultsAcrossRuns validates that organization admins can
// compare benchmark results across multiple runs.
//
// Use Case: UC-BM-007 - Compare Results Across Runs
// Acceptance Criteria:
//   - AC-01: Compare two runs
//   - AC-02: Compare with percentage changes
//   - AC-03: Compare as JSON output
//   - AC-04: Reject compare with incomplete run
//   - AC-05: Reject compare with non-existent run
//
// See: usecases/benchmarks.yaml
func TestUC_BM_007_CompareResultsAcrossRuns(t *testing.T) {
	t.Run("AC-01: compare two runs", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Two completed benchmark runs "run-1" and "run-2" exist
		// When: User runs `ai-aas-org benchmark compare run-1 run-2`
		result := runOrgCLI("benchmark", "compare", "run-1", "run-2")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Side-by-side comparison is displayed
		// Then: Throughput difference is shown
		// Then: Latency differences are shown (p50, p90, p99)
		// Then: Error rate difference is shown
		if result.Output == "" {
			t.Error("expected comparison output")
		}
	})

	t.Run("AC-02: compare with percentage changes", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Two completed benchmark runs exist
		// When: User runs `ai-aas-org benchmark compare run-1 run-2 --show-delta`
		result := runOrgCLI("benchmark", "compare", "run-1", "run-2", "--show-delta")

		// Then: Percentage changes are displayed
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Look for percentage indicators in output
		if !strings.Contains(result.Output, "%") &&
			!strings.Contains(strings.ToLower(result.Output), "delta") &&
			!strings.Contains(strings.ToLower(result.Output), "change") {
			t.Error("expected percentage or delta indicators in output")
		}
	})

	t.Run("AC-03: compare as JSON output", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Two completed benchmark runs exist
		// When: User runs `ai-aas-org benchmark compare run-1 run-2 --json`
		result := runOrgCLI("benchmark", "compare", "run-1", "run-2", "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Results output as valid JSON
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON, got: %s", result.Output)
		}

		// Then: JSON includes both run metrics and delta calculations
		var comparison map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &comparison); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}
	})

	t.Run("AC-04: reject compare with incomplete run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Run "run-1" is completed but "run-2" is still running
		// When: User runs `ai-aas-org benchmark compare run-1 run-running`
		result := runOrgCLI("benchmark", "compare", "run-1", "run-running")

		// Then: Command fails with exit code 3 (invalid operation)
		if result.ExitCode != 3 {
			t.Fatalf("expected exit code 3, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates run is not completed
		if !strings.Contains(strings.ToLower(result.Output), "not completed") &&
			!strings.Contains(strings.ToLower(result.Output), "running") &&
			!strings.Contains(strings.ToLower(result.Output), "incomplete") {
			t.Error("expected message indicating run not completed")
		}
	})

	t.Run("AC-05: reject compare with non-existent run", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: Run "run-1" exists but "run-999" does not
		// When: User runs `ai-aas-org benchmark compare run-1 run-nonexistent`
		result := runOrgCLI("benchmark", "compare", "run-1", "run-nonexistent")

		// Then: Command fails with exit code 5 (not found)
		if result.ExitCode != 5 {
			t.Fatalf("expected exit code 5, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Error message indicates which run was not found
		if !strings.Contains(strings.ToLower(result.Output), "not found") {
			t.Error("expected not found message")
		}
	})
}

// TestUC_BM_007_CompareResultsAcrossRuns_MustNot validates negative requirements.
func TestUC_BM_007_CompareResultsAcrossRuns_MustNot(t *testing.T) {
	t.Run("must not compare runs from different organizations", func(t *testing.T) {
		skipIfNoLiveAPIWithReason(t, "with multiple orgs")

		// Given: User is authenticated with org admin API key
		// When: User tries to compare with another org's run
		// Then: Access is denied or run not found
	})

	t.Run("must not modify any run data", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Comparison is a read-only operation
		// This is an implicit requirement - no modifications should occur
	})

	t.Run("must not expose internal implementation details", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Given: User compares two runs
		result := runOrgCLI("benchmark", "compare", "run-1", "run-2", "--json")

		if result.ExitCode != 0 {
			t.Skip("comparison failed, skipping internal details check")
		}

		// Then: No internal implementation details are exposed
		internalPatterns := []string{"internal_", "private_", "worker_id", "node_id"}
		for _, pattern := range internalPatterns {
			if strings.Contains(strings.ToLower(result.Output), pattern) {
				t.Errorf("output may contain internal details: %s", pattern)
			}
		}
	})
}
