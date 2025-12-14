# Secrets Management

This document describes how secrets are managed across the AI-AAS platform.

## Overview

The platform uses multiple secret management approaches depending on the component and environment:
- **Git-encrypted secrets**: Using `git-crypt` for sensitive credentials in the repository
- **Kubernetes Secrets**: For runtime secrets distributed to pods
- **Environment variables**: Loaded from encrypted `.env` files

## Master Admin API Key

### Authentication Model

The Admin API uses a **master admin API key** (`MASTER_ADMIN_API_KEY`) for authentication:
- This is the primary administrative credential with full platform access
- All Admin API requests must include this key in the `X-API-Key` header
- The key is a cryptographically secure random string with SHA-256 fingerprint

### Location

The `MASTER_ADMIN_API_KEY` is stored in:
- **File**: `secrets/env/.env` (git-crypt encrypted)
- **Variable**: `MASTER_ADMIN_API_KEY=<secret-value>`
- **See also**: `docs/admin/MASTER_ADMIN_SETUP.md` for complete details

### Operator Requirements

**CRITICAL**: All operators that need to communicate with the Admin API must have their `ADMIN_API_KEY` secret configured to match the `MASTER_ADMIN_API_KEY` value.

This includes:
- **AI Model Operator** (`operators/ai-model-operator/`)
  - Updates AIModel status via Admin API
  - Requires `ADMIN_API_KEY` environment variable
  - Secret must be deployed to operator namespace

### Creating Operator Secrets

When deploying an operator that needs Admin API access:

1. **Get the master admin API key**:
   ```bash
   # Load from encrypted secrets file
   source secrets/env/.env
   echo $MASTER_ADMIN_API_KEY
   ```

2. **Create Kubernetes Secret in operator namespace**:
   ```bash
   kubectl create secret generic ai-model-operator-secrets \
     --from-literal=ADMIN_API_KEY=$MASTER_ADMIN_API_KEY \
     -n ai-model-operator-system
   ```

3. **Mount secret in operator deployment**:
   ```yaml
   # operators/ai-model-operator/config/manager/manager.yaml
   env:
     - name: ADMIN_API_KEY
       valueFrom:
         secretKeyRef:
           name: ai-model-operator-secrets
           key: ADMIN_API_KEY
     - name: ADMIN_API_ENDPOINT
       value: "http://admin-api-service.system.svc.cluster.local:8080"
   ```

### Verification

To verify operator has correct API key:

```bash
# Check secret exists
kubectl get secret ai-model-operator-secrets -n ai-model-operator-system

# Check secret value matches (decode and compare)
kubectl get secret ai-model-operator-secrets -n ai-model-operator-system \
  -o jsonpath='{.data.ADMIN_API_KEY}' | base64 -d

# Check operator logs for authentication errors
kubectl logs -n ai-model-operator-system -l control-plane=controller-manager \
  | grep -i "auth\|api key\|401\|403"
```

## Other Secrets

### Database Credentials

**Location**: `secrets/env/.env`
**Variable**: `DATABASE_URL`
**Format**: `postgresql://user:password@host:port/database`

Used by:
- Admin API Service
- User-Org Service
- Analytics Service
- Migration jobs

### HuggingFace Token

**Location**: `secrets/env/.env`
**Variable**: `HF_TOKEN`
**Purpose**: Download models from HuggingFace Hub

Used by:
- Model downloader jobs
- vLLM pods (for gated models)

Deployment:
```bash
kubectl create secret generic huggingface-token \
  --from-literal=HF_TOKEN=$HF_TOKEN \
  -n system
```

### S3 Credentials

**Location**: `secrets/env/.env`
**Variables**:
- `S3_ENDPOINT`
- `S3_ACCESS_KEY_ID`
- `S3_SECRET_ACCESS_KEY`
- `S3_BUCKET`

Used by:
- Model downloader (to upload models)
- vLLM pods (to download cached models)

### ArgoCD Admin Password

**Location**: Kubernetes Secret `argocd-initial-admin-secret` in `argocd` namespace
**Retrieval**:
```bash
kubectl get secret argocd-initial-admin-secret -n argocd \
  -o jsonpath='{.data.password}' | base64 -d
```

**See**: `docs/platform/environment-access.md` for access details

## Secret Rotation

### Rotating Master Admin API Key

⚠️ **WARNING**: This affects all operators and services using the Admin API.

1. **Generate new API key**:
   ```bash
   python3 scripts/create-e2e-admin-key.py
   ```

