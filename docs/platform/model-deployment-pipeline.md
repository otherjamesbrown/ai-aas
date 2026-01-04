# Model Deployment Pipeline

This document describes the complete model deployment to inference pipeline for the AI-AAS platform.

## Overview

The model deployment pipeline consists of five stages, each managed by a different component:

```
+-------------------+     +-------------------+     +-------------------+
|   1. GitOps       |---->|   2. Kubernetes   |---->|   3. Admin API    |
|  (ai-aas-config)  |     | (ai-model-operator)|    |  (model registry) |
+-------------------+     +-------------------+     +-------------------+
                                                            |
                                                            v
                          +-------------------+     +-------------------+
                          |  5. Inference     |<----|   4. Routing      |
                          |   (API Router)    |     |    (Policies)     |
                          +-------------------+     +-------------------+
```

A model must successfully pass through all five stages to be accessible via the API.

## Stage 1: GitOps Configuration (ai-aas-config)

**Component:** ai-aas-config repository
**Resource:** AIModel Custom Resource Definition (CRD)

The pipeline begins with an AIModel manifest in the ai-aas-config GitOps repository.

### Location

```
ai-aas-config/
  environments/
    development/
      models/
        gpt-oss-20b.yaml     # AIModel for development
    staging/
      models/
        gpt-oss-20b.yaml     # AIModel for staging
    production/
      models/
        gpt-oss-20b.yaml     # AIModel for production
```

### AIModel Manifest Example

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: gpt-oss-20b
  namespace: development
spec:
  modelID: unsloth/gpt-oss-20b                 # HuggingFace model ID
  externalName: gpt-oss-20b                    # Name exposed in API
  recipe: vllm-default                         # Serving recipe
  resources:
    gpu:
      count: 1
      vendor: nvidia
```

### Branch Targeting

| Branch | ArgoCD Target | Environment |
|--------|---------------|-------------|
| `develop` | development | Fast iteration |
| `staging` | staging | Pre-prod validation |
| `main` | production | Production-ready |

### What Can Go Wrong

- **Manifest not committed**: Model won't be deployed
- **Wrong branch**: Model deployed to wrong environment
- **Invalid YAML**: ArgoCD sync will fail
- **ArgoCD sync disabled**: Changes won't propagate

### Verification

```bash
# Check if manifest exists in ai-aas-config
ls ~/ai-aas-config/environments/<env>/models/<model>.yaml

# Check ArgoCD sync status
argocd app get aimodels-<env> --core
```

## Stage 2: Kubernetes Operator (ai-model-operator)

**Component:** ai-model-operator
**Resources:** AIModel CRD, InferenceService (KServe)

The operator watches for AIModel resources and creates the underlying InferenceService.

### Reconciliation Flow

1. Operator detects new/updated AIModel
2. Creates InferenceService from AIModel spec
3. Waits for InferenceService to become Ready
4. Syncs deployment status to Admin API
5. **Auto-creates routing policy when Ready** (new in Phase 1)

### InferenceService Lifecycle

```
Pending -> Creating -> Running -> Ready
                    \-> Failed
```

### Auto-Routing Policy Creation

When an InferenceService transitions to Ready, the operator automatically:

1. Derives the external name from the AIModel spec
2. Checks if a routing policy already exists
3. Creates a global routing policy (`organization_id: "*"`)
4. Sets 100% traffic weight to the InferenceService backend

This happens in the `ensureRoutingPolicy` method and is **best-effort** - failures are logged but don't block reconciliation.

### What Can Go Wrong

- **No GPU resources**: Pod stuck in Pending
- **Model too large**: OOM during loading
- **HuggingFace auth**: Gated model without credentials
- **Image pull errors**: Container registry issues
- **Operator not running**: No reconciliation

### Verification

```bash
# Check AIModel status
kubectl get aimodel <model> -n <env> -o yaml

# Check InferenceService
kubectl get inferenceservice -n <env>

