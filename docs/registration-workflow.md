# Model Registration Workflow

This document describes the workflow for registering vLLM model deployments in the model registry for API routing.

## Overview

After deploying a model via Helm, it must be registered in the model registry (`model_registry_entries` table) to make it available for API routing through the API Router Service.

## Architecture

```
┌─────────────┐     1. Deploy      ┌──────────────┐
│ Helm Chart  │─────────────────>│ vLLM Pod     │
│ (vllm)      │                    │ (GPU Node)   │
└─────────────┘                    └──────────────┘
                                         │
                                         │ 2. Verify Ready
                                         ▼
┌─────────────┐     3. Register    ┌──────────────┐
│ admin-cli   │─────────────────>│ Registry DB  │
│ registry    │                    │ (PostgreSQL) │
└─────────────┘                    └──────────────┘
                                         │
                                         │ 4. Query
                                         ▼
                                   ┌──────────────┐
                                   │ API Router   │
                                   │ (Routing)    │
                                   └──────────────┘
```

## Prerequisites

- Model deployed via Helm and pod is Ready (1/1)
- Database connection configured (`DATABASE_URL` environment variable)
- `admin-cli` binary available
- Knowledge of:
  - Model name (as configured in Helm values)
  - Kubernetes service endpoint
  - Deployment environment (development, staging, production)
  - Namespace where model is deployed

## Step-by-Step Workflow

### Step 1: Deploy Model with Helm

Deploy the vLLM model using the Helm chart:

```bash
cd infra/helm/charts/vllm-deployment

helm install gpt-oss-20b . \
  -f values-unsloth-gpt-oss-20b.yaml \
  --namespace system \
  --set prometheus.serviceMonitor.enabled=false \
  --set preInstallChecks.enabled=false
```

### Step 2: Wait for Pod to be Ready

Wait for the pod to be Running and Ready (this can take 10-15 minutes for first-time model download):

```bash
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/instance=gpt-oss-20b \
  -n system \
  --timeout=20m
```

Or monitor manually:

```bash
kubectl get pods -n system -l app.kubernetes.io/instance=gpt-oss-20b -w
```

### Step 3: Verify Model is Serving

Before registering, verify the model is serving requests:

```bash
# Port-forward to the service
kubectl port-forward -n system svc/gpt-oss-20b-vllm-deployment 8000:8000 &

# Test the /v1/models endpoint
curl http://localhost:8000/v1/models

# Expected output:
# {
#   "object": "list",
#   "data": [
#     {
#       "id": "gpt-oss-20b",
#       "object": "model",
#       "created": 1234567890,
#       "owned_by": "vllm",
#       "root": "unsloth/gpt-oss-20b",
#       ...
#     }
#   ]
# }
```

### Step 4: Register Model in Registry

Register the model deployment using the admin CLI:

```bash
# Set database connection
export DATABASE_URL="postgresql://<DB_USER>:<DB_PASSWORD>@<DB_HOST>:5432/<DB_NAME>"

# Register the model
admin-cli registry register \
  --model-name gpt-oss-20b \
  --endpoint gpt-oss-20b-vllm-deployment.system.svc.cluster.local:8000 \
  --environment development \
  --namespace system
```

**Output:**
```
Registering model deployment...
  Model Name: gpt-oss-20b
  Endpoint: gpt-oss-20b-vllm-deployment.system.svc.cluster.local:8000
  Environment: development
  Namespace: system

✓ Model registered successfully in 0.15s
  ID: 42
  Status: ready
  Updated: 2025-01-27T12:00:00Z
```

### Step 5: Verify Registration

List registered models to confirm:

```bash
admin-cli registry list --environment development
```

**Output:**
```
Found 1 model deployment(s)

ID | Model Name    | Endpoint                                               | Status | Environment | Namespace | Last Health | Updated
42 | gpt-oss-20b   | gpt-oss-20b-vllm-deployment.system.svc.cluster.local:8000 | ready  | development | system    |             | 2025-01-27 12:00
```

