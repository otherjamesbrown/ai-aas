# AI Model Operator User Guide

This guide walks you through deploying AI models using the GitOps-based AI Model Operator.

## Overview

The AI Model Operator automates the lifecycle of AI model deployments:

1. **You commit** an `AIModel` manifest to the GitOps repository
2. **ArgoCD syncs** the manifest to Kubernetes
3. **Operator downloads** model from HuggingFace to S3 (if not cached)
4. **Operator deploys** vLLM inference server loading from S3
5. **Model is ready** to serve inference requests

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Git Repo  │────▶│   ArgoCD    │────▶│  Operator   │────▶│    vLLM     │
│  (AIModel)  │     │   (sync)    │     │ (reconcile) │     │  (serving)  │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
                                              │
                                              ▼
                                        ┌─────────────┐
                                        │     S3      │
                                        │   (cache)   │
                                        └─────────────┘
```

## Prerequisites

Before deploying models, ensure you have:

- [ ] Kubernetes cluster with GPU nodes
- [ ] ArgoCD installed and configured
- [ ] S3-compatible object storage (AWS S3, MinIO, Linode Object Storage)
- [ ] `kubectl` access to the cluster
- [ ] HuggingFace account (for private models)

## Step 1: Deploy the AI Model Operator

### 1.1 Install the Operator via ArgoCD

The operator should be deployed via GitOps. Create an ArgoCD Application:

```yaml
# gitops/clusters/development/apps/ai-model-operator.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: ai-model-operator
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: develop  # or main for production
    path: operators/ai-model-operator/deployments/helm/ai-model-operator
  destination:
    server: https://kubernetes.default.svc
    namespace: ai-model-system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

### 1.2 Verify Operator is Running

```bash
# Check operator pod
kubectl get pods -n ai-model-system

# Expected output:
# NAME                                  READY   STATUS    RESTARTS   AGE
# ai-model-operator-xxxxxxxxx-xxxxx    1/1     Running   0          1m

# Check CRD is installed
kubectl get crd aimodels.aimodel.ai-aas.io
```

## Step 2: Create Required Secrets

The operator requires secrets for S3 access and optionally HuggingFace access.

### 2.1 Create S3 Credentials Secret (Required)

Create this secret in the namespace where you'll deploy AIModel resources:

```bash
# For AWS S3
kubectl create secret generic s3-credentials \
  --namespace=default \
  --from-literal=access-key-id=<YOUR_AWS_ACCESS_KEY_ID> \
  --from-literal=secret-access-key=<YOUR_AWS_SECRET_ACCESS_KEY>

# Verify
kubectl get secret s3-credentials -n default
```

**For Linode Object Storage:**
```bash
kubectl create secret generic s3-credentials \
  --namespace=default \
  --from-literal=access-key-id=<LINODE_ACCESS_KEY> \
  --from-literal=secret-access-key=<LINODE_SECRET_KEY>
```

> **Note**: The secret must be named `s3-credentials` and contain keys `access-key-id` and `secret-access-key`.

### 2.2 Create HuggingFace Token Secret (Optional)

Only required for private/gated models (e.g., Llama 2, Mistral):

```bash
kubectl create secret generic hf-credentials \
  --namespace=default \
  --from-literal=token=<YOUR_HUGGINGFACE_TOKEN>

# Verify
kubectl get secret hf-credentials -n default
```

**To get a HuggingFace token:**
1. Go to https://huggingface.co/settings/tokens
2. Create a new token with "Read" permissions
3. For gated models, ensure you've accepted the model's license on HuggingFace

> **Note**: The secret must be named `hf-credentials` with key `token`. This secret is optional - public models don't require it.

### 2.3 Verify Secrets

```bash
# List secrets
kubectl get secrets -n default | grep -E "s3-credentials|hf-credentials"

# Expected:
# hf-credentials    Opaque   1      1m
# s3-credentials    Opaque   2      1m
```

## Step 3: Provision S3 Bucket

Create an S3 bucket to store model artifacts. This serves as a cache to avoid re-downloading from HuggingFace.

### 3.1 AWS S3

```bash
aws s3 mb s3://ai-aas-models-dev --region us-east-1
```

