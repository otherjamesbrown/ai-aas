# Test Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-30 | **Commit**: 51afbadb

---

## Domain Overview

The test-developer agent owns cross-service testing infrastructure:

| Directory | Purpose |
|-----------|---------|
| `tests/e2e/` | End-to-end tests against live cluster |
| `tests/usecases/` | Contract tests (CLI-to-API validation) |
| `tests/integration/` | Cross-service integration tests |

**NOT owned by test-developer** (use domain agent):
- `services/*/internal/*_test.go` - Unit tests (go-services-developer)
- `services/ai-aas-cli/tests/` - CLI unit tests (cli-developer)
- `services/ai-aas-org/tests/` - Org CLI unit tests (cli-developer)

---

## Test Architecture

```
tests/
├── e2e/                          # End-to-end tests
│   ├── harness/                  # Test infrastructure
│   │   ├── client.go            # HTTP client with auth, retries
│   │   ├── context.go           # Test context with run ID
│   │   └── fixture_manager.go   # Resource lifecycle
│   ├── fixtures/                 # Test data management
│   │   ├── organizations.go     # Org CRUD
│   │   ├── service_accounts.go  # SA CRUD
│   │   ├── api_keys.go          # API key issuance
│   │   └── users.go             # User management
│   ├── suites/                   # Test suites by tier
│   │   ├── smoke_test.go        # Quick health validation
│   │   ├── happy_path_test.go   # Full CRUD flows
│   │   ├── auth_test.go         # Authentication tests
│   │   └── analytics_export_test.go # Analytics tests
│   └── utils/                    # Shared utilities
├── usecases/                     # Contract tests
│   ├── helpers_test.go          # CLI execution helpers
│   ├── contract_test.go         # CLI-API contract validation
│   └── organization_test.go     # Use case tests
└── integration/                  # Integration tests
```

---

## Test Categories

### E2E Tests

Full platform validation against live development cluster.

```yaml
tiers:
  smoke:
    files: [smoke_test.go]
    duration: ~2 min
    purpose: Quick health validation
    command: GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier"

  nightly:
    files: [smoke_test.go, happy_path_test.go, auth_test.go, budget_test.go]
    duration: ~15 min
    purpose: Daily regression
    command: GOWORK=off go test -v ./suites/... -tags="nightly,e2e_tier"

  full:
    files: [all test files]
    duration: ~30 min
    purpose: Weekly/pre-release
    command: GOWORK=off go test -v ./suites/... -tags="full,e2e_tier"
```

### Contract Tests

Validate CLI can parse actual API responses. Prevents contract drift bugs.

**Purpose**: Catch mismatches where CLI expects one JSON format but API returns another.

```go
// Example: Verify CLI can parse list responses
func TestContract_ListAPIKeys_JSONParsing(t *testing.T) {
    skipIfNoLiveAPI(t)
    result := runOrgCLI("apikey", "list", "--json")
    if result.ExitCode != 0 {
        t.Fatalf("apikey list failed (contract mismatch?): %s", result.Output)
    }
    var keys []map[string]interface{}
    if err := json.Unmarshal([]byte(result.Output), &keys); err != nil {
        t.Fatalf("Failed to parse API keys array (contract mismatch?): %v", err)
    }
}
```

### Use Case Tests

Test acceptance criteria from use case specifications.

```go
// Test naming convention: TestUC_<ID>_<Name>
func TestUC_ORG_001_ShowOrganizationDetails(t *testing.T) {
    t.Run("AC-01: show organization details", func(t *testing.T) {
        // Given/When/Then structure
    })
}
```

---

## Patterns

### pattern: fixture_lifecycle

**Rule**: Always use FixtureManager for resource creation and cleanup.

