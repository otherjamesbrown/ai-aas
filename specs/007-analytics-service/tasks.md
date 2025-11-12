# Tasks: Analytics Service

**Input**: Design documents from `/specs/007-analytics-service/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Integration and data-quality tests are included where needed to keep each story independently verifiable.

**Organization**: Tasks are grouped by user story so each slice can ship independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Task can execute in parallel (different files, no hard dependency)
- **[Story]**: User story label (US1, US2, US3)
- Include exact file paths in each description

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish analytics service scaffolding and tooling integration.

- [ ] T-S007-P01-001 Create analytics service module scaffold (`services/analytics-service/cmd/analytics-service/main.go`, `internal/`, `pkg/`, `README.md`) per plan structure.
- [ ] T-S007-P01-002 Register analytics service with build orchestration (`go.work`, root `Makefile`, `scripts/analytics/run-hourly.sh`) so `make analytics-*` targets run locally and in CI.
- [ ] T-S007-P01-003 [P] Generate local development compose stack (`analytics/local-dev/docker-compose.yml`) covering Postgres, Redis, and RabbitMQ endpoints referenced in `quickstart.md`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required before implementing any user story.

**⚠️ CRITICAL**: Complete this phase before starting user story work.

- [ ] T-S007-P02-004 Implement centralized configuration loader with validation (`services/analytics-service/internal/config/config.go`) and wire defaults for DB, RabbitMQ, Redis, S3.
- [ ] T-S007-P02-005 [P] Build HTTP server bootstrap with chi router, health/ready probes, and middleware chain (`services/analytics-service/internal/api/server.go`).
- [ ] T-S007-P02-006 Create ingestion consumer skeleton handling RabbitMQ stream connection, dedupe hooks, and backpressure controls (`services/analytics-service/internal/ingestion/consumer.go`).
- [ ] T-S007-P02-007 Author initial Timescale migrations defining `usage_events`, `ingestion_batches`, `freshness_status` tables (`db/migrations/analytics/0001_init.sql`, `0002_rollups.sql`).
- [ ] T-S007-P02-008 [P] Align SQL transforms with data model (`analytics/transforms/hourly_rollup.sql`, `analytics/transforms/daily_rollup.sql`) including continuous aggregate policies and compression.
- [ ] T-S007-P02-009 Establish observability instrumentation (OpenTelemetry metrics/traces, structured logging) in `services/analytics-service/internal/observability/telemetry.go` and expose Prometheus `/metrics`.

---

## Phase 3: User Story 1 - Org-level usage and spend visibility (Priority: P1) 🎯 MVP

**Goal**: Allow org admins to view usage and estimated spend filtered by model and time range with freshness guarantees.

**Independent Test**: Seed sample events and verify `/analytics/v1/orgs/{orgId}/usage` returns correct aggregates and freshness indicators for the selected org/model window.

### Implementation for User Story 1

- [ ] T-S007-P03-010 [P] [US1] Implement deduplicated persistence pipeline writing RabbitMQ payloads into `usage_events` with batch tracking (`services/analytics-service/internal/ingestion/processor.go`).
- [ ] T-S007-P03-011 [US1] Build rollup worker orchestrating Timescale continuous aggregates and freshness updates (`services/analytics-service/internal/aggregation/rollup_worker.go`).
- [ ] T-S007-P03-012 [US1] Implement usage API handler and routing per `analytics-views-openapi.yaml` (`services/analytics-service/internal/api/usage_handler.go`).
- [ ] T-S007-P03-013 [P] [US1] Add Redis-backed freshness cache and repository (`services/analytics-service/internal/freshness/cache.go`) syncing with `freshness_status`.
- [ ] T-S007-P03-014 [P] [US1] Update Grafana dashboards to surface usage, spend, and freshness (`dashboards/analytics/usage.json`).
- [ ] T-S007-P03-015 [US1] Add integration test validating usage aggregates and freshness lag (`tests/analytics/integration/usage_visibility_test.go`).

**Checkpoint**: Org admins can retrieve accurate usage and spend data within 5 minutes of ingestion.

---

## Phase 4: User Story 2 - Reliability and error insights (Priority: P2)

**Goal**: Enable engineers to detect error/latency spikes, attribute issues, and export recent usage slices for incidents.

**Independent Test**: Inject error events and confirm `/analytics/v1/orgs/{orgId}/reliability` highlights spikes while incident export provides shareable CSV scoped to the incident window.

### Implementation for User Story 2

- [ ] T-S007-P04-016 [P] [US2] Extend aggregation layer with error-rate and latency percentile queries (`services/analytics-service/internal/aggregation/reliability_repository.go`).
- [ ] T-S007-P04-017 [US2] Implement reliability API handler per contract (`services/analytics-service/internal/api/reliability_handler.go`) with percentile selection.
- [ ] T-S007-P04-018 [P] [US2] Wire synthetic freshness/error alerts into Prometheus rules and alertmanager config (`gitops/templates/analytics-alerts.yaml`).
- [ ] T-S007-P04-019 [US2] Build incident export builder generating scoped CSV datasets (`services/analytics-service/internal/exports/incident_exporter.go`).
- [ ] T-S007-P04-020 [US2] Document incident response runbook updates (`docs/runbooks/analytics-incident-response.md`) covering alerts and export usage.
- [ ] T-S007-P04-021 [P] [US2] Add integration test covering reliability API and incident export flow (`tests/analytics/integration/reliability_incident_test.go`).

**Checkpoint**: Engineers can detect and share reliability insights rapidly during incidents.

---

## Phase 5: User Story 3 - Finance-friendly reporting (Priority: P3)

**Goal**: Provide finance stakeholders with reconciled month-to-date cost exports and org breakdowns.

**Independent Test**: Trigger month-to-date export via API, verify CSV totals reconcile within 1% to aggregate queries, and ensure signed URL delivery works.

### Implementation for User Story 3

- [ ] T-S007-P05-022 [P] [US3] Create export job repository and migration for `export_jobs` lifecycle (`services/analytics-service/internal/exports/job_repository.go`, `db/migrations/analytics/0003_exports.sql`).
- [ ] T-S007-P05-023 [US3] Implement export worker pipeline to generate CSVs and upload to S3 (`services/analytics-service/internal/exports/job_runner.go`).
- [ ] T-S007-P05-024 [US3] Expose export management endpoints per OpenAPI (`services/analytics-service/internal/api/exports_handler.go`).
- [ ] T-S007-P05-025 [P] [US3] Implement S3 delivery adapter honoring org-specific prefixes (`services/analytics-service/internal/exports/s3_delivery.go`).
- [ ] T-S007-P05-026 [P] [US3] Add reconciliation integration test ensuring export totals align with rollups (`tests/analytics/integration/export_reconciliation_test.go`).
- [ ] T-S007-P05-027 [US3] Update finance documentation with export process and retention policy (`docs/metrics/report.md`).

**Checkpoint**: Finance can self-serve reconciled exports with audit trail and retention controls.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Harden security, performance, and documentation across stories.

- [ ] T-S007-P06-028 [P] Implement RBAC middleware and audit logging hooks for all analytics endpoints (`services/analytics-service/internal/middleware/rbac.go`, `internal/audit/logger.go`).
- [ ] T-S007-P06-029 Execute load and freshness benchmarks, capturing results in `tests/analytics/perf/freshness_benchmark_test.go` and documenting thresholds.
- [ ] T-S007-P06-030 [P] Finalize quickstart/runbook alignment (`specs/007-analytics-service/quickstart.md`, `docs/runbooks/analytics-incident-response.md`) reflecting shipped behavior.
- [ ] T-S007-P06-031 Update knowledge artifacts (`llms.txt`, `docs/specs-progress.md`) with analytics service links and status.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)** → No dependencies; start immediately.
- **Phase 2 (Foundational)** → Depends on Phase 1; blocks all user stories.
- **Phase 3 (US1)** → Depends on Foundational completion; delivers MVP.
- **Phase 4 (US2)** → Depends on Foundational; may proceed after USA1 if staffed separately, remains independently testable.
- **Phase 5 (US3)** → Depends on Foundational; can run after or alongside US2 once shared exports infrastructure is stable.
- **Phase 6 (Polish)** → Depends on targeted user stories completing.

### User Story Dependencies

- **US1 (P1)**: No dependency on other stories; requires Foundational infrastructure.
- **US2 (P2)**: Builds on shared ingestion/aggregation primitives (Phase 2 + US1 rollups) but exposes independent reliability surfaces.
- **US3 (P3)**: Reuses rollups; independent export pipeline once foundational migrations exist.

### Within Each User Story

- Contract/integration tests (marked [P]) can start once upstream interfaces exist.
- Repository/service components land before HTTP handlers.
- Handlers merge before dashboards/docs updates.
- Integration tests finish each story and must pass before moving downstream.

### Parallel Opportunities

- Setup task T-S007-P01-003 can run alongside T-S007-P01-001–T-S007-P01-002 once paths confirmed.
- Foundational tasks T-S007-P02-005, T-S007-P02-008, T-S007-P02-009 operate in parallel after configuration scaffolding.
- Within US1, T-S007-P03-010, T-S007-P03-013, T-S007-P03-014, and T-S007-P03-014 can execute concurrently with coordination on shared interfaces.
- US2 and US3 can progress in parallel teams after Foundational completion; tasks marked [P] highlight non-blocking workstreams.

---

## Parallel Example: User Story 1

```bash
# Parallel implementation slice once Phase 2 completes:
Task: "T-S007-P03-010 [P] [US1] Implement deduplicated persistence pipeline..."
Task: "T-S007-P03-013 [P] [US1] Add Redis-backed freshness cache..."
Task: "T-S007-P03-014 [P] [US1] Update Grafana dashboards..."

