# UC Test Infrastructure

This document describes the infrastructure for running Use Case (UC) tests against multiple environments.

## Quick Start

```bash
# Run all UC tests against development
cd tests/usecases
make test-dev

# Run all UC tests against staging
make test-staging

# Run specific UC test
make test-uc UC=UC-ORG-001
```

## Configuration

### Environment Variables (Recommended)

Set these environment variables before running tests:

```bash
export AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com
export AI_AAS_API_KEY=<your-admin-api-key>
export AI_AAS_ORG_ID=<optional-org-id>
```

### Config File (Alternative)

Create `.ai-aas-test.yaml` from the template:

```bash
cp .ai-aas-test.yaml.template .ai-aas-test.yaml
# Edit .ai-aas-test.yaml with your credentials
```

Config file format:

```yaml
api_endpoint: https://user-org.dev.otherjamesbrown.com
api_key: your-admin-api-key-here
org_id: ""  # optional
```

**Note**: Config file is only used if environment variables are not set.

## Makefile Targets

| Target | Description | Environment |
|--------|-------------|-------------|
| `test-dev` | Run all UC tests | Development cluster |
| `test-staging` | Run all UC tests | Staging cluster |
| `test-uc UC=XXX` | Run specific UC test | Development cluster |
| `help` | Show help message | - |

### Examples

```bash
# Development environment
make test-dev

# Staging environment
make test-staging

# Specific UC test
make test-uc UC=UC-ORG-001
make test-uc UC=UC-AUTH-002

# Run specific test function
AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
AI_AAS_API_KEY=your-key \
go test -v -run TestUC_ORG_001
```

## Test Helpers

### CLI Helpers

Execute platform CLI commands in tests:

```go
func TestPlatformCLI(t *testing.T) {
    skipIfNoPlatformCLI(t)  // Skip if ai-aas-cli not available

    // Run ai-aas-cli command
    result := runPlatformCLI("model", "list", "--json")
    if result.ExitCode != 0 {
        t.Fatalf("CLI failed: %s", result.Output)
    }

    // With profile
    result = runPlatformCLIWithProfile("production", "org", "list")
}
```

### Org CLI Helpers (Existing)

Execute org CLI commands in tests:

```go
func TestOrgCLI(t *testing.T) {
    skipIfNoLiveAPI(t)  // Skip if live API not configured

    result := runOrgCLI("org", "list", "--json")
    if result.ExitCode != 0 {
        t.Fatalf("CLI failed: %s", result.Output)
    }
}
```

## Fixture Framework

The fixture framework provides automatic resource creation and cleanup for tests.

### Overview

```go
func TestWithFixtures(t *testing.T) {
    skipIfNoLiveAPI(t)

    // Create client and fixture manager
    client, _ := NewTestClientFromEnv()
    fm := NewFixtureManager(t, client)

    // Create fixtures - automatically cleaned up after test
    orgFixture := NewOrganizationFixture(fm, client)
    org, _ := orgFixture.Create("test-org")

    saFixture := NewServiceAccountFixture(fm, client)
    sa, _ := saFixture.Create(org.ID, "test-sa")

    apiKeyFixture := NewAPIKeyFixture(fm, client)
    apiKey, _ := apiKeyFixture.Create(org.ID, sa.ID, "test-key", nil)

    // Test uses the fixtures...

    // Cleanup happens automatically via t.Cleanup()
}
```

### Available Fixtures

#### OrganizationFixture

```go
orgFixture := NewOrganizationFixture(fm, client)

// Create organization
org, err := orgFixture.Create("my-org")

// Delete organization (usually not needed - automatic cleanup)
err := orgFixture.Delete(org.ID)
```

#### ServiceAccountFixture

```go
saFixture := NewServiceAccountFixture(fm, client)

// Create service account
sa, err := saFixture.Create(orgID, "my-sa")

// Delete service account (usually not needed - automatic cleanup)
err := saFixture.Delete(orgID, sa.ID)
```

#### APIKeyFixture

```go
apiKeyFixture := NewAPIKeyFixture(fm, client)

// Create API key (requires service account)
apiKey, err := apiKeyFixture.Create(orgID, saID, "my-key", []string{"inference:read"})

// Convenience: create service account + API key in one call
apiKey, err := apiKeyFixture.CreateWithServiceAccount(orgID, "my-key", nil)

// Delete API key (usually not needed - automatic cleanup)
err := apiKeyFixture.Delete(orgID, apiKey.ID)
```

#### ModelDeploymentFixture

```go
modelFixture := NewModelDeploymentFixture(fm, client)

// Create a deployment (registers for automatic cleanup)
deployment, err := modelFixture.Create("llama-7b", "development")

// Create with custom options
deployment, err := modelFixture.CreateWithOptions(CreateDeploymentRequest{
    ModelName:   "llama-7b",
    Environment: "development",
    GPUCount:    1,
    Replicas:    2,
})

// Get a deployment
deployment, err := modelFixture.Get("llama-7b", "development")

// List deployments with filters
deployments, err := modelFixture.List("development", "")

// Delete (usually not needed - automatic cleanup)
err := modelFixture.Delete("llama-7b", "development")
```

### Fixture Cleanup

Fixtures are automatically cleaned up when the test ends via `t.Cleanup()`:

- Cleanup happens in reverse order (last created, first deleted)
- Cleanup failures are logged but don't fail the test
- No manual cleanup needed in most cases

## HTTP Client

The `TestClient` provides HTTP methods for API testing:

```go
client, _ := NewTestClientFromEnv()

// GET request
resp, err := client.GET("/v1/orgs")

// POST request
resp, err := client.POST("/v1/orgs", map[string]interface{}{
    "name": "test-org",
    "slug": "test-org",
})

// PUT, DELETE, PATCH
resp, err := client.PUT("/v1/orgs/123", body)
resp, err := client.DELETE("/v1/orgs/123")
resp, err := client.PATCH("/v1/orgs/123", body)

// Parse JSON response
var orgs []Organization
err := resp.DecodeJSON(&orgs)
```

## Test Structure

### Use Case Test Pattern

```go
func TestUC_<ID>_<Name>(t *testing.T) {
    skipIfNoLiveAPI(t)

    // Setup
    client, _ := NewTestClientFromEnv()
    fm := NewFixtureManager(t, client)

    t.Run("AC-01: description", func(t *testing.T) {
        // Given
        orgFixture := NewOrganizationFixture(fm, client)
        org, _ := orgFixture.Create("")

        // When
        resp, _ := client.GET(fmt.Sprintf("/v1/orgs/%s", org.ID))

        // Then
        if resp.StatusCode != 200 {
            t.Errorf("Expected 200, got %d", resp.StatusCode)
        }
    })
}
```

## Secrets Management

API keys are loaded from `secrets/env/.env`:

- **Development**: `MASTER_ADMIN_API_KEY`
- **Staging**: `STAGING_MASTER_ADMIN_API_KEY`

The Makefile automatically loads the correct key based on the target.

## CI Integration

For CI environments, set environment variables before running tests:

```bash
# GitHub Actions example
- name: Run UC Tests
  env:
    AI_AAS_API_ENDPOINT: ${{ secrets.DEV_API_ENDPOINT }}
    AI_AAS_API_KEY: ${{ secrets.DEV_API_KEY }}
  run: |
    cd tests/usecases
    go test -v ./... -timeout 10m
```

## Troubleshooting

### Tests skip with "requires live API"

Set environment variables:

```bash
export AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com
export AI_AAS_API_KEY=<your-key>
```

### "Config file not found"

Either:
1. Set environment variables (recommended), or
2. Create `.ai-aas-test.yaml` from the template

### "ai-aas-cli not found"

For platform CLI tests, ensure `ai-aas-cli` is installed:

```bash
cd ../../services/ai-aas-cli
go install
```

Or use `skipIfNoPlatformCLI(t)` in tests.

### Cleanup failures

Fixture cleanup failures are logged but don't fail tests. Check logs for:
- Network connectivity issues
- Permission problems
- Resource already deleted

### "Organization with this slug already exists" (409 Conflict)

This error indicates stale test organizations from interrupted test runs.

**Cause**: When tests are interrupted (Ctrl+C, timeout, panic), the `t.Cleanup()` handlers don't run, leaving test organizations in the database.

**Solution 1: Shell script cleanup** (recommended for manual cleanup):

```bash
cd tests/usecases
export AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com
export AI_AAS_API_KEY=<your-admin-key>

# Dry run (list only)
./cleanup_test_orgs.sh

# Actually delete
./cleanup_test_orgs.sh --delete
```

**Solution 2: Go test cleanup** (for automated cleanup):

```bash
cd tests/usecases
AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
AI_AAS_API_KEY=<your-admin-key> \
go test -v -run TestCleanupStaleTestOrgs
```

**Prevention**: The test fixture framework now uses cryptographically random suffixes for organization slugs (timestamp + 6 random hex chars), making collisions extremely unlikely even across multiple test runs.

## Related Documentation

- [Use Case Schema](../../usecases/SCHEMA.md) - Use case YAML format
- [Contract Tests](./CONTRACT_TESTS.md) - CLI-to-API contract testing
- [E2E Tests](../e2e/README.md) - Full platform E2E tests
