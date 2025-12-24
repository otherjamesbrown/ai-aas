# Spec 029: Issues to be Addressed

## Executive Summary

During implementation of TensorRT-LLM/Triton support (Spec 029), several critical issues were identified that require resolution before the feature can be fully operational. This document categorizes these issues, analyzes root causes, and provides resolution strategies.

**Epic**: `ai-aas-2t34` (Spec 029: GPU Infrastructure and Model Deployment Optimization)

---

## Issue Categories

| Category | Severity | Issues | Impact |
|----------|----------|--------|--------|
| Knative/KServe Deployment | Critical | 4 | GPU workloads fail to deploy or clean up |
| Infrastructure/Terraform | High | 2 | Cannot provision proper GPU node pools |
| Operator Logic | Medium | 2 | Incorrect resource lifecycle management |
| Documentation | Low | 3 | Knowledge gaps for operations |

---

## Critical Issue 1: Knative vs RawDeployment for GPU Workloads

### Problem Statement

The original spec assumed KServe with Knative Serving (Serverless mode) would work for TensorRT-LLM deployments. However, Knative Serving has fundamental incompatibilities with GPU workloads:

1. **Knative validation rejects `nodeSelector`** - GPU workloads require scheduling to GPU nodes via `nodeSelector`, but Knative's webhook validation doesn't allow `nodeSelector` in pod specs
2. **Single-port container restriction** - Knative requires exactly one container port, but TensorRT-LLM/Triton needs multiple ports (HTTP 8080, gRPC 9000, metrics 8002)
3. **Revision management overhead** - Knative creates immutable revisions for every change, causing stale revisions to consume GPU resources

### Current Workaround (Implemented)

The operator at `operators/ai-model-operator/internal/kserve/inferenceservice.go:238-248` has logic to use RawDeployment mode when `nodeSelector` is present:

```go
deploymentMode := "Serverless"
if len(b.nodeSelector) > 0 {
    // Knative Serving validation doesn't allow nodeSelector in pod spec.
    // Use RawDeployment mode to bypass Knative and use standard Kubernetes Deployments.
    deploymentMode = "RawDeployment"
}
```

### Issues with Current Approach

1. **Implicit behavior** - The switch to RawDeployment is implicit based on nodeSelector presence, not explicit
2. **No autoscaling** - RawDeployment mode loses Knative's scale-to-zero and autoscaling capabilities
3. **Inconsistent UX** - Same model recipe behaves differently depending on scheduling configuration

### Resolution Options

#### Option A: Default to RawDeployment for GPU Runtimes (Recommended)

Make `runtime: tensorrt-llm` or `runtime: triton` automatically use RawDeployment mode regardless of nodeSelector.

**Changes required:**
- `operators/ai-model-operator/controllers/aimodel_controller.go`: Add runtime check before deployment mode selection
- `operators/ai-model-operator/internal/kserve/inferenceservice.go`: Add `ForceRawDeployment` builder option

**Pros:**
- Explicit, predictable behavior
- GPU workloads always work correctly
- ClusterServingRuntime can define multiple ports

**Cons:**
- No scale-to-zero for GPU workloads (acceptable trade-off)

#### Option B: Investigate Knative Serving Configuration

Research if Knative can be configured to:
- Allow nodeSelector in pod specs (custom webhook?)
- Support multi-port containers

**Status:** Issue `ai-aas-v4dr` is tracking this investigation.

**Preliminary Finding:** This would require significant Knative customization and may not be maintainable.

### Recommendation

**Implement Option A**: Make RawDeployment the default for GPU runtimes (`tensorrt-llm`, `triton`, and possibly `vllm` when GPU is requested).

**Beads:**
- `ai-aas-v4dr`: Complete investigation (current status: open)
- New issue needed: "Implement explicit RawDeployment mode for GPU runtimes"

---

## Critical Issue 2: Knative Revision Garbage Collection

### Problem Statement

When a model deployment is updated, Knative creates a new revision but doesn't clean up old revisions. For GPU workloads, stale revisions:
- Continue to hold GPU memory allocations
- Prevent new deployments from scheduling (GPU exhaustion)
- Require manual cleanup

**Example:** `unsloth-gpt-oss-20b-predictor-00001` is a stale revision blocking resources.

### Root Cause Analysis

1. **Knative revision GC configuration** - Default GC settings are not aggressive enough
2. **Operator doesn't manage min-scale** - Old revisions retain `min-scale=1` annotation, preventing scale-to-zero
3. **RawDeployment mode** - When using RawDeployment, revisions are not created (standard Deployments instead), but legacy revisions from Serverless mode remain

### Resolution Strategy

#### Phase 1: Clean Up Existing Stale Revisions (Immediate)

```bash
# List all revisions
kubectl get revisions -A

# Delete stale revisions (those ending in -00001, -00002 etc. for models with newer active versions)
kubectl delete revision unsloth-gpt-oss-20b-predictor-00001 -n <namespace>
```

**Bead:** `ai-aas-9qs9` (P1 - Delete stale revision)

