# AI Model Operator

A Kubernetes operator for declarative AI model management using GitOps principles.

## Overview

The AI Model Operator automates the full lifecycle of AI model deployments on Kubernetes:

1. **Download**: Pulls model artifacts from HuggingFace to S3 storage
2. **Deploy**: Creates KServe InferenceServices with the specified runtime
3. **Scale**: Manages autoscaling from 0 to N replicas via KServe/Knative
4. **Monitor**: Updates status to reflect deployment health

## Architecture

```
AIModel CR → Operator → [Downloader Job] → S3 Storage
                     ↓
              InferenceService → KServe → Runtime Pod (vLLM/Triton/TGI)
```

### Components

- **AIModel CRD**: High-level abstraction for AI model deployments
- **Controller**: Reconciles AIModel resources to desired state
- **Downloader**: Job that syncs HuggingFace artifacts to S3
- **KServe Integration**: Manages InferenceService lifecycle

## Features

- **GitOps-native**: Declarative model management via Kubernetes manifests
- **Multi-runtime**: Supports vLLM, Triton, and TGI inference runtimes
- **Autoscaling**: Scale-to-zero and load-based scaling via KServe/Knative
- **Hardware targeting**: NodeSelector and tolerations for GPU placement
- **Flexible configuration**: Runtime args and env vars for optimization
- **Status tracking**: Detailed phase reporting and readiness

## Quick Start

See the [Quickstart Guide](../../specs/023-model-gitops/quickstart.md) for a 5-minute walkthrough.

For detailed usage, see the [Operator Guide](../../docs/platform/ai-model-operator-guide.md).

## Development

### Prerequisites

- Go 1.22+
- Kubernetes 1.28+
- KServe 0.11+ (with Knative Serving)
- kubectl
- kustomize (optional)

### Local Development

```bash
# Install dependencies
go mod download

# Generate CRD manifests
make generate
make manifests

# Run tests
go test ./...

# Build operator
go build -o bin/manager main.go

# Run locally (requires kubeconfig)
export KUBECONFIG=/path/to/kubeconfig
go run main.go
```

### Building Container Image

```bash
# Build image
docker build -t ai-model-operator:dev .

# Test image locally
kind load docker-image ai-model-operator:dev
```

### Deploying to Development Cluster

```bash
# Apply CRDs
kubectl apply -f api/v1alpha1/aimodel_crd.yaml

# Deploy operator
kubectl apply -f deployments/helm/ai-model-operator/templates/

# Or use Helm
helm install ai-model-operator deployments/helm/ai-model-operator/
```

## Project Structure

```
operators/ai-model-operator/
├── api/v1alpha1/              # CRD definitions
│   └── aimodel_types.go       # AIModel type definitions
├── controllers/               # Reconciliation logic
│   └── aimodel_controller.go  # Main controller
├── internal/                  # Internal packages
│   ├── downloader/            # Downloader job creation
│   ├── inferenceservice/      # KServe InferenceService management
│   └── status/                # Status update utilities
├── deployments/helm/          # Helm chart for operator
├── Dockerfile                 # Operator container image
├── main.go                    # Operator entrypoint
└── README.md                  # This file
```

## AIModel CRD

### Spec Fields

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `modelName` | string | Yes | - | Human-readable model name |
| `modelID` | string | Yes | - | HuggingFace model ID |
| `s3Bucket` | string | Yes | - | S3 bucket for artifacts |
| `s3Key` | string | Yes | - | S3 path prefix |
| `enabled` | bool | No | true | Enable/disable deployment |
| `runtime` | string | No | vllm | Runtime: vllm, triton, tgi |
| `minReplicas` | int32 | No | 0 | Minimum replicas (0 = scale-to-zero) |
| `maxReplicas` | int32 | No | 1 | Maximum replicas |
| `resources` | ResourceRequirements | No | - | CPU/memory/GPU resources |
| `nodeSelector` | map[string]string | No | - | Node labels for scheduling |
| `tolerations` | []Toleration | No | - | Pod tolerations |
| `runtimeArgs` | []string | No | - | Additional CLI arguments |
| `runtimeEnv` | []EnvVar | No | - | Additional environment variables |

### Status Fields

| Field | Type | Description |
|-------|------|-------------|
| `phase` | string | Current deployment phase |
| `inferenceServiceName` | string | Name of managed InferenceService |
| `inferenceEndpoint` | string | Inference URL |
| `readyReplicas` | int32 | Number of ready replicas |
| `downloadProgress` | int32 | Download progress (0-100) |
| `message` | string | Status message |
| `lastTransitionTime` | Time | Last phase change |
| `conditions` | []Condition | Detailed conditions |

## Controller Logic

### Reconciliation Flow

1. **Fetch AIModel**: Get the resource from the API server
2. **Validate Spec**: Check required fields and constraints
3. **Check Enabled**: If disabled, delete InferenceService and return
4. **Download Phase**: Create/monitor downloader job if artifacts missing
5. **Deploy Phase**: Create/update KServe InferenceService
6. **Monitor Phase**: Update status based on InferenceService state
7. **Update Status**: Write phase, endpoint, and readiness to status

### Status Phases

- **Pending**: Initial state, starting reconciliation
- **Downloading**: Downloader job running
- **Deploying**: InferenceService created, waiting for ready
- **Ready**: InferenceService ready, serving requests
- **Failed**: Error occurred
- **Disabled**: Model disabled via `enabled: false`

## Downloader Job

The operator creates a Kubernetes Job to download model artifacts from HuggingFace to S3.

**Image**: `ai-model-downloader:latest` (see `../ai-model-downloader/`)

