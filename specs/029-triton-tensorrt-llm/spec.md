# Spec 029: TensorRT-LLM/Triton Support

## Overview

Add full TensorRT-LLM support to the ai-aas platform via NVIDIA Triton Inference Server, enabling deployment of Llama 3.1 8B Instruct with pre-compiled TensorRT engines.

## Scope

- **Full Triton infrastructure** (not just one model)
- **Pre-compiled engines** (no compilation pipeline)
- **Target model**: Llama 3.1 8B Instruct

## Background

### Current State

The platform currently supports:
- **vLLM** - Primary inference runtime for LLMs
- **TGI** - HuggingFace Text Generation Inference
- **Triton** (partial) - Generic support exists but TensorRT-LLM specific features missing

### Why TensorRT-LLM?

TensorRT-LLM provides:
- Optimized inference kernels for NVIDIA GPUs
- Higher throughput via in-flight batching
- Lower latency through kernel fusion
- Better GPU memory utilization

### Existing Schema Support

The `ModelRecipeSpec` already supports Triton via:

```go
// Runtime enum (modelrecipe_types.go:39): vllm;triton;tgi
Runtime string `json:"runtime"`

// TritonArgs (modelrecipe_types.go:162-187)
type TritonArgs struct {
    Backend string           // python;tensorrt;onnxruntime;pytorch
    ModelRepository string   // S3/GCS path
    InstanceGroup []TritonInstanceGroup
    DynamicBatching *TritonDynamicBatching
    InputConfig []TritonTensorConfig
    OutputConfig []TritonTensorConfig
}

// HealthCheckSpec (modelrecipe_types.go:251-264) - ALREADY CONFIGURABLE
type HealthCheckSpec struct {
    StartupProbeSeconds int32  // default 300
    LivenessPath string        // default /health
    ReadinessPath string       // default /health
}
```

**Key Insight**: The schema already supports configurable health probe paths. The operator needs to USE these values from the recipe.

---

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────────────────────────┐
│                        User Request                              │
│                    (ai-aas-cli / API)                           │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                     Admin API Service                            │
│              (validates runtime: tensorrt-llm)                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    AI Model Operator                             │
│    - Maps tensorrt-llm → kserve-tensorrt-llm runtime            │
│    - Reads HealthCheckSpec from recipe                          │
│    - Creates InferenceService with correct probes               │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                  KServe InferenceService                         │
│           (uses kserve-tensorrt-llm ClusterServingRuntime)      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│              Triton Inference Server Pod                         │
│    - Image: nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3 │
│    - Health: /v2/health/live, /v2/health/ready                  │
│    - Model: loaded from S3 via storage-initializer              │
└─────────────────────────────────────────────────────────────────┘
```

### Key Differences from vLLM

| Aspect | vLLM | TensorRT-LLM |
|--------|------|--------------|
| Health endpoints | `/health` | `/v2/health/live`, `/v2/health/ready` |
| HTTP port | 8000 | 8080 |
| Startup time | ~5 min | ~10 min (engine load) |
| Model format | HuggingFace | Pre-compiled TRT engines |
| Configuration | CLI args | config.pbtxt files |

---

## Implementation

### Phase 1: KServe Infrastructure

#### 1.1 Create TensorRT-LLM ClusterServingRuntime

**File**: `infra/k8s/kserve/base/cluster-serving-runtime-tensorrt-llm.yaml`

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ClusterServingRuntime
metadata:
  name: kserve-tensorrt-llm
spec:
  annotations:
    prometheus.kserve.io/port: "8002"
    prometheus.kserve.io/path: /metrics
  supportedModelFormats:
    - name: tensorrt-llm
      version: "1"
      autoSelect: true
  protocolVersions: [v2, grpc-v2]
  containers:
    - name: kserve-container
      image: nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3
      args:
        - tritonserver
        - --model-store=/mnt/models
        - --http-port=8080
        - --grpc-port=9000
        - --allow-http=true
        - --allow-grpc=true
      ports:
        - containerPort: 8080
          name: http
        - containerPort: 9000
          name: grpc
        - containerPort: 8002
          name: metrics
      resources:
        requests:
          cpu: "4"
          memory: "24Gi"
        limits:
          cpu: "8"
          memory: "48Gi"
      livenessProbe:
        httpGet:
          path: /v2/health/live
          port: 8080
        initialDelaySeconds: 600
        periodSeconds: 30
        timeoutSeconds: 10
        failureThreshold: 3
      readinessProbe:
        httpGet:
          path: /v2/health/ready
          port: 8080
        initialDelaySeconds: 600
        periodSeconds: 10
        timeoutSeconds: 10
        failureThreshold: 3
```

