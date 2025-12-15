# AI Model Operator User Guide

This guide walks you through deploying AI models using the GitOps-based AI Model Operator.

## Overview

The AI Model Operator automates the lifecycle of AI model deployments:

1. **You commit** an `AIModel` manifest to the GitOps repository
2. **ArgoCD syncs** the manifest to Kubernetes
3. **Operator downloads** model from HuggingFace to S3 (if not cached)
4. **Operator creates** KServe InferenceService with the specified runtime (vLLM, Triton, or TGI)
5. **KServe deploys** the inference runtime with autoscaling support
6. **Model is ready** to serve inference requests

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Git Repo  │────▶│   ArgoCD    │────▶│  Operator   │────▶│   KServe    │────▶│    vLLM     │
│  (AIModel)  │     │   (sync)    │     │ (reconcile) │     │ (InfSvc)    │     │  (serving)  │
└─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘     └─────────────┘
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

The `AIModel` CRD supports flexible configuration for different deployment scenarios. The operator supports two deployment modes:

### Deployment Modes

#### 1. S3-Based Deployment (Default)

Models are downloaded from HuggingFace, cached in S3, and loaded via KServe's storage initializer.

**Use when:**
- Standard models with built-in architecture support
- Production deployments (predictable startup)
- Air-gapped or offline environments
- Cost optimization (download once, reuse many times)

**Required fields:** `modelName`, `modelID`, `s3Bucket`, `s3Key`

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
spec:
  modelName: llama-7b
  modelID: meta-llama/Llama-2-7b-hf
  s3Bucket: ai-aas-models
  s3Key: llama-2-7b
```

#### 2. HuggingFace-Direct Deployment (TrustRemoteCode)

Models are downloaded directly from HuggingFace at container startup. Required for models with custom architectures.

**Use when (REQUIRED for):**
- Models with custom architectures (e.g., GPT-OSS, custom LLMs)
- Models that need `trust_remote_code=True`
- Models with custom tokenizers or processors
- Rapid development/testing

**Required fields:** `modelName`, `modelID`, `trustRemoteCode: true`

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
spec:
  modelName: gpt-oss-20b
  modelID: openai/gpt-oss-20b
  trustRemoteCode: true
  # s3Bucket and s3Key are optional with trustRemoteCode
```

**Why trustRemoteCode needs direct HuggingFace access:**

When a model uses custom architecture code, vLLM needs to:
1. Download Python files from HuggingFace (e.g., `modeling_*.py`)
2. Execute that code to define the model class
3. Load weights into the custom architecture

S3-based deployment only caches model weights, not the HuggingFace repo structure and Python files that custom architectures require.

### Comparison

| Aspect | S3-Based | HuggingFace-Direct |
|--------|----------|-------------------|
| Startup time | Fast (seconds) | Slow (minutes) |
| Custom architectures | Not supported | Supported |
| Offline operation | Yes | No |
| Storage costs | S3 storage | None |
| Model updates | Re-download needed | Automatic |

### AIModel Spec Reference

Below is a comprehensive reference of all available fields:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: <model-name>           # Kubernetes resource name
  namespace: <namespace>       # Where to deploy (must have secrets)
spec:
  # Required fields
  modelName: <display-name>    # Human-readable name for the model
  modelID: <huggingface-id>    # HuggingFace model ID (e.g., "mistralai/Mistral-7B-Instruct-v0.2")

  # S3 fields (required for S3-based deployment, optional with trustRemoteCode)
  s3Bucket: <bucket-name>      # S3 bucket for model cache
  s3Key: <path/to/model>       # S3 prefix for this model's artifacts

  # Custom architecture support
  trustRemoteCode: false       # Set to true for models with custom architectures (default: false)

  # Deployment control
  enabled: true                # Set to false to disable without deleting (default: true)

  # Runtime configuration
  runtime: vllm                # Inference runtime: vllm, triton, or tgi (default: vllm)

  # Autoscaling configuration
  minReplicas: 0               # Minimum replicas (0 enables scale-to-zero, default: 0)
  maxReplicas: 1               # Maximum replicas for autoscaling (default: 1)

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

  # Hardware targeting
  nodeSelector:
    accelerator: nvidia-l4     # Target specific GPU hardware

  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule

  # Runtime-specific configuration
  runtimeArgs:
    - --max-model-len=4096
    - --gpu-memory-utilization=0.9

  runtimeEnv:
    - name: CUSTOM_VAR
      value: "custom-value"
