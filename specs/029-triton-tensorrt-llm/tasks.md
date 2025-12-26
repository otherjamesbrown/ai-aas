# Spec 029: TensorRT-LLM/Triton Support - Tasks

## Task Breakdown

### Phase 1: KServe Infrastructure

- [ ] **1.1** Create ClusterServingRuntime for TensorRT-LLM
  - File: `infra/k8s/kserve/base/cluster-serving-runtime-tensorrt-llm.yaml`
  - Image: `nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3`
  - Health probes: `/v2/health/live`, `/v2/health/ready` on port 8080
  - Initial delay: 600s (TRT engine load time)

- [ ] **1.2** Create PodMonitor for Prometheus metrics
  - File: `infra/k8s/kserve/monitoring/podmonitor-tensorrt-llm.yaml`
  - Scrape port 8002, path `/metrics`

- [ ] **1.3** Add ArgoCD Application for ClusterServingRuntime (if needed)

---

### Phase 2: Operator Changes

- [ ] **2.1** Add `tensorrt-llm` to runtime mapping
  - File: `operators/ai-model-operator/controllers/aimodel_controller.go`
  - Location: Lines 1144-1157
  - Map to `modelFormat: tensorrt-llm`, `runtimeName: kserve-tensorrt-llm`

- [ ] **2.2** Make health probes configurable in InferenceServiceBuilder
  - File: `operators/ai-model-operator/internal/kserve/inferenceservice.go`
  - Location: Lines 484-501
  - Add: `livenessPath`, `readinessPath`, `probePort` fields
  - Add: `WithHealthProbes()` builder method
  - Update: `BuildContainerBased()` to use fields instead of hardcoded values

- [ ] **2.3** Pass HealthCheckSpec from recipe to builder
  - File: `operators/ai-model-operator/controllers/aimodel_controller.go`
  - Location: Around line 1230
  - Read `recipeSpec.HealthCheck.LivenessPath` and `ReadinessPath`
  - Fall back to runtime-aware defaults

- [ ] **2.4** Implement Triton runtime args conversion
  - File: `operators/ai-model-operator/controllers/aimodel_controller.go`
  - Location: Lines 1697-1701 (empty `case "triton":`)
  - Convert `TritonArgs` to env vars / args

- [ ] **2.5** Add TensorRT-LLM validation rules
  - File: `operators/ai-model-operator/internal/recipe/validator.go`
  - Validate: `tensorrt` backend requires GPU
  - Validate: GPU vendor must be `nvidia`

- [ ] **2.6** Add unit tests for new operator functionality
  - Test runtime mapping
  - Test health probe configuration
  - Test validation rules

---

### Phase 3: CRD & Domain Updates

- [ ] **3.1** Update ModelRecipe Runtime enum
  - File: `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`
  - Location: Line 39
  - Change: `Enum=vllm;triton;tgi` → `Enum=vllm;triton;tensorrt-llm;tgi`

- [ ] **3.2** Update AIModel Runtime enum
  - File: `operators/ai-model-operator/api/v1alpha1/aimodel_types.go`
  - Add `tensorrt-llm` to enum

- [ ] **3.3** Update ValidRuntimes in Admin API
  - File: `services/admin-api-service/internal/domain/recipe.go`
  - Location: Line 55
  - Add `"tensorrt-llm"` to slice

- [ ] **3.4** Regenerate CRDs
  - Run: `cd operators/ai-model-operator && make generate && make manifests`
  - Verify: CRD YAML updated with new enum value

---

### Phase 4: Model Recipe & Deployment

- [ ] **4.1** Create Llama 3.1 8B Instruct recipe
  - File: `infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml`
  - Runtime: `tensorrt-llm`
  - Resources: 1 GPU, 24GB min memory, 8 CPU, 32Gi RAM
  - Health check paths: `/v2/health/live`, `/v2/health/ready`

- [ ] **4.2** Document S3 model repository structure
  - Add to spec or create separate doc
  - Include: ensemble, preprocessing, postprocessing, tensorrt_llm directories

- [ ] **4.3** Build TensorRT-LLM engines for Llama 3.1 8B (external)
  - Convert HuggingFace checkpoint
  - Build TRT engine with trtllm-build
  - Upload to S3

---

### Phase 5: Documentation

- [ ] **5.1** Create engine build runbook
  - File: `docs/runbooks/build-tensorrt-llm-engine.md`
  - TensorRT-LLM installation
  - Checkpoint conversion
  - Engine building commands
  - S3 upload process

- [ ] **5.2** Update platform docs with TensorRT-LLM support
  - Add to supported runtimes list
  - Document health check differences

---

### Phase 6: Testing & Validation

- [ ] **6.1** Deploy ClusterServingRuntime to development cluster
  - Verify runtime appears in `kubectl get clusterservingruntimes`

- [ ] **6.2** Deploy operator with new changes
  - Build and push new operator image
  - Update operator deployment

- [ ] **6.3** Apply Llama 3.1 8B recipe
  - Create ModelRecipe in cluster
  - Verify validation passes

- [ ] **6.4** Deploy AIModel with TensorRT-LLM runtime
  - Create AIModel referencing recipe
  - Verify InferenceService created with correct health probes

- [ ] **6.5** End-to-end inference test
  - Send chat completion request
  - Verify response quality

---

## Implementation Order

1. Phase 1 (Infrastructure) - Can be done independently
2. Phase 3 (CRD Updates) - Must be done before operator changes
3. Phase 2 (Operator) - Depends on CRD updates
4. Phase 4 (Recipe) - Depends on operator changes
5. Phase 5 (Docs) - Can be done in parallel
6. Phase 6 (Testing) - Final validation

---

## Dependencies

```
Phase 1 ──────────────────────────────────────────┐
                                                   │
Phase 3.1-3.4 (CRD) ──► Phase 2 (Operator) ──────►│
                                                   │
Phase 4.3 (Build Engines) ──► Phase 4.1 (Recipe) ─┤
                                                   │
Phase 5 (Docs) ───────────────────────────────────┤
                                                   │
                                                   ▼
                                          Phase 6 (Testing)
```

---

## Estimated Effort

| Phase | Tasks | Complexity |
|-------|-------|------------|
| Phase 1 | 3 | Low |
| Phase 2 | 6 | High |
| Phase 3 | 4 | Low |
| Phase 4 | 3 | Medium |
| Phase 5 | 2 | Low |
| Phase 6 | 5 | Medium |

**Total**: 23 tasks
