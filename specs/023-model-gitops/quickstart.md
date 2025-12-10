# AI Model Operator Quickstart

**5-minute guide to deploying your first AI model with the AI Model Operator**

## Prerequisites

- Kubernetes cluster with GPU nodes
- AI Model Operator installed (`kubectl get crd aimodels.aimodel.ai-aas.io`)
- S3 bucket for model storage
- S3 credentials configured

## Step 1: Create Secrets

```bash
# Create S3 credentials (required)
kubectl create secret generic s3-credentials \
  --from-literal=access-key-id=<YOUR_ACCESS_KEY> \
  --from-literal=secret-access-key=<YOUR_SECRET_KEY>

# Create HuggingFace token (optional, for private models)
kubectl create secret generic hf-credentials \
  --from-literal=token=<YOUR_HF_TOKEN>
```

## Step 2: Deploy a Model

Create a minimal `AIModel` resource:

```yaml
# gpt2-quickstart.yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: gpt2-quickstart
  namespace: default
spec:
  modelName: "GPT-2"
  modelID: "gpt2"
  s3Bucket: "ai-aas-models-dev"
  s3Key: "models/gpt2"
  enabled: true
```

Apply it:

```bash
kubectl apply -f gpt2-quickstart.yaml
```

## Step 3: Watch Deployment

```bash
# Watch AIModel status
kubectl get aimodel gpt2-quickstart -w

# Expected progression:
# NAME              MODEL   RUNTIME   ENABLED   READY   PHASE          AGE
# gpt2-quickstart   GPT-2   vllm      true      0       Pending        0s
# gpt2-quickstart   GPT-2   vllm      true      0       Downloading    5s
# gpt2-quickstart   GPT-2   vllm      true      0       Deploying      2m
# gpt2-quickstart   GPT-2   vllm      true      1       Ready          3m
```

## Step 4: Test the Model

```bash
# Port-forward to the InferenceService
kubectl port-forward -n default \
  svc/gpt2-quickstart-predictor-default-private 8080:80

# Send a test request (in another terminal)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "GPT-2",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'
```

You should see a response with generated text!

## Advanced Examples

### Production Deployment with Autoscaling

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: mistral-7b
  namespace: production
spec:
  # Model configuration
  modelName: "Mistral-7B-Instruct"
  modelID: "mistralai/Mistral-7B-Instruct-v0.2"
  s3Bucket: "ai-aas-models-prod"
  s3Key: "models/mistral-7b-instruct"

  enabled: true
  runtime: vllm

  # Autoscaling
  minReplicas: 1     # Keep 1 replica warm
  maxReplicas: 5     # Scale up to 5 under load

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

  # Target specific GPU hardware
  nodeSelector:
    accelerator: nvidia-l4

  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule

  # vLLM optimization
  runtimeArgs:
    - --max-model-len=4096
    - --gpu-memory-utilization=0.9
```

### Scale-to-Zero Development Setup

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: dev-model
  namespace: development
spec:
  modelName: "Dev-Model"
  modelID: "gpt2"
  s3Bucket: "ai-aas-models-dev"
  s3Key: "models/dev-model"

  enabled: true
  runtime: vllm

  # Scale to zero when idle (cost savings)
  minReplicas: 0
  maxReplicas: 1

  # Minimal resources
  resources:
    requests:
      cpu: "2"
      memory: "4Gi"
      nvidia.com/gpu: "1"
```

### Multi-GPU Large Model

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3-70b
  namespace: production
spec:
  modelName: "Llama-3-70B"
  modelID: "meta-llama/Meta-Llama-3-70B"
  s3Bucket: "ai-aas-models-prod"
  s3Key: "models/llama-3-70b"

  enabled: true
  runtime: vllm

  minReplicas: 1
  maxReplicas: 2

  # 4x A100 GPUs for 70B model
  resources:
    requests:
      cpu: "16"
      memory: "128Gi"
      nvidia.com/gpu: "4"
    limits:
      cpu: "32"
      memory: "256Gi"
      nvidia.com/gpu: "4"

  nodeSelector:
    accelerator: nvidia-a100

  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule

  # Tensor parallelism for multi-GPU
  runtimeArgs:
    - --max-model-len=8192
    - --gpu-memory-utilization=0.95
    - --tensor-parallel-size=4
    - --trust-remote-code
