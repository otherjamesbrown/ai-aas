# Benchmark E2E Tests

This document describes the E2E tests for the benchmark management system.

## Prerequisites

1. **ai-aas-cli** installed and in PATH
2. CLI configured with admin credentials:
   ```bash
   # Verify configuration
   ai-aas-cli --profile dev-admin status
   ```
3. **guidellm-runner** deployed and accessible
4. At least one model deployed (e.g., `gpt-oss-20b`, `llama-3.1-8b-instruct`)
5. The `short-qa` scenario synced from ai-aas-config

## Running Tests

### Run All Benchmark Tests
```bash
cd tests/e2e
go test -v ./suites -run TestBenchmark -timeout 15m
```

### Run Specific Tests
```bash
# CLI availability check
go test -v ./suites -run TestBenchmarkCLIAvailable

# Scenario listing
go test -v ./suites -run TestBenchmarkScenarioList

# Target lifecycle (create, start, stop, remove)
go test -v ./suites -run TestBenchmarkTargetLifecycle -timeout 10m

# Config propagation (verifies request_type fix)
go test -v ./suites -run TestBenchmarkScenarioConfigPropagation -timeout 10m

# Error handling
go test -v ./suites -run TestBenchmarkErrorHandling

# Full E2E suite
go test -v ./suites -run TestBenchmarkE2EComplete -timeout 15m
```

### Skip Long-Running Tests
```bash
go test -v -short ./suites -run TestBenchmark
```

## Test Descriptions

| Test | Duration | Description |
|------|----------|-------------|
| `TestBenchmarkCLIAvailable` | ~1s | Verifies CLI is installed |
| `TestBenchmarkScenarioList` | ~2s | Lists available scenarios |
| `TestBenchmarkScenarioListJSON` | ~2s | Tests JSON output format |
| `TestBenchmarkScenarioShow` | ~2s | Shows scenario details |
| `TestBenchmarkTargetLifecycle` | ~3min | Full CRUD + start/stop cycle |
| `TestBenchmarkScenarioConfigPropagation` | ~3min | Verifies request_type is passed correctly |
| `TestBenchmarkChatVsTextCompletions` | ~6min | Tests different model types |
| `TestBenchmarkMultipleTargets` | ~1min | Tests concurrent targets |
| `TestBenchmarkErrorHandling` | ~10s | Tests error cases |
| `TestBenchmarkE2EComplete` | ~5min | Comprehensive end-to-end test |

## What's Tested

### Scenario Management
- List scenarios from synced config
- Show scenario details (profile, rate, request_type, etc.)
- JSON output format

### Target Lifecycle
- Create targets with model/scenario/environment
- List and filter targets
- Show target details
- Start benchmarking
- Stop benchmarking
- Remove targets

### Config Propagation (aas-v8cy fix)
- Scenario's `request_type` is passed to guidellm-runner
- Scenario's `rate`, `profile`, `max_seconds` are passed
- Models requiring `chat_completions` work correctly

### Error Handling
- Invalid scenario names rejected
- Duplicate target names rejected
- Operations on nonexistent targets fail appropriately

## Troubleshooting

### CLI Not Found
```bash
# Verify CLI is installed
which ai-aas-cli

# If not in PATH, add it
export PATH=$PATH:/path/to/ai-aas-cli
```

### Profile Not Configured
```bash
# Check available profiles
cat ~/.ai-aas-cli.yaml

# Verify profile works
ai-aas-cli --profile dev-admin status
```

### No Scenarios Available
```bash
# Sync scenarios from config repo
ai-aas-cli --profile dev-admin benchmark scenario sync
```

### Model Not Available
```bash
# List available models
ai-aas-cli --profile dev-admin model list --live
```

## CI Integration

These tests are tagged with `benchmark` build constraint:

```go
//go:build (benchmark || nightly || full) && e2e_tier || !e2e_tier
```

To run in CI:
```bash
go test -tags=benchmark -v ./suites -run TestBenchmark -timeout 15m
```
