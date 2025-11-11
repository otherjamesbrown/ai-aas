# Tasks: Database Schemas & Migrations

**Input**: Design documents from `/specs/003-database-schemas/`  
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks are grouped by user story so each slice remains independently implementable and testable.

## Format: `[ID] [P?] [Story] Description`

- **[P]** → task can run in parallel (distinct files, no ordering dependency)
- **[Story]** → maps to spec user stories (`US1`, `US2`, `US3`)
- Every description calls out the exact file path(s) involved

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish repository scaffolding and documentation hooks referenced by the implementation plan.

- [ ] T001 Add scaffolding placeholders (`db/.gitkeep`, `analytics/.gitkeep`, `scripts/db/.gitkeep`) to materialize directory structure from plan.md
- [ ] T002 Create migration environment example at `configs/migrate.example.env` enumerating required variables (DB_URL, ANALYTICS_URL, OTEL headers)
- [ ] T003 [P] Extend root `README.md` with database prerequisites pointing to `specs/003-database-schemas/quickstart.md`
- [ ] T004 [P] Register new phony targets (`db-migrate-status`, `db-docs-generate`, `db-docs-validate`) inside root `Makefile`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core tooling, governance, and documentation shells required before implementing user stories.  
**⚠️ CRITICAL**: No user story may begin until this phase is complete.

- [ ] T005 Scaffold telemetry-aware migration CLI entrypoint in `db/tools/migrate/main.go` (flag parsing, OTEL bootstrap, exit handling)
- [ ] T006 [P] Author migration apply wrapper script `scripts/db/apply.sh` (env loading, target selection, logging structure)
- [ ] T007 [P] Author rollback wrapper script `scripts/db/rollback.sh` with safeguards for partial runs
- [ ] T008 Create smoke-cycle harness `scripts/db/smoke-test.sh` to execute apply → rollback → reapply against local containers
- [ ] T009 Define security governance map in `configs/data-classification.yml` with classifications/retention for all entities
- [ ] T010 Seed documentation shell in `db/docs/dictionary.md` (front matter, placeholder sections per entity)
- [ ] T011 [P] Generate ERD baseline placeholder `db/docs/erd/schema.puml` containing legend + stub nodes
- [ ] T012 Configure migration tool settings in `configs/migrate.yaml` (source paths, telemetry sink, retry/backoff)

**Checkpoint**: Migration tooling, governance, and documentation scaffolds are ready.

---

## Phase 3: User Story 1 – Stable Data Model for Core Services (Priority: P1) 🎯 MVP

**Goal**: Deliver canonical operational schema, documentation, and deterministic seeds for core entities.  
**Independent Test**: Run `scripts/db/validate-schema.sh` against local Postgres to confirm schema matches `db/docs/dictionary.md` and rejects invalid foreign keys.

- [ ] T013 [P] [US1] Author core entity DDL (`Organization`, `User`, `APIKey`, `ModelRegistryEntry`, `AuditLogEntry`) in `db/migrations/operational/20251115001_core_entities.up.sql`
- [ ] T014 [P] [US1] Provide matching rollback script `db/migrations/operational/20251115001_core_entities.down.sql`
- [ ] T015 [P] [US1] Define time-partitioned usage event fact table in `db/migrations/operational/20251115002_usage_events.up.sql`
- [ ] T016 [P] [US1] Provide rollback for usage events + partition cleanup at `db/migrations/operational/20251115002_usage_events.down.sql`
- [ ] T017 [US1] Populate entity dictionary (attributes, relationships, constraints) in `db/docs/dictionary.md`
- [ ] T018 [P] [US1] Update ERD diagram nodes/edges in `db/docs/erd/schema.puml` to match finalized DDL
- [ ] T019 [P] [US1] Implement naming/index lint enforcement in `db/tools/lint/naming.go` (prefix patterns, partition checks)
- [ ] T020 [US1] Implement deterministic tenant/user/key bootstrap with idempotency guards in `db/seeds/operational/seed.go`
- [ ] T021 [P] [US1] Seed analytics fixtures (usage events, models, budgets) in `db/seeds/analytics/seed.sql`
- [ ] T022 [US1] Create schema validation script `scripts/db/validate-schema.sh` to diff live DB vs documentation catalog
- [ ] T023 [US1] Document and enforce hashing/encryption rules for sensitive fields in `db/tools/lint/naming.go` and `db/seeds/operational/seed.go`
- [ ] T024 [US1] Benchmark core operational query latency (target P95 < 200 ms) via `tests/perf/operational_queries_test.go`