2. **Update database**:
   ```sql
   UPDATE api_keys
   SET fingerprint = '<new-sha256-hash>',
       updated_at = NOW()
   WHERE api_key_id = 'b3a115c6-e4b4-4ed9-823c-250ebed4e3ec';
   ```

3. **Update encrypted secrets file**:
   ```bash
   # Edit secrets/env/.env
   MASTER_ADMIN_API_KEY=<new-key-value>

   # Commit with git-crypt
   git add secrets/env/.env
   git commit -m "chore: Rotate master admin API key"
   ```

4. **Update all operator secrets**:
   ```bash
   # For each operator namespace
   kubectl delete secret ai-model-operator-secrets -n ai-model-operator-system
   kubectl create secret generic ai-model-operator-secrets \
     --from-literal=ADMIN_API_KEY=$MASTER_ADMIN_API_KEY \
     -n ai-model-operator-system

   # Restart operators to pick up new secret
   kubectl rollout restart deployment ai-model-operator-controller-manager \
     -n ai-model-operator-system
   ```

5. **Update Admin CLI config**:
   ```bash
   # Edit ~/.admin-cli/config.yaml
   api_key: <new-key-value>
   ```

6. **Verify**:
   ```bash
   # Test Admin CLI
   admin-cli org list

   # Test operator (check logs for API errors)
   kubectl logs -n ai-model-operator-system -l control-plane=controller-manager --tail=50
   ```

### Rotating Database Password

1. **Create new password in database**:
   ```sql
   ALTER USER ai_aas_user WITH PASSWORD 'new-secure-password';
   ```

2. **Update secrets file**:
   ```bash
   # Edit secrets/env/.env
   DATABASE_URL=postgresql://ai_aas_user:new-secure-password@...

   git add secrets/env/.env
   git commit -m "chore: Rotate database password"
   ```

3. **Restart all services**:
   ```bash
   kubectl rollout restart deployment admin-api-service -n system
   kubectl rollout restart deployment user-org-service -n system
   kubectl rollout restart deployment analytics-service -n system
   ```

## Security Best Practices

1. **Never commit unencrypted secrets** to Git
2. **Use git-crypt** for all files in `secrets/` directory
3. **Rotate credentials** after suspected compromise
4. **Use least privilege**: Create service-specific API keys when possible
5. **Audit secret access**: Monitor `~/.admin-cli/audit.log` and service logs
6. **Encrypt secrets at rest** in Kubernetes (enable encryption provider)
7. **Use short-lived tokens** when possible (API keys should expire)

## Related Documentation

- [Master Admin Setup](../admin/MASTER_ADMIN_SETUP.md) - Complete master admin account details
- [Environment Access](./environment-access.md) - Cluster access and credentials
- [Bootstrap New Environment](../runbooks/bootstrap-new-environment.md) - Setting up secrets in new environments
- [Admin CLI README](../../services/admin-cli/README.md) - Using the Admin CLI

## Troubleshooting

### Operator Cannot Authenticate to Admin API

**Symptom**: Operator logs show `401 Unauthorized` or `403 Forbidden`

**Diagnosis**:
```bash
# Check if secret exists
kubectl get secret ai-model-operator-secrets -n ai-model-operator-system

# Check if secret value matches master key
OPERATOR_KEY=$(kubectl get secret ai-model-operator-secrets -n ai-model-operator-system \
  -o jsonpath='{.data.ADMIN_API_KEY}' | base64 -d)
echo "Operator key: $OPERATOR_KEY"
echo "Master key: $MASTER_ADMIN_API_KEY"
```

**Solution**: Recreate operator secret with correct master admin API key (see "Creating Operator Secrets" above)

### Services Cannot Connect to Database

**Symptom**: Service logs show `connection refused` or `authentication failed`

**Diagnosis**:
```bash
# Check DATABASE_URL format
source secrets/env/.env
echo $DATABASE_URL

# Test connection
psql $DATABASE_URL -c "SELECT 1;"
```

**Solution**:
- Verify database host, port, username, password in `DATABASE_URL`
- Ensure database service is running
- Check network policies allow traffic to database

### HuggingFace Downloads Fail

**Symptom**: Model downloader logs show `401 Unauthorized` from HuggingFace

**Diagnosis**:
```bash
# Check if HF_TOKEN secret exists
kubectl get secret huggingface-token -n system

# Verify token
source secrets/env/.env
curl -H "Authorization: Bearer $HF_TOKEN" https://huggingface.co/api/whoami
```

**Solution**:
- Recreate HuggingFace token secret
- Ensure token has read access to required models
- For gated models, accept model license on HuggingFace
