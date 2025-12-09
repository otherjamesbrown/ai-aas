# RTX 4000 Ada Deployment Guide

## Overview

This guide covers deploying vLLM on a single NVIDIA RTX 4000 Ada GPU (20GB VRAM).

**GPU Specifications**:
- **Model**: NVIDIA RTX 4000 Ada Generation
- **Memory**: 20GB GDDR6
- **Architecture**: Ada Lovelace
- **CUDA Cores**: 6,144
- **TDP**: 70W

## Supported Models

### ✅ Recommended Models (Fit comfortably in 20GB)

| Model | Size | GPU Memory | Tokens/sec | Use Case |
|-------|------|------------|------------|----------|
| **Llama-2-7B-chat** | 7B | ~14GB | 30-40 | Chat, general |
| **Mistral-7B-Instruct** | 7B | ~14GB | 35-45 | Instructions |
| **Llama-2-13B-chat** | 13B | ~18GB | 15-25 | Better quality |
| **CodeLlama-7B** | 7B | ~14GB | 30-40 | Code generation |
| **TinyLlama-1.1B** | 1.1B | ~3GB | 100+ | Ultra-fast |

### ⚠️ Possible but Tight (Requires optimization)

| Model | Size | GPU Memory | Notes |
|-------|------|------------|-------|
| **Llama-2-13B** | 13B | ~19GB | Set `gpu_memory_utilization=0.95` |
| **Mistral-7B (32k context)** | 7B | ~18GB | Reduce max_model_len |

### ❌ Too Large (Won't fit)

| Model | Size | GPU Memory Needed |
|-------|------|-------------------|
| Llama-2-70B | 70B | 80GB+ |
| CodeLlama-34B | 34B | 40GB+ |
| Mixtral-8x7B | 47B | 50GB+ |

## Step 1: Add GPU Node to Cluster

### Option A: Cloud Provider

**GKE (Google Cloud)**:
```bash
gcloud container node-pools create gpu-pool-rtx4000 \
  --cluster=ai-aas-dev \
  --zone=us-central1-a \
  --machine-type=n1-standard-4 \
  --accelerator=type=nvidia-rtx-4000-ada,count=1 \
  --num-nodes=1 \
  --node-labels=node-type=gpu \
  --node-taints=gpu-workload=true:NoSchedule \
  --enable-autoscaling \
  --min-nodes=0 \
  --max-nodes=1
```

**EKS (AWS)**:
```bash
# AWS doesn't have RTX 4000 Ada directly
# Use g5.xlarge (A10G 24GB) as alternative
eksctl create nodegroup \
  --cluster=ai-aas-dev \
  --name=gpu-nodes \
  --node-type=g5.xlarge \
  --nodes=1 \
  --nodes-min=0 \
  --nodes-max=1 \
  --node-labels="node-type=gpu" \
  --node-taints="gpu-workload=true:NoSchedule"
```

**AKS (Azure)**:
```bash
# Azure uses NCasT4_v3 series
az aks nodepool add \
  --cluster-name ai-aas-dev \
  --name gpupool \
  --node-count 1 \
  --node-vm-size Standard_NCasT4_v3 \
  --labels node-type=gpu \
  --node-taints gpu-workload=true:NoSchedule
```

### Option B: On-Premises / Bare Metal

1. **Install NVIDIA Drivers** (if not already installed):
```bash
# Install CUDA drivers
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
```

2. **Label the node**:
```bash
kubectl label nodes <your-node-name> node-type=gpu
kubectl taint nodes <your-node-name> gpu-workload=true:NoSchedule
```

3. **Verify GPU is detected**:
```bash
kubectl describe node <your-node-name> | grep nvidia.com/gpu
# Should show: nvidia.com/gpu: 1
```

## Step 2: Deploy vLLM with RTX 4000 Ada Configuration

### Deploy Llama-2-7B (Recommended for testing)

```bash
# Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Deploy using the RTX 4000 Ada optimized values
helm install llama-7b-dev \
  infra/helm/charts/vllm-deployment \
  -f infra/helm/charts/vllm-deployment/values-rtx4000ada.yaml \
  --set model.path=meta-llama/Llama-2-7b-chat-hf \
  --set model.size=small \
  --namespace system \
  --create-namespace
```

### Deploy Mistral-7B (Alternative)

```bash
helm install mistral-7b-dev \
  infra/helm/charts/vllm-deployment \
  -f infra/helm/charts/vllm-deployment/values-rtx4000ada.yaml \
  --set model.path=mistralai/Mistral-7B-Instruct-v0.2 \
  --set model.size=small \
  --namespace system
```

### Deploy Llama-2-13B (Larger model)

```bash
# For 13B models, increase memory utilization
helm install llama-13b-dev \
  infra/helm/charts/vllm-deployment \
  -f infra/helm/charts/vllm-deployment/values-rtx4000ada.yaml \
  --set model.path=meta-llama/Llama-2-13b-chat-hf \
  --set model.size=small \
  --set vllm.env[0].name=VLLM_GPU_MEMORY_UTILIZATION \
  --set vllm.env[0].value="0.90" \
  --set resources.limits.memory=19Gi \
  --namespace system
```

