# Architecture Overview

## High-Level Architecture

The AIaaS platform is a microservices-based inference-as-a-service platform built on Kubernetes, designed for multi-tenancy, scalability, and operational excellence.

## Architecture Principles

### 1. API-First Design
- All services expose REST APIs with OpenAPI specifications
- UI/CLI remain client-only (no business logic)
- OpenAI-compatible inference endpoint (`/v1/chat/completions`)

### 2. Statelessness & Service Boundaries
- No reliance on in-process state across requests
- Persistent state in PostgreSQL; cache in Redis; async work via RabbitMQ
- No shared databases across services; clear single-responsibility per service

### 3. Async Non-Critical Work
- Non-critical operations (analytics, logging) removed from critical path
- Queue consumers are idempotent and resilient to retries
- Fire-and-forget pattern for usage events

### 4. Security by Default
- Authentication on every request
- Authorization via RBAC middleware on every action
- Secrets never in Git; API keys SHA-256; passwords bcrypt
- NetworkPolicies enforce zero-trust; TLS via Ingress + cert-manager

### 5. Declarative Operations & GitOps
- Terraform for cloud infrastructure
- Helm for application deployment
- ArgoCD for GitOps reconciliation
- Git is source of truth; no manual applies in production

### 6. Observability
- `/health`, `/ready`, `/metrics` for every service
- Logs (JSON), metrics (Prometheus/Mimir), traces (OpenTelemetry→Tempo)
- Required dashboards for system, org usage, model performance, infra health

## System Architecture Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                         Clients                                 │
│              (Applications, SDKs, CLI)                         │
└────────────────────────┬────────────────────────────────────────┘
                         │ HTTPS
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                    NGINX Ingress                                 │
│              (TLS Termination, Routing)                          │
└────────────────────────┬────────────────────────────────────────┘
                         │
         ┌───────────────┴───────────────┐
         │                               │
┌────────▼────────┐            ┌─────────▼──────────┐
│  API Router     │            │  User & Org Service │
│   Service       │            │   (Admin API)       │
│                 │            │                     │
│ - Auth          │            │ - User Management  │
│ - Routing       │            │ - API Keys         │
│ - Rate Limiting │            │ - Budgets          │
│ - Usage Track   │            │ - RBAC             │
└────────┬────────┘            └─────────┬──────────┘
         │                               │
         │ HTTP                          │ HTTP
         │                               │
┌────────▼────────┐            ┌─────────▼──────────┐
│  Model Backends │            │   PostgreSQL        │
│  (vLLM, etc.)  │            │   (User/Org DB)     │
└─────────────────┘            └─────────┬──────────┘
                                        │
         ┌──────────────────────────────┼──────────────┐
         │                              │              │
┌────────▼────────┐         ┌──────────▼──────┐  ┌───▼──────────┐
│   RabbitMQ      │         │   PostgreSQL    │  │    Redis     │
│  (Usage Events) │         │  (TimescaleDB)  │  │   (Cache)    │
└────────┬────────┘         └─────────────────┘  └──────────────┘
         │
         │
