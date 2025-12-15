# Runbook: Enable Model Readiness Probes for KServe InferenceServices

**Feature**: `018-model-readiness-probes`
**Last Updated**: 2025-11-28
**Spec**: `specs/018-model-readiness-probes/spec.md`

## Overview

This runbook documents how to configure KServe InferenceService deployments to allow sufficient time for vLLM models to load into GPU memory before receiving traffic.

## Problem Statement

Without proper configuration:
1. Knative's default `progress-deadline` is ~2 minutes
2. vLLM model loading takes 2-15 minutes depending on model size
3. Knative kills pods that don't become ready within the deadline
4. Pod restart loops or `BACKEND_ERROR` responses occur

## Key Finding: Knative Progress Deadline

**CRITICAL**: Standard Kubernetes `startupProbe` configurations are **NOT honored** by KServe/Knative. Knative manages probing through its queue-proxy sidecar and ignores container-level startup probes.

**Solution**: Use the `serving.knative.dev/progress-deadline` annotation to allow sufficient startup time:

```yaml
annotations:
  # Allow 15 minutes for 20B model to load
  serving.knative.dev/progress-deadline: "900s"
```

## Configuration by Model Size

| Model Size | Examples | `progress-deadline` | Loading Time |
|:-----------|:---------|:--------------------|:-------------|
| **7B** | mistral-7b, llama-2-7b | `360s` (6 min) | 2-5 min |
| **13B** | llama-2-13b, codellama-13b | `600s` (10 min) | 5-8 min |
| **20B** | gpt-oss-20b | `900s` (15 min) | 8-12 min |
| **70B+** | llama-2-70b, mixtral | `1800s` (30 min) | 15-25 min |

**Note**: Set `progress-deadline` to ~1.5x the expected loading time to allow buffer.

---

## Step 1: Determine Your Model's Port

| vLLM Version | Port |
|:-------------|:-----|
| v0.6.x+ | 8000 |
| v0.3.x | 8080 |

Check your InferenceService image tag to determine the version.

---

## Step 2: Add Progress Deadline to AIModel CR

The platform uses AIModel CRs (managed by the AI Model Operator) to deploy models. Health probes and progress deadlines are automatically configured by the operator based on best practices for each runtime.

### Standard Configuration (7B Models)

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: mistral-7b-instruct
  namespace: development
spec:
  modelName: mistral-7b-instruct
  modelID: mistralai/Mistral-7B-Instruct-v0.3
  enabled: true
  runtime: vllm

  # Keep 1 replica warm to avoid cold starts
  minReplicas: 1
  maxReplicas: 3

  resources:
    requests:
      cpu: "4"
      memory: "16Gi"
      nvidia.com/gpu: "1"
    limits:
      cpu: "8"
      memory: "32Gi"
      nvidia.com/gpu: "1"

  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule

  runtimeArgs:
    - --dtype=auto
    - --max-model-len=8192
    - --gpu-memory-utilization=0.9

  trustRemoteCode: true
```

**Note:** The AI Model Operator automatically configures:
- `serving.knative.dev/progress-deadline: "360s"` annotation for 7B models
- Appropriate readiness and liveness probes with vLLM health endpoints
- Container-level health checks (though Knative ignores startupProbe)

### Large Model Configuration (20B+)

For larger models, the operator automatically adjusts timeout values. No manual annotation changes are needed - the operator derives appropriate values from model size and runtime.

### Very Large Model Configuration (70B+)

Same as above - the operator handles timeout configuration automatically based on the model's resource requirements.

### What the Progress Deadline Does

1. **Without progress-deadline**: Knative kills pods that aren't ready within ~2 minutes (default)
2. **With progress-deadline**: Knative waits the specified duration before marking the revision as failed
3. **readinessProbe**: Still controls when Knative routes traffic (vLLM /health returns 200 only after model loaded)
4. **livenessProbe**: Restarts hung pods after model is loaded

**Note:** The AI Model Operator automatically configures these settings when creating the InferenceService from your AIModel CR

---

## Step 3: Deploy via GitOps

```bash
# 1. Commit your AIModel CR
git add infra/k8s/aimodels/development/your-model.yaml
git commit -m "feat(aimodels): Deploy your-model with AIModel CR

- Configure vLLM runtime with appropriate timeouts
- Set resource limits for 7B model
- Enable GPU scheduling with tolerations

Resolves: timeout errors during autoscaling"
git push origin your-branch

# 2. Create PR and merge to development

# 3. ArgoCD syncs automatically (development) or manually sync
argocd app sync aimodels-development --server argocd.dev.otherjamesbrown.com
```

---

## Step 4: Validate Probes Are Working

### Check Pod Status During Model Loading

```bash
# Set kubeconfig
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml

# Watch pod status - should see 1/2 → 2/2 transition
kubectl get pods -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b -w

