# Infrastructure Architecture Overview

---
last_updated: 2025-12-08
document_type: overview
---

## High-Level Architecture

- **Provider**: Akamai Linode Kubernetes Engine (LKE) in `fr-par` region
- **Environments**: `development`, `staging`, `production` (separate clusters)
- **GitOps**: ArgoCD reconciles Helm charts from this repository

## Directory Structure

```
infra/
├── terraform/
│   └── environments/
│       ├── _shared/          # Shared Terraform modules
│       ├── development/      # Development cluster config
│       ├── staging/          # Staging cluster config
│       ├── production/       # Production cluster config
│       └── system/           # Shared system components
├── secrets/
│   ├── certs/               # Self-signed certificates for local dev
│   └── README.md
└── helm/
    └── charts/              # Shared Helm charts (ingress configs)

gitops/
└── clusters/
    ├── development/         # ArgoCD apps for development
    │   ├── apps/            # Application definitions
    │   └── projects/        # AppProject definitions
    ├── staging/             # ArgoCD apps for staging
    │   ├── apps/
    │   └── projects/
    └── production/          # ArgoCD apps for production
        ├── apps/
        └── projects/

services/
├── admin-api-service/       # Admin API
├── admin-cli/               # Admin CLI tool
├── ai-aas-cli/              # Platform CLI
├── analytics-service/       # Analytics
├── api-router-service/      # API routing/gateway
├── user-org-service/        # User & org management
└── _template/               # Service template
```

## Node Pools

| Pool Type | Instance Type | Purpose |
|-----------|--------------|---------|
| Baseline | `g6-standard-8` | General workloads |
| GPU | `g1-gpu-rtx6000` | vLLM/ML workloads |

Node pool configuration is in Terraform: `infra/terraform/environments/<env>/`

## Networking

- **Ingress**: NGINX Ingress Controller + cert-manager
- **TLS**: Let's Encrypt (production), self-signed (development)
- **Network Policies**: Calico with default-deny stance
- **DNS**:
  - Production: Linode DNS
  - Development: Local hosts file (`.ai-aas.local` domains)

## Change Management Flow

1. **PR**: Modify files in `infra/terraform/`, `gitops/`, or `services/*/deployments/helm/`
2. **Validation**: GitHub Actions runs lint, validate, plan
3. **Approval**: Required for production changes
4. **Apply**:
   - **Terraform**: `make -C infra/terraform apply ENV=<env>`
   - **ArgoCD**: Auto-sync (dev/staging) or manual sync (production)
5. **Verification**: Health checks validate deployment

## Secrets & Access

- **Sealed Secrets**: Encrypts secrets for GitOps
- **Certificate Storage**: `infra/secrets/certs/`
- **Access Setup**: See [Environment Access](environment-access.md)

## Observability Stack

The platform uses a unified observability stack for logs, traces, metrics, and error tracking:

| Component | Purpose | Namespace | Configuration |
|-----------|---------|-----------|---------------|
| Loki | Log aggregation & storage | `system` | `infra/k8s/monitoring/loki/` |
| Promtail | Log collection (DaemonSet) | `system` | `infra/k8s/monitoring/promtail/` |
| Tempo | Distributed tracing | `system` | `infra/k8s/monitoring/tempo/` |
| OTEL Collector | Telemetry routing | `system` | `infra/k8s/monitoring/otel-collector.yaml` |
| Grafana | Dashboards & visualization | `system` | `infra/k8s/monitoring/grafana/` |
| Prometheus | Metrics collection | `system` | Existing kube-prometheus-stack |
| Sentry | Frontend error tracking | External SaaS | `web/portal/src/lib/sentry.ts` |

**Key Features**:
- **Logs**: Loki + Promtail with 14-day retention, LogQL queries
- **Traces**: Tempo + OTEL Collector with trace-to-logs correlation
- **Metrics**: Prometheus with service graph from traces
- **Frontend**: Sentry for React errors, session replay, performance monitoring
- **Inference Backends**: vLLM log collection with GPU error detection

**GitOps Deployment**: `gitops/clusters/<env>/apps/` (loki, promtail, tempo, otel-collector, grafana applications)

See [Observability Architecture](../architecture/observability-architecture.md) for detailed architecture and [Observability Guide](observability-guide.md) for operational details.

## TLS/SSL Certificates

| Environment | Method | Details |
|-------------|--------|---------|
| Production | Let's Encrypt | Automatic via cert-manager |
| Development | Self-signed | See `infra/secrets/certs/` |

Setup instructions: [TLS/SSL Setup](tls-ssl-setup.md)

## Terraform State

State is stored in Linode Object Storage:
- Bucket: `ai-aas`
- Path: `terraform/environments/<env>/terraform.tfstate`

Generated artifacts are written to: `infra/terraform/environments/<env>/.generated/`

## Related Documentation

### Platform Docs
- [Environment Access](environment-access.md) - Credentials and access setup
- [Endpoints and URLs](endpoints-and-urls.md) - Service endpoints
- [TLS/SSL Setup](tls-ssl-setup.md) - Certificate management
- [Observability Guide](observability-guide.md) - Monitoring and logging
- [CI/CD Pipeline](ci-cd-pipeline.md) - Deployment workflow

### Runbooks
- [Linode Setup](../runbooks/linode-setup.md) - Linode CLI and token setup
- [Infrastructure Rollback](../runbooks/infrastructure-rollback.md) - Rollback procedures
- [Infrastructure Troubleshooting](../runbooks/infrastructure-troubleshooting.md) - Issue resolution

### Source of Truth Locations

| Information | Location |
|-------------|----------|
| Cluster configuration | `infra/terraform/environments/<env>/` |
| ArgoCD applications | `gitops/clusters/<env>/apps/` |
| Service Helm charts | `services/<name>/deployments/helm/<name>/` |
| Shared Helm charts | `infra/helm/charts/` |
| Self-signed certs | `infra/secrets/certs/` |
