# Tasks: End-to-End Tests

**Input**: Design documents from `/specs/012-e2e-tests/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `quickstart.md`

**Tests**: E2E test suites are the primary deliverable. Unit tests for test harness components are included to ensure reliability.

**Organization**: Tasks are grouped by phase to enable independent implementation and testing.

## Task Naming Convention

**Format**: `T-S{spec_number}-P{phase_number}-{task_number}`

- **Spec Number**: `012` for `012-e2e-tests`
- **Phase Number**: Two-digit phase number (e.g., `01` for Phase 1, `02` for Phase 2)
- **Task Number**: Three-digit sequential task number within the phase (continues across phases)

**Examples**:
- Spec 012, Phase 1, Task 1: `T-S012-P01-001`
- Spec 012, Phase 1, Task 6: `T-S012-P01-006`
- Spec 012, Phase 2, Task 7: `T-S012-P02-007` (continues sequence from Phase 1)

## Format: `[ID] [P?] [Story] Description`

- **[ID]**: Task ID following the naming convention above
- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

---

## Phase 1: Setup (Project Initialization)

**Purpose**: Create project structure and initialize test harness foundation.

- [x] T-S012-P01-001 Create test directory structure (`tests/e2e/`, `tests/e2e/harness/`, `tests/e2e/suites/`, `tests/e2e/fixtures/`, `tests/e2e/utils/`) per implementation plan
- [x] T-S012-P01-002 Initialize Go module for e2e tests (`tests/e2e/go.mod`) with dependencies (`net/http`, `encoding/json`, `testing`)
- [x] T-S012-P01-003 [P] Create Makefile for test execution (`tests/e2e/Makefile`) with targets for `test-local`, `test-dev`, `test-ci`
- [x] T-S012-P01-004 [P] Create README for e2e tests (`tests/e2e/README.md`) with overview, prerequisites, and quick start
- [x] T-S012-P01-005 [P] Configure `.gitignore` entries for test artifacts (`tests/e2e/.gitignore`) excluding `artifacts/`, `*.log`, `test-results.*`

**Checkpoint**: Project structure created, Go module initialized, basic tooling configured.

---

## Phase 2: Foundational (Test Harness Core)

**Purpose**: Implement core test harness infrastructure that all user stories depend on.

**⚠️ CRITICAL**: Complete this phase before starting user story work.

### Test Harness Infrastructure

- [x] T-S012-P02-006 Implement test HTTP client wrapper (`tests/e2e/harness/client.go`) with request/response logging, timeout handling, and error wrapping
- [x] T-S012-P02-007 [P] Implement test context (`tests/e2e/harness/context.go`) with test run ID, environment config, client instances, and artifact collectors
- [x] T-S012-P02-008 [P] Implement test configuration loading (`tests/e2e/harness/config.go`) with environment variable support and YAML config file parsing
- [x] T-S012-P02-009 Implement basic console reporting (`tests/e2e/harness/reporting.go`) with test case status, duration, and pass/fail output

### Fixture Management

- [x] T-S012-P02-010 Implement fixture manager (`tests/e2e/harness/fixtures.go`) with create, track, and cleanup operations
- [x] T-S012-P02-011 [P] Implement organization fixture (`tests/e2e/fixtures/organizations.go`) with creation, tagging, and cleanup methods
- [x] T-S012-P02-012 [P] Implement user fixture (`tests/e2e/fixtures/users.go`) with creation, invite flow, and cleanup methods
- [x] T-S012-P02-013 [P] Implement API key fixture (`tests/e2e/fixtures/api_keys.go`) with issuance, validation, and cleanup methods
- [x] T-S012-P02-014 Implement budget fixture (`tests/e2e/fixtures/budgets.go`) with creation, limit setting, and usage tracking
- [x] T-S012-P02-015 Implement automatic cleanup (`tests/e2e/harness/fixtures.go`) with test run ID tagging and delayed cleanup support

### Test Utilities

- [x] T-S012-P02-016 [P] Implement retry utilities (`tests/e2e/utils/retry.go`) with exponential backoff and max attempts
- [x] T-S012-P02-017 [P] Implement wait utilities (`tests/e2e/utils/wait.go`) with deterministic waits and timeout handling
- [x] T-S012-P02-018 [P] Implement correlation ID utilities (`tests/e2e/utils/correlation.go`) with generation, extraction, and propagation helpers
- [x] T-S012-P02-027 Implement log masking for sensitive values (`tests/e2e/harness/client.go`) masking API keys, tokens, and passwords in logs and artifacts (security requirement)

### Advanced Features (Foundational)

- [x] T-S012-P02-019 Implement parallel execution support (`tests/e2e/harness/context.go`) with worker ID generation and namespace isolation
- [x] T-S012-P02-020 Implement test isolation mechanisms (`tests/e2e/harness/fixtures.go`) with unique namespace prefixes and worker-specific resources
- [x] T-S012-P02-021 Implement configurable timeouts (`tests/e2e/harness/config.go`) with environment-specific timeout values
- [x] T-S012-P02-022 Implement retry logic with exponential backoff (`tests/e2e/utils/retry.go`) integrated into test client

### Artifact Collection (Foundational)

- [x] T-S012-P02-023 Implement artifact collection (`tests/e2e/harness/artifacts.go`) with request/response body capture and file storage
- [x] T-S012-P02-024 [P] Implement JUnit XML report generation (`tests/e2e/harness/reporting.go`) with test case results and failure details
- [x] T-S012-P02-025 [P] Implement JSON report generation (`tests/e2e/harness/reporting.go`) with summary statistics and test case outcomes
- [x] T-S012-P02-026 Implement correlation ID tracking (`tests/e2e/harness/artifacts.go`) with request/response correlation and audit log linking
- [x] T-S012-P02-028 [P] Implement mock/test double framework (`tests/e2e/utils/mocks.go`) for external dependencies (model backends, payment systems) with configurable mock responses and failure scenarios

**Checkpoint**: Test harness core complete, fixtures available, utilities ready, artifact collection working, security hardened. User story implementation can now begin.

---

## Phase 3: User Story 1 - Validate Critical Happy Paths (Priority: P1) 🎯 MVP

**Goal**: As a maintainer, I can run tests that exercise login, org/user management, API key issuance, routing to models, and successful completions.

**Independent Test**: Execute the suite and verify all happy-path steps pass and produce expected artifacts.

### E2E Tests for User Story 1

- [x] T-S012-P03-027 [P] [US1] Implement organization lifecycle test (`tests/e2e/suites/happy_path_test.go`) creating org, verifying creation, and cleanup
- [x] T-S012-P03-028 [US1] Implement user invite flow test (`tests/e2e/suites/happy_path_test.go`) creating user, sending invite, accepting invite, and verifying status
- [x] T-S012-P03-029 [US1] Implement API key issuance test (`tests/e2e/suites/happy_path_test.go`) creating API key, verifying key works, and testing authentication
- [x] T-S012-P03-030 [US1] Implement model request routing test (`tests/e2e/suites/happy_path_test.go`) submitting inference request, verifying routing, and receiving response
- [x] T-S012-P03-031 [US1] Implement successful completion test (`tests/e2e/suites/happy_path_test.go`) end-to-end flow from org creation through model completion with artifact verification
- [x] T-S012-P03-032 [US1] Add test validation for artifact collection (`tests/e2e/suites/happy_path_test.go`) verifying request/response bodies and correlation IDs are captured

**Checkpoint**: Happy path test suite complete and independently executable. MVP deliverable ready.

---

## Phase 4: User Story 2 - Enforce Budgets and Authorization (Priority: P2)

**Goal**: As a maintainer, I can verify that budget limits and role permissions are enforced with clear denials and audit logs.

**Independent Test**: Exceed budget thresholds and attempt forbidden actions in tests, verifying denials and audit events.

### E2E Tests for User Story 2

- [ ] T-S012-P04-033 [P] [US2] Implement budget limit enforcement test (`tests/e2e/suites/budget_test.go`) setting budget limit, exceeding limit, and verifying denial
- [ ] T-S012-P04-034 [US2] Implement budget exceeded denial test (`tests/e2e/suites/budget_test.go`) submitting request after limit, verifying 403 response, and checking error message
- [ ] T-S012-P04-035 [P] [US2] Implement authorization denial test (`tests/e2e/suites/auth_test.go`) attempting restricted action with insufficient role, verifying 403 response
- [ ] T-S012-P04-036 [US2] Implement audit log verification for denials (`tests/e2e/suites/budget_test.go`) querying audit logs and verifying denial events are recorded
- [ ] T-S012-P04-037 [US2] Implement budget reset and recovery test (`tests/e2e/suites/budget_test.go`) resetting budget, submitting requests, and verifying success
- [ ] T-S012-P04-038 [US2] Implement misconfiguration detection test (`tests/e2e/suites/budget_test.go`) detecting disabled budget enforcement and failing with warning

**Checkpoint**: Budget and authorization tests complete, independently executable, verify enforcement and audit logging.

---

## Phase 5: User Story 3 - Declarative Convergence (Priority: P3)

**Goal**: As a maintainer, I can validate that declarative (Git-as-source-of-truth) changes are applied and drift is detected/reported.

**Independent Test**: Apply a declarative change and verify convergence and drift reporting in tests.

### E2E Tests for User Story 3

- [ ] T-S012-P05-039 [P] [US3] Implement declarative change application test (`tests/e2e/suites/declarative_test.go`) applying Git change, triggering reconciliation, and verifying state
- [ ] T-S012-P05-040 [US3] Implement drift detection test (`tests/e2e/suites/declarative_test.go`) creating intentional divergence, running reconciliation, and verifying drift reporting
- [ ] T-S012-P05-041 [US3] Implement reconciliation status test (`tests/e2e/suites/declarative_test.go`) querying reconciliation status, waiting for completion, and verifying "synced" state
- [ ] T-S012-P05-042 [US3] Implement reconciliation failure test (`tests/e2e/suites/declarative_test.go`) simulating reconciliation failure, querying status, and verifying error reporting
- [ ] T-S012-P05-043 [US3] Add timeout handling for reconciliation (`tests/e2e/suites/declarative_test.go`) with configurable timeout and clear failure messages

**Checkpoint**: Declarative convergence tests complete, independently executable, verify Git-as-source-of-truth workflows.

---

## Phase 6: User Story 4 - Audit Trail Validation (Priority: P2)

**Goal**: As a maintainer, I can verify that all critical operations generate audit events with proper correlation IDs and metadata.

**Independent Test**: Perform operations and query audit logs to verify events are recorded with expected fields.

### E2E Tests for User Story 4

- [ ] T-S012-P06-044 [P] [US4] Implement audit event verification test (`tests/e2e/suites/audit_test.go`) performing API request, querying audit logs, and verifying event fields
- [ ] T-S012-P06-045 [US4] Implement correlation ID validation test (`tests/e2e/suites/audit_test.go`) submitting request with correlation ID, querying logs, and verifying correlation across services
- [ ] T-S012-P06-046 [US4] Implement audit log query test (`tests/e2e/suites/audit_test.go`) querying audit logs by request ID, actor, action, and verifying results
- [ ] T-S012-P06-047 [US4] Implement denial event verification test (`tests/e2e/suites/audit_test.go`) attempting unauthorized action, querying logs, and verifying denial event
- [ ] T-S012-P06-048 [US4] Implement audit log unavailability handling (`tests/e2e/suites/audit_test.go`) handling temporary unavailability gracefully with clear error reporting

**Checkpoint**: Audit trail tests complete, independently executable, verify audit logging and correlation ID tracking.

---

## Phase 7: User Story 5 - Service Health and Resilience (Priority: P2)

**Goal**: As a maintainer, I can verify that services handle failures gracefully and maintain availability during partial outages.

**Independent Test**: Simulate service failures and verify graceful degradation and error handling.

### E2E Tests for User Story 5

- [ ] T-S012-P07-049 [P] [US5] Implement backend failover test (`tests/e2e/suites/resilience_test.go`) simulating backend unavailability, submitting request, and verifying failover or error message
- [ ] T-S012-P07-050 [US5] Implement health check test (`tests/e2e/suites/resilience_test.go`) simulating database failure, checking health endpoint, and verifying unhealthy status
- [ ] T-S012-P07-051 [US5] Implement partial outage test (`tests/e2e/suites/resilience_test.go`) simulating partial service outage, running tests, and verifying graceful degradation
- [ ] T-S012-P07-052 [US5] Implement all backends unavailable test (`tests/e2e/suites/resilience_test.go`) detecting condition and skipping with clear messaging
- [ ] T-S012-P07-053 [US5] Integrate mock/test double framework for resilience tests (`tests/e2e/suites/resilience_test.go`) using mock framework from Phase 2 for backend unavailability scenarios

**Checkpoint**: Resilience tests complete, independently executable, verify failure handling and graceful degradation.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: CI/CD integration, documentation, and final polish.

### CI/CD Integration

- [ ] T-S012-P08-054 Create GitHub Actions workflow for e2e tests (`.github/workflows/e2e-tests.yml`) with test execution, artifact upload, and result reporting
- [ ] T-S012-P08-055 [P] Configure test execution in CI (`.github/workflows/e2e-tests.yml`) with environment detection, service URL configuration, and timeout settings
- [ ] T-S012-P08-056 [P] Configure artifact upload in CI (`.github/workflows/e2e-tests.yml`) uploading test reports and artifacts to GitHub Actions
- [ ] T-S012-P08-057 Configure test result reporting in CI (`.github/workflows/e2e-tests.yml`) with JUnit XML parsing and test summary generation

### Documentation

- [ ] T-S012-P08-058 [P] Document test harness usage (`tests/e2e/README.md`) with API reference, configuration options, and examples
- [ ] T-S012-P08-059 Finalize quickstart guide (`specs/012-e2e-tests/quickstart.md`) with prerequisites, running tests, and troubleshooting
- [ ] T-S012-P08-060 [P] Document test patterns and best practices (`tests/e2e/README.md`) with fixture usage, retry patterns, and isolation strategies
- [ ] T-S012-P08-061 Update `llms.txt` with test documentation links (spec, plan, quickstart, README)
- [ ] T-S012-P08-062 Create troubleshooting guide (`tests/e2e/TROUBLESHOOTING.md`) with common issues, solutions, and debugging tips

### Security & Cleanup

- [ ] T-S012-P08-063 [P] Add unit tests for test harness components (`tests/e2e/harness/*_test.go`) testing client, fixtures, context, and reporting
- [ ] T-S012-P08-065 Validate test cleanup removes ≥99% of resources (`tests/e2e/utils/cleanup_test.go`) with resource audit and cleanup verification

**Checkpoint**: CI/CD integrated, documentation complete, security hardened, test harness tested.

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - User stories can proceed in priority order (P1 → P2 → P3)
  - Or in parallel if team capacity allows (after foundational complete)
- **Polish (Phase 8)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories - **MVP**
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Independently testable
- **User Story 4 (P2)**: Can start after Foundational (Phase 2) - Independently testable
- **User Story 5 (P2)**: Can start after Foundational (Phase 2) - Independently testable

### Within Each User Story

- Test implementation before validation
- Core test cases before edge cases
- Happy path before negative scenarios
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, user stories can start in parallel (if team capacity allows)
- Tests within a story marked [P] can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all test implementations for User Story 1 together:
Task: "Implement organization lifecycle test in tests/e2e/suites/happy_path_test.go"
Task: "Implement user invite flow test in tests/e2e/suites/happy_path_test.go"
Task: "Implement API key issuance test in tests/e2e/suites/happy_path_test.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (Happy Path Tests)
4. **STOP and VALIDATE**: Test User Story 1 independently
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test independently → Deploy/Demo
4. Add User Story 4 → Test independently → Deploy/Demo
5. Add User Story 5 → Test independently → Deploy/Demo
6. Add User Story 3 → Test independently → Deploy/Demo
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (P1 - MVP)
   - Developer B: User Story 2 (P2)
   - Developer C: User Story 4 (P2)
3. After P1 complete:
   - Developer A: User Story 5 (P2)
   - Developer B: User Story 3 (P3)
4. Stories complete and integrate independently

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Verify tests pass against real services or mocks
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
- Test harness must be tested itself (unit tests in Phase 8)
- All sensitive values must be masked in logs and artifacts (security requirement)

