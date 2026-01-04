# Benchmark Test Harness

This document describes the `BenchmarkRunFixture` test harness for UC-BM tests.

## Overview

The `BenchmarkRunFixture` provides helpers to create benchmark targets, trigger runs, and wait for completion in UC acceptance tests. It follows the existing fixture pattern used by other test fixtures (`OrganizationFixture`, `ServiceAccountFixture`, etc.).

## Usage

### Basic Example

```go
func TestMyBenchmark(t *testing.T) {
    skipIfNoLiveAPI(t)

    // Setup test org and benchmark fixture
    orgCtx := NewTestOrgContext(t)
    fm := NewFixtureManager(t, NewTestClient(getAdminAPIEndpoint(), getAdminAPIKey()))
    benchmarkFixture := NewBenchmarkRunFixture(fm, t, orgCtx)

    // Create a benchmark target
    target, err := benchmarkFixture.CreateTarget("my-target", "llama-7b", "standard")
    if err != nil {
        t.Fatalf("failed to create benchmark target: %v", err)
    }
    // Target is automatically cleaned up via FixtureManager

    // Trigger a benchmark run
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

    // Verify results
    if completedRun.Status != "completed" {
        t.Errorf("expected status 'completed', got: %s", completedRun.Status)
    }
}
```

### API Reference

#### `NewBenchmarkRunFixture(fm *FixtureManager, t *testing.T, orgCtx *TestOrgContext) *BenchmarkRunFixture`

Creates a new benchmark run fixture manager.

- `fm`: FixtureManager for resource cleanup
- `t`: Test instance
- `orgCtx`: TestOrgContext with org credentials

#### `CreateTarget(targetName, model, scenario string) (*BenchmarkTarget, error)`

Creates a benchmark target and registers it for cleanup.

- `targetName`: Unique name for the target (auto-generated if empty)
- `model`: Model name to benchmark (default: "llama-7b")
- `scenario`: Scenario type (default: "standard")

Returns: `*BenchmarkTarget` with Name, Model, Scenario fields

#### `TriggerRun(targetName string) (*BenchmarkRun, error)`

Triggers a benchmark run on an existing target.

- `targetName`: Name of the target to run

Returns: `*BenchmarkRun` with ID, TargetID, Status fields

#### `WaitForCompletion(ctx context.Context, runID string, timeout time.Duration) (*BenchmarkRun, error)`

Waits for a benchmark run to complete or fail.

**NOTE**: This uses polling with `benchmark run show` since `benchmark run wait` command doesn't exist yet (see UC-BM-005).

- `ctx`: Context for cancellation
- `runID`: Run ID to wait for
- `timeout`: Maximum time to wait (e.g., 5*time.Minute)

Returns: `*BenchmarkRun` with final Status and Results

Errors:
- Timeout if run doesn't complete within timeout
- Error if run fails or is cancelled

#### `GetRunStatus(runID string) (*BenchmarkRun, error)`

Retrieves the current status of a benchmark run without waiting.

- `runID`: Run ID to check

Returns: `*BenchmarkRun` with current Status and Results

#### `DeleteTarget(targetName string) error`

Deletes a benchmark target. This is called automatically by FixtureManager cleanup.

- `targetName`: Name of the target to delete

## Implementation Notes

### Polling vs. Wait Command

The `WaitForCompletion` method uses polling with 5-second intervals because the `ai-aas-org benchmark run wait` command doesn't exist yet (see UC-BM-005 in usecases/benchmarks.yaml).

When the `wait` command is implemented, we can update the fixture to use it directly, which will be more efficient.

### Cleanup

All targets created via `CreateTarget()` are automatically cleaned up at test end via `FixtureManager`. Runs are tracked but may not need explicit cleanup (depends on backend implementation).

### Timeout Handling

The default timeout in tests is 5 minutes. Adjust based on benchmark scenario:

- Simple/standard scenarios: 2-5 minutes
- Throughput scenarios: 5-10 minutes
- Long-running scenarios: Consider skipping in CI

### Status Mapping

The fixture recognizes these terminal statuses:

- **Completed**: "completed", "success", "succeeded"
- **Failed**: "failed", "error"
- **Cancelled**: "cancelled", "canceled"

Non-terminal statuses (pending, running, queued) cause continued polling.

## Examples

### UC-BM-003 AC-01: View Completed Run Results

See `tests/usecases/benchmarks_uc_bm_003_test.go` for the full implementation of UC-BM-003 using the harness.

### UC-BM-003 AC-02: View Pending Run Status

```go
// Trigger run
run, err := benchmarkFixture.TriggerRun(targetName)
require.NoError(t, err)

// Immediately check status (should be pending/running)
status, err := benchmarkFixture.GetRunStatus(run.ID)
require.NoError(t, err)

assert.Contains(t, []string{"pending", "running"}, strings.ToLower(status.Status))
```

### UC-BM-003 AC-04: Export Results as JSON

```go
// Trigger run
run, err := benchmarkFixture.TriggerRun(targetName)
require.NoError(t, err)

// Get JSON output directly via CLI
result := orgCtx.RunCLI("benchmark", "run", "show", run.ID, "--json")
assert.Equal(t, 0, result.ExitCode)
assert.True(t, isValidJSON(result.Output))
```

## Future Enhancements

When UC-BM-005 (Wait for Run Completion) and UC-BM-006 (Cancel Running Benchmarks) are implemented:

1. Add `CancelRun(runID string) error` method
2. Update `WaitForCompletion` to use native wait command
3. Add `WaitWithProgress(runID string, callback func(status string))` for progress updates

## Related Files

- `tests/usecases/fixtures_test.go` - Fixture implementations
- `tests/usecases/benchmarks_test.go` - UC-BM test suite
- `tests/usecases/benchmarks_uc_bm_003_test.go` - UC-BM-003 with harness
- `usecases/benchmarks.yaml` - Use case specifications
