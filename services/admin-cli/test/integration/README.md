# Integration Tests

Integration tests verify the admin-cli commands work correctly against running services (user-org-service, analytics-service).

## Running Tests

### Prerequisites

1. **User-org-service running** (for bootstrap, org, user tests):
   ```bash
   cd ../../user-org-service
   make run
   ```

2. **Analytics-service running** (for export tests):
   ```bash
   cd ../../analytics-service
   make run
   ```

### Running All Integration Tests

```bash
# From services/admin-cli directory
make integration-test
```

### Running Specific Test Files

```bash
# Bootstrap tests
go test -v ./test/integration -run TestBootstrap

# Org/user tests
go test -v ./test/integration -run TestOrg
```

### Configuring Endpoints

Tests use environment variables to configure service endpoints:

```bash
# Custom user-org-service endpoint
ADMIN_CLI_USER_ORG_ENDPOINT=http://custom:8081 make integration-test

# Custom analytics-service endpoint
ADMIN_CLI_ANALYTICS_ENDPOINT=http://custom:8084 make integration-test

# Both
ADMIN_CLI_USER_ORG_ENDPOINT=http://localhost:8081 \
ADMIN_CLI_ANALYTICS_ENDPOINT=http://localhost:8084 \
make integration-test
```

Default endpoints:
- User-org-service: `http://localhost:8081`
- Analytics-service: `http://localhost:8084`

## Test Behavior

- **Service Unavailable**: Tests skip gracefully if services are not running (useful for CI)
- **Health Checks**: Tests verify service health before running operations
- **JSON Output**: Tests verify structured JSON output from commands
- **Dry-run Mode**: Tests verify dry-run functionality where implemented

## Test Structure

- `bootstrap_test.go`: Bootstrap and health check tests
- `org_user_test.go`: Organization and user lifecycle tests
- `export_test.go`: Export command tests

## CI Integration

Integration tests are excluded in short mode (`go test -short`) to allow running unit tests quickly in CI without requiring running services.

