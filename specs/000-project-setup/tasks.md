# Tasks: Project Setup & Repository Structure

## Summary

- **Feature**: Project Setup & Repository Structure
- **Spec**: `/specs/000-project-setup/spec.md`
- **Plan**: `/specs/000-project-setup/plan.md`
- **Total Tasks**: 95
- **Parallelizable Tasks**: 30
- **Primary Dependencies**: Root automation tooling, GitHub Actions workflows, Linode API integration, metrics storage

## Dependency Graph

```mermaid
graph TD
    A[Phase 1: Setup] --> B[Phase 2: Foundational]
    B --> C[Phase 3: US-001 (P1)]
    C --> D[Phase 4: US-002 (P2)]
    C --> E[Phase 5: US-003 (P2)]
    D --> F[Phase 6: US-004 (P3)]
    E --> G[Phase 7: US-005 (P3)]
    G --> H[Phase 8: Polish]
```

## Implementation Strategy

1. Complete setup and foundational phases to establish automation scaffolding, shared templates, and Linode integration utilities.
2. Deliver the P1 user story (US-001) to provide immediate onboarding value—this forms the MVP baseline.
3. Implement P2 stories (US-002 and US-003) to ensure consistent build/test commands and local/remote CI parity.
4. Finish P3 stories (US-004 and US-005) to round out automation templates and reusable quality checks.
5. Conclude with polish to validate metrics pipelines, documentation, and Linode alignment.

## Phase 1: Setup (Repository Initialization)

### Goal

Initialize repository tooling, shared configuration, and baseline documentation for new contributors.

### Independent Test Criteria

- Running `make help` displays expected tasks (`build`, `test`, `lint`, `check`, `ci-remote`, `ci-local`, `service-new`) with descriptions.
- `./scripts/setup/bootstrap.sh --check-only` validates prerequisites on macOS/Linux/WSL and exits 0 when compliant.

### Tasks

- [x] T001 Create root Makefile skeleton with help target (`Makefile`)
- [x] T002 Add `.editorconfig` with formatting defaults (`.editorconfig`)
- [x] T003 Create `go.work` referencing placeholder modules (`go.work`)
- [x] T004 Scaffold `scripts/setup/bootstrap.sh` with prerequisite checks (`scripts/setup/bootstrap.sh`)
- [x] T005 Create root documentation skeleton (`README.md`)
- [x] T006 Add contributing guide skeleton (`CONTRIBUTING.md`)
- [x] T007 Configure `.gitignore` baseline (`.gitignore`)
- [x] T008 Configure `.dockerignore` baseline (`.dockerignore`)
- [x] T009 Seed documentation index (`docs/README.md`)
- [x] T010 Seed `services/README.md` describing service layout (`services/README.md`)
- [x] T011 Add `.github/ISSUE_TEMPLATE/config.yml` (`.github/ISSUE_TEMPLATE/config.yml`)
- [x] T012 Add `.github/PULL_REQUEST_TEMPLATE.md` (`.github/PULL_REQUEST_TEMPLATE.md`)

## Phase 2: Foundational (Shared Automation & Templates)

### Goal

Implement shared automation assets, templates, and Linode platform utilities required by all services and CI workflows.

### Independent Test Criteria

- `make help` shows commands populated from Makefile metadata.
- `make ci-local` executes GitHub Actions workflow locally using `act`.
- Linode API helper scripts authenticate via environment variables and tokens.

### Tasks

