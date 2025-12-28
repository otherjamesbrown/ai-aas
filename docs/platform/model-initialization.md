# Model Initialization Timeout Strategy

## Overview

vLLM model deployments require time to load models into GPU memory, especially for large models. This document describes the timeout strategy for model initialization to ensure reliable deployments while handling various model sizes.

## Timeout Calculation

The startup probe timeout is calculated as:
```
Total Timeout = failureThreshold × periodSeconds
```

### Model Size Categories

| Model Size | Parameters | failureThreshold | periodSeconds | Total Timeout |
|------------|-----------|-------------------|---------------|---------------|
| Small      | 7B-13B    | 20                | 30s           | 10 minutes    |
| Medium     | 30B-40B   | 30                | 30s           | 15 minutes    |
| Large      | 70B+      | 40                | 30s           | 20 minutes    |

### Configuration per Environment

The timeout configuration is set in environment-specific values files:

- **Development** (`values-development.yaml`): `failureThreshold: 20` (10 minutes)
- **Staging** (`values-staging.yaml`): `failureThreshold: 30` (15 minutes)
- **Production** (`values-production.yaml`): `failureThreshold: 40` (20 minutes)

## Startup Probe Configuration

The startup probe is configured in the Helm chart deployment template:

```yaml
startupProbe:
  httpGet:
    path: /ready
    port: http
  initialDelaySeconds: 0
  periodSeconds: 30
  timeoutSeconds: 10
  failureThreshold: 20  # Adjust based on model.size
```

### Probe Behavior

1. **Initial Delay**: 0 seconds (starts immediately)
2. **Period**: 30 seconds between checks
3. **Timeout**: 10 seconds per check
4. **Failure Threshold**: Number of consecutive failures before marking as failed

## Fallback Behavior on Timeout

If the startup probe times out:

1. **Kubernetes Behavior**: Pod is marked as failed and may be restarted (depending on restart policy)
2. **Deployment Status**: Helm deployment shows as failed
3. **Remediation Steps**:
   - Check pod logs: `kubectl logs <pod-name> -n <namespace>`
   - Verify GPU availability: `kubectl describe nodes -l node-type=gpu`
   - Check model path and accessibility
   - Review resource requests/limits
   - Consider increasing `failureThreshold` for very large models

## Monitoring and Alerting

### Metrics to Monitor

- **Initialization Duration**: Time from pod start to ready state
- **Initialization Failures**: Count of pods that fail to initialize
- **GPU Memory Usage**: Monitor GPU memory during model loading
- **Model Load Errors**: Errors during model loading phase

### Alerting Thresholds

- **Warning**: Initialization takes > 80% of timeout threshold
- **Critical**: Initialization fails (timeout exceeded)
- **Info**: Initialization completes successfully

### Grafana Dashboard

Create a dashboard panel tracking:
- Model initialization duration histogram
- Initialization success/failure rate
- Average initialization time by model size

## Best Practices

### 1. Model Size Selection

- Use the appropriate `failureThreshold` based on actual model size
- For custom models, estimate based on parameter count
- Test initialization time in development before production

### 2. Resource Allocation

- Ensure GPU memory is sufficient for model + KV cache
- Allocate CPU resources for model loading (8-16 cores recommended)
- Monitor resource usage during initialization

### 3. Pre-warming Strategy

- For production, consider pre-warming models during low-traffic periods
- Use readiness probe to gate traffic until model is fully loaded
- Implement health checks to verify model is ready for inference

### 4. Troubleshooting

**Common Issues:**

1. **Timeout Too Short**: Increase `failureThreshold` in values file
2. **GPU Memory Insufficient**: Reduce model size or increase GPU memory
3. **Model Path Incorrect**: Verify model path in Helm values
4. **Network Issues**: Check model download/access if using remote models

**Debug Commands:**

```bash
# Check pod status
kubectl get pods -n <namespace> -l app.kubernetes.io/instance=<release-name>

# View pod logs
kubectl logs <pod-name> -n <namespace>

# Check startup probe status
kubectl describe pod <pod-name> -n <namespace> | grep -A 5 "Startup"

# Check GPU resources
kubectl describe nodes -l node-type=gpu | grep -A 10 "nvidia.com/gpu"
```

## Configuration Examples

### Small Model (7B-13B)

```yaml
model:
  path: "meta-llama/Llama-2-7b-chat-hf"
  size: "small"

startupProbe:
  failureThreshold: 20  # 10 minutes
```

### Medium Model (30B-40B)

```yaml
model:
  path: "meta-llama/Llama-2-30b-chat-hf"
  size: "medium"

startupProbe:
  failureThreshold: 30  # 15 minutes
```

### Large Model (70B+)

```yaml
model:
  path: "meta-llama/Llama-2-70b-chat-hf"
  size: "large"

startupProbe:
  failureThreshold: 40  # 20 minutes
```

## Related Documentation

- [Deployment Workflow](./deployment-workflow.md)
- [Troubleshooting Guide](./troubleshooting.md)
- [Helm Chart Values](../infra/helm/charts/vllm-deployment/values.yaml)

