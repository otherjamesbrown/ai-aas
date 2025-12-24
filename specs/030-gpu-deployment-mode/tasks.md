# Tasks: GPU Deployment Mode Migration

**Feature**: `030-gpu-deployment-mode`
**Generated**: 2025-12-17
**Source**: plan.md, spec.md, impact.md
**Type**: Migration
**Epic**: ai-aas-spec030

## Phase 1: Prepare (Backward Compatible)

**Risk**: LOW | **Rollback**: Remove new fields, regenerate CRDs

- [ ] T001 [ADD] Add DeploymentMode field to AIModelSpec in `operators/ai-model-operator/api/v1alpha1/aimodel_types.go`
- [ ] T002 [ADD] Add DeploymentMode field to ModelRecipeSpec in `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`
- [ ] T003 [ADD] Add AutoscalingSpec struct in `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`
- [ ] T004 [P] Run `make generate && make manifests` in `operators/ai-model-operator/`
- [ ] T005 [P] Apply updated CRDs to development cluster via ArgoCD

## Phase 2: Implement

**Risk**: MEDIUM | **Rollback**: Revert operator code

### Builder Changes
- [ ] T006 [MODIFY] Add `deploymentMode` field to InferenceServiceBuilder struct in `operators/ai-model-operator/internal/kserve/inferenceservice.go`
- [ ] T007 [ADD] Add `WithDeploymentMode()` builder method in `operators/ai-model-operator/internal/kserve/inferenceservice.go`
- [ ] T008 [MODIFY] Update `Build()` to use explicit mode (keep implicit fallback) in `operators/ai-model-operator/internal/kserve/inferenceservice.go`
- [ ] T009 [MODIFY] Update `BuildContainerBased()` to use explicit mode in `operators/ai-model-operator/internal/kserve/inferenceservice.go`

### Controller Changes
- [ ] T010 [ADD] Implement `determineDeploymentMode()` function in `operators/ai-model-operator/controllers/aimodel_controller.go`
- [ ] T011 [MODIFY] Update builder calls to use `WithDeploymentMode()` in `operators/ai-model-operator/controllers/aimodel_controller.go`

### HPA Support
- [ ] T012 [ADD] Create HPA builder in `operators/ai-model-operator/internal/kserve/hpa.go` (new file)

### Tests
- [ ] T013 [ADD] Add `TestInferenceServiceBuilder_WithDeploymentMode` in `operators/ai-model-operator/internal/kserve/inferenceservice_test.go`
- [ ] T014 [ADD] Add `TestInferenceServiceBuilder_Build_RawDeployment` in `operators/ai-model-operator/internal/kserve/inferenceservice_test.go`
- [ ] T015 [ADD] Add `TestDetermineDeploymentMode_*` tests in `operators/ai-model-operator/controllers/aimodel_controller_test.go`
- [ ] T016 [P] Run `go test ./...` in `operators/ai-model-operator/`

## Phase 3: Migrate

**Risk**: HIGH | **Rollback**: Set deploymentMode: Serverless on affected models

- [ ] T017 Deploy updated operator to development cluster
- [ ] T018 Verify GPU models use RawDeployment annotation
- [ ] T019 Verify InferenceServices created successfully
- [ ] T020 Verify pods scheduled to GPU nodes
- [ ] T021 Test inference requests work end-to-end
- [ ] T022 [MODIFY] Update GPU recipes with explicit `deploymentMode: RawDeployment` in ai-aas-config repo

## Phase 4: Documentation

**Risk**: LOW | **Rollback**: Revert doc changes

### Human Documentation (`/docs/`)
- [ ] T023 [UPDATE] Add deploymentMode documentation in `docs/operators/ai-model-operator.md`
- [ ] T024 [ADD] Create GPU deployment runbook in `docs/runbooks/gpu-deployment-mode.md`
- [ ] T025 [UPDATE] Update with RawDeployment guidance in `docs/platform/vllm-best-practices.md`
- [ ] T026 [UPDATE] Document new CRD fields in `operators/ai-model-operator/README.md`

### Agent Context (`/context/`) - CRITICAL
- [ ] T027 [UPDATE] Add deployment mode patterns/anti-patterns in `context/operator-developer/agents.md`

## Phase 5: Cleanup

**Risk**: LOW | **Rollback**: N/A (implicit logic already unused)

- [ ] T028 [REMOVE] Remove implicit nodeSelector logic from `Build()` in `operators/ai-model-operator/internal/kserve/inferenceservice.go` (lines 243-249)
- [ ] T029 [REMOVE] Remove implicit nodeSelector logic from `BuildContainerBased()` in `operators/ai-model-operator/internal/kserve/inferenceservice.go` (lines 430-435)
- [ ] T030 [ADD] Create cleanup script in `scripts/cleanup-stale-revisions.sh`
- [ ] T031 Run cleanup script in dry-run mode
- [ ] T032 Confirm stale Knative revisions cleaned up

## Rollback Checkpoints

- After Phase 1: Remove new CRD fields, regenerate
- After Phase 2: Revert operator code
- After Phase 3: Set deploymentMode: Serverless on affected models
- After Phase 4: Revert doc changes
- After Phase 5: N/A (cleanup only)

## Dependencies

```yaml
phase_1:
  T004: [T001, T002, T003]  # Generate after types added
  T005: [T004]               # Deploy after generated

phase_2:
  T007: [T006]               # Method needs field
  T008: [T007]               # Build needs method
  T009: [T007]               # BuildContainerBased needs method
  T011: [T010]               # Builder calls need determineDeploymentMode
  T013: [T007]               # Test needs method
  T014: [T008]               # Test needs Build update
  T015: [T010]               # Test needs function
  T016: [T013, T014, T015]   # Run tests after all tests written

phase_3:
  T017: [T016]               # Deploy after tests pass
  T018: [T017]               # Verify after deploy
  T019: [T017]
  T020: [T017]
  T021: [T017]
  T022: [T018, T019, T020, T021]  # Update recipes after validation

phase_4:
  T023: [T022]               # Docs after migration validated
  T024: [T022]
  T025: [T022]
  T026: [T022]
  T027: [T022]

phase_5:
  T028: [T027]               # Remove after docs complete
  T029: [T027]
  T030: [T029]               # Script after code cleanup
  T031: [T030]
  T032: [T031]
```

## Task Summary

| Phase | Tasks | Parallel |
|-------|-------|----------|
| 1. Prepare | T001-T005 | T004, T005 can run in parallel after deps |
| 2. Implement | T006-T016 | T013-T015 tests can run in parallel |
| 3. Migrate | T017-T022 | T018-T021 verification in parallel |
| 4. Documentation | T023-T027 | T023-T027 all parallel |
| 5. Cleanup | T028-T032 | T028-T029 parallel |

**Total**: 32 tasks across 5 phases