┌────────▼────────┐
│   Analytics     │
│    Service      │
│                 │
│ - Ingestion     │
│ - Aggregation   │
│ - Exports       │
└─────────────────┘
```

## Service Architecture

### Request Flow

1. **Client Request** → NGINX Ingress (TLS termination)
2. **Ingress** → API Router Service
3. **API Router**:
   - Authenticates API key (sync call to User & Org Service)
   - Checks budget/rate limits (Redis)
   - Routes to model backend
   - Publishes usage event (async to RabbitMQ)
   - Returns response to client

### Data Flow

1. **Usage Events**: API Router → RabbitMQ → Analytics Service
2. **Analytics Processing**: Analytics Service → PostgreSQL (TimescaleDB)
3. **Aggregations**: Continuous aggregates in TimescaleDB
4. **Queries**: Analytics API → PostgreSQL → Client
5. **Exports**: Analytics Service → S3 → Client

## Infrastructure Architecture

### Kubernetes Cluster

- **Provider**: Akamai Linode Kubernetes Engine (LKE)
- **Region**: `fr-par` (configurable)
- **Environments**: `development`, `staging`, `production`, `system`
- **Node Pools**:
  - Baseline: `g6-standard-8` (3-6 nodes, autoscaling to 10)
  - GPU: `g1-gpu-rtx6000` (2 nodes) for vLLM workloads

### Networking

- **Ingress**: NGINX Ingress Controller + cert-manager (Let's Encrypt)
- **DNS**: Linode DNS for external records
- **Network Policies**: Calico NetworkPolicies (default-deny stance)
- **Service Mesh**: Not currently implemented (future consideration)

### Storage

- **PostgreSQL**: Managed databases per service
- **Redis**: Shared cluster with database separation
- **Object Storage**: Linode Object Storage (S3-compatible)
- **Secrets**: Linode Secret Manager + Sealed Secrets

### Observability Stack

- **Metrics**: Prometheus/Grafana (`kube-prometheus-stack`)
- **Logs**: Loki (tenant per environment)
- **Traces**: Tempo (optional by environment)
- **Alerting**: Alertmanager → Slack

## Deployment Architecture

### GitOps Flow

1. **Terraform**: Provisions infrastructure (clusters, networking, secrets)
2. **Helm Charts**: Application definitions (`infra/helm/charts/*`)
3. **ArgoCD**: GitOps reconciliation (watches Git repository)
4. **Kubernetes**: Runs workloads

### Environment Management

- **Single Control Plane**: All environments share one Kubernetes cluster
- **Namespace Isolation**: Dedicated namespaces per environment
- **Resource Quotas**: Per-environment quotas (supports 30 services)
- **Configuration**: Kustomize overlays (`development`, `staging`, `production`)

### Secrets Management

- **Source of Truth**: Linode Secret Manager
- **GitOps**: Sealed Secrets controller encrypts bundles
- **ArgoCD**: Applies manifests per environment
- **Rotation**: Quarterly rotation checks

## Scalability Architecture

### Horizontal Scaling

- **Stateless Services**: All services scale horizontally
- **Load Balancing**: Kubernetes Service objects
- **Auto-scaling**: HPA (Horizontal Pod Autoscaler) based on CPU/memory

### Database Scaling

- **Connection Pooling**: Per-service connection pools
- **Read Replicas**: Future consideration for read-heavy workloads
- **Partitioning**: TimescaleDB hypertables for time-series data

### Caching Strategy

- **Redis**: Distributed cache for rate limits, sessions, freshness
- **Local Cache**: BoltDB for configuration (API Router)
- **Cache Invalidation**: TTL-based expiration

## Security Architecture

### Authentication

- **API Keys**: SHA-256 hashed, stored in User & Org Service
- **OAuth2**: Fosite provider with MFA support
- **HMAC**: Optional signature verification for API requests

### Authorization

- **RBAC**: Role-based access control middleware
- **Policy Engine**: OPA/Rego for policy evaluation
- **Network Policies**: Calico default-deny stance

### Data Protection

- **Encryption at Rest**: Database encryption, object storage encryption
- **Encryption in Transit**: TLS 1.3 via Ingress
- **Secrets**: Never in Git, managed via Secret Manager

## Reliability Architecture

### High Availability

- **Multi-Replica**: Services run with multiple replicas
- **Health Checks**: Liveness and readiness probes
- **Graceful Degradation**: Services operate with optional dependencies unavailable

### Fault Tolerance

- **Circuit Breakers**: Future consideration for external dependencies
- **Retries**: Idempotent async operations with retries
- **Failover**: Intelligent routing with backend failover

### Disaster Recovery

- **Backups**: Database backups (automated)
- **State Management**: Terraform state in object storage
- **Recovery Procedures**: Documented runbooks

## Performance Architecture

### Latency Targets

- **Inference TTFB**: P50 <100ms, P95 <300ms
- **Management API**: P99 <500ms
- **Analytics Queries**: Sub-second for common queries

### Optimization Strategies

- **Caching**: Redis for frequently accessed data
- **Async Processing**: Non-critical work off critical path
- **Connection Pooling**: Database connection reuse
- **Batch Processing**: Analytics aggregations in batches

## Related Documentation

- [System Components](./system-components.md) - Detailed component descriptions
- [Service Interactions](./service-interactions.md) - Service communication patterns
- [Data Flow](./data-flow.md) - Request and data flow details
- [Deployment Architecture](./deployment-architecture.md) - Infrastructure and deployment

