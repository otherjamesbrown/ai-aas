# End-to-End Test Harness

This directory contains the end-to-end test harness for the AI-as-a-Service platform. The harness validates critical flows across services including authentication, organization/user management, API key issuance, routing to model backends, budget enforcement, audit logging, and Git-as-source-of-truth convergence.

## Overview

The test harness provides:
- **Test orchestration**: Setup, execution, and teardown with clear logs and artifacts
- **Deterministic test data**: Fixtures for organizations, users, API keys, and budgets
- **Parallel execution**: Support for running tests in parallel with isolated resources
- **Failure diagnostics**: Request/response bodies, timings, and correlation IDs
- **CI-friendly outputs**: JUnit XML and JSON reports compatible with CI systems

## Prerequisites

- Go 1.21 or later
- Network access to test environment (local or remote)
- Test environment with services deployed and accessible
- `curl` (for bootstrap script)

## Initial Setup (One-Time)

Before running tests for the first time, set up the test environment:

```bash
cd tests/e2e

# Run the setup script (handles all configuration)
make setup

# Or run manually:
./scripts/setup-test-env.sh
```

This will:
1. Check prerequisites
2. Configure service URLs
3. Bootstrap an admin API key (if needed)
4. Verify connectivity

The admin key will be saved to `.admin-key.env` (git-ignored) for future use.

**Note**: The bootstrap script requires an existing admin API key or user credentials to create the test organization. If you don't have one:

1. **Option A**: Use the seed command to create an admin user:
   ```bash
   cd services/user-org-service
   go run cmd/seed/main.go -org-slug=e2e-admin -user-email=admin@e2e.test
   ```
   Then log in via web portal and create an API key.

2. **Option B**: If you have an existing admin key:
   ```bash
   export ADMIN_API_KEY=your-existing-key
   make setup
   ```

## Quick Start

### Run Tests Locally

```bash
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

### Run Tests Against Development Environment

**Option 1: Via Public Internet (Recommended if ingress is enabled)**

Run tests directly over the internet using public ingress URLs:

```bash
# Set admin API key (required for test setup)
export ADMIN_API_KEY=your-admin-api-key

# Optionally override default URLs (defaults shown below)
export USER_ORG_SERVICE_URL=https://user-org.api.ai-aas.dev
export API_ROUTER_SERVICE_URL=https://router.api.ai-aas.dev
export ANALYTICS_SERVICE_URL=https://analytics.api.ai-aas.dev

# Run tests via internet
make test-dev-internet

# Or run the script directly
./scripts/test-dev-internet.sh
```

The script will attempt to discover ingress URLs from the cluster if `kubectl` is available.

**Option 2: Automatic Port-Forwarding**

The test script automatically sets up port-forwarding to access services in the development cluster:

```bash
# Set admin API key (required for test setup)
export ADMIN_API_KEY=your-admin-api-key

# Optionally set kubeconfig path (defaults to ~/kubeconfigs/kubeconfig-development.yaml)
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Run tests with automatic port-forwarding
make test-dev-remote

# Or run the script directly
./scripts/test-dev.sh
```

**Option 3: Manual Port-Forwarding**

If you prefer to manage port-forwards manually:

```bash
# In separate terminals, set up port-forwards:
kubectl port-forward -n development svc/user-org-service 8081:8081
kubectl port-forward -n development svc/api-router-service 8082:8080
kubectl port-forward -n development svc/analytics-service 8083:8083

# Then run tests:
export TEST_ENV=development
export USER_ORG_SERVICE_URL=http://localhost:8081
export API_ROUTER_SERVICE_URL=http://localhost:8082
export ANALYTICS_SERVICE_URL=http://localhost:8083
export ADMIN_API_KEY=your-admin-api-key

make test-dev
```

**Option 4: Direct IP Address (No /etc/hosts modification needed)**

If you can't modify `/etc/hosts`, use the IP address directly:

```bash
# Set admin API key
export ADMIN_API_KEY=your-admin-api-key

# Run tests with IP address
make test-dev-ip