#### Phase 2: Configure Knative Revision GC (Short-term)

Modify Knative Serving ConfigMap to enable aggressive GC:

```yaml
# infra/k8s/knative/config/config-gc.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: config-gc
  namespace: knative-serving
data:
  # Delete revisions older than 1 hour that have 0% traffic
  retain-since-create-time: "1h"
  retain-since-last-active-time: "1h"
  min-non-active-revisions: "0"
  max-non-active-revisions: "2"
```

**Bead:** `ai-aas-bbxv` (P2 - Configure Knative revision GC)

#### Phase 3: Operator Sets min-scale=0 on Old Revisions (Medium-term)

When updating a deployment, the operator should:
1. Identify previous revision
2. Patch annotation `autoscaling.knative.dev/min-scale: "0"`
3. Allow Knative GC to clean it up

**Bead:** `ai-aas-ju3g` (P2 - Fix operator min-scale behavior)

#### Phase 4: Migrate to RawDeployment (Long-term)

Once Option A from Issue 1 is implemented, new GPU deployments won't create Knative revisions. This eliminates the revision GC problem entirely for GPU workloads.

---

## High Issue 3: Terraform Node Pool Limitations

### Problem Statement

The Linode Terraform provider (`linode_lke_cluster`) has critical limitations:

1. **No support for node labels** - Cannot set `nvidia.com/gpu.product` or custom labels via Terraform
2. **No support for node taints** - Cannot taint GPU nodes via Terraform
3. **Stateless node pools** - Linode can replace nodes at any time, losing manually-applied labels

### Current Workaround

Manual script execution after Terraform:

```bash
./scripts/infra/apply-gpu-node-labels.sh
```

This script must be re-run whenever:
- Node pools are scaled
- Nodes are replaced
- New GPU nodes join the cluster

### Infrastructure Impact

| Environment | GPU Node Pools | Labels Required |
|-------------|----------------|-----------------|
| Development | 1x RTX6000 | `nvidia.com/gpu.product`, `ai-aas.io/gpu-class` |
| Staging | 1x RTX4000-Ada-M, 1x RTX4000-Ada-L | Same + size class labels |
| Production | TBD | TBD |

### Resolution Strategy

#### Option A: DaemonSet-based Auto-labeling (Recommended)

Deploy a DaemonSet that:
1. Detects GPU type from NVIDIA device info
2. Applies appropriate labels automatically
3. Runs on node startup/restart

**Implementation:**
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: gpu-node-labeler
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: gpu-node-labeler
  template:
    spec:
      tolerations:
        - operator: Exists
      containers:
        - name: labeler
          image: bitnami/kubectl:latest
          command: ["/bin/sh", "-c"]
          args:
            - |
              # Detect GPU and apply labels
              GPU_TYPE=$(nvidia-smi --query-gpu=name --format=csv,noheader | head -1)
              kubectl label node $NODE_NAME nvidia.com/gpu.product="$GPU_TYPE" --overwrite
