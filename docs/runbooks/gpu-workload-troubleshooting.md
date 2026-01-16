# GPU Workload Troubleshooting

---
last_updated: 2025-12-30
document_type: runbook
---

## Overview

This runbook covers diagnosing and recovering from GPU workload issues on the AI-AAS platform.

## Diagnostic Commands

### Check Model Pod Status

```bash
# List all model pods
kubectl get pods -n development -l serving.kserve.io/inferenceservice

# Check specific model pods
kubectl get pods -n development -l serving.kserve.io/inferenceservice=<model-name>

# Check pod restart counts and reasons
kubectl get pods -n development -o wide | grep -E "NAME|Restart"
```

### Check InferenceService Status

```bash
# List all InferenceServices
kubectl get inferenceservices -A

# Get detailed status
kubectl describe inferenceservice <name> -n development

# Check conditions
kubectl get inferenceservice <name> -n development -o jsonpath='{.status.conditions}' | jq
```

### Check AIModel Status

```bash
# List all AIModels
kubectl get aimodel -A -o wide

# Check specific model phase
kubectl get aimodel <name> -n development -o jsonpath='{.status.phase}'
```

### Check Knative Revisions

```bash
# List revisions (look for stuck ones)
kubectl get revisions -n development -o wide

# Check revision conditions
kubectl describe revision <name>-predictor-00001 -n development
```

### Check GPU Node Availability

```bash
# List GPU nodes
kubectl get nodes -l nvidia.com/gpu.present=true

# Check GPU allocation
kubectl describe nodes | grep -A 10 'Allocated resources'

# Check GPU capacity vs requests
kubectl get nodes -l nvidia.com/gpu.present=true -o custom-columns=NAME:.metadata.name,GPU:.status.capacity.nvidia\\.com/gpu
```

### Check Cluster Autoscaler

```bash
# Check autoscaler status
kubectl get configmap cluster-autoscaler-status -n kube-system -o yaml

# Check autoscaler logs
kubectl logs -n kube-system -l app=cluster-autoscaler --tail=50
```

## Common Issues and Recovery

### Issue: Pod Stuck in Pending (No GPU Available)

**Symptoms:**
- Pod shows `Pending` status
- Events show "Insufficient nvidia.com/gpu"

**Recovery:**
1. Check if other pods are using GPUs: `kubectl get pods -A -o wide | grep nvidia`
2. Scale down non-essential models
3. Wait for cluster autoscaler to add nodes (if configured)

### Issue: Pod CrashLoopBackOff

**Symptoms:**
- Pod repeatedly crashes
- High restart count

**Diagnosis:**
```bash
# Check pod logs
kubectl logs -n development <pod-name> --previous

# Check events
kubectl get events -n development --sort-by='.lastTimestamp' | grep <pod-name>
```

**Common Causes:**
- OOM (Out of Memory) - increase memory limits
- CUDA errors - check GPU driver compatibility
- Model loading failure - check storage/S3 access

### Issue: Stuck Knative Revision

**Symptoms:**
- Old revision won't scale down
- New revision not becoming active

**Recovery:**
```bash
# Delete stuck revision (CAUTION: may cause brief downtime)
kubectl delete revision -n development <name>-predictor-00002

# Force InferenceService recreation
kubectl delete inferenceservice <name> -n development
# Operator will recreate it
```

### Issue: GPU Memory Exhausted

**Symptoms:**
- CUDA OOM errors in logs
- Pod crashes during model loading

**Recovery:**
1. Reduce model batch size or max_model_len
2. Use quantization (e.g., --dtype=float16)
3. Use larger GPU node

### Issue: Model Loading Timeout

**Symptoms:**
- Pod killed by liveness probe during startup
- Logs show model still loading when killed

**Recovery:**
1. Increase `initialDelaySeconds` on liveness probe
2. Check if model needs to be re-downloaded (S3 access)
3. Consider pre-warming the model cache

## Recovery Procedures

### Force Delete Stuck Pod

```bash
# Standard delete (wait for graceful shutdown)
kubectl delete pod <name> -n development

# Force delete (immediate, use with caution)
kubectl delete pod <name> -n development --force --grace-period=0
```

### Scale Down Operator Temporarily

If the operator is causing issues (e.g., creating too many revisions):

```bash
# Scale down operator
kubectl scale deployment ai-model-operator -n ai-model-operator-system --replicas=0

# Fix the issue manually

# Scale back up
kubectl scale deployment ai-model-operator -n ai-model-operator-system --replicas=1
```

### Full Model Reset

If a model is completely broken:

```bash
# 1. Disable the model
kubectl patch aimodel <name> -n development --type=merge -p '{"spec":{"enabled":false}}'

# 2. Wait for InferenceService deletion
kubectl get inferenceservice <name> -n development  # Should show NotFound

# 3. Re-enable
kubectl patch aimodel <name> -n development --type=merge -p '{"spec":{"enabled":true}}'
```

## Root Cause Indicators

| Symptom | Likely Cause |
|---------|--------------|
| Pod Pending + "Insufficient nvidia.com/gpu" | No GPU capacity |
| Pod CrashLoopBackOff + "CUDA out of memory" | GPU memory exhausted |
| Pod killed after 90-150s | Liveness probe timeout during loading |
| Revision stuck in "Activating" | Knative autoscaler issue |
| Multiple revisions accumulating | Operator creating unnecessary updates |
| Pod ImagePullBackOff | Registry auth or image not found |

## Monitoring

### Key Metrics to Watch

- `kube_pod_container_status_restarts_total` - Restart counts
- `nvidia_gpu_memory_used_bytes` - GPU memory usage
- `kube_pod_status_phase` - Pod phases

### Grafana Dashboards

- GPU Node Dashboard: Shows GPU utilization across nodes
- Model Inference Dashboard: Shows model latency and throughput

## Related Documentation

- [GPU Deployment Mode](gpu-deployment-mode.md) - RawDeployment vs Serverless
- [Force InferenceService Recreation](force-inferenceservice-recreation.md) - Recovery procedure
- [KServe Migration Deployment](kserve-migration-deployment.md) - Deployment patterns
