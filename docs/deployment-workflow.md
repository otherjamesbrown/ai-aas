# vLLM Deployment Workflow

## Overview

This document describes the workflow for deploying vLLM model inference engines on GPU nodes with health checks, readiness verification, and completion endpoint testing.

## Prerequisites

- Kubernetes cluster with GPU node pool
- Helm 3.x installed
- kubectl configured with cluster access
- Access to GPU nodes (node-type: gpu label)

## Deployment Steps

### 1. Prepare Helm Chart Values

Create or use environment-specific values file:

```bash
# Development
cp infra/helm/charts/vllm-deployment/values-development.yaml my-model-dev.yaml

# Edit values as needed
vim my-model-dev.yaml
```

Key values to configure:

```yaml
model:
  path: "meta-llama/Llama-2-7b-chat-hf"  # Model identifier
  size: "small"  # small, medium, or large

environment: development
namespace: system

resources:
  limits:
    nvidia.com/gpu: 1
    memory: "32Gi"
    cpu: "8"
```

### 2. Deploy with Retry Logic

Use the deployment script with retry logic to handle GPU availability:

```bash
./scripts/vllm/deploy-with-retry.sh \
  llama-7b-development \
  infra/helm/charts/vllm-deployment \
  my-model-dev.yaml \
  system \
  30
```

**Script Behavior:**
- Checks GPU node availability
- Waits with exponential backoff if GPUs unavailable
- Deploys using Helm
- Times out after specified minutes (default: 30)

### 3. Wait for Deployment

Monitor deployment progress:

```bash
# Watch pod status
kubectl get pods -n system -l app.kubernetes.io/instance=llama-7b-development -w

# Or wait for ready condition
kubectl wait --for=condition=ready \
  pod -l app.kubernetes.io/instance=llama-7b-development \
  -n system \
  --timeout=600s
```

### 4. Verify Deployment

Run the verification script to check all endpoints:

```bash
./scripts/vllm/verify-deployment.sh \
  llama-7b-development \
  system \
  600
```

**Verification Checks:**
- ✅ Pod status (Running)
- ✅ Pod ready condition
- ✅ `/health` endpoint (HTTP 200)
- ✅ `/ready` endpoint (HTTP 200)
- ✅ `/v1/chat/completions` endpoint (HTTP 200, ≤3s response)

### 5. Manual Verification (Optional)

If you need to verify manually:

```bash
# Check pod status
kubectl get pods -n system -l app.kubernetes.io/instance=llama-7b-development

# Port forward to service
kubectl port-forward -n system svc/llama-7b-development 8000:8000

# Test health endpoint
curl http://localhost:8000/health

# Test ready endpoint
curl http://localhost:8000/ready

# Test completion endpoint
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 10
  }'
```

## Deployment Verification Checklist

- [ ] Pod is in `Running` state
- [ ] Pod ready condition is `True`
- [ ] `/health` endpoint returns HTTP 200
- [ ] `/ready` endpoint returns HTTP 200
- [ ] `/v1/chat/completions` endpoint returns HTTP 200
- [ ] Completion response time ≤ 3 seconds (95th percentile)
- [ ] Service endpoint is accessible: `{release-name}.{namespace}.svc.cluster.local:8000`

## Troubleshooting

### Pod Not Starting

**Symptoms:**
- Pod status: `Pending` or `CrashLoopBackOff`
- Pod events show scheduling failures

**Solutions:**
1. Check GPU node availability:
   ```bash
   kubectl get nodes -l node-type=gpu
   kubectl describe nodes -l node-type=gpu | grep -A 10 "nvidia.com/gpu"
   ```

2. Check resource requests:
   ```bash
   kubectl describe pod <pod-name> -n <namespace> | grep -A 10 "Requests:"
   ```

3. Review pod events:
   ```bash
   kubectl describe pod <pod-name> -n <namespace> | grep -A 20 "Events:"
   ```

### Model Loading Timeout

**Symptoms:**
- Pod status: `Running` but not ready
- Startup probe failing
- Logs show model loading in progress

**Solutions:**
1. Check model initialization timeout (see [Model Initialization](./model-initialization.md))
2. Increase `failureThreshold` in values file for large models
3. Verify model path is correct
4. Check GPU memory is sufficient

### Health Endpoint Failing

**Symptoms:**
- `/health` returns non-200 status
- Pod is running but health checks fail

**Solutions:**
1. Check pod logs:
   ```bash
   kubectl logs <pod-name> -n <namespace>
   ```

2. Verify vLLM is running:
   ```bash
   kubectl exec <pod-name> -n <namespace> -- ps aux | grep vllm
   ```

3. Check resource limits (may be OOM killed):
   ```bash
   kubectl describe pod <pod-name> -n <namespace> | grep -A 5 "Limits:"
   ```

## Environment-Specific Notes

### Development

- Faster iteration, shorter timeouts
- May use smaller models for testing
- Auto-sync enabled in ArgoCD

### Staging

- Mirrors production configuration
- Full validation before production
- Manual sync in ArgoCD

### Production

- Longer timeouts for large models
- Strict resource limits
- Manual approval required in ArgoCD

## Next Steps

After successful deployment:

1. **Register Model** (User Story 2):
   - Register model in `model_registry_entries` table
   - Enable routing via API Router Service

2. **Monitor Deployment**:
   - Set up Grafana dashboards
   - Configure Prometheus alerts
   - Monitor initialization times

3. **Document Configuration**:
   - Record model path and version
   - Document resource requirements
   - Note any custom configurations

## Related Documentation

- [Model Initialization Timeout Strategy](./model-initialization.md)
- [Helm Chart Values](../infra/helm/charts/vllm-deployment/values.yaml)
- [Troubleshooting Guide](./troubleshooting.md)