```go
// CORRECT: Use fixture manager
fm := harness.NewFixtureManager(client)
defer fm.Cleanup(ctx)  // Automatic cleanup in reverse order

org, _ := fm.CreateOrganization(ctx, "test-org")
sa, _ := fm.CreateServiceAccount(ctx, org.ID, "test-sa")
apiKey, _ := fm.CreateAPIKey(ctx, org.ID, sa.ID, scopes)

// WRONG: Manual creation without cleanup
resp, _ := client.Post("/v1/orgs", body)
// Resources leak on test failure!
```

### pattern: api_key_flow

**Rule**: API keys require a service account. Cannot create directly under org.

```go
// CORRECT: Create service account first
sa, _ := saFixture.Create(ctx, org.ID, "")
apiKey, _ := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", scopes)

// CORRECT: Use convenience method
apiKey, _ := apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", scopes)

// WRONG: Try to create API key without service account
apiKey, _ := apiKeyFixture.Create(ctx, org.ID, "", nil)  // Missing SA ID!
```

### pattern: json_field_mapping

**Rule**: Use exact JSON field names from API responses.

```go
// API Response fields (from user-org-service)
type Organization struct {
    ID        string `json:"orgId"`      // NOT "id"
    CreatedAt string `json:"createdAt"`  // NOT "created_at"
}

type ServiceAccount struct {
    ID             string `json:"serviceAccountId"`  // NOT "id"
    OrganizationID string `json:"orgId"`
}

type APIKey struct {
    ID  string `json:"keyId"`   // NOT "id"
    Key string `json:"token"`   // NOT "key" or "secret"
}
```

### pattern: error_assertion

**Rule**: Use flexible substring matching for error messages.

```go
// CORRECT: Meaningful substring that exists in API response
testCases := []struct {
    name          string
    expectedError string
}{
    {
        name:          "invalid time range",
        expectedError: "must be after",  // Exists in actual response
    },
}

// Assertion
if !strings.Contains(strings.ToLower(bodyStr), strings.ToLower(tc.expectedError)) {
    t.Errorf("Expected error containing '%s', got: %s", tc.expectedError, bodyStr)
}
```

### pattern: skip_unimplemented

**Rule**: Skip tests for unimplemented features with tracking reference.

```go
// CORRECT: Skip with bead reference
func TestFutureFeature(t *testing.T) {
    t.Skip("Feature not implemented - see bead aas-xxxx")
}

// CORRECT: Conditional skip based on feature availability
func TestOptionalFeature(t *testing.T) {
    resp, err := client.Get(ctx, "/v1/optional-feature/health")
    if err != nil || resp.StatusCode == 404 {
        t.Skip("Optional feature not deployed")
    }
    // Continue with test...
}

// WRONG: Commit failing test
func TestFutureFeature(t *testing.T) {
    resp, _ := client.Post(ctx, "/v1/future-endpoint", body)
    require.Equal(t, 200, resp.StatusCode)  // Fails every CI run!
}

// WRONG: Comment out test (loses visibility)
// func TestFutureFeature(t *testing.T) { ... }
```

### pattern: gowork_disabled

**Rule**: E2E tests must run with GOWORK=off.

```bash
# CORRECT: Disable go.work
cd tests/e2e && GOWORK=off go test ./suites/...

# WRONG: Run from repo root
go test ./tests/e2e/suites/...  # Module error!
```

---

## Anti-patterns

### anti-pattern: exact_error_matching

```go
// WRONG: Exact error message (too brittle)
expectedError: "timeRange.end must be after timeRange.start"
// Breaks if API message changes

// CORRECT: Flexible substring
expectedError: "must be after"
```

### anti-pattern: missing_fixture_cleanup

```go
// WRONG: No cleanup on failure
org, _ := createOrg(ctx)
// If test fails here, org leaks!
doSomethingThatMightFail()
deleteOrg(ctx, org.ID)

// CORRECT: Use fixture manager with defer
fm := harness.NewFixtureManager(client)
defer fm.Cleanup(ctx)
org, _ := fm.CreateOrganization(ctx, "")
```

### anti-pattern: wrong_json_tags

