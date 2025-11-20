# GPU Deployment Quickstart for Linode LKE

**Status Date**: 2025-11-19
**Cluster**: lke531921 (fr-par-2)
**Current State**: 3 non-GPU nodes, 0 GPU nodes, 0 models deployed

## Overview

This guide walks you through adding a GPU node to your Linode LKE cluster and deploying your first vLLM model.

## Prerequisites

- Access to Linode Cloud Console
- kubectl configured (✓ already working)
- Kubeconfig at `~/kubeconfigs/kubeconfig-development.yaml` (✓ configured)

## Step 1: Add GPU Node Pool via Linode Console

### 1.1 Access Your Cluster

Go to: https://cloud.linode.com/kubernetes/clusters/lke531921

### 1.2 Add GPU Node Pool

1. Click **"Add a Node Pool"**
2. Select **"Dedicated GPU"** tab
3. Choose **"RTX 6000 (24GB)"** - This is better than RTX 4000 Ada!
4. Set count: **1** (for cost efficiency)
5. Click **"Add Pool"**

**Expected Cost**: ~$300-400/month for RTX 6000 x1

**GPU Specifications**:
- Model: NVIDIA RTX 6000
- Memory: 24GB GDDR6
- Perfect for: 7B-13B models
- Concurrent users: 10-20 (7B model)

### 1.3 Wait for Node Provisioning (3-5 minutes)

Monitor node creation:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
watch kubectl get nodes
```

You should see a 4th node appear with a name like `lke531921-XXXXXX-XXXXXXXXXXXX`.

## Step 2: Configure GPU Node

### 2.1 Identify the GPU Node

Once the new node appears:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# List all nodes
kubectl get nodes

# The newest node is your GPU node (look at AGE column)
```

### 2.2 Label the GPU Node

```bash
# Replace <gpu-node-name> with the actual node name
GPU_NODE=$(kubectl get nodes --sort-by=.metadata.creationTimestamp | tail -1 | awk '{print $1}')

# Label the node
kubectl label nodes $GPU_NODE node-type=gpu

# Verify label
kubectl get nodes -l node-type=gpu
```

### 2.3 Taint the GPU Node

This ensures only GPU workloads run on this expensive node:

```bash
kubectl taint nodes $GPU_NODE gpu-workload=true:NoSchedule

# Verify taint
kubectl describe node $GPU_NODE | grep Taints
```

## Step 3: Install NVIDIA Device Plugin

The NVIDIA device plugin makes GPUs available to Kubernetes pods.

### 3.1 Install the Plugin

```bash
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
```

### 3.2 Verify GPU is Available (wait 30 seconds)

```bash
# Check if nvidia.com/gpu appears in allocatable resources
kubectl describe node $GPU_NODE | grep nvidia.com/gpu

# You should see:
#   nvidia.com/gpu: 1
```

Expected output:
```
Allocatable:
  cpu:                8
  ephemeral-storage:  49255941901
  hugepages-1Gi:      0
  hugepages-2Mi:      0
  memory:             32946516Ki
  nvidia.com/gpu:     1    <-- This confirms GPU is ready!
  pods:               110
```

## Step 4: Deploy Your First Model

### 4.1 Choose a Model

**Recommended for first deployment**: Llama-2-7B-chat
- Size: 7B parameters (~14GB GPU memory)
- Download: ~13GB, takes 5-15 minutes
- Performance: 30-40 tokens/sec
- Good for: Testing, development, low-medium traffic

**Alternative models**:
- `mistralai/Mistral-7B-Instruct-v0.2` - Faster, better quality
- `meta-llama/Llama-2-13b-chat-hf` - Higher quality, slower
- `TinyLlama/TinyLlama-1.1B-Chat-v1.0` - Ultra-fast for testing

### 4.2 Deploy vLLM with Automated Script

The deployment script handles everything automatically:

```bash
cd /home/dev/ai-aas

# Deploy Llama-2-7B (recommended)
./scripts/deploy-vllm-linode.sh meta-llama/Llama-2-7b-chat-hf

# Or deploy Mistral-7B
./scripts/deploy-vllm-linode.sh mistralai/Mistral-7B-Instruct-v0.2
```

**What the script does**:
1. ✓ Verifies GPU node is ready
2. ✓ Checks NVIDIA device plugin
3. ✓ Deploys vLLM with optimized RTX 6000 configuration
4. ✓ Waits for model to download from HuggingFace (~5-15 min)
5. ✓ Tests health and inference endpoints
6. ✓ Provides next steps