**Checkpoint**: Schema, docs, linting, and seeds align; MVP delivers independent value.

---

## Phase 4: User Story 2 – Safe, Versioned Schema Changes (Priority: P2)

**Goal**: Provide migration process with telemetry, guardrails, and recovery documentation.  
**Independent Test**: Execute `scripts/db/smoke-test.sh --env staging` to apply/rollback a sample change while capturing OTEL metrics and health probe output.

- [ ] T025 [P] [US2] Add span + structured logging instrumentation to `db/tools/migrate/main.go` (version tags, duration metrics, status attributes)
- [ ] T026 [P] [US2] Build pre/post check hooks in `db/tools/migrate/hooks/pre_post.go` (row count diff, long-running detection)
- [ ] T027 [P] [US2] Wire guardrails (dry-run, approval prompts, change-set lint) into `scripts/db/apply.sh`
- [ ] T028 [US2] Extend `scripts/db/rollback.sh` with partial-apply detection and failure annotations
- [ ] T029 [US2] Publish migration status probe `scripts/db/status.sh` exposing schema version + last migration timestamp
- [ ] T030 [US2] Document migration procedure, telemetry dashboards, and rollback guidance in `docs/runbooks/migrations.md`
- [ ] T031 [US2] Define dual-approval workflow in `docs/runbooks/migrations.md` and enforce reviewer check via `scripts/db/apply.sh`
- [ ] T032 [US2] Measure migration apply/rollback duration with `scripts/db/measure-window.sh` to verify 10-minute window compliance
- [ ] T033 [US2] Verify rollback downtime stays under 1 minute via `tests/perf/rollback_downtime_test.go` and document mitigation steps in `docs/runbooks/migrations.md`
- [ ] T034 [US2] Emit audit log entries (actor, action, timestamp, metadata) from migration hooks in `db/tools/migrate/hooks/audit.go` and validate outputs

**Checkpoint**: Migration workflows observable, reversible, and documented for operators.

---

## Phase 5: User Story 3 – Efficient Analytics Over Time (Priority: P3)

**Goal**: Deliver analytics warehouse schema, transforms, and reconciliation tooling to satisfy rollup SLAs.  
**Independent Test**: Run `make analytics-verify` to load sample data, execute rollups, and confirm reconciliation within SLA (<2s queries, consistent aggregates).

- [ ] T035 [P] [US3] Create analytics rollup DDL (hour/day tables, indexes) in `db/migrations/analytics/20251116001_rollups.up.sql`
- [ ] T036 [P] [US3] Provide rollback script `db/migrations/analytics/20251116001_rollups.down.sql`
- [ ] T037 [P] [US3] Implement hourly rollup transform handling late arrivals in `analytics/transforms/hourly_rollup.sql`
- [ ] T038 [P] [US3] Author data quality assertions in `analytics/tests/hourly_rollup.yml` (counts, token sums, error ratios)
- [ ] T039 [US3] Build reconciliation harness `analytics/tests/reconciliation_test.go` comparing aggregates vs raw usage events
- [ ] T040 [US3] Update operator guidance in `specs/003-database-schemas/quickstart.md` with analytics run + verification steps
- [ ] T041 [P] [US3] Implement adapter configuration for warehouse vs local targets in `analytics/transforms/config.yml`
- [ ] T042 [US3] Document adapter switch instructions in `specs/003-database-schemas/quickstart.md`
- [ ] T043 [US3] Add rollup duration monitoring (<5 minutes/hour) in `analytics/tests/perf/rollup_duration_test.go`

**Checkpoint**: Analytics queries satisfy performance and data-quality requirements.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Finalization tasks spanning multiple user stories.

