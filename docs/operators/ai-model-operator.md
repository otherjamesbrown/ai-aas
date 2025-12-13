---
title: AI Model Operator
last_updated: 2025-12-10
owner: operator-developer
---

# AI Model Operator

The AI Model Operator manages the lifecycle of AI models on the platform. It handles downloading models from HuggingFace Hub, caching them in S3, and deploying them via KServe InferenceServices.

## Custom Resource Definition

### AIModel Spec

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: gpt2-test
  namespace: development
spec:
  # Required: HuggingFace model ID
  modelID: gpt2

  # Required: Display name for the model
  modelName: GPT-2

  # Required: Inference runtime (vllm, tgi, triton)
  runtime: vllm

  # Required: Enable/disable the model
  enabled: true

  # Required: S3 storage location
  s3Bucket: ai-aas
  s3Key: models/gpt2

  # Optional: Replica scaling
  minReplicas: 0
  maxReplicas: 1

  # Optional: Resource requirements
  resources:
    requests:
      cpu: "2"
      memory: "4Gi"
      nvidia.com/gpu: "1"
    limits:
      cpu: "4"
      memory: "8Gi"
      nvidia.com/gpu: "1"

  # Optional: GPU node scheduling
  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
    - key: gpu-workload
      operator: Equal
      value: "true"
      effect: NoSchedule

  # Optional: Node selection
  nodeSelector:
    gpu-type: a100

  # Optional: Runtime-specific arguments
  runtimeArgs:
    - "--dtype=auto"
    - "--max-model-len=4096"

  # Optional: Runtime environment variables
  runtimeEnv:
    - name: CUSTOM_VAR
      value: "value"
```

### AIModel Status

```yaml
status:
  phase: Ready              # Current phase
  message: "Model is ready" # Human-readable message
  inferenceServiceName: gpt2-test
  inferenceEndpoint: "http://gpt2-test.development.svc.cluster.local"
  readyReplicas: 1
  retryCount: 0             # Number of retry attempts
  lastRetryTime: null       # Timestamp of last retry
  nextRetryTime: null       # Scheduled next retry time
```

## Source Code Structure

```
operators/ai-model-operator/
├── api/v1alpha1/
│   ├── aimodel_types.go      # AIModel struct definitions
│   ├── groupversion_info.go  # API group registration
│   └── zz_generated.deepcopy.go
├── controllers/
│   ├── aimodel_controller.go      # Main reconciliation logic
│   └── aimodel_controller_test.go # Unit tests
├── internal/
│   └── kserve/
│       └── inferenceservice.go    # KServe client helpers
├── config/
│   ├── crd/bases/                 # Generated CRD manifests
│   ├── rbac/                      # RBAC rules
│   └── manager/                   # Deployment manifests
├── deployments/helm/ai-model-operator/
│   ├── Chart.yaml
│   ├── values.yaml
│   ├── values-development.yaml
│   ├── crds/                      # CRDs for Helm install
│   └── templates/
└── Makefile
```

## Key Files

### aimodel_types.go

Defines the AIModel CRD structure:

```go
// AIModelSpec defines the desired state
type AIModelSpec struct {
    ModelID     string `json:"modelID"`
    ModelName   string `json:"modelName"`
    Runtime     string `json:"runtime"`
    Enabled     bool   `json:"enabled"`
    S3Bucket    string `json:"s3Bucket"`
    S3Key       string `json:"s3Key"`
    // ... other fields
}

// AIModelStatus defines the observed state
type AIModelStatus struct {
    Phase                string `json:"phase,omitempty"`
    Message              string `json:"message,omitempty"`
    InferenceServiceName string `json:"inferenceServiceName,omitempty"`
    InferenceEndpoint    string `json:"inferenceEndpoint,omitempty"`
    ReadyReplicas        int32  `json:"readyReplicas,omitempty"`
}
```

### aimodel_controller.go

Key functions:

| Function | Purpose |
|----------|---------|
| `Reconcile()` | Main reconciliation loop |
| `checkS3ArtifactExists()` | Check if model is cached in S3 |
| `modelDownloaderJob()` | Create HuggingFace download job |
| `createOrUpdateInferenceService()` | Manage KServe InferenceService |
| `updateStatusFromInferenceService()` | Sync status from InferenceService |
| `finalizeAIModel()` | Cleanup on deletion |

## Reconciliation Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                        Reconcile()                               │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  1. Fetch AIModel CR                                            │
│     └─ Not found? Return (garbage collected)                    │
│                                                                  │
│  2. Handle deletion (if DeletionTimestamp set)                  │
│     └─ Run finalizer, remove owned resources                    │
│                                                                  │
│  3. Add finalizer if not present                                │
│                                                                  │
│  4. Check if enabled=false                                      │
│     └─ Set phase=Disabled, return                               │
│                                                                  │
│  5. Check for existing downloader job                           │
│     ├─ Not found?                                               │
│     │   ├─ S3 artifacts exist? → Skip to step 6                │
│     │   └─ Create downloader job, phase=Downloading            │
│     ├─ Job complete? → phase=Downloaded                        │
│     ├─ Job failed? → phase=RetryPending, schedule retry        │
│     └─ Job running? → Requeue                                  │
│                                                                  │
│  6. Create/Update InferenceService                              │
│     └─ phase=Deploying                                          │
│                                                                  │
│  7. Update status from InferenceService                         │
│     └─ InferenceService ready? → phase=Ready                   │
│                                                                  │
└─────────────────────────────────────────────────────────────────┘
```

