# Tasks: Shared Libraries & Conventions

**Input**: Design documents from `/specs/004-shared-libraries/`
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Include contract and integration verification where they provide measurable safety for upgrades.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish workspace structure and tooling for polyglot shared libraries.

- [x] T-S004-P01-001 Create shared directory skeleton (`shared/`, `policies/`, `dashboards/`, `samples/`) per plan.md
- [x] T-S004-P01-002 Configure Go module entries for shared libraries in `shared/go/go.mod` and update `go.work`
- [x] T-S004-P01-003 [P] Configure TypeScript workspace with `shared/ts/package.json` and shared build scripts
- [x] T-S004-P01-004 [P] Seed policy bundle index and dashboard scaffolding in `policies/bundles/index.json` and `dashboards/README.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required by all user stories.

**⚠️ CRITICAL**: Complete before tackling user stories.

- [x] T-S004-P02-005 Implement shared build tooling targets for Go/TS libraries in `Makefile` and `scripts/ci/run-local.sh`
- [x] T-S004-P02-006 [P] Establish shared testing harnesses (`tests/go/unit`, `tests/ts/unit`) with baseline configs
- [x] T-S004-P02-007 [P] Create OPA policy bundle distribution pipeline scripts in `policies/bundles/Makefile`
- [x] T-S004-P02-008 Provision reference sample service skeleton in `samples/service-template/` with Docker Compose telemetry stack

**Checkpoint**: Foundation ready—user stories can proceed independently.

---

## Phase 3: User Story 1 - Accelerate new service development (Priority: P1) 🎯 MVP

**Goal**: Deliver reusable components and quickstart enabling a new service to adopt shared libraries end-to-end in under 60 minutes.

**Independent Test**: Use `samples/service-template` to integrate shared libraries, run health check, emit telemetry, and verify standardized error responses without modifying library code.

### Implementation for User Story 1

- [x] T-S004-P03-009 [P] [US1] Implement Go configuration loader with validation in `shared/go/config/config.go`
- [x] T-S004-P03-010 [P] [US1] Implement Go data access helpers (connection guards, health probes) in `shared/go/dataaccess/health.go`
- [x] T-S004-P03-011 [P] [US1] Implement Go observability bootstrap with OpenTelemetry in `shared/go/observability/otel.go`
- [x] T-S004-P03-012 [P] [US1] Implement Go standardized error types in `shared/go/errors/errors.go`
- [x] T-S004-P03-013 [US1] Compose Go sample service wiring config, data access, observability, errors, and health checks in `samples/service-template/go/main.go`
- [x] T-S004-P03-014 [P] [US1] Implement TypeScript configuration loader with validation in `shared/ts/config/index.ts`
- [x] T-S004-P03-015 [P] [US1] Implement TypeScript data access helpers (connection pooling, health probes) in `shared/ts/dataaccess/health.ts`
- [x] T-S004-P03-016 [P] [US1] Implement TypeScript observability bootstrap in `shared/ts/observability/index.ts`
- [x] T-S004-P03-017 [P] [US1] Implement TypeScript error helpers aligned with JSON schema in `shared/ts/errors/index.ts`
- [x] T-S004-P03-018 [US1] Wire TypeScript sample service in `samples/service-template/ts/src/index.ts` using config, data access, observability, and errors
- [x] T-S004-P03-019 [US1] Document quickstart integration flow and update `specs/004-shared-libraries/quickstart.md`
- [x] T-S004-P03-020 [US1] Create automated end-to-end smoke script in `samples/service-template/scripts/smoke.sh`

**Checkpoint**: MVP ready—new service scaffold can adopt shared libraries with telemetry and standardized errors.

---

## Phase 4: User Story 2 - Consistent authorization and request handling (Priority: P2)

**Goal**: Provide authorization middleware and request metadata instrumentation ensuring consistent security and telemetry.

**Independent Test**: Enable middleware in sample services, run allow/deny scenarios, confirm audit events, metrics, and standardized error payloads emitted.

### Implementation for User Story 2

- [x] T-S004-P04-021 [P] [US2] Implement Go authorization middleware integrating policy bundles in `shared/go/auth/middleware.go`
- [x] T-S004-P04-022 [P] [US2] Implement TypeScript authorization middleware integrating policy bundles in `shared/ts/auth/middleware.ts`
- [x] T-S004-P04-023 [P] [US2] Add request context injectors (request IDs, trace IDs) in `shared/go/observability/middleware.go`
- [x] T-S004-P04-024 [P] [US2] Add request context injectors for Node in `shared/ts/observability/middleware.ts`
- [x] T-S004-P04-025 [US2] Extend sample Go service with protected routes and policy tests in `samples/service-template/go/cmd/service-template/main.go`
- [x] T-S004-P04-026 [US2] Extend sample TypeScript service with protected routes and policy tests in `samples/service-template/ts/src/index.ts`
- [x] T-S004-P04-027 [US2] Publish audit event formatter shared between languages in `shared/go/auth/audit.go` and `shared/ts/auth/audit.ts`
- [x] T-S004-P04-028 [US2] Validate middleware via integration tests in `tests/go/integration/authz_test.go`
- [x] T-S004-P04-029 [US2] Validate middleware via integration tests in `tests/ts/integration/src/authz.spec.ts`

**Checkpoint**: Authorization and request handling standardized with telemetry and audit coverage.

---

## Phase 5: User Story 3 - Maintainability and quality (Priority: P3)

**Goal**: Ensure libraries are documented, versioned, and test-hardened so upgrades do not break consumers unexpectedly.

**Independent Test**: Upgrade sample services between library versions using compatibility scripts; CI contract tests stay green without consumer code changes.

- [ ] T-S004-P05-030 [P] [US3] Implement semantic versioning and changelog automation in `.github/workflows/shared-libraries-release.yml`
- [ ] T-S004-P05-031 [P] [US3] Add contract tests for error schema in `tests/go/contract/error_response_test.go` and `tests/ts/contract/error-response.spec.ts`
- [ ] T-S004-P05-032 [P] [US3] Add contract tests for telemetry profiles in `tests/go/contract/telemetry_profile_test.go` and `tests/ts/contract/telemetry-profile.spec.ts`
- [ ] T-S004-P05-033 [US3] Create upgrade checklist and compatibility script in `docs/upgrades/shared-libraries.md` and `scripts/shared/upgrade-verify.sh`
- [ ] T-S004-P05-034 [US3] Configure CI matrix to run consumer-driven tests against sample service in `.github/workflows/shared-libraries-ci.yml`
- [ ] T-S004-P05-035 [US3] Add coverage reporting thresholds (>80%) in `shared/go/Makefile` and `shared/ts/package.json`
- [ ] T-S004-P05-036 [US3] Publish documentation site or README updates for library APIs in `shared/README.md`

**Checkpoint**: Libraries stable with upgrade guidance, automated compatibility checks, and documented APIs.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final refinements, performance validation, and operational readiness.

- [x] T-S004-P06-037 [P] Finalize Grafana dashboards and alert templates in `dashboards/grafana/shared-libraries.json` and `dashboards/alerts/shared-libraries.yaml`
- [x] T-S004-P06-038 Harden telemetry fallback logic and add chaos tests in `tests/go/integration/telemetry_failover_test.go` and `tests/ts/integration/telemetry-failover.spec.ts`
- [x] T-S004-P06-039 [P] Produce troubleshooting and runbook docs in `docs/runbooks/shared-libraries.md`
- [x] T-S004-P06-040 Conduct quickstart validation dry-run and capture findings in `specs/004-shared-libraries/quickstart.md`
- [x] T-S004-P06-041 [P] Implement Go performance benchmark suite ensuring ≤5% overhead in `tests/go/perf/middleware_bench_test.go`
- [x] T-S004-P06-042 [P] Implement TypeScript performance benchmark suite ensuring ≤5% overhead in `tests/ts/perf/middleware.bench.spec.ts`
- [x] T-S004-P06-043 Integrate benchmark gating into CI and publish results in `docs/perf/shared-libraries.md`

---

## Phase 7: Pilot Adoption & Measurement

**Purpose**: Validate real-service adoption and quantify boilerplate reduction per success criteria.

- [x] T-S004-P07-044 Coordinate pilot adoption with two existing services and capture implementation plan in `docs/adoption/pilot-plan.md`
- [ ] T-S004-P07-045 [P] Measure and document boilerplate reduction (≥30%) across pilots in `docs/adoption/pilot-results.md`
- [x] T-S004-P07-046 [P] Update `llms.txt` and `docs/runbooks/shared-libraries.md` with pilot lessons and version skew mitigation guidance

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)** → prerequisite for all other phases.
- **Foundational (Phase 2)** → depends on Setup; blocks user stories until complete.
- **User Stories (Phases 3–5)** → each depends on Foundational completion; execute in priority order (P1 → P2 → P3) or in parallel once prerequisites satisfied.
- **Polish (Phase 6)** → begins after desired user stories finish.

### User Story Dependencies

- **User Story 1 (P1)**: Independent after Foundational; delivers MVP scaffold.
- **User Story 2 (P2)**: Depends on shared middleware hooks from Phase 2 and integrates with US1 sample services.
- **User Story 3 (P3)**: Depends on artifacts from US1 and US2 for compatibility testing.

### Within Each User Story

- Implement library components before wiring sample services.
- Integration tests follow component implementations.
- Documentation updates close each story to maintain traceability.

### Parallel Opportunities

- Tasks marked `[P]` can execute concurrently (different files or services).
- Separate language implementations (Go vs TypeScript) can be developed in parallel once interfaces agreed.
- Contract tests for different schemas can run concurrently.
- Dashboard and runbook work (Phase 6) can overlap with final testing if resources available.

---

## Parallel Example: User Story 2

```bash
# Parallelizable tasks once policy loader interface defined:
sprint:
  - T-S004-P04-021 Implement Go auth middleware in shared/go/auth/middleware.go
  - T-S004-P04-022 Implement TypeScript auth middleware in shared/ts/auth/middleware.ts
  - T-S004-P04-023 Add Go request context injector in shared/go/observability/middleware.go
  - T-S004-P04-024 Add Node request context injector in shared/ts/observability/middleware.ts
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup (Phase 1).
2. Complete Foundational (Phase 2).
3. Deliver User Story 1 (Phase 3).
4. Validate sample service end-to-end and gather feedback before expanding scope.

### Incremental Delivery

1. Setup + Foundational establish reusable baseline.
2. Add User Story 1 for MVP.
3. Layer User Story 2 for security and advanced telemetry.
4. Layer User Story 3 for upgrade safety and documentation.
5. Execute Polish phase for operational readiness.

### Parallel Team Strategy

- After Phase 2, assign developers per language or user story:
  - Developer A: Go implementations for US1 & US2.
  - Developer B: TypeScript implementations for US1 & US2.
  - Developer C: Quality/upgrade tooling for US3 and dashboards.
- Synchronize via sample service integration checkpoints and shared contract tests.

---

## Notes

- `[P]` indicates parallel-safe tasks.
- `[US#]` ties tasks to specific user stories for traceability.
- Maintain semver discipline and update documentation alongside code changes.
- Ensure contract tests fail before implementation (where applicable) to uphold upgrade guarantees.