# Check pods
kubectl get pods -n <env> -l serving.kserve.io/inferenceservice=<model>
```

## Stage 3: Model Registry & Deployment (Admin API)

**Component:** admin-api-service
**Tables:** `model_registry`, `model_deployments`

The Admin API tracks model metadata and deployment records.

### Auto-Registration

When the operator syncs a deployment to Admin API, the service automatically:

1. Registers the model in `model_registry` if not exists
2. Creates/updates deployment record in `model_deployments`
3. **Auto-creates routing policy if first deployment** (new in Phase 1)

The Admin API's `CreateDeployment` method includes:

```go
// Auto-create routing policy if this is the first deployment for this model
if !s.policyExists(ctx, req.ModelName) {
    if err := s.createDefaultRoutingPolicy(ctx, req.ModelName); err != nil {
        // Log warning but don't fail the deployment
        s.logger.Warn("failed to auto-create routing policy", ...)
    }
}
```

### Deployment Status Values

| Status | Description |
|--------|-------------|
| `pending` | Deployment created, not yet started |
| `deploying` | InferenceService being created |
| `ready` | InferenceService is Ready, model accessible |
| `failed` | Deployment failed |
| `disabled` | Manually disabled |
| `terminated` | Deployment deleted |

### What Can Go Wrong

- **Admin API unreachable**: Operator can't sync status
- **Database connectivity**: Deployment records not persisted
- **Policy creation fails**: Model accessible but no routing

### Verification

```bash
# Check model in registry
ai-aas-cli model registry get <model>

# Check deployment status
ai-aas-cli model deploy list -e <env>

# Check deployment details
ai-aas-cli model deploy get <model> -e <env>
```

## Stage 4: Routing Configuration (Routing Policies)

**Component:** admin-api-service (policies) + api-router-service (consumption)
**Storage:** PostgreSQL `routing_policies` table, replicated to etcd

Routing policies control how API requests are mapped to backends.

### Policy Resolution Chain

The API Router uses a three-level fallback chain:

1. **Organization Policy**: Check for org-specific policy (`organization_id: <org-uuid>`)
2. **Global Policy**: Check for global policy (`organization_id: "*"`)
3. **Registry Discovery**: Query Admin API for deployed models (ephemeral policy)

This is implemented in `GetPolicyWithFallback`:

```go
func (l *Loader) GetPolicyWithFallback(ctx context.Context, organizationID, model string, adminAPIClient AdminAPIClient) (*RoutingPolicy, string, error) {
    // 1. Try explicit org policy first
    policy, err := l.getExactPolicy(organizationID, model)
    if err == nil && policy != nil {
        return policy, "org_policy", nil
    }

    // 2. Try global policy
    policy, err = l.getExactPolicy(etcdGlobalOrgID, model)
    if err == nil && policy != nil {
        return policy, "global_policy", nil
    }

    // 3. Try registry discovery
    if adminAPIClient != nil {
        modelInfo, err := adminAPIClient.GetModelDeployment(ctx, model)
        if err == nil && modelInfo != nil && *modelInfo.DeploymentStatus == "ready" {
            // Create ephemeral policy
            return ephemeralPolicy, "registry_discovery", nil
        }
    }

    return nil, "", fmt.Errorf("model %q is not deployed or not accessible", model)
}
```

### Auto-Created Policy Structure

When a routing policy is auto-created (by operator or Admin API), it has:

```json
{
  "policy_id": "<uuid>",
  "organization_id": "*",
  "model": "<model-name>",
  "backends": [
    {
      "backend_id": "<inferenceservice-name>",
      "weight": 100
    }
  ],
  "failover_threshold": 3,
  "enabled": true,
  "backend_type": "openai",
  "metadata": {
    "auto_created": true,
    "created_reason": "auto-created on deployment"
  }
}
```

### What Can Go Wrong

- **Policy not created**: Both auto-creation paths failed
- **Wrong backend ID**: Policy points to non-existent service
- **Policy disabled**: `enabled: false`
- **etcd sync lag**: Policy created but not yet visible to API Router

### Verification

```bash
# List routing policies for a model
ai-aas-cli routing policy list --model <model>

# Check if global policy exists
ai-aas-cli routing policy get --global --model <model>
```

## Stage 5: Inference Serving (API Router -> vLLM)

**Component:** api-router-service -> vLLM/InferenceService
**Protocol:** OpenAI-compatible REST API

The API Router receives requests and forwards them to the appropriate backend.

### Request Flow

1. Client sends request to API Router
2. Router extracts model name from request
3. Router resolves routing policy (org -> global -> registry)
4. Router selects backend based on weights
5. Router forwards request to InferenceService endpoint
6. vLLM processes inference
7. Response returned to client

### Health Checking

The API Router checks backend health via `/health` endpoint. Backends that fail health checks are temporarily marked degraded.

### What Can Go Wrong

- **Backend not responding**: vLLM crashed or not ready
- **Network issues**: Cannot reach InferenceService
- **GPU memory exhausted**: OOM during inference
- **Request timeout**: Model too slow
- **Incompatible request**: Wrong input format for model

### Verification

```bash
# Check API Router health
curl https://api.dev.otherjamesbrown.com/health

# Test inference endpoint
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<model-name>",
    "messages": [{"role": "user", "content": "Hello"}]
  }'

