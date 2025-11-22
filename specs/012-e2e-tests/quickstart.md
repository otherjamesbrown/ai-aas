# Quickstart: End-to-End Tests

**Feature**: `012-e2e-tests`  
**Date**: 2025-01-27  
**Status**: Draft

## Overview

This guide provides a quick start for running end-to-end tests against the AI-as-a-Service platform. Tests validate critical flows across services including authentication, organization management, API routing, budget enforcement, and audit logging.

## Prerequisites

### Required

- Go 1.21 or later
- Network access to test environment (local or remote)
- Test environment with services deployed and accessible

### Optional

- Docker (for local testcontainers)
- kubectl (for Kubernetes-based test execution)
- jq (for JSON report parsing)

## Quick Start

### 1. Run Tests Locally

```bash
# Navigate to test directory
cd tests/e2e

# Run all tests against local environment
make test-local

# Run specific test suite
go test -v ./suites -run TestHappyPath

# Run with custom configuration
TEST_ENV=local \
  USER_ORG_SERVICE_URL=http://localhost:8081 \
  API_ROUTER_SERVICE_URL=http://localhost:8082 \
  make test-local
```

### 2. Run Tests Against Development Environment

```bash
# Set environment variables
export TEST_ENV=development
export USER_ORG_SERVICE_URL=http://user-org-service.dev.platform.internal:8081
export API_ROUTER_SERVICE_URL=http://api-router-service.dev.platform.internal:8082
export ANALYTICS_SERVICE_URL=http://analytics-service.dev.platform.internal:8083
export ADMIN_API_KEY=your-admin-api-key

# Run tests
make test-dev
```

### 3. Run Tests in CI/CD

Tests run automatically in CI/CD pipelines. To trigger manually:

```bash
# GitHub Actions
gh workflow run e2e-tests.yml

# Or via Makefile
make test-ci
```

## Test Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `TEST_ENV` | Test environment (`local`, `development`, `staging`) | `local` | No |
| `USER_ORG_SERVICE_URL` | User-org service API URL | `http://localhost:8081` | Yes |
| `API_ROUTER_SERVICE_URL` | API router service URL | `http://localhost:8082` | Yes |
| `ANALYTICS_SERVICE_URL` | Analytics service URL | `http://localhost:8083` | Yes |
| `ADMIN_API_KEY` | Admin API key for test setup | - | Yes (for remote) |
| `TEST_RUN_ID` | Unique test run identifier | Auto-generated | No |
| `PARALLEL_WORKERS` | Number of parallel test workers | `1` | No |
| `TEST_TIMEOUT` | Test case timeout (seconds) | `300` | No |
| `ARTIFACTS_DIR` | Artifact output directory | `./artifacts` | No |
| `ENABLE_CLEANUP` | Enable automatic cleanup | `true` | No |

### Configuration File

Create `tests/e2e/config.yaml`:

```yaml
environment: development
api_urls:
  user_org_service: http://user-org-service.dev.platform.internal:8081
  api_router_service: http://api-router-service.dev.platform.internal:8082
  analytics_service: http://analytics-service.dev.platform.internal:8083
timeouts:
  request_timeout_ms: 30000
  test_timeout_ms: 300000
parallel:
  enabled: true
  workers: 4
cleanup:
  enabled: true
  delay_seconds: 60
artifacts:
  enabled: true
  output_dir: ./artifacts
```

## Test Suites

### Happy Path Tests

Tests critical success flows:

```bash
go test -v ./suites -run TestHappyPath
```

**Tests**:
- Organization lifecycle
- User invite flow
- API key issuance
- Model request routing
- Successful completion

### Budget Enforcement Tests

Tests budget limits and enforcement:

```bash
go test -v ./suites -run TestBudget
```

**Tests**:
- Budget limit enforcement
- Budget exceeded denial
- Budget reset and recovery

### Authorization Tests

Tests role-based access control:

```bash
go test -v ./suites -run TestAuth
```

**Tests**:
- Role permission enforcement
- Unauthorized access denial
- API key scope validation

### Declarative Convergence Tests

Tests Git-as-source-of-truth convergence:

