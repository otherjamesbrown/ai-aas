# Architecture

This document provides a high-level overview of the AI-as-a-Service platform architecture.

## System Overview

The platform is designed as a set of cooperating microservices. The central component is the **API Router Service**, which acts as the public-facing gateway for all AI model inference requests. It is responsible for authentication, authorization, rate limiting, budget enforcement, and routing requests to the appropriate backend model services.

Other key services include:

*   **User & Organization Service**: Manages users, organizations, and API keys.
*   **Budget Service**: Tracks and enforces spending budgets for organizations.
*   **Analytics Service**: Collects and processes usage data for billing and analysis.
*   **Admin CLI**: A command-line tool for administering the platform.
*   **AI Model Operator**: Kubernetes operator that manages the lifecycle of AI model deployments via AIModel Custom Resources.
*   **vLLM Deployments**: GPU-accelerated model inference engines managed by the AI Model Operator and registered in the model registry.

The system also relies on several backing services:

*   **Redis**: Used for caching, rate limiting, and model registry caching.
*   **Kafka**: Used for asynchronous messaging, particularly for usage data.
*   **PostgreSQL**: The primary database for services like the User & Organization Service, Budget Service, and Model Registry.

## Service Interaction Diagram

The following diagram illustrates the primary request flow and service interactions:

```
+-----------------+      +----------------------+      +--------------------+
|                 |----->| User & Org Service   |----->|      Database      |
| API Key Auth    |      | (Authentication)     |      |     (PostgreSQL)   |
+-----------------+      +----------------------+      +--------------------+
       ^
       |
+------+----------+      +----------------------+
|                 |----->|    Budget Service    |
| API Router      |      | (Budget Enforcement) |
| (Gateway)       |      +----------------------+
|                 |
+------+-+-+------+      +----------------------+
       | | |             |   Analytics Service  |
       | | +------------>| (Usage Tracking)     |
       | |               +----------------------+
       | |                      ^
       | |                      |
       | +----------------------V--------------------+
       |                        |                     |
       |                      +---+                   |
       +--------------------->|   |                   |
                              |AI |<------------------+
       +--------------------->|   |
       |                      |   |
       |                      +---+
       |                        |
       +--------------------->|...| (etc.)
                              +---+
                         Backend Model
                            Services

                              ^
                              |  (creates/manages)
                              |
                    +--------------------+
                    |  AI Model Operator |
                    |  (Watches AIModel  |
                    |   Custom Resources)|
                    +--------------------+
                              ^
                              |
                    +--------------------+
                    |     ArgoCD         |
                    |  (GitOps Sync)     |
                    +--------------------+
```

## AI Model Operator

The **AI Model Operator** is a Kubernetes operator that manages the complete lifecycle of AI model deployments through the `AIModel` Custom Resource Definition (CRD). This is the **recommended approach** for deploying models to the platform.

### AIModel Custom Resource

Models are defined declaratively using `AIModel` Custom Resources:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: mistral-7b-instruct-v03
  namespace: development
spec:
  modelName: mistralai/Mistral-7B-Instruct-v0.3
  source:
    type: huggingface
    huggingface:
      modelId: mistralai/Mistral-7B-Instruct-v0.3
  runtime:
    type: vllm
    vllm:
      args:
        - "--max-model-len=4096"
        - "--dtype=half"
  resources:
    requests:
      nvidia.com/gpu: "1"
      memory: "16Gi"
    limits:
      nvidia.com/gpu: "1"
      memory: "24Gi"
```

### Operator Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                      GitOps Flow                                 │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌────────────┐     ┌────────────┐     ┌─────────────────────┐ │
│  │   Git Repo │────▶│   ArgoCD   │────▶│  AIModel CR         │ │
│  │ (ai-aas or │     │  (Sync)    │     │  (Kubernetes)       │ │
│  │ ai-aas-    │     └────────────┘     └──────────┬──────────┘ │
│  │ config)    │                                    │            │
│  └────────────┘                                    │ (watches)  │
│                                                    ▼            │
│                                        ┌─────────────────────┐ │
│                                        │  AI Model Operator  │ │
│                                        │  (Reconcile Loop)   │ │
│                                        └──────────┬──────────┘ │
│                                                   │             │
│                              ┌────────────────────┼─────────┐  │
│                              │ (creates/manages)  │         │  │
│                              ▼                    ▼         ▼  │
│                    ┌──────────────┐    ┌────────┐  ┌────────┐ │
│                    │  Deployment  │    │Service │  │ConfigMap│ │
│                    │  (vLLM Pod)  │    │        │  │        │ │
│                    └──────────────┘    └────────┘  └────────┘ │
│                                                                │
└─────────────────────────────────────────────────────────────────┘
```

### Key Features

