# vLLM Deployment Integration Tests

This directory contains integration tests for vLLM deployments via Helm chart,
implementing the test requirements from `specs/010-vllm-deployment/`.

## Overview

These tests validate that the vLLM Helm chart correctly deploys inference engines
on GPU nodes with proper health checks, readiness probes, and functional completion endpoints.

## Test Files

### 1. `deployment_readiness_test.go`

**Task**: T-S010-P03-016 - Integration test for deployment readiness

**What it tests**:
- Helm chart structure is valid
- All Kubernetes resources are created (Deployment, Service, etc.)
- Pod reaches Running and Ready state
- Health endpoints (/health, /v1/models) respond correctly
- Service is properly configured with ClusterIP

**Prerequisites**:
- Existing vLLM deployment in cluster
- KUBECONFIG set to cluster with GPU nodes
- RUN_VLLM_TESTS=1

**Usage**:
```bash
export KUBECONFIG=/path/to/kubeconfig
export RUN_VLLM_TESTS=1
go test -v ./tests/infra/vllm -run TestVLLMDeploymentReadiness
```

### 2. `completion_endpoint_test.go`

**Task**: T-S010-P03-017 - Integration test for completion endpoint

**What it tests**:
- vLLM /v1/chat/completions endpoint is accessible
- Model generates valid responses
- Response format matches OpenAI specification
- Token counting is accurate (prompt_tokens, completion_tokens, total_tokens)
- Response latency is reasonable (≤30s with warmup, faster for subsequent requests)
- Multiple sequential requests work correctly

**Prerequisites**:
- vLLM deployment running and healthy
- VLLM_BACKEND_URL set (typically via kubectl port-forward)
- VLLM_MODEL_NAME set (optional, defaults to gpt-oss-20b)
- RUN_VLLM_TESTS=1

**Usage**:
```bash
# Start port-forward in another terminal:
kubectl port-forward -n system svc/gpt-oss-20b-vllm-deployment 8000:8000

# Run test:
export KUBECONFIG=/path/to/kubeconfig
export VLLM_BACKEND_URL=http://localhost:8000
export VLLM_MODEL_NAME=gpt-oss-20b
export RUN_VLLM_TESTS=1
go test -v ./tests/infra/vllm -run TestVLLMCompletionEndpoint
```

### 3. `deployment_e2e_test.go`

**Task**: T-S010-P03-018 - E2E test for deployment flow

**What it tests**:
- Complete end-to-end deployment lifecycle
- Deployment → Ready → Health checks → Completion
- Full validation of User Story 1: Provision reliable inference endpoints

**Prerequisites**:
- KUBECONFIG set to cluster with GPU nodes
- Existing vLLM deployment (test validates existing deployment)
- Optional: VLLM_BACKEND_URL for endpoint testing
- RUN_VLLM_E2E_TESTS=1

**Usage**:
```bash
# Deploy vLLM first:
cd infra/helm/charts/vllm-deployment
helm install gpt-oss-20b . -f values-unsloth-gpt-oss-20b.yaml \
  --namespace system \
  --set prometheus.serviceMonitor.enabled=false \
  --set preInstallChecks.enabled=false

# Wait for deployment to be ready (can take 10-15 minutes)

# Run E2E test:
export KUBECONFIG=/path/to/kubeconfig
export RUN_VLLM_E2E_TESTS=1

# Optional: For endpoint testing, port-forward in another terminal
kubectl port-forward -n system svc/gpt-oss-20b-vllm-deployment 8000:8000
export VLLM_BACKEND_URL=http://localhost:8000

go test -v ./tests/infra/vllm -run TestVLLMDeploymentE2E
```

## Test Environment Setup

### Required Environment Variables

| Variable | Required | Description | Example |
|----------|----------|-------------|---------|
| `KUBECONFIG` | Yes | Path to kubeconfig file | `/home/dev/kubeconfigs/kubeconfig-development.yaml` |
| `RUN_VLLM_TESTS` | Yes | Enable vLLM tests | `1` |
| `VLLM_BACKEND_URL` | Optional* | vLLM service URL for endpoint tests | `http://localhost:8000` |
| `VLLM_MODEL_NAME` | Optional | Model name to test | `gpt-oss-20b` (default) |
| `RUN_VLLM_E2E_TESTS` | For E2E only | Enable E2E tests | `1` |

*Required for completion endpoint tests and E2E endpoint validation

### Cluster Requirements

- Kubernetes cluster with GPU node pool
- GPU nodes with NVIDIA runtime configured
- At least 1 GPU available (NVIDIA RTX 4000 Ada or similar)
- Sufficient resources: 16Gi+ RAM, 8+ CPUs per GPU node
- Helm 3.x installed
- kubectl access to cluster

### Pre-deployment Setup

Before running tests, deploy vLLM using the Helm chart:

```bash
cd infra/helm/charts/vllm-deployment

helm install gpt-oss-20b . \
  -f values-unsloth-gpt-oss-20b.yaml \
  --namespace system \
  --set prometheus.serviceMonitor.enabled=false \
  --set preInstallChecks.enabled=false

# Wait for pod to be ready (10-15 minutes for model download + loading)
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/instance=gpt-oss-20b \
  -n system \
  --timeout=20m
```

