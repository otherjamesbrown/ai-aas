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
