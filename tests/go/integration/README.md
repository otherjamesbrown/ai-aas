# Integration Tests

This directory contains integration tests that validate cross-component behavior against a live Kubernetes cluster.

## Test Categories

### CLI Model Status Tests (`cli_model_status_test.go`)

Tests the `ai-aas-cli model status` command against various failure mode scenarios.

**References:**
- Use Case: UC-OPS-001 - Model Status Observability
- Bead: aas-5f2xh

**Test Coverage:**
- Scheduling failures (insufficient CPU, memory, GPU)
- Pod scheduling blocks (missing tolerations, node taints)
- Autoscaler behavior (scaling up, at max capacity)
- Download failures (invalid S3 path, invalid HuggingFace model)
- Runtime failures (OOMKilled, CrashLoopBackOff)
- Configuration errors (invalid recipe)

## Prerequisites

### 1. Kubernetes Cluster Access

Tests require access to a development Kubernetes cluster with the AI Model Operator installed.

```bash
# Verify cluster access
kubectl get nodes

# Verify operator is running
kubectl get pods -n ai-system | grep ai-model-operator
```

### 2. CLI Binary

Build the CLI before running tests:

```bash
# From repo root
./scripts/build-clis.sh

# Or build and install
./scripts/build-clis.sh --install
```

### 3. Test Configs

Failure mode configs must exist at `tests/failure-modes/configs/`:

```bash
ls tests/failure-modes/configs/
# Should show: test-insufficient-cpu.yaml, test-invalid-s3-path.yaml, etc.
```

## Running Tests

### Run All Integration Tests

```bash
cd tests/go/integration
go test -v -tags=integration ./... -timeout 30m
```

### Run Specific Test Suite

```bash
# CLI status tests only
go test -v -tags=integration ./... -run TestCLIModelStatus -timeout 30m
```

### Run Single Test Case

```bash
# Test a specific failure mode
go test -v -tags=integration ./... -run "TestCLIModelStatusFailureModes/insufficient_CPU" -timeout 5m
```

### Skip Integration Tests

Integration tests are excluded by default when the `integration` build tag is not specified:

```bash
# This will NOT run integration tests
go test -v ./...

# This WILL run integration tests
go test -v -tags=integration ./...
```

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `KUBECONFIG` | Path to kubeconfig file | `~/.kube/config` |
| `CLI_PATH` | Path to ai-aas-cli binary | `~/.local/bin/ai-aas-cli` or `services/ai-aas-cli/ai-aas-cli` |

## Test Execution Flow

For each failure mode test:

1. **Apply** the test config with `enabled=false`
2. **Enable** the model by patching `spec.enabled=true`
3. **Wait** for expected phase (Scheduling, Failed, etc.)
4. **Run** CLI status command and capture output
5. **Verify** phase and expected error patterns in output
6. **Cleanup** delete the AIModel resource

## Expected Test Duration

| Test Category | Typical Duration |
|---------------|------------------|
| Scheduling failures | 1-2 minutes each |
| Download failures | 2-3 minutes each |
| Runtime failures | 3-4 minutes each |
| Full suite | 20-30 minutes |

## Troubleshooting

### Test Timeout

If tests timeout, increase timeout in test code or via flag:

```bash
go test -v -tags=integration ./... -timeout 60m
```

### Cluster Access Issues

```bash
# Verify KUBECONFIG
echo $KUBECONFIG

# Test cluster access
kubectl get aimodels -n development

# Check operator logs
kubectl logs -n ai-system deployment/ai-model-operator-controller-manager -f
```

### CLI Not Found

```bash
# Build CLI
cd services/ai-aas-cli
go build -o ai-aas-cli

# Or use environment variable
export CLI_PATH=/path/to/ai-aas-cli
```

### Models Not Reaching Expected Phase

Check operator logs for the reason:

```bash
# View operator logs
kubectl logs -n ai-system deployment/ai-model-operator-controller-manager -f

# Check AIModel status
kubectl get aimodel test-insufficient-cpu -n development -o yaml
```

## Adding New Tests

### 1. Create Test Config

Add a new config file to `tests/failure-modes/configs/`:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: test-new-failure
  namespace: development
  labels:
    test-type: failure-mode
    failure-category: <category>
spec:
  # ... config that triggers the failure
  enabled: false  # Important!
```

### 2. Add Test Case

Add to the `tests` slice in `TestCLIModelStatusFailureModes`:

```go
{
    name:            "UC-OPS-001/AC-XX: description",
    configFile:      "test-new-failure.yaml",
    expectedPhase:   "Failed",
    expectedPattern: `(?i)(error|pattern|to|match)`,
    timeout:         3 * time.Minute,
},
```

### 3. Run Test

```bash
go test -v -tags=integration ./... -run "TestCLIModelStatusFailureModes/description" -timeout 5m
```

## CI Integration

To run in CI:

```bash
# Example GitHub Actions workflow
- name: Run integration tests
  run: |
    export KUBECONFIG=${{ secrets.DEV_KUBECONFIG }}
    ./scripts/build-clis.sh
    cd tests/go/integration
    go test -v -tags=integration ./... -timeout 30m
```

## Related Documentation

- [E2E Testing Context](../../../context/e2e-testing/agents.md)
- [Test Developer Context](../../../context/test-developer/agents.md)
- [CLI Model Status Command](../../../services/ai-aas-cli/cmd/model/status.go)
- [Use Case UC-OPS-001](../../../usecases/UC-OPS-001-model-status-observability.yaml)
