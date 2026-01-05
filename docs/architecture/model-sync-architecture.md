# Model Sync Architecture

This document describes how model definitions are synchronized between Kubernetes AIModel CRDs and the Admin API database.

## Overview

The AI-AAS platform uses a Kubernetes operator pattern to manage model deployments. The ai-model-operator watches AIModel Custom Resources and synchronizes their state to the Admin API, which maintains the model registry and deployment records in PostgreSQL.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Kubernetes Cluster                                 │
│  ┌──────────────┐     ┌────────────────────┐     ┌────────────────────────┐ │
│  │   AIModel    │     │  ai-model-operator │     │   InferenceService     │ │
│  │     CRD      │────▶│   (Reconciler)     │────▶│      (KServe)          │ │
│  └──────────────┘     └─────────┬──────────┘     └────────────────────────┘ │
│                                 │                                            │
└─────────────────────────────────┼────────────────────────────────────────────┘
                                  │ HTTP
                                  ▼
                     ┌────────────────────────┐
                     │      Admin API         │
                     │  ┌──────────────────┐  │
                     │  │  Model Service   │  │
                     │  └────────┬─────────┘  │
                     │           │            │
                     │  ┌────────▼─────────┐  │
                     │  │    PostgreSQL    │  │
                     │  │  ┌────────────┐  │  │
                     │  │  │  registry  │  │  │
                     │  │  ├────────────┤  │  │
                     │  │  │deployments │  │  │
                     │  │  ├────────────┤  │  │
                     │  │  │  policies  │  │  │
                     │  │  └────────────┘  │  │
                     │  └──────────────────┘  │
                     └────────────────────────┘
```

## Components

### AIModel CRD

The AIModel Custom Resource Definition is the source of truth for model deployments. It defines:

| Field | Description |
|-------|-------------|
| `spec.modelName` | DNS-compatible name for the model |
| `spec.modelID` | Full HuggingFace model path (e.g., `unsloth/gpt-oss-20b`) |
| `spec.externalName` | Name exposed in OpenAI-compatible APIs (optional) |
| `spec.modelType` | Type: text, vision-language, embedding, audio |
| `spec.runtime` | Inference runtime: vllm, triton, tensorrt-llm, tgi |
| `spec.enabled` | Whether the deployment should be active |
| `spec.minReplicas` / `spec.maxReplicas` | Autoscaling configuration |

**Location:** AIModel manifests are stored in the ai-aas-config GitOps repository and applied via ArgoCD.

**Code:** `/home/dev/worktrees/develop/operators/ai-model-operator/api/v1alpha1/aimodel_types.go`

### ai-model-operator

The operator is a controller-runtime based Kubernetes controller that:

1. Watches AIModel resources
2. Creates/updates KServe InferenceServices
3. Syncs deployment status to Admin API
4. Creates routing policies when deployments become Ready

**Key Components:**

| Component | File | Purpose |
|-----------|------|---------|
| Controller | `controllers/aimodel_controller.go` | Main reconciliation loop |
| Admin API Client | `internal/adminapi/client.go` | HTTP client for Admin API |
| KServe Integration | `internal/kserve/inferenceservice.go` | InferenceService builder |

**Configuration:**

The operator requires two environment variables for Admin API sync:

```bash
ADMIN_API_BASE_URL=http://admin-api-service.development:8080
ADMIN_API_KEY=<admin-api-key>
```

If these are not set, Admin API sync is disabled and the operator only manages Kubernetes resources.

### Admin API

The Admin API provides REST endpoints for model management and maintains persistent state in PostgreSQL.

**Key Endpoints:**

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/v1/deployments` | Create deployment record |
| PUT | `/v1/deployments/{name}/{env}` | Update deployment status |
| DELETE | `/v1/deployments/{name}/{env}` | Remove deployment record |
| GET | `/v1/routing/policies` | List routing policies |
| POST | `/v1/routing/policies` | Create routing policy |

**Code:** `/home/dev/worktrees/develop/services/admin-api-service/internal/services/models/deployment.go`

## Sync Flow

### 1. Deployment Creation

When a new AIModel is created:

```
1. AIModel applied to Kubernetes
2. Operator detects new resource (Watch)
3. Operator validates spec and resolves recipe
4. Operator creates InferenceService
5. Operator waits for InferenceService to start
6. Operator calls Admin API CreateDeployment
7. Admin API:
   a. Checks if model exists in registry
   b. Auto-registers if not found
   c. Creates deployment record
   d. Auto-creates routing policy if first deployment
```

### 2. Status Updates

The operator continuously reconciles and updates status:

```
1. Operator detects InferenceService status change
2. Operator maps AIModel phase to deployment status:
   - AIModelPhaseReady → "ready"
   - AIModelPhaseDeploying → "deploying"
   - AIModelPhaseFailed → "failed"
   - AIModelPhaseDisabled → "disabled"
3. Operator calls Admin API UpdateDeploymentStatus
4. Admin API updates model_deployments table
```

### 3. Periodic Sync

Ready models are periodically re-synced to handle:
- Admin API restarts/database resets
- Network partitions
- State drift

```go
// From aimodel_controller.go
const syncInterval = 5 * time.Minute

func (r *AIModelReconciler) shouldPeriodicSync(aiModel *aimodelv1alpha1.AIModel) bool {
    if aiModel.Status.LastAdminAPISyncTime == nil {
        return true
    }
    timeSinceLastSync := time.Since(aiModel.Status.LastAdminAPISyncTime.Time)
    return timeSinceLastSync > syncInterval
}
```

### 4. Deletion

When an AIModel is deleted:

```
1. Kubernetes sends delete event
2. Operator handles finalizer
3. Operator deletes InferenceService
4. Operator calls Admin API DeleteDeployment
5. Admin API marks deployment as "terminated"
```

