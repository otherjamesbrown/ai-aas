# Operator Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-15 | **Commit**: be4e1768

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
    deploymentMode: "Serverless | RawDeployment (optional) - explicit KServe deployment type"

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

## Deployment Mode Patterns

### Pattern: Explicit Deployment Mode Selection

**DO**: Use explicit `deploymentMode` field to control KServe deployment type.

```go
// Good: Explicit mode selection
deploymentMode := r.determineDeploymentMode(aimodel, recipe)
builder.WithDeploymentMode(deploymentMode)
```

**DON'T**: Infer deployment mode from nodeSelector presence.

```go
// Bad: Implicit mode selection (DEPRECATED)
if len(nodeSelector) > 0 {
    deploymentMode = "RawDeployment"  // Don't do this
}
```

### Pattern: Runtime-Aware Defaults

When deployment mode is not explicitly set:

| Runtime | GPU | Default Mode |
|---------|-----|--------------|
| tensorrt-llm | - | RawDeployment |
| triton | - | RawDeployment |
| vllm | Yes | RawDeployment |
| vllm | No | Serverless |
| tgi | Yes | RawDeployment |
| tgi | No | Serverless |
| Other | - | Serverless |

### Anti-Pattern: Implicit NodeSelector Logic

The old pattern of checking `len(nodeSelector) > 0` to determine deployment mode is deprecated. This was:
- Implicit and surprising behavior
- Undocumented
- Inconsistent across code paths

Always use the `determineDeploymentMode()` function which follows the priority:
1. AIModel.Spec.DeploymentMode (explicit override)
2. Recipe.Spec.DeploymentMode (recipe default)
3. Runtime-based defaults (GPU-aware)

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

### Knative Serving Probe Configuration

**CRITICAL**: Knative Serving in Serverless mode does not support `startupProbe`. Even if you define it in the InferenceService spec, Knative will remove it from the actual pod deployment.

```go
// WRONG: Relying on startupProbe in Serverless mode
container.StartupProbe = &corev1.Probe{
    InitialDelaySeconds: 30,
    FailureThreshold: 90,
    // This probe will be REMOVED by Knative!
}

// WRONG: Liveness probe with no initialDelaySeconds for slow-starting containers
container.LivenessProbe = &corev1.Probe{
    HTTPGet: &corev1.HTTPGetAction{
        Path: "/health",
        Port: intstr.FromInt(8000),
    },
    PeriodSeconds:    30,
    FailureThreshold: 3,
    // Missing: InitialDelaySeconds
    // Container will be killed after 90s if startup takes longer!
}

// CORRECT: Set initialDelaySeconds on livenessProbe for long-loading containers
container.LivenessProbe = &corev1.Probe{
    HTTPGet: &corev1.HTTPGetAction{
        Path: "/health",
        Port: intstr.FromInt(8000),
    },
    InitialDelaySeconds: 300,  // 5 minutes for vLLM model loading
    PeriodSeconds:       30,
    FailureThreshold:    3,
    TimeoutSeconds:      5,
}
```

**Why this matters**:
- vLLM model loading can take 90-150 seconds (download + load + compile)
- Without `initialDelaySeconds`, liveness probe starts immediately
- After 3 failures (3 × 30s = 90s), kubelet kills container with SIGKILL
- Container restarts in a loop, never completing model initialization
- Readiness probe still prevents traffic until model is ready

**Recommended values**:
- Small/medium models (≤20B): 300 seconds (5 minutes)
- Large models (>20B): Consider making configurable via CRD


### KServe Admission Webhook Probe Override

**CRITICAL**: KServe's admission webhook modifies InferenceService specs and may override or remove probe configuration set in container specs. This is distinct from Knative Serving's behavior.

```go
// WRONG: Setting probes directly in container spec
container := map[string]interface{}{
    "livenessProbe": map[string]interface{}{
        "httpGet": map[string]interface{}{
            "path": "/health",
            "port": 8000,
        },
        "initialDelaySeconds": 300,
        "periodSeconds":       30,
        "failureThreshold":    3,
    },
}
// KServe admission webhook may override or remove this configuration!

// WRONG: Assuming operator-set probes will be preserved
func BuildContainerBased() {
    container.StartupProbe = &corev1.Probe{...}
    container.LivenessProbe = &corev1.Probe{...}
    // These probes may be REMOVED by KServe webhook
}

// CORRECT: Use KServe/Knative annotations to influence probe behavior
annotations := map[string]string{
    "serving.knative.dev/initialDelaySeconds": "300",
    "serving.knative.dev/timeoutSeconds": "5",
    // Or configure at ClusterServingRuntime level
}
```

**Why this happens**:
- KServe admission webhook processes InferenceService resources
- Webhook may apply defaults, override container specs, or remove incompatible probes
- Knative Serving (used by KServe in serverless mode) removes `startupProbe` entirely
- Probes set in operator code may not match deployed pod configuration

**Evidence from bug ai-aas-9s4z**:
- Operator set: `livenessProbe.initialDelaySeconds = 300`
- Deployed pod had: `livenessProbe` with NO `initialDelaySeconds`
- Result: vLLM containers killed after 90s during model loading

**Solutions**:
1. **Use ClusterServingRuntime probe configuration** (preferred for KServe)
2. **Use Knative annotations** for probe timing (if supported)
3. **Configure at InferenceService.spec level**, not container level
4. **Test deployed pods** - always verify actual probe config with `kubectl get pod -o yaml`
5. **Monitor for restarts** - liveness probe failures appear in events

**Verification after deployment**:
```bash
# Check actual probe configuration
kubectl get pod <predictor-pod> -n <namespace> -o yaml | grep -A 10 livenessProbe

# Check events for probe failures
kubectl get events -n <namespace> | grep "failed liveness probe"
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
| Behavioral contract | `docs/operators/operator-behavioral-contract.md` |
| Code patterns | `docs/operators/operator-patterns.md` |

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