```

#### Field Descriptions

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `modelName` | string | Yes | - | Human-readable display name for the model |
| `modelID` | string | Yes | - | HuggingFace model ID (e.g., "meta-llama/Llama-2-7b-hf") |
| `s3Bucket` | string | Conditional* | - | S3 bucket for caching model artifacts |
| `s3Key` | string | Conditional* | - | S3 path prefix for this model's artifacts |
| `trustRemoteCode` | bool | No | false | Enable custom architecture support (loads from HuggingFace directly) |
| `enabled` | bool | No | true | Whether to deploy the model (false scales to zero) |
| `runtime` | string | No | vllm | Inference runtime: `vllm`, `triton`, or `tgi` |
| `minReplicas` | int32 | No | 0 | Minimum replicas (0 enables scale-to-zero via Knative) |
| `maxReplicas` | int32 | No | 1 | Maximum replicas for KServe autoscaling |
| `resources` | ResourceRequirements | No | - | CPU, memory, and GPU resource requests/limits |
| `nodeSelector` | map[string]string | No | - | Node labels for pod scheduling (e.g., GPU type) |
| `tolerations` | []Toleration | No | - | Tolerations for pod scheduling on tainted nodes |
| `runtimeArgs` | []string | No | - | Additional CLI arguments passed to the runtime |
| `runtimeEnv` | []EnvVar | No | - | Additional environment variables for the runtime |

*`s3Bucket` and `s3Key` are required for S3-based deployment, but optional when `trustRemoteCode: true`

**Deprecated fields** (still supported but not recommended):
- `replicas`: Use `minReplicas` and `maxReplicas` instead

### 4.2 Example: Basic Deployment (Mistral-7B)

Minimal configuration for quick deployment:

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
  enabled: true
  # Defaults: runtime=vllm, minReplicas=0, maxReplicas=1
```

### 4.3 Example: Production Deployment with Autoscaling

Full configuration with resource limits and autoscaling:

```yaml
# gitops/clusters/production/apps/ai-models/llama-3-70b.yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3-70b
  namespace: production
spec:
  # Model identification
  modelName: "Llama-3-70B"
  modelID: "meta-llama/Meta-Llama-3-70B"
  s3Bucket: "ai-aas-models-prod"
  s3Key: "models/llama-3-70b"

  # Deployment configuration
  enabled: true
  runtime: vllm

  # Autoscaling (handled by KServe/Knative)
  minReplicas: 1               # Keep 1 replica warm
  maxReplicas: 5               # Scale up to 5 under load

  # Resource requirements
  resources:
    requests:
      cpu: "16"
      memory: "128Gi"
      nvidia.com/gpu: "4"      # 4x A100 GPUs
    limits:
      cpu: "32"
      memory: "256Gi"
      nvidia.com/gpu: "4"

  # Target A100 GPU nodes
  nodeSelector:
    accelerator: nvidia-a100
    node-type: gpu-large

  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
    - key: gpu-workload
      operator: Equal
      value: "true"
      effect: NoSchedule

  # vLLM-specific arguments
  runtimeArgs:
    - --max-model-len=8192
    - --gpu-memory-utilization=0.95
    - --tensor-parallel-size=4
    - --trust-remote-code

  # Custom environment variables
  runtimeEnv:
    - name: VLLM_LOGGING_LEVEL
      value: "INFO"
```

### 4.4 Example: Scale-to-Zero Development Model

Cost-optimized configuration for development:

```yaml
# gitops/clusters/development/apps/ai-models/gpt2.yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: gpt2-dev
  namespace: development
spec:
  modelName: "GPT-2"
  modelID: "gpt2"
  s3Bucket: "ai-aas-models-dev"
  s3Key: "models/gpt2"

  enabled: true
  runtime: vllm

  # Scale to zero when idle (default)
  minReplicas: 0
  maxReplicas: 1

  # Minimal resources
  resources:
    requests:
      cpu: "2"
      memory: "4Gi"
      nvidia.com/gpu: "1"
    limits:
      cpu: "4"
      memory: "8Gi"
      nvidia.com/gpu: "1"

  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
```

### 4.5 Apply Directly (for testing)

```bash
kubectl apply -f mistral-7b.yaml
```

### 4.6 Or Commit to GitOps Repository (recommended)

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

