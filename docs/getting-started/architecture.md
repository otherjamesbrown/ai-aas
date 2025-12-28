# Platform Architecture

This document provides a comprehensive overview of the AI-AAS platform architecture, including how models are deployed, how requests flow through the system, and the role of each component.

## System Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Your Application                                │
│                         (Python, Node.js, curl, etc.)                        │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      │ HTTPS + API Key
                                      ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                            NGINX Ingress Controller                          │
│                      (TLS termination, host-based routing)                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                    ┌─────────────────┴─────────────────┐
                    ▼                                   ▼
┌───────────────────────────────┐     ┌───────────────────────────────────────┐
│       API Router Service       │     │         Admin API Service              │
│   (Inference Gateway)          │     │    (Platform Management)               │
│                                │     │                                        │
│  • Validate API keys           │     │  • Model registry CRUD                 │
│  • Rate limiting               │     │  • Deployment management               │
│  • Route to model backends     │     │  • Credentials storage                 │
│  • Usage event publishing      │     │  • Routing policies                    │
└───────────────────────────────┘     └───────────────────────────────────────┘
          │         │                              │
          │         │                              │
    ┌─────┘         └────────┐                     │
    ▼                        ▼                     ▼
┌────────────┐    ┌─────────────────┐    ┌─────────────────────┐
│   Redis    │    │ User-Org Service│    │   PostgreSQL        │
│            │    │                 │    │                     │
│ • Caching  │    │ • Organizations │    │ • Model registry    │
│ • Rate     │    │ • Users         │    │ • Users/Orgs        │
│   limits   │    │ • API keys      │    │ • Usage data        │
└────────────┘    │ • RBAC          │    │ • Audit logs        │
                  └─────────────────┘    └─────────────────────┘
          │
          │ Forward inference request (via Protocol Adapters)
          ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                        Model Inference Layer                                 │
│                                                                             │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐     │
│  │  gpt-oss-20b │  │  llama-3-8b  │  │  mistral-7b  │  │ codellama-34b│     │
│  │              │  │              │  │              │  │              │     │
│  │    vLLM      │  │ TensorRT-LLM │  │    vLLM      │  │   Triton     │     │
│  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │  │  ┌────────┐  │     │
│  │  │ OpenAI │  │  │  │ Triton │  │  │  │ OpenAI │  │  │  │ gRPC   │  │     │
│  │  │  API   │  │  │  │  V2    │  │  │  │  API   │  │  │  │  V2    │  │     │
│  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │  │  └────────┘  │     │
│  │   RTX 6000   │  │   A100 80GB  │  │   RTX 4000   │  │   A100 40GB  │     │
│  └──────────────┘  └──────────────┘  └──────────────┘  └──────────────┘     │
│                                                                             │
│  Runtimes: vLLM (OpenAI-native) │ TensorRT-LLM (Triton) │ Triton (gRPC/HTTP)│
└─────────────────────────────────────────────────────────────────────────────┘
          │
          │ Usage events (async)
          ▼
┌───────────────────────────────┐     ┌───────────────────────────────────────┐
│          Kafka                │────▶│       Analytics Service                │
│    (Message Queue)            │     │  • Consume usage events                │
│                               │     │  • Aggregate metrics                   │
│  • Usage events               │     │  • Generate billing data               │
│  • Async processing           │     │  • Usage reports                       │
└───────────────────────────────┘     └───────────────────────────────────────┘
```

## Core Services

### API Router Service
**Purpose**: Gateway for all inference requests

The API Router is the entry point for all AI inference calls. It:

1. **Authenticates** requests by validating API keys against the User-Org Service
2. **Applies rate limits** using Redis-backed token bucket algorithm
3. **Routes requests** to the appropriate model backend based on the model name
4. **Forwards** the request to vLLM and returns the response
5. **Publishes usage events** to Kafka for analytics

```
Client Request → Auth Check → Rate Limit Check → Model Lookup → Forward to vLLM → Response
                     ↓              ↓                 ↓              ↓
              User-Org Service   Redis           Registry DB      Kafka (async)
```

### User-Org Service
**Purpose**: Identity and access management

Manages the multi-tenant hierarchy:

```
Organization
    ├── Users
    │     └── API Keys
    └── Model Access Policies
