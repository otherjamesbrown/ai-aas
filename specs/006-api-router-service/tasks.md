# Tasks: API Router Service

**Input**: Design documents from `/specs/006-api-router-service/`
**Prerequisites**: Go 1.21 toolchain, shared service framework (`004-shared-libraries`), access to Redis and Kafka dev clusters, Config Service credentials
**Tests**: Contract (`buf`), integration (`docker-compose` harness), load (`vegeta`), chaos drills (backend failover, exporter outage)
**Organization**: Tasks grouped by setup, foundational work, user stories, then polish; all user story tasks carry `[US#]`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish repository structure, build tooling, and developer ergonomics

- [ ] T-S006-P01-001 Create service scaffold directories in `services/api-router-service/` (`cmd/router/`, `internal/auth/`, `internal/limiter/`, `internal/routing/`, `internal/usage/`, `internal/admin/`, `internal/telemetry/`, `pkg/contracts/`, `configs/`, `deployments/helm/api-router-service/`, `scripts/`, `docs/`, `test/`, `dev/`)
- [ ] T-S006-P01-002 Initialize Go module with shared framework dependencies in `services/api-router-service/go.mod`
- [ ] T-S006-P01-003 [P] Author bootstrap Makefile with run/test targets in `services/api-router-service/Makefile`
- [ ] T-S006-P01-004 [P] Seed sample runtime configs in `services/api-router-service/configs/router.sample.yaml` and `services/api-router-service/configs/policies.sample.yaml`
- [ ] T-S006-P01-005 [P] Create developer docker-compose harness for Redis, Kafka, and mock backends in `services/api-router-service/dev/docker-compose.yml`
- [ ] T-S006-P01-006 [P] Add GitHub Actions workflow running Go, contract, and integration checks in `.github/workflows/api-router-service.yml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

- [ ] T-S006-P02-007 Implement configuration loader consuming Config Service watch stream in `services/api-router-service/internal/config/loader.go`
- [ ] T-S006-P02-008 [P] Persist configuration cache to BoltDB in `services/api-router-service/internal/config/cache.go`
- [ ] T-S006-P02-009 [P] Wire telemetry bootstrap with zap logger and OpenTelemetry exporters in `services/api-router-service/internal/telemetry/telemetry.go`
- [ ] T-S006-P02-010 [P] Compose shared middleware stack and lifecycle management in `services/api-router-service/cmd/router/main.go`
- [ ] T-S006-P02-011 [P] Define contract generation workflow using `buf` in `services/api-router-service/pkg/contracts/generate.go` and supporting targets in `services/api-router-service/Makefile`

**Checkpoint**: Foundation ready — proceed to user stories

---

## Phase 3: User Story 1 - Route authenticated inference requests (Priority: P1) 🎯 MVP

**Goal**: Deliver authenticated inference routing that returns completions with latency metadata
**Independent Test**: Send a signed request to `/v1/inference` and receive a valid completion payload with router-computed metrics

- [ ] T-S006-P03-012 [P] [US1] Add contract test for `POST /v1/inference` in `services/api-router-service/test/contract/inference_contract_test.go`
- [ ] T-S006-P03-013 [P] [US1] Add integration test covering happy-path routing in `services/api-router-service/test/integration/inference_success_test.go`
- [ ] T-S006-P03-014 [P] [US1] Implement request DTOs and validation logic in `services/api-router-service/internal/api/public/inference.go`
- [ ] T-S006-P03-015 [P] [US1] Implement API key and HMAC authentication adapter in `services/api-router-service/internal/auth/authenticator.go`
- [ ] T-S006-P03-016 [P] [US1] Implement backend client wrapper for synchronous completions in `services/api-router-service/internal/routing/backend_client.go`
- [ ] T-S006-P03-017 [US1] Compose `/v1/inference` handler pipeline with error mapping in `services/api-router-service/internal/api/public/handler.go`
- [ ] T-S006-P03-018 [US1] Register public router and middleware stack in `services/api-router-service/cmd/router/http.go`

**Parallel Example**:
```
Task: T-S006-P03-012
Task: T-S006-P03-013
Task: T-S006-P03-014
```

---

## Phase 4: User Story 2 - Enforce budgets and safe usage (Priority: P1)

**Goal**: Enforce budget, quota, and rate limits with auditable denials
**Independent Test**: Simulate limit exhaustion and confirm HTTP 402/429 responses plus durable audit records

- [ ] T-S006-P04-019 [P] [US2] Add integration test for quota exhaustion denial in `services/api-router-service/test/integration/limiter_budget_test.go`
- [ ] T-S006-P04-020 [P] [US2] Implement budget service client integration in `services/api-router-service/internal/limiter/budget_client.go`
- [ ] T-S006-P04-021 [P] [US2] Implement Redis token-bucket limiter wrapper in `services/api-router-service/internal/limiter/rate_limiter.go`
- [ ] T-S006-P04-022 [P] [US2] Attach rate-limit and budget middleware to request pipeline in `services/api-router-service/internal/api/public/middleware.go`
- [ ] T-S006-P04-023 [US2] Emit audit events for deny and queue outcomes in `services/api-router-service/internal/usage/audit_logger.go`
- [ ] T-S006-P04-024 [US2] Produce structured limit error responses and metrics in `services/api-router-service/internal/telemetry/limits.go`

**Parallel Example**:
```
Task: T-S006-P04-020
Task: T-S006-P04-021
Task: T-S006-P04-022
```

---

## Phase 5: User Story 3 - Intelligent routing and fallback (Priority: P2)

**Goal**: Provide weighted routing, automatic failover, and admin overrides with observability
**Independent Test**: Apply routing fixtures, simulate backend degradation, and observe automatic failover with decision logs

- [ ] T-S006-P05-025 [P] [US3] Add routing weight distribution test in `services/api-router-service/test/integration/routing_weights_test.go`
- [ ] T-S006-P05-026 [P] [US3] Implement routing policy cache with Config Service watch updates in `services/api-router-service/internal/routing/policy_cache.go`
- [ ] T-S006-P05-027 [P] [US3] Implement backend health probe scheduler in `services/api-router-service/internal/routing/health_monitor.go`
- [ ] T-S006-P05-028 [P] [US3] Implement routing engine with failover and weight logic in `services/api-router-service/internal/routing/engine.go`
- [ ] T-S006-P05-029 [US3] Expose admin routing override endpoints in `services/api-router-service/internal/api/admin/routing_handlers.go`
- [ ] T-S006-P05-030 [US3] Publish routing decision metrics and alerts in `services/api-router-service/internal/telemetry/routing_metrics.go`

**Parallel Example**:
```
Task: T-S006-P05-026
Task: T-S006-P05-027
Task: T-S006-P05-028
```

---

## Phase 6: User Story 4 - Accurate, timely usage accounting (Priority: P2)

**Goal**: Emit near-real-time usage records with at-least-once guarantees and audit retrieval
**Independent Test**: Generate requests with known usage and verify Kafka records plus audit API responses within SLA

- [ ] T-S006-P06-031 [P] [US4] Add contract test validating usage record schema in `services/api-router-service/test/contract/usage_record_contract_test.go`
- [ ] T-S006-P06-032 [P] [US4] Implement usage record builder capturing routing metadata in `services/api-router-service/internal/usage/record.go`
- [ ] T-S006-P06-033 [P] [US4] Implement Kafka exporter using shared publisher in `services/api-router-service/internal/usage/publisher.go`
- [ ] T-S006-P06-034 [P] [US4] Persist exporter buffer state to disk in `services/api-router-service/internal/usage/buffer_store.go`
- [ ] T-S006-P06-035 [US4] Hook usage emission and retry logic into inference flow in `services/api-router-service/internal/api/public/usage_hook.go`
- [ ] T-S006-P06-036 [US4] Implement audit lookup endpoint `/v1/audit/requests/{requestId}` in `services/api-router-service/internal/api/public/audit_handler.go`

**Parallel Example**:
```
Task: T-S006-P06-031
Task: T-S006-P06-032
Task: T-S006-P06-033
```

---

## Phase 7: User Story 5 - Operational visibility and reliability (Priority: P3)

**Goal**: Deliver health endpoints, metrics, dashboards, and runbooks for operational readiness
**Independent Test**: Exercise health endpoints, dashboards, and runbooks during simulated incident to confirm 15-minute recovery

- [ ] T-S006-P07-037 [P] [US5] Add integration tests for `/v1/status/healthz` and `/v1/status/readyz` in `services/api-router-service/test/integration/health_endpoints_test.go`
- [ ] T-S006-P07-038 [P] [US5] Implement health and readiness handlers with component probes in `services/api-router-service/internal/api/public/status_handlers.go`
- [ ] T-S006-P07-039 [P] [US5] Integrate per-backend metrics and tracing exporters in `services/api-router-service/internal/telemetry/exporters.go`
- [ ] T-S006-P07-040 [US5] Author Grafana dashboard definitions for router operations in `deployments/helm/api-router-service/dashboards/api-router.json`
- [ ] T-S006-P07-041 [US5] Document incident response runbooks in `services/api-router-service/docs/runbooks.md`

**Parallel Example**:
```
Task: T-S006-P07-037
Task: T-S006-P07-038
Task: T-S006-P07-039
```

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Service-wide hardening, documentation, and release readiness

- [ ] T-S006-P08-042 [P] Harden error catalog and response codes in `services/api-router-service/internal/api/errors.go`
- [ ] T-S006-P08-043 [P] Finalize Helm values and alert rules for staging/production in `deployments/helm/api-router-service/values.yaml`
- [ ] T-S006-P08-044 [P] Tune load test scenarios for SLO coverage in `services/api-router-service/scripts/loadtest.sh`
- [ ] T-S006-P08-045 [P] Extend smoke test coverage for limiter and routing validation in `services/api-router-service/scripts/smoke.sh`
- [ ] T-S006-P08-046 Validate quickstart instructions and capture follow-ups in `services/api-router-service/docs/quickstart-validation.md`

---

## Dependencies & Execution Order

### Phase Dependencies
- Setup (Phase 1): No dependencies — run immediately
- Foundational (Phase 2): Depends on Phase 1 completion — blocks all user stories
- User Stories (Phases 3–7): Each depends on Phase 2; execute in priority order (P1 → P1 → P2 → P2 → P3) or in parallel once prerequisites met
- Polish (Phase 8): Depends on desired user stories completing

### User Story Dependencies
- User Story 1 (P1): Independent once foundational infrastructure is ready
- User Story 2 (P1): Requires limiter integration points from US1 handler pipeline but can progress in parallel after T-S006-P03-017
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
- **US1**: T-S006-P03-012, T-S006-P03-013, and T-S006-P03-014 can proceed together after foundational middleware exists
- **US2**: T-S006-P04-020, T-S006-P04-021, and T-S006-P04-022 run concurrently while T-S006-P04-019 executes against the shared harness
- **US3**: T-S006-P05-026, T-S006-P05-027, and T-S006-P05-028 can run in parallel once policy cache scaffolding is ready
- **US4**: T-S006-P06-031, T-S006-P06-032, and T-S006-P06-033 build independent components before wiring T-S006-P06-035
- **US5**: T-S006-P07-037, T-S006-P07-038, and T-S006-P07-039 progress simultaneously leveraging telemetry groundwork

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
