# Runbook: Model Not Accessible

This runbook guides you through diagnosing and resolving issues when a model appears deployed but is not accessible via the API.

## Symptoms

- Model shows `Ready` in Kubernetes but API returns errors
- API returns `500 Internal Server Error` when requesting a deployed model
- API returns `"no routing policy configured"` error
- API returns `"model not found"` despite model being deployed

## Quick Diagnosis

Use the CLI pipeline diagnostic command:

```bash
ai-aas-cli model pipeline <model-name> -e <environment>
```

This command checks all five pipeline stages and identifies the failure point.

### Example Output with Issues

```
Model Pipeline: my-model (development)

Overall Status: Model has errors and is not operational

4. Routing Policy
   ├─ Global Policy:     Missing
   │  ERROR: No global routing policy found. Model is not accessible.

Suggested Actions:
  → Create routing policy: ai-aas-cli routing policy create --model my-model --backends <backend-id>
```

## Step-by-Step Diagnosis

If the pipeline command is unavailable, follow these manual steps:

### Step 1: Verify Kubernetes Resources

Check if the AIModel exists and is Ready:

```bash
kubectl get aimodel <model-name> -n <environment>
```

Expected output:
```
NAME        PHASE   REPLICAS   AGE
my-model    Ready   1          1h
```

If not Ready, check the InferenceService:

```bash
kubectl describe aimodel <model-name> -n <environment>
kubectl get inferenceservice -n <environment>
kubectl describe inferenceservice <model-name>-isvc -n <environment>
```

### Step 2: Verify Admin API Sync

Check if the model is registered and has a deployment record:

```bash
# Check registry
ai-aas-cli model registry get <model-name>

# Check deployment
ai-aas-cli model deploy get <model-name> -e <environment>
```

Expected: Both commands return data. If registry is missing, the model wasn't synced from the operator.

### Step 3: Verify Routing Policy

This is the most common failure point.

```bash
# List all policies for the model
ai-aas-cli routing policy list --model <model-name>

# Check for global policy specifically
ai-aas-cli routing policy get --global --model <model-name>
```

Expected: At least one global policy (`organization_id: "*"`) should exist.

### Step 4: Verify API Router Health

```bash
# Check API Router is running
kubectl get pods -n api-router -l app=api-router

# Check API Router logs for errors
kubectl logs -n api-router -l app=api-router --tail=50 | grep -i error

# Test health endpoint
curl https://api.dev.otherjamesbrown.com/health
```

### Step 5: Verify Backend Endpoint

```bash
# Get InferenceService URL
kubectl get inferenceservice <model-name>-isvc -n <environment> -o jsonpath='{.status.url}'

# Test backend directly (from within cluster)
kubectl run curl-test --rm -i --tty --image=curlimages/curl -- \
  curl -s http://<inferenceservice-name>.<namespace>.svc.cluster.local/health
```

## Resolution Procedures

### Resolution A: Create Missing Routing Policy

If the routing policy is missing (most common issue):

```bash
# Identify the backend ID (InferenceService name)
kubectl get inferenceservice -n <environment> -o name | grep <model>
# Output: inferenceservice.serving.kserve.io/my-model-isvc

# Create global routing policy
ai-aas-cli routing policy create \
  --global \
  --model <model-name> \
  --backends <inferenceservice-name>:100
```

### Resolution B: Fix Backend ID Mismatch

If the routing policy exists but points to the wrong backend:

```bash
# Get current policy
ai-aas-cli routing policy get --global --model <model-name>

# Get actual InferenceService name
kubectl get inferenceservice -n <environment> -o jsonpath='{.items[*].metadata.name}'

# Delete and recreate policy with correct backend
ai-aas-cli routing policy delete --global --model <model-name>
ai-aas-cli routing policy create \
  --global \
  --model <model-name> \
  --backends <correct-backend-id>:100
```

### Resolution C: Fix External Name Mismatch

If the model is registered with a different external name:

```bash
# Check external name in registry
ai-aas-cli model registry get <model-name>

# If external_name differs from what you're requesting, either:
# 1. Use the correct name in API requests
# 2. Update the model's external_name
ai-aas-cli model registry update <model-name> --external-name <new-name>
```

### Resolution D: Restart API Router to Force Policy Reload

If policies were recently created but API Router hasn't loaded them:

```bash
# Force policy reload by restarting API Router
kubectl rollout restart deployment api-router -n api-router

# Wait for rollout
kubectl rollout status deployment api-router -n api-router

# Verify health
curl https://api.dev.otherjamesbrown.com/health
```

### Resolution E: Fix Admin API Sync

If the operator cannot sync to Admin API:

```bash
# Check operator logs
kubectl logs -n ai-model-operator-system deploy/ai-model-operator-controller-manager --tail=100 | grep -i admin

# Verify Admin API is reachable from operator
kubectl exec -n ai-model-operator-system deploy/ai-model-operator-controller-manager -- \
  curl -s http://admin-api-service.development:8080/health

# If Admin API is down, restart it
kubectl rollout restart deployment admin-api-service -n development
```

## Why Auto-Routing Might Fail

The platform has two auto-routing mechanisms that should prevent this issue:

1. **Operator auto-creation**: When InferenceService becomes Ready
2. **Admin API auto-creation**: When CreateDeployment is called

Both can fail silently (best-effort, non-blocking) if:

- Admin API is unreachable during the Ready transition
- Database connectivity issues
- External name cannot be derived from AIModel spec
- Policy already exists with different configuration (duplicate key)

## Verification

After resolution, verify the model is accessible:

```bash
# Test via API
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<model-name>",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# Run pipeline check again
ai-aas-cli model pipeline <model-name> -e <environment>
```

Expected: API returns a valid response, pipeline shows all green.

## Prevention

To prevent future occurrences:

1. **Monitor operator logs** for routing policy creation failures
2. **Set up alerts** for models with Ready status but no routing policy
3. **Verify auto-routing** is working after operator upgrades
4. **Run pipeline check** after each deployment

## Escalation

If none of the above resolves the issue:

1. Check Grafana dashboards for API Router errors
2. Query Loki for detailed error logs:
   ```bash
   curl -G https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range \
     --data-urlencode 'query={service="api-router-service",level="error"} |~ "<model-name>"' \
     --data-urlencode 'limit=50'
   ```
3. Create a bead issue with:
   - Pipeline command output
   - Relevant kubectl outputs
   - API error responses
   - Timeline of when the issue started

## Related Documentation

- [Model Deployment Pipeline](../platform/model-deployment-pipeline.md) - Full pipeline architecture
- [Routing Policies](../platform/routing-policies.md) - Policy configuration details
- [AI Debugging Workflow](./ai-debugging-workflow.md) - General debugging guide