### 3.2 Linode Object Storage

```bash
# Using Linode CLI
linode-cli obj mb ai-aas-models-dev --cluster us-east-1

# Or via AWS CLI with Linode endpoint
aws s3 mb s3://ai-aas-models-dev \
  --endpoint-url https://us-east-1.linodeobjects.com
```

### 3.3 Verify Bucket Access

```bash
# Test write access
echo "test" | aws s3 cp - s3://ai-aas-models-dev/test.txt

# Test read access
aws s3 ls s3://ai-aas-models-dev/

# Clean up test file
aws s3 rm s3://ai-aas-models-dev/test.txt
```

## Step 4: Create an AIModel Resource

### 4.1 AIModel CRD Reference

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: <model-name>           # Kubernetes resource name
  namespace: <namespace>       # Where to deploy (must have secrets)
spec:
  modelName: <display-name>    # Human-readable name for the model
  modelID: <huggingface-id>    # HuggingFace model ID (e.g., "mistralai/Mistral-7B-Instruct-v0.2")
  s3Bucket: <bucket-name>      # S3 bucket for model cache
  s3Key: <path/to/model>       # S3 prefix for this model's artifacts
  replicas: 1                  # Number of vLLM replicas (optional, default: 1)
  enabled: true                # Set to false to scale down without deleting config
```

### 4.2 Example: Deploy Mistral-7B

Create the manifest:

```yaml
# gitops/clusters/development/apps/ai-models/mistral-7b.yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: mistral-7b-instruct
  namespace: default
spec:
  modelName: "Mistral-7B-Instruct"
  modelID: "mistralai/Mistral-7B-Instruct-v0.2"
  s3Bucket: "ai-aas-models-dev"
  s3Key: "models/mistral-7b-instruct-v0.2"
  replicas: 1
  enabled: true
```

### 4.3 Apply Directly (for testing)

```bash
kubectl apply -f mistral-7b.yaml
```

### 4.4 Or Commit to GitOps Repository (recommended)

```bash
# Add to GitOps repo
cp mistral-7b.yaml gitops/clusters/development/apps/ai-models/

# Commit and push
git add gitops/clusters/development/apps/ai-models/mistral-7b.yaml
git commit -m "feat: Deploy Mistral-7B-Instruct model"
git push origin develop

# ArgoCD will automatically sync
```

## Step 5: Monitor Deployment

### 5.1 Watch AIModel Status

```bash
# Watch status changes
kubectl get aimodel mistral-7b-instruct -w

# Expected progression:
# NAME                  MODEL NAME           ENABLED   REPLICAS   PHASE          AGE
# mistral-7b-instruct   Mistral-7B-Instruct  true      1          Pending        0s
# mistral-7b-instruct   Mistral-7B-Instruct  true      1          Downloading    5s
# mistral-7b-instruct   Mistral-7B-Instruct  true      1          Downloaded     10m
# mistral-7b-instruct   Mistral-7B-Instruct  true      1          Deploying      10m
# mistral-7b-instruct   Mistral-7B-Instruct  true      1          Ready          12m
```

### 5.2 Check Detailed Status

```bash
kubectl describe aimodel mistral-7b-instruct
```

### 5.3 Monitor Downloader Job

```bash
# Check job status
kubectl get jobs | grep mistral-7b-instruct

# Watch job logs (model download progress)
kubectl logs -f job/mistral-7b-instruct-downloader
```

### 5.4 Monitor vLLM Deployment

```bash
# Check deployment
kubectl get deployment mistral-7b-instruct-vllm

# Check pods
kubectl get pods -l aimodel_cr=mistral-7b-instruct

# View vLLM logs
kubectl logs -f deployment/mistral-7b-instruct-vllm
```

## Step 6: Test the Model

### 6.1 Get the Service Endpoint

```bash
# Get service
kubectl get svc mistral-7b-instruct-vllm-svc

# Port-forward for local testing
kubectl port-forward svc/mistral-7b-instruct-vllm-svc 8000:8000
```

### 6.2 Send a Test Request

```bash
curl http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Mistral-7B-Instruct",
    "messages": [{"role": "user", "content": "Hello, how are you?"}],
    "max_tokens": 100
  }'
