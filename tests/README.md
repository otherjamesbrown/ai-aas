# Test Repository

This directory contains cross-service tests, shared library tests, and test utilities for the AI-AAS platform. Service-specific tests are located within each service directory.

## E2E Testing Credentials

For E2E tests that require authentication against the development cluster, use the admin API key documented in:
- **[E2E Admin API Key](../docs/e2e/E2E_ADMIN_API_KEY.md)** - Current credentials and usage
- **[Setup Guide](../docs/e2e/SETUP_E2E_ADMIN_KEY.md)** - How to recreate or rotate the key

```bash
# Use in tests
export E2E_API_KEY="1z_V2QVOJJt0d2f2aQI9PUwQfudAugmOs4issi96jv0"
export E2E_ORG_ID="aa6f9015-132a-4694-8b10-7d4d4550faed"
```

## Test Organization

### Root-Level Tests (`tests/`)

Cross-service and shared library tests:

- **`tests/go/`** - Shared Go library tests
  - `unit/` - Unit tests for shared Go packages
  - `integration/` - Integration tests for shared Go libraries
  - `contract/` - Contract tests (OpenAPI/Protocol Buffers)
  - `perf/` - Performance benchmarks

- **`tests/ts/`** - Shared TypeScript library tests
  - `unit/` - Unit tests for shared TypeScript packages
  - `integration/` - Integration tests for shared TypeScript libraries
  - `contract/` - Contract tests (Type definitions, API contracts)
  - `perf/` - Performance benchmarks

- **`tests/analytics/`** - Analytics service tests
  - `integration/` - Integration tests for analytics workflows
  - `integration-phase5/` - Phase 5 specific integration tests
  - `perf/` - Performance benchmarks

- **`tests/infra/`** - Infrastructure tests
  - `terratest/` - Terraform infrastructure tests
  - `synthetics/` - Synthetic monitoring probes
  - `perf/` - Infrastructure performance tests

- **`tests/dev/`** - Development workflow tests
  - `service_happy_path.sh` - Service lifecycle happy path tests

- **`tests/perf/`** - Cross-platform performance benchmarks
  - `build_all_benchmark.sh` - Build performance benchmarks
  - `measure_help.sh` - Help command performance

- **`tests/check/`** - Code quality check fixtures
  - `fixtures/` - Test fixtures for code checks

### Service-Specific Tests

#### User-Org-Service (`services/user-org-service/`)

- **Location**: 
  - `cmd/e2e-test/main.go` - End-to-end HTTP tests
  - `internal/server/server_test.go` - Server middleware and CORS tests
- **Type**: End-to-end HTTP tests, unit tests
- **Coverage**:
  - Health checks
  - Organization CRUD operations
  - User invitation and management
  - Authentication flows (login, refresh, logout, userinfo)
  - API key lifecycle (with seeded users)
  - CORS middleware (preflight, allowed origins, error responses)
  - Request/response logging
  - Route debugging endpoint

**Run Tests**:
```bash
cd services/user-org-service
make e2e-test-local          # Run against localhost:8081
make smoke-k6-local          # Run K6 load tests
make test                    # Run unit/integration tests (includes server tests)
go test ./internal/server/...  # Run server-specific tests
```

**Dependencies**: 
- E2E tests: Service must be running on port 8081, database migrated
- Unit tests: No dependencies (can run standalone)

**Documentation**: `services/user-org-service/docs/e2e-testing.md`

#### API Router Service (`services/api-router-service/`)

- **Location**: `test/integration/`, `test/contract/`
- **Type**: Integration tests, contract tests
- **Coverage**:
  - API key validation
  - Routing weights and failover
  - Health endpoints
  - Rate limiting and budget enforcement
  - Inference request success paths

**Run Tests**:
```bash
cd services/api-router-service
make test                    # Run unit tests
make integration-test        # Run integration tests (TODO: implement)
make contract-test           # Run contract tests (TODO: implement)
```

#### Analytics Service (`services/analytics-service/`)

- **Location**: `test/integration-phase5/`, `test/perf/`
- **Type**: Integration tests, performance benchmarks
- **Coverage**:
  - Export reconciliation
  - Reliability incident tracking
  - Usage visibility
  - Freshness benchmarks

**Run Tests**:
```bash
cd services/analytics-service
make test                    # Run unit tests
# Integration tests also in tests/analytics/
```

#### Web Portal (`web/portal/`)

- **Location**: `tests/`, `src/**/*.test.ts`, `src/**/*.test.tsx`
- **Type**: Unit tests (Vitest), E2E tests (Playwright)
- **Coverage**:
  - React component unit tests
  - Hook unit tests
  - End-to-end user flows
  - Accessibility tests (a11y)

