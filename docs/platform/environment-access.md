# Environment Access Guide

---
last_updated: 2025-12-08
document_type: reference
---

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
- URL: https://argocd.dev.ai-aas.local
- Username: `admin`
- Password: Retrieve with `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 --decode`
- Alternative: Check `secrets/env/.env` for stored credentials

**Database (PostgreSQL)**
- Host: Found in `secrets/env/.env` as `DATABASE_URL`
- Connection String: Found in `secrets/env/.env` as `DATABASE_URL`
- Format: `postgresql://username:password@host:port/database?sslmode=require`

**API Endpoints**
- API Router: https://api.dev.otherjamesbrown.com or https://api.dev.ai-aas.local
- Admin API: https://admin-api.dev.otherjamesbrown.com or https://admin-api.dev.ai-aas.local
- User Org Service: https://user-org.dev.otherjamesbrown.com or https://user-org.dev.ai-aas.local
- Ingress IP: `172.232.58.222`

**Monitoring & Observability**
- Grafana: https://grafana.dev.otherjamesbrown.com or https://grafana.dev.ai-aas.local
- Loki (Log Aggregation): https://loki.dev.otherjamesbrown.com or https://loki.dev.ai-aas.local
- Loki API: `https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range`

**API Keys**
- Master Admin API Key: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY`
- API Key ID: Found in `secrets/env/.env` as `MASTER_ADMIN_API_KEY_ID`
- Master Admin User ID: Found in `secrets/env/.env` as `MASTER_ADMIN_USER_ID`
- Master Admin Org ID: Found in `secrets/env/.env` as `MASTER_ADMIN_ORG_ID`

**AI-AAS CLI**
- Binary: `services/ai-aas-cli/bin/ai-aas-cli` (build with `cd services/ai-aas-cli && make build`)
- Configuration: Uses profile-based configuration
  ```bash
  # Configure CLI for an environment
  ai-aas-cli profile create dev \
    --admin-api-url=https://admin-api.dev.otherjamesbrown.com \
    --api-key=$(grep MASTER_ADMIN_API_KEY secrets/env/.env | cut -d'=' -f2)
  ai-aas-cli profile use dev
  ```
- Usage: `ai-aas-cli status`, `ai-aas-cli model list`, `ai-aas-cli model deploy create <model>`

### Production Environment

**Kubernetes Cluster**
- Kubeconfig: `secrets/kubeconfigs/kubeconfig-production.yaml` (encrypted with git-crypt)
- Context: Use with `--kubeconfig` flag

**ArgoCD**
- URL: https://argocd.prod.ai-aas.local
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

### Log Access (Development)

**Via Loki HTTP API (Recommended - No port-forward needed):**
```bash
# Query all errors in last 15 minutes
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={level="error"}' \
  --data-urlencode 'limit=50' \
  --data-urlencode 'since=15m' | jq '.data.result'

# Query specific service
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service"}' \
  --data-urlencode 'limit=100' | jq '.data.result'

# Search by request_id across all services
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={} |= "<request-id>"' \
  --data-urlencode 'limit=50' | jq '.data.result'

# Query with multiple filters
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={service="user-org-service", level=~"error|warn"}' \
  --data-urlencode 'limit=100' | jq '.data.result'
```

**Via kubectl logs (Real-time streaming):**
```bash
# Stream logs from a service
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs -f deployment/api-router-service -n default --tail=100

# Check previous container logs (after restart)
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  logs deployment/api-router-service -n default --previous
```

**Via Grafana (Visual exploration):**
- Open https://grafana.dev.otherjamesbrown.com
- Navigate to Explore → Select Loki datasource
- Use LogQL queries: `{service="api-router-service", level="error"}`

### Port Forwarding (Fallback)
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

### Cannot Access Loki/Grafana
1. Check if monitoring namespace exists: `kubectl get ns monitoring`
2. Check if pods are running: `kubectl get pods -n monitoring`
3. Check ingress is configured: `kubectl get ingress -n monitoring`
4. Verify DNS resolves: `nslookup loki.dev.otherjamesbrown.com`
5. Test direct service access (fallback):
   ```bash
   kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
     port-forward -n monitoring svc/loki 3100:3100
   curl http://localhost:3100/ready
   ```

### No Logs Appearing in Loki
1. Check Promtail is running: `kubectl get pods -n monitoring -l app=promtail`
2. Check Promtail logs: `kubectl logs -n monitoring -l app=promtail --tail=50`
3. Verify pod annotations: `kubectl get pods -o yaml | grep logging.enabled`
4. Check Loki is receiving data: `curl https://loki.dev.otherjamesbrown.com/ready`

## Important Notes

- **NEVER commit unencrypted credentials** to git
- All credentials in `secrets/env/.env` are encrypted with git-crypt
- Rotate credentials regularly
- Use API keys for service-to-service communication
- Use OAuth for user authentication
- Master admin credentials are for emergency access only

## Related Documentation

- [Infrastructure Overview](infrastructure-overview.md) - Architecture and directory structure
- [Endpoints and URLs](endpoints-and-urls.md) - All service endpoints
- [Observability Guide](observability-guide.md) - Monitoring, logging, dashboards
- [ArgoCD Deployment Workflow](../runbooks/argocd-deployment-workflow.md) - Deployment procedures
- [Database Migrations](../runbooks/migrations.md) - Database setup
- [Deploy to Environments](../runbooks/deploy-to-environments.md) - Environment deployments