```

- **Organizations**: Top-level tenants with isolated resources
- **Users**: Members of an organization
- **API Keys**: Credentials issued to users for API access
- **RBAC**: Role-based access control for platform operations

### Admin API Service
**Purpose**: Platform administration

Used by the CLI and admin UI for:

- Model registry management (add/remove/update models)
- Deployment operations (create/scale/delete)
- Credential storage (HuggingFace tokens, S3 keys)
- Routing policy configuration

### Analytics Service
**Purpose**: Usage tracking and billing

Consumes events from Kafka and:

- Aggregates token usage by org/user/model
- Calculates costs based on pricing tiers
- Generates usage reports
- Provides data for the usage CLI commands

## Protocol Adapter Layer

The API Router exposes an **OpenAI-compatible API** to clients, but backend inference engines use different protocols. The adapter layer handles bidirectional translation.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                            API Router Service                                │
│                                                                             │
│  ┌───────────────┐     ┌─────────────────────────────────────────────────┐ │
│  │   OpenAI API  │     │            Protocol Adapter Layer               │ │
│  │   Handler     │────▶│                                                 │ │
│  │               │     │  ┌─────────────┐  ┌─────────────┐  ┌─────────┐ │ │
│  │ POST /v1/chat │     │  │   Triton    │  │   KServe    │  │  vLLM   │ │ │
│  │ /completions  │     │  │  Adapter    │  │  Adapter    │  │ Adapter │ │ │
│  │               │     │  │             │  │             │  │         │ │ │
│  └───────────────┘     │  │ TensorRT-LLM│  │  KServe V2  │  │ OpenAI  │ │ │
│                        │  │   format    │  │   format    │  │ native  │ │ │
│                        │  └──────┬──────┘  └──────┬──────┘  └────┬────┘ │ │
│                        └─────────┼───────────────┼───────────────┼──────┘ │
│                                  │               │               │        │
└──────────────────────────────────┼───────────────┼───────────────┼────────┘
                                   ▼               ▼               ▼
                            ┌──────────┐    ┌──────────┐    ┌──────────┐
                            │ TensorRT │    │  KServe  │    │   vLLM   │
                            │   -LLM   │    │  Model   │    │  OpenAI  │
                            │ Backend  │    │ Backend  │    │  Server  │
                            └──────────┘    └──────────┘    └──────────┘
```

### Supported Backend Protocols

| Backend | Protocol | Adapter | Notes |
|---------|----------|---------|-------|
| **vLLM** | OpenAI-native | Passthrough | vLLM natively speaks OpenAI format |
| **KServe** | KServe V2 | `kserve.Translator` | Translates to/from V2 inference protocol |
| **Triton** | Triton V2 | `triton.Translator` | For TensorRT-LLM with gRPC/HTTP |

### Request Translation Example

When a client sends an OpenAI request to a Triton/TensorRT-LLM backend:

```
Client Request (OpenAI format):
{
  "model": "llama-7b-trt",
  "messages": [{"role": "user", "content": "Hello"}],
  "max_tokens": 50,
  "temperature": 0.7
}
        │
        ▼ Triton Adapter translates

Triton V2 Request:
{
  "id": "req-uuid",
  "inputs": [
    {"name": "text_input", "shape": [1], "datatype": "BYTES", "data": ["Hello"]},
    {"name": "max_tokens", "shape": [1], "datatype": "INT32", "data": [50]},
    {"name": "temperature", "shape": [1], "datatype": "FP32", "data": [0.7]}
  ]
}
        │
        ▼ Backend responds

Triton V2 Response:
{
  "outputs": [{"name": "text_output", "data": ["Hi! How can I help?"]}]
}
        │
        ▼ Triton Adapter translates back

Client Response (OpenAI format):
{
  "choices": [{"message": {"role": "assistant", "content": "Hi! How can I help?"}}],
  "usage": {"prompt_tokens": 5, "completion_tokens": 8, "total_tokens": 13}
}
```

### Protocol Mapping

| OpenAI Field | Triton V2 Tensor | KServe V2 Field |
|--------------|------------------|-----------------|
| `messages` | `text_input` | `inputs[0].data` |
| `max_tokens` | `max_tokens` | `parameters.max_tokens` |
| `temperature` | `temperature` | `parameters.temperature` |
| `top_p` | `top_p` | `parameters.top_p` |
| Response text | `text_output` | `outputs[0].data` |

### Why This Matters

1. **Unified Client Experience**: All clients use the familiar OpenAI SDK/format
2. **Backend Flexibility**: Platform can run vLLM, TensorRT-LLM, or other engines
3. **Transparent Optimization**: Switch backends without client changes
4. **Future-Proof**: Add new engines by writing adapters

## Multi-Inference Engine Architecture

The platform supports running multiple AI models simultaneously, each as an independent inference engine.

### How Models Are Deployed