## Running Tests

### Run All Tests

```bash
export KUBECONFIG=/path/to/kubeconfig
export RUN_VLLM_TESTS=1
export RUN_VLLM_E2E_TESTS=1
export VLLM_BACKEND_URL=http://localhost:8000
export VLLM_MODEL_NAME=gpt-oss-20b

go test -v ./tests/infra/vllm
```

### Run Individual Test

```bash
# Deployment readiness only
go test -v ./tests/infra/vllm -run TestVLLMDeploymentReadiness

# Completion endpoint only
go test -v ./tests/infra/vllm -run TestVLLMCompletionEndpoint

# E2E test only
go test -v ./tests/infra/vllm -run TestVLLMDeploymentE2E
```

### Run With Port-Forward

In one terminal:
```bash
kubectl port-forward -n system svc/gpt-oss-20b-vllm-deployment 8000:8000
```

In another terminal:
```bash
export KUBECONFIG=/path/to/kubeconfig
export VLLM_BACKEND_URL=http://localhost:8000
export RUN_VLLM_TESTS=1
go test -v ./tests/infra/vllm -run TestVLLMCompletionEndpoint
```

## Test Success Criteria

### Deployment Readiness Test

- ✅ Helm chart structure is valid
- ✅ Deployment exists and has desired replicas ready
- ✅ Pod is in Running phase
- ✅ Pod passes readiness probe (Ready condition = True)
- ✅ Service exists with ClusterIP assigned
- ✅ Service is accessible (validated via port-forward documentation)

### Completion Endpoint Test

- ✅ /health endpoint returns 200 OK
- ✅ /v1/models endpoint returns 200 OK with model list
- ✅ /v1/chat/completions accepts valid requests
- ✅ Response format matches OpenAI specification
- ✅ Response has valid ID, object type, choices
- ✅ Token counting is accurate (prompt, completion, total)
- ✅ Response latency is reasonable (≤30s first request, faster after warmup)
- ✅ Response content is non-empty and meaningful

### E2E Test

- ✅ Deployment exists and is accessible
- ✅ All resources reach Ready state
- ✅ Health endpoints respond correctly
- ✅ Completion endpoint generates valid responses
- ✅ Full deployment lifecycle works end-to-end

## Troubleshooting

### Test Skips with "Deployment not found"

**Cause**: vLLM deployment doesn't exist in cluster

**Solution**: Deploy vLLM first using Helm:
```bash
cd infra/helm/charts/vllm-deployment
helm install gpt-oss-20b . -f values-unsloth-gpt-oss-20b.yaml --namespace system
```

### Test Skips with "VLLM_BACKEND_URL not set"

**Cause**: Completion endpoint tests need direct access to vLLM service

**Solution**: Set up port-forward and environment variable:
```bash
kubectl port-forward -n system svc/gpt-oss-20b-vllm-deployment 8000:8000 &
export VLLM_BACKEND_URL=http://localhost:8000
```

### Pod Not Ready Within Timeout

**Cause**: Model download and loading takes time (10-15 minutes first time)

**Solutions**:
1. Check pod logs: `kubectl logs -n system <pod-name> -f`
2. Verify GPU is available: `kubectl describe pod -n system <pod-name>`
3. Check node GPU status: `kubectl describe node <gpu-node-name>`
4. Ensure sufficient resources (RAM, GPU memory)
5. Wait longer - first-time model download is ~40GB

### Connection Refused to Backend

**Cause**: Port-forward not running or wrong port

**Solutions**:
1. Start port-forward: `kubectl port-forward -n system svc/<service-name> 8000:8000`
2. Verify service exists: `kubectl get svc -n system`
3. Check VLLM_BACKEND_URL matches port-forward

### Test Failures Related to Token Counting

**Cause**: vLLM version or model doesn't support token counting

**Solution**: Check vLLM logs for errors, verify model compatibility

## Integration with CI/CD

These tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions workflow
- name: Run vLLM Integration Tests
  env:
    KUBECONFIG: ${{ secrets.KUBECONFIG }}
    RUN_VLLM_TESTS: "1"
  run: |
    go test -v ./tests/infra/vllm -run TestVLLMDeploymentReadiness
```

For completion and E2E tests in CI, you'll need:
1. Access to a cluster with GPU nodes
2. Pre-deployed vLLM instance (or deploy as part of test setup)
3. Port-forwarding or cluster-internal access
4. Sufficient timeout for model loading

## Next Steps

After these tests pass:

1. ✅ User Story 1 (Provision reliable inference endpoints) is validated
2. → Implement User Story 2: Model Registration (T-S010-P04-029 through T-S010-P04-041)
3. → Implement User Story 3: Safe Operations (T-S010-P05-042 through T-S010-P05-057)
4. → Add observability and monitoring (Phase 6)

## References

- Spec: `/specs/010-vllm-deployment/`
- Tasks: `/specs/010-vllm-deployment/tasks.md`
- Helm Chart: `/infra/helm/charts/vllm-deployment/`
- Progress: `/specs/010-vllm-deployment/PROGRESS.md`
