# Endpoints and URLs

---
last_updated: 2025-12-09
last_verified: 2025-12-09
document_type: reference
verification_command: "kubectl get ingress -A"
---

## Overview

This document lists all exposed endpoints in the platform across all environments.

**Domain Patterns**:
- Development: `.dev.otherjamesbrown.com` and `.dev.otherjamesbrown.com`
- Staging: `.staging.otherjamesbrown.com` and `.staging.otherjamesbrown.com`
- Production: `.otherjamesbrown.com` and `.otherjamesbrown.com`

**Ingress IPs**:
- Development: `172.232.58.222`
- Staging: `172.236.135.55`

## Quick Reference - Development Environment

| Service | URL | Status |
|---------|-----|--------|
| API Router | `https://api.dev.otherjamesbrown.com` | ✅ Active |
| Web Portal | `https://portal.dev.otherjamesbrown.com` | ✅ Active |
| User-Org Service | `https://user-org.dev.otherjamesbrown.com` | ✅ Active |
| Analytics Service | `https://analytics.dev.otherjamesbrown.com` | ✅ Active |
| Admin API | `https://admin-api.dev.otherjamesbrown.com` | ✅ Active |
| Grafana | `https://grafana.dev.otherjamesbrown.com` | ✅ Active |
| Loki | `https://loki.dev.otherjamesbrown.com` | ✅ Active |
| ArgoCD | `https://argocd.dev.otherjamesbrown.com` | ✅ Active |
| etcd | `https://etcd.dev.otherjamesbrown.com` | ✅ Active |

## Quick Reference - Staging Environment

| Service | URL | Status |
|---------|-----|--------|
| API Router | `https://api.staging.otherjamesbrown.com` | ✅ Active |
| User-Org Service | `https://user-org.staging.otherjamesbrown.com` | ✅ Active |
| Analytics Service | `https://analytics.staging.otherjamesbrown.com` | ✅ Active |
| Admin API | `https://admin-api.staging.otherjamesbrown.com` | ✅ Active |
| Grafana | `https://grafana.staging.otherjamesbrown.com` | ⚠️ Local only |

**Note**: Staging uses Let's Encrypt staging certificates (not trusted by browsers). Use `-k` flag with curl for testing.

## Verification

```bash
# Development
kubectl --kubeconfig=~/kubeconfigs/kubeconfig-development.yaml get ingress -A
curl -k https://api.dev.otherjamesbrown.com/v1/status/healthz

# Staging
kubectl --kubeconfig=~/kubeconfigs/kubeconfig-staging.yaml get ingress -A
curl -k https://api.staging.otherjamesbrown.com/v1/status/healthz
```

## Application Services

### API Router Service (Gateway)

| Property | Value |
|----------|-------|
| Purpose | Main API gateway for inference requests |
| Namespace | `development` |
| Port | 8080 |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://api.dev.otherjamesbrown.com`
- `https://api.dev.otherjamesbrown.com`

**Endpoints**:
| Path | Method | Auth | Description |
|------|--------|------|-------------|
| `/v1/chat/completions` | POST | API Key | OpenAI-compatible chat |
| `/v1/completions` | POST | API Key | OpenAI-compatible completions |
| `/v1/inference` | POST | API Key | Custom inference |
| `/v1/status/healthz` | GET | None | Health check |
| `/v1/status/readyz` | GET | None | Readiness check |

**Configuration**: `services/api-router-service/deployments/helm/api-router-service/values-development.yaml`

### User-Org Service

| Property | Value |
|----------|-------|
| Purpose | User and organization management |
| Namespace | `user-org-service` |
| Port | 8081 |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://user-org.dev.otherjamesbrown.com`
- `https://user-org.dev.otherjamesbrown.com`

**Endpoints**:
| Path | Description |
|------|-------------|
| `/v1/auth/*` | Authentication |
| `/v1/orgs/*` | Organization management |
| `/v1/users/*` | User management |
| `/healthz` | Health check |

**Configuration**: `services/user-org-service/deployments/helm/user-org-service/values-development.yaml`

### Analytics Service

| Property | Value |
|----------|-------|
| Purpose | Usage metrics and analytics |
| Namespace | `development` |
| Port | 8084 |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://analytics.dev.otherjamesbrown.com`
- `https://analytics.dev.otherjamesbrown.com`

**Endpoints**:
| Path | Description |
|------|-------------|
| `/analytics/v1/orgs/{orgId}/usage` | Usage data |
| `/analytics/v1/orgs/{orgId}/reliability` | Reliability metrics |
| `/analytics/v1/orgs/{orgId}/exports` | Data exports |
| `/analytics/v1/status/healthz` | Health check |

