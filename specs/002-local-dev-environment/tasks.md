# Tasks: Local Development Environment

**Input**: Design documents from `/specs/002-local-dev-environment/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Tests are included only where the specification or plan calls for verifiable automation (Go unit tests, shell smoke tests). Additional tests may be added during implementation as needed.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Task can run in parallel (different files, no direct dependency)
- **[Story]**: Maps task to the relevant user story label (`US1`, `US2`, `US3`, `US4`)
- Every description includes the exact file path to modify or create

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Prepare repository tooling so contributors can install required dependencies before story work begins.

- [ ] T001 Update Terraform/Vault tooling versions in `configs/tool-versions.mk` for remote/local dev environment support
- [ ] T002 [P] Extend `scripts/setup/bootstrap.sh` to install & verify `terraform`, `linode-cli`, `vault`, and Docker Compose v2
- [ ] T003 [P] Refresh `README.md` prerequisites to reference the local dev quickstart and new tooling expectations

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared assets consumed by all user stories. No story work may begin until these tasks are complete.

- [ ] T004 Create the shared dependency stack definition in `.dev/compose/compose.base.yaml` for Postgres, Redis, NATS, MinIO, and mock inference
- [ ] T005 [P] Seed default port mappings in `.specify/local/ports.yaml` covering database, cache, messaging, and mock inference services
- [ ] T006 [P] Add secret redaction patterns in `configs/log-redaction.yaml` for Vector/system logs
- [ ] T007 Create `scripts/dev/common.sh` helper library (logging, argument parsing, SSH helpers) for lifecycle scripts to source

**Checkpoint**: Base stack, port mapping, log hygiene, and scripting utilities are ready. Proceed to user stories.

---

## Phase 3: User Story 1 - Secure cloud workspace available (Priority: P1) 🎯 MVP

**Goal**: Developers provision a secure Linode workspace, hydrate secrets, start dependencies remotely, and confirm health via documented commands.

**Independent Test**: Provision workspace with `make remote-provision`, run `make remote-up`, and verify `make remote-status --json` reports all components healthy while MFA tunnel/audit logging remain enforced.

### Implementation for User Story 1

- [ ] T008 [US1] Implement Linode workspace module core in `infra/terraform/modules/dev-workspace/main.tf` (instance, VLAN, tags, StackScript wiring)
- [ ] T009 [P] [US1] Define module variables in `infra/terraform/modules/dev-workspace/variables.tf` (workspace_name, region, ttl, secrets)
- [ ] T010 [P] [US1] Expose required outputs in `infra/terraform/modules/dev-workspace/outputs.tf` (instance ID, private IP, SSH command, labels)
- [ ] T011 [US1] Create bootstrap StackScript at `infra/terraform/modules/dev-workspace/files/bootstrap.sh` (install Docker Compose stack, Vector agent, systemd units)
- [ ] T012 [P] [US1] Author remote override in `.dev/compose/compose.remote.yaml` mounting systemd-managed volumes and remote-specific environment
- [ ] T013 [US1] Wire development environment to the new module in `infra/terraform/environments/development/main.tf`
- [ ] T014 [US1] Implement remote provisioning wrapper in `scripts/dev/remote_provision.sh` (terraform init/plan/apply/destroy + audit logging)
- [ ] T015 [US1] Implement initial remote lifecycle operations (`remote-up`, `remote-status`) in `scripts/dev/remote_lifecycle.sh` using SSH + systemd
- [ ] T016 [US1] Implement Vault-backed secrets hydration in `cmd/secrets-sync/main.go` writing `.env.linode` & `.env.local` with masking
- [ ] T017 [P] [US1] Add Go unit tests in `cmd/secrets-sync/main_test.go` covering AppRole auth, `.gitignore` enforcement, and masking
- [ ] T018 [US1] Implement dependency health probe in `cmd/dev-status/main.go` (remote mode, JSON output, latency metrics)
- [ ] T019 [P] [US1] Add Go unit tests in `cmd/dev-status/main_test.go` with mocked endpoints for success/failure cases
- [ ] T020 [US1] Expose remote command targets in `Makefile` (`remote-provision`, `remote-up`, `remote-status`, `remote-secrets`) tied to scripts/dev helpers
- [ ] T021 [US1] Add Vector agent configuration at `scripts/dev/vector-agent.toml` and reference it from the StackScript
- [ ] T039 [US1] Instrument remote startup/status timing in `cmd/dev-status/main.go` and emit metrics compatible with CI latency checks
- [ ] T040 [P] [US1] Add `scripts/dev/measure_remote_startup.sh` to assert `make remote-up` < 5 minutes and `make remote-status --json` < 10 seconds (wired into CI)
- [ ] T041 [US1] Document data classification & retention policies in `docs/platform/data-classification.md`, covering operational vs analytics artifacts for dev environments
- [ ] T042 [US1] Enforce 90-day remote access log retention and classification tags in `scripts/dev/remote_lifecycle.sh` and `scripts/dev/vector-agent.toml`

**Checkpoint**: Remote workspace flow is operational end-to-end and independently testable.

---

## Phase 4: User Story 2 - Spin up local platform in minutes (Priority: P2)

**Goal**: Developers run the local stack with parity commands, validate health, and iterate without remote dependencies.

**Independent Test**: Execute `make up`, confirm containers start with provided ports, and verify `make status --json` shows all components healthy on a managed laptop.

### Implementation for User Story 2

- [ ] T022 [US2] Implement local override in `.dev/compose/compose.local.yaml` (host volumes, port mappings, resource limits)
- [ ] T023 [P] [US2] Implement local lifecycle wrapper (`up`, `status`, `stop`) in `scripts/dev/local_lifecycle.sh` using Docker Compose v2
- [ ] T024 [US2] Wire local command targets into `Makefile` (`up`, `status`, `logs`, `stop`, `reset`) referencing `scripts/dev/local_lifecycle.sh`
- [ ] T025 [P] [US2] Extend `cmd/dev-status/main.go` to support local mode, reading `.specify/local/ports.yaml` and Compose health endpoints
- [ ] T026 [US2] Add sample data seeding in `dev/data/seed.sql` and invoke it from `scripts/dev/local_lifecycle.sh` on fresh resets
- [ ] T043 [US2] Capture local startup/status telemetry in `cmd/dev-status/main.go` for parity with remote metrics
- [ ] T044 [P] [US2] Add `scripts/dev/measure_local_startup.sh` verifying `make up` < 5 minutes and `make status --json` < 10 seconds (hooked into CI)

**Checkpoint**: Local stack parity is achieved and independently testable.

---

## Phase 5: User Story 3 - Services connect to local dependencies (Priority: P3)

**Goal**: Services consume example configuration, connect to dependencies, and complete a happy-path request in local or remote modes.

**Independent Test**: Use the supplied template to configure a service, run the example script, and verify it completes a mock inference request via local stack.

### Implementation for User Story 3

- [ ] T027 [US3] Create service configuration template at `configs/dev/service-example.env.tpl` mapping secrets bundle values
- [ ] T028 [P] [US3] Add example runner script `scripts/dev/examples/run_sample_service.sh` demonstrating env usage for local & remote modes
- [ ] T029 [US3] Add integration smoke test `tests/dev/service_happy_path.sh` that exercises the sample service against local stack
- [ ] T030 [US3] Document service connection workflow in `docs/runbooks/service-dev-connect.md` (env templates, sample run, troubleshooting)

**Checkpoint**: Service connectivity verified; users can follow documentation to execute primary flows.

---

## Phase 6: User Story 4 - Simple lifecycle management (Priority: P4)

**Goal**: Developers manage resets, logs, and teardown with a concise command set for both remote and local stacks.

**Independent Test**: Trigger `make remote-reset`, `make remote-logs COMPONENT=postgres`, `make stop`, and observe correct behavior with audited logging and cleanup.

### Implementation for User Story 4

- [ ] T031 [US4] Extend `scripts/dev/remote_lifecycle.sh` to support `remote-reset`, `remote-logs`, `remote-stop`, and `remote-destroy` with TTL enforcement and audit events
- [ ] T032 [P] [US4] Extend `scripts/dev/local_lifecycle.sh` with reset/log/diagnose flows, including port-conflict remediation guidance
- [ ] T033 [US4] Enhance `cmd/dev-status/main.go` with `--diagnose` output to surface port conflicts and remote TTL warnings
- [ ] T034 [US4] Update `Makefile` help text and lifecycle command descriptions to reflect the full command suite

**Checkpoint**: Lifecycle management commands operate uniformly across modes.

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Finalize documentation, automation, and repository wiring after user stories are complete.

- [ ] T035 Update `docs/specs/002-local-dev-environment/quickstart.md` with TTL automation, secrets rotation, and lifecycle troubleshooting guidance
- [ ] T036 [P] Add `.github/workflows/dev-environment-ci.yml` running Go fmt/test, Terraform fmt/validate, shell lint for `scripts/dev/*`, and invoking `scripts/dev/measure_remote_startup.sh` + `scripts/dev/measure_local_startup.sh`
- [ ] T037 Add `scripts/dev/validate_quickstart.sh` to automate remote/local smoke checks referenced by documentation
- [ ] T038 Update `llms.txt` with links to the spec, plan, quickstart, contracts, and CI workflow for discoverability

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)** → no prerequisites.
- **Phase 2 (Foundational)** → depends on Phase 1; blocks all story phases.
- **Phase 3 (US1, P1)** → depends on Phase 2; delivers MVP.
- **Phase 4 (US2, P2)** → depends on Phase 2; can start after US1 or in parallel once foundation is stable.
- **Phase 5 (US3, P3)** → depends on Phases 2 & 4 (local stack ready for connectivity tests).
- **Phase 6 (US4, P4)** → depends on Phases 3 & 4 (command suite exists); extends lifecycle coverage.
- **Phase 7 (Polish)** → depends on completion of targeted user stories.

### User Story Dependencies

- **US1** → foundational for all other stories; delivers remote core.
- **US2** → relies on shared assets from foundation; independent of US1 but benefits from shared command helpers.
- **US3** → builds atop US2 (local stack) and shared secrets; can start once US2’s commands are stable.
- **US4** → extends scripts from US1/US2; must follow those implementations.

### Within-Story Flow

- Implement Terraform/Go assets before wiring Makefile targets.
- Write or update tests (`cmd/*_test.go`, `tests/dev/*.sh`) immediately after corresponding implementation tasks.
- Documentation updates occur after commands/scripts are finalized.

---

## Parallel Execution Examples

### User Story 1
- T009 and T010 (module variables/outputs) can run parallel while T008 builds the core module.
- T017 and T019 (Go tests) can run in parallel once `cmd/*` implementations exist.

### User Story 2
- T023 (local lifecycle script) and T025 (dev-status local mode) can proceed in parallel after T022 because they touch different files.

### User Story 3
- T028 (example runner script) and T030 (documentation) can proceed in parallel once the service template (T027) is defined.

### User Story 4
- T032 (local lifecycle extensions) and T033 (diagnostic output) can run concurrently after T031 stabilizes the remote script updates.

---

## Implementation Strategy

### MVP First (User Story 1 Only)
1. Complete Setup (Phase 1) and Foundational (Phase 2).
2. Deliver Phase 3 (US1) end-to-end and validate with `make remote-provision`, `make remote-up`, `make remote-status --json`.
3. Demo remote workspace provisioning to stakeholders before proceeding.

### Incremental Delivery
1. Finish US1 → remote workspace MVP.
2. Add US2 → local stack parity (developers iterate without cloud dependencies).
3. Layer US3 → sample service connectivity to ensure real-world flows.
4. Add US4 → lifecycle convenience and resilience.
5. Apply Polish tasks for CI, docs, and discovery (consider adding a follow-up `make dev-metrics` helper to call measurement scripts locally if teams request it).

### Parallel Team Strategy
- Developer A: Focus on Terraform modules and remote scripts (US1).
- Developer B: Build local lifecycle tooling and Compose overrides (US2).
- Developer C: Create service templates/tests (US3) once local stack is ready.
- Developer D: Extend lifecycle/diagnostics (US4) and finalize CI/documentation in Phase 7.

---

## Summary Metrics

- **Total tasks**: 44
- **Tasks per user story**:
  - US1: 20 tasks
  - US2: 7 tasks
  - US3: 4 tasks
  - US4: 4 tasks
- **Parallel opportunities**: Identified across all stories (see Parallel Execution Examples)
- **Independent test criteria**: Each story section lists the validation steps needed to accept the user story independently
- **Suggested MVP scope**: Complete through Phase 3 (US1) before layering additional stories

---