- [x] T013 Build `scripts/ci/run-local.sh` wrapper for `act` (`scripts/ci/run-local.sh`)
- [x] T014 Build `scripts/ci/trigger-remote.sh` using GitHub CLI (`scripts/ci/trigger-remote.sh`)
- [x] T015 Add shared service Makefile template (`templates/service.mk`)
- [x] T016 Configure lint rules (`configs/golangci.yml`)
- [x] T017 Configure `gosec` policy file (`configs/gosec.toml`)
- [x] T018 Configure markdown linting defaults (`configs/markdownlint.json`)
- [x] T019 Implement metrics collector (`scripts/metrics/collector.go`)
- [x] T020 Implement metrics upload script (`scripts/metrics/upload.sh`)
- [x] T021 Add Linode helper library (`scripts/platform/linode_helpers.sh`)
- [x] T022 Author Linode access guide (`docs/platform/linode-access.md`)
- [x] T023 Configure GitHub Actions `ci.yml` skeleton (`.github/workflows/ci.yml`)
- [x] T024 Configure GitHub Actions `ci-remote.yml` skeleton (`.github/workflows/ci-remote.yml`)
- [x] T025 Add reusable workflow for build/test matrix (`.github/workflows/reusable/build.yml`)
- [x] T026 Create metrics lifecycle policy doc (`docs/metrics/policy.md`)
- [x] T027 Scaffold service generator script (`scripts/service/new.sh`)
- [x] T028 Create quickstart entry for Linode prerequisites (`specs/000-project-setup/quickstart.md`)
- [x] T029 Update llms.txt with foundational links (`llms.txt`)
- [x] T030 Add runbook stubs for CI remote (`docs/runbooks/ci-remote.md`)
- [x] T031 Add runbook stubs for Linode setup (`docs/runbooks/linode-setup.md`)
- [x] T090 Create shared tool version manifest (`configs/tool-versions.mk`)
- [x] T091 Update Makefile/scripts to consume shared version manifest (`Makefile`)
- [x] T092 Annotate templates/workflows with inline guidance comments (`templates/service.mk`)
- [x] T093 Document cross-repo version bump process (`docs/platform/tooling-versions.md`)

## Phase 3: User Story 1 (US-001 – Priority P1) – New Developer Productivity

### Goal

Enable a new contributor to clone the repo, run setup, discover tasks, and begin working without manual configuration.

### Independent Test Criteria

- Fresh clone on compliant machine: `./scripts/setup/bootstrap.sh` completes with success message.
- Running `make help` displays tasks with descriptions and colorized output.
- Quickstart Section 1–2 contains verified copy-paste commands.

### Tasks

- [x] T032 [US1] Implement OS detection & install guidance in bootstrap script (`scripts/setup/bootstrap.sh`)
- [x] T033 [US1] Integrate dynamic descriptions into `make help` (`Makefile`)
- [x] T034 [US1] Document setup workflow in quickstart (Prerequisites & Setup) (`specs/000-project-setup/quickstart.md`)
- [x] T035 [US1] Extend contributing guide with onboarding steps (`CONTRIBUTING.md`)
- [x] T036 [US1] Add troubleshooting doc for setup script (`docs/troubleshooting/bootstrap.md`)
- [x] T037 [US1] Validate setup flow on macOS/Linux/WSL and log results (`docs/troubleshooting/bootstrap.md`)
- [x] T038 [US1] Provide Linode API token instructions referencing docs (`docs/platform/linode-access.md`)
- [x] T039 [US1] Update README with setup summary & links (`README.md`)
- [x] T040 [US1] Ensure llms.txt references quickstart onboarding (`llms.txt`)
- [x] T086 [US1] Instrument `make help` benchmark ensuring <1s response (`tests/perf/measure_help.sh`)
- [x] T087 [US1] Add progress indicators during bootstrap steps (`scripts/setup/bootstrap.sh`)
- [x] T088 [US1] Validate bootstrap rollback/resume scenarios and document outcomes (`docs/troubleshooting/bootstrap.md`)
- [x] T089 [US1] Time full bootstrap run (<10 min) and record results in quickstart (`specs/000-project-setup/quickstart.md`)

## Phase 4: User Story 2 (US-002 – Priority P2) – Consistent Build/Test Commands

### Goal

Provide consistent `make build` and `make test` commands across services with identical output formats.

### Independent Test Criteria