- [ ] T044 Update progress tracker entry for spec 003 in `docs/specs-progress.md`
- [ ] T045 [P] Add CI workflow `.github/workflows/db-guardrails.yml` enforcing smoke tests and lint on `db/` & `analytics/` changes
- [ ] T046 [P] Publish release summary at `docs/release-notes/003-database-schemas.md`
- [ ] T047 Verify quickstart instructions end-to-end by running `scripts/db/smoke-test.sh` and capturing results in `specs/003-database-schemas/quickstart.md`
- [ ] T048 [P] Wire `db-docs-generate` and `db-docs-validate` into `.github/workflows/db-guardrails.yml`
- [ ] T049 [P] Enforce migration filename pattern `YYYYMMDDHHMM_slug` via `db/tools/lint/naming.go` and CI guardrail
- [ ] T050 Validate migration scripts across local (Docker), CI, and managed staging environments via matrix job in `.github/workflows/db-guardrails.yml`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)** → enables foundational tooling.
- **Foundational (Phase 2)** → must complete before any user story; delivers migration CLI, scripts, and governance.
- **User Story 1 (Phase 3)** → builds MVP schema/seeds; US2 & US3 rely on its artifacts.
- **User Story 2 (Phase 4)** → depends on US1 migrations to exercise guardrails; can overlap with US3 once US1 data exists.
- **User Story 3 (Phase 5)** → relies on US1 seeds + documentation and foundational configs; telemetry from US2 enhances observability but is not a hard blocker.
- **Polish (Phase 6)** → executes after targeted user stories complete.

### User Story Dependencies

- **US1**: Independent after foundational phase; produces MVP.
- **US2**: Requires US1 schema + seeds to validate guardrails.
- **US3**: Requires US1 data model + seeds; optionally consumes US2 telemetry for dashboards.

### Within Each User Story

- Up/Down migration pairs (e.g., T013/T014, T035/T036) should be implemented together.
- Documentation tasks (T017, T040, T042, T047) follow structural changes but can be staged in parallel once DDL stabilizes.
- Seeds and linting (T019–T021) can execute while ERD/doc updates progress.
- Independent test or validation tasks (T022, T033, T041, T043, T047) must run before closing the phase.

### Parallel Opportunities

- Setup: T003 and T004 proceed once scaffolding (T001) begins.
- Foundational: T006, T007, and T011 operate concurrently after CLI scaffold (T005).
- US1: Engineers split DDL (T013–T016), docs (T017–T018), and seeds/lint/security/perf work (T019–T024).
- US2: Instrumentation (T025), hooks (T026), guardrail scripts (T027–T032), downtime validation (T033), and audit logging (T034) can advance in parallel once CLI scaffold is ready.
- US3: Rollup DDL (T035/T036) and transforms/tests/adapters (T037–T043) can advance concurrently after foundational work.
- Polish: CI workflow (T045), documentation automation (T048), release notes (T046), and environment matrix validation (T050) proceed independently of quickstart validation (T047).

---

## Parallel Example: User Story 1

```bash
# Engineer A – core schema DDL
Task T013 -> db/migrations/operational/20251115001_core_entities.up.sql
Task T014 -> db/migrations/operational/20251115001_core_entities.down.sql

# Engineer B – usage events & documentation
Task T015 -> db/migrations/operational/20251115002_usage_events.up.sql
Task T016 -> db/migrations/operational/20251115002_usage_events.down.sql
Task T017 -> db/docs/dictionary.md

# Engineer C – linting & seeds
Task T019 -> db/tools/lint/naming.go
Task T020 -> db/seeds/operational/seed.go
Task T021 -> db/seeds/analytics/seed.sql
```

---

## Implementation Strategy

### MVP First (User Story 1)
1. Complete Phases 1–2 to enable tooling and governance.
2. Implement Phase 3 and validate via `scripts/db/validate-schema.sh`.
3. Demo schema ERD + seeds to stakeholders before proceeding.

### Incremental Delivery
1. Ship US1 (stable schema) → unlocks services to develop against shared model.
2. Ship US2 (migration guardrails) → ensures safe operations in staging/production.
3. Ship US3 (analytics rollups) → provides observability & reporting capabilities.

### Parallel Team Strategy
1. Team jointly finishes Setup + Foundational phases.
2. Assign leads per story:
   - Engineer A: US1 (schema, docs, seeds)
   - Engineer B: US2 (tooling, guardrails)
   - Engineer C: US3 (analytics transforms)
3. Regroup for Polish tasks to finalize CI, release notes, and quickstart validation.

---

## Notes

- Keep migrations, documentation, and seeds in sync within each PR.
- Update `llms.txt` once plan/research/data-model/contracts/quickstart/tasks are live.
- Coordinate with security on `configs/data-classification.yml` before merging sensitive-field changes.
- After major steps, run `make db-migrate-status` and `scripts/db/smoke-test.sh` to catch regressions early.