# Expected output with new columns:
# NAME                  MODEL                RUNTIME   ENABLED   READY   PHASE          AGE
# mistral-7b-instruct   Mistral-7B-Instruct  vllm      true      0       Pending        0s
# mistral-7b-instruct   Mistral-7B-Instruct  vllm      true      0       Downloading    5s
# mistral-7b-instruct   Mistral-7B-Instruct  vllm      true      0       Deploying      10m
# mistral-7b-instruct   Mistral-7B-Instruct  vllm      true      1       Ready          12m
```

The status progression:
1. **Pending**: Initial state, operator starting reconciliation
2. **Downloading**: Model artifacts being downloaded from HuggingFace to S3
3. **Deploying**: InferenceService created, KServe starting runtime pods
4. **Ready**: Model deployed and ready to serve requests

### 5.2 Check Detailed Status

```bash
kubectl describe aimodel mistral-7b-instruct
```

Look for the `Status` section which includes:
- `Phase`: Current deployment phase
- `InferenceServiceName`: Name of the managed InferenceService
- `InferenceEndpoint`: URL for inference requests
- `ReadyReplicas`: Number of ready pods
- `Message`: Additional status information

### 5.3 Monitor Downloader Job

The operator creates a Kubernetes Job to download model artifacts:

```bash
# Check job status
kubectl get jobs | grep mistral-7b-instruct

# Watch job logs (model download progress)
kubectl logs -f job/mistral-7b-instruct-downloader

# Check if job completed
kubectl get job mistral-7b-instruct-downloader -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}'
```

### 5.4 Monitor KServe InferenceService

The operator creates a KServe InferenceService to manage the runtime deployment:

```bash
# Check InferenceService status
kubectl get inferenceservice mistral-7b-instruct

# Detailed InferenceService info
kubectl describe inferenceservice mistral-7b-instruct

# Check KServe predictor pods
kubectl get pods -l serving.kserve.io/inferenceservice=mistral-7b-instruct

# View runtime logs (vLLM, Triton, or TGI)
kubectl logs -l serving.kserve.io/inferenceservice=mistral-7b-instruct -c kserve-container
```

### 5.5 Monitor Autoscaling

KServe uses Knative for autoscaling, including scale-to-zero:

```bash
# Check Pod Autoscaler (KPA)
kubectl get podautoscaler -l serving.kserve.io/inferenceservice=mistral-7b-instruct

# Check current replica count
kubectl get pods -l serving.kserve.io/inferenceservice=mistral-7b-instruct --watch

# View autoscaler metrics
kubectl describe podautoscaler mistral-7b-instruct-predictor-default
```

## Step 6: Test the Model

### 6.1 Get the Inference Endpoint

KServe creates a service endpoint for the InferenceService:

```bash
# Get InferenceService with endpoint URL
kubectl get inferenceservice mistral-7b-instruct

# Get the endpoint URL from AIModel status
kubectl get aimodel mistral-7b-instruct -o jsonpath='{.status.inferenceEndpoint}'
```

### 6.2 Access via Port-Forward (for testing)

```bash
# Port-forward to the InferenceService predictor
kubectl port-forward -n default \
  svc/mistral-7b-instruct-predictor-default-private 8080:80
```

Note: KServe exposes the service on port 80, which we forward to localhost:8080

### 6.3 Send a Test Request

```bash
# OpenAI-compatible API (vLLM runtime)
curl http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Mistral-7B-Instruct",
    "messages": [{"role": "user", "content": "Hello, how are you?"}],
    "max_tokens": 100
  }'
```

### 6.4 Check Model Info

```bash
# List available models
curl http://localhost:8080/v1/models

# Health check
curl http://localhost:8080/health
```

### 6.5 Access via Ingress (production)

In production environments, KServe InferenceServices are typically exposed via Ingress:

```bash
# Get the external URL (if Ingress is configured)
kubectl get inferenceservice mistral-7b-instruct -o jsonpath='{.status.url}'

# Example request to external endpoint
curl https://mistral-7b-instruct.default.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "Mistral-7B-Instruct",
    "messages": [{"role": "user", "content": "Explain quantum computing"}],
    "max_tokens": 200
  }'
```

## Step 7: Manage Model Lifecycle

### 7.1 Scale Down (Disable)

To stop the model without deleting the configuration:

```bash
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"enabled":false}}'
```

This will:
- Delete the InferenceService (stops GPU usage)
- Keep the AIModel resource (configuration preserved)
- Keep S3 artifacts (no re-download needed)

The model status will transition to `Disabled`.

### 7.2 Scale Up (Re-enable)

```bash
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"enabled":true}}'
```

The operator will recreate the InferenceService and the model will become ready again.

### 7.3 Adjust Autoscaling

Change the minimum and maximum replica counts:

```bash
# Enable scale-to-zero (cost savings)
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"minReplicas":0}}'