### Step 6: Verify API Routing (When API Router Integration is Complete)

Once the API Router is integrated with the registry, test routing:

```bash
# Via the public API (assuming ingress is configured)
curl -X POST https://api.dev.ai-aas.local/v1/chat/completions \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 10
  }'
```

## Automated Registration

For automated deployments, use the provided registration script:

```bash
# After helm install
./scripts/register-model.sh gpt-oss-20b development system
```

See `scripts/register-model.sh` for implementation details.

## Registry Management Commands

### Register a Model

```bash
admin-cli registry register \
  --model-name MODEL_NAME \
  --endpoint SERVICE.NAMESPACE.svc.cluster.local:8000 \
  --environment ENVIRONMENT \
  --namespace NAMESPACE
```

**Parameters:**
- `--model-name`: Model identifier (must match the model served by vLLM)
- `--endpoint`: Kubernetes service endpoint with port
- `--environment`: `development`, `staging`, or `production`
- `--namespace`: Kubernetes namespace where pod is deployed

**Flags:**
- `--dry-run`: Simulate registration without making changes
- `--format json`: Output result as JSON
- `--quiet`: Suppress non-error output

### List Registered Models

```bash
# List all models
admin-cli registry list

# List models in specific environment
admin-cli registry list --environment production

# List only ready models
admin-cli registry list --status ready

# JSON output
admin-cli registry list --format json
```

### Disable a Model (Temporary)

Temporarily disable a model without removing its registration:

```bash
admin-cli registry disable \
  --model-name gpt-oss-20b \
  --environment development
```

The model remains in the registry with status `disabled` and will not be routed to by the API Router.

### Enable a Model

Re-enable a previously disabled model:

```bash
admin-cli registry enable \
  --model-name gpt-oss-20b \
  --environment development
```

### Deregister a Model

Permanently deregister a model (sets status to `disabled`):

```bash
admin-cli registry deregister \
  --model-name gpt-oss-20b \
  --environment development
```

## Endpoint Format

The endpoint must be a valid Kubernetes service endpoint with port:

**Format:** `<service-name>.<namespace>.svc.cluster.local:<port>`

**Examples:**
- `gpt-oss-20b-vllm-deployment.system.svc.cluster.local:8000`
- `llama-2-7b-development.models.svc.cluster.local:8000`
- `mixtral-production.system.svc.cluster.local:8000`

**Important:**
- Always include the port (`:8000` for vLLM)
- Use the full Kubernetes DNS name for reliability
- Ensure the service name matches the Helm release

## Environment Best Practices

### Development Environment

- **Purpose**: Testing, experimentation, rapid iteration
- **Namespace**: `system` or `development`
- **Characteristics**:
  - Frequent deployments and updates
  - May have unstable models
  - Lower resource allocation acceptable
- **Registration**: Automated via CI/CD or scripts

### Staging Environment

- **Purpose**: Pre-production validation, integration testing
- **Namespace**: `staging` or `system-staging`
- **Characteristics**:
  - Production-like configuration
  - Stable, validated models only
  - Full resource allocation
- **Registration**: Manual approval required

### Production Environment

- **Purpose**: Live customer traffic
- **Namespace**: `production` or `system-prod`
- **Characteristics**:
  - High availability required
  - Only promoted from staging
  - Full monitoring and alerting
- **Registration**: Requires review and approval

## Multi-Environment Deployments

The same model can be registered in multiple environments:

```bash
# Deploy to development
helm install gpt-oss-20b vllm-deployment \
  -f values-unsloth-gpt-oss-20b.yaml \
  -f values-development.yaml \
  --namespace system

# Register in development
admin-cli registry register \
  --model-name gpt-oss-20b \
  --endpoint gpt-oss-20b-vllm-deployment.system.svc.cluster.local:8000 \
  --environment development \
  --namespace system

# Later: Deploy to production
helm install gpt-oss-20b-prod vllm-deployment \
  -f values-unsloth-gpt-oss-20b.yaml \
  -f values-production.yaml \
  --namespace production

# Register in production (different endpoint)
admin-cli registry register \
  --model-name gpt-oss-20b \
  --endpoint gpt-oss-20b-prod-vllm-deployment.production.svc.cluster.local:8000 \
  --environment production \
  --namespace production
```

