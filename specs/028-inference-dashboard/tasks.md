# Tasks: Observability Dashboard Suite

**Feature**: `028-inference-dashboard`
**Generated**: 2024-12-16
**Source**: plan.md, dashboard-suite-spec.md

---

## Phase 1: Foundation Dashboards

### Platform Overview
- [ ] T001 [P] Create `infra/k8s/monitoring/dashboards/platform-overview.json` with health score, API success rate, GPU utilization stats
- [ ] T002 [P] Add data source status panel template to platform-overview
- [ ] T003 Add drill-down links from Platform Overview to other dashboards

### GPU Fleet
- [ ] T004 [P] Create `infra/k8s/monitoring/dashboards/gpu-fleet.json` with GPU inventory table
- [ ] T005 Add GPU utilization/memory heatmaps to gpu-fleet.json
- [ ] T006 Add GPU health panels (temperature, power, ECC errors) to gpu-fleet.json

### Kubernetes Resources
- [ ] T007 [P] Create `infra/k8s/monitoring/dashboards/kubernetes-resources.json` with node status table
- [ ] T008 Add pod status and restart panels to kubernetes-resources.json
- [ ] T009 Add CPU/memory usage panels to kubernetes-resources.json

---

## Phase 2: Performance Dashboards

### Inference Performance
- [ ] T010 [P] Create `infra/k8s/monitoring/dashboards/inference-performance.json` with model selector variable
- [ ] T011 Add throughput panels (requests/sec, tokens/sec) to inference-performance.json
- [ ] T012 Add latency panels (TTFT, E2E, heatmap) to inference-performance.json
- [ ] T013 Add KV cache panels to inference-performance.json
- [ ] T014 Add error breakdown and logs panel to inference-performance.json

### API Performance
- [ ] T015 [P] Create `infra/k8s/monitoring/dashboards/api-performance-v2.json` with org selector variable
- [ ] T016 Add request rate and latency panels to api-performance-v2.json
- [ ] T017 Add error rate and status code breakdown to api-performance-v2.json
- [ ] T018 Add per-org and per-model breakdown panels to api-performance-v2.json

### Inference Engine
- [ ] T019 [P] Create `infra/k8s/monitoring/dashboards/inference-engine.json` with engine selector variable
- [ ] T020 Add request processing panels (running vs waiting) to inference-engine.json
- [ ] T021 Add memory management panels (GPU/CPU cache blocks) to inference-engine.json
- [ ] T022 Add performance breakdown panels (prefill vs decode) to inference-engine.json
- [ ] T023 Add error panels (CUDA errors, OOM) and logs to inference-engine.json

---

## Phase 3: Instrumentation Work

### Task 1: Token Metrics
- [ ] T024 Add `TokensProcessedTotal` counter metric to `services/api-router-service/internal/telemetry/exporters.go`
- [ ] T025 Add `TokensPerRequest` histogram metric to `services/api-router-service/internal/telemetry/exporters.go`
- [ ] T026 Add `RecordTokens()` helper function to `services/api-router-service/internal/telemetry/exporters.go`
- [ ] T027 Call `RecordTokens()` from chat completions handler in `services/api-router-service/internal/api/public/openai.go`
- [ ] T028 Call `RecordTokens()` from text completions handler in `services/api-router-service/internal/api/public/openai.go`
- [ ] T029 Write unit tests for token metrics in `services/api-router-service/internal/telemetry/exporters_test.go`

### Task 2: Cost Configuration
- [ ] T030 Create `infra/k8s/monitoring/cost-config.yaml` ConfigMap with GPU costs

---

## Phase 4: Business Dashboards

### Org Usage
- [ ] T031 Create `infra/k8s/monitoring/dashboards/org-usage.json` with org selector variable
- [ ] T032 Add usage overview stats (total orgs, requests, tokens) to org-usage.json
- [ ] T033 Add usage over time panels (requests by org, tokens by org) to org-usage.json
- [ ] T034 Add usage pattern panels (hour of day heatmap) to org-usage.json

### Cost & Efficiency
- [ ] T035 Create `infra/k8s/monitoring/dashboards/cost-efficiency.json` with cost overview stats
- [ ] T036 Add cost trend and breakdown panels to cost-efficiency.json
- [ ] T037 Add efficiency panels (tokens per GPU-hour, idle time) to cost-efficiency.json

---

## Phase 5: Cleanup

- [ ] T038 Validate all new dashboards load without errors
- [ ] T039 Validate all queries return data
- [ ] T040 Remove `infra/k8s/monitoring/dashboards/api-performance.json`
- [ ] T041 Remove `infra/k8s/monitoring/dashboards/fleet-overview.json`
- [ ] T042 Remove `infra/k8s/monitoring/dashboards/inference-backends.json`
- [ ] T043 Remove `infra/k8s/monitoring/dashboards/node-cluster-view.json`
- [ ] T044 Remove `infra/k8s/monitoring/dashboards/per-gpu-type-analysis.json`
- [ ] T045 Remove `infra/k8s/monitoring/dashboards/per-model-performance.json`
- [ ] T046 Remove `infra/k8s/monitoring/dashboards/service-logs.json`
- [ ] T047 Update kustomization.yaml if needed

---

## Dependencies

```
Phase 1 (T001-T009): No dependencies - can start immediately
Phase 2 (T010-T023): No dependencies - can start immediately
Phase 3 (T024-T030): No dependencies - can start immediately

T031-T034 (Org Usage): Depends on T024-T029 (token metrics deployed)
T035-T037 (Cost): Depends on T024-T029 AND T030

T038-T047 (Cleanup): Depends on T001-T037 all complete
```

---

## Agent Assignment

| Tasks | Agent | Component |
|-------|-------|-----------|
| T001-T023 | `agent:infra-ops` | `component:monitoring` |
| T024-T029 | `agent:go-services` | `component:api-router` |
| T030 | `agent:infra-ops` | `component:monitoring` |
| T031-T037 | `agent:infra-ops` | `component:monitoring` |
| T038-T047 | `agent:infra-ops` | `component:monitoring` |

---

## Parallel Opportunities

**Can run in parallel:**
- T001, T004, T007 (Phase 1 dashboard creation)
- T010, T015, T019 (Phase 2 dashboard creation)
- T024-T029 (Token metrics) parallel with T001-T023 (Dashboards)
- T031, T035 (Business dashboard creation, after T029)

**Total**: 47 tasks across 5 phases
**Parallel opportunities**: ~15 tasks can run concurrently