**Expected timeline**:
- Pre-flight checks: 30 seconds
- Model download: 5-15 minutes (first time only)
- Model loading: 2-3 minutes
- Total: 10-20 minutes for first deployment

### 4.3 Monitor Deployment

The script monitors automatically, but you can also check manually:

```bash
# Watch pod status
kubectl get pods -n system -w

# Check logs (in another terminal)
kubectl logs -n system -l app.kubernetes.io/name=vllm-deployment -f
```

**Expected log messages**:
```
INFO: Downloading model from HuggingFace...
INFO: Loading model weights...
INFO: Initializing vLLM engine...
INFO: Server started at http://0.0.0.0:8000
```

## Step 5: Test the Deployment

### 5.1 Get Service Name

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get svc -n system

# Note the service name (e.g., llama-2-7b-chat-hf)
```

### 5.2 Run E2E Test

This tests the actual "Capital of France" → "Paris" inference:

```bash
cd /home/dev/ai-aas

# Test with cluster port-forward (automatic)
./test-api-inference.sh cluster system <service-name> meta-llama/Llama-2-7b-chat-hf
```

**Expected output**:
```
[SUCCESS] ✓ vLLM backend is healthy
[SUCCESS] ✓ E2E tests passed!
[SUCCESS] ✓ The 'capital of France' question returned 'Paris'
```

### 5.3 Manual Test with curl

```bash
# Port-forward to service
kubectl port-forward -n system svc/<service-name> 8000:8000 &

# Test inference
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

**Expected response**:
```json
{
  "choices": [
    {
      "message": {
        "content": "Paris"
      }
    }
  ],
  "usage": {
    "prompt_tokens": 18,
    "completion_tokens": 1,
    "total_tokens": 19
  }
}
```

## Step 6: Verify Model Deployment

### 6.1 Check Deployment Status

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Check deployments
kubectl get deployments -n system

# Check pods
kubectl get pods -n system

# Should show:
# NAME                                   READY   STATUS    RESTARTS   AGE
# llama-2-7b-chat-hf-vllm-XXXXX-XXXXX    1/1     Running   0          5m
```

### 6.2 Check GPU Utilization

```bash
# Get pod name
POD_NAME=$(kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment -o jsonpath='{.items[0].metadata.name}')