- `make build SERVICE=user-org-service` and `make build SERVICE=api-router-service` succeed with the same output structure.
- `make test` within services yields uniform results and exit codes.
- Service template clarifies extension points and variables.

### Tasks

- [x] T041 [US2] Implement root `build`, `test`, `clean` targets orchestrating services (`Makefile`)
- [x] T042 [US2] Enhance `templates/service.mk` with build/test hooks (`templates/service.mk`)
- [x] T043 [US2] Add sample service README showing usage (`services/README.md`)
- [x] T044 [US2] Document build/test workflow in quickstart Section 3 (`specs/000-project-setup/quickstart.md`)
- [x] T045 [US2] Configure GitHub Actions build matrix referencing services (`.github/workflows/reusable/build.yml`)
- [x] T046 [US2] Add shared environment variable defaults (`configs/build.env.example`)
- [x] T047 [US2] Create docs on customizing service targets (`docs/services/customizing.md`)
- [x] T094 [US2] Benchmark `make build-all` and capture timing evidence (`tests/perf/build_all_benchmark.sh`)

## Phase 5: User Story 3 (US-003 – Priority P2) – Local CI Parity & Remote Execution

### Goal

Allow contributors to run the same quality checks locally and via GitHub Actions, supporting restricted laptops through remote execution.

### Independent Test Criteria

- `make check` (local) and remote `ci` workflow produce the same failures on injected lint issues.
- `make ci-local` runs relevant workflow subset using `act`.
- `make ci-remote` triggers workflow_dispatch, returns results <10 minutes, surfaces run URL.

### Tasks

- [x] T048 [US3] Implement root `check` target chaining format/lint/security (`Makefile`)
- [x] T049 [US3] Integrate `gosec` invocation into `check` (`Makefile`)
- [x] T050 [US3] Implement `make ci-local` target calling runner script (`Makefile`)
- [x] T051 [US3] Implement `make ci-remote` target calling trigger script (`Makefile`)
- [x] T052 [US3] Expand `scripts/ci/run-local.sh` with container selection & caching (`scripts/ci/run-local.sh`)
- [x] T053 [US3] Expand `scripts/ci/trigger-remote.sh` with gh CLI error handling (`scripts/ci/trigger-remote.sh`)
- [x] T054 [US3] Configure metrics publishing step within `ci.yml` (`.github/workflows/ci.yml`)
- [x] T055 [US3] Ensure `ci-remote.yml` reuses same jobs (`.github/workflows/ci-remote.yml`)
- [x] T056 [US3] Document remote CI flow in quickstart Section 3 (`specs/000-project-setup/quickstart.md`)
- [x] T057 [US3] Document troubleshooting for local/remote CI parity (`docs/troubleshooting/ci.md`)
- [x] T058 [US3] Verify metrics collector integrates with workflow outputs (`scripts/metrics/collector.go`)
- [x] T059 [US3] Create sample GitHub CLI usage doc (`docs/platform/ci-remote-cli.md`)
- [x] T060 [US3] Update llms.txt with remote CI link (`llms.txt`)

## Phase 6: User Story 4 (US-004 – Priority P3) – Service Automation Template

### Goal

Allow developers to create a new service quickly using shared automation templates with minimal customization.

### Independent Test Criteria

- `make service-new NAME=billing-service` creates service skeleton referencing shared template.
- Running `make build` and `make check` inside new service succeeds without manual edits.
- Quickstart contains documented instructions for adding new services.

### Tasks

- [x] T061 [US4] Finalize service generator script to update go.work and directories (`scripts/service/new.sh`)
- [x] T062 [US4] Implement `make service-new` target invoking script (`Makefile`)
- [x] T063 [US4] Create service template directory `_template/` with README and Makefile (`services/_template/Makefile`)
- [x] T064 [US4] Document new service workflow in quickstart Section 5 (`specs/000-project-setup/quickstart.md`)
- [x] T065 [US4] Add automated test verifying generator output tidy (`scripts/service/test_new_service.sh`)
- [x] T066 [US4] Update llms.txt referencing service automation resources (`llms.txt`)
- [x] T067 [US4] Provide checklist for adopting templates in new service specs (`docs/services/checklist.md`)