## Step 3: Monitor Deployment

### Watch pod startup

```bash
kubectl get pods -n system -w | grep vllm
```

### Check logs

```bash
kubectl logs -n system -l app.kubernetes.io/name=vllm-deployment -f
```

### Expected startup time:
- **7B models**: 2-4 minutes
- **13B models**: 4-7 minutes

### Check readiness

```bash
kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment
# Wait for READY 1/1
```

## Step 4: Test the Deployment

### Port-forward to vLLM service

```bash
kubectl port-forward -n system svc/llama-7b-dev 8000:8000 &
```

### Test with curl

```bash
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-2-7b-chat-hf",
    "messages": [
      {"role": "user", "content": "In one word, what is the capital of France?"}
    ],
    "max_tokens": 10,
    "temperature": 0.1
  }'
```

### Expected response

```json
{
  "id": "cmpl-xxx",
  "object": "chat.completion",
  "created": 1732035600,
  "model": "meta-llama/Llama-2-7b-chat-hf",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Paris"
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 18,
    "completion_tokens": 1,
    "total_tokens": 19
  }
}
```

### Run E2E Test

```bash
# From project root
export VLLM_BACKEND_URL=http://localhost:8000
export VLLM_MODEL_NAME=meta-llama/Llama-2-7b-chat-hf

./test-api-inference.sh e2e http://localhost:8000 meta-llama/Llama-2-7b-chat-hf
```

## Step 5: Configure API Router

Once vLLM is running, configure the API Router to route to it:

```yaml
# services/api-router-service/configs/router.yaml
backends:
  - id: vllm-rtx4000-llama7b
    model_variant: gpt-4o  # Alias for client requests
    uri: http://llama-7b-dev.system.svc.cluster.local:8000/v1/chat/completions
    health_check_interval: 30s
    timeout: 60s
```

```yaml
# services/api-router-service/configs/policies.yaml
models:
  - model: gpt-4o
    organization_id: "*"
    backends:
      - backend_id: vllm-rtx4000-llama7b
        weight: 100
```

## Performance Expectations

### RTX 4000 Ada Performance

| Model | Prompt Tokens/sec | Generation Tokens/sec | Latency (1st token) |
|-------|-------------------|------------------------|---------------------|
| Llama-2-7B | 1000+ | 30-40 | ~100ms |
| Mistral-7B | 1200+ | 35-45 | ~80ms |
| Llama-2-13B | 600+ | 15-25 | ~150ms |

### Concurrent Users

| Model | Concurrent Requests | Tokens/sec per user |
|-------|---------------------|---------------------|
| 7B | 10-20 | 20-30 |
| 13B | 5-10 | 10-15 |

## Troubleshooting

### Pod stuck in Pending

```bash
# Check events
kubectl describe pod -n system <pod-name>

# Common issues:
# 1. No GPU nodes available
kubectl get nodes -l node-type=gpu

# 2. GPU already allocated
kubectl describe node <gpu-node> | grep nvidia.com/gpu
```

### Out of Memory (OOM)

If you see OOM errors:

1. **Reduce GPU memory utilization**:
```bash
--set vllm.env[0].value="0.75"  # Use 75% instead of 85%
```

2. **Reduce context length**:
```bash
--set vllm.env[3].value="2048"  # 2K instead of 4K
```

3. **Try a smaller model**:
```bash
# Use 7B instead of 13B
--set model.path=meta-llama/Llama-2-7b-chat-hf
```

### Slow inference

1. **Check GPU utilization**:
```bash
kubectl exec -n system <pod-name> -- nvidia-smi
```

2. **Increase batch size** (in vLLM settings)
3. **Reduce max_tokens** in requests

## Cost Analysis

### Cloud Provider Costs (Monthly)

| Provider | Instance Type | GPU | vCPUs | RAM | Cost/Month* |
|----------|--------------|-----|-------|-----|-------------|
| GCP | n1-standard-4 + RTX 4000 Ada | 1 | 4 | 15GB | ~$250-350 |
| AWS | g5.xlarge | A10G (24GB) | 4 | 16GB | $450-550 |
| Azure | NCasT4_v3 | T4 (16GB) | 4 | 28GB | $300-400 |

*Prices vary by region and commitment level

### On-Premises Cost

| Component | Cost |
|-----------|------|
| RTX 4000 Ada GPU | $1,000-1,500 |
| Server (one-time) | $2,000-3,000 |
| Electricity (yearly) | $100-200 |
| **Break-even** | **4-6 months** vs cloud |

## Summary

The RTX 4000 Ada (20GB) is **perfect** for:
- ✅ Development and testing
- ✅ 7B-13B models
- ✅ Low-to-medium traffic
- ✅ Cost-efficient single-GPU setup

**Next Steps**:
1. Add GPU node to cluster
2. Deploy vLLM with `values-rtx4000ada.yaml`
3. Test with E2E test: `./test-api-inference.sh e2e`
4. Configure API Router
5. Monitor and optimize
