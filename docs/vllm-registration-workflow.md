# vLLM Model Registration Workflow

This document describes the workflow for registering vLLM model deployments in the model registry to enable API routing.

**Feature**: `010-vllm-deployment` (User Story 2 - Register models for routing)
**Last Updated**: 2025-01-27

## Overview

After deploying a vLLM model to Kubernetes, you must register it in the model registry to make it available for API routing. The API Router Service queries the model registry to find deployment endpoints and route client requests to the appropriate vLLM instances.

## Prerequisites

- vLLM model deployed to Kubernetes using Helm chart
- admin-cli binary built and available in PATH
- DATABASE_URL environment variable configured
- PostgreSQL database with model_registry_entries table

## Registration Methods

### Method 1: Automatic Registration (Post-Deployment Hook)

Use the `register-model.sh` script after deploying a model:

```bash
# Deploy model with Helm
helm install llama-2-7b infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-development.yaml \
  --namespace system

# Wait for deployment to be ready
kubectl wait --for=condition=Available deployment/llama-2-7b-development \
  -n system --timeout=600s

# Register the model
./scripts/vllm/register-model.sh llama-2-7b development system
```

**Output:**
```
[INFO] Registering model deployment...
[INFO]   Model: llama-2-7b
[INFO]   Environment: development
[INFO]   Namespace: system
[INFO]   Endpoint: llama-2-7b-development.system.svc.cluster.local:8000
[INFO] Verifying deployment...
[INFO] ✓ Deployment is healthy (1/1 replicas ready)
[INFO] Registering model in registry...
[INFO] ✓ Model registered successfully
```

### Method 2: Manual Registration (admin-cli)

Register a model directly using the admin-cli:

```bash
admin-cli registry register \
  --model-name llama-2-7b \
  --endpoint llama-2-7b-development.system.svc.cluster.local:8000 \
  --environment development \
  --namespace system
```

**Flags:**
- `--model-name`: Model identifier (required)
- `--endpoint`: Kubernetes service endpoint with port (required)
- `--environment`: Deployment environment (development, staging, production)
- `--namespace`: Kubernetes namespace (default: system)
- `--dry-run`: Preview registration without applying changes
- `--format`: Output format (table, json, csv)

## Registration Workflow

1. **Deploy Model**: Use Helm to deploy vLLM model to Kubernetes
2. **Verify Health**: Ensure deployment is healthy and pods are ready
3. **Register Model**: Use admin-cli to register deployment in model registry
4. **Verify Routing**: Test that API Router can route requests to the model
5. **Monitor**: Check health status and routing metrics

## Endpoint Naming Convention

Model endpoints follow this predictable naming pattern:

```
{model-name}-{environment}.{namespace}.svc.cluster.local:8000
```

**Examples:**
- `llama-2-7b-development.system.svc.cluster.local:8000`
- `gpt-neo-production.system.svc.cluster.local:8000`
- `mistral-7b-staging.inference.svc.cluster.local:8000`

This naming convention ensures:
- Predictable endpoint URLs
- Environment isolation
- Easy troubleshooting
- Consistent routing configuration

## Registry Status Management

### Enable a Model

Enable a previously disabled model for routing:

```bash
admin-cli registry enable \
  --model-name llama-2-7b \
  --environment development
```

### Disable a Model

Temporarily disable a model without deregistering:

```bash
admin-cli registry disable \
  --model-name llama-2-7b \
  --environment development
```

### Deregister a Model

Permanently remove a model from routing (sets status to 'disabled'):

```bash
admin-cli registry deregister \
  --model-name llama-2-7b \
  --environment development
```

### List Registered Models

View all registered models:

```bash
# List all models
admin-cli registry list

# Filter by environment
admin-cli registry list --environment production

# Filter by status
admin-cli registry list --status ready

# JSON output
admin-cli registry list --format json
```

**Example Output:**
```
Found 3 model deployment(s)

ID  Model Name    Endpoint                                           Status  Environment  Namespace  Last Health         Updated
1   llama-2-7b    llama-2-7b-development.system.svc.cluster.local:8000  ready   development  system                     2025-01-27 10:30
2   gpt-neo       gpt-neo-production.system.svc.cluster.local:8000      ready   production   system     2025-01-27 10:25  2025-01-27 10:20
3   mistral-7b    mistral-7b-staging.system.svc.cluster.local:8000      disabled staging     system                     2025-01-27 09:15
```

## API Router Integration

Once registered, the API Router Service automatically routes requests to the model:

```bash
# Client request to API Router
curl -X POST http://api-router:8080/v1/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <api-key>' \
  -d '{
    "model": "llama-2-7b",
    "prompt": "Hello, how are you?",
    "max_tokens": 50
  }'
```

**Routing Flow:**
1. Client sends request to API Router with model name
2. API Router queries model registry for "llama-2-7b" in current environment
3. Registry returns deployment endpoint (from cache or database)
4. API Router forwards request to vLLM deployment
5. vLLM processes request and returns completion
6. API Router returns response to client

## Redis Caching

The model registry uses Redis caching for performance:

- **Cache TTL**: 2 minutes (configurable)
- **Cache Key**: `model_registry:{environment}:{model-name}`
- **Invalidation**: Automatic on status changes (enable/disable/deregister)

To manually invalidate cache (if needed):

```bash
# Via Redis CLI
redis-cli DEL "model_registry:development:llama-2-7b"
```

## Environment Separation

Models are isolated by environment:

- **Development**: `development` - for testing and development
- **Staging**: `staging` - for pre-production validation
- **Production**: `production` - for production workloads

The same model can be deployed in multiple environments with separate registrations:

```bash
# Register in development
admin-cli registry register \
  --model-name llama-2-7b \
  --endpoint llama-2-7b-development.system.svc.cluster.local:8000 \
  --environment development

# Register in production (different endpoint)
admin-cli registry register \
  --model-name llama-2-7b \
  --endpoint llama-2-7b-production.system.svc.cluster.local:8000 \
  --environment production
```

## Troubleshooting

### Model Not Found

**Error**: `model not found in registry: llama-2-7b`

**Solutions:**
1. Verify model is registered:
   ```bash
   admin-cli registry list --environment development | grep llama-2-7b
   ```

2. Check deployment status:
   ```bash
   admin-cli registry list --model-name llama-2-7b
   ```

3. Re-register if needed:
   ```bash
   ./scripts/vllm/register-model.sh llama-2-7b development
   ```

### Model Not Ready

**Error**: `model not ready: llama-2-7b (status: disabled)`

**Solutions:**
1. Enable the model:
   ```bash
   admin-cli registry enable --model-name llama-2-7b --environment development
   ```

2. Check deployment health in Kubernetes:
   ```bash
   kubectl get deployment llama-2-7b-development -n system
   kubectl get pods -l app=llama-2-7b -n system
   ```

### Database Connection Errors

**Error**: `failed to connect to database`

**Solutions:**
1. Verify DATABASE_URL is set:
   ```bash
   echo $DATABASE_URL
   ```

2. Test database connectivity:
   ```bash
   psql "$DATABASE_URL" -c "SELECT 1;"
   ```

3. Check database migrations:
   ```bash
   # Verify model_registry_entries table exists
   psql "$DATABASE_URL" -c "\d model_registry_entries"
   ```

### Cache Issues

If routing seems stale (changes not reflected):

1. Clear Redis cache:
   ```bash
   redis-cli FLUSHDB  # Clears all cache (use with caution)
   ```

2. Or clear specific model:
   ```bash
   redis-cli DEL "model_registry:development:llama-2-7b"
   ```

3. Restart API Router Service to force refresh

## Best Practices

1. **Always verify deployment health** before registering
2. **Use dry-run mode** when testing registration commands
3. **Monitor registration propagation** (typically < 2 minutes due to cache TTL)
4. **Keep environment naming consistent** across deployments
5. **Document custom model configurations** in values files
6. **Use scripted registration** for automation and consistency
7. **Test routing** after registration to verify end-to-end flow

## Integration with CI/CD

Example GitLab CI/CD pipeline step:

```yaml
deploy-and-register:
  stage: deploy
  script:
    # Deploy model
    - helm upgrade --install llama-2-7b infra/helm/charts/vllm-deployment \
        --values infra/helm/charts/vllm-deployment/values-${CI_ENVIRONMENT_NAME}.yaml \
        --namespace system \
        --wait --timeout 10m

    # Register model
    - ./scripts/vllm/register-model.sh llama-2-7b ${CI_ENVIRONMENT_NAME} system

    # Verify routing
    - curl -f http://api-router:8080/v1/completions \
        -H 'Content-Type: application/json' \
        -d "{\"model\": \"llama-2-7b\", \"prompt\": \"test\", \"max_tokens\": 1}"
  environment:
    name: $CI_ENVIRONMENT_NAME
```

## See Also

- [Model Deployment Workflow](./deployment-workflow.md)
- [Model Initialization Timeouts](./model-initialization.md)
- [API Router Service Documentation](../services/api-router-service/README.md)
- [vLLM Deployment Spec](../specs/010-vllm-deployment/spec.md)