## Phase 7: User Story 5 (US-005 – Priority P3) – Reusable Quality Checks

### Goal

Provide standardized format, lint, and security checks with clear pass/fail output and remediation guidance.

### Independent Test Criteria

- Formatting issues cause `make check` to fail with fix guidance (`make fmt`).
- Clean repository passes `make check` in under 3 minutes.
- Security scan results include severity with remediation hints and metrics capture.

### Tasks

- [x] T068 [US5] Wire `gofmt`/`goimports` into check target (`Makefile`)
- [x] T069 [US5] Add `make fmt` auto-fix target (`Makefile`)
- [x] T070 [US5] Finalize `golangci-lint` configuration (`configs/golangci.yml`)
- [x] T071 [US5] Finalize `gosec` configuration and severity mapping (`configs/gosec.toml`)
- [x] T072 [US5] Add documentation for `make check` workflow (`specs/000-project-setup/quickstart.md`)
- [x] T073 [US5] Add troubleshooting guide for check failures (`docs/troubleshooting/check.md`)
- [x] T074 [US5] Add failing fixture to validate lint/test detection (`tests/check/fixtures/bad_format.go`)
- [x] T075 [US5] Ensure metrics collector captures check results with status metadata (`scripts/metrics/collector.go`)
- [x] T095 [US5] Audit check-related templates for inline remediation comments (`configs/golangci.yml`)

## Phase 8: Polish & Cross-Cutting Concerns

### Goal

Finalize observability, documentation, metrics validation, and Linode integration alignment.

### Independent Test Criteria

- `make ci-remote` smoke test completes under 10 minutes and uploads metrics to Linode object storage.
- Metrics artifacts discoverable via documented CLI commands.
- Quickstart, llms.txt, and runbooks reflect final automation instructions.

### Tasks

- [x] T076 Validate metrics upload via MinIO/AWS CLI with recorded steps (`docs/metrics/README.md`)
- [x] T077 Publish sample metrics report demonstrating JSON schema (`docs/metrics/report.md`)
- [x] T078 Run end-to-end smoke test (bootstrap → check → ci-remote) and document results (`docs/runbooks/ci-remote.md`)
- [x] T079 Review quickstart, llms.txt, README for final accuracy (`specs/000-project-setup/quickstart.md`)
- [x] T080 Cross-reference constitution gates in plan/tasks (`specs/000-project-setup/plan.md`)
- [x] T081 Create automation checklist for downstream features (`specs/000-project-setup/checklists/automation.md`)
- [x] T082 Verify Linode API documentation links across project (`README.md`)
- [x] T083 Add status badges to README (CI, docs, metrics) (`README.md`)
- [x] T084 Ensure docs include reference to llms.txt standard (`docs/README.md`)
- [x] T085 Prepare release notes template for automation updates (`docs/release-notes/template.md`)

## Parallel Task Opportunities

- Early setup tasks (T002–T008) operate on distinct files and can run in parallel.
- Foundational scripts/templates (T013–T027) largely independent once scaffolding exists; mark `[P]` when scheduling.
- Within user stories, documentation vs script updates can happen concurrently as long as file overlap avoided.
- Phases 4 and 5 depend on Phase 3 completion but contain sub-tasks on separate files enabling parallel execution.

## MVP Scope

- **MVP**: Deliver Phases 1–3 (Setup, Foundational, US-001). This ensures new contributors can onboard, run documented commands, and operate within the shared automation layer.
- Subsequent phases extend capability (US-002 & US-003), provide templates (US-004), enforce quality checks (US-005), and polish documentation/metrics for long-term maintainability.