1. **Declarative Model Deployment**: Define models as YAML, let the operator handle the complexity
2. **Automatic vLLM Configuration**: Generates optimal vLLM arguments based on model and GPU specs
3. **Health Monitoring**: Built-in readiness/liveness probes for all deployments
4. **Status Tracking**: `AIModel.status` shows deployment state, inference endpoint, and errors
5. **GitOps Integration**: Works seamlessly with ArgoCD for automated deployments

### Model Configuration Storage

AIModel configurations are stored in two locations:

| Repository | Path | Purpose |
|------------|------|---------|
| `ai-aas` (main) | `infra/k8s/aimodels/{env}/` | Internal platform models |
| `ai-aas-config` (public) | `environments/{env}/models/` | Community-contributed models |

For detailed AIModel reference, see: [docs/operators/ai-model-operator.md](docs/operators/ai-model-operator.md)

## vLLM Deployment Architecture

The platform uses **vLLM** (Very Large Language Model) for high-performance model inference with GPU acceleration. vLLM deployments are managed by the AI Model Operator through AIModel Custom Resources.

### Component Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Control Plane                                 │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌────────────┐        ┌────────────┐        ┌────────────┐        │
│  │  AIModel   │───────▶│   ArgoCD   │───────▶│  Operator  │        │
│  │  CRs (Git) │        │  (GitOps)  │        │ (Reconcile)│        │
│  └────────────┘        └────────────┘        └─────┬──────┘        │
│                                                     │                │
└─────────────────────────────────────────────────────┼────────────────┘
                                                      │
                           ┌──────────────────────────┘
                           │
┌──────────────────────────┴───────────────────────────────────────────┐
│                         Data Plane                                    │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌────────────────────┐          ┌────────────────────┐            │
│  │  Model Registry    │◀─────────│   API Router       │            │
│  │  (PostgreSQL)      │          │   Service          │            │
│  │                    │          │                    │            │
│  │ - Deployment info  │          │ - Registry lookup  │            │
│  │ - Health status    │          │ - Request routing  │            │
│  │ - Endpoints        │          └─────────┬──────────┘            │
│  └─────┬──────────────┘                    │                        │
│        │                                    │                        │
│  ┌─────▼──────────────┐                    │                        │
│  │  Redis Cache       │                    │                        │
│  │  (2-min TTL)       │                    │                        │
│  └────────────────────┘                    │                        │
│                                             │                        │
│         ┌───────────────────────────────────┴──────────┐            │
│         │                                               │            │
│  ┌──────▼──────────┐                      ┌────────────▼─────────┐ │
│  │  vLLM Pod       │                      │  vLLM Pod            │ │
│  │  (Model A)      │                      │  (Model B)           │ │
│  │                 │                      │                      │ │
│  │ ┌─────────────┐ │                      │ ┌─────────────┐     │ │
│  │ │ GPU (NVIDIA)│ │                      │ │ GPU (NVIDIA)│     │ │
│  │ │ - Model     │ │                      │ │ - Model     │     │ │
│  │ │ - Inference │ │                      │ │ - Inference │     │ │
│  │ └─────────────┘ │                      │ └─────────────┘     │ │
│  │                 │                      │                      │ │
│  │ Health: /health │                      │ Health: /health      │ │
│  │ Metrics: /metrics│                     │ Metrics: /metrics    │ │
│  └─────────────────┘                      └──────────────────────┘ │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### Key Components

1. **AIModel Custom Resources** (`infra/k8s/aimodels/{env}/`)
   - Declarative model definitions in Git
   - Environment-specific configurations (development, staging, production)
   - GPU resource specifications
   - Runtime arguments (vLLM, Triton, TGI)

2. **AI Model Operator** (`operators/ai-model-operator/`)
   - Watches AIModel CRs and creates Kubernetes resources
   - Manages Deployments, Services, ConfigMaps
   - Auto-configures health probes and resource limits
   - Handles model downloads from HuggingFace or S3

3. **Model Registry** (PostgreSQL)
   - Tracks deployed models and their endpoints
   - Maintains deployment status and health information
   - Enables dynamic routing without hardcoded endpoints
   - Redis caching layer for performance (2-minute TTL)

3. **Admin CLI Extensions**
   - `admin-cli registry` - Manage model registry entries
   - `admin-cli deployment` - Inspect deployment status
   - Post-deployment registration automation

4. **API Router Integration**
   - Registry-based model lookup
   - Dynamic endpoint resolution
   - Redis caching for reduced database load
   - Graceful fallback on registry failures

5. **GitOps with ArgoCD**
   - Automated sync for development/staging
   - Manual approval for production
   - Rollback capabilities
   - Declarative configuration management

### Deployment Workflow