# Keep 2 replicas warm (lower latency)
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"minReplicas":2}}'

# Increase max replicas for high load
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"maxReplicas":10}}'
```

### 7.4 Update Runtime Configuration

Modify runtime arguments or environment variables:

```bash
# Add/modify runtime args (e.g., increase context length)
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{
  "spec": {
    "runtimeArgs": [
      "--max-model-len=8192",
      "--gpu-memory-utilization=0.95"
    ]
  }
}'
```

The operator will update the InferenceService with the new configuration.

### 7.5 Switch Runtime

Change the inference runtime (requires redeployment):

```bash
# Switch from vLLM to Triton
kubectl patch aimodel mistral-7b-instruct --type=merge -p '{"spec":{"runtime":"triton"}}'
```

### 7.6 Delete Model Completely

```bash
kubectl delete aimodel mistral-7b-instruct
```

This will:
- Delete the InferenceService and all associated pods
- Delete the Downloader Job
- Keep S3 artifacts (manual cleanup required if desired)

## Troubleshooting

### Phase: Pending (stuck)

AIModel stuck in Pending phase:

```bash
# Check operator logs
kubectl logs -n ai-model-system -l app=ai-model-operator --tail=100

# Check AIModel status for error messages
kubectl get aimodel mistral-7b-instruct -o jsonpath='{.status.message}'

# Common issues:
# - Operator not running
# - CRD not installed
# - RBAC permissions missing
```

### Phase: Downloading (stuck)

Check the downloader job logs:

```bash
kubectl logs job/mistral-7b-instruct-downloader

# Common issues:
# - HuggingFace rate limiting (wait and retry)
# - Invalid HF token for gated model
# - S3 write permission denied
# - Network connectivity issues
```

Verify S3 access:

```bash
# Verify S3 credentials secret exists
kubectl get secret s3-credentials -o yaml

# Test S3 access manually
aws s3 ls s3://<bucket>/<key>/
```

### Phase: Deploying (stuck)

Model stuck in Deploying phase, InferenceService not becoming ready:

```bash
# Check InferenceService status
kubectl get inferenceservice mistral-7b-instruct
kubectl describe inferenceservice mistral-7b-instruct

# Check KServe controller logs
kubectl logs -n kserve -l control-plane=kserve-controller-manager

# Check predictor pods
kubectl get pods -l serving.kserve.io/inferenceservice=mistral-7b-instruct
kubectl describe pods -l serving.kserve.io/inferenceservice=mistral-7b-instruct
```

Common issues:
- **Insufficient GPU resources**: Check node GPU availability
- **Image pull errors**: Verify runtime image is accessible
- **Resource limits too low**: Increase CPU/memory requests
- **Node selector mismatch**: Check nodeSelector matches available nodes

### Phase: Failed

Check operator and InferenceService logs:

```bash
# Operator logs
kubectl logs -n ai-model-system -l app=ai-model-operator --tail=100

# InferenceService events
kubectl describe inferenceservice mistral-7b-instruct

# Predictor pod logs (if pods exist)
kubectl logs -l serving.kserve.io/inferenceservice=mistral-7b-instruct -c kserve-container
```

### InferenceService Not Ready

InferenceService exists but not ready:

```bash
# Check InferenceService conditions
kubectl get inferenceservice mistral-7b-instruct -o jsonpath='{.status.conditions}' | jq

# Check KServe predictor revision
kubectl get revision -l serving.kserve.io/inferenceservice=mistral-7b-instruct

# Check Knative service
kubectl get ksvc -l serving.kserve.io/inferenceservice=mistral-7b-instruct
```

Common issues:
- **Model too large for GPU**: Check GPU memory vs model size
- **S3 access from pod**: Verify S3 credentials and network access
- **Runtime configuration error**: Check runtimeArgs for typos
- **Knative/KServe not installed**: Verify KServe components are running

### Runtime Pod CrashLooping

Pod starts but crashes repeatedly:

```bash
# View pod logs
kubectl logs -l serving.kserve.io/inferenceservice=mistral-7b-instruct -c kserve-container --tail=100

