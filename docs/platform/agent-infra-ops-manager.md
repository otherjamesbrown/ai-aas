# Infrastructure Operations Manager - Document Map

---
last_updated: 2025-12-08
document_type: reference
purpose: Navigation index for infra-ops-manager AI agent
---

## Quick Navigation

This document provides a map of all platform documentation to help the infra-ops-manager agent quickly locate relevant information.

## Document Index

### Core Infrastructure

| Document | Purpose | Use When |
|----------|---------|----------|
| [infrastructure-overview.md](infrastructure-overview.md) | Architecture, directory structure, components | Understanding overall system design |
| [environment-access.md](environment-access.md) | Credentials, kubeconfigs, connection strings | Accessing clusters, databases, services |
| [endpoints-and-urls.md](endpoints-and-urls.md) | All service URLs, ingress status, ports | Finding service endpoints, checking ingress |

### Deployment & CI/CD

| Document | Purpose | Use When |
|----------|---------|----------|
| [ci-cd-pipeline.md](ci-cd-pipeline.md) | GitHub Actions, ArgoCD, deployment flow | Understanding CI/CD, fixing pipelines |
| [argocd-testing-guide.md](argocd-testing-guide.md) | ArgoCD validation, health checks | Debugging ArgoCD sync issues |
| [github-actions-guide.md](github-actions-guide.md) | Workflow patterns, troubleshooting | Fixing GitHub Actions issues |

### Security & Certificates

| Document | Purpose | Use When |
|----------|---------|----------|
| [tls-ssl-setup.md](tls-ssl-setup.md) | Certificate generation, trust setup | TLS issues, certificate problems |
| [certificate-architecture.md](certificate-architecture.md) | Cert-manager, webhook certs, KServe | Understanding certificate design |
| [access-control.md](access-control.md) | Kubernetes/ArgoCD access, Linode setup | Understanding access model |

### Observability

| Document | Purpose | Use When |
|----------|---------|----------|
| [observability-guide.md](observability-guide.md) | Prometheus, Grafana, Loki, logging | Monitoring issues, log queries |
| [data-classification.md](data-classification.md) | Data retention, log redaction | Data handling policies |

### Specialized Topics

| Document | Purpose | Use When |
|----------|---------|----------|
| [cache-salt-security.md](cache-salt-security.md) | Cache security for vLLM | Model caching security |
| [ci-remote-cli.md](ci-remote-cli.md) | Remote CI dispatch | Triggering remote CI jobs |
| [ui-pages.md](ui-pages.md) | Web portal pages, admin CLI | Understanding UI structure |

### Meta Documentation

| Document | Purpose | Use When |
|----------|---------|----------|
| [STANDARDS.md](STANDARDS.md) | Documentation standards | Creating/updating docs |

## Key Source-of-Truth Locations

### Infrastructure Configuration

| Information | Location |
|-------------|----------|
| Terraform configs | `infra/terraform/environments/<env>/` |
| ArgoCD applications | `gitops/clusters/<env>/apps/` |
| ArgoCD projects | `gitops/clusters/<env>/projects/` |
| Shared Helm charts | `infra/helm/charts/` |

### Service Configuration

| Information | Location |
|-------------|----------|
| **Service deployment specs** | `services/<name>/DEPLOYMENT.md` **(READ THIS FIRST!)** |
| Service Helm charts | `services/<name>/deployments/helm/<name>/` |
| Service values (dev) | `services/<name>/deployments/helm/<name>/values-development.yaml` |
| Service values (staging) | `services/<name>/deployments/helm/<name>/values-staging.yaml` |
| Web portal Helm | `web/portal/deployments/helm/web-portal/` |

**IMPORTANT**: The `DEPLOYMENT.md` file is maintained by the go-services-developer agent and contains the deployment contract: health endpoints, env vars, dependencies, resources, and ports.

### GitHub Actions

| Information | Location |
|-------------|----------|
| All workflows | `.github/workflows/` |
| Service CI | `.github/workflows/service-ci.yml` |
| Web portal CI | `.github/workflows/web-portal.yml` |
| Remote CI dispatch | `.github/workflows/remote-ci.yml` |
| Branch enforcement | `.github/workflows/branch-flow-enforcement.yml` |

### Credentials & Secrets

| Information | Location |
|-------------|----------|
| Kubeconfigs | `~/kubeconfigs/kubeconfig-<env>.yaml` |
| Self-signed certs | `infra/secrets/certs/` |
| Environment variables | `secrets/env/.env` |