# Expected output during model loading:
# NAME                                        READY   STATUS    RESTARTS   AGE
# gpt-oss-20b-predictor-xxx-xxx              1/2     Running   0          30s
# gpt-oss-20b-predictor-xxx-xxx              2/2     Running   0          8m
```

### Check Probe Events

```bash
# View probe events
kubectl describe pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b | grep -A 15 "Events:"

# Expected events during loading:
# Events:
#   Warning  Unhealthy  2m   kubelet  Startup probe failed: HTTP probe failed with statuscode: 503
#   Warning  Unhealthy  1m   kubelet  Startup probe failed: HTTP probe failed with statuscode: 503
#   Normal   Started    30s  kubelet  Started container kserve-container
```

### Verify Probe Configuration in Pod

```bash
# Check probe config is applied
kubectl get pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b -o yaml | grep -A 8 "startupProbe:"
```

### Test Inference After Pod Ready

```bash
# Send test request immediately after pod shows 2/2 Running
curl -X POST "https://api.dev.otherjamesbrown.com/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello, how are you?"}],
    "max_tokens": 50
  }'

# Expected: Successful response (no timeout)
```

### Measure First Request Latency

```bash
# Time the first request after pod Ready
time curl -X POST "https://api.dev.otherjamesbrown.com/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Say hello"}],
    "max_tokens": 10
  }'

# Target: <5 seconds (NFR-002)
```

---

## Step 5: Monitor for Issues

### Check for Timeout Errors in Logs

```bash
# Check api-router-service for BACKEND_ERROR
kubectl logs -n system -l app=api-router-service --tail=100 | grep -i "BACKEND_ERROR\|timeout\|deadline"

# Should be zero after probes are working
```

### Check Pod Restart Count

```bash
# Verify pods are not being killed by liveness probe
kubectl get pods -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b \
  -o jsonpath='{.items[*].status.containerStatuses[*].restartCount}'

# Target: 0 restarts (except for legitimate crashes)
```

### Verify minReplicas Configuration

```bash
# Ensure production models have minReplicas: 1 to avoid cold starts
kubectl get inferenceservice -n development -o jsonpath='{range .items[*]}{.metadata.name}: {.spec.predictor.minReplicas}{"\n"}{end}'

# Expected: All models show "1" or higher
```

---

## Troubleshooting

### Problem: Pod stuck at 1/2 Running for too long

**Cause**: Model loading slower than expected or startup probe timeout too short.

**Solution**:
```bash
# Check vLLM logs for loading progress
kubectl logs -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b -c kserve-container --tail=50

# If model is still loading, wait or increase failureThreshold
# If model load failed, check GPU memory and resources
```

### Problem: Pod keeps restarting

**Cause**: Liveness probe killing pod before model loads (missing startup probe).

**Solution**: Ensure `startupProbe` is configured. The startup probe disables liveness/readiness probes until it succeeds.

### Problem: Timeout errors even after pod shows Ready

**Cause**: Readiness probe passing but vLLM not fully initialized.

**Solution**:
```bash
# Verify vLLM health endpoint directly
kubectl exec -n development -it <pod-name> -c kserve-container -- curl http://localhost:8000/health

# Expected: HTTP 200 OK
# If 503, model not fully loaded - check logs
```

### Problem: High liveness probe failure rate

**Cause**: Probe timeout too short or model under heavy load.

**Solution**: Increase `livenessProbe.failureThreshold` or `periodSeconds` for tolerance:
```yaml
livenessProbe:
  periodSeconds: 60  # Increase from 30
  failureThreshold: 5  # Increase from 3
```

---

## Rollback Procedure

If the AIModel deployment causes issues, revert the manifest:

```bash
# 1. Revert the commit
git revert HEAD
git push origin your-branch

# 2. Or delete the AIModel CR (this will remove the InferenceService)
kubectl delete aimodel your-model -n development

# 3. Verify cleanup
kubectl get aimodel -n development
kubectl get inferenceservice -n development
kubectl get pods -n development -l serving.kserve.io/inferenceservice=your-model -w
```

---

## Reference Templates

See `specs/018-model-readiness-probes/contracts/probe-config-templates.yaml` for copy-paste probe configurations for each model size category.

---

## Related Documentation

- [vLLM Deployment Best Practices](../best-practices/vllm-deployment-best-practices.md)
- [Quickstart Guide](../../specs/018-model-readiness-probes/quickstart.md)
- [Feature Specification](../../specs/018-model-readiness-probes/spec.md)
- [InferenceService Template](../../infra/k8s/kserve/templates/inference-service-vllm-template.yaml)

---

## Success Criteria

After implementing probes, verify:

- [ ] Zero `context deadline exceeded` errors during autoscaling
- [ ] Pods show `1/2 Running` during model load, then `2/2 Running` after
- [ ] First request after pod Ready succeeds within 5 seconds
- [ ] No `BACKEND_ERROR` responses in api-router-service logs
- [ ] Pod restart count remains at 0 (no liveness probe false positives)
- [ ] All InferenceServices have `minReplicas: 1` for production
