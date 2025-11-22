# Implementation Plan: End-to-End Tests

**Branch**: `012-e2e-tests` | **Date**: 2025-01-27 | **Spec**: `/specs/012-e2e-tests/spec.md`  
**Input**: Feature specification from `/specs/012-e2e-tests/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Deliver a comprehensive end-to-end test harness that validates critical flows across services: authentication, organization/user management, API key issuance, routing to model backends, budget enforcement, audit logging, and Git-as-source-of-truth convergence. The harness supports parallel execution, deterministic test data, clear failure diagnostics, and CI-friendly outputs. Tests are isolated, retry-safe, and produce actionable artifacts for debugging.

## Technical Context

**Language/Version**: Go 1.21+, testing package  
**Primary Dependencies**: `net/http`, `encoding/json`, existing service APIs (user-org-service, api-router-service, analytics-service)  
**Testing**: Custom Go test harness with JUnit XML and JSON reporting  
**Target Platform**: Local development, CI/CD (GitHub Actions), deployed environments (development, staging)  
**Project Type**: Test infrastructure and test suites  
**Performance Goals**: Full happy-path suite completes in ≤10 minutes, parallel execution with ≥4 workers  
**Constraints**: Tests must not impact production, test data must be isolated and cleanable, tests must be deterministic and retry-safe  
**Scale/Scope**: Support 15+ test cases across 3+ services with parallel execution

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **API-First Interfaces**: Tests interact with services via HTTP APIs; test harness exposes configuration APIs  
- **Stateless Microservices & Async Non-Critical Paths**: Tests are stateless and can run in parallel; cleanup operates asynchronously  
- **Security by Default**: Test credentials stored securely, test data isolated from production, sensitive values masked in logs  
- **Declarative Infrastructure & GitOps**: Test configuration versioned in Git, test results stored as artifacts  
- **Observability**: Test execution emits metrics and logs; test failures include correlation IDs for debugging  
- **Testing**: Test harness is tested via unit tests; test suites validate themselves  
- **Performance**: Test execution meets performance goals; parallel execution optimizes resource utilization

## Project Structure

### Documentation (this feature)

```text
specs/012-e2e-tests/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
└── tasks.md                 # generated via /speckit.tasks
```

### Source Code (repository root)

```text
tests/
├── e2e/
│   ├── harness/
│   │   ├── client.go              # Test HTTP client wrapper
│   │   ├── fixtures.go            # Fixture management
│   │   ├── context.go             # Test context
│   │   ├── artifacts.go           # Artifact collection
│   │   ├── reporting.go           # Report generation
│   │   └── config.go              # Test configuration
│   ├── suites/
│   │   ├── happy_path_test.go     # Happy path tests
│   │   ├── budget_test.go         # Budget enforcement tests
│   │   ├── auth_test.go           # Authorization tests
│   │   ├── declarative_test.go    # Declarative convergence tests
│   │   └── audit_test.go          # Audit trail tests
│   ├── fixtures/
│   │   ├── organizations.go       # Organization fixtures
│   │   ├── users.go               # User fixtures
│   │   └── api_keys.go            # API key fixtures
│   └── utils/
│       ├── retry.go               # Retry utilities
│       ├── wait.go                # Wait utilities
│       └── correlation.go         # Correlation ID utilities
├── README.md
└── Makefile
```

## Phases

### Phase 0: Research & Design
- Evaluate test framework options (Go testing, testcontainers, etc.)
- Design test harness architecture
- Design test data models and fixtures
- Design test isolation strategies
- Design reporting and artifact collection
- Document test patterns and best practices

### Phase 1: Basic Test Harness
- Implement test HTTP client wrapper
- Implement basic test context
- Implement test configuration loading
- Create initial test suite structure
- Implement basic reporting (console output)

### Phase 2: Fixture Management
- Implement fixture manager
- Implement organization fixtures
- Implement user fixtures
- Implement API key fixtures
- Implement budget fixtures
- Implement automatic cleanup

### Phase 3: Happy Path Tests
- Implement organization lifecycle test
- Implement user invite flow test
- Implement API key issuance test
- Implement model request routing test
- Implement successful completion test
- Validate test execution and reporting

### Phase 4: Negative Tests
- Implement budget enforcement tests
- Implement authorization denial tests
- Implement error handling tests
- Validate negative test scenarios

### Phase 5: Advanced Features
- Implement parallel execution support
- Implement test isolation (namespaces, worker IDs)
- Implement retry logic with exponential backoff
- Implement configurable timeouts
- Validate parallel execution

### Phase 6: Artifact Collection & Reporting
- Implement artifact collection (request/response bodies)
- Implement JUnit XML report generation
- Implement JSON report generation
- Implement correlation ID tracking
- Validate artifact collection and reporting

### Phase 7: Declarative Convergence Tests
- Implement declarative change application test
- Implement drift detection test
- Implement reconciliation verification test
- Validate declarative convergence scenarios

### Phase 8: Audit Trail Tests
- Implement audit event verification test
- Implement correlation ID validation test
- Implement audit log query test
- Validate audit trail scenarios

### Phase 9: CI/CD Integration
- Create GitHub Actions workflow for e2e tests
- Configure test execution in CI
- Configure artifact upload
- Configure test result reporting
- Validate CI/CD integration

### Phase 10: Documentation & Quickstart
- Document test harness usage
- Create quickstart guide
- Document test patterns and best practices
- Update `llms.txt` with test documentation links
- Create troubleshooting guide

## Dependencies

- **005-user-org-service**: User and organization management APIs  
- **006-api-router-service**: API routing and model backend integration  
- **007-analytics-service**: Analytics and audit log APIs  
- **011-observability**: Metrics, logs, and traces for test diagnostics  
- **001-infrastructure**: Kubernetes clusters and networking for test execution

## Risks & Mitigations

| Risk | Impact | Mitigation |
|------|--------|------------|
| Test flakiness | False positives, reduced confidence | Implement retries, deterministic test data, proper timeouts |
| Test data leakage | Cross-test contamination | Use unique namespaces, automatic cleanup, test isolation |
| Slow test execution | Delayed feedback | Parallel execution, optimize test cases, selective test runs |
| Resource conflicts | Test failures | Isolated namespaces, worker-specific resources, resource limits |
| External dependency failures | Test failures | Mock external dependencies, skip tests when dependencies unavailable |
| Test maintenance burden | Outdated tests | Clear test patterns, reusable fixtures, good documentation |

## Success Metrics

- **SC-001**: Full happy-path suite completes in ≤10 minutes (measured via CI execution time)
- **SC-002**: Budget and authorization negative tests pass consistently across three consecutive CI runs (measured via CI success rate)
- **SC-003**: Test artifacts include correlation IDs and response samples for ≥95% of failed steps (measured via artifact analysis)
- **SC-004**: Declarative convergence tests complete with verified "synced" state within ≤3 minutes 95% of the time (measured via test execution logs)
- **SC-005**: Test suite can run in parallel with ≥4 concurrent test workers without resource conflicts (measured via parallel execution tests)
- **SC-006**: Test cleanup removes ≥99% of test-created resources within 5 minutes of test completion (measured via resource audit)
- **SC-007**: Test failures include actionable diagnostics in ≥95% of cases (measured via failure analysis)

## Next Steps

1. Complete research phase (Phase 0)
2. Implement basic test harness (Phase 1)
3. Implement fixture management (Phase 2)
4. Implement happy path tests (Phase 3)
5. Implement negative tests (Phase 4)
6. Add advanced features (Phase 5)
7. Implement artifact collection (Phase 6)
8. Implement declarative convergence tests (Phase 7)
9. Implement audit trail tests (Phase 8)
10. Integrate with CI/CD (Phase 9)
11. Document and create quickstart (Phase 10)