## Auto-Registration

The Admin API supports auto-registration of models. When `CreateDeployment` is called for a model that doesn't exist in the registry:

```go
// From deployment.go
if errors.Is(err, ErrModelNotFound) {
    // Auto-register the model with minimal metadata
    autoRegReq := AddModelRequest{
        Name:         req.ModelName,
        HFModelID:    req.ModelID,      // e.g., "unsloth/gpt-oss-20b"
        ExternalName: req.ExternalName, // e.g., "gpt-oss-20b"
        ModelType:    req.ModelType,
    }
    model, err = s.AddModel(ctx, autoRegReq)
}
```

This allows models to be deployed via GitOps without manual registry steps.

## Routing Policy Creation

Routing policies are auto-created through two paths:

### Path 1: Operator (on Ready)

When an InferenceService transitions to Ready:

```go
// From aimodel_controller.go
func (r *AIModelReconciler) ensureRoutingPolicy(ctx context.Context, aiModel *aimodelv1alpha1.AIModel) error {
    externalName := deriveExternalName(aiModel)

    // Check if policy exists
    existingPolicies, _ := r.AdminAPIClient.ListRoutingPolicies(ctx, externalName, "*")
    if existingPolicies != nil && len(existingPolicies.Policies) > 0 {
        return nil // Policy exists
    }

    // Create global policy
    policy := adminapi.RoutingPolicyCreate{
        Model:          externalName,
        OrganizationID: "*",
        Backends:       []adminapi.Backend{{BackendID: inferenceServiceName, Weight: 100}},
        BackendType:    determineBackendType(aiModel),
    }
    _, err := r.AdminAPIClient.CreateRoutingPolicy(ctx, policy)
    return err
}
```

### Path 2: Admin API (on CreateDeployment)

When a deployment is created:

```go
// From deployment.go
if !s.policyExists(ctx, req.ModelName) {
    if err := s.createDefaultRoutingPolicy(ctx, req.ModelName); err != nil {
        s.logger.Warn("failed to auto-create routing policy", ...)
    }
}
```

Both paths are idempotent - if a policy already exists, no action is taken.

## ExternalName Derivation

The external name determines how the model appears in the OpenAI-compatible API:

```go
// From aimodel_controller.go
func deriveExternalName(aiModel *aimodelv1alpha1.AIModel) string {
    if aiModel.Spec.ExternalName != "" {
        return aiModel.Spec.ExternalName
    }
    // Derive from ModelID: "unsloth/gpt-oss-20b" -> "gpt-oss-20b"
    modelID := aiModel.Spec.ModelID
    if idx := strings.LastIndex(modelID, "/"); idx >= 0 {
        return modelID[idx+1:]
    }
    return modelID
}
```

## Error Handling

### Best-Effort Sync

Admin API sync is best-effort - failures don't block Kubernetes reconciliation:

```go
if err := r.syncDeploymentToAdminAPI(ctx, aiModel, status); err != nil {
    log.Error(err, "Failed to sync deployment to Admin API")
    // Don't return error - continue reconciliation
}
```

### Retry Logic

The Admin API client includes retry logic for rate limiting (429 responses):

```go
const (
    maxRetries     = 5
    initialBackoff = 1 * time.Second
    maxBackoff     = 30 * time.Second
)
```

### Not Found Handling

If `UpdateDeploymentStatus` returns 404, the operator creates the deployment:

```go
if err != nil && contains(err.Error(), "404") {
    createErr := r.AdminAPIClient.CreateDeployment(ctx, createReq)
}
```

## Database Schema

The Admin API stores model and deployment data in these tables:

### model_registry

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| name | VARCHAR | Model name (e.g., "gpt-oss-20b") |
| hf_model_id | VARCHAR | HuggingFace model ID |
| external_name | VARCHAR | Name in OpenAI API |
| model_type | VARCHAR | text, vision-language, etc. |

### model_deployments

| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Primary key |
| model_id | UUID | Foreign key to model_registry |
| environment | VARCHAR | development, staging, production |
| namespace | VARCHAR | Kubernetes namespace |
| status | VARCHAR | pending, deploying, ready, failed |
| inferenceservice_name | VARCHAR | KServe InferenceService name |
| endpoint | VARCHAR | Inference endpoint URL |
| replicas_ready | INT | Ready replica count |

### routing_policies

| Column | Type | Description |
|--------|------|-------------|
| policy_id | UUID | Primary key |
| organization_id | VARCHAR | "*" for global, UUID for org-specific |
| model | VARCHAR | Model name for routing |
| backends | JSONB | Backend configuration |
| backend_type | VARCHAR | openai, triton, triton-grpc |

## Debugging

### Check Operator Logs

```bash
kubectl logs -n ai-model-operator-system deploy/ai-model-operator-controller-manager | grep -i "admin"
```

### Check Sync Status

The AIModel status includes the last sync time:

```bash
kubectl get aimodel <name> -o jsonpath='{.status.lastAdminAPISyncTime}'
```

### Verify Admin API Connection

```bash
# From operator pod
kubectl exec -n ai-model-operator-system deploy/ai-model-operator-controller-manager -- \
  curl -s http://admin-api-service.development:8080/health
```

### Check Deployment Record

```bash
ai-aas-cli model deploy get <name> -e development
```

### Check Routing Policy

```bash
ai-aas-cli routing policy list --model <external-name>
```

## Related Documentation

- [Model Deployment Pipeline](../platform/model-deployment-pipeline.md) - End-to-end deployment flow
- [Inference Routing](./inference-routing.md) - How requests are routed to backends
- [Routing Policies](./routing-policies.md) - Policy configuration details
