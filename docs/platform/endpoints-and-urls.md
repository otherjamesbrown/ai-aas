# Endpoints and URLs

---
last_updated: 2025-12-08
last_verified: 2025-12-08
document_type: reference
verification_command: "kubectl get ingress -A"
---

## Overview

This document lists all exposed endpoints in the platform. URLs use two domain patterns:
- `.ai-aas.local` - For local development (requires hosts file entry)
- `.otherjamesbrown.com` - Public DNS (nip.io fallback available)

**Ingress IP**: `172.232.58.222` (development cluster)

## Quick Reference - Development Environment

| Service | URL | Status |
|---------|-----|--------|
| API Router | `https://api.dev.ai-aas.local` | ✅ Active |
| Web Portal | `https://portal.dev.ai-aas.local` | ✅ Active |
| User-Org Service | `https://user-org.dev.ai-aas.local` | ✅ Active |
| Analytics Service | `https://analytics.dev.ai-aas.local` | ✅ Active |
| Admin API | `https://admin-api.dev.ai-aas.local` | ✅ Active |
| Grafana | `https://grafana.dev.ai-aas.local` | ✅ Active |
| Loki | `https://loki.dev.ai-aas.local` | ✅ Active |
| ArgoCD | `https://argocd.dev.ai-aas.local` | ✅ Active |
| etcd | `https://etcd.dev.ai-aas.local` | ✅ Active |

## Verification

```bash
# List all ingresses
kubectl get ingress -A

# Check specific service
curl -k https://api.dev.ai-aas.local/v1/status/healthz
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
- `https://api.dev.ai-aas.local`
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
- `https://user-org.dev.ai-aas.local`
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
- `https://analytics.dev.ai-aas.local`
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
- `https://admin-api.dev.ai-aas.local`
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
- `https://portal.dev.ai-aas.local`
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
- `https://grafana.dev.ai-aas.local`
- `https://grafana.dev.otherjamesbrown.com`
- `http://grafana.172.232.58.222.nip.io` (fallback)

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
- `https://loki.dev.ai-aas.local`
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

**URL**: `https://argocd.dev.ai-aas.local`

**Configuration**: `gitops/templates/argocd-values.yaml`

### etcd

| Property | Value |
|----------|-------|
| Purpose | Distributed key-value store |
| Namespace | `development` |
| Ingress | ✅ Configured |
| TLS | ✅ Configured |

**URLs**:
- `https://etcd.dev.ai-aas.local`
- `https://etcd.dev.otherjamesbrown.com`

## Local DNS Setup

### Hosts File Entries

Add to `/etc/hosts` (Linux/macOS) or `C:\Windows\System32\drivers\etc\hosts` (Windows):

```
# AI-AAS Development Environment
172.232.58.222  api.dev.ai-aas.local
172.232.58.222  portal.dev.ai-aas.local
172.232.58.222  user-org.dev.ai-aas.local
172.232.58.222  analytics.dev.ai-aas.local
172.232.58.222  admin-api.dev.ai-aas.local
172.232.58.222  grafana.dev.ai-aas.local
172.232.58.222  loki.dev.ai-aas.local
172.232.58.222  argocd.dev.ai-aas.local
172.232.58.222  etcd.dev.ai-aas.local
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