**Configuration**: `services/analytics-service/deployments/helm/analytics-service/values-development.yaml`

### Admin API Service

| Property | Value |
|----------|-------|
| Purpose | Administrative operations |
| Namespace | `admin-api-service` |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://admin-api.dev.otherjamesbrown.com`
- `https://admin-api.dev.otherjamesbrown.com`

**Configuration**: `services/admin-api-service/deployments/helm/admin-api-service/values-development.yaml`

### Web Portal

| Property | Value |
|----------|-------|
| Purpose | React/TypeScript web UI |
| Namespace | `development` |
| Port | 80 |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://portal.dev.otherjamesbrown.com`
- `https://portal.dev.otherjamesbrown.com`

**Configuration**: `web/portal/deployments/helm/web-portal/values-development.yaml`

## Observability Stack

### Grafana

| Property | Value |
|----------|-------|
| Purpose | Dashboards and visualization |
| Namespace | `monitoring` |
| Port | 3000 |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://grafana.dev.otherjamesbrown.com`
- `https://grafana.dev.otherjamesbrown.com`
- `https://grafana.dev.otherjamesbrown.com` (fallback)

**Configuration**: `gitops/clusters/development/apps/` (kube-prometheus-stack)

### Loki

| Property | Value |
|----------|-------|
| Purpose | Log aggregation |
| Namespace | `monitoring` |
| Port | 3100 |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://loki.dev.otherjamesbrown.com`
- `https://loki.dev.otherjamesbrown.com`

**Configuration**: `gitops/clusters/development/apps/` (loki application)

### Prometheus

| Property | Value |
|----------|-------|
| Purpose | Metrics collection |
| Namespace | `monitoring` |
| Port | 9090 |
| Ingress | Internal only |

Access via Grafana or port-forward:
```bash
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090
```

## Infrastructure Services

### ArgoCD

| Property | Value |
|----------|-------|
| Purpose | GitOps deployment |
| Namespace | `argocd` |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URL**: `https://argocd.dev.otherjamesbrown.com`

**Configuration**: `gitops/templates/argocd-values.yaml`

### etcd

| Property | Value |
|----------|-------|
| Purpose | Distributed key-value store |
| Namespace | `development` |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://etcd.dev.otherjamesbrown.com`
- `https://etcd.dev.otherjamesbrown.com`

## Local DNS Setup

### Hosts File Entries

Add to `/etc/hosts` (Linux/macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows):

```
# AI-AAS Development Environment
172.232.58.222  api.dev.otherjamesbrown.com
172.232.58.222  portal.dev.otherjamesbrown.com
172.232.58.222  user-org.dev.otherjamesbrown.com
172.232.58.222  analytics.dev.otherjamesbrown.com
172.232.58.222  admin-api.dev.otherjamesbrown.com
172.232.58.222  grafana.dev.otherjamesbrown.com
172.232.58.222  loki.dev.otherjamesbrown.com
172.232.58.222  argocd.dev.otherjamesbrown.com
172.232.58.222  etcd.dev.otherjamesbrown.com
```

### Automation Script

```bash
# Auto-update hosts file
sudo ./scripts/infra/update-hosts-file.sh
```

## TLS Certificates

Self-signed certificates are used for development. See:
- Certificate files: `infra/secrets/certs/`
- Setup guide: [TLS/SSL Setup](tls-ssl-setup.md)

To trust the CA:
```bash
# Linux (Debian/Ubuntu)
sudo cp infra/secrets/certs/ai-aas-ca.crt /usr/local/share/ca-certificates/
sudo update-ca-certificates

# macOS
sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain infra/secrets/certs/ai-aas-ca.crt
```

## Finding Ingress Configuration

| Service | Helm Values Location |
|---------|---------------------|
| API Router | `services/api-router-service/deployments/helm/api-router-service/values-*.yaml` |
| User-Org | `services/user-org-service/deployments/helm/user-org-service/values-*.yaml` |
| Analytics | `services/analytics-service/deployments/helm/analytics-service/values-*.yaml` |
| Admin API | `services/admin-api-service/deployments/helm/admin-api-service/values-*.yaml` |
| Web Portal | `web/portal/deployments/helm/web-portal/values-*.yaml` |
| ArgoCD | `gitops/templates/argocd-values.yaml` |
| Observability | `gitops/clusters/<env>/apps/` |

## Related Documentation

- [Environment Access](environment-access.md) - Credentials and kubeconfig setup
- [TLS/SSL Setup](tls-ssl-setup.md) - Certificate configuration
- [Infrastructure Overview](infrastructure-overview.md) - Architecture overview
