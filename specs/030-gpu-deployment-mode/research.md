# Research: GPU Deployment Mode Migration

**Date**: 2025-12-17
**Spec**: [spec.md](./spec.md)

## Open Questions from Spec/Impact Analysis

### Q1: Should HPA be created automatically?

**Context**: RawDeployment mode loses Knative's built-in autoscaling. The spec proposes optional HPA support.

**Options**:
1. **Automatic HPA** when `minReplicas != maxReplicas` - Implicitly create HPA
2. **Explicit opt-in** via `autoscaling` config in recipe - User must enable
3. **No HPA initially** - Defer to future work

**Decision**: **Option 2 - Explicit opt-in**

**Rationale**:
- GPU workloads are expensive; accidental autoscaling could be costly
- Different models have different scaling characteristics
- Users should consciously choose scaling behavior
- Aligns with "explicit over implicit" principle from this migration

**Implementation**:
```yaml
# ModelRecipe with optional autoscaling
spec:
  autoscaling:
    enabled: true          # Must explicitly enable
    minReplicas: 1
    maxReplicas: 3
    targetCPUUtilization: 70
```

---

### Q2: Should we update existing recipes with explicit deploymentMode?

**Context**: After migration, GPU recipes will infer `RawDeployment` automatically. Should we update them to be explicit?

**Options**:
1. **Update all recipes** - Add explicit `deploymentMode: RawDeployment`
2. **Leave as-is** - Rely on runtime inference
3. **Update selectively** - Only recipes with non-obvious behavior

**Decision**: **Option 1 - Update all recipes**

**Rationale**:
- Explicit is better than implicit (entire point of this migration)
- Makes intent clear in git history
- Easier debugging when mode is visible in CR
- No functional impact (just documentation)

**Implementation**:
- Phase 3 task: Update all GPU recipes in ai-aas-config repo
- Add `deploymentMode: RawDeployment` to each

---

### Q3: Should Knative Serverless be blocked for GPU workloads?

**Context**: Users could set `deploymentMode: Serverless` on GPU workloads, which may fail.

**Options**:
1. **Block via validation** - Webhook rejects Serverless + GPU
2. **Warn but allow** - Log warning, let user proceed
3. **Silent allow** - User's choice, no warning

**Decision**: **Option 2 - Warn but allow**

**Rationale**:
- Some users may have specific Knative configurations that work
- Blocking prevents experimentation
- Warning provides awareness without forcing behavior
- Can tighten later based on feedback

**Implementation**:
```go
if deploymentMode == "Serverless" && gpuCount > 0 {
    log.Warn("Serverless mode with GPU workloads may have issues",
        "recommendation", "Consider RawDeployment for GPU workloads")
}
```

---

### Q4: Custom metrics for GPU HPA?

**Context**: CPU-based HPA may not reflect GPU utilization accurately.

**Options**:
1. **GPU utilization metrics** - Use DCGM exporter + custom metrics adapter
2. **Request queue depth** - Scale based on pending requests
3. **CPU-only initially** - Simpler, defer GPU metrics to future

**Decision**: **Option 3 - CPU-only initially**

**Rationale**:
- GPU metrics require additional infrastructure (DCGM, Prometheus adapter)
- CPU utilization is a reasonable proxy for inference load
- Can add GPU metrics as future enhancement
- Keeps scope manageable

**Future Work**: Add GPU utilization-based autoscaling as separate spec

---

### Q5: Traffic splitting without Knative?

**Context**: Knative provides traffic splitting for canary releases. RawDeployment doesn't.

**Options**:
1. **Istio VirtualService** - Use existing Istio for traffic splitting
2. **Multiple InferenceServices** - Deploy v1 and v2, split at ingress
3. **No traffic splitting** - Full cutover only

**Decision**: **Option 3 - No traffic splitting (initially)**

**Rationale**:
- Traffic splitting adds complexity
- GPU workloads typically don't need gradual rollout (model either works or doesn't)
- Istio VirtualService can be added later if needed
- Full cutover with rollback is sufficient for MVP

**Future Work**: Add traffic splitting support as separate spec if needed

---

### Q6: Warm pool for faster scaling?

**Context**: GPU models take 5-10 minutes to load. Warm standby replicas could reduce scale-up time.

**Decision**: **Out of scope**

**Rationale**:
- Warm pools are expensive (idle GPUs)
- Current approach: set minReplicas >= 1 for critical models
- Can be added as future enhancement

---

## Technical Decisions

### TD1: DeploymentMode Field Location

**Decision**: Add to both `AIModelSpec` AND `ModelRecipeSpec`

**Rationale**:
- Recipe provides default (e.g., GPU runtime recipes default to RawDeployment)
- AIModel can override if needed (e.g., testing with Serverless)
- Hierarchy: AIModel override > Recipe setting > Runtime default

### TD2: Backward Compatibility

**Decision**: New field is optional, defaults to runtime-based inference

**Rationale**:
- Existing AIModels/Recipes continue working without changes
- Gradual migration possible
- No breaking changes

### TD3: Implicit Logic Removal Timing

**Decision**: Remove implicit nodeSelector logic in Phase 5 (cleanup), not earlier

**Rationale**:
- Keep as fallback during Phase 2-3
- Remove only after explicit mode is proven in production
- Reduces risk during migration

### TD4: Test Strategy

**Decision**: Unit tests first, integration tests manual in development cluster

**Rationale**:
- Unit tests can verify mode selection logic exhaustively
- Integration tests require real cluster with KServe
- E2E tests can be added post-migration

---

## Alternatives Considered

### Alternative: Annotation-based Configuration

Instead of CRD fields, use annotations on AIModel:
```yaml
metadata:
  annotations:
    ai-aas.io/deployment-mode: RawDeployment
```

**Rejected**: CRD fields provide schema validation, documentation, and better UX than annotations.

### Alternative: Separate CRD for RawDeployment

Create `GPUModel` CRD that always uses RawDeployment.

**Rejected**: Duplicates most of AIModel logic. Single CRD with mode selection is cleaner.

### Alternative: Remove Knative Entirely

Since GPU workloads don't benefit from Knative, remove it from the cluster.

**Rejected**: CPU workloads (embeddings, classifiers) benefit from scale-to-zero. Keep Knative for those.

---

## References

- [KServe Deployment Modes](https://kserve.github.io/website/latest/admin/serverless/serverless/)
- [Kubernetes HPA](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale/)
- Spec 029: TensorRT-LLM/Triton Support
- Impact Analysis: [impact.md](./impact.md)
