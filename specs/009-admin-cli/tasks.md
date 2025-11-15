# Tasks: Admin CLI

**Input**: Design documents from `/specs/009-admin-cli/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), quickstart.md

**Tests**: Unit tests and integration tests are included as implementation progresses. Tests should be written to verify each user story independently.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Task Naming Convention

**Format**: `T-S009-P{phase_number}-{task_number}`

- **Spec Number**: `009` for admin-cli
- **Phase Number**: Two-digit phase number (e.g., `01` for Phase 1, `02` for Phase 2)
- **Task Number**: Three-digit sequential task number within the phase

**Examples**:
- Spec 009, Phase 1, Task 1: `T-S009-P01-001`
- Spec 009, Phase 1, Task 6: `T-S009-P01-006`
- Spec 009, Phase 2, Task 7: `T-S009-P02-007` (continues sequence from Phase 1)

**Important**: Task numbers continue sequentially across phases within the spec.

## Format: `[ID] [P?] [Story] Description`

- **[ID]**: Task ID following the naming convention above
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Create project structure and initialize Go module

- [ ] T-S009-P01-001 Create project structure per implementation plan in services/admin-cli/
- [ ] T-S009-P01-002 Initialize Go module in services/admin-cli/go.mod with module name
- [ ] T-S009-P01-003 [P] Add Cobra dependency (v1.7+) to services/admin-cli/go.mod
- [ ] T-S009-P01-004 [P] Add Viper dependency for configuration management to services/admin-cli/go.mod
- [ ] T-S009-P01-005 [P] Create Makefile in services/admin-cli/ with build/test/lint targets
- [ ] T-S009-P01-006 [P] Create README.md in services/admin-cli/ with service documentation
- [ ] T-S009-P01-007 Create cmd/admin-cli/main.go as CLI entrypoint

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T-S009-P02-008 Implement configuration management in services/admin-cli/internal/config/config.go with Viper support
- [ ] T-S009-P02-009 [P] Create configuration defaults in services/admin-cli/internal/config/defaults.go
- [ ] T-S009-P02-010 [P] Implement root command setup in services/admin-cli/cmd/admin-cli/main.go with Cobra
- [ ] T-S009-P02-011 [P] Implement health check functionality in services/admin-cli/internal/health/checker.go for service dependencies
- [ ] T-S009-P02-012 [P] Implement output formatting foundation in services/admin-cli/internal/output/table.go
- [ ] T-S009-P02-013 [P] Implement JSON output formatter in services/admin-cli/internal/output/json.go
- [ ] T-S009-P02-014 [P] Implement CSV output formatter in services/admin-cli/internal/output/csv.go
- [ ] T-S009-P02-015 [P] Implement audit logging in services/admin-cli/internal/audit/logger.go
- [ ] T-S009-P02-016 [P] Implement progress indicators in services/admin-cli/internal/progress/indicator.go
- [ ] T-S009-P02-017 [P] Create user-org-service API client package structure in services/admin-cli/internal/client/userorg/ with placeholder files: client.go (empty), types.go (empty), auth.go (empty)
- [ ] T-S009-P02-018 [P] Create analytics-service API client package structure in services/admin-cli/internal/client/analytics/ with placeholder files: client.go (empty), types.go (empty)
- [ ] T-S009-P02-019 Implement error handling utilities with structured error types and recovery suggestions in services/admin-cli/internal/errors/
- [ ] T-S009-P02-020 Implement retry logic with exponential backoff in services/admin-cli/internal/client/retry.go

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: User Story 1 - Bootstrap and break-glass operations (Priority: P1) 🎯 MVP

**Goal**: Enable platform initialization and break-glass recovery via CLI with dry-run and confirmation support

**Independent Test**: Run bootstrap and recovery flows in a test environment using dry-run and live modes. Verify platform initialization and break-glass recovery work independently without day-2 management (US-002) or reporting (US-003).

### Implementation for User Story 1

- [ ] T-S009-P03-021 [US1] Implement user-org-service API client types in services/admin-cli/internal/client/userorg/types.go
- [ ] T-S009-P03-022 [US1] Implement user-org-service REST client in services/admin-cli/internal/client/userorg/client.go with authentication
- [ ] T-S009-P03-023 [US1] Implement bootstrap command in services/admin-cli/internal/commands/bootstrap.go with dry-run support
- [ ] T-S009-P03-024 [US1] Add confirmation prompts and risk warnings to bootstrap command in services/admin-cli/internal/commands/bootstrap.go
- [ ] T-S009-P03-025 [US1] Implement secure credential output (masked in logs) in services/admin-cli/internal/commands/bootstrap.go
- [ ] T-S009-P03-026 [US1] Add bootstrap command to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P03-027 [US1] Implement existing admin detection and overwrite confirmation in services/admin-cli/internal/commands/bootstrap.go
- [ ] T-S009-P03-028 [US1] Implement credential rotation command in services/admin-cli/internal/commands/credentials.go
- [ ] T-S009-P03-029 [US1] Implement break-glass recovery command in services/admin-cli/internal/commands/credentials.go with explicit authentication flags
- [ ] T-S009-P03-030 [US1] Add credentials rotate command to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P03-031 [US1] Implement service health check before bootstrap operations in services/admin-cli/internal/commands/bootstrap.go
- [ ] T-S009-P03-032 [US1] Add audit logging for bootstrap operations in services/admin-cli/internal/commands/bootstrap.go
- [ ] T-S009-P03-033 [US1] Add audit logging for credential rotation operations in services/admin-cli/internal/commands/credentials.go
- [ ] T-S009-P03-034 [US1] Implement error handling with clear messages for service unavailability in services/admin-cli/internal/commands/bootstrap.go
- [ ] T-S009-P03-035 [US1] Write unit tests for bootstrap command in services/admin-cli/internal/commands/bootstrap_test.go
- [ ] T-S009-P03-036 [US1] Write unit tests for credential rotation in services/admin-cli/internal/commands/credentials_test.go
- [ ] T-S009-P03-037 [US1] Write integration tests for bootstrap flow in services/admin-cli/test/integration/bootstrap_test.go

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently. Bootstrap and break-glass recovery work without US-002 or US-003.

---

## Phase 4: User Story 2 - Day-2 management at speed (Priority: P2)

**Goal**: Enable org/user/key lifecycle management and sync triggers via scripts with predictable output and exit codes

**Independent Test**: Integrate CLI commands into scripts and verify idempotent outcomes and consistent exit codes. Works with existing platform state and delivers value even without bootstrap (US-001).

### Implementation for User Story 2

- [ ] T-S009-P04-038 [US2] Implement org list command in services/admin-cli/internal/commands/org.go
- [ ] T-S009-P04-039 [US2] Implement org create command in services/admin-cli/internal/commands/org.go with dry-run support
- [ ] T-S009-P04-040 [US2] Implement org update command in services/admin-cli/internal/commands/org.go with file input (JSON/YAML) support
- [ ] T-S009-P04-041 [US2] Implement org delete command in services/admin-cli/internal/commands/org.go with confirmation and force flags
- [ ] T-S009-P04-042 [US2] Add org subcommands to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P04-043 [US2] Implement user list command in services/admin-cli/internal/commands/user.go
- [ ] T-S009-P04-044 [US2] Implement user create command in services/admin-cli/internal/commands/user.go with idempotent support (--upsert flag)
- [ ] T-S009-P04-045 [US2] Implement user update command in services/admin-cli/internal/commands/user.go
- [ ] T-S009-P04-046 [US2] Implement user delete command in services/admin-cli/internal/commands/user.go
- [ ] T-S009-P04-047 [US2] Add user subcommands to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P04-048 [US2] Implement API key lifecycle commands in services/admin-cli/internal/commands/apikey.go (list, create, delete)
- [ ] T-S009-P04-049 [US2] Add apikey subcommands to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P04-050 [US2] Implement batch operation file parsing (JSON/YAML) in services/admin-cli/internal/commands/batch.go
- [ ] T-S009-P04-051 [US2] Implement dry-run preview with diff output and summary counts in services/admin-cli/internal/commands/batch.go
- [ ] T-S009-P04-052 [US2] Implement checkpoint/resume capability for batch operations in services/admin-cli/internal/commands/batch.go
- [ ] T-S009-P04-053 [US2] Implement partial failure handling with detailed error reporting in services/admin-cli/internal/commands/batch.go
- [ ] T-S009-P04-054 [US2] Implement --continue-on-error flag for resilient batch processing in services/admin-cli/internal/commands/batch.go
- [ ] T-S009-P04-055 [US2] Implement sync trigger command in services/admin-cli/internal/commands/sync.go
- [ ] T-S009-P04-056 [US2] Implement sync status command with job ID monitoring in services/admin-cli/internal/commands/sync.go
- [ ] T-S009-P04-057 [US2] Implement progress indicators for long-running operations in services/admin-cli/internal/commands/sync.go
- [ ] T-S009-P04-058 [US2] Add sync subcommands to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P04-059 [US2] Implement structured output (JSON format) for all org/user commands in services/admin-cli/internal/output/json.go
- [ ] T-S009-P04-060 [US2] Implement predictable exit codes (0=success, 1=error, 2=usage, 3=service unavailable) in services/admin-cli/internal/commands/
- [ ] T-S009-P04-061 [US2] Implement non-interactive mode (all prompts via flags) for CI/CD integration in services/admin-cli/internal/commands/
- [ ] T-S009-P04-062 [US2] Add audit logging for all org/user/key lifecycle operations in services/admin-cli/internal/commands/
- [ ] T-S009-P04-063 [US2] Write unit tests for org commands in services/admin-cli/internal/commands/org_test.go
- [ ] T-S009-P04-064 [US2] Write unit tests for user commands in services/admin-cli/internal/commands/user_test.go
- [ ] T-S009-P04-065 [US2] Write unit tests for batch operations in services/admin-cli/internal/commands/batch_test.go
- [ ] T-S009-P04-066 [US2] Write integration tests for org/user lifecycle flows in services/admin-cli/test/integration/org_user_test.go
- [ ] T-S009-P04-067 [US2] Write integration tests for batch operations in services/admin-cli/test/integration/batch_test.go

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently. Day-2 management is functional with scriptable operations.

---

## Phase 5: User Story 3 - Exports and reporting (Priority: P3)

**Goal**: Enable usage and membership report exports from CLI for reconciliation and audits

**Independent Test**: Run exports and verify column definitions and totals match system-of-record data. Provides reporting capabilities independently even if only analytics-service APIs are available.

### Implementation for User Story 3

- [ ] T-S009-P05-068 [US3] Implement analytics-service API client types in services/admin-cli/internal/client/analytics/types.go
- [ ] T-S009-P05-069 [US3] Implement analytics-service REST client in services/admin-cli/internal/client/analytics/client.go
- [ ] T-S009-P05-070 [US3] Implement export usage command in services/admin-cli/internal/commands/export.go with time range support
- [ ] T-S009-P05-071 [US3] Implement export memberships command in services/admin-cli/internal/commands/export.go with changes-only option
- [ ] T-S009-P05-072 [US3] Implement CSV export formatting with column headers and schema comments in services/admin-cli/internal/output/csv.go
- [ ] T-S009-P05-073 [US3] Implement reconciliation verification against service counters in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-074 [US3] Implement streaming output for large datasets (>10k rows) in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-075 [US3] Implement progress indicators for large exports (>30 seconds) in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-076 [US3] Implement cancellation handling (Ctrl+C) with partial file cleanup in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-077 [US3] Implement file permission setting (0600) for sensitive export files in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-078 [US3] Implement compression support (--compress=gzip) for large exports in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-079 [US3] Add export subcommands to root command in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P05-080 [US3] Add audit logging for export operations in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-081 [US3] Implement error handling with retry suggestions for service unavailability in services/admin-cli/internal/commands/export.go
- [ ] T-S009-P05-082 [US3] Write unit tests for export commands in services/admin-cli/internal/commands/export_test.go
- [ ] T-S009-P05-083 [US3] Write integration tests for export flows in services/admin-cli/test/integration/export_test.go
- [ ] T-S009-P05-084 [US3] Write integration tests for reconciliation verification in services/admin-cli/test/integration/reconciliation_test.go

**Checkpoint**: All user stories should now be independently functional. Export capabilities are complete with reconciliation support.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and final polish

- [ ] T-S009-P06-085 [P] Implement help system with --help flags and comprehensive examples in services/admin-cli/cmd/admin-cli/main.go
- [ ] T-S009-P06-086 [P] Generate command completion (bash/zsh) in services/admin-cli/internal/completion/
- [ ] T-S009-P06-087 [P] Add --verbose and --quiet flags support across all commands in services/admin-cli/internal/commands/
- [ ] T-S009-P06-088 [P] Implement configuration file support (YAML/JSON) with flag overrides in services/admin-cli/internal/config/config.go
- [ ] T-S009-P06-089 [P] Implement credential masking in all log outputs in services/admin-cli/internal/audit/logger.go
- [ ] T-S009-P06-090 [P] Implement token validation with clock skew tolerance (±5 minutes) in services/admin-cli/internal/client/userorg/auth.go
- [ ] T-S009-P06-091 [P] Add operation duration tracking to audit logs in services/admin-cli/internal/audit/logger.go
- [ ] T-S009-P06-092 [P] Implement structured JSON log output option for log aggregation in services/admin-cli/internal/logging/
- [ ] T-S009-P06-093 [P] Add progress events for monitoring systems in services/admin-cli/internal/progress/indicator.go
- [ ] T-S009-P06-094 [P] Update README.md in services/admin-cli/ with complete usage documentation
- [ ] T-S009-P06-095 [P] Add example usage scripts in services/admin-cli/examples/
- [ ] T-S009-P06-096 [P] Performance benchmarks for bootstrap, operations, batch, and exports in services/admin-cli/test/perf/
- [ ] T-S009-P06-097 [P] Performance verification test for bootstrap ≤2 minutes (NFR-001) in services/admin-cli/test/perf/bootstrap_perf_test.go
- [ ] T-S009-P06-098 [P] Performance verification test for single org/user operations ≤5 seconds (NFR-002) in services/admin-cli/test/perf/operations_perf_test.go
- [ ] T-S009-P06-099 [P] Performance verification test for batch operations (100 items ≤2 minutes) (NFR-003) in services/admin-cli/test/perf/batch_perf_test.go
- [ ] T-S009-P06-100 [P] Performance verification test for exports (≤1 minute per 10k rows) (NFR-004) in services/admin-cli/test/perf/export_perf_test.go
- [ ] T-S009-P06-101 [P] Performance verification test for help system (<1 second response) (NFR-015) in services/admin-cli/test/perf/help_perf_test.go
- [ ] T-S009-P06-102 [P] Integration test to verify progress events are emitted correctly for monitoring systems (NFR-027) in services/admin-cli/test/integration/progress_events_test.go
- [ ] T-S009-P06-103 [P] Code cleanup and refactoring across all packages
- [ ] T-S009-P06-104 [P] Run quickstart.md validation against implemented features
- [ ] T-S009-P06-105 [P] Update quickstart.md with any discovered improvements in specs/009-admin-cli/quickstart.md
- [ ] T-S009-P06-106 [P] Security review and hardening for all privileged operations
- [ ] T-S009-P06-107 [P] Usability testing plan for command completion (80% error reduction target per NFR-016) in services/admin-cli/docs/usability-testing.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3)
- **Polish (Final Phase)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Works with existing platform state, independent of US1
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Independent of US1/US2, uses analytics-service APIs

### Within Each User Story

- API client setup before commands
- Commands before integration tests
- Core implementation before polish
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel (T-S009-P01-003 through T-S009-P01-006)
- All Foundational tasks marked [P] can run in parallel (T-S009-P02-009 through T-S009-P02-018)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- Models/client types within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members
- All Polish tasks marked [P] can run in parallel

---

## Parallel Example: User Story 1

```bash
# Launch all foundational API client setup together:
Task: "Implement user-org-service API client types in services/admin-cli/internal/client/userorg/types.go"
Task: "Implement user-org-service REST client in services/admin-cli/internal/client/userorg/client.go"