```
┌──────────────────────────────────────────────────────────────────────────┐
│                           GitOps Workflow                                 │
│                                                                          │
│  1. AIModel CR     2. ArgoCD         3. Operator         4. KServe       │
│     in Git            Syncs             Watches             Creates      │
│        │                │                  │                   │         │
│        ▼                ▼                  ▼                   ▼         │
│  ┌──────────┐    ┌──────────────┐   ┌──────────────┐   ┌──────────────┐ │
│  │ aimodel/ │───▶│   ArgoCD     │──▶│  AI-Model    │──▶│ Inference-   │ │
│  │  gpt-oss │    │              │   │  Operator    │   │ Service      │ │
│  │  -20b.   │    │ (GitOps)     │   │              │   │              │ │
│  │  yaml    │    └──────────────┘   └──────────────┘   └──────────────┘ │
│  └──────────┘                              │                   │         │
│                                            │                   │         │
│                                            ▼                   ▼         │
│                                   ┌──────────────┐   ┌──────────────┐   │
│                                   │ Model        │   │ vLLM Pod     │   │
│                                   │ Registry     │   │ (GPU)        │   │
│                                   │ Entry        │   │              │   │
│                                   └──────────────┘   └──────────────┘   │
└──────────────────────────────────────────────────────────────────────────┘
```

### AIModel Custom Resource

Models are defined as Kubernetes Custom Resources:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: gpt-oss-20b
  namespace: system
spec:
  # Model identity
  modelName: "gpt-oss-20b"
  modelID: "unsloth/gpt-oss-20b"       # HuggingFace model ID

  # Runtime configuration
  runtime: "vllm"                       # Inference runtime (vllm, triton, tgi)
  enabled: true                         # Enable/disable deployment

  # Scaling
  minReplicas: 1
  maxReplicas: 3

  # Resource requirements
  resources:
    requests:
      nvidia.com/gpu: "1"
      memory: "24Gi"
    limits:
      nvidia.com/gpu: "1"
      memory: "48Gi"
```

### AI Model Operator

The AI Model Operator is a Kubernetes controller that watches AIModel resources and manages their lifecycle:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        AI Model Operator                                 │
│                                                                         │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐     │
│  │   Watch Loop    │───▶│   Reconciler    │───▶│  Status Update  │     │
│  │                 │    │                 │    │                 │     │
│  │ • AIModel CRs   │    │ • Create ISVC   │    │ • Phase         │     │
│  │ • Events        │    │ • Update ISVC   │    │ • Endpoint      │     │
│  │ • Status        │    │ • Delete ISVC   │    │ • Ready         │     │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘     │
│                                │                                        │
│                                ▼                                        │
│                    ┌─────────────────────────┐                         │
│                    │  Registry Sync          │                         │
│                    │                         │                         │
│                    │ • Update model registry │                         │
│                    │ • Set inference endpoint│                         │
│                    │ • Mark as ready         │                         │
│                    └─────────────────────────┘                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Operator responsibilities**:

1. **Watch** for AIModel custom resources
2. **Create** KServe InferenceService for each model
3. **Configure** vLLM with appropriate GPU resources
4. **Monitor** health and update status
5. **Register** healthy models in the registry
6. **Scale** replicas based on demand

### KServe Integration

KServe manages the actual inference workloads:

```
AIModel CR
     │
     ▼
InferenceService (KServe)
     │
     ├── Predictor (vLLM container)
     │     ├── GPU allocation
     │     ├── Model loading
     │     └── Inference serving
     │
     └── Autoscaler
           ├── Scale based on queue depth
           └── Scale to zero when idle
```

### Why RawDeployment Instead of Serverless

KServe supports two deployment modes. We use **RawDeployment for GPU workloads** due to Knative Serving limitations:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    KServe Deployment Modes                                   │
│                                                                             │
│  ┌─────────────────────────────────┐  ┌─────────────────────────────────┐  │
│  │      Serverless (Knative)       │  │        RawDeployment            │  │
│  │                                 │  │                                 │  │
│  │  ✓ Scale to zero               │  │  ✓ nodeSelector for GPU nodes  │  │
│  │  ✓ Request-based autoscaling   │  │  ✓ Custom health probes        │  │
│  │  ✓ Traffic splitting           │  │  ✓ Multiple container ports    │  │
│  │                                 │  │  ✓ startupProbe support        │  │
│  │  ✗ No nodeSelector allowed     │  │                                 │  │
│  │  ✗ Single port only            │  │  ✗ No scale to zero            │  │
│  │  ✗ startupProbe removed        │  │  ✗ Manual autoscaling (HPA)    │  │
│  │  ✗ Probe timing overridden     │  │                                 │  │
│  └─────────────────────────────────┘  └─────────────────────────────────┘  │
│                                                                             │
│           Best for: CPU-only, stateless         Best for: GPU workloads    │
└─────────────────────────────────────────────────────────────────────────────┘
```

