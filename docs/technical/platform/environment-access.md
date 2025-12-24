# Environment Access Guide

This document provides quick access information for all environments and services in the AI-AAS platform.

## Important Note on Credentials

**All actual credentials are stored in encrypted files**:
- Database passwords, API keys, tokens: `secrets/env/.env` (git-crypt encrypted)
- Kubeconfigs: `secrets/kubeconfigs/` directory (git-crypt encrypted)
- SSH keys: `secrets/` directory (git-crypt encrypted)

**To access encrypted secrets**:
```bash
git-crypt unlock
```

## Quick Reference

### Development Environment

**Kubernetes Cluster**
- Kubeconfig: `secrets/kubeconfigs/kubeconfig-development.yaml` (encrypted with git-crypt)
- Context: Use with `--kubeconfig` flag
- Access: `kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml`

**ArgoCD**
- URL: https://argocd.dev.otherjamesbrown.com
- Username: `admin`
- Password: Retrieve with `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 --decode`
- Alternative: Check `secrets/env/.env` for stored credentials

**Database (PostgreSQL)**
- Host: Found in `secrets/env/.env` as `DATABASE_URL`
- Connection String: Found in `secrets/env/.env` as `DATABASE_URL`
- Format: `postgresql://username:password@host:port/database?sslmode=require`

**API Endpoints**
- API Router: https://api.dev.otherjamesbrown.com or https://api.dev.otherjamesbrown.com
- User Org Service: http://172.232.58.222 (via ingress) or `kubectl port-forward -n user-org-service svc/user-org-service-development-user-org-service 18081:8081`
- Ingress IP: `172.232.58.222`

**API Keys**
- Master Admin API Key: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY`
- API Key ID: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY_ID`
- Master Admin User ID: Found in `secrets/env/.env` as `MASTER_ADMIN_USER_ID`
- Master Admin Org ID: Found in `secrets/env/.env` as `MASTER_ADMIN_ORG_ID`

**Admin CLI**
- Binary: `bin/admin-cli` (or `bin/admin-cli-mac` for macOS)
- Configuration:
  ```bash
  # Load from secrets/env/.env
  export ADMIN_CLI_USER_ORG_ENDPOINT=http://172.232.58.222
  export ADMIN_CLI_API_KEY=$(grep MASTER_ADMIN_API_KEY secrets/env/.env | cut -d'=' -f2)
  ```
- Usage: `./bin/admin-cli org list` or `./bin/admin-cli user list --org-id=<uuid>`

### Production Environment

**Kubernetes Cluster**
- Kubeconfig: `secrets/kubeconfigs/kubeconfig-production.yaml` (encrypted with git-crypt)
- Context: Use with `--kubeconfig` flag

**ArgoCD**
- URL: https://argocd.prod.otherjamesbrown.com
- Username: `admin`
- Password: Same retrieval method as development

## Secrets Management

### Git-Crypt
All sensitive files are encrypted with git-crypt. To unlock:
```bash
git-crypt unlock
```

Check encryption status:
```bash
git-crypt status
```

### Environment Files
- Development: `secrets/env/.env`
- Contains all credentials, tokens, and connection strings
- Format: `KEY=value` (one per line)

### SSH Keys
- Location: Check `secrets/` directory for SSH keys
- Linode/Infrastructure: Keys stored in `.env` as `LINODE_TOKEN`

## Service Access Patterns

### Port Forwarding (Development)
```bash
# User Org Service
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  port-forward -n user-org-service \
  svc/user-org-service-development-user-org-service 18081:8081

# ArgoCD (if needed)
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  port-forward -n argocd \
  svc/argocd-server 8080:443
```

### Direct Database Access
```bash
# Load database credentials from secrets/env/.env first
source secrets/env/.env

# Using psql from kubectl
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  run psql-temp --rm -i --image=postgres:15 \
  --env="PGPASSWORD=${DATABASE_PASSWORD}" -- \
  psql -h ${DATABASE_HOST} -p ${DATABASE_PORT} -U ${DATABASE_USER} -d ${DATABASE_NAME}

# Or with local psql client
export PGPASSWORD=${DATABASE_PASSWORD}
psql -h ${DATABASE_HOST} -p ${DATABASE_PORT} -U ${DATABASE_USER} -d ${DATABASE_NAME}
```

## GitHub Credentials

**Tokens (from secrets/env/.env)**
- All GitHub tokens are stored in `secrets/env/.env` with prefix `GITHUB_`
- GitHub Actions Key: `GITHUB_ACTIONS_KEY`
- GitHub AI-AAS Remote CI: `GITHUB_AI_AAS_REMOTE_CI`
- GitHub AI-AAS Full: `GITHUB_AI_AAS_FULL`
- GitHub PAT (Fine-grained): `GITHUB_PAT`

## Linode Infrastructure

**API Token**
- Token: Found in `secrets/env/.env` as `LINODE_TOKEN`
- Default Region: `fr-par`
- Default Firewall: `akamai-home`

**Object Storage**
- Access Key: Found in `secrets/env/.env` as `LINODE_OBJECT_STORAGE_ACCESS_KEY`
- Secret Key: Found in `secrets/env/.env` as `LINODE_OBJECT_STORAGE_SECRET_KEY`

## Redis

**Development**
- URL: `redis://localhost:6379` (local)
- Cluster: `redis.development.svc.cluster.local:6379`
- Password: Found in `secrets/env/.env` as `REDIS_PASSWORD`

## Troubleshooting

### Cannot Access Kubernetes
1. Check if kubeconfig is decrypted: `git-crypt status`
2. Unlock if needed: `git-crypt unlock`
3. Verify kubeconfig path: `ls -la secrets/kubeconfigs/`

### ArgoCD Not Accessible
1. Check if ArgoCD is running: `kubectl get pods -n argocd`
2. Port-forward if URL doesn't work: `kubectl port-forward -n argocd svc/argocd-server 8080:443`
3. Access via: `https://localhost:8080`

### API Key Authentication Failing
1. Check API key in database:
   ```sql
   SELECT api_key_id, fingerprint, status, expires_at
   FROM api_keys
   WHERE api_key_id = '<api-key-id-from-env>';
   ```
2. Verify status is 'active'
3. Check expiration date

## Important Notes

- **NEVER commit unencrypted credentials** to git
- All credentials in `secrets/env/.env` are encrypted with git-crypt
- Rotate credentials regularly
- Use API keys for service-to-service communication
- Use OAuth for user authentication
- Master admin credentials are for emergency access only

## References

- GitOps Workflow: `docs/technical/workflows/argocd-deployment-workflow.md`
- Database Setup: `docs/technical/workflows/migrations.md`
- Infrastructure: `docs/technical/platform/infrastructure-overview.md`
- Service Deployment: `docs/technical/workflows/deploy-to-environments.md`