#### 1.2 Add PodMonitor for Metrics

**File**: `infra/k8s/kserve/monitoring/podmonitor-tensorrt-llm.yaml`

---

### Phase 2: Operator Changes

#### 2.1 Add `tensorrt-llm` Runtime Mapping

**File**: `operators/ai-model-operator/controllers/aimodel_controller.go`
**Location**: Lines 1144-1157

```go
case "tensorrt-llm":
    modelFormat = "tensorrt-llm"
    runtimeName = "kserve-tensorrt-llm"
```

#### 2.2 Use HealthCheckSpec from Recipe (CRITICAL FIX)

**File**: `operators/ai-model-operator/internal/kserve/inferenceservice.go`
**Location**: Lines 484-501 (hardcoded `/health` on port 8000)

The schema already has `HealthCheckSpec.LivenessPath` and `ReadinessPath`, but the InferenceServiceBuilder ignores them!

Changes needed:
1. Add fields to `InferenceServiceBuilder`: `livenessPath`, `readinessPath`, `probePort`
2. Add builder method: `WithHealthProbes(livenessPath, readinessPath string, port int32)`
3. Update `BuildContainerBased()` to use these fields instead of hardcoded `/health`

#### 2.3 Pass HealthCheck Config from Recipe to Builder

**File**: `operators/ai-model-operator/controllers/aimodel_controller.go`
**Location**: Around line 1230

```go
// Get health check config from recipe (use defaults if not specified)
livenessPath := "/health"
readinessPath := "/health"
probePort := int32(8000)

if recipeSpec != nil {
    if recipeSpec.HealthCheck.LivenessPath != "" {
        livenessPath = recipeSpec.HealthCheck.LivenessPath
    }
    if recipeSpec.HealthCheck.ReadinessPath != "" {
        readinessPath = recipeSpec.HealthCheck.ReadinessPath
    }
}

// Runtime-aware defaults (if recipe doesn't specify)
if recipeSpec == nil || recipeSpec.HealthCheck.LivenessPath == "" {
    switch runtime {
    case "triton", "tensorrt-llm":
        livenessPath = "/v2/health/live"
        readinessPath = "/v2/health/ready"
        probePort = 8080
    }
}
```

#### 2.4 Implement Triton Runtime Args Conversion

**File**: `operators/ai-model-operator/controllers/aimodel_controller.go`
**Location**: Lines 1697-1701 (empty `case "triton":`)

#### 2.5 Add Triton-Specific Validation

**File**: `operators/ai-model-operator/internal/recipe/validator.go`

---

### Phase 3: CRD & Domain Updates

#### 3.1 Update Runtime Enums

**Files**:
- `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go` (line 39)
- `operators/ai-model-operator/api/v1alpha1/aimodel_types.go`

```go
// +kubebuilder:validation:Enum=vllm;triton;tensorrt-llm;tgi
```

#### 3.2 Update ValidRuntimes

**File**: `services/admin-api-service/internal/domain/recipe.go` (line 55)

```go
var ValidRuntimes = []string{"vllm", "triton", "tensorrt-llm", "tgi"}
```

#### 3.3 Regenerate CRDs

```bash
cd operators/ai-model-operator
make generate
make manifests
```

---

### Phase 4: Model Recipe & Deployment

#### 4.1 Create Llama 3.1 8B Recipe