**Run Tests**:
```bash
cd web/portal
pnpm test                    # Run unit tests
pnpm test:e2e               # Run E2E tests (headless by default)
pnpm test:e2e:ui            # Run E2E tests with UI mode
pnpm test:a11y              # Run accessibility tests
pnpm test:all               # Run all tests

# Run E2E tests with existing server (skip webServer)
SKIP_WEBSERVER=true PLAYWRIGHT_BASE_URL=http://localhost:5173 pnpm test:e2e

# Run in non-headless mode (see browser)
PLAYWRIGHT_HEADLESS=false pnpm test:e2e
```

**Documentation**: `web/portal/tests/README.md`

## Running Tests

### Shared Library Tests

```bash
# Run all shared Go tests
make shared-go-test

# Run all shared TypeScript tests
make shared-ts-test

# Run all shared library tests
make shared-test
```

### Service Tests

```bash
# Run tests for a specific service
make test SERVICE=user-org-service

# Run tests for all services
make test SERVICE=all

# Run full check (fmt, lint, security, test) for a service
make check SERVICE=user-org-service

# Run specific test package (e.g., server tests)
cd services/user-org-service
go test -v ./internal/server/...
```

### Cross-Service Tests

```bash
# Analytics reconciliation tests
make analytics-verify

# Infrastructure tests (Terratest)
cd tests/infra/terratest && go test ./...

# Performance benchmarks
cd tests/perf && ./build_all_benchmark.sh
```

## Test Types

### Unit Tests

- **Purpose**: Test individual functions, components, or modules in isolation
- **Tools**: 
  - Go: `go test`
  - TypeScript: Vitest
  - React: Vitest + React Testing Library
- **Location**: Co-located with source code (`*_test.go`, `*.test.ts`, `*.test.tsx`)

### Integration Tests

- **Purpose**: Test interactions between components or services
- **Tools**: 
  - Go: Testcontainers, Docker Compose
  - TypeScript: Vitest with service mocks
- **Location**: 
  - `tests/go/integration/`
  - `tests/ts/integration/`
  - `services/*/test/integration/`

### End-to-End (E2E) Tests

- **Purpose**: Test complete user workflows across multiple services
- **Tools**: 
  - Go: Custom HTTP client tests (`cmd/e2e-test/`)
  - Web: Playwright (runs headless by default, no report server)
  - Load: K6
- **Location**:
  - `services/user-org-service/cmd/e2e-test/`
  - `web/portal/tests/e2e/`
  - `specs/012-e2e-tests/` (specification)
- **Configuration**: 
  - Playwright tests run headless by default (`PLAYWRIGHT_HEADLESS=true`)
  - HTML reports generated but server not started automatically
  - Use `SKIP_WEBSERVER=true` to use existing dev server

### Contract Tests

- **Purpose**: Validate API contracts (OpenAPI, Protocol Buffers, TypeScript types)
- **Tools**: 
  - Go: Custom validators, buf
  - TypeScript: Type validation
- **Location**:
  - `tests/go/contract/`
  - `tests/ts/contract/`
  - `services/*/test/contract/`

### Performance Tests

- **Purpose**: Measure latency, throughput, and resource usage
- **Tools**: 
  - Go: `go test -bench`
  - TypeScript: Vitest benchmarks
  - Load: K6
- **Location**:
  - `tests/go/perf/`
  - `tests/ts/perf/`
  - `tests/analytics/perf/`
  - `tests/infra/perf/`

### Infrastructure Tests

- **Purpose**: Validate Terraform configurations and deployment
- **Tools**: Terratest
- **Location**: `tests/infra/terratest/`

### Synthetic Tests

- **Purpose**: Continuous monitoring and availability checks
- **Tools**: Custom probes
- **Location**: `tests/infra/synthetics/`

## Test Coverage Targets

- **Shared Libraries**: 80% coverage target (configurable via `SHARED_GO_COVERAGE_TARGET`)
- **Services**: Service-specific targets (check individual service Makefiles)
- **Web Portal**: Target 80%+ for critical components

## Test Data and Fixtures

- **Test Data**: Each test suite manages its own test data
- **Cleanup**: Tests should clean up after themselves
- **Seeding**: Some services provide seed commands for E2E tests:
  - `services/user-org-service/cmd/seed/` - Creates test users and orgs

## CI/CD Integration

### GitHub Actions

- Tests run automatically on pull requests
- Service-specific workflows in `.github/workflows/`
- Shared library tests run in `shared-go-test` and `shared-ts-test` jobs

### Local Testing

```bash
# Run CI checks locally
make ci-local              # Run GitHub Actions locally via act

# Run specific workflow
act -j test                # Run test job
```

## Troubleshooting

