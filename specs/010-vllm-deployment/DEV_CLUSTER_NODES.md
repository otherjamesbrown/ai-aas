# Development Cluster Node Configuration

**Date**: 2025-11-20
**Status**: ✅ GPU node available and vLLM deployed successfully

## Current State

### Actual Nodes in Dev Cluster

The development cluster has **4 nodes** (3 CPU + 1 GPU):

| Node Name | Instance Type | GPU | CPU | Memory | Status |
|-----------|--------------|-----|-----|--------|--------|
| lke531921-770211-1b59efcf0000 | `g6-standard-4` | ❌ None | 4 | ~8GB | CPU node |
| lke531921-770211-3813f3520000 | `g6-standard-4` | ❌ None | 4 | ~8GB | CPU node |
| lke531921-770211-611c01ef0000 | `g6-standard-4` | ❌ None | 4 | ~8GB | CPU node |
| lke531921-776664-51386eeb0000 | GPU instance | ✅ **1x NVIDIA** | ? | ? | **GPU node** (Linode ID: 87352812) |

**Key Points**:
- ✅ GPU node available: `lke531921-776664-51386eeb0000`
- ✅ **1x NVIDIA GPU allocatable** (`nvidia.com/gpu: 1`)
- ✅ vLLM deployment `vllm-gpt-oss-20b` running successfully
- ✅ Inference endpoint responding correctly

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

## Current Deployments

### vLLM Deployment

**Status**: ✅ Running successfully

- **Deployment**: `vllm-gpt-oss-20b` in `system` namespace
- **Pod**: `vllm-gpt-oss-20b-7ccc4c947b-lg2h9` (Running, 1/1 ready)
- **Service**: `vllm-gpt-oss-20b` (ClusterIP: 10.128.254.198:8000)
- **Model**: `unsloth/gpt-oss-20b` (20B parameters)
- **GPU Node**: Scheduled on `lke531921-776664-51386eeb0000`
- **Uptime**: 21+ hours

### Access Instructions

```bash
# Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Port-forward to service
kubectl port-forward -n system svc/vllm-gpt-oss-20b 8000:8000

# Test inference
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "unsloth/gpt-oss-20b",
    "messages": [{"role": "user", "content": "What is the capital of France?"}],
    "max_tokens": 50,
    "temperature": 0.1
  }'
```

**Expected response**: Model correctly answers "Paris"

## Cluster Configuration

### Node Pool 770211 (CPU Nodes)
- **Type**: g6-standard-4
- **Count**: 3 nodes
- **Purpose**: General workloads
- **Labels**: `role=general`

### Node Pool 776664 (GPU Node)
- **Type**: GPU instance (Linode)
- **Count**: 1 node
- **Linode ID**: 87352812
- **GPU**: 1x NVIDIA GPU
- **Purpose**: vLLM inference workloads
- **Status**: ✅ Active and running vLLM

## Historical Context

This document was previously incorrect (dated 2025-01-27) and stated:
- ❌ "Development cluster has NO GPU nodes"
- ❌ "GPU nodes are configured for the system cluster"
- ❌ "vLLM deployments will fail"

**Actual Reality (2025-11-20)**:
- ✅ Development cluster **DOES** have a GPU node
- ✅ vLLM deployment is running successfully
- ✅ Inference endpoint is responding correctly
- ✅ No "system cluster" - only the dev cluster (LKE 531921)

The confusion arose from outdated documentation that didn't reflect the actual infrastructure state.

## Verification Commands

```bash
# Check current nodes
kubectl get nodes -o custom-columns=NAME:.metadata.name,TYPE:.metadata.labels.node\\.kubernetes\\.io/instance-type,GPU:.status.allocatable.nvidia\\.com/gpu

# Check which cluster has GPU nodes
# Dev cluster: Should show <none> for GPU
# System cluster: Should show actual GPU count
```