```
1. Define AIModel CR → 2. Push to Git → 3. ArgoCD Syncs → 4. Operator Deploys
   (YAML in Git)        (GitOps)         (Auto/Manual)     (Creates Resources)
```

**Development**: Auto-sync via ArgoCD on git push to `develop` branch
**Staging**: Auto-sync on git push to `staging` branch
**Production**: Manual approval required before sync to `main` branch

### Environment Separation

Models are deployed to three isolated environments:

- **Development**: Rapid iteration, 1 replica, relaxed SLOs
- **Staging**: Production validation, 1 replica, production SLOs
- **Production**: Live traffic, 2-3 replicas, strict SLOs

Each environment has:
- Separate service endpoints: `{model}-{env}.system.svc.cluster.local:8000`
- Separate registry entries: `(model_name, environment)` unique constraint
- Separate resource quotas and NetworkPolicies
- Environment-specific monitoring and alerting

### Operational Procedures

**Rollout**: `Development → Staging → Production` with validation gates via branch promotion
**Rollback**: Revert AIModel CR changes in Git, ArgoCD auto-syncs the rollback
**Promotion**: PR from `develop` → `staging` → `main` with validation gates

For detailed operational procedures, see:
- [AI Model Operator Guide](docs/operators/ai-model-operator.md)
- [Rollout Workflow](docs/rollout-workflow.md)
- [Rollback Workflow](docs/rollback-workflow.md)

## Services

### API Router Service (`api-router-service`)

*   **Description**: The main entry point for all API requests. It handles routing, authentication, rate limiting, and more. Includes model registry integration for dynamic vLLM endpoint discovery with Redis caching.
*   **Language**: Go
*   **Dependencies**: User & Org Service, Budget Service, Model Registry (PostgreSQL), Redis, Kafka.

### User & Organization Service (`user-org-service`)

*   **Description**: Manages all data related to users, organizations, API keys, and authentication.
*   **Language**: Go
*   **Dependencies**: PostgreSQL.

### Budget Service (`budget-service`)

*   **Description**: Manages and enforces spending limits for organizations.
*   **Language**: Go
*   **Dependencies**: (Likely PostgreSQL or another database).

### Analytics Service (`analytics-service`)

*   **Description**: Consumes usage data from Kafka to provide analytics and billing information.
*   **Language**: (Likely Go or Python)
*   **Dependencies**: Kafka, (Likely a data warehouse like ClickHouse or Snowflake).

### Admin CLI (`admin-cli`)

*   **Description**: A command-line interface for administrators to manage the platform (e.g., creating users, managing organizations, managing model registry, inspecting deployments).
*   **Language**: Go
*   **Commands**: `registry` (register/deregister/enable/disable/list models), `deployment` (inspect status)
*   **Dependencies**: Interacts with the APIs of the various services and directly with PostgreSQL for model registry operations.

### Hello Service & World Service

*   **Description**: These are likely example or template services, demonstrating how to build a new service within the platform's architecture.
*   **Language**: Go

### AI Model Operator (`ai-model-operator`)

*   **Description**: Kubernetes operator that manages the lifecycle of AI model deployments. Watches AIModel Custom Resources and creates corresponding Deployments, Services, and ConfigMaps.
*   **Language**: Go
*   **CRD**: `AIModel` (apiVersion: `aimodel.ai-aas.io/v1alpha1`)
*   **Dependencies**: Kubernetes API, model sources (HuggingFace, S3)

### vLLM Model Deployments

*   **Description**: GPU-accelerated model inference engines managed by the AI Model Operator. Each model runs in its own pod with dedicated GPU resources and exposes OpenAI-compatible API endpoints.
*   **Technology**: vLLM (Python), NVIDIA GPUs, Kubernetes
*   **Management**: AIModel CRs (declarative), AI Model Operator (lifecycle), ArgoCD (GitOps)
*   **Environments**: Development, Staging, Production (with separate configurations and SLOs)
*   **Observability**: Prometheus metrics, Grafana dashboards, Alertmanager alerts, Loki logs

## Infrastructure Components

### Kubernetes Resources

**AI Model Deployments** are managed by the AI Model Operator, which creates the following resources from AIModel CRs:
- Deployment: Manages vLLM pods with GPU resource requests (auto-generated from AIModel spec)
- Service: Exposes inference endpoint within cluster (ClusterIP)
- ConfigMap: Runtime configuration (model path, args, environment variables)
- NetworkPolicy: Restricts traffic to API Router Service only (future)
- ServiceMonitor: Prometheus scrape configuration (future)

### Model Registry (PostgreSQL)

Tracks all deployed models with:
- Model name and deployment endpoint
- Environment (development/staging/production)
- Deployment status (ready/disabled)
- Health check timestamp
- Namespace and metadata