# Check pod events
kubectl describe pods -l serving.kserve.io/inferenceservice=mistral-7b-instruct
```

Common issues:
- **Model too large for GPU**: Reduce `gpu-memory-utilization` or use smaller model
- **S3 download failed**: Check S3 credentials and bucket access
- **Invalid runtime args**: Review `runtimeArgs` for syntax errors
- **OOM (Out of Memory)**: Increase memory limits

### Scale-to-Zero Not Working

Model should scale to zero but stays running:

```bash
# Check minReplicas setting
kubectl get aimodel mistral-7b-instruct -o jsonpath='{.spec.minReplicas}'

# Check KServe autoscaler
kubectl describe podautoscaler mistral-7b-instruct-predictor-default

# Check Knative autoscaler config
kubectl get configmap -n knative-serving config-autoscaler -o yaml
```

Verify `minReplicas: 0` is set in the AIModel spec.

### Secrets Not Found

Ensure secrets are in the same namespace as the AIModel:

```bash
# Check namespace
kubectl get aimodel mistral-7b-instruct -o jsonpath='{.metadata.namespace}'

# Secrets must be in that namespace
kubectl get secrets -n <namespace> | grep -E "s3-credentials|hf-credentials"
```

### KServe Webhook Errors

If InferenceService creation fails with webhook errors:

```bash
# Check KServe webhook is running
kubectl get pods -n kserve -l control-plane=kserve-controller-manager

# Check webhook configuration
kubectl get validatingwebhookconfigurations inferenceservice.serving.kserve.io

# Verify webhook service is accessible
kubectl get svc -n kserve kserve-webhook-server-service
```

## Reference

### AIModel Status Phases

| Phase | Description |
|-------|-------------|
| `Pending` | Initial state, waiting for reconciliation |
| `Downloading` | Downloader job running (HuggingFace → S3) |
| `Deploying` | InferenceService created, KServe deploying runtime pods |
| `Ready` | InferenceService ready, model serving requests |
| `Failed` | Error occurred (check logs and status message) |
| `Disabled` | Model disabled via `enabled: false` |

### AIModel Status Fields

The AIModel status provides detailed information about the deployment:

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current deployment phase (see above) |
| `inferenceServiceName` | string | Name of the managed KServe InferenceService |
| `inferenceEndpoint` | string | URL where the model can be accessed |
| `readyReplicas` | int32 | Number of ready replicas serving requests |
| `downloadProgress` | int32 | Download progress percentage (0-100) |
| `message` | string | Additional information about current state |
| `lastTransitionTime` | time | When the phase last changed |
| `conditions` | []Condition | Detailed Kubernetes conditions |

**Deprecated status fields** (still present but use new fields instead):
- `vllmDeploymentName`: Use `inferenceServiceName`
- `vllmServiceName`: Use `inferenceEndpoint`

### Required Secrets

| Secret Name | Keys | Required | Description |
|-------------|------|----------|-------------|
| `s3-credentials` | `access-key-id`, `secret-access-key` | Yes | S3/Object storage credentials for model artifacts |
| `hf-credentials` | `token` | No | HuggingFace token for private/gated models |

### Supported Runtimes

| Runtime | Value | Description | Default Image |
|---------|-------|-------------|---------------|
| vLLM | `vllm` | High-performance LLM inference | `vllm/vllm-openai:latest` |
| Triton | `triton` | NVIDIA Triton Inference Server | `nvcr.io/nvidia/tritonserver:latest` |
| TGI | `tgi` | HuggingFace Text Generation Inference | `ghcr.io/huggingface/text-generation-inference:latest` |

### Default Runtime Configuration (vLLM)

When using the vLLM runtime, the operator configures these default arguments:

```
--model s3://<bucket>/<key>
--served-model-name <modelName>
--dtype auto
--max-model-len 4096
--gpu-memory-utilization 0.9
--trust-remote-code
```

You can override or extend these using the `runtimeArgs` field.

### Default Resources

If `resources` is not specified, these defaults apply:

```yaml
resources:
  requests:
    nvidia.com/gpu: "1"
  limits:
    nvidia.com/gpu: "1"
```

Default tolerations (automatically added):
```yaml
tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
```

### KServe Integration

The operator creates a KServe InferenceService with:
- **Predictor**: Runs the inference runtime (vLLM/Triton/TGI)
- **Autoscaling**: Via Knative Pod Autoscaler (KPA)
- **Scale-to-zero**: Enabled when `minReplicas: 0`
- **Storage**: S3-compatible storage for model artifacts

InferenceService naming: `<aimodel-name>` (same as AIModel resource name)