# Check vLLM pod logs
kubectl logs -n <env> -l serving.kserve.io/inferenceservice=<model> --tail=50
```

## Pipeline Diagnostic Command

Use the CLI pipeline command to diagnose issues across all stages:

```bash
ai-aas-cli model pipeline <model-name> -e <environment>
```

### Example Output

```
Model Pipeline: gpt-oss-20b (development)

Overall Status: ✓ Model is fully operational

1. GitOps (ai-aas-config)
   ├─ AIModel CR:        ✓ Found at environments/development/models/gpt-oss-20b.yaml
   ├─ Target Revision:   ✓ develop
   └─ Last Sync:         ✓ Unknown (ArgoCD API not integrated)

2. Kubernetes (ai-model-operator)
   ├─ AIModel:           ✓ Ready
   ├─ InferenceService:  ✓ gpt-oss-20b-isvc (Ready)
   ├─ Pods:              ✓ 1 Running
   └─ GPU Allocation:    ✓ Check pod spec

3. Admin API (model registry)
   ├─ Registry:          ✓ Registered (unsloth/gpt-oss-20b)
   ├─ Deployment:        ✓ ready (id: abc123)
   └─ Backend:           ✓ gpt-oss-20b-vllm-deployment

4. Routing Policy
   ├─ Global Policy:     ✓ Exists
   ├─ Org Policies:      ✓ 0 custom policies
   └─ Active Backend:    ✓ Configured (see policy details)

5. Inference Endpoint
   ├─ URL:               ✓ https://api.dev.otherjamesbrown.com/v1/chat/completions
   ├─ Health:            ✓ Responding (42ms)
   └─ Last Request:      ✓ Unknown

Summary: Model is fully operational
```

### Example with Errors

```
Model Pipeline: broken-model (development)

Overall Status: Model has errors and is not operational

1. GitOps (ai-aas-config)
   ├─ AIModel CR:        ✓ Found at environments/development/models/broken-model.yaml

2. Kubernetes (ai-model-operator)
   ├─ AIModel:           ⚠ Creating
   ├─ InferenceService:  ⚠ Not created yet
   └─ Pods:              ✓ 0 Running

3. Admin API (model registry)
   ├─ Registry:          ✗ Not registered

4. Routing Policy
   ├─ Global Policy:     ✗ Not configured
   │  ERROR: No global routing policy found. Model is not accessible.

5. Inference Endpoint
   ├─ Health:            ✗ Not responding
   │  ERROR: Health check failed: connection refused

Summary: Model has errors and is not operational

Suggested Actions:
  → Register model: ai-aas-cli model registry add <hf-model-id> --name broken-model
  → Create routing policy: ai-aas-cli routing policy create --model broken-model --backends <backend-id>
```

## Common Failure Scenarios

### Model Shows Ready But Not Accessible

**Symptom:** `kubectl get aimodel` shows Ready, but API returns 500.

**Likely Causes:**
1. No routing policy (both auto-creation paths failed)
2. Routing policy points to wrong backend ID
3. etcd sync lag

**Resolution:**
```bash
# Use pipeline command to diagnose
ai-aas-cli model pipeline <model> -e <env>

# If routing policy missing, create manually
ai-aas-cli routing policy create --global --model <model> --backends <backend-id>:100
```

### Routing Policy Exists But Model Not Found

**Symptom:** Policy exists, but API returns "model not found".

**Likely Causes:**
1. External name mismatch (policy uses different name than request)
2. Model name in policy doesn't match request

**Resolution:**
```bash
# Check policy details
ai-aas-cli routing policy get --global --model <model>

# Verify external_name in model registry
ai-aas-cli model registry get <model>
```

### Auto-Routing Not Working

**Symptom:** Deployed models don't get routing policies automatically.

**Likely Causes:**
1. Admin API client not configured in operator
2. Admin API unreachable from operator
3. External name not derivable from AIModel spec

**Resolution:**
```bash
# Check operator logs for routing policy errors
kubectl logs -n ai-model-operator-system deploy/ai-model-operator-controller-manager | grep -i "routing"

# Verify Admin API connectivity from operator
kubectl exec -n ai-model-operator-system deploy/ai-model-operator-controller-manager -- curl -s http://admin-api-service.development:8080/health
```

## Related Documentation

- [Routing Policies](./routing-policies.md) - Detailed policy configuration
- [vLLM Deployment Best Practices](./vllm-deployment-best-practices.md) - vLLM configuration
- [KServe Management](./kserve-management.md) - InferenceService details
- [Model Not Accessible Runbook](../runbooks/model-not-accessible.md) - Step-by-step troubleshooting
