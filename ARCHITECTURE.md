# Architecture

This document provides a high-level overview of the AI-as-a-Service platform architecture.

## System Overview

The platform is designed as a set of cooperating microservices. The central component is the **API Router Service**, which acts as the public-facing gateway for all AI model inference requests. It is responsible for authentication, authorization, rate limiting, budget enforcement, and routing requests to the appropriate backend model services.

Other key services include:

*   **User & Organization Service**: Manages users, organizations, and API keys.
*   **Budget Service**: Tracks and enforces spending budgets for organizations.
*   **Analytics Service**: Collects and processes usage data for billing and analysis.
*   **Admin CLI**: A command-line tool for administering the platform.
*   **vLLM Deployments**: GPU-accelerated model inference engines managed via Helm and registered in the model registry.

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
```

## vLLM Deployment Architecture

The platform uses **vLLM** (Very Large Language Model) for high-performance model inference with GPU acceleration. The deployment architecture follows GitOps principles and provides safe operational procedures.

### Component Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Control Plane                                 │
├──────────────────────────────────────────────────────────────────────┤
│                                                                       │
│  ┌────────────┐        ┌────────────┐        ┌────────────┐        │
│  │  Admin CLI │───────▶│   Helm     │───────▶│  ArgoCD    │        │
│  │  (Manage)  │        │ (Package)  │        │  (GitOps)  │        │
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

1. **Helm Charts** (`infra/helm/charts/vllm-deployment/`)
   - Declarative deployment configuration
   - Environment-specific values files (development, staging, production)
   - GPU resource management
   - Health probes and auto-scaling

2. **Model Registry** (PostgreSQL)
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
1. Deploy via Helm → 2. Register in Registry → 3. Route Traffic
   (Kubernetes)        (admin-cli registry)     (API Router)
```

**Development/Staging**: Auto-sync via ArgoCD on git push
**Production**: Manual approval required before sync

### Environment Separation

Models are deployed to three isolated environments:

- **Development**: Rapid iteration, 1 replica, relaxed SLOs
- **Staging**: Production validation, 1 replica, production SLOs
- **Production**: Live traffic, 2-3 replicas, strict SLOs

Each environment has:
- Separate service endpoints: `{model}-{env}.system.svc.cluster.local`
- Separate registry entries: `(model_name, environment)` unique constraint
- Separate resource quotas and NetworkPolicies
- Environment-specific monitoring and alerting

### Operational Procedures

**Rollout**: `Development → Staging → Production` with validation gates
**Rollback**: `helm rollback` with automatic registry status management
**Promotion**: Automated script validates staging before production deployment

For detailed operational procedures, see:
- [Rollout Workflow](docs/rollout-workflow.md)
- [Rollback Workflow](docs/rollback-workflow.md)
- [Registration Workflow](docs/vllm-registration-workflow.md)

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

### vLLM Model Deployments

*   **Description**: GPU-accelerated model inference engines deployed via Helm charts. Each model runs in its own pod with dedicated GPU resources and exposes OpenAI-compatible API endpoints.
*   **Technology**: vLLM (Python), NVIDIA GPUs, Kubernetes
*   **Management**: Helm (packaging), ArgoCD (GitOps), Admin CLI (registry management)
*   **Environments**: Development, Staging, Production (with separate configurations and SLOs)
*   **Observability**: Prometheus metrics, Grafana dashboards, Alertmanager alerts, Loki logs

## Infrastructure Components

### Kubernetes Resources

**vLLM Deployments** are managed through Helm charts with the following resources:
- Deployment: Manages vLLM pods with GPU resource requests
- Service: Exposes inference endpoint within cluster
- ConfigMap: vLLM configuration (model path, max length, etc.)
- NetworkPolicy: Restricts traffic to API Router Service only
- ServiceMonitor: Prometheus scrape configuration

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

**Grafana**:
- vLLM Deployment Dashboard (pod status, latency, GPU usage)
- SLO tracking dashboard (availability, error budget)
- Log explorer (via Loki)

**Alertmanager**:
- Routes alerts based on severity
- Critical → PagerDuty
- High → Slack (#vllm-alerts-high)
- Medium → Slack (#vllm-alerts)

**Loki** (optional):
- Aggregates logs from all vLLM pods
- Provides unified log search across environments

## Next Steps

For more detailed information on a specific service or component, please refer to:
- Service READMEs: `services/<service-name>/README.md`
- vLLM Deployment: `infra/helm/charts/vllm-deployment/README.md`
- Operational Workflows: `docs/rollout-workflow.md`, `docs/rollback-workflow.md`
- Troubleshooting: `docs/troubleshooting/vllm-deployment-troubleshooting.md`
- Monitoring: `docs/monitoring/performance-slo-tracking.md`
