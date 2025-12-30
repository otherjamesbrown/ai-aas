# KServe Operator Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-30

---

## Domain

KServe-specific patterns for deploying and managing inference workloads. This context supplements the ai-model-operator context with KServe-specific resource sizing and configuration guidance.

Hand off to:
- AIModel CR logic → `operator-developer`
- Cluster setup → `infra-ops-manager`
- Model routing → `go-services-developer` (api-router)

---

## Resource Sizing Patterns

### GPU Memory Planning

**CRITICAL**: GPU memory is the primary constraint for LLM inference. Underestimating causes OOM; overestimating wastes expensive resources.

```yaml
resource_estimation:
  formula: |
    GPU_Memory_Required = Model_Size_GB × Loading_Factor + KV_Cache_Size

    Loading_Factor: 1.2 (typical overhead for weights + optimizer states)
    KV_Cache_Size: Depends on max_model_len and batch_size

  common_sizes:
    7B_float16:   ~14 GB   # Fits on single T4 (16GB) or A10G (24GB)
    13B_float16:  ~26 GB   # Requires A100-40GB or 2× A10G
    20B_float16:  ~40 GB   # Requires A100-40GB/80GB
    70B_float16:  ~140 GB  # Requires 2× A100-80GB or 4× A100-40GB
    70B_int8:     ~70 GB   # Quantized fits 2× A100-40GB

  safety_margin: 10-15%   # Leave headroom for KV cache growth
```

### Resource Request Patterns

```yaml
# Small model (7B) on T4
resources:
  limits:
    nvidia.com/gpu: 1
    memory: 16Gi
    cpu: "4"
  requests:
    nvidia.com/gpu: 1
    memory: 14Gi
    cpu: "2"

# Medium model (20B) on A100-40GB
resources:
  limits:
    nvidia.com/gpu: 1
    memory: 48Gi
    cpu: "8"
  requests:
    nvidia.com/gpu: 1
    memory: 44Gi
    cpu: "4"

# Large model (70B) on A100-80GB (tensor parallel=2)
resources:
  limits:
    nvidia.com/gpu: 2
    memory: 192Gi
    cpu: "16"
  requests:
    nvidia.com/gpu: 2
    memory: 180Gi
    cpu: "8"
```

### Anti-pattern: Memory Overcommitment

```yaml
# WRONG: Memory request exceeds GPU memory
# This pod will OOM crash during model loading
resources:
  requests:
    nvidia.com/gpu: 1     # Gets T4 with 16GB VRAM
    memory: 32Gi          # But model needs 26GB GPU memory!
  limits:
    memory: 64Gi          # CPU memory won't save you - GPU will OOM

# WRONG: No memory limit
# Pod can consume all node memory, affecting other workloads
resources:
  requests:
    nvidia.com/gpu: 1
    # No memory limit = dangerous

# CORRECT: Match resources to model requirements
resources:
  requests:
    nvidia.com/gpu: 1     # A100-40GB node pool
    memory: 48Gi          # CPU memory for model loading
  limits:
    nvidia.com/gpu: 1
    memory: 64Gi
    cpu: "8"
```

---

## Model Cache with PersistentVolume

### Why Use PV for Model Cache

Without a PV, models are downloaded on every pod restart. Large models (50GB+) can take 10-30 minutes to download.

```yaml
pattern:
  benefit: "Model cached on disk, survives pod restart"
  tradeoff: "PV must be in same zone as GPU node"

  storageClass: "premium-rwo"  # High IOPS recommended
  size: |
    7B model:   ~15 GB
    20B model:  ~45 GB
    70B model:  ~150 GB
    # Add 20% buffer for temp files during download
```

### PV Configuration

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: model-cache-7b
  namespace: system
spec:
  accessModes:
    - ReadWriteOnce
  storageClassName: premium-rwo
  resources:
    requests:
      storage: 20Gi

# Mount in InferenceService container
containers:
  - name: kserve-container
    volumeMounts:
      - name: model-cache
        mountPath: /mnt/models
volumes:
  - name: model-cache
    persistentVolumeClaim:
      claimName: model-cache-7b
```

### Anti-pattern: EmptyDir for Large Models

```yaml
# WRONG: EmptyDir is ephemeral
volumes:
  - name: model-cache
    emptyDir: {}  # Lost on pod restart!

# WRONG: PV size too small
resources:
  requests:
    storage: 10Gi  # Model is 45GB - will fail to download!

# CORRECT: Properly sized persistent storage
volumes:
  - name: model-cache
    persistentVolumeClaim:
      claimName: model-cache-20b  # 60Gi PVC
```

---

## Deployment Mode Selection

### When to Use RawDeployment

```yaml
rawdeployment:
  use_when:
    - GPU workloads (no surge capacity for rolling updates)
    - Need persistent TCP connections
    - Custom networking requirements
  tradeoffs:
    - No scale-to-zero
    - No Knative revision management
    - Simpler debugging (standard Deployment)
```

### When to Use Serverless

```yaml
serverless:
  use_when:
    - CPU-only workloads
    - Bursty traffic patterns
    - Cost optimization via scale-to-zero
  tradeoffs:
    - Requires Istio
    - Cold start latency (30-60s for models)
    - Knative revision complexity
```

---

## Troubleshooting

### Common Issues

| Symptom | Likely Cause | Solution |
|---------|--------------|----------|
| `CUDA out of memory` | Model too large for GPU | Use smaller GPU or quantization |
| Pod stuck `Pending` | No GPU nodes available | Check autoscaler, scale node pool |
| Model loads then OOM | KV cache grows too large | Reduce `max_model_len` or batch size |
| Pod killed at 90-150s | Liveness probe timeout | Increase `initialDelaySeconds` |
| PVC stuck `Pending` | Storage class not in zone | Check zone affinity |

### Debug Commands

```bash
# Check GPU memory on node
kubectl exec -it <pod> -- nvidia-smi

# Check model loading progress
kubectl logs -n system <pod> -f

# Check InferenceService status
kubectl get inferenceservice -n system <name> -o yaml

# Check Knative revision status (serverless only)
kubectl get revision -n system -o wide
```

---

## Sources

| What | Where |
|------|-------|
| AIModel CRD | `operators/ai-model-operator/api/v1alpha1/` |
| InferenceService builder | `operators/ai-model-operator/internal/kserve/` |
| ClusterServingRuntimes | `gitops/clusters/*/apps/kserve-runtimes.yaml` |
| GPU Runbook | `docs/runbooks/gpu-workload-troubleshooting.md` |

---

## Checklist

Before deploying a model:
- [ ] Calculated GPU memory requirements
- [ ] Set appropriate resource requests/limits
- [ ] PVC sized correctly (if using persistent cache)
- [ ] Liveness probe has adequate `initialDelaySeconds`
- [ ] Tested model loading time
- [ ] Verified GPU node availability