**The Problem with Serverless for GPUs**:

1. **No nodeSelector**: Knative rejects pods with `nodeSelector`, but GPU workloads must target GPU nodes
2. **Probe issues**: Knative removes `startupProbe` and overrides liveness probe timing. Large models (20B+ parameters) need 5-10 minutes to load - without proper probes, Kubernetes kills them prematurely
3. **Port restrictions**: Knative requires specific port naming (`h2c`, `http1`) and only allows one port

**Deployment Mode by Runtime**:

| Runtime | Has GPU | Default Mode | Reason |
|---------|---------|--------------|--------|
| `tensorrt-llm` | Always | RawDeployment | Requires nodeSelector, custom probes |
| `triton` | Always | RawDeployment | Multi-port, nodeSelector |
| `vllm` | Yes | RawDeployment | nodeSelector for GPU nodes |
| `vllm` | No | Serverless | Can scale to zero for cost savings |
| `tgi` | Yes | RawDeployment | nodeSelector for GPU nodes |

**Example AIModel with explicit deployment mode**:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-7b-trt
spec:
  modelName: "llama-7b-trt"
  runtime: "tensorrt-llm"
  deploymentMode: "RawDeployment"  # Explicit - required for TensorRT-LLM

  # These would be rejected by Knative Serverless:
  nodeSelector:
    nvidia.com/gpu.product: "NVIDIA-RTX-A6000"
  tolerations:
    - key: "nvidia.com/gpu"
      operator: "Exists"
      effect: "NoSchedule"
```

### Multiple Models Example

With multiple models deployed using different runtimes:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                                │
│                                                                         │
│  Namespace: system                                                      │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                                                                  │   │
│  │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐          │   │
│  │  │  gpt-oss-20b │  │ llama-3-8b   │  │  mistral-7b  │          │   │
│  │  │              │  │   -instruct  │  │              │          │   │
│  │  │  Runtime:    │  │  Runtime:    │  │  Runtime:    │          │   │
│  │  │    vLLM      │  │ TensorRT-LLM │  │    vLLM      │          │   │
│  │  │              │  │              │  │              │          │   │
│  │  │  Protocol:   │  │  Protocol:   │  │  Protocol:   │          │   │
│  │  │  OpenAI API  │  │  Triton V2   │  │  OpenAI API  │          │   │
│  │  │              │  │              │  │              │          │   │
│  │  │  GPU: RTX6000│  │  GPU: A100   │  │  GPU: RTX4000│          │   │
│  │  │  Mode: Raw   │  │  Mode: Raw   │  │  Mode: Raw   │          │   │
│  │  └──────────────┘  └──────────────┘  └──────────────┘          │   │
│  │         ▲                 ▲                 ▲                   │   │
│  │         │                 │                 │                   │   │
│  │         └─────────────────┼─────────────────┘                   │   │
│  │                           │                                      │   │
│  │                    ┌──────────────┐                             │   │
│  │                    │ AI-Model     │                             │   │
│  │                    │ Operator     │                             │   │
│  │                    │              │                             │   │
│  │                    │ • Creates    │                             │   │
│  │                    │   ISVCs      │                             │   │
│  │                    │ • Selects    │                             │   │
│  │                    │   runtime    │                             │   │
│  │                    │ • Configures │                             │   │
│  │                    │   probes     │                             │   │
│  │                    └──────────────┘                             │   │
│  │                                                                  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Request Flow (Detailed)

Here's exactly what happens when you make an API request:

```
1. Client sends POST /v1/chat/completions
   └── Headers: X-API-Key: sk-xxx, Content-Type: application/json
   └── Body: {"model": "gpt-oss-20b", "messages": [...]}

2. NGINX Ingress receives request
   └── TLS termination
   └── Route to api-router-service based on host header

3. API Router validates API key
   └── Check Redis cache first (TTL: 2 min)
   └── If miss, call User-Org Service
   └── Cache the result

4. API Router checks rate limits
   └── Redis token bucket per API key
   └── Return 429 if exceeded

5. API Router looks up model endpoint
   └── Check Redis cache first
   └── If miss, query PostgreSQL model registry
   └── Get internal endpoint: gpt-oss-20b.system.svc.cluster.local:8000

6. API Router forwards request to vLLM
   └── HTTP POST to internal endpoint
   └── Stream response back to client

