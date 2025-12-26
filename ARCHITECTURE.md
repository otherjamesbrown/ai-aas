# Architecture

> **Last verified**: 2025-12-13 | **Type**: reference

---

## System Overview

```yaml
platform:
  gateway: api-router-service
  core_services:
    - user-org-service (auth, users, orgs, API keys)
    - admin-api-service (model registry, deployments)
    - analytics-service (usage tracking, billing)

  model_deployment:
    - ai-model-operator (Kubernetes operator)
    - vLLM pods (GPU inference)
    - KServe InferenceServices

  backing_services:
    - PostgreSQL (primary database)
    - Redis (caching, rate limiting)
    - Kafka (async messaging)

  gitops:
    - ArgoCD (deployment sync)
    - Branch promotion: develop → staging → main
```

---

## Request Flow

```yaml
request_flow:
  1_ingress: "Client → NGINX Ingress → api-router-service"
  2_auth: "api-router → user-org-service (validate API key)"
  3_routing: "api-router → Model Registry (lookup endpoint)"
  4_inference: "api-router → vLLM pod (forward request)"
  5_response: "vLLM → api-router → Client"
  6_async: "api-router → Kafka (usage event)"

caching:
  redis_ttl: 2 minutes
  what: Model registry lookups, rate limits
```

---

## AI Model Operator

```yaml
aimodel_lifecycle:
  1: "AIModel CR created in Git (infra/k8s/aimodels/{env}/)"
  2: "ArgoCD syncs CR to cluster"
  3: "Operator watches, creates InferenceService"
  4: "KServe creates vLLM pod with GPU"
  5: "Operator updates status.inferenceEndpoint"

aimodel_storage:
  internal: "infra/k8s/aimodels/{env}/"
  community: "ai-aas-config repo: environments/{env}/models/"

detailed_reference: "docs/operators/ai-model-operator.md"
crd_spec: "context/operator-developer/agents.md (aimodel_crd_spec)"
```

---

## Services

```yaml
services:
  api-router-service:
    purpose: Gateway for all inference requests
    responsibilities:
      - Authentication (API key validation)
      - Rate limiting
      - Model routing (registry lookup)
      - Request forwarding to vLLM
    dependencies: [user-org-service, PostgreSQL, Redis, Kafka]
    code: services/api-router-service/

  admin-api-service:
    purpose: Platform administration API
    responsibilities:
      - Model registry CRUD
      - Deployment management
      - Credentials management
      - Routing policies
    dependencies: [PostgreSQL, S3]
    code: services/admin-api-service/

  user-org-service:
    purpose: Identity and access management
    responsibilities:
      - User authentication
      - Organization management
      - API key issuance/validation
      - RBAC
    dependencies: [PostgreSQL]
    code: services/user-org-service/

  analytics-service:
    purpose: Usage tracking and billing
    responsibilities:
      - Consume usage events from Kafka
      - Aggregate metrics
      - Generate billing data
    dependencies: [Kafka, PostgreSQL]
    code: services/analytics-service/

  ai-model-operator:
    purpose: Model deployment automation
    responsibilities:
      - Watch AIModel CRs
      - Create/update InferenceServices
      - Manage model downloads
      - Status reporting
    dependencies: [Kubernetes API, KServe, S3]
    code: operators/ai-model-operator/

  ai-aas-cli:
    purpose: Admin command-line tool
    responsibilities:
      - Model registry management
      - Deployment operations
      - Credentials management
    rule: Thin client - calls admin-api-service
    code: services/ai-aas-cli/
```

---

## Infrastructure

```yaml
kubernetes:
  resources_per_model:
    - InferenceService (KServe manages vLLM pods)
    - Service (ClusterIP for internal access)
    - ConfigMap (runtime configuration)

  namespaces:
    system: Model deployments, operators
    argocd: GitOps
    monitoring: Prometheus, Grafana, Loki

model_registry:
  database: PostgreSQL
  schema:
    - model_name (unique per environment)
    - environment (development/staging/production)
    - inference_endpoint
    - status (ready/disabled)
    - health_check_timestamp

environments:
  development:
    branch: develop
    replicas: 1
    auto_sync: true
  staging:
    branch: staging
    replicas: 1
    auto_sync: true
  production:
    branch: main
    replicas: 2-3
    auto_sync: false (manual approval)
```

---

## Observability

```yaml
prometheus:
  scrape_interval: 30s
  retention: 30d
  targets: [vLLM pods, Go services, operators]

grafana:
  url: "https://grafana.dev.otherjamesbrown.com"
  dashboards:
    - service-logs.json (aggregated logs)
    - request-tracing.json (trace correlation)
    - inference-backends.json (vLLM metrics)

loki:
  url: "https://loki.dev.otherjamesbrown.com"
  purpose: Log aggregation
  format: Structured JSON
  query_api: "/loki/api/v1/query_range"

alertmanager:
  routing:
    critical: PagerDuty
    high: "Slack #vllm-alerts-high"
    medium: "Slack #vllm-alerts"
```

---

## Ingress

```yaml
architecture: Dual ingress (NGINX primary, Istio secondary)

nginx_ingress:
  purpose: External HTTP/HTTPS traffic
  features:
    - TLS termination
    - Host-based routing
  config: infra/k8s/ingress/

istio_gateway:
  purpose: Advanced traffic management
  features:
    - mTLS between services
    - Canary deployments
    - Traffic mirroring
  config: Service Helm charts (VirtualServices)

endpoints:
  api_router: "https://api.dev.otherjamesbrown.com"
  grafana: "https://grafana.dev.otherjamesbrown.com"
  loki: "https://loki.dev.otherjamesbrown.com"
  argocd: "https://argocd.{domain}"

detailed_reference: "docs/technical/platform/ingress-best-practices.md"
```

---

## References

| Topic | Document |
|-------|----------|
| AI Model Operator | `docs/operators/ai-model-operator.md` |
| AIModel CRD spec | `context/operator-developer/agents.md` |
| Environment access | `docs/platform/environment-access.md` |
| Debugging | `docs/runbooks/ai-debugging-workflow.md` |
| Deployment | `docs/runbooks/deploy-to-environments.md` |
| Service READMEs | `services/<name>/README.md` |
