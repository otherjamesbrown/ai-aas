# Data Model: KServe Migration

This document defines the data structures and schemas for the KServe migration feature.

## Table of Contents

1. [InferenceService CRD](#inferenceservice-crd)
2. [ClusterStorageContainer](#clusterstoragecontainer)
3. [ServingRuntime](#servingruntime)
4. [Knative Service (Generated)](#knative-service-generated)
5. [Model Registry Schema](#model-registry-schema)
6. [API Router Backend Configuration](#api-router-backend-configuration)
7. [Autoscaling Configuration](#autoscaling-configuration)
8. [Metrics Data Model](#metrics-data-model)

---

## InferenceService CRD

The primary resource for deploying models in KServe.

### Schema Definition

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: string                    # Model deployment name (e.g., llama-2-7b)
  namespace: string               # Kubernetes namespace
  labels:
    app: string                   # Application label
    model: string                 # Model identifier
    version: string               # Model version
    environment: string           # Environment (development, production)
  annotations:
    serving.kserve.io/deploymentMode: string  # "Serverless" or "RawDeployment"
spec:
  predictor:
    # Model configuration
    model:
      modelFormat:
        name: string              # Runtime type (e.g., "vllm", "pytorch", "tensorflow")
      storageUri: string          # Model location (e.g., "hf://meta-llama/Llama-2-7b-hf")
      runtime: string             # Optional: ServingRuntime name
      protocolVersion: string     # "v1" or "v2"

    # Resource requirements
    resources:
      requests:
        cpu: string               # CPU request (e.g., "2")
        memory: string            # Memory request (e.g., "8Gi")
        nvidia.com/gpu: integer   # GPU request (e.g., 1)
      limits:
        cpu: string               # CPU limit
        memory: string            # Memory limit
        nvidia.com/gpu: integer   # GPU limit

    # Scaling configuration
    minReplicas: integer          # Minimum pods (0 for scale-to-zero)
    maxReplicas: integer          # Maximum pods for autoscaling
    scaleTarget: integer          # Target concurrent requests per pod
    scaleMetric: string           # "concurrency" or "rps" (requests per second)

    # Container configuration
    containers:
      - name: string
        image: string             # Optional: custom vLLM image
        command: [string]         # Optional: override entrypoint
        args: [string]            # vLLM arguments (e.g., --dtype, --max-model-len)
        env:
          - name: string
            value: string
        volumeMounts:
          - name: string
            mountPath: string

    # Storage configuration
    volumes:
      - name: string
        persistentVolumeClaim:
          claimName: string       # PVC for model caching

    # Node affinity for GPU scheduling
    nodeSelector:
      nvidia.com/gpu.product: string  # GPU type filter
    tolerations:
      - key: string
        operator: string
        value: string
        effect: string

    # Model initialization
    storageInitializer:
      image: string               # Storage downloader image
      env:
        - name: HF_TOKEN
          valueFrom:
            secretKeyRef:
              name: string
              key: string

status:
  # Observed state
  conditions:
    - type: string                # "Ready", "IngressReady", "RoutesReady"
      status: string              # "True", "False", "Unknown"
      lastTransitionTime: timestamp
      reason: string
      message: string

  url: string                     # External inference URL
  address:
    url: string                   # Internal cluster URL

  components:
    predictor:
      latestReadyRevision: string
      latestCreatedRevision: string
      traffic:
        - revisionName: string
          percent: integer
```

### Example: vLLM Model Deployment

```yaml
apiVersion: serving.kserve.io/v1beta1
kind: InferenceService
metadata:
  name: llama-2-7b
  namespace: development
  labels:
    app: vllm-inference
    model: llama-2-7b
    version: v1
    environment: development
spec:
  predictor:
    model:
      modelFormat:
        name: vllm
      storageUri: hf://meta-llama/Llama-2-7b-hf
      protocolVersion: v2

    minReplicas: 1              # Keep 1 pod warm for production
    maxReplicas: 5
    scaleTarget: 5              # 5 concurrent requests per pod
    scaleMetric: concurrency

    resources:
      requests:
        cpu: "4"
        memory: "16Gi"
        nvidia.com/gpu: 1
      limits:
        cpu: "8"
        memory: "32Gi"
        nvidia.com/gpu: 1

    containers:
      - name: kserve-container
        image: vllm/vllm-openai:v0.3.0
        args:
          - --model=/mnt/models
          - --dtype=float16
          - --max-model-len=4096
          - --gpu-memory-utilization=0.9
          - --trust-remote-code
        env:
          - name: HF_TOKEN
            valueFrom:
              secretKeyRef:
                name: huggingface-secret
                key: token

    nodeSelector:
      nvidia.com/gpu.product: NVIDIA-A100-SXM4-40GB

    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
```

---

## ClusterStorageContainer

Configures storage backends for model artifacts (Hugging Face, S3, etc.).

### Schema Definition

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ClusterStorageContainer
metadata:
  name: string                    # Storage config name
spec:
  container:
    name: string                  # Container name (e.g., "storage-initializer")
    image: string                 # Downloader image
    resources:
      requests:
        cpu: string
        memory: string
      limits:
        cpu: string
        memory: string

  supportedUriFormats:
    - regex: string               # URI pattern (e.g., "hf://.*")
      prefix: [string]            # Supported prefixes

  # Optional: credentials for private repos
  env:
    - name: string
      valueFrom:
        secretKeyRef:
          name: string
          key: string
```

### Example: Hugging Face Storage

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ClusterStorageContainer
metadata:
  name: huggingface
spec:
  container:
    name: storage-initializer
    image: kserve/storage-initializer:v0.11.0
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"
      limits:
        cpu: "1"
        memory: "1Gi"

  supportedUriFormats:
    - regex: "hf://.*"
      prefix:
        - hf://

  env:
    - name: HF_TOKEN
      valueFrom:
        secretKeyRef:
          name: huggingface-secret
          key: token
```

### Example: S3 Storage

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ClusterStorageContainer
metadata:
  name: s3-storage
spec:
  container:
    name: storage-initializer
    image: kserve/storage-initializer:v0.11.0
    resources:
      requests:
        cpu: "100m"
        memory: "256Mi"

  supportedUriFormats:
    - regex: "s3://.*"
      prefix:
        - s3://

  env:
    - name: AWS_ACCESS_KEY_ID
      valueFrom:
        secretKeyRef:
          name: s3-credentials
          key: access_key
    - name: AWS_SECRET_ACCESS_KEY
      valueFrom:
        secretKeyRef:
          name: s3-credentials
          key: secret_key
    - name: AWS_REGION
      value: us-east-1
```

---

## ServingRuntime

Custom runtime definitions for specialized model serving configurations.

### Schema Definition

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: string                    # Runtime name
  namespace: string               # Optional: namespace-scoped
spec:
  supportedModelFormats:
    - name: string                # Model format (e.g., "vllm")
      version: string             # Optional: version constraint
      autoSelect: boolean         # Auto-select for this format

  protocolVersions:
    - string                      # Supported protocols (v1, v2)

  containers:
    - name: string
      image: string
      command: [string]
      args: [string]
      env:
        - name: string
          value: string
      resources:
        requests:
          cpu: string
          memory: string
        limits:
          cpu: string
          memory: string

  # Optional: built-in adapter config
  builtInAdapter:
    serverType: string            # "triton", "mlserver", etc.
    runtimeManagementPort: integer
    memBufferBytes: integer
    modelLoadingTimeoutSeconds: integer
```

### Example: Custom vLLM Runtime

```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ServingRuntime
metadata:
  name: vllm-runtime-custom
spec:
  supportedModelFormats:
    - name: vllm
      autoSelect: true

  protocolVersions:
    - v2

  containers:
    - name: kserve-container
      image: vllm/vllm-openai:v0.3.0
      command:
        - python
        - -m
        - vllm.entrypoints.openai.api_server
      args:
        - --host=0.0.0.0
        - --port=8080
        - --model=/mnt/models
        - --served-model-name={{.Name}}
        - --dtype=auto
        - --max-model-len=4096
      env:
        - name: HF_HOME
          value: /tmp/hf_home
      resources:
        requests:
          cpu: "2"
          memory: "8Gi"
        limits:
          cpu: "8"
          memory: "32Gi"

  builtInAdapter:
    serverType: vllm
    runtimeManagementPort: 8080
    memBufferBytes: 134217728     # 128MB
    modelLoadingTimeoutSeconds: 1200  # 20 minutes
```

---

## Knative Service (Generated)

KServe automatically creates Knative Services. This is the generated resource (not user-defined).

### Schema Definition (Read-Only)

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: string                    # Generated: {inferenceservice-name}-predictor
  namespace: string
  ownerReferences:
    - apiVersion: serving.kserve.io/v1beta1
      kind: InferenceService
      name: string
      uid: string
      controller: true
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/target: string
        autoscaling.knative.dev/metric: string
        autoscaling.knative.dev/class: string
    spec:
      containers:
        - name: kserve-container
          image: string
          ports:
            - containerPort: 8080
              protocol: TCP
          resources:
            requests: {}
            limits: {}

      # Knative-specific configuration
      timeoutSeconds: integer     # Request timeout
      containerConcurrency: integer  # Max concurrent requests per pod

status:
  url: string                     # Service URL
  latestReadyRevisionName: string
  latestCreatedRevisionName: string
  traffic:
    - revisionName: string
      percent: integer
      latestRevision: boolean
```

---

## Model Registry Schema

### Current Schema (PostgreSQL)

The existing custom model registry stores model metadata in PostgreSQL.

```sql
-- Current models table
CREATE TABLE models (
    model_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_name VARCHAR(255) NOT NULL UNIQUE,
    model_version VARCHAR(50) NOT NULL,
    model_type VARCHAR(50),              -- "llm", "embedding", etc.
    framework VARCHAR(50),               -- "vllm", "pytorch", etc.
    storage_uri TEXT,                    -- "hf://..." or "s3://..."
    backend_type VARCHAR(50),            -- "legacy_helm" (current), "kserve" (future)
    deployment_config JSONB,             -- Helm values or InferenceService spec
    status VARCHAR(50) DEFAULT 'active', -- "active", "deprecated", "archived"
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(255),
    tags JSONB                           -- Arbitrary key-value metadata
);

-- Index for fast lookups
CREATE INDEX idx_models_name ON models(model_name);
CREATE INDEX idx_models_status ON models(status);
```

### Future Schema (MLflow Model Registry)

If migrating to MLflow, models will be stored in MLflow's schema.

```python
# MLflow Model Registry structure (conceptual)
{
    "name": "llama-2-7b",
    "latest_versions": [
        {
            "version": "1",
            "stage": "Production",
            "source": "hf://meta-llama/Llama-2-7b-hf",
            "run_id": "abc123",
            "status": "READY",
            "creation_timestamp": 1706380800000,
            "tags": {
                "framework": "vllm",
                "gpu_required": "true",
                "deployment_type": "kserve"
            }
        }
    ],
    "tags": {
        "task": "text-generation",
        "license": "llama2"
    }
}
```

### Migration Mapping

| Current Field | MLflow Equivalent | Notes |
|:--------------|:------------------|:------|
| `model_name` | `name` | Direct mapping |
| `model_version` | `version` | Direct mapping |
| `storage_uri` | `source` | Direct mapping |
| `status` | `stage` | Map: active→Production, deprecated→Archived |
| `deployment_config` | `tags.deployment_config` | Store as JSON string in tags |
| `tags` | `tags` | Merge with MLflow tags |
| `backend_type` | `tags.backend_type` | Store as tag |

---

## API Router Backend Configuration

Configuration for api-router-service to route to KServe endpoints.

### Current Backend Config (Custom vLLM)

```yaml
# values-development.yaml (current)
backends:
  - name: llama-2-7b
    type: vllm
    endpoint: http://llama-2-7b-vllm.system.svc.cluster.local:8000
    protocol: openai            # OpenAI-compatible API
    healthCheckPath: /health
    timeout: 30s
```

### New Backend Config (KServe)

```yaml
# values-development.yaml (new)
backends:
  - name: llama-2-7b
    type: kserve
    endpoint: http://llama-2-7b-predictor.development.svc.cluster.local/v2/models/llama-2-7b/infer
    protocol: kserve-v2         # KServe V2 Inference Protocol
    protocolAdapter: openai     # Translate OpenAI → KServe V2
    healthCheckPath: /v2/health/ready
    timeout: 30s

    # Knative-specific
    knativeService: true        # Expect cold-starts
    coldStartTimeout: 60s       # Wait longer for cold-start
```

### Protocol Translation Map

```yaml
# OpenAI to KServe V2 mapping
protocolTranslation:
  openai:
    endpoint: /v1/chat/completions
    requestFormat:
      model: string
      messages: array
      temperature: float
      max_tokens: integer
    responseFormat:
      id: string
      object: string
      created: integer
      choices: array

  kserveV2:
    endpoint: /v2/models/{model}/infer
    requestFormat:
      id: string                # Generated UUID
      inputs:
        - name: "prompt"
          shape: [1]
          datatype: "BYTES"
          data: [string]        # Serialized messages
      parameters:
        temperature: float
        max_tokens: integer
    responseFormat:
      id: string
      model_name: string
      outputs:
        - name: "text"
          shape: [1]
          datatype: "BYTES"
          data: [string]        # Completion text
```

---

## Autoscaling Configuration

### Knative Pod Autoscaler (KPA) Annotations

```yaml
# Annotations on InferenceService predictor template
metadata:
  annotations:
    # Autoscaling class
    autoscaling.knative.dev/class: "kpa.autoscaling.knative.dev"

    # Target metric
    autoscaling.knative.dev/metric: "concurrency"  # or "rps"

    # Target value (requests per pod)
    autoscaling.knative.dev/target: "5"

    # Min/Max replicas
    autoscaling.knative.dev/minScale: "1"
    autoscaling.knative.dev/maxScale: "10"

    # Scale-to-zero configuration
    autoscaling.knative.dev/scaleToZeroGracePeriod: "30s"
    autoscaling.knative.dev/scaleDownDelay: "5m"

    # Panic mode (rapid scale-up)
    autoscaling.knative.dev/panicWindowPercentage: "10.0"
    autoscaling.knative.dev/panicThresholdPercentage: "200.0"

    # Window for scaling decisions
    autoscaling.knative.dev/window: "60s"
```

### Horizontal Pod Autoscaler (HPA) Alternative

For RawDeployment mode (non-serverless):

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: llama-2-7b-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: llama-2-7b-predictor
  minReplicas: 1
  maxReplicas: 10
  metrics:
    - type: Pods
      pods:
        metric:
          name: kserve_request_count
        target:
          type: AverageValue
          averageValue: "5"
  behavior:
    scaleDown:
      stabilizationWindowSeconds: 300
      policies:
        - type: Percent
          value: 50
          periodSeconds: 60
    scaleUp:
      stabilizationWindowSeconds: 0
      policies:
        - type: Percent
          value: 100
          periodSeconds: 15
```

---

## Metrics Data Model

### Prometheus Metrics (KServe + Knative)

```yaml
# KServe InferenceService status
kserve_inference_service_status{
  name: "llama-2-7b",
  namespace: "development",
  status: "Ready"
} 1

# Knative autoscaler metrics
knative_serving_autoscaler_actual_pods{
  revision_name: "llama-2-7b-predictor-v1",
  namespace: "development"
} 3

knative_serving_autoscaler_desired_pods{
  revision_name: "llama-2-7b-predictor-v1",
  namespace: "development"
} 5

knative_serving_revision_request_count{
  revision_name: "llama-2-7b-predictor-v1",
  namespace: "development",
  response_code: "200"
} 1523

knative_serving_revision_request_latencies_bucket{
  revision_name: "llama-2-7b-predictor-v1",
  namespace: "development",
  le: "1000"  # milliseconds
} 1200

# Istio metrics (if using service mesh)
istio_requests_total{
  destination_service: "llama-2-7b-predictor.development.svc.cluster.local",
  response_code: "200"
} 1523

istio_request_duration_milliseconds{
  destination_service: "llama-2-7b-predictor.development.svc.cluster.local",
  quantile: "0.95"
} 1250
```

### Custom vLLM Metrics (via Sidecar)

```yaml
# vLLM-specific metrics (requires custom exporter)
vllm_num_requests_running{
  model: "llama-2-7b",
  pod: "llama-2-7b-predictor-v1-abc123"
} 3

vllm_num_requests_waiting{
  model: "llama-2-7b",
  pod: "llama-2-7b-predictor-v1-abc123"
} 2

vllm_gpu_cache_usage_perc{
  model: "llama-2-7b",
  pod: "llama-2-7b-predictor-v1-abc123"
} 0.75

vllm_time_to_first_token_seconds{
  model: "llama-2-7b",
  pod: "llama-2-7b-predictor-v1-abc123",
  quantile: "0.95"
} 0.85

vllm_time_per_output_token_seconds{
  model: "llama-2-7b",
  pod: "llama-2-7b-predictor-v1-abc123",
  quantile: "0.95"
} 0.045
```

---

## Summary of Key Data Structures

| Resource | Purpose | User-Defined | Generated |
|:---------|:--------|:-------------|:----------|
| **InferenceService** | Primary model deployment resource | ✅ Yes | Status fields |
| **ClusterStorageContainer** | Storage backend configuration | ✅ Yes | - |
| **ServingRuntime** | Custom runtime definitions | ✅ Yes (optional) | - |
| **Knative Service** | Serverless serving infrastructure | ❌ No | ✅ By KServe |
| **Knative Revision** | Immutable deployment version | ❌ No | ✅ By Knative |
| **Istio VirtualService** | Traffic routing rules | ❌ No | ✅ By KServe |
| **HPA** | Horizontal autoscaling (RawDeployment) | ✅ Yes (optional) | - |
| **Model Registry Entry** | Model metadata and lineage | ✅ Yes (via API) | - |

---

## Data Flow Summary

1. **User creates InferenceService** → KServe Controller validates and reconciles
2. **KServe creates Knative Service** → Knative Serving creates initial Revision
3. **Knative creates Deployment** → Kubernetes schedules Pods on GPU nodes
4. **Storage Initializer runs** → Downloads model from storageUri using ClusterStorageContainer
5. **vLLM container starts** → Loads model, exposes inference endpoint
6. **Knative Activator** → Routes traffic, triggers scale-from-zero if needed
7. **Istio Ingress** → Handles external traffic, applies VirtualService rules
8. **API Router Service** → Translates OpenAI → KServe V2, forwards to predictor
9. **Metrics exported** → Prometheus scrapes KServe, Knative, Istio, vLLM metrics
10. **Grafana dashboards** → Visualize performance, autoscaling, costs

---

## References

- [KServe API Reference](https://kserve.github.io/website/latest/reference/api/)
- [Knative Serving API](https://knative.dev/docs/serving/spec/knative-api-specification-1.0/)
- [KServe V2 Inference Protocol](https://github.com/kserve/kserve/blob/master/docs/predict-api/v2/required_api.md)
- [MLflow Model Registry](https://mlflow.org/docs/latest/model-registry.html)