### Tests Fail with Connection Refused

- Verify services are running
- Check environment variables (API_URL, DATABASE_URL)
- For Kubernetes tests: Verify service names and namespaces

### Tests Fail with 404 Not Found

- Verify database migrations are applied
- Check service logs for errors
- Ensure test data doesn't conflict with existing data

### Tests Timeout

- Increase timeout in test configuration
- Check network connectivity
- Verify services are not overloaded

### Test Coverage Issues

- Run with coverage flags: `go test -coverprofile=coverage.out`
- View coverage: `go tool cover -html=coverage.out`
- Check coverage targets in service Makefiles

### Playwright Authentication Hangs (Known Issue)

**Issue**: Playwright E2E tests hang when attempting browser-based authentication via OAuth/password login endpoints.

**Symptoms**:
- Frontend reaches login POST request but never receives response
- Backend successfully processes request and returns 200 OK
- Both `axios` and `fetch()` exhibit identical hanging behavior
- Real browsers (Firefox, Chrome) work correctly
- curl/API requests work correctly

**Root Cause**: Incompatibility between Playwright's network interception and Fosite OAuth library's response writing mechanism.

**Workarounds**:

1. **Use Playwright's Request API** (Recommended):
   ```typescript
   // Login via API instead of browser
   const response = await request.post('http://localhost:8081/v1/auth/login', {
     data: { email: 'admin@example-acme.com', password: 'password' }
   });
   const { access_token } = await response.json();

   // Inject token into browser context
   await page.evaluate((token) => {
     sessionStorage.setItem('auth_token', token);
   }, access_token);
   ```

2. **Mock Authentication**:
   - Pre-generate valid tokens
   - Inject directly into sessionStorage before tests
   - Test functionality independently of login flow

3. **Split Test Coverage**:
   - Unit tests for authentication logic
   - API tests (curl/request API) for auth endpoints
   - E2E tests for UI flows with mocked/injected auth

**Documentation**: See `tmp_md/API_KEY_CREATION_ISSUE_SUMMARY.md` for detailed investigation notes.

## Adding New Tests

### Service Tests

1. Add test files co-located with source code (`*_test.go`, `*.test.ts`)
2. Service Makefile will automatically include them in `make test`

**Example**: Server middleware tests in `services/user-org-service/internal/server/server_test.go`:
- Tests CORS handling, request logging, error responses
- Can run standalone: `go test ./internal/server/...`
- Included in `make test SERVICE=user-org-service`

### Cross-Service Tests

1. Add to appropriate directory in `tests/`
2. Follow existing patterns for organization
3. Update this README if adding new test categories

### E2E Tests

1. Follow patterns in `services/user-org-service/cmd/e2e-test/`
2. Reference `specs/012-e2e-tests/spec.md` for requirements
3. Ensure tests are parallelizable and deterministic

## Test Execution Examples

### Run User-Org-Service E2E Tests

```bash
# 1. Ensure service is running
cd services/user-org-service
make run                    # Start service on port 8081

# 2. Ensure database is migrated
export DATABASE_URL="postgres://postgres:postgres@localhost:5433/ai_aas?sslmode=disable"
export USER_ORG_DATABASE_URL="$DATABASE_URL"
make migrate

# 3. Run E2E tests
make e2e-test-local

# Optional: Run K6 smoke tests
make smoke-k6-local
```

### Run Shared Library Tests

```bash
# Run Go shared library tests
make shared-go-test

# Run TypeScript shared library tests
make shared-ts-test

# Run all shared library tests
make shared-test
```

### Run Web Portal Tests

```bash
cd web/portal

# Unit tests
pnpm test

# E2E tests (headless, no report server)
pnpm test:e2e

# E2E tests with existing server
SKIP_WEBSERVER=true PLAYWRIGHT_BASE_URL=http://localhost:5173 pnpm test:e2e

# E2E tests with visible browser
PLAYWRIGHT_HEADLESS=false pnpm test:e2e

# E2E tests against remote development deployment (bypasses local Playwright/Fosite issues)
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
API_ROUTER_URL=https://api.ai-aas.dev \
pnpm test:e2e

# All tests
pnpm test:all
```

**Note**: For testing against remote development cluster, see `tmp_md/REMOTE_DEV_DEPLOYMENT_GUIDE.md` for complete deployment and testing instructions.

## Related Documentation

- [Testing Guidelines](../.cursorrules#testing-guidelines)
- [Contributing Guide](../CONTRIBUTING.md#testing)
- [E2E Test Spec](../specs/012-e2e-tests/spec.md)
- [User-Org-Service E2E Testing](../services/user-org-service/docs/e2e-testing.md)
- [Web Portal Test Guide](../web/portal/tests/README.md)