# Check GPU usage
kubectl exec -n system $POD_NAME -- nvidia-smi
```

Expected output:
```
+-----------------------------------------------------------------------------+
| NVIDIA-SMI 535.xx.xx              Driver Version: 535.xx.xx    CUDA Version: 12.2 |
|-------------------------------+----------------------+----------------------+
| GPU  Name        Persistence-M| Bus-Id        Disp.A | Volatile Uncorr. ECC |
| Fan  Temp  Perf  Pwr:Usage/Cap|         Memory-Usage | GPU-Util  Compute M. |
|===============================+======================+======================|
|   0  RTX 6000              On   | 00000000:00:05.0 Off |                    0 |
| 30%   45C    P2    75W / 300W |  14000MiB / 24576MiB |     25%      Default |
+-------------------------------+----------------------+----------------------+
```

## Model Seeding Details

### How Model Seeding Works

Models are **automatically downloaded** from HuggingFace on first run:

1. **vLLM container starts** → Checks if model exists locally
2. **Model not found** → Downloads from HuggingFace Hub
3. **Downloads to pod storage** → Cached at `/root/.cache/huggingface/`
4. **Model loads into GPU memory** → Ready for inference
5. **Subsequent restarts** → Uses cached model (fast!)

### Download Sizes and Times

| Model | Download Size | Download Time | GPU Memory |
|-------|---------------|---------------|------------|
| Llama-2-7B | ~13GB | 5-15 min | ~14GB |
| Mistral-7B | ~14GB | 5-15 min | ~14GB |
| Llama-2-13B | ~25GB | 10-20 min | ~18GB |

**Factors affecting download time**:
- Internet connection speed
- HuggingFace API rate limits
- Time of day (peak hours slower)

### Model Storage

- **Location**: Pod ephemeral storage at `/root/.cache/huggingface/`
- **Persistence**: Models persist while pod is running
- **Pod restart**: Model is cached, loads in 2-3 minutes
- **Pod delete**: Model must be re-downloaded (~10-15 min)

**Important**: If you delete the pod/deployment, the model cache is lost and must be re-downloaded.

## Troubleshooting

### GPU Node Not Appearing

**Symptom**: No 4th node after 5 minutes

**Fix**:
1. Check Linode console for errors
2. Verify node pool creation succeeded
3. Check cluster events: `kubectl get events -n kube-system`

### GPU Not Allocatable

**Symptom**: `nvidia.com/gpu: 0` or not shown

**Fix**:
1. Verify NVIDIA device plugin is running:
   ```bash
   kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset
   ```
2. Check plugin logs:
   ```bash
   kubectl logs -n kube-system -l name=nvidia-device-plugin-ds
   ```
3. Restart plugin:
   ```bash
   kubectl delete pod -n kube-system -l name=nvidia-device-plugin-ds
   ```

### Pod Stuck in Pending

**Symptom**: vLLM pod shows `Pending` status

**Fix**:
1. Check if GPU is available:
   ```bash
   kubectl describe node $GPU_NODE | grep nvidia.com/gpu
   ```
2. Check pod events:
   ```bash
   kubectl describe pod -n system <pod-name>
   ```
3. Common issues:
   - GPU not labeled: Add `node-type=gpu` label
   - GPU already allocated: Only 1 GPU available
   - Missing toleration: Check Helm values include GPU taint

### Model Download Slow/Stuck

**Symptom**: Pod stuck in `ContainerCreating` for >20 minutes

**Fix**:
1. Check pod logs:
   ```bash
   kubectl logs -n system <pod-name> -f
   ```
2. Look for download progress:
   - Should see: `Downloading (pytorch_model.bin): 25%`
3. If stuck:
   - Wait 30 minutes (large models take time)
   - Check HuggingFace status: https://status.huggingface.co/
   - Delete pod and retry: `kubectl delete pod -n system <pod-name>`

### Out of Memory (OOM)

**Symptom**: Pod crashes with OOM error

**Fix**:
1. Reduce GPU memory utilization:
   ```bash
   helm upgrade <release-name> \
     infra/helm/charts/vllm-deployment \
     -f infra/helm/charts/vllm-deployment/values-rtx4000ada.yaml \
     --set vllm.env[0].value="0.75" \
     -n system
   ```
2. Try smaller model:
   - Llama-2-7B instead of Llama-2-13B
3. Reduce max context length:
   ```bash
   --set vllm.env[3].value="2048"
   ```

## Next Steps After Successful Deployment

1. **Configure API Router** to route to this backend:
   - Edit `services/api-router-service/configs/router.yaml`
   - Add backend pointing to `<service-name>.system.svc.cluster.local:8000`

2. **Deploy API Router** to cluster

3. **Run full E2E test** through API Router

4. **Set up monitoring**:
   - Prometheus metrics
   - GPU utilization dashboards
   - Request latency tracking

5. **Consider cost optimizations**:
   - Auto-scaling (scale to 0 when idle)
   - Spot instances (70% cheaper)
   - Scheduled shutdowns (nights/weekends)

## Summary of Commands

```bash
# 1. Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# 2. After adding GPU node via console, label it
GPU_NODE=$(kubectl get nodes --sort-by=.metadata.creationTimestamp | tail -1 | awk '{print $1}')
kubectl label nodes $GPU_NODE node-type=gpu
kubectl taint nodes $GPU_NODE gpu-workload=true:NoSchedule

# 3. Install NVIDIA device plugin
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml

# 4. Deploy vLLM with model
cd /home/dev/ai-aas
./scripts/deploy-vllm-linode.sh meta-llama/Llama-2-7b-chat-hf

# 5. Test deployment
./test-api-inference.sh cluster system <service-name> meta-llama/Llama-2-7b-chat-hf
```

## Current Status

- ✓ Cluster connected (lke531921)
- ✓ kubectl configured
- ✓ Deployment scripts ready
- ✓ Test scripts ready
- ⏳ **Waiting for**: GPU node pool to be added via Linode Console
- ⏳ **Next action**: Add GPU node pool → Label/taint → Deploy model

**Estimated time to first inference**: 15-25 minutes after adding GPU node

## Questions?

- GPU specifications: See `/home/dev/ai-aas/docs/RTX4000_ADA_DEPLOYMENT.md`
- Deployment details: See `/home/dev/ai-aas/scripts/deploy-vllm-linode.sh`
- Testing guide: See `/home/dev/ai-aas/services/api-router-service/test/integration/README.md`