# Launch command implementation after types are ready:
Task: "Implement bootstrap command in services/admin-cli/internal/commands/bootstrap.go"
Task: "Implement credential rotation command in services/admin-cli/internal/commands/credentials.go"
```

---

## Parallel Example: User Story 2

```bash
# Launch all command implementations together (after dependencies):
Task: "Implement org list command in services/admin-cli/internal/commands/org.go"
Task: "Implement user list command in services/admin-cli/internal/commands/user.go"
Task: "Implement API key lifecycle commands in services/admin-cli/internal/commands/apikey.go"
Task: "Implement sync trigger command in services/admin-cli/internal/commands/sync.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (bootstrap and break-glass recovery)
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 3 → Test independently → Deploy/Demo
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (bootstrap/break-glass)
   - Developer B: User Story 2 (day-2 management)
   - Developer C: User Story 3 (exports)
3. Stories complete and integrate independently

---

## Summary

**Total Task Count**: 107 tasks

**Task Count Per User Story**:
- Phase 1 (Setup): 7 tasks
- Phase 2 (Foundational): 13 tasks
- Phase 3 (User Story 1): 17 tasks
- Phase 4 (User Story 2): 30 tasks
- Phase 5 (User Story 3): 17 tasks
- Phase 6 (Polish): 23 tasks (includes performance verification and testing tasks)

**Parallel Opportunities Identified**:
- Phase 1: 4 parallel tasks (T-S009-P01-003 through T-S009-P01-006)
- Phase 2: 10 parallel tasks (T-S009-P02-009 through T-S009-P02-018)
- Phase 6: All 23 tasks can run in parallel (including new performance verification tasks)

**Independent Test Criteria for Each Story**:
- **US-001**: Run bootstrap and recovery flows in test environment with dry-run and live modes
- **US-002**: Integrate CLI commands into scripts, verify idempotent outcomes and consistent exit codes
- **US-003**: Run exports and verify column definitions and totals match system-of-record data

**Suggested MVP Scope**: Phase 1 (Setup) + Phase 2 (Foundational) + Phase 3 (User Story 1 - Bootstrap and break-glass operations)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence

