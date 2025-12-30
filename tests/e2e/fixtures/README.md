# E2E Fixtures Development Guide

This document explains how to maintain and extend E2E test fixtures.

## Overview

Fixtures in `tests/e2e/fixtures/` provide reusable test data creation and cleanup. They wrap service API endpoints to create test resources (organizations, users, API keys, etc.).

## Architecture

```
fixtures/
├── organizations.go     # Organization CRUD
├── service_accounts.go  # Service account management
├── api_keys.go          # API key issuance
├── users.go             # User management
├── budgets.go           # Budget configuration
└── benchmarks.go        # Benchmark data
```

## Source of Truth

**OpenAPI specifications are the source of truth for all endpoints.**

| Service | Spec Location |
|---------|---------------|
| user-org-service | `services/user-org-service/api/openapi.yaml` |
| admin-api-service | `services/admin-api-service/api/openapi.yaml` |
| analytics-service | `services/analytics-service/api/openapi.yaml` |

## Keeping Fixtures in Sync

### When Service Endpoints Change

1. **Check the OpenAPI spec** for the updated endpoint path and request/response schema
2. **Update the fixture** to match the new endpoint
3. **Run smoke tests** to verify: `GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier"`

### Anti-pattern: Hard-coded Paths

```go
// WRONG: Hard-coded path without checking spec
func (f *UserFixture) Create(ctx context.Context, orgID, name string) (*User, error) {
    resp, err := f.client.Post(ctx, "/v1/orgs/"+orgID+"/users", body)  // Is this still correct?
    // ...
}

// CORRECT: Use constants with reference to spec
const (
    // From services/user-org-service/api/openapi.yaml: POST /v1/orgs/{orgId}/users
    createUserPath = "/v1/orgs/%s/users"
)

func (f *UserFixture) Create(ctx context.Context, orgID, name string) (*User, error) {
    path := fmt.Sprintf(createUserPath, orgID)
    resp, err := f.client.Post(ctx, path, body)
    // ...
}
```

### Pattern: Match JSON Field Names Exactly

API responses use camelCase JSON keys. Fixtures must match exactly:

```go
// WRONG: Go-style field naming
type Organization struct {
    ID        string `json:"id"`          // API returns "orgId"!
    CreatedAt string `json:"created_at"`  // API returns "createdAt"!
}

// CORRECT: Match API response exactly
type Organization struct {
    ID        string `json:"orgId"`
    CreatedAt string `json:"createdAt"`
}
```

### Pattern: Service Account Flow for API Keys

API keys require a service account. Never try to create API keys directly under an organization:

```go
// WRONG: Skip service account
apiKey, err := apiKeyFixture.Create(ctx, orgID, "", scopes)

// CORRECT: Create service account first
sa, _ := saFixture.Create(ctx, orgID, "test-sa")
apiKey, _ := apiKeyFixture.Create(ctx, orgID, sa.ID, scopes)

// BEST: Use convenience method
apiKey, sa, _ := apiKeyFixture.CreateWithServiceAccount(ctx, orgID, scopes)
```

## Contract Validation

### Validating Fixtures Locally

```bash
cd tests/e2e

# Build all fixtures (catch compile errors)
GOWORK=off go build ./fixtures/...

# Run against development cluster
GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier" -run TestSmokeInference

# Check specific fixture
GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier" -run TestOrganizationCRUD
```

### Using Response Validation

```go
func (f *OrganizationFixture) Create(ctx context.Context, name string) (*Organization, error) {
    resp, err := f.client.Post(ctx, "/v1/orgs", body)
    if err != nil {
        return nil, err
    }

    // Validate expected fields are present
    var org Organization
    if err := json.Unmarshal(resp.Body, &org); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w (check OpenAPI spec for changes)")
    }

    // Fail fast if critical field is missing
    if org.ID == "" {
        return nil, fmt.Errorf("organization ID is empty - API response schema may have changed")
    }

    return &org, nil
}
```

## Adding New Fixtures

1. **Check OpenAPI spec** for the endpoint you need
2. **Create fixture file** following naming convention: `<resource>.go`
3. **Implement CRUD methods** matching the API
4. **Register with FixtureManager** for automatic cleanup
5. **Add tests** to verify the fixture works

### Template

```go
package fixtures

import (
    "context"
    "fmt"

    "github.com/ai-aas/tests/e2e/harness"
)

type NewResourceFixture struct {
    client  *harness.Client
    manager *FixtureManager
}

func NewNewResourceFixture(client *harness.Client, manager *FixtureManager) *NewResourceFixture {
    return &NewResourceFixture{client: client, manager: manager}
}

// Create creates a new resource
// Endpoint: POST /v1/new-resources (from services/xxx/api/openapi.yaml)
func (f *NewResourceFixture) Create(ctx context.Context, name string) (*NewResource, error) {
    body := map[string]interface{}{
        "name": name,
    }

    resp, err := f.client.Post(ctx, "/v1/new-resources", body)
    if err != nil {
        return nil, fmt.Errorf("failed to create new resource: %w", err)
    }

    var resource NewResource
    if err := json.Unmarshal(resp.Body, &resource); err != nil {
        return nil, fmt.Errorf("failed to parse response: %w", err)
    }

    // Register for cleanup
    f.manager.Register("new-resource", resource.ID, func() error {
        return f.Delete(ctx, resource.ID)
    })

    return &resource, nil
}
```

## Troubleshooting

### 404 Not Found Errors

**Check**: Is the endpoint path correct?
```bash
# Compare fixture path with OpenAPI spec
grep -A5 "path:" services/user-org-service/api/openapi.yaml | grep users
```

### JSON Unmarshal Errors

**Check**: Do struct field JSON tags match API response?
```bash
# Make API call and inspect actual response
curl -X POST https://user-org.dev.otherjamesbrown.com/v1/orgs \
  -H "Authorization: Bearer $API_KEY" \
  -d '{"name":"test"}' | jq
```

### Test Cleanup Failures

**Check**: Are resources being registered with FixtureManager?
```go
// Every Create should register for cleanup
f.manager.Register("resource-type", id, func() error {
    return f.Delete(ctx, id)
})
```

## Related Documentation

- [E2E Testing Context](../../../context/e2e-testing/agents.md)
- [E2E Test Suite README](../README.md)
- [user-org-service OpenAPI](../../../services/user-org-service/api/openapi.yaml)