```bash
go test -v ./suites -run TestDeclarative
```

**Tests**:
- Declarative change application
- Drift detection
- Reconciliation verification

### Audit Trail Tests

Tests audit logging and correlation:

```bash
go test -v ./suites -run TestAudit
```

**Tests**:
- Audit event generation
- Correlation ID tracking
- Audit log querying

## Test Execution

### Sequential Execution

Run tests sequentially (default):

```bash
go test -v ./suites
```

### Parallel Execution

Run tests in parallel:

```bash
PARALLEL_WORKERS=4 go test -v ./suites -parallel 4
```

### Selective Test Execution

Run specific test cases:

```bash
# Run single test
go test -v ./suites -run TestOrganizationLifecycle

# Run tests matching pattern
go test -v ./suites -run "TestBudget.*"

# Skip specific tests
go test -v ./suites -run "Test.*" -skip "TestDeclarative"
```

## Test Results

### Console Output

Tests output results to console:

```
=== RUN   TestOrganizationLifecycle
[TEST] TestOrganizationLifecycle
[STEP] CreateOrganization
[PASS] Organization created: org-e2e-abc123
[STEP] CreateUser
[PASS] User created: user-e2e-def456
[PASS] TestOrganizationLifecycle
--- PASS: TestOrganizationLifecycle (5.23s)
```

### JUnit XML Reports

Generate JUnit XML reports for CI/CD:

```bash
go test -v ./suites -json | go-junit-report > test-results.xml
```

### JSON Reports

Generate JSON reports for analysis:

```bash
go test -v ./suites -json > test-results.json
```

### Artifacts

Test artifacts are stored in `./artifacts`:

```
artifacts/
├── run-xyz789/
│   ├── test-org-lifecycle/
│   │   ├── request-001.json
│   │   ├── response-001.json
│   │   └── correlation-ids.json
│   └── test-budget-enforcement/
│       └── ...
└── test-report.json
```

## Troubleshooting

### Common Issues

#### Tests Fail with Connection Errors

**Problem**: Tests cannot connect to services.

**Solution**:
- Verify services are running and accessible
- Check network connectivity
- Verify service URLs in configuration
- Check firewall rules

#### Tests Fail with Timeout Errors

**Problem**: Tests timeout before completion.

**Solution**:
- Increase `TEST_TIMEOUT` environment variable
- Check service performance and latency
- Verify test environment has sufficient resources
- Review test case complexity

#### Tests Fail with Resource Conflicts

**Problem**: Tests conflict when running in parallel.

**Solution**:
- Reduce `PARALLEL_WORKERS`
- Verify test isolation (unique namespaces)
- Check resource cleanup
- Review test data generation

#### Tests Leave Resources Behind

**Problem**: Test resources not cleaned up.

**Solution**:
- Verify `ENABLE_CLEANUP=true`
- Check cleanup timeout settings
- Manually clean up resources:
  ```bash
  make cleanup-test-resources
  ```

### Debug Mode

Enable verbose logging:

```bash
TEST_DEBUG=true go test -v ./suites
```

### Manual Cleanup

Clean up test resources manually:

```bash
# List test resources
make list-test-resources

# Clean up specific test run
TEST_RUN_ID=run-xyz789 make cleanup-test-resources

# Clean up all test resources (use with caution)
make cleanup-all-test-resources
```

## Best Practices

### 1. Run Tests Before Committing

```bash
make test-local
```

### 2. Run Tests in CI Before Merging

Tests run automatically in CI/CD pipelines.

### 3. Review Test Artifacts on Failure

Check `./artifacts` directory for request/response details.

### 4. Use Correlation IDs for Debugging

Correlation IDs link test steps to service logs:

```bash
# Search logs by correlation ID
grep "req-xyz789" service-logs.json
```

### 5. Keep Tests Independent

Each test should be independently runnable and clean up after itself.

## Next Steps

- Read [Test Patterns Guide](../docs/test-patterns.md)
- Review [Test Data Models](./data-model.md)
- Explore [Test Framework Research](./research.md)
- Check [Implementation Plan](./plan.md)

