# Tasks: API Router Service

**Input**: Design documents from `/specs/006-api-router-service/`
**Prerequisites**: Go 1.21 toolchain, shared service framework (`004-shared-libraries`), access to Redis and Kafka dev clusters, Config Service credentials
**Tests**: Contract (`buf`), integration (`docker-compose` harness), load (`vegeta`), chaos drills (backend failover, exporter outage)
**Organization**: Tasks grouped by setup, foundational work, user stories, then polish; all user story tasks carry `[US#]`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish repository structure, build tooling, and developer ergonomics

- [ ] T001 Create service scaffold directories in `services/api-router-service/` (`cmd/router/`, `internal/auth/`, `internal/limiter/`, `internal/routing/`, `internal/usage/`, `internal/admin/`, `internal/telemetry/`, `pkg/contracts/`, `configs/`, `deployments/helm/api-router-service/`, `scripts/`, `docs/`, `test/`, `dev/`)
- [ ] T002 Initialize Go module with shared framework dependencies in `services/api-router-service/go.mod`
- [ ] T003 [P] Author bootstrap Makefile with run/test targets in `services/api-router-service/Makefile`
- [ ] T004 [P] Seed sample runtime configs in `services/api-router-service/configs/router.sample.yaml` and `services/api-router-service/configs/policies.sample.yaml`
- [ ] T005 [P] Create developer docker-compose harness for Redis, Kafka, and mock backends in `services/api-router-service/dev/docker-compose.yml`
- [ ] T006 [P] Add GitHub Actions workflow running Go, contract, and integration checks in `.github/workflows/api-router-service.yml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

- [ ] T007 Implement configuration loader consuming Config Service watch stream in `services/api-router-service/internal/config/loader.go`
- [ ] T008 [P] Persist configuration cache to BoltDB in `services/api-router-service/internal/config/cache.go`
- [ ] T009 [P] Wire telemetry bootstrap with zap logger and OpenTelemetry exporters in `services/api-router-service/internal/telemetry/telemetry.go`
- [ ] T010 [P] Compose shared middleware stack and lifecycle management in `services/api-router-service/cmd/router/main.go`
- [ ] T011 [P] Define contract generation workflow using `buf` in `services/api-router-service/pkg/contracts/generate.go` and supporting targets in `services/api-router-service/Makefile`

**Checkpoint**: Foundation ready — proceed to user stories

---

## Phase 3: User Story 1 - Route authenticated inference requests (Priority: P1) 🎯 MVP

**Goal**: Deliver authenticated inference routing that returns completions with latency metadata
**Independent Test**: Send a signed request to `/v1/inference` and receive a valid completion payload with router-computed metrics

- [ ] T012 [P] [US1] Add contract test for `POST /v1/inference` in `services/api-router-service/test/contract/inference_contract_test.go`
- [ ] T013 [P] [US1] Add integration test covering happy-path routing in `services/api-router-service/test/integration/inference_success_test.go`
- [ ] T014 [P] [US1] Implement request DTOs and validation logic in `services/api-router-service/internal/api/public/inference.go`
- [ ] T015 [P] [US1] Implement API key and HMAC authentication adapter in `services/api-router-service/internal/auth/authenticator.go`
- [ ] T016 [P] [US1] Implement backend client wrapper for synchronous completions in `services/api-router-service/internal/routing/backend_client.go`
- [ ] T017 [US1] Compose `/v1/inference` handler pipeline with error mapping in `services/api-router-service/internal/api/public/handler.go`
- [ ] T018 [US1] Register public router and middleware stack in `services/api-router-service/cmd/router/http.go`

**Parallel Example**:
```
Task: T012
Task: T013
Task: T014
```

---

## Phase 4: User Story 2 - Enforce budgets and safe usage (Priority: P1)

**Goal**: Enforce budget, quota, and rate limits with auditable denials
**Independent Test**: Simulate limit exhaustion and confirm HTTP 402/429 responses plus durable audit records

- [ ] T019 [P] [US2] Add integration test for quota exhaustion denial in `services/api-router-service/test/integration/limiter_budget_test.go`
- [ ] T020 [P] [US2] Implement budget service client integration in `services/api-router-service/internal/limiter/budget_client.go`
- [ ] T021 [P] [US2] Implement Redis token-bucket limiter wrapper in `services/api-router-service/internal/limiter/rate_limiter.go`
- [ ] T022 [P] [US2] Attach rate-limit and budget middleware to request pipeline in `services/api-router-service/internal/api/public/middleware.go`
- [ ] T023 [US2] Emit audit events for deny and queue outcomes in `services/api-router-service/internal/usage/audit_logger.go`
- [ ] T024 [US2] Produce structured limit error responses and metrics in `services/api-router-service/internal/telemetry/limits.go`

**Parallel Example**:
```
Task: T020
Task: T021
Task: T022
```

---

## Phase 5: User Story 3 - Intelligent routing and fallback (Priority: P2)

**Goal**: Provide weighted routing, automatic failover, and admin overrides with observability
**Independent Test**: Apply routing fixtures, simulate backend degradation, and observe automatic failover with decision logs

- [ ] T025 [P] [US3] Add routing weight distribution test in `services/api-router-service/test/integration/routing_weights_test.go`
- [ ] T026 [P] [US3] Implement routing policy cache with Config Service watch updates in `services/api-router-service/internal/routing/policy_cache.go`
- [ ] T027 [P] [US3] Implement backend health probe scheduler in `services/api-router-service/internal/routing/health_monitor.go`
- [ ] T028 [P] [US3] Implement routing engine with failover and weight logic in `services/api-router-service/internal/routing/engine.go`
- [ ] T029 [US3] Expose admin routing override endpoints in `services/api-router-service/internal/api/admin/routing_handlers.go`
- [ ] T030 [US3] Publish routing decision metrics and alerts in `services/api-router-service/internal/telemetry/routing_metrics.go`

**Parallel Example**:
```
Task: T026
Task: T027
Task: T028
```

---

## Phase 6: User Story 4 - Accurate, timely usage accounting (Priority: P2)

**Goal**: Emit near-real-time usage records with at-least-once guarantees and audit retrieval
**Independent Test**: Generate requests with known usage and verify Kafka records plus audit API responses within SLA

- [ ] T031 [P] [US4] Add contract test validating usage record schema in `services/api-router-service/test/contract/usage_record_contract_test.go`
- [ ] T032 [P] [US4] Implement usage record builder capturing routing metadata in `services/api-router-service/internal/usage/record.go`
- [ ] T033 [P] [US4] Implement Kafka exporter using shared publisher in `services/api-router-service/internal/usage/publisher.go`
- [ ] T034 [P] [US4] Persist exporter buffer state to disk in `services/api-router-service/internal/usage/buffer_store.go`
- [ ] T035 [US4] Hook usage emission and retry logic into inference flow in `services/api-router-service/internal/api/public/usage_hook.go`
- [ ] T036 [US4] Implement audit lookup endpoint `/v1/audit/requests/{requestId}` in `services/api-router-service/internal/api/public/audit_handler.go`

**Parallel Example**:
```
Task: T031
Task: T032
Task: T033
```

---

## Phase 7: User Story 5 - Operational visibility and reliability (Priority: P3)

**Goal**: Deliver health endpoints, metrics, dashboards, and runbooks for operational readiness
**Independent Test**: Exercise health endpoints, dashboards, and runbooks during simulated incident to confirm 15-minute recovery

- [ ] T037 [P] [US5] Add integration tests for `/v1/status/healthz` and `/v1/status/readyz` in `services/api-router-service/test/integration/health_endpoints_test.go`
- [ ] T038 [P] [US5] Implement health and readiness handlers with component probes in `services/api-router-service/internal/api/public/status_handlers.go`
- [ ] T039 [P] [US5] Integrate per-backend metrics and tracing exporters in `services/api-router-service/internal/telemetry/exporters.go`
- [ ] T040 [US5] Author Grafana dashboard definitions for router operations in `deployments/helm/api-router-service/dashboards/api-router.json`
- [ ] T041 [US5] Document incident response runbooks in `services/api-router-service/docs/runbooks.md`

**Parallel Example**:
```
Task: T037
Task: T038
Task: T039
```

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Service-wide hardening, documentation, and release readiness

- [ ] T042 [P] Harden error catalog and response codes in `services/api-router-service/internal/api/errors.go`
- [ ] T043 [P] Finalize Helm values and alert rules for staging/production in `deployments/helm/api-router-service/values.yaml`
- [ ] T044 [P] Tune load test scenarios for SLO coverage in `services/api-router-service/scripts/loadtest.sh`
- [ ] T045 [P] Extend smoke test coverage for limiter and routing validation in `services/api-router-service/scripts/smoke.sh`
- [ ] T046 Validate quickstart instructions and capture follow-ups in `services/api-router-service/docs/quickstart-validation.md`

---

## Dependencies & Execution Order

### Phase Dependencies
- Setup (Phase 1): No dependencies — run immediately
- Foundational (Phase 2): Depends on Phase 1 completion — blocks all user stories
- User Stories (Phases 3–7): Each depends on Phase 2; execute in priority order (P1 → P1 → P2 → P2 → P3) or in parallel once prerequisites met
- Polish (Phase 8): Depends on desired user stories completing

### User Story Dependencies
- User Story 1 (P1): Independent once foundational infrastructure is ready
- User Story 2 (P1): Requires limiter integration points from US1 handler pipeline but can progress in parallel after T017
- User Story 3 (P2): Requires policy cache hooks laid down in foundational work; no additional story dependencies
- User Story 4 (P2): Requires inference pipeline from US1 and audit hooks from US2 to emit records
- User Story 5 (P3): Builds on telemetry wiring from foundational work and routing data from US3/US4 for dashboards

### Within Each User Story
- Write contract/integration tests before implementation tasks and confirm they fail
- Implement models/clients before handlers that depend on them
- Register endpoints only after middleware and business logic exist
- Complete story verification before moving to the next priority

---

## Parallel Execution Examples (Expanded)
- **US1**: T012, T013, and T014 can proceed together after foundational middleware exists
- **US2**: T020, T021, and T022 run concurrently while T019 executes against the shared harness
- **US3**: T026, T027, and T028 can run in parallel once policy cache scaffolding is ready
- **US4**: T031, T032, and T033 build independent components before wiring T035
- **US5**: T037, T038, and T039 progress simultaneously leveraging telemetry groundwork

---

## Implementation Strategy

### MVP First (User Story 1)
1. Complete Phases 1 and 2 to establish infrastructure
2. Deliver Phase 3 (US1) and validate via contract and integration tests
3. Deploy MVP to staging and confirm routing metrics

### Incremental Delivery
1. Add Phase 4 (US2) for budget enforcement — validate audit records
2. Add Phase 5 (US3) for intelligent routing — verify failover scenarios
3. Add Phase 6 (US4) for usage accounting — confirm Kafka exports
4. Add Phase 7 (US5) for operational visibility — ensure dashboards and runbooks ready

### Parallel Team Strategy
- While one team finalizes US1, another can stage US2 limiter components after middleware landing
- Routing specialists can begin US3 policy and health modules once foundational config tooling ships
- Data/analytics teammates can implement US4 publisher pieces in parallel with US3 failover work
- SRE partners can tackle US5 observability tasks concurrently once telemetry scaffolding is available