# Parallel validation slice:
Task: "T-S007-P03-011 [US1] Build rollup worker..."  # runs while dashboards prepare
Task: "T-S007-P03-015 [US1] Add integration test validating usage aggregates..."
```

---

## Implementation Strategy

### MVP First (Deliver User Story 1)

1. Complete Phases 1–2 to establish runtime and data plumbing.
2. Execute Phase 3 tasks to ship usage/spend dashboards.
3. Validate via integration test T-S007-P03-015 and Grafana dashboard T-S007-P03-014.
4. Demo MVP to stakeholders and gather feedback before expanding scope.

### Incremental Delivery

1. Ship US1 (MVP) → provides org-level insights.
2. Add US2 → introduces real-time reliability visibility and incident exports.
3. Add US3 → unlocks finance-ready exports without regressing earlier stories.

### Parallel Team Strategy

1. Assemble team to finish Foundational work.
2. Assign streams:
   - Team A: US1 ingestion + API (T-S007-P03-010–T-S007-P03-015)
   - Team B: US2 reliability features (T-S007-P04-016–T-S007-P04-021)
   - Team C: US3 finance exports (T-S007-P05-022–T-S007-P05-027)
3. Converge on Phase 6 for hardening, documentation, and knowledge updates.

---

## Notes

- [P] tasks avoid file conflicts and may run concurrently.
- Story labels ensure traceability back to specification priorities.
- Each story concludes with an integration test or dashboard validation to confirm independent deliverability.
- Update documentation alongside implementation to keep quickstart/runbooks trustworthy.