```

**Pros:**
- Automatic, no manual intervention
- Survives node replacement
- GitOps compatible

**Cons:**
- Requires RBAC for labeling nodes
- Slight delay between node creation and labeling

#### Option B: Linode Node Pool Labels (Future)

Monitor Linode provider for label/taint support:
- [GitHub Issue #XXX](https://github.com/linode/terraform-provider-linode/issues)

#### Option C: Alternative Cloud Provider

For production, consider providers with better Terraform node pool support (GKE, EKS, AKS).

**Beads:**
- `ai-aas-gxo0`: Document current Terraform configuration
- `ai-aas-6yfl`: Add GPU Large node pool for TensorRT-LLM
- New issue needed: "Implement GPU node auto-labeler DaemonSet"

---

## High Issue 4: GPU Node Pool Sizing for TensorRT-LLM

### Problem Statement

TensorRT-LLM models have different resource requirements than vLLM:

| Aspect | vLLM | TensorRT-LLM |
|--------|------|--------------|
| GPU Memory | Model weights + KV cache | Pre-compiled engine (larger) |
| CPU Memory | ~32GB typical | ~48-64GB (engine loading) |
| Startup Time | ~5 minutes | ~10 minutes (engine load) |
| Instance Type | RTX4000-Ada-M (20GB VRAM, 32GB RAM) | RTX4000-Ada-L (20GB VRAM, 64GB RAM) |

### Current State

- Development: 1x RTX6000 (48GB VRAM) - sufficient but expensive
- Staging: 1x RTX4000-Ada-M, 1x RTX4000-Ada-L - appropriate sizing
- Production: No GPU nodes defined

### Resolution

1. **Development:** Keep RTX6000 for flexibility during development
2. **Staging:** Current configuration is appropriate
3. **Production:** Define based on production model requirements

**Changes to `infra/terraform/environments/_shared/locals.tf`:**

```hcl
# staging node_pools - add dedicated TensorRT-LLM pool
{
  type  = "g2-gpu-rtx4000a1-l"  # Large instance for TensorRT-LLM
  count = 1
  autoscaler = {
    min = 0  # Scale to zero when no TRT-LLM workloads
    max = 2
  }
  # Labels applied via auto-labeler DaemonSet:
  # ai-aas.io/gpu-class=rtx4000-large
  # ai-aas.io/runtime=tensorrt-llm
}
```

**Bead:** `ai-aas-6yfl` (P2 - Add GPU Large node pool)

---

## Medium Issue 5: Missing `nvidia-container-toolkit` DaemonSet

### Problem Statement

The `nvidia-container-toolkit-daemonset` is in CrashLoopBackOff state across GPU nodes.

**Bead:** `ai-aas-bxs` (P2 - nvidia-container-toolkit-daemonset CrashLoopBackOff)

### Root Cause (Suspected)

1. Incorrect container runtime configuration
2. Missing NVIDIA drivers on node
3. Incompatible toolkit version

### Resolution

1. Check node driver installation:
   ```bash
   kubectl debug -it node/<gpu-node> --image=ubuntu -- nvidia-smi
   ```

2. Verify GPU Operator configuration:
   ```bash
   kubectl get clusterpolicy -o yaml
   ```

3. Check toolkit logs:
   ```bash
   kubectl logs -n gpu-operator -l app=nvidia-container-toolkit-daemonset
   ```

---

## Documentation Gaps

### Gap 1: Terraform Node Pool Configuration

**Bead:** `ai-aas-gxo0`

Create `docs/runbooks/terraform-node-pools.md` covering:
- Available Linode GPU instance types
- How to add/modify node pools
- Post-Terraform label/taint application
- Environment-specific configurations

### Gap 2: Knative Revision Management

**Bead:** `ai-aas-hlwj`

Create `docs/runbooks/knative-revision-management.md` covering:
- How revisions work
- GC configuration
- Manual cleanup procedures
- RawDeployment vs Serverless tradeoffs

### Gap 3: GPU Workload Troubleshooting

**Bead:** `ai-aas-25l1`

Create `docs/runbooks/gpu-troubleshooting.md` covering:
- nvidia-container-toolkit issues
- GPU memory exhaustion
- Node scheduling failures
- Driver compatibility

---

## Implementation Roadmap

### Phase 1: Immediate Fixes (P1)

| Task | Bead | Action |
|------|------|--------|
| Delete stale revision | ai-aas-9qs9 | Manual kubectl delete |
| Fix nvidia-toolkit crash | ai-aas-bxs | Debug and fix DaemonSet |

### Phase 2: Short-term (P2)

| Task | Bead | Action |
|------|------|--------|
| Configure Knative GC | ai-aas-bbxv | Apply config-gc ConfigMap |
| Add GPU Large pool | ai-aas-6yfl | Update Terraform and apply |
| Document Terraform | ai-aas-gxo0 | Create runbook |
| Fix operator min-scale | ai-aas-ju3g | Modify operator code |

### Phase 3: Medium-term

| Task | Bead | Action |
|------|------|--------|
| Knative investigation | ai-aas-v4dr | Complete analysis, document decision |
| GPU auto-labeler | NEW | Implement DaemonSet solution |
| Default RawDeployment for GPU | NEW | Modify operator deployment logic |

### Phase 4: Long-term

| Task | Action |
|------|--------|
| Production GPU planning | Define production node pools |
| Multi-cluster GPU support | Consider dedicated GPU clusters |

---

## Decision Required: Knative Deprecation for GPU Workloads

### Context

The issues documented above reveal a fundamental mismatch between Knative Serving and GPU workloads:

1. Knative was designed for stateless, quickly-starting serverless functions
2. GPU workloads are stateful (model loaded in GPU memory), slow to start (5-10 minutes), and expensive

### Options

| Option | Description | Recommendation |
|--------|-------------|----------------|
| A | Abandon Knative for GPU, use RawDeployment exclusively | **Recommended** |
| B | Maintain Knative with workarounds | Not recommended - ongoing operational burden |
| C | Fork/modify Knative for GPU | Not recommended - maintenance nightmare |

### Recommended Decision

**For GPU workloads (`tensorrt-llm`, `triton`, `vllm` with GPU):**
- Default to RawDeployment mode in KServe
- Implement custom HPA for GPU-aware autoscaling (future)
- Remove Knative from the GPU workload path entirely

**For non-GPU workloads (embeddings on CPU, etc.):**
- Continue using Knative/Serverless mode
- Benefit from scale-to-zero for cost savings

---

## Summary

The implementation of Spec 029 revealed that the original design assumption (KServe with Knative Serving) is not suitable for GPU workloads. The recommended path forward:

1. **Immediately:** Clean up stale resources, fix nvidia-toolkit
2. **Short-term:** Configure Knative GC, add proper node pools
3. **Medium-term:** Make RawDeployment the default for GPU runtimes
4. **Long-term:** Consider dedicated GPU cluster architecture

This document should be updated as issues are resolved and new decisions are made.