The registry stores one entry per `(model_name, environment)` combination.

## Troubleshooting

### Registration Fails: "Invalid endpoint format"

**Error:**
```
Error: invalid endpoint format: gpt-oss-20b
Validation Error: Endpoint must include port (e.g., service.namespace.svc.cluster.local:8000)
```

**Solution:** Include the full service DNS name with port:
```bash
--endpoint gpt-oss-20b-vllm-deployment.system.svc.cluster.local:8000
```

### Registration Fails: "Failed to connect to database"

**Error:**
```
Error: failed to connect to database: connection refused
```

**Solutions:**
1. Verify `DATABASE_URL` is set:
   ```bash
   echo $DATABASE_URL
   ```

2. Test database connectivity:
   ```bash
   psql $DATABASE_URL -c "SELECT 1"
   ```

3. Check network access to database
4. Verify database credentials

### Model Not Found After Registration

**Symptoms:**
- Registration succeeds
- `admin-cli registry list` shows model with status `ready`
- API Router returns 404 or "model not found"

**Causes:**
1. **API Router not yet integrated with registry** - API Router integration is pending
2. **Model name mismatch** - Model name in registry must match exactly what vLLM serves
3. **Cache invalidation needed** - API Router may cache routing decisions

**Solutions:**
1. Verify model name matches:
   ```bash
   # Check what vLLM is serving
   curl http://localhost:8000/v1/models | jq '.data[].id'

   # Should match registry entry
   admin-cli registry list --format json | jq '.[] | .model_name'
   ```

2. Wait for cache TTL (2 minutes) or restart API Router

### Pod Not Ready After Deployment

If the pod doesn't become Ready within 15-20 minutes, check:

```bash
# Check pod status
kubectl describe pod -n system -l app.kubernetes.io/instance=gpt-oss-20b

# Check pod logs
kubectl logs -n system -l app.kubernetes.io/instance=gpt-oss-20b

# Common issues:
# - GPU not available (check node resources)
# - Model download failure (check network/HuggingFace access)
# - Insufficient memory (check resource limits)
# - Image pull issues (check image tag and registry access)
```

Do not register a model until the pod is Ready and serving requests.

## Integration with CI/CD

For automated deployments, integrate registration into your CI/CD pipeline:

```yaml
# Example GitHub Actions workflow
- name: Deploy vLLM Model
  run: |
    helm install ${{ env.MODEL_NAME }} vllm-deployment \
      -f values-${{ env.MODEL_NAME }}.yaml \
      --namespace system \
      --wait --timeout 20m

- name: Register Model
  run: |
    admin-cli registry register \
      --model-name ${{ env.MODEL_NAME }} \
      --endpoint ${{ env.MODEL_NAME }}-vllm-deployment.system.svc.cluster.local:8000 \
      --environment ${{ env.ENVIRONMENT }} \
      --namespace system
  env:
    DATABASE_URL: ${{ secrets.DATABASE_URL }}
```

## Health Check Monitoring

The `last_health_check_at` field is reserved for future health check automation. Currently, it must be updated manually or via external health check services.

Planned feature: Automated health check service that periodically:
1. Tests model `/health` endpoint
2. Updates `last_health_check_at` timestamp
3. Updates `deployment_status` if health check fails

## See Also

- [Rollback Workflow](./rollback-workflow.md) - How to rollback deployments
- [Rollout Workflow](./rollout-workflow.md) - Safe deployment practices
- [Environment Separation](./environment-separation.md) - Multi-environment strategy
- [vLLM Deployment Guide](../specs/010-vllm-deployment/README.md) - Full specification
