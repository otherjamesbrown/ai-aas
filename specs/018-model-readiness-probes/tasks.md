# Tasks: Model Readiness Probes for KServe InferenceServices

**Input**: Design documents from `/specs/018-model-readiness-probes/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Tests**: Manual E2E validation tests specified in acceptance criteria. No automated test framework required (infrastructure/configuration feature).

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

**Implementation Status Note**: Probe configurations are **already implemented** in all InferenceService manifests. This task list focuses on **validation, testing, and documentation**.

## Task Naming Convention

**Format**: `T-S018-P{phase}-{task}`

- **Spec Number**: 018 (Model Readiness Probes)
- **Phase Number**: Two-digit phase number
- **Task Number**: Three-digit sequential task number

---

## Phase 1: Setup (Verification & Prerequisite Checks)

**Purpose**: Verify existing probe configurations and establish validation baseline

- [ ] T-S018-P01-001 Verify cluster access and kubectl configured for development environment
- [ ] T-S018-P01-002 [P] Review existing probe configuration in `infra/k8s/kserve/models/gpt-oss-20b.yaml`
- [ ] T-S018-P01-003 [P] Review existing probe configuration in `infra/k8s/kserve/models/mistral-7b-instruct.yaml`
- [ ] T-S018-P01-004 [P] Review existing probe configuration in `infra/k8s/kserve/models/llama-2-7b.yaml`
- [ ] T-S018-P01-005 Document probe configuration differences per model size (output to `specs/018-model-readiness-probes/validation-notes.md`)

**Checkpoint**: Current state documented, ready for validation testing

---

## Phase 2: Foundational (Environment Validation)

**Purpose**: Confirm deployed InferenceServices have functioning probes before proceeding

**⚠️ CRITICAL**: Must confirm probes are active in deployed pods before documentation work

- [ ] T-S018-P02-006 Verify ArgoCD has synced latest manifests to development cluster
- [ ] T-S018-P02-007 Check pod status for gpt-oss-20b: confirm `2/2 Running` after model load
- [ ] T-S018-P02-008 Check pod status for mistral-7b-instruct: confirm `2/2 Running` after model load
- [ ] T-S018-P02-009 Check pod status for llama-2-7b: confirm `2/2 Running` after model load
- [ ] T-S018-P02-010 Verify probe configuration visible in `kubectl describe pod` output for each model

**Checkpoint**: Foundation ready - probes confirmed active, validation testing can begin

---

## Phase 3: User Story 1 - Configure Readiness Probes (Priority: P1) 🎯 MVP

**Goal**: Verify HTTP-based readiness probes are correctly checking vLLM's `/health` endpoint

**Independent Test**: Deploy an InferenceService, verify pod doesn't become Ready until vLLM reports model loaded, send test request immediately after pod Ready status, confirm successful response.

### Validation for User Story 1

- [ ] T-S018-P03-011 [US1] Delete and recreate gpt-oss-20b pod to observe probe lifecycle
- [ ] T-S018-P03-012 [US1] Monitor pod status transition: verify `1/2 Running` during model load
- [ ] T-S018-P03-013 [US1] Verify pod becomes `2/2 Running` only after `/health` returns 200
- [ ] T-S018-P03-014 [US1] Send test inference request immediately after pod Ready, verify success (no timeout)
- [ ] T-S018-P03-015 [US1] Measure first request latency after pod Ready, verify <5 seconds (NFR-002)
- [ ] T-S018-P03-016 [US1] Check kubectl events for expected probe failure messages during load
- [ ] T-S018-P03-017 [US1] Document validation results in `specs/018-model-readiness-probes/validation-us1.md`

**Checkpoint**: US1 validated - readiness probes correctly gate traffic until model loaded

---

## Phase 4: User Story 2 - Apply Probes to All InferenceServices (Priority: P1)

**Goal**: Confirm standardized probe configurations are applied across all models via GitOps

**Independent Test**: Verify each InferenceService YAML has probes, ArgoCD sync completed, rolling updates work correctly.

### Validation for User Story 2

- [ ] T-S018-P04-018 [US2] Verify gpt-oss-20b manifest has startup (90×10s), readiness (10s), liveness (30s) probes
- [ ] T-S018-P04-019 [US2] Verify mistral-7b-instruct manifest has startup (36×10s), readiness (10s), liveness (30s) probes
- [ ] T-S018-P04-020 [US2] Verify llama-2-7b manifest has startup (36×10s), readiness (10s), liveness (30s) probes
- [ ] T-S018-P04-021 [US2] Confirm ArgoCD Application status is Synced for kserve models
- [ ] T-S018-P04-022 [US2] Perform rolling restart of one model, verify no downtime during update
- [ ] T-S018-P04-023 [US2] Check api-router-service logs for BACKEND_ERROR during scaling (should be zero)
- [ ] T-S018-P04-024 [US2] Verify `minReplicas: 1` configured in all production InferenceService manifests (FR-008)
- [ ] T-S018-P04-025 [US2] Document validation results in `specs/018-model-readiness-probes/validation-us2.md`

**Checkpoint**: US2 validated - all InferenceServices have consistent probe configurations

---

## Phase 5: User Story 3 - Configure Liveness Probes (Priority: P2)

**Goal**: Verify liveness probes detect and restart unhealthy pods

**Independent Test**: Simulate vLLM hang/crash, verify liveness probe triggers pod restart, confirm recovery.

### Validation for User Story 3

- [ ] T-S018-P05-025 [US3] Verify liveness probe configuration: periodSeconds=30, failureThreshold=3
- [ ] T-S018-P05-026 [US3] Document expected liveness behavior (90s to restart after failure detection)
- [ ] T-S018-P05-027 [US3] Check pod restart count via `kube_pod_container_status_restarts_total` metric
- [ ] T-S018-P05-028 [US3] Establish baseline for liveness probe false positive monitoring (NFR-003: target <1%)
- [ ] T-S018-P05-029 [US3] Optional: Simulate vLLM hang in non-production pod to test liveness probe
- [ ] T-S018-P05-030 [US3] Document validation results in `specs/018-model-readiness-probes/validation-us3.md`

**Checkpoint**: US3 validated - liveness probes correctly detect and recover unhealthy pods

---

## Phase 6: User Story 4 - Configure Startup Probes for Large Models (Priority: P2)

**Goal**: Verify startup probes allow large models (20B+) to complete loading without premature termination

**Independent Test**: Deploy 20B model, verify startup probe allows 15-minute load without killing pod.

### Validation for User Story 4

- [ ] T-S018-P06-031 [US4] Verify gpt-oss-20b startup probe: initialDelay=30s, period=10s, failureThreshold=90 (~15 min)
- [ ] T-S018-P06-032 [US4] Monitor gpt-oss-20b pod during cold start, confirm no premature termination
- [ ] T-S018-P06-033 [US4] Verify startup probe hands off to readiness/liveness after success
- [ ] T-S018-P06-034 [US4] Document actual model load time vs configured timeout
- [ ] T-S018-P06-035 [US4] Update `specs/018-model-readiness-probes/contracts/probe-config-templates.yaml` if adjustments needed
- [ ] T-S018-P06-036 [US4] Document validation results in `specs/018-model-readiness-probes/validation-us4.md`

**Checkpoint**: US4 validated - startup probes correctly accommodate large model loading times

---

## Phase 7: User Story 5 - Document Best Practices (Priority: P3)

**Goal**: Create comprehensive documentation for probe configuration

**Independent Test**: Follow documentation to configure probes for a new model, verify correct behavior.

### Implementation for User Story 5

- [ ] T-S018-P07-037 [P] [US5] Create runbook at `docs/runbooks/enable-model-readiness-probes.md`
- [ ] T-S018-P07-038 [P] [US5] Add probe configuration section to `docs/best-practices/vllm-deployment-best-practices.md`
- [ ] T-S018-P07-039 [US5] Document probe parameters by model size in runbook (7B, 13B, 20B, 70B+)
- [ ] T-S018-P07-040 [US5] Add troubleshooting section for common probe issues
- [ ] T-S018-P07-041 [US5] Add examples for configuring new model probes
- [ ] T-S018-P07-042 [US5] Update template at `infra/k8s/kserve/templates/inference-service-vllm-template.yaml` with detailed comments

**Checkpoint**: US5 validated - documentation enables self-service probe configuration

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Final validation and documentation cleanup

- [ ] T-S018-P08-043 [P] Consolidate validation notes into final report at `specs/018-model-readiness-probes/validation-report.md`
- [ ] T-S018-P08-044 [P] Verify Grafana dashboard shows pod readiness metrics correctly
- [ ] T-S018-P08-045 Check for `context deadline exceeded` errors in api-router logs (should be zero post-implementation)
- [ ] T-S018-P08-046 Update `specs/018-model-readiness-probes/spec.md` status from Validation to Complete
- [ ] T-S018-P08-047 Run quickstart.md validation - follow guide to verify probe configuration
- [ ] T-S018-P08-048 Create PR to merge feature branch `018-model-readiness-probes` to development

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 - BLOCKS all user story validation
- **Phases 3-7 (User Stories)**: All depend on Phase 2 completion
  - US1 (P1) and US2 (P1) should be validated first
  - US3 (P2) and US4 (P2) can proceed after P1 stories
  - US5 (P3) can proceed after P2 stories
- **Phase 8 (Polish)**: Depends on all user story phases complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Phase 2 - No dependencies on other stories
- **User Story 2 (P1)**: Can start after Phase 2 - Validates same manifests as US1
- **User Story 3 (P2)**: Can start after Phase 2 - Independent validation
- **User Story 4 (P2)**: Can start after Phase 2 - Independent validation  
- **User Story 5 (P3)**: Depends on US1-4 validation results for accurate documentation

### Within Each User Story

- Validation tasks are sequential within each story
- Documentation tasks (marked [P]) can run in parallel
- Validation results inform documentation content

### Parallel Opportunities

- All review tasks in Phase 1 marked [P] can run in parallel
- US3 and US4 can be validated in parallel
- Documentation tasks in US5 marked [P] can run in parallel
- Polish documentation tasks marked [P] can run in parallel

---

## Parallel Example: Phase 1 Setup

```bash
# Launch all manifest reviews together:
Task: "Review existing probe configuration in infra/k8s/kserve/models/gpt-oss-20b.yaml"
Task: "Review existing probe configuration in infra/k8s/kserve/models/mistral-7b-instruct.yaml"
Task: "Review existing probe configuration in infra/k8s/kserve/models/llama-2-7b.yaml"
```

---

## Parallel Example: User Story 5 Documentation

```bash
# Launch documentation tasks together:
Task: "Create runbook at docs/runbooks/enable-model-readiness-probes.md"
Task: "Add probe configuration section to docs/best-practices/vllm-deployment-best-practices.md"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (verify existing configurations)
2. Complete Phase 2: Foundational (confirm probes active in cluster)
3. Complete Phase 3: User Story 1 (validate readiness probes)
4. **STOP and VALIDATE**: Test inference after pod Ready
5. Document findings and proceed

### Incremental Delivery

1. Complete Setup + Foundational → Baseline established
2. Validate User Story 1 → Readiness probes confirmed working ✓
3. Validate User Story 2 → All manifests consistent ✓
4. Validate User Story 3 + 4 → Liveness and startup probes confirmed ✓
5. Complete User Story 5 → Documentation complete ✓
6. Each story adds confidence without breaking previous validation

### Suggested MVP Scope

**For minimum viable completion**: Complete Phases 1-4 (US1 + US2)

This validates:
- Readiness probes prevent traffic to unready pods
- All InferenceServices have consistent configurations
- No BACKEND_ERROR timeout errors during scaling

---

## Notes

- [P] tasks = different files or independent operations, no dependencies
- [USn] label maps task to specific user story for traceability
- This is a **validation-focused** task list (probes already implemented)
- All validation should be performed in **development** environment first
- Document actual observed behavior vs expected behavior
- Avoid: making configuration changes without validating current state first
- Commit validation documents after each user story completion