# Or manually:
export TEST_ENV=development
export USER_ORG_SERVICE_URL=https://172.232.58.222
export API_ROUTER_SERVICE_URL=https://172.232.58.222
export ANALYTICS_SERVICE_URL=https://172.232.58.222
go test -v ./suites/... -timeout 30m
```

**Note:** If services require specific Host headers, you may need to configure those in the test harness.

**Prerequisites:**
- For internet access: Ingress enabled and services publicly accessible
- For port-forwarding: `kubectl` configured with access to the development cluster
- Kubeconfig file (default: `~/kubeconfigs/kubeconfig-development.yaml`)
- Admin API key for test setup and cleanup

### Run Tests in CI

Tests run automatically in CI/CD pipelines. To trigger manually:

```bash
make test-ci
```

## Test Suites

- **Happy Path Tests** (`suites/happy_path_test.go`): Critical success flows
- **Budget Tests** (`suites/budget_test.go`): Budget enforcement and limits
- **Authorization Tests** (`suites/auth_test.go`): Role-based access control
- **Declarative Tests** (`suites/declarative_test.go`): Git-as-source-of-truth convergence
- **Audit Tests** (`suites/audit_test.go`): Audit trail validation
- **Resilience Tests** (`suites/resilience_test.go`): Service health and failure handling

## Configuration

See `specs/012-e2e-tests/quickstart.md` for detailed configuration options.

## Test Patterns and Best Practices

### Fixture Usage

Use fixtures to create test data with automatic cleanup:

```go
orgFixture := fixtures.NewOrganizationFixture(ctx.Client, ctx.Fixtures)
org, err := orgFixture.Create(ctx, "")
// org is automatically registered for cleanup
```

### Retry Patterns

Use retry utilities for flaky operations:

```go
err := utils.Retry(func() error {
    return someOperation()
}, 3, 1*time.Second)
```

### Isolation Strategies

- Each test gets a unique `RunID` for resource isolation
- Use `ctx.GenerateResourceName()` for unique resource names
- Fixtures automatically register resources for cleanup

### Correlation IDs

Always set correlation IDs for traceability:

```go
corrID := utils.GenerateCorrelationID()
ctx.Client.SetHeader("X-Correlation-ID", corrID)
ctx.Client.SetHeader("X-Request-ID", corrID)
```

## Configuration Reference

### Environment Variables

- `TEST_ENV`: Test environment (`local`, `development`, `ci`)
- `USER_ORG_SERVICE_URL`: User/org service base URL
- `API_ROUTER_SERVICE_URL`: API router service base URL
- `ANALYTICS_SERVICE_URL`: Analytics service base URL
- `ADMIN_API_KEY`: Admin API key for test setup
- `REQUEST_TIMEOUT_MS`: Request timeout in milliseconds (default: 30000)
- `PARALLEL_WORKERS`: Number of parallel test workers (default: 1)
- `WORKER_ID`: Worker ID for parallel execution

### API Reference

#### Test Context

```go
ctx := setupTestContext(t)
defer ctx.Cleanup()

// Access client
ctx.Client.GET("/v1/orgs")

// Generate unique resource names
name := ctx.GenerateResourceName("org")

// Access fixtures manager
ctx.Fixtures.Register("org", orgID, metadata)
```

#### Fixtures

```go
// Organizations
orgFixture := fixtures.NewOrganizationFixture(client, fixtures)
org, err := orgFixture.Create(ctx, "")

// Users
userFixture := fixtures.NewUserFixture(client, fixtures)
invite, err := userFixture.Invite(orgID, email)

// API Keys
apiKeyFixture := fixtures.NewAPIKeyFixture(client, fixtures)
apiKey, err := apiKeyFixture.Create(ctx, orgID, name, scopes)

// Budgets
budgetFixture := fixtures.NewBudgetFixture(client, fixtures)
budget, err := budgetFixture.Create(ctx, orgID, limit, currency, period)
```

#### Utilities

```go
// Retry with exponential backoff
err := utils.Retry(func() error { return op() }, maxAttempts, initialDelay)

// Wait for condition
err := utils.WaitFor(func() bool { return condition() }, timeout, interval)

// Generate correlation IDs
corrID := utils.GenerateCorrelationID()

// Mock backends
mockBackend := utils.NewMockBackend()
defer mockBackend.Close()
```

## Documentation

- [Specification](../specs/012-e2e-tests/spec.md)
- [Implementation Plan](../specs/012-e2e-tests/plan.md)
- [Quickstart Guide](../specs/012-e2e-tests/quickstart.md)
- [Data Model](../specs/012-e2e-tests/data-model.md)
- [Troubleshooting Guide](TROUBLESHOOTING.md)

## Making Setup Repeatable

The test harness includes automated setup scripts to make testing repeatable:

1. **One-time setup**: Run `make setup` to bootstrap admin credentials
2. **Automatic loading**: Tests automatically load `.admin-key.env` if it exists
3. **Idempotent**: Safe to run setup multiple times

The `.admin-key.env` file is git-ignored and contains your admin API key for test execution.
