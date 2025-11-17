# Development Cluster Node Configuration

**Date**: 2025-01-27  
**Status**: Current state and recommendations

## Current State

### Actual Nodes in Dev Cluster

The development cluster currently has **3 standard CPU nodes**:

| Node Name | Instance Type | GPU | CPU | Memory | Labels |
|-----------|--------------|-----|-----|--------|--------|
| lke531921-770211-1b59efcf0000 | `g6-standard-4` | ❌ None | 4 | ~8GB | `node-type=gpu` (manually added) |
| lke531921-770211-3813f3520000 | `g6-standard-4` | ❌ None | 4 | ~8GB | `node-type=gpu` (manually added) |
| lke531921-770211-611c01ef0000 | `g6-standard-4` | ❌ None | 4 | ~8GB | `node-type=gpu` (manually added) |

**Key Points**:
- ✅ All nodes are `g6-standard-4` (4 vCPU, 8GB RAM)
- ❌ **No GPU resources available** (`nvidia.com/gpu: none`)
- ⚠️ Nodes are labeled `node-type=gpu` but **do not have actual GPUs**

### Terraform Configuration

According to `infra/terraform/environments/_shared/locals.tf`:

**Development Cluster** (lines 18-53):
```hcl
node_pools = [
  {
    type  = "g6-standard-4"
    count = 3
    autoscaler = {
      min = 3
      max = 6
    }
    labels = {
      role = "general"
    }
    taints = []
  }
]
```

**System Cluster** (lines 141-194):
```hcl
node_pools = [
  {
    type  = "g6-standard-6"
    count = 3
    labels = { role = "ops" }
  },
  {
    type  = "g1-gpu-rtx6000"  # ← GPU nodes are HERE
    count = 2
    labels = { role = "gpu" }
    taints = [
      {
        key    = "workload"
        value  = "gpu"
        effect = "NoSchedule"
      }
    ]
  }
]
```

## The Problem

1. **Development cluster has NO GPU nodes** - only standard CPU nodes
2. **GPU nodes are configured for the "system" cluster**, not development
3. **vLLM requires actual GPU hardware** - cannot run on CPU-only nodes
4. **I manually labeled dev nodes** with `node-type=gpu` to allow scheduling, but they don't have GPUs

## Impact

- ❌ **vLLM deployments will fail** - pods will schedule but cannot actually run without GPUs
- ❌ **Model loading will fail** - vLLM requires CUDA/GPU drivers
- ⚠️ **Testing is blocked** - cannot test vLLM inference without GPU nodes

## Solutions

### Option 1: Add GPU Nodes to Development Cluster (Recommended for Testing)

Update Terraform configuration to add GPU node pool to development:

```hcl
development = {
  node_pools = [
    {
      type  = "g6-standard-4"
      count = 3
      labels = { role = "general" }
    },
    {
      type  = "g1-gpu-rtx6000"  # Add GPU nodes
      count = 1  # Start with 1 for cost
      autoscaler = {
        min = 1
        max = 2
      }
      labels = {
        role = "gpu"
        node-type = "gpu"  # Match vLLM chart requirement
      }
      taints = [
        {
          key    = "gpu-workload"
          value  = "true"
          effect = "NoSchedule"
        }
      ]
    }
  ]
}
```

**Cost**: ~$1,200/month per GPU node (g1-gpu-rtx6000)

### Option 2: Deploy vLLM to System Cluster

Since GPU nodes exist in the system cluster:

1. Update vLLM deployment to use `system` namespace
2. Update node selector to match system cluster labels (`role: gpu`)
3. Update tolerations to match system cluster taints (`workload: gpu`)

**Pros**: No additional cost, GPU nodes already exist  
**Cons**: Mixes development workloads with system infrastructure

### Option 3: Use CPU-Only Testing (Not Recommended)

- Use mock inference service for testing
- Cannot test actual vLLM inference
- Limited value for validation

## Recommendation

For **development/testing**, add 1 GPU node to the development cluster:

1. **Cost**: ~$1,200/month (acceptable for dev)
2. **Flexibility**: Isolated from production/system workloads
3. **Testing**: Can validate full vLLM deployment workflow
4. **Scaling**: Can scale down to 0 when not in use (if autoscaling configured)

## Next Steps

1. **Immediate**: Remove `node-type=gpu` label from dev cluster nodes (they don't have GPUs)
2. **Short-term**: Decide on Option 1 (add GPU to dev) or Option 2 (use system cluster)
3. **Long-term**: Update Terraform to provision GPU nodes for development if needed

## Verification Commands

```bash
# Check current nodes
kubectl get nodes -o custom-columns=NAME:.metadata.name,TYPE:.metadata.labels.node\\.kubernetes\\.io/instance-type,GPU:.status.allocatable.nvidia\\.com/gpu

# Check which cluster has GPU nodes
# Dev cluster: Should show <none> for GPU
# System cluster: Should show actual GPU count
```

