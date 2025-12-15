# Routing Policy Management

## Overview

Routing policies control how API requests are routed to vLLM model backends. They map (organization, model) pairs to backend endpoints with configurable load balancing weights and failover thresholds.

## Policy Architecture

### Global Policies

Global policies use `organization_id: "*"` to apply to all organizations. This is the recommended approach for most deployments, allowing all organizations to access all deployed models.

**Benefits:**
- Simplified management: One policy per model
- Consistent access: All organizations can use all models
- Easy to add new models: Just deploy and create a global policy

### Organization-Specific Policies

Organization-specific policies override global policies for particular organizations. Use cases:
- **Premium tier access**: Give specific organizations access to premium/large models
- **Rate limiting**: Route specific organizations to dedicated backends with rate limits
- **Regional routing**: Route organizations to region-specific backends
- **A/B testing**: Route specific organizations to different model versions

**Precedence:** Organization-specific policies take precedence over global policies.

## Policy Structure

Policies are stored in etcd at `/ai-aas/routing/policies/` with the following structure:

```json
{
  "policy_id": "*:gpt-oss-20b",
  "organization_id": "*",
  "model": "gpt-oss-20b",
  "backends": [
    {
      "backend_id": "gpt-oss-20b-vllm-deployment",
      "weight": 100
    }
  ],
  "failover_threshold": 3,
  "updated_at": "2025-11-23T14:22:05Z",
  "version": 1700000000
}
```

**Fields:**
- `policy_id`: Unique identifier in format `{org_id}:{model}`
- `organization_id`: Organization ID or `"*"` for global
- `model`: Model name as specified in API requests
- `backends`: Array of backend IDs with routing weights (must sum to 100)
- `failover_threshold`: Number of consecutive failures before failover
- `updated_at`: Last update timestamp
- `version`: Version number for optimistic locking

## Admin CLI Commands

### Create or Update Policy

Create a global policy (recommended):

```bash
admin-cli routing policy create \
  --global \
  --model gpt-oss-20b \
  --backends gpt-oss-20b-vllm-deployment:100
```

Create an organization-specific policy:

```bash
admin-cli routing policy create \
  --org-id aa6f9015-132a-4694-8b10-7d4d4550faed \
  --model gpt-4-turbo \
  --backends "gpt4-backend-1:70,gpt4-backend-2:30"
```

**Flags:**
- `--global`: Create a global policy (mutually exclusive with `--org-id`)
- `--org-id`: Organization ID for org-specific policy
- `--model`: Model name (required)
- `--backends`: Comma-separated list of `backend_id:weight` pairs (required, weights must sum to 100)
- `--format`: Output format (table, json)
- `--quiet`: Suppress non-error output
- `--dry-run`: Simulate creation without applying changes

### List Policies

List all routing policies:

```bash
admin-cli routing policy list
```

Output in JSON format:

```bash
admin-cli routing policy list --format json
```

### Delete Policy

Delete a global policy:

```bash
admin-cli routing policy delete \
  --global \
  --model gpt-oss-20b
```

Delete an organization-specific policy:

```bash
admin-cli routing policy delete \
  --org-id aa6f9015-132a-4694-8b10-7d4d4550faed \
  --model gpt-4-turbo
```

## Workflow Integration

### Model Deployment Workflow

1. Deploy vLLM model using Helm chart
2. Verify deployment health
3. **Create global routing policy** (this step)
4. Register model in model registry (optional)
5. Test with API requests

Example:

```bash
# 1. Deploy model
helm install gpt-oss-20b infra/helm/charts/vllm-deployment \
  -f values-gpt-oss-20b.yaml \
  --namespace system

# 2. Wait for ready
kubectl wait --for=condition=ready pod \
  -l app.kubernetes.io/instance=gpt-oss-20b \
  -n system --timeout=600s

# 3. Create routing policy
admin-cli routing policy create \
  --global \
  --model gpt-oss-20b \
  --backends gpt-oss-20b-vllm-deployment:100

# 4. Test via API
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

### Future Enhancement: Auto-Policy Creation

In future versions, `admin-cli registry register` will automatically create global routing policies when registering new models. This will simplify the workflow to:

1. Deploy vLLM model
2. Register model (creates policy automatically)
3. Test via API

## Load Balancing

### Single Backend

Most deployments use a single backend with 100% weight:

```bash
--backends backend-id:100
```

### Multiple Backends

For high-availability or A/B testing, distribute traffic across multiple backends:

```bash
--backends "backend-1:70,backend-2:30"
```

**Requirements:**
- Weights must be integers between 0-100
- Total weight must equal 100
- At least one backend required

## Failover

The `failover_threshold` (default: 3) controls how many consecutive failures trigger a failover to the next available backend. The API router tracks failure counts per backend and automatically fails over when the threshold is reached.

## Configuration

The admin-CLI connects to etcd using the `config-service` endpoint. Configure in `~/.admin-cli/config.yaml`:

```yaml
api-endpoints:
  config-service: "localhost:2379"  # etcd endpoint
```

Or via environment variable:

```bash
export ADMIN_CLI_API_ENDPOINTS_CONFIG_SERVICE="localhost:2379"
```

Default: `localhost:2379`

## Troubleshooting

### "no routing policy configured" Error

**Symptom:** API requests fail with error:
```json
{"error": "no routing policy configured", "code": "ROUTING_ERROR"}
```

**Solution:** Create a global routing policy for the model:

```bash
admin-cli routing policy create \
  --global \
  --model <model-name> \
  --backends <backend-id>:100
```

### Backend Weights Don't Sum to 100

**Error:**
```
backend weights must sum to 100, got 75
```

**Solution:** Adjust backend weights to sum exactly to 100:

```bash
# Before (incorrect)
--backends "backend-1:50,backend-2:25"

# After (correct)
--backends "backend-1:75,backend-2:25"
```

### Cannot Connect to etcd

**Error:**
```
failed to connect to etcd: context deadline exceeded
```

**Solutions:**
1. Verify etcd is running:
   ```bash
   kubectl get pods -n development -l app=etcd
   ```

2. Set up port-forward if running locally:
   ```bash
   kubectl port-forward -n development svc/etcd-service 2379:2379
   ```

3. Check configuration:
   ```bash
   echo $ADMIN_CLI_API_ENDPOINTS_CONFIG_SERVICE
   ```

## Related Documentation

- [vLLM Deployment Workflow](./deployment-workflow.md)
- [API Router Architecture](../services/api-router-service/docs/router-architecture.md)
- [Admin CLI Guide](../services/admin-cli/README.md)