```go
// WRONG: Assumed field names
type Organization struct {
    ID string `json:"id"`  // API returns "orgId"!
}

// CORRECT: Verify against actual API
type Organization struct {
    ID string `json:"orgId"`
}
```

### anti-pattern: hardcoded_endpoints

```go
// WRONG: Hardcoded URLs
client.Post("https://user-org.dev.otherjamesbrown.com/v1/orgs", body)

// CORRECT: Use environment-based configuration
client.Post(cfg.UserOrgBaseURL + "/v1/orgs", body)
```

### anti-pattern: missing_live_api_skip

```go
// WRONG: Test fails when live API not available
func TestContract_ListUsers(t *testing.T) {
    result := runOrgCLI("user", "list", "--json")  // Fails without API!
    // ...
}

// CORRECT: Skip when live API unavailable
func TestContract_ListUsers(t *testing.T) {
    skipIfNoLiveAPI(t)
    result := runOrgCLI("user", "list", "--json")
    // ...
}
```

---

## API Endpoints Reference

```yaml
user_org_service:
  base: https://user-org.dev.otherjamesbrown.com
  organizations:
    create: POST /v1/orgs
    get: GET /v1/orgs/{orgId}
    delete: DELETE /v1/orgs/{orgId}
  service_accounts:
    create: POST /v1/orgs/{orgId}/service-accounts
    delete: DELETE /v1/orgs/{orgId}/service-accounts/{serviceAccountId}
  api_keys:
    create: POST /v1/orgs/{orgId}/service-accounts/{serviceAccountId}/api-keys
    list_for_user: GET /v1/orgs/{orgId}/users/{userId}/api-keys
  users:
    create: POST /v1/orgs/{orgId}/users
    list: GET /v1/orgs/{orgId}/users
    get: GET /v1/orgs/{orgId}/users/{userId}

api_router:
  base: https://api.dev.otherjamesbrown.com
  inference: POST /v1/chat/completions

admin_api:
  base: https://admin-api.dev.otherjamesbrown.com
  models: GET /v1/registry/models

analytics_service:
  base: https://analytics.dev.otherjamesbrown.com
  export: POST /analytics/v1/orgs/{orgId}/exports
```

---

## Running Tests

```bash
# E2E smoke tests
cd tests/e2e
GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier" -timeout 5m

# E2E nightly tests
GOWORK=off go test -v ./suites/... -tags="nightly,e2e_tier" -timeout 20m

# E2E full suite
GOWORK=off go test -v ./suites/... -tags="full,e2e_tier" -timeout 45m

# Contract tests
cd tests/usecases
go test -v ./...

# Single test
GOWORK=off go test -v ./suites/... -run TestSpecificTest

# With live API (contract tests)
AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
AI_AAS_API_KEY=<key> \
go test -v ./...
```

---

## Handoff Triggers

| Trigger | Hand Off To |
|---------|-------------|
| API returns unexpected data | go-services-developer |
| CLI command fails | cli-developer |
| Test environment issues | infra-ops-manager |
| Complex test failure investigation | debugger |
| Unit test for specific service | Service's developer agent |

---

## Completion Checklist

Before completing test work:
- [ ] Tests pass locally with `GOWORK=off`
- [ ] JSON struct tags verified against actual API responses
- [ ] Fixtures register all resources for cleanup
- [ ] Error assertions use flexible substring matching
- [ ] Skipped tests reference tracking beads
- [ ] New fixtures follow service account flow
- [ ] Contract tests have `skipIfNoLiveAPI(t)` check
- [ ] Commits reference bead ID
- [ ] Bead closed with commit hash and summary

---

## Sources

| Component | Location |
|-----------|----------|
| E2E Harness | `tests/e2e/harness/` |
| E2E Fixtures | `tests/e2e/fixtures/` |
| E2E Suites | `tests/e2e/suites/` |
| Contract Tests | `tests/usecases/` |
| E2E README | `tests/e2e/README.md` |
| E2E Testing Context | `context/e2e-testing/agents.md` |