Accessed by:
- API Router Service (for dynamic routing)
- Admin CLI (for management)

### Observability Stack

**Prometheus**:
- Scrapes metrics from vLLM pods every 30s
- Evaluates alert rules (deployment health, performance, resources)
- Retains 30 days of metrics

**Grafana** (http://grafana.{cluster-ip}.nip.io):
- **Service Logs Dashboard** (`service-logs.json`): Aggregated logs from all Go services, filterable by service/level/trace_id
- **Request Tracing Dashboard** (`request-tracing.json`): End-to-end request correlation across services using trace_id
- **Inference Backends Dashboard** (`inference-backends.json`): vLLM pod status, GPU utilization, latency, throughput
- **SLO Tracking Dashboard**: Availability, error budget consumption, latency percentiles

**Alertmanager**:
- Routes alerts based on severity
- Critical → PagerDuty
- High → Slack (#vllm-alerts-high)
- Medium → Slack (#vllm-alerts)

**Loki** (required):
- Aggregates logs from all services (Go services, vLLM pods, operators)
- Structured JSON log ingestion with label extraction
- Required for Service Logs and Request Tracing dashboards
- Query endpoint: http://loki.{cluster-ip}.nip.io/loki/api/v1/query_range

## Model Recipes (Planned)

**ModelRecipe** is a planned CRD (spec 025) that will enable user-defined inference pipelines by chaining multiple AI models together.

### Concept

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: ModelRecipe
metadata:
  name: summarize-and-translate
spec:
  steps:
    - name: summarize
      model: gpt-oss-20b
      prompt: "Summarize the following text: {{input}}"
    - name: translate
      model: mistral-7b-instruct
      prompt: "Translate to Spanish: {{summarize.output}}"
  output: "{{translate.output}}"
```

### Use Cases

- **Multi-step pipelines**: Summarize → Translate → Format
- **Ensemble inference**: Run multiple models, aggregate results
- **Conditional routing**: Route to different models based on input characteristics
- **Pre/post processing**: Add validation, filtering, or formatting steps

For detailed specification, see: [specs/025-model-recipes/spec.md](specs/025-model-recipes/spec.md)

## Ingress Architecture

The platform uses a **dual ingress architecture** with NGINX as the primary ingress and Istio for service mesh capabilities.

### Components

```
                 Internet
                     │
          ┌──────────┴──────────┐
          │                     │
    ┌─────▼─────┐        ┌─────▼─────┐
    │   NGINX   │        │   Istio   │
    │  Ingress  │        │  Gateway  │
    │ (Primary) │        │(Secondary)│
    └─────┬─────┘        └─────┬─────┘
          │                    │
    ┌─────▼────────────────────▼─────┐
    │         Kubernetes Services     │
    │                                 │
    │  api-router  grafana  argocd   │
    │  loki        prometheus  etc.  │
    └─────────────────────────────────┘
```

### NGINX Ingress (Primary)

- **Purpose**: External traffic ingress for all HTTP/HTTPS services
- **TLS**: Handles TLS termination for public endpoints
- **Routing**: Host-based routing (e.g., `api.172.232.58.222.nip.io`, `grafana.172.232.58.222.nip.io`)
- **Configuration**: `infra/k8s/ingress/` and service-specific Helm values

### Istio Service Mesh (Secondary)

- **Purpose**: Advanced traffic management, mTLS between services
- **Use Cases**: Canary deployments, traffic mirroring, circuit breaking
- **Configuration**: VirtualServices, DestinationRules in service Helm charts

### Endpoint Patterns

| Service | External URL Pattern |
|---------|---------------------|
| API Router | `https://api.{cluster-ip}.nip.io` |
| Grafana | `http://grafana.{cluster-ip}.nip.io` |
| Loki | `http://loki.{cluster-ip}.nip.io` |
| ArgoCD | `https://argocd.{domain}` |

For detailed ingress configuration, see: [docs/technical/platform/ingress-best-practices.md](docs/technical/platform/ingress-best-practices.md)

## Next Steps

For more detailed information on a specific service or component, please refer to:
- **AI Model Operator**: [docs/operators/ai-model-operator.md](docs/operators/ai-model-operator.md)
- **AIModel Configuration**: [ai-aas-config repository](https://github.com/otherjamesbrown/ai-aas-config)
- **Service READMEs**: `services/<service-name>/README.md`
- **Operational Workflows**: [docs/rollout-workflow.md](docs/rollout-workflow.md), [docs/rollback-workflow.md](docs/rollback-workflow.md)
- **Debugging Guide**: [docs/runbooks/ai-debugging-workflow.md](docs/runbooks/ai-debugging-workflow.md)
- **Monitoring**: [docs/monitoring/performance-slo-tracking.md](docs/monitoring/performance-slo-tracking.md)