```

### 6.3 Check Model Info

```bash
curl http://localhost:8000/v1/models
```

## Step 7: Manage Model Lifecycle

### 7.1 Scale Down (Disable)

To stop the model without deleting the configuration:

```bash
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"enabled":false}}'
```

This will:
- Delete the vLLM Deployment (stops GPU usage)
- Keep the AIModel resource (configuration preserved)
- Keep S3 artifacts (no re-download needed)

### 7.2 Scale Up (Re-enable)

```bash
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"enabled":true}}'
```

### 7.3 Change Replica Count

```bash
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"replicas":2}}'
```

### 7.4 Delete Model Completely

```bash
kubectl delete aimodel mistral-7b-instruct
```

This will:
- Delete the vLLM Deployment and Service
- Delete the Downloader Job
- Keep S3 artifacts (manual cleanup required)

## Troubleshooting

### Phase: ArtifactMissing

The operator checks if artifacts exist in S3 before creating the downloader job. This phase means either:
- First deployment: Normal, will proceed to create downloader job
- S3 credentials incorrect: Check `s3-credentials` secret

```bash
# Verify S3 access
kubectl get secret s3-credentials -o yaml
aws s3 ls s3://<bucket>/<key>/
```

### Phase: Downloading (stuck)

Check the downloader job logs:

```bash
kubectl logs job/mistral-7b-instruct-downloader

# Common issues:
# - HuggingFace rate limiting (wait and retry)
# - Invalid HF token for gated model
# - S3 write permission denied
```

### Phase: Failed

Check both job and operator logs:

```bash
# Job logs
kubectl logs job/mistral-7b-instruct-downloader

# Operator logs
kubectl logs -n ai-model-system deployment/ai-model-operator
```

### vLLM Pod CrashLooping

Usually means GPU memory issues:

```bash
kubectl logs deployment/mistral-7b-instruct-vllm

# Common issues:
# - Model too large for GPU (try smaller model or larger GPU)
# - S3 download failed (check credentials)
# - GPU not available (check node taints/tolerations)
```

### Secrets Not Found

Ensure secrets are in the same namespace as the AIModel:

```bash
# Check namespace
kubectl get aimodel mistral-7b-instruct -o jsonpath='{.metadata.namespace}'

# Secrets must be in that namespace
kubectl get secrets -n <namespace> | grep -E "s3-credentials|hf-credentials"
```

## Reference

### AIModel Status Phases

| Phase | Description |
|-------|-------------|
| `Pending` | Initial state, waiting for reconciliation |
| `ArtifactMissing` | S3 artifacts not found, will create downloader job |
| `Downloading` | Downloader job running (HuggingFace → S3) |
| `Downloaded` | Model artifacts in S3, ready for deployment |
| `Deploying` | vLLM Deployment created, waiting for pods |
| `Ready` | vLLM pods running, model serving requests |
| `Failed` | Error occurred (check logs) |
| `Disabled` | Model disabled via `enabled: false` |

### Required Secrets

| Secret Name | Keys | Required | Description |
|-------------|------|----------|-------------|
| `s3-credentials` | `access-key-id`, `secret-access-key` | Yes | S3/Object storage credentials |
| `hf-credentials` | `token` | No | HuggingFace token for private models |

### Environment Variables (vLLM Pod)

The operator automatically configures these on the vLLM deployment:

| Variable | Source | Description |
|----------|--------|-------------|
| `AWS_ACCESS_KEY_ID` | `s3-credentials` secret | S3 authentication |
| `AWS_SECRET_ACCESS_KEY` | `s3-credentials` secret | S3 authentication |

### vLLM Configuration

The operator deploys vLLM with these default arguments:

```
--model s3://<bucket>/<key>
--served-model-name <modelName>
--dtype auto
--max-model-len 4096
--gpu-memory-utilization 0.9
--trust-remote-code
```

### GPU Resources

Default GPU configuration:
- Requests: 1 x `nvidia.com/gpu`
- Limits: 1 x `nvidia.com/gpu`
- Tolerations: `nvidia.com/gpu`, `gpu-workload=true`