**Environment Variables**:
- `MODEL_ID`: HuggingFace model ID
- `S3_BUCKET`: Target S3 bucket
- `S3_KEY`: Target S3 prefix
- `AWS_ACCESS_KEY_ID`: From s3-credentials secret
- `AWS_SECRET_ACCESS_KEY`: From s3-credentials secret
- `HF_TOKEN`: From hf-credentials secret (optional)

**Job Lifecycle**:
- Created when artifacts don't exist in S3
- Runs to completion (1 success)
- Retries on failure (backoff: 30s, 60s, 120s)
- Logs download progress

## KServe Integration

The operator manages KServe InferenceService resources:

### InferenceService Template

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: <aimodel-name>
  namespace: <aimodel-namespace>
spec:
  predictor:
    minReplicas: <spec.minReplicas>
    maxReplicas: <spec.maxReplicas>
    containers:
      - name: kserve-container
        image: <runtime-image>
        args: <runtime-args>
        env: <runtime-env>
        resources: <spec.resources>
    nodeSelector: <spec.nodeSelector>
    tolerations: <spec.tolerations>
```

### Runtime Images

| Runtime | Default Image |
|---------|---------------|
| vllm | `vllm/vllm-openai:latest` |
| triton | `nvcr.io/nvidia/tritonserver:latest` |
| tgi | `ghcr.io/huggingface/text-generation-inference:latest` |

### Autoscaling

KServe uses Knative Pod Autoscaler (KPA) for autoscaling:

- **Scale-to-zero**: Enabled when `minReplicas: 0`
- **Scale-from-zero**: Cold start on first request
- **Load-based**: Scales based on concurrency metrics
- **Max replicas**: Enforced by `maxReplicas`

### Revision Garbage Collection

When InferenceServices are updated, Knative creates new revisions. For GPU workloads, old revisions can hold expensive GPU resources even when not receiving traffic.

The platform configures aggressive garbage collection via the `config-gc` ConfigMap in the `knative-serving` namespace:

- **min-non-active-revisions: 1** - Keep only 1 old revision for rollback
- **max-non-active-revisions: 2** - Never keep more than 2 old revisions
- **retain-since-last-active-time: 5m** - Clean up revisions 5 minutes after they become inactive
- **retain-since-create-time: 10m** - Clean up created-but-never-active revisions after 10 minutes

This prevents scenarios where multiple revisions hold GPUs unnecessarily. For example, if `mistral-7b-instruct` has 3 revisions (00001, 00002, 00003), each holding 1 GPU, only the active revision should consume GPU resources.

Configuration: `/infra/k8s/knative-serving/config-gc.yaml`

## Configuration

### Environment Variables (Operator)

| Variable | Default | Description |
|----------|---------|-------------|
| `DOWNLOADER_IMAGE` | `ai-model-downloader:latest` | Downloader job image |
| `VLLM_IMAGE` | `vllm/vllm-openai:latest` | vLLM runtime image |
| `TRITON_IMAGE` | `nvcr.io/nvidia/tritonserver:latest` | Triton runtime image |
| `TGI_IMAGE` | `ghcr.io/huggingface/text-generation-inference:latest` | TGI runtime image |

### RBAC

The operator requires permissions for:
- `aimodels.aimodel.ai-aas.io/*`: Full access to AIModel CRD
- `inferenceservices.serving.kserve.io/*`: Manage InferenceServices
- `jobs.batch/*`: Create/monitor downloader jobs
- `secrets/read`: Read S3 and HF credentials

See `deployments/helm/ai-model-operator/templates/rbac.yaml` for full RBAC configuration.

## Testing

### Unit Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific tests
go test ./controllers -v
```

### Integration Tests

```bash
# Start a test cluster (kind or minikube)
kind create cluster

# Install dependencies (KServe, Knative)
kubectl apply -f https://github.com/knative/serving/releases/download/knative-v1.11.0/serving-crds.yaml
kubectl apply -f https://github.com/kserve/kserve/releases/download/v0.11.0/kserve.yaml

# Deploy operator
make deploy

# Run test scenarios
kubectl apply -f test/e2e/aimodel-basic.yaml
kubectl wait --for=condition=ready aimodel/test-model --timeout=300s
```

## Troubleshooting

### Operator Not Starting

```bash
# Check operator pod
kubectl get pods -n ai-model-system

# Check logs
kubectl logs -n ai-model-system -l app=ai-model-operator

# Common issues:
# - CRD not installed
# - RBAC permissions missing
# - Invalid kubeconfig
```

### AIModel Not Reconciling

```bash
# Check AIModel status
kubectl describe aimodel <name>

# Check operator logs for reconciliation errors
kubectl logs -n ai-model-system -l app=ai-model-operator --tail=100 | grep <name>

# Force reconciliation (update annotation)
kubectl annotate aimodel <name> reconcile="$(date +%s)"
```

### InferenceService Not Created

```bash
# Check if KServe is installed
kubectl get crd inferenceservices.serving.kserve.io

# Check operator permissions
kubectl auth can-i create inferenceservices --as=system:serviceaccount:ai-model-system:ai-model-operator

# Check KServe webhook
kubectl get validatingwebhookconfigurations inferenceservice.serving.kserve.io
```

## Contributing

### Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Add comments for exported functions
- Write unit tests for new features

### Pull Request Checklist

- [ ] Tests added/updated
- [ ] CRD manifests regenerated (`make manifests`)
- [ ] Documentation updated
- [ ] Commit message follows convention
- [ ] PR description explains changes

## Documentation

- [Quickstart Guide](../../specs/023-model-gitops/quickstart.md) - 5-minute getting started
- [Operator Guide](../../docs/platform/ai-model-operator-guide.md) - Detailed usage guide
- [PRD](../../specs/023-model-gitops/spec.md) - Product requirements and architecture

## License

Apache License 2.0 - See LICENSE file for details
