# E2E Testing Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-26 | **Commit**: 7c070674

---

## Domain

Location: `tests/e2e/`

E2E tests validate the full platform across all services against a live environment (development cluster).

---

## Test Tiers

```yaml
tiers:
  smoke:
    files: [smoke_test.go]
    duration: ~2 min
    purpose: Quick health validation
    run: "GOWORK=off go test -v ./suites/... -tags='smoke,e2e_tier'"

  nightly:
    files: [smoke_test.go, happy_path_test.go, auth_test.go, budget_test.go, resilience_test.go]
    duration: ~15 min
    purpose: Daily regression
    run: "GOWORK=off go test -v ./suites/... -tags='nightly,e2e_tier'"

  full:
    files: [all test files including audit, declarative, recipe_deploy]
    duration: ~30 min
    purpose: Weekly/pre-release
    run: "GOWORK=off go test -v ./suites/... -tags='full,e2e_tier'"
```

---

## Test Structure

```yaml
directory_layout:
  tests/e2e/:
    harness/:
      client.go: HTTP client with auth, retries, correlation IDs
      context.go: Test context with run ID, resource naming
      fixture_manager.go: Fixture lifecycle and cleanup
    fixtures/:
      organizations.go: Organization CRUD
      service_accounts.go: Service account CRUD
      api_keys.go: API key issuance (requires service account)
      users.go: User management
    suites/:
      smoke_test.go: Basic inference + usage
      happy_path_test.go: Full CRUD flow
      auth_test.go: Token validation, scopes
      budget_test.go: Limit enforcement
      resilience_test.go: Timeouts, retries
      audit_test.go: Audit log verification
      declarative_test.go: GitOps convergence
      recipe_deploy_test.go: Model deployment recipes
    utils/:
      correlation.go: Trace ID generation
```

---

## Fixture Patterns

```yaml
fixture_flow:
  api_key_creation:
    requires: [organization, service_account]
    sequence:
      1: orgFixture.Create(ctx, "")
      2: saFixture.Create(ctx, org.ID, "")
      3: apiKeyFixture.Create(ctx, org.ID, sa.ID, "", scopes)
    convenience: apiKeyFixture.CreateWithServiceAccount(ctx, org.ID, "", scopes)

  json_field_mapping:
    organization:
      - ID: "orgId" (not "id")
      - CreatedAt: "createdAt" (not "created_at")
    service_account:
      - ID: "serviceAccountId" (not "id")
      - OrganizationID: "orgId"
    api_key:
      - ID: "keyId" (not "id")
      - Key: "token" (not "key")

  cleanup:
    automatic: FixtureManager registers resources
    order: "Reverse creation order (API keys → service accounts → orgs)"
```

---

## API Routes

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
    note: "Requires service account - cannot create directly under org"

api_router:
  base: https://api.dev.otherjamesbrown.com
  inference: POST /v1/chat/completions

admin_api:
  base: https://admin-api.dev.otherjamesbrown.com
  models: GET /v1/registry/models
```

---

## Running Tests

```bash
# IMPORTANT: Must disable go.work for E2E tests
cd tests/e2e

# Smoke only
GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier" -timeout 5m

# Nightly
GOWORK=off go test -v ./suites/... -tags="nightly,e2e_tier" -timeout 20m

# Full
GOWORK=off go test -v ./suites/... -tags="full,e2e_tier" -timeout 45m

# Single test
GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier" -run TestSmokeInference
```

---

## Anti-patterns

```go
// WRONG: Create API key directly without service account
apiKey, err := apiKeyFixture.Create(ctx, org.ID, "", nil)  // Missing SA ID!

// CORRECT: Use service account flow
sa, _ := saFixture.Create(ctx, org.ID, "")
apiKey, _ := apiKeyFixture.Create(ctx, org.ID, sa.ID, "", nil)

// WRONG: Wrong JSON field names in fixtures
type Organization struct {
    ID string `json:"id"`  // API returns "orgId"!
}

// CORRECT: Match actual API response
type Organization struct {
    ID string `json:"orgId"`
}

// WRONG: Use apiKey.Secret
authHeader := "Bearer " + apiKey.Secret

// CORRECT: Use apiKey.Key (maps to "token" in response)
authHeader := "Bearer " + apiKey.Key

// WRONG: Run from repo root with go.work
go test ./tests/e2e/suites/...  // Fails with module error

// CORRECT: Disable go.work
cd tests/e2e && GOWORK=off go test ./suites/...
```

---

## Environment

```yaml
development_cluster:
  user_org: https://user-org.dev.otherjamesbrown.com
  api_router: https://api.dev.otherjamesbrown.com
  admin_api: https://admin-api.dev.otherjamesbrown.com

auth:
  master_key: Found in secrets/env/.env as MASTER_ADMIN_API_KEY
  test_keys: Created via fixtures during test run

models:
  available: Check admin-api /v1/registry/models
  commonly_used: gpt-oss-20b
```

---

## Sources

| Component | Location |
|-----------|----------|
| Harness | `tests/e2e/harness/` |
| Fixtures | `tests/e2e/fixtures/` |
| Test Suites | `tests/e2e/suites/` |
| Utils | `tests/e2e/utils/` |
| README | `tests/e2e/README.md` |

---

## Checklist

Before completing E2E test work:
- [ ] `GOWORK=off go build ./...` succeeds in tests/e2e
- [ ] Smoke tests pass: `GOWORK=off go test -v ./suites/... -tags="smoke,e2e_tier"`
- [ ] JSON struct tags match actual API responses
- [ ] Fixtures register resources for cleanup
- [ ] New fixtures follow service account → API key flow