**File**: `infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml`

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: llama-3.1-8b-instruct-trtllm
  namespace: ai-model-system
  labels:
    ai.ai-aas.io/model-family: llama
    ai.ai-aas.io/runtime: tensorrt-llm
spec:
  modelID: meta-llama/Llama-3.1-8B-Instruct
  displayName: "Llama 3.1 8B Instruct (TensorRT-LLM)"
  description: "Meta's Llama 3.1 8B Instruct optimized with TensorRT-LLM"

  runtime: tensorrt-llm
  image: nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3

  resources:
    gpu:
      vendor: nvidia
      count: 1
      minMemoryGB: 24
    cpu:
      requests: "8"
      limits: "16"
    memory:
      requests: "32Gi"
      limits: "48Gi"

  runtimeArgs:
    triton:
      backend: tensorrt
      modelRepository: "s3://ai-aas-models/triton/llama-3.1-8b-instruct-trtllm"
      instanceGroup:
        - kind: KIND_GPU
          count: 1
      dynamicBatching:
        maxBatchSize: 64
        maxQueueDelayMicroseconds: 100000

  scheduling:
    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
    nodeSelector:
      nvidia.com/gpu.present: "true"

  healthCheck:
    startupProbeSeconds: 600
    livenessPath: /v2/health/live
    readinessPath: /v2/health/ready

  metadata:
    parameters: "8B"
    contextLength: 128000
    architecture: LlamaForCausalLM
    license: "Llama 3.1 Community License"
    sourceURL: "https://huggingface.co/meta-llama/Llama-3.1-8B-Instruct"
```

#### 4.2 S3 Model Repository Structure

```
s3://ai-aas-models/triton/llama-3.1-8b-instruct-trtllm/
├── ensemble/
│   └── config.pbtxt
├── preprocessing/
│   ├── 1/model.py
│   └── config.pbtxt
├── postprocessing/
│   ├── 1/model.py
│   └── config.pbtxt
└── tensorrt_llm/
    ├── 1/
    │   ├── rank0.engine  # Pre-compiled TRT engine
    │   └── tokenizer/
    └── config.pbtxt
```

---

### Phase 5: Documentation

**File**: `docs/runbooks/build-tensorrt-llm-engine.md`

Document:
- TensorRT-LLM installation
- Checkpoint conversion from HuggingFace
- Engine building commands
- S3 upload process

---

## Critical Files Summary

| File | Change Type | Description |
|------|-------------|-------------|
| `infra/k8s/kserve/base/cluster-serving-runtime-tensorrt-llm.yaml` | New | ClusterServingRuntime |
| `infra/k8s/kserve/monitoring/podmonitor-tensorrt-llm.yaml` | New | Prometheus metrics |
| `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go` | Modify | Runtime enum (line 39) |
| `operators/ai-model-operator/api/v1alpha1/aimodel_types.go` | Modify | Runtime enum |
| `operators/ai-model-operator/controllers/aimodel_controller.go` | Modify | Runtime mapping, health probes, args |
| `operators/ai-model-operator/internal/kserve/inferenceservice.go` | Modify | Configurable probes (484-501) |
| `operators/ai-model-operator/internal/recipe/validator.go` | Modify | TensorRT-LLM validation |
| `services/admin-api-service/internal/domain/recipe.go` | Modify | ValidRuntimes (line 55) |
| `infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml` | New | Model recipe |
| `docs/runbooks/build-tensorrt-llm-engine.md` | New | Engine build docs |

---

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Breaking vLLM deployments | High | Extensive testing, separate runtime name |
| GPU-specific engines | Medium | Document GPU architecture requirements |
| Extended startup times | Medium | 600s initial delay in probes |

---

## Success Criteria

1. `tensorrt-llm` runtime accepted by Admin API and operator
2. ClusterServingRuntime deploys Triton with TRT-LLM image
3. Health probes use `/v2/health/*` endpoints
4. Llama 3.1 8B recipe validates and deploys successfully
5. Inference requests return correct responses