```

## GitOps Workflow

For production deployments, use GitOps:

```bash
# 1. Add AIModel manifest to GitOps repo
mkdir -p gitops/clusters/development/apps/ai-models
cp gpt2-quickstart.yaml gitops/clusters/development/apps/ai-models/

# 2. Commit and push
git add gitops/clusters/development/apps/ai-models/gpt2-quickstart.yaml
git commit -m "feat: Deploy GPT-2 model"
git push origin develop

# 3. ArgoCD will automatically sync (if auto-sync enabled)
# Or manually sync:
argocd app sync ai-models
```

## Common Operations

### Scale Down (Disable)

Temporarily stop the model without deleting configuration:

```bash
kubectl patch aimodel gpt2-quickstart --type=merge \
  -p '{"spec":{"enabled":false}}'
```

### Scale Up (Re-enable)

```bash
kubectl patch aimodel gpt2-quickstart --type=merge \
  -p '{"spec":{"enabled":true}}'
```

### Update Configuration

```bash
# Increase context length
kubectl patch aimodel gpt2-quickstart --type=merge \
  -p '{"spec":{"runtimeArgs":["--max-model-len=8192"]}}'

# Adjust autoscaling
kubectl patch aimodel gpt2-quickstart --type=merge \
  -p '{"spec":{"minReplicas":2,"maxReplicas":10}}'
```

### Delete Model

```bash
kubectl delete aimodel gpt2-quickstart
```

## Troubleshooting

### Check Status

```bash
# View status
kubectl get aimodel gpt2-quickstart

# Detailed information
kubectl describe aimodel gpt2-quickstart

# Get status message
kubectl get aimodel gpt2-quickstart -o jsonpath='{.status.message}'
```

### Check Logs

```bash
# Operator logs
kubectl logs -n ai-model-system -l app=ai-model-operator

# Download job logs
kubectl logs job/gpt2-quickstart-downloader

# InferenceService logs
kubectl logs -l serving.kserve.io/inferenceservice=gpt2-quickstart
```

### Common Issues

| Issue | Solution |
|-------|----------|
| Stuck in Downloading | Check HuggingFace token and S3 credentials |
| Stuck in Deploying | Check GPU availability and node resources |
| Pod CrashLooping | Check GPU memory vs model size |
| Secrets not found | Ensure secrets are in the same namespace |

## Next Steps

- Read the [full operator guide](../../docs/platform/ai-model-operator-guide.md) for detailed configuration
- Explore the [PRD](./spec.md) for architecture and design decisions
- Check the [operator README](../../operators/ai-model-operator/README.md) for development

## Quick Reference

### AIModel Spec Fields

| Field | Required | Default | Description |
|-------|----------|---------|-------------|
| `modelName` | Yes | - | Display name for the model |
| `modelID` | Yes | - | HuggingFace model ID |
| `s3Bucket` | Yes | - | S3 bucket for caching |
| `s3Key` | Yes | - | S3 path prefix |
| `enabled` | No | true | Deploy or disable |
| `runtime` | No | vllm | Runtime: vllm, triton, tgi |
| `minReplicas` | No | 0 | Min replicas (0 = scale-to-zero) |
| `maxReplicas` | No | 1 | Max replicas for autoscaling |
| `resources` | No | - | CPU/memory/GPU requests/limits |
| `nodeSelector` | No | - | Node labels for scheduling |
| `tolerations` | No | - | Tolerations for taints |
| `runtimeArgs` | No | - | Additional CLI args |
| `runtimeEnv` | No | - | Additional env vars |

### Status Phases

- **Pending**: Initial state
- **Downloading**: Downloading from HuggingFace to S3
- **Deploying**: Creating KServe InferenceService
- **Ready**: Model serving requests
- **Failed**: Error occurred
- **Disabled**: Model scaled to zero
