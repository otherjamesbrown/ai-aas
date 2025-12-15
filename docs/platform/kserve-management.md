# KServe Management Guide

---
last_updated: 2025-12-12
document_type: guide
---

Quick reference for managing KServe InferenceServices on the AI-AAS platform.

## Table of Contents

- [Overview](#overview)
- [Common Operations](#common-operations)
- [Deploying New Models](#deploying-new-models)
- [Monitoring & Troubleshooting](#monitoring--troubleshooting)
- [Autoscaling Configuration](#autoscaling-configuration)
- [Best Practices](#best-practices)

---

## Overview

KServe manages model serving through `InferenceService` custom resources that automatically:
- Deploy models with vLLM inference engine
- Configure Knative autoscaling (0→N replicas)
- Expose OpenAI-compatible APIs
- Handle traffic routing via Istio

**Architecture**: Client → Istio → Knative → KServe → vLLM → Model

---

## Common Operations

### List All Models

```bash
kubectl get inferenceservice -n development
```

### Check Model Status

```bash
kubectl get inferenceservice mistral-7b-instruct -n development -o yaml
```

### View Model Logs

```bash
# Get pod name
kubectl get pods -n development -l serving.kserve.io/inferenceservice=mistral-7b-instruct

# View logs
kubectl logs -n development <pod-name> -c kserve-container --tail=100 -f
```

### Scale Model Manually

```bash
# Edit replicas
kubectl patch inferenceservice mistral-7b-instruct -n development \
  --type='json' -p='[{"op":"replace","path":"/spec/predictor/minReplicas","value":2}]'
```

### Delete Model

```bash
kubectl delete inferenceservice mistral-7b-instruct -n development
```

---

## Deploying New Models

### 1. Create AIModel Custom Resource

The platform uses AIModel CRs (managed by the AI Model Operator) to deploy models. The operator automatically creates and manages the underlying KServe InferenceServices.

Use the template from `infra/k8s/aimodels/staging/mistral-7b-instruct-v03.yaml`:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: <model-name>
  namespace: development
  labels:
    app: vllm-inference
    model: <model-name>
    environment: development
spec:
  # Model identification
  modelName: <model-name>
  modelID: <huggingface-model-id>

  # Deployment configuration
  enabled: true
  runtime: vllm

  # Replica configuration
  minReplicas: 1
  maxReplicas: 3

  # Resource requirements
  resources:
    requests:
      cpu: "4"
      memory: "16Gi"
      nvidia.com/gpu: "1"
    limits:
      cpu: "8"
      memory: "32Gi"
      nvidia.com/gpu: "1"

  # GPU node scheduling
  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
    - key: gpu-workload
      operator: Equal
      value: "true"
      effect: NoSchedule

  # vLLM runtime arguments
  runtimeArgs:
    - --dtype=auto
    - --max-model-len=4096
    - --gpu-memory-utilization=0.9

  # Security: only enable for trusted models
  trustRemoteCode: true
```

### 2. Apply via GitOps

```bash
# Save to infra/k8s/aimodels/development/<model-name>.yaml
git add infra/k8s/aimodels/development/<model-name>.yaml
git commit -m "Add <model-name> AIModel CR"
git push origin develop
```

ArgoCD will automatically sync and deploy the model. The AI Model Operator will create the KServe InferenceService and manage its lifecycle.

### 3. Verify Deployment

```bash
# Check AIModel status
kubectl get aimodel <model-name> -n development -w

# Check InferenceService created by the operator
kubectl get inferenceservice <model-name> -n development

# Check pods
kubectl get pods -n development -l serving.kserve.io/inferenceservice=<model-name>

# Test inference
kubectl port-forward -n development <pod-name> 8000:8000 &
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{"model":"<model-name>","messages":[{"role":"user","content":"test"}]}'
```

### 4. Update API Router

Add backend configuration to `services/api-router-service/deployments/helm/api-router-service/values-development.yaml`:

```yaml
backends:
  endpoints: "mistral-7b-instruct:http://mistral-7b-instruct-predictor-00001-private.development.svc.cluster.local,<model-name>:http://<model-name>-predictor-00001-private.development.svc.cluster.local"
```

And add routing policy in `services/api-router-service/cmd/router/main.go` bootstrap section.

---

## Monitoring & Troubleshooting

### Check Knative Revision Status

```bash
kubectl get revision -n development -l serving.kserve.io/inferenceservice=mistral-7b-instruct
```

### View Autoscaler Metrics

```bash
kubectl get hpa -n development
kubectl get podautoscaler -n knative-serving
```

### Common Issues

#### Pod Stuck in Pending

```bash
# Check events
kubectl describe pod <pod-name> -n development

# Common causes:
# - GPU not available: kubectl get nodes -l nvidia.com/gpu=true
# - Resource limits: kubectl describe node <node-name>
# - Image pull: kubectl get events -n development --sort-by='.lastTimestamp'
```

#### Model Loading Failures

```bash
# Check init container logs
kubectl logs <pod-name> -n development -c storage-initializer

# Check vLLM logs for model loading errors
kubectl logs <pod-name> -n development -c kserve-container | grep -i error
```

#### Inference Timeout

```bash
# Check pod health
kubectl get pods -n development -l serving.kserve.io/inferenceservice=<model-name>

# Check if pod scaled to zero
kubectl get revision -n development

# Trigger activation with test request
curl -X POST http://<model-name>-predictor.development.svc.cluster.local/v1/chat/completions ...
```

---

## Autoscaling Configuration

### Knative Autoscaling Annotations

Add to `InferenceService.metadata.annotations`:

| Annotation | Description | Default | Recommendation |
|------------|-------------|---------|----------------|
| `autoscaling.knative.dev/target` | Concurrent requests per pod | 100 | 5-10 for LLMs |
| `autoscaling.knative.dev/scaleDownDelay` | Time before scaling down | 0s | 5m (reduce thrashing) |
| `autoscaling.knative.dev/window` | Metrics window for decisions | 60s | 60s |
| `autoscaling.knative.dev/panicThreshold` | Rapid scaling threshold | 200% | 200% |
| `autoscaling.knative.dev/minScale` | Minimum replicas (0 = scale-to-zero) | 0 | 1 for production |
| `autoscaling.knative.dev/maxScale` | Maximum replicas | 0 (unlimited) | 3-5 for dev |

### Example: Optimized Production Config

```yaml
annotations:
  autoscaling.knative.dev/target: "5"
  autoscaling.knative.dev/scaleDownDelay: "10m"  # Longer delay for production
  autoscaling.knative.dev/window: "60s"
  autoscaling.knative.dev/panicThreshold: "200"
spec:
  predictor:
    minReplicas: 2  # Always keep 2 warm
    maxReplicas: 10
```

---

## Best Practices

### ✅ Do

- **Use private service URLs** for internal communication (`<model>-predictor-00001-private.development.svc.cluster.local`)
- **Set appropriate resource limits** based on model size and expected load
- **Monitor cold start times** and adjust `minReplicas` accordingly
- **Use GitOps** for all InferenceService changes
- **Test locally** via port-forward before updating API Router
- **Version your models** using labels (`model: mistral-7b-instruct`, `version: v0.2`)
- **Configure autoscaling** based on observed traffic patterns

### ❌ Don't

- **Don't use `kubectl apply` directly** - always commit to git
- **Don't skip testing** - verify inference before exposing to production
- **Don't set `minReplicas: 0`** for production (cold starts are slow)
- **Don't use public service URLs** internally (adds unnecessary latency via Istio)
- **Don't ignore resource limits** (GPU OOM kills are hard to debug)
- **Don't commit secrets** to git (use Sealed Secrets for HF tokens)
- **Don't use startupProbe** - KServe/Knative ignores it (see Health Probes section below)

### Resource Guidelines

| Model Size | CPU | Memory | GPU |
|------------|-----|--------|-----|
| 7B params | 4-8 | 16-32Gi | 1x NVIDIA GPU |
| 13B params | 8-16 | 32-64Gi | 1-2x NVIDIA GPU |
| 70B+ params | 16-32 | 64-128Gi | 2-4x NVIDIA GPU |

### Cold Start Optimization

For models with slow cold starts (>30s):
1. Set `minReplicas: 1` to keep one pod warm
2. Implement model caching with persistent volumes
3. Use smaller quantized models (int8, int4)
4. Pre-download models to nodes

### Health Probes for Large Models

**CRITICAL LIMITATION**: KServe/Knative does NOT support `startupProbe`. Any startupProbe defined in the InferenceService spec will be **silently ignored**.

**Problem**: Large models (20B+ parameters) can take 5-10 minutes to initialize. Without startupProbe support, the liveness probe starts immediately and may kill the pod during initialization.

**Solution**: Use lenient readinessProbe and livenessProbe settings:

```yaml
spec:
  predictor:
    containers:
      - name: kserve-container
        readinessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 60  # Delay before first check
          periodSeconds: 10        # Check every 10 seconds
          failureThreshold: 90     # Total: 60s + 90*10s = 16 minutes
          timeoutSeconds: 5
        livenessProbe:
          httpGet:
            path: /health
            port: 8000
          initialDelaySeconds: 600  # Don't check for first 10 minutes
          periodSeconds: 60         # Check every minute
          failureThreshold: 10      # Allow 10 minutes of downtime
          timeoutSeconds: 5
```

**Rationale**:
- **readinessProbe**: Prevents traffic routing until model is loaded (can take 16 min)
- **livenessProbe**: Avoids killing pod during initialization (10 min grace + 10 min tolerance)
- **Without proper settings**: Pod enters crash loop, restarting repeatedly

**Model Load Time Estimates** (GPU memory loading):
- 7B params: 30-60 seconds
- 13B params: 1-2 minutes
- 20B params: 5-10 minutes
- 70B+ params: 10-20 minutes

Adjust `initialDelaySeconds` and `failureThreshold` based on observed model load times.

---

## Quick Reference

### Useful Commands

```bash
# Port forward to test model
kubectl port-forward -n development svc/<model>-predictor-00001-private 8000:80

# Restart InferenceService
kubectl delete pod -n development -l serving.kserve.io/inferenceservice=<model>

# Watch autoscaling
watch kubectl get pods -n development -l serving.kserve.io/inferenceservice=<model>

# Check Istio routing
kubectl get virtualservice -n development

# View Knative Service
kubectl get ksvc -n development
```

### Environment URLs

- **Development**: `https://api.172.232.58.222.nip.io`
- **Web Portal**: `https://portal.172.232.58.222.nip.io`
- **Grafana**: `http://grafana.172.232.58.222.nip.io`

---

## Further Reading

- [KServe Documentation](https://kserve.github.io/website/)
- [Knative Autoscaling](https://knative.dev/docs/serving/autoscaling/)
- [vLLM Configuration](https://docs.vllm.ai/)
- [Platform Runbooks](../runbooks/)
- [KServe Migration Spec](../../specs/016-kserve-migration/)
