package usecases_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

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
func TestUC_BM_003_ViewBenchmarkResults_New(t *testing.T) {
	t.Run("AC-01: view completed run results", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// NOTE: This test uses BenchmarkRunFixture to create a target, trigger a run,
		// and wait for completion. The fixture polls `benchmark run show` since
		// `benchmark run wait` command doesn't exist yet (see UC-BM-005).
		//
		// TODO(aas-9fp5j): Once backend supports actual benchmark execution:
		// - Verify real metrics are returned (throughput, latency, error rate)
		// - Add timeout handling for long-running benchmarks

		// Setup test org and benchmark fixture
		orgCtx := NewTestOrgContext(t)
		fm := NewFixtureManager(t, NewTestClient(getAdminAPIEndpoint(), getAdminAPIKey()))
		benchmarkFixture := NewBenchmarkRunFixture(fm, t, orgCtx)

		// Given: Create a benchmark target
		target, err := benchmarkFixture.CreateTarget("", "llama-7b", "standard")
		if err != nil {
			t.Fatalf("failed to create benchmark target: %v", err)
		}

		// When: Trigger a benchmark run
		run, err := benchmarkFixture.TriggerRun(target.Name)
		if err != nil {
			t.Fatalf("failed to trigger benchmark run: %v", err)
		}

		// Wait for completion (with timeout)
		ctx := context.Background()
		completedRun, err := benchmarkFixture.WaitForCompletion(ctx, run.ID, 5*time.Minute)
		if err != nil {
			t.Fatalf("failed to wait for completion: %v", err)
		}

		// Then: Run status is displayed as "completed"
		if !strings.Contains(strings.ToLower(completedRun.Status), "complet") {
			t.Errorf("expected status to contain 'completed', got: %s", completedRun.Status)
		}

		// Then: Results are available
		if completedRun.Results == nil {
			t.Error("expected results to be available")
		}

		// TODO: Once real benchmarks are implemented, verify:
		// - Throughput metric is shown (requests/second)
		// - Latency percentiles are shown (p50, p90, p99)
		// - Error rate is shown (percentage)
		// - Duration is shown
	})

	t.Run("AC-02: view pending run status", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Setup test org and benchmark fixture
		orgCtx := NewTestOrgContext(t)
		fm := NewFixtureManager(t, NewTestClient(getAdminAPIEndpoint(), getAdminAPIKey()))
		benchmarkFixture := NewBenchmarkRunFixture(fm, t, orgCtx)

		// Given: Create a benchmark target
		target, err := benchmarkFixture.CreateTarget("", "llama-7b", "standard")
		if err != nil {
			t.Fatalf("failed to create benchmark target: %v", err)
		}

		// When: Trigger a benchmark run
		run, err := benchmarkFixture.TriggerRun(target.Name)
		if err != nil {
			t.Fatalf("failed to trigger benchmark run: %v", err)
		}

		// Then: Immediately check status (should be pending or running)
		status, err := benchmarkFixture.GetRunStatus(run.ID)
		if err != nil {
			t.Fatalf("failed to get run status: %v", err)
		}

		// Then: Run status is displayed as "running" or "pending"
		statusLower := strings.ToLower(status.Status)
		if !strings.Contains(statusLower, "pending") && !strings.Contains(statusLower, "running") {
			t.Logf("Note: Expected 'pending' or 'running', got '%s' (benchmark may complete quickly)", status.Status)
		}

		// Then: Start time is displayed (check Results map has timestamp-like fields)
		if status.Results != nil {
			// Look for common timestamp field names
			hasTimestamp := false
			for key := range status.Results {
				keyLower := strings.ToLower(key)
				if strings.Contains(keyLower, "created") || strings.Contains(keyLower, "start") || strings.Contains(keyLower, "time") {
					hasTimestamp = true
					break
				}
			}
			if !hasTimestamp {
				t.Log("Note: No timestamp field found in results")
			}
		}
	})

	t.Run("AC-03: view failed run details", func(t *testing.T) {
		t.Skip("TODO(aas-9fp5j): Requires ability to create a run that fails - needs invalid target or model")

		// Given: Benchmark run failed during execution
		// When: User runs `ai-aas-org benchmark run show <run-id>`
		// Then: Run status is displayed as "failed"
		// Then: Error message explains failure reason
		// Then: Partial results shown if available
		// Then: Suggestion for troubleshooting included
	})

	t.Run("AC-04: export results as JSON", func(t *testing.T) {
		skipIfNoLiveAPI(t)

		// Setup test org and benchmark fixture
		orgCtx := NewTestOrgContext(t)
		fm := NewFixtureManager(t, NewTestClient(getAdminAPIEndpoint(), getAdminAPIKey()))
		benchmarkFixture := NewBenchmarkRunFixture(fm, t, orgCtx)

		// Given: Create a benchmark target and trigger run
		target, err := benchmarkFixture.CreateTarget("", "llama-7b", "standard")
		if err != nil {
			t.Fatalf("failed to create benchmark target: %v", err)
		}

		run, err := benchmarkFixture.TriggerRun(target.Name)
		if err != nil {
			t.Fatalf("failed to trigger benchmark run: %v", err)
		}

		// When: User runs `ai-aas-org benchmark run show <run-id> --json`
		result := orgCtx.RunCLI("benchmark", "run", "show", run.ID, "--json")

		// Then: Exit code is 0
		if result.ExitCode != 0 {
			t.Fatalf("expected exit code 0, got %d: %s", result.ExitCode, result.Output)
		}

		// Then: Results are output as valid JSON
		if !isValidJSON(result.Output) {
			t.Errorf("expected valid JSON output, got: %s", result.Output)
		}

		// Then: JSON includes all metrics (at minimum, status and ID)
		var runData map[string]interface{}
		if err := json.Unmarshal([]byte(result.Output), &runData); err != nil {
			t.Fatalf("failed to parse JSON: %v", err)
		}

		// Verify essential fields are present
		if _, ok := runData["status"]; !ok {
			t.Error("JSON output missing 'status' field")
		}
		if id, ok := runData["id"].(string); !ok || id == "" {
			if runId, ok := runData["runId"].(string); !ok || runId == "" {
				t.Error("JSON output missing 'id' or 'runId' field")
			}
		}
	})
}