7. API Router publishes usage event
   └── Async write to Kafka
   └── Includes: org_id, user_id, model, tokens_used

8. Analytics Service consumes event
   └── Aggregate into usage tables
   └── Available via CLI: ai-aas-cli usage query
```

## GitOps Deployment Model

All changes flow through Git:

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   develop   │────▶│   staging   │────▶│    main     │
│   branch    │     │   branch    │     │   branch    │
└─────────────┘     └─────────────┘     └─────────────┘
       │                   │                   │
       ▼                   ▼                   ▼
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│ Development │     │   Staging   │     │ Production  │
│   Cluster   │     │   Cluster   │     │   Cluster   │
│             │     │             │     │             │
│ Auto-sync   │     │ Auto-sync   │     │ Manual sync │
└─────────────┘     └─────────────┘     └─────────────┘
```

**Branch promotion workflow**:
1. Develop in `develop` branch → auto-deploys to development
2. PR to `staging` branch → auto-deploys to staging for testing
3. PR to `main` branch → requires manual approval for production

## Scaling & High Availability

### Model Scaling

Each model can scale independently:

```yaml
spec:
  minReplicas: 1      # Scale to zero when idle
  maxReplicas: 10     # Max replicas during load
  scaleMetric: "concurrency"  # Scale based on in-flight requests
  scaleTarget: 10     # Target 10 concurrent requests per replica
```

### Component Redundancy

| Component | Replicas | Notes |
|-----------|----------|-------|
| API Router | 2-3 | Stateless, load balanced |
| User-Org Service | 2 | Stateless |
| Admin API | 2 | Stateless |
| Analytics | 2 | Stateless, Kafka ensures ordering |
| PostgreSQL | 1 (HA in prod) | Primary + replica in production |
| Redis | 1 (HA in prod) | Cluster mode in production |

## Security Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Security Layers                                  │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 1. Edge Security                                                 │   │
│  │    • TLS 1.2+ for all traffic                                   │   │
│  │    • DDoS protection (Cloudflare/provider)                      │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 2. Authentication                                                │   │
│  │    • API key validation on every request                        │   │
│  │    • Keys scoped to organization                                │   │
│  │    • Optional key expiration                                    │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 3. Authorization                                                 │   │
│  │    • Model access per organization                              │   │
│  │    • Rate limits per API key                                    │   │
│  │    • RBAC for admin operations                                  │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │ 4. Network Security                                              │   │
│  │    • Internal services not exposed externally                   │   │
│  │    • mTLS between services (Istio)                              │   │
│  │    • Network policies limiting pod communication                │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

## Observability Stack

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Observability                                    │
│                                                                         │
│  ┌─────────────┐   ┌─────────────┐   ┌─────────────┐   ┌─────────────┐ │
│  │ Prometheus  │   │    Loki     │   │   Grafana   │   │ AlertManager│ │
│  │             │   │             │   │             │   │             │ │
│  │ • Metrics   │   │ • Logs      │   │ • Dashboards│   │ • Alerts    │ │
│  │ • 30d       │   │ • Queries   │   │ • Explore   │   │ • PagerDuty │ │
│  │   retention │   │             │   │             │   │ • Slack     │ │
│  └─────────────┘   └─────────────┘   └─────────────┘   └─────────────┘ │
│         ▲                 ▲                                             │
│         │                 │                                             │
│    ┌────┴────┐      ┌────┴────┐                                        │
│    │ /metrics│      │  stdout │                                        │
│    └─────────┘      └─────────┘                                        │
│         │                 │                                             │
│  ┌──────┴─────────────────┴──────┐                                     │
│  │         All Services          │                                     │
│  │  (API Router, vLLM, Operator) │                                     │
│  └───────────────────────────────┘                                     │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

**Key dashboards**:
- Service Logs: Aggregated logs from all services
- Request Tracing: Trace requests across services by trace_id
- Inference Backends: vLLM metrics (latency, throughput, GPU utilization)

## Summary

| Layer | Components | Technology |
|-------|------------|------------|
| **Gateway** | API Router | Go, OpenAI-compatible API |
| **Auth** | User-Org Service | Go, PostgreSQL |
| **Admin** | Admin API, CLI | Go, PostgreSQL |
| **Inference** | vLLM pods | Python, CUDA, KServe |
| **Orchestration** | AI Model Operator | Go, Kubernetes, KServe |
| **Analytics** | Analytics Service | Go, Kafka, PostgreSQL |
| **GitOps** | ArgoCD | Kubernetes manifests, Helm |
| **Observability** | Prometheus, Loki, Grafana | Standard stack |
