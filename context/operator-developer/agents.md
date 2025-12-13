# Operator Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-13 | **Commit**: 24c3e0ee

---

## Domain

You own: `operators/ai-model-operator/`

Hand off to:
- API endpoints → `go-services-developer`
- Helm deployment → `infra-ops-manager`
- CI/CD → `infra-ops-manager`
- CLI commands → `cli-developer`

---

## Key Patterns

```yaml
patterns:
  reconciliation_flow:
    steps:
      1: "AIModel CR created/updated"
      2: "Check spec.enabled (false → delete InferenceService, Disabled)"
      3: "Check model source (trustRemoteCode → skip download)"
      4: "Create/Update InferenceService (KServe)"
      5: "Monitor status (Ready/RetryPending/Failed)"
    rule: MUST be idempotent - check before create

  crd_changes:
    rule: Always regenerate after modifying aimodel_types.go
    commands:
      - "make generate   # Regenerate deepcopy"
      - "make manifests  # Regenerate CRD YAML"

  status_updates:
    rule: Status subresource requires separate call
    method: "r.Status().Update(ctx, aiModel)"
    why: "r.Update() ignores status subresource"

  owner_references:
    rule: Set on all child resources
    why: Enables garbage collection on parent deletion
    method: "ctrl.SetControllerReference(aiModel, child, r.Scheme)"

  long_operations:
    rule: Never block reconciliation queue
    do: Create Job, requeue to check status
    pattern: "return ctrl.Result{RequeueAfter: 30 * time.Second}, nil"

  aimodel_phases:
    - Pending: Initial state
    - Downloading: Downloader job running
    - Deploying: InferenceService created
    - Ready: Model serving
    - Failed: Permanent error
    - Disabled: spec.enabled=false
    - RetryPending: Transient failure, will retry

aimodel_crd_spec:
  required:
    modelName: "Human-readable name"
    modelID: "HuggingFace model ID (e.g., mistralai/Mistral-7B-Instruct-v0.3)"

  runtime:
    runtime: "vllm | triton | tgi (default: vllm)"
    runtimeName: "Custom ClusterServingRuntime (optional)"
    runtimeArgs: "[]string - CLI args (e.g., --dtype=float16, --max-model-len=4096)"
    runtimeEnv: "[]EnvVar - additional env vars"
    trustRemoteCode: "bool - load from HuggingFace directly (skips S3)"

  storage:  # Optional if trustRemoteCode=true
    s3Bucket: "Bucket for model artifacts"
    s3Key: "Path to model in bucket"

  scaling:
    enabled: "bool (default: true) - false scales to zero"
    minReplicas: "int32 (default: 0) - scale-to-zero capable"
    maxReplicas: "int32 (default: 1)"

  scheduling:
    resources: "requests/limits for cpu, memory, nvidia.com/gpu"
    nodeSelector: "map[string]string - GPU type targeting"
    tolerations: "[]Toleration - GPU workload tolerations"

aimodel_crd_status:
  phase: "Pending|Downloading|Deploying|Ready|Failed|Disabled|RetryPending"
  inferenceServiceName: "Name of KServe InferenceService"
  inferenceEndpoint: "URL for inference (http://name.ns.svc.cluster.local)"
  readyReplicas: "int32"
  message: "Human-readable status message"
  retryCount: "Download retry attempts"
```

---

## Anti-patterns

```go
// WRONG: Block reconciliation with long operation
downloadModel(ctx)  // Takes 30 minutes

// WRONG: Ignore errors
r.Create(ctx, obj)

// WRONG: Update status with regular Update()
aiModel.Status.Phase = "Ready"
r.Update(ctx, aiModel)  // Status won't update!

// WRONG: Hardcode images
image := "vllm/vllm-openai:v0.10.2"

// WRONG: No owner reference (child resources orphaned on delete)
deployment := &appsv1.Deployment{...}
r.Create(ctx, deployment)  // Missing ctrl.SetControllerReference()

// WRONG: Forgot make generate after CRD changes
// Types won't have DeepCopy methods, build fails

// WRONG: Create without checking existence first
r.Create(ctx, inferenceService)  // Will error if exists!
// Should: Get() first, then Create() or Update()
```

---

## Commands

```bash
# CRD management
make generate              # After modifying aimodel_types.go
make manifests             # Regenerate CRD YAML
make install               # Install CRDs to cluster

# Test
cd operators/ai-model-operator && go test ./...
go test ./controllers/... -v -run TestReconcile

# Local development
make run                   # Run operator locally
make docker-build docker-push IMG=ghcr.io/ai-aas/ai-model-operator:dev
```

---

## Sources

| What | Where |
|------|-------|
| CRD types | `operators/ai-model-operator/api/v1alpha1/aimodel_types.go` |
| Controller | `operators/ai-model-operator/controllers/aimodel_controller.go` |
| KServe builder | `operators/ai-model-operator/internal/kserve/inferenceservice.go` |
| Status extraction | `operators/ai-model-operator/internal/kserve/status.go` |
| Admin API client | `operators/ai-model-operator/internal/adminapi/client.go` |
| AIModel examples | `infra/k8s/aimodels/development/` |
| CRD reference | `docs/operators/ai-model-operator.md` |

---

## Checklist

Before completing work:
- [ ] `make generate` after CRD changes
- [ ] `make manifests` after CRD changes
- [ ] `go test ./...` passes
- [ ] Reconciliation is idempotent
- [ ] Status uses Status().Update()
- [ ] Owner references on child resources
- [ ] No blocking operations in reconciler