## Dependencies

### Secrets Required

The operator expects these secrets in the AIModel's namespace:

**s3-credentials**
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: s3-credentials
type: Opaque
data:
  AWS_ACCESS_KEY_ID: <base64>
  AWS_SECRET_ACCESS_KEY: <base64>
  AWS_ENDPOINT_URL_S3: <base64>  # e.g., https://fr-par-1.linodeobjects.com
  S3_ENDPOINT: <base64>          # Alternative key
  S3_REGION: <base64>            # Optional, defaults to us-east-1
```

**hf-credentials** (optional)
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hf-credentials
type: Opaque
data:
  token: <base64>  # HuggingFace API token for private models
```

### RBAC Requirements

The operator needs permissions for:

- AIModel CRD (full access)
- Jobs (create, get, list, watch, delete)
- Secrets (get - for S3 credentials)
- InferenceServices (full access)
- Deployments, Services (legacy support)
- Events (create, patch)

## Downloader Job

The operator creates a Kubernetes Job to download models:

```python
# Simplified download script
from huggingface_hub import snapshot_download
import boto3

# Download from HuggingFace
local_dir = snapshot_download(model_id, local_dir='/tmp/model')

# Upload to S3
s3 = boto3.client('s3', endpoint_url=s3_endpoint)
for file in files:
    s3.upload_file(local_path, bucket, s3_path)
```

Job configuration:
- Image: `python:3.11-slim`
- Backoff limit: 6 (Kubernetes default)
- Restart policy: OnFailure

## Retry Logic

The operator implements automatic retry with exponential backoff for failed download jobs:

| Configuration | Default Value | Description |
|--------------|---------------|-------------|
| `maxDownloadRetries` | 5 | Maximum retry attempts for download jobs |
| `initialRetryDelay` | 1 minute | Initial delay before first retry |
| `maxRetryDelay` | 16 minutes | Maximum delay between retries |

When a download job fails:
1. The operator sets phase to `RetryPending`
2. Calculates next retry time using exponential backoff: `min(initialDelay * 2^retryCount, maxDelay)`
3. Deletes the failed job
4. Creates a new download job when the retry time is reached
5. After `maxDownloadRetries` attempts, sets phase to `Failed`

Status fields for retry tracking:
- `retryCount`: Number of retry attempts made
- `lastRetryTime`: Timestamp of the last retry attempt
- `nextRetryTime`: Scheduled time for the next retry

## Known Issues / Planned Improvements

| Issue ID | Description | Status |
|----------|-------------|--------|
| ai-aas-evb | Automatic retry for failed download jobs | ✅ Implemented |
| ai-aas-hqx | RetryCount/LastRetryTime status fields | ✅ Implemented |
| ai-aas-p04 | RetryPending phase | ✅ Implemented |
| ai-aas-ujh | Retry logic with exponential backoff | ✅ Implemented |

## Debugging

### Check Operator Logs

```bash
KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml \
  kubectl logs -n ai-model-system -l app.kubernetes.io/name=ai-model-operator -f
```

### Check AIModel Status

```bash
kubectl get aimodel -A
kubectl describe aimodel <name> -n <namespace>
```

### Check Downloader Job

```bash
kubectl get jobs -n <namespace> -l aimodel=<name>
kubectl logs job/<name>-downloader -n <namespace>
```

### Check InferenceService

```bash
kubectl get inferenceservice -n <namespace>
kubectl describe inferenceservice <name> -n <namespace>
```

## Testing

```bash
cd operators/ai-model-operator

# Run unit tests
make test

# Run with verbose output
go test ./controllers/... -v

# Run specific test
go test ./controllers/... -run TestReconcile -v

# Check for race conditions
go test -race ./...
```