## Common Tasks - Where to Look

### Deploying a Service

1. **Read deployment spec first**: `services/<name>/DEPLOYMENT.md`
2. Check branch workflow: [ci-cd-pipeline.md](ci-cd-pipeline.md) or `docs/development/branching-workflow.md`
3. Find Helm chart: `services/<name>/deployments/helm/<name>/`
4. Check ArgoCD app: `gitops/clusters/<env>/apps/<name>.yaml`
5. Verify endpoint: [endpoints-and-urls.md](endpoints-and-urls.md)

### Debugging Pod Issues

1. Get cluster access: [environment-access.md](environment-access.md)
2. **Check deployment spec**: `services/<name>/DEPLOYMENT.md` (for health endpoints, env vars)
3. Check service config: `services/<name>/deployments/helm/<name>/values-<env>.yaml`
4. View logs: [observability-guide.md](observability-guide.md)
5. Check health endpoints: [endpoints-and-urls.md](endpoints-and-urls.md)

### Fixing ArgoCD Sync

1. Understand sync policy: [ci-cd-pipeline.md](ci-cd-pipeline.md)
2. Check ArgoCD app definition: `gitops/clusters/<env>/apps/`
3. Troubleshoot: [argocd-testing-guide.md](argocd-testing-guide.md)
4. Access ArgoCD UI: [environment-access.md](environment-access.md)

### Certificate Issues

1. Understand architecture: [certificate-architecture.md](certificate-architecture.md)
2. Generate/trust certs: [tls-ssl-setup.md](tls-ssl-setup.md)
3. Check cert files: `infra/secrets/certs/`

### CI/CD Pipeline Issues

1. Find workflow: `.github/workflows/`
2. Understand patterns: [github-actions-guide.md](github-actions-guide.md)
3. Check branch rules: [ci-cd-pipeline.md](ci-cd-pipeline.md)

## Environment Quick Reference

### Development

| Resource | Value |
|----------|-------|
| Branch | `develop` |
| ArgoCD sync | Automated |
| Kubeconfig | `~/kubeconfigs/kubeconfig-development.yaml` |
| Ingress IP | `172.232.58.222` |
| Apps location | `gitops/clusters/development/apps/` |

### Staging

| Resource | Value |
|----------|-------|
| Branch | `staging` |
| ArgoCD sync | Automated |
| Kubeconfig | `~/kubeconfigs/kubeconfig-staging.yaml` |
| Apps location | `gitops/clusters/staging/apps/` |

### Production

| Resource | Value |
|----------|-------|
| Branch | `main` |
| ArgoCD sync | Manual |
| Kubeconfig | `~/kubeconfigs/kubeconfig-production.yaml` |
| Apps location | `gitops/clusters/production/apps/` |

## Services Inventory

| Service | Namespace | Helm Chart Location |
|---------|-----------|---------------------|
| api-router-service | development | `services/api-router-service/deployments/helm/api-router-service/` |
| user-org-service | user-org-service | `services/user-org-service/deployments/helm/user-org-service/` |
| analytics-service | development | `services/analytics-service/deployments/helm/analytics-service/` |
| admin-api-service | admin-api-service | `services/admin-api-service/deployments/helm/admin-api-service/` |
| web-portal | development | `web/portal/deployments/helm/web-portal/` |

## Verification Commands

```bash
# Check all ingresses
kubectl get ingress -A

# Check ArgoCD apps
kubectl get applications -n argocd

# Check all pods
kubectl get pods -A

# Check services
kubectl get svc -A

# View ArgoCD app status
argocd app list

# Check Helm releases
helm list -A
```

## Related Runbooks

| Runbook | Location | Purpose |
|---------|----------|---------|
| Linode Setup | `docs/runbooks/linode-setup.md` | Initial Linode/LKE setup |
| ArgoCD Deployment | `docs/runbooks/argocd-deployment-workflow.md` | Deployment procedures |
| ArgoCD Bootstrap | `docs/runbooks/argocd-bootstrap.md` | Bootstrap ArgoCD |
| Infrastructure Rollback | `docs/runbooks/infrastructure-rollback.md` | Rollback procedures |
| Infrastructure Troubleshooting | `docs/runbooks/infrastructure-troubleshooting.md` | Issue resolution |
| Deploy to Environments | `docs/runbooks/deploy-to-environments.md` | Environment deployments |
| Migrations | `docs/runbooks/migrations.md` | Database migrations |
