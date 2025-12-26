# Infrastructure Access Control Guide

---
last_updated: 2025-12-08
document_type: guide
---

This document covers access control for the AI-AAS platform infrastructure.

## Current Access Model

### Kubernetes Access

Access to Kubernetes clusters is managed via kubeconfig files:

| Environment | Kubeconfig Location |
|-------------|---------------------|
| Development | `~/kubeconfigs/kubeconfig-development.yaml` |
| Staging | `~/kubeconfigs/kubeconfig-staging.yaml` |
| Production | `~/kubeconfigs/kubeconfig-production.yaml` |

```bash
# Access development cluster
kubectl --kubeconfig=~/kubeconfigs/kubeconfig-development.yaml get pods -A

# Or set KUBECONFIG
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get pods -A
```

### ArgoCD Access

| Environment | URL | Username |
|-------------|-----|----------|
| Development | https://argocd.dev.otherjamesbrown.com | admin |
| Staging | https://argocd.staging.otherjamesbrown.com | admin |
| Production | https://argocd.prod.otherjamesbrown.com | admin |

Password retrieval:
```bash
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 --decode
```

### API Access

API endpoints require authentication via API keys:
- API keys are stored in `secrets/env/.env`
- Use `X-API-Key` header for authentication
- See [Environment Access](environment-access.md) for details

## Linode Access Setup

### Create Personal Access Token

1. Log into the Akamai Control Panel
2. Navigate to **My Profile → API Tokens → Add a Personal Access Token**
3. Grant permissions:
   - `linodes:read_write`
   - `lke:read_write`
   - `object-storage:read_write`
4. Store token securely (1Password or `secrets/env/.env`)

### Configure Environment

```bash
export LINODE_TOKEN=<token>
export LINODE_DEFAULT_REGION=fr-par
```

### Object Storage Credentials

For Terraform S3-compatible backend:

```bash
export LINODE_OBJECT_STORAGE_ACCESS_KEY=<access-key>
export LINODE_OBJECT_STORAGE_SECRET_KEY=<secret-key>
export AWS_ACCESS_KEY_ID=$LINODE_OBJECT_STORAGE_ACCESS_KEY
export AWS_SECRET_ACCESS_KEY=$LINODE_OBJECT_STORAGE_SECRET_KEY
```

Values stored in `secrets/env/.env` as `LINODE_OBJECT_STORAGE_ACCESS_KEY` / `LINODE_OBJECT_STORAGE_SECRET_KEY`.

### CLI Tools

- Linode CLI: https://www.linode.com/docs/products/tools/cli/
- Object Storage commands require cluster flag:
  ```bash
  linode-cli obj ls --cluster fr-par-1 ai-aas/terraform/environments/production
  ```

See [Linode Setup Runbook](../runbooks/linode-setup.md) for provisioning workflows.

## Secrets Management

### Git-Crypt

Sensitive files are encrypted with git-crypt:

```bash
# Unlock encrypted files
git-crypt unlock

# Check encryption status
git-crypt status
```

### Secret Locations

| Secret Type | Location |
|-------------|----------|
| Environment variables | `secrets/env/.env` |
| Kubeconfigs | `~/kubeconfigs/` |
| TLS certificates | `infra/secrets/certs/` |

## Security Guidelines

- **NEVER** commit unencrypted credentials to git
- Rotate credentials regularly
- Use API keys for service-to-service communication
- Use OAuth for user authentication
- Master admin credentials are for emergency access only

## Related Documentation

- [Environment Access](environment-access.md) - Complete credential reference
- [Linode Setup](../runbooks/linode-setup.md) - Linode provisioning
- [TLS/SSL Setup](tls-ssl-setup.md) - Certificate management

---

## Planned Features (Not Yet Implemented)

> **Note**: The following sections describe planned access control features that are not yet implemented. Scripts referenced do not exist yet.

### Roles & Responsibilities (Planned)

| Role | Scope | Capabilities |
|------|-------|--------------|
| `platform-engineer` | Cluster-wide | Terraform applies, ArgoCD admin, secrets rotation |
| `app-team` | Namespaced | Deploy workloads, read metrics/logs |
| `read-only` | Namespaced | kubectl get access, Grafana read-only |
| `break-glass` | Cluster-wide | Temporary admin access (≤8h) for incidents |

### Access Package Issuance (Planned)

```bash
# NOT YET IMPLEMENTED
./scripts/infra/access-package.sh \
  --env staging \
  --role app-team \
  --expires-in 8h \
  --ticket INFRA-1234
```

### Credential Rotation (Planned)

```bash
# NOT YET IMPLEMENTED
./scripts/infra/secrets/rotate.sh --env production --role app-team
```

### Break-Glass Procedure (Planned)

1. Incident commander requests access in `#platform-incident`
2. Two-factor confirmation by Director
3. Platform engineer issues package with `--role break-glass --expires-in 4h`
4. Access automatically revoked at expiration
