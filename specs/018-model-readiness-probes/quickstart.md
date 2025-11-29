# Quickstart: Model Readiness Probes

**Feature**: `018-model-readiness-probes`
**Time to Complete**: 5-10 minutes

## Prerequisites

- Access to the Kubernetes cluster
- kubectl configured with appropriate credentials
- Familiarity with InferenceService manifests

## TL;DR

Add these probes to your InferenceService container spec:

```yaml
containers:
  - name: kserve-container
    # ... your existing config ...
    startupProbe:
      httpGet:
        path: /health
        port: 8000
      initialDelaySeconds: 30
      periodSeconds: 10
      failureThreshold: 36  # ~6 min for 7B, 90 for 20B, 180 for 70B
      timeoutSeconds: 5
    readinessProbe:
      httpGet:
        path: /health
        port: 8000
      periodSeconds: 10
      failureThreshold: 3
      timeoutSeconds: 5
    livenessProbe:
      httpGet:
        path: /health
        port: 8000
      periodSeconds: 30
      failureThreshold: 3
      timeoutSeconds: 5
```

## Step-by-Step Guide

### Step 1: Determine Your Model Size

| Model Size | Examples | failureThreshold | Total Timeout |
|------------|----------|------------------|---------------|
| 7B | mistral-7b, llama-2-7b | 36 | ~6 minutes |
| 13B | llama-2-13b, codellama-13b | 60 | ~10 minutes |
| 20B | gpt-oss-20b | 90 | ~15 minutes |
| 70B+ | llama-2-70b, mixtral | 180 | ~30 minutes |

### Step 2: Identify Your vLLM Port

| vLLM Version | Port |
|--------------|------|
| v0.6.x+ | 8000 |
| v0.3.x | 8080 |

Check your InferenceService image tag to determine the version.

### Step 3: Add Probes to Your Manifest

Open your InferenceService manifest (e.g., `infra/k8s/kserve/models/my-model.yaml`) and add the probe configuration inside the `kserve-container`:

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: my-model
  namespace: development
spec:
  predictor:
    minReplicas: 1  # IMPORTANT: Keep at least 1 for production
    containers:
      - name: kserve-container
        image: vllm/vllm-openai:v0.10.2
        ports:
          - containerPort: 8000
            name: http1
            protocol: TCP
        # ADD PROBES HERE:
        startupProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 30
          periodSeconds: 10
          failureThreshold: 36  # Adjust based on model size!
          timeoutSeconds: 5
        readinessProbe:
          httpGet:
            path: /health
            port: 8000
          periodSeconds: 10
          failureThreshold: 3
          timeoutSeconds: 5
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          periodSeconds: 30
          failureThreshold: 3
          timeoutSeconds: 5
```

### Step 4: Deploy and Validate

```bash
# Apply the updated manifest
kubectl apply -f infra/k8s/kserve/models/my-model.yaml

# Watch the pod status (should see 1/2 → 2/2 transition)
kubectl get pods -n development -l serving.kserve.io/inferenceservice=my-model -w

# Expected output during model loading:
# NAME                                      READY   STATUS    RESTARTS   AGE
# my-model-predictor-xxx-xxx               1/2     Running   0          30s
# my-model-predictor-xxx-xxx               2/2     Running   0          5m
```

### Step 5: Verify Probes are Working

```bash
# Check probe events
kubectl describe pod -n development -l serving.kserve.io/inferenceservice=my-model | grep -A 10 "Events:"

# Expected: Startup probe failures during loading, then success
# Events:
#   Warning  Unhealthy  2m   kubelet  Startup probe failed: HTTP probe failed with statuscode: 503
#   Warning  Unhealthy  1m   kubelet  Startup probe failed: HTTP probe failed with statuscode: 503
#   Normal   Started    30s  kubelet  Started container kserve-container

# Test inference after pod is Ready
curl -X POST "https://api.example.com/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"model": "my-model", "messages": [{"role": "user", "content": "Hello"}]}'
```

## Troubleshooting

### Pod stuck at 1/2 Running

**Cause**: Model is still loading or probe timeout is too short.

**Fix**: Check vLLM logs for loading progress:
```bash
kubectl logs -n development -l serving.kserve.io/inferenceservice=my-model -c kserve-container
```

### Pod keeps restarting

**Cause**: Liveness probe killing pod before model loads.

**Fix**: Ensure you have a `startupProbe` configured. The startup probe disables liveness/readiness probes until it succeeds.

### Timeout errors immediately after pod Ready

**Cause**: Readiness probe is passing but model initialization not complete.

**Fix**: Verify vLLM `/health` endpoint behavior matches expectations:
```bash
kubectl exec -n development -it <pod-name> -c kserve-container -- curl http://localhost:8000/health
```

## Reference Templates

See `specs/018-model-readiness-probes/contracts/probe-config-templates.yaml` for copy-paste templates for each model size.

## Related Documentation

- [vLLM Deployment Best Practices](../../docs/best-practices/vllm-deployment-best-practices.md)
- [Model Initialization Timeout Strategy](../../docs/model-initialization.md)
- [InferenceService Template](../../infra/k8s/kserve/templates/inference-service-vllm-template.yaml)

