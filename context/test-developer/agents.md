# Test Developer Context

> **Inherits**: context/agents.md | **Verified**: 2026-01-01 | **Commit**: phase-4-cleanup

---

## Domain Overview

The test-developer agent owns cross-service testing infrastructure:

| Directory | Purpose |
|-----------|---------|
| `tests/usecases/` | UC acceptance tests (CLI-based, requirement traceability) |
| `tests/integration/` | Cross-service integration tests |

**NOT owned by test-developer** (use domain agent):
- `services/*/internal/*_test.go` - Unit tests (go-services-developer)
- `services/ai-aas-cli/tests/` - CLI unit tests (cli-developer)
- `services/ai-aas-org/tests/` - Org CLI unit tests (cli-developer)

---

## Test Architecture

```
tests/
├── usecases/                     # UC acceptance tests
│   ├── Makefile                 # test-dev, test-staging, test-uc
│   ├── README.md                # Test infrastructure docs
│   │
│   │ # User Domain (UC-AUTH, UC-USR, UC-KEY, UC-ORG, UC-USG, UC-AUD, UC-BM, UC-MDL)
│   ├── auth_test.go             # UC-AUTH-* (bootstrap, config)
│   ├── users_test.go            # UC-USR-* (user management)
│   ├── apikeys_test.go          # UC-KEY-* (API key lifecycle)
│   ├── organization_test.go     # UC-ORG-* (org management)
│   ├── usage_test.go            # UC-USG-* (usage queries)
│   ├── audit_test.go            # UC-AUD-* (audit logs)
│   ├── benchmarks_test.go       # UC-BM-* (benchmarking)
│   ├── models_test.go           # UC-MDL-* (user model access)
│   │
│   │ # Platform Domain (UC-MLC, UC-RTG, UC-RCP, UC-PLH)
│   ├── model_lifecycle_test.go  # UC-MLC-* (model deployment)
│   ├── routing_test.go          # UC-RTG-* (traffic routing)
│   ├── recipes_test.go          # UC-RCP-* (deployment recipes)
│   ├── platform_health_test.go  # UC-PLH-* (platform status)
│   │
│   │ # Integration Domain (UC-INF, UC-RSL, UC-ANL)
│   ├── inference_flow_test.go   # UC-INF-* (end-to-end inference)
│   ├── resilience_test.go       # UC-RSL-* (rate limits, budgets)
│   ├── analytics_flow_test.go   # UC-ANL-* (analytics export)
│   │
│   │ # Infrastructure
│   ├── contract_test.go         # CLI-API contract validation
│   ├── helpers_test.go          # CLI execution helpers
│   ├── fixtures_test.go         # Test fixture management
│   ├── client_test.go           # HTTP client helpers
│   └── config_test.go           # Environment configuration
│
└── integration/                  # Cross-service integration tests
```

---

## Test Domains

### User Domain

Actor: Organization administrators using `ai-aas-org` CLI.

| UC Prefix | Area | Spec File |
|-----------|------|-----------|
| UC-AUTH | Authentication | usecases/authentication.yaml |
| UC-USR | User management | usecases/users.yaml |
| UC-KEY | API key lifecycle | usecases/apikeys.yaml |
| UC-ORG | Organization management | usecases/organization.yaml |
| UC-USG | Usage queries | usecases/usage.yaml |
| UC-AUD | Audit logging | usecases/audit.yaml |
| UC-BM | Benchmarking | usecases/benchmarks.yaml |
| UC-MDL | Model access (user) | usecases/models.yaml |

### Platform Domain

Actor: Platform operators using `ai-aas-cli`.

| UC Prefix | Area | Spec File |
|-----------|------|-----------|
| UC-MLC | Model lifecycle | usecases/model-lifecycle.yaml |
| UC-RTG | Traffic routing | usecases/routing.yaml |
| UC-RCP | Deployment recipes | usecases/recipes.yaml |
| UC-PLH | Platform health | usecases/platform-health.yaml |

### Integration Domain

Cross-service flows testing end-to-end behavior.

| UC Prefix | Area | Spec File |
|-----------|------|-----------|
| UC-INF | Inference flow | usecases/inference-flow.yaml |
| UC-RSL | Resilience | usecases/resilience.yaml |
| UC-ANL | Analytics flow | usecases/analytics-flow.yaml |

---

## Test Categories

### UC Acceptance Tests

Test acceptance criteria from use case specifications. Each test maps to a specific UC and AC.

```go
// Test naming convention: TestUC_<PREFIX>_<NUM>_<Name>
func TestUC_ORG_001_ShowOrganizationDetails(t *testing.T) {
    t.Run("AC-01: show organization details", func(t *testing.T) {
        // Given: authenticated org admin
        // When: run ai-aas-org org show
        // Then: organization details displayed
    })
}
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

---

## Patterns

### pattern: uc_test_naming

**Rule**: Test names must include UC ID and AC number.

```go
// CORRECT: Full UC traceability
func TestUC_KEY_002_CreateAPIKey(t *testing.T) {
    t.Run("AC-01: create key with required fields", func(t *testing.T) { ... })
    t.Run("AC-02: create key with expiration", func(t *testing.T) { ... })
}

// WRONG: No UC traceability
func TestCreateAPIKey(t *testing.T) { ... }
```

### pattern: cli_execution

**Rule**: Use helper functions for CLI execution.

```go
// CORRECT: Use CLI helpers
result := runOrgCLI("user", "list", "--json")
require.Equal(t, 0, result.ExitCode)

result := runPlatformCLI("model", "list")
require.Equal(t, 0, result.ExitCode)

// WRONG: Direct exec
cmd := exec.Command("ai-aas-org", "user", "list")
output, _ := cmd.Output()
```

### pattern: skip_unimplemented

**Rule**: Skip tests for unimplemented features with tracking reference.

```go
// CORRECT: Skip with bead reference
func TestUC_MLC_005_CanaryDeployment(t *testing.T) {
    t.Skip("Feature not implemented - see bead aas-xxxx")
}

// CORRECT: Conditional skip based on feature availability
func TestUC_PLH_003_GPUMetrics(t *testing.T) {
    result := runPlatformCLI("status", "--check-gpu")
    if result.ExitCode != 0 {
        t.Skip("GPU monitoring not enabled")
    }
    // Continue with test...
}
```

### pattern: json_field_mapping

**Rule**: Use exact JSON field names from API responses.

```go
// API Response fields (from user-org-service)
type Organization struct {
    ID        string `json:"orgId"`      // NOT "id"
    CreatedAt string `json:"createdAt"`  // NOT "created_at"
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
expectedError := "must be after"

// Assertion
if !strings.Contains(strings.ToLower(result.Output), strings.ToLower(expectedError)) {
    t.Errorf("Expected error containing '%s', got: %s", expectedError, result.Output)
}

// WRONG: Exact error message (too brittle)
expectedError := "timeRange.end must be after timeRange.start"
```

---

## Anti-patterns

### anti-pattern: missing_uc_traceability

```go
// WRONG: No UC reference
func TestUserCreation(t *testing.T) { ... }

// CORRECT: Full traceability
func TestUC_USR_002_CreateUser(t *testing.T) { ... }
```

### anti-pattern: api_level_tests

```go
// WRONG: Direct API testing (use CLI instead)
resp, _ := http.Post(apiURL+"/v1/orgs", "application/json", body)

// CORRECT: CLI-based testing
result := runOrgCLI("org", "create", "--name", "test-org")
```

### anti-pattern: missing_live_api_skip

```go
// WRONG: Test fails when live API not available
func TestContract_ListUsers(t *testing.T) {
    result := runOrgCLI("user", "list", "--json")  // Fails without API!
}

// CORRECT: Skip when live API unavailable
func TestContract_ListUsers(t *testing.T) {
    skipIfNoLiveAPI(t)
    result := runOrgCLI("user", "list", "--json")
}
```

---

## Running Tests

```bash
# Run all UC tests against development
cd tests/usecases
make test-dev

# Run all UC tests against staging
make test-staging

# Run specific UC tests
make test-uc UC=UC-ORG-001

# Run with verbose output
AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
AI_AAS_API_KEY=<key> \
go test -v ./... -run "TestUC_KEY"

# Run by domain
go test -v ./... -run "TestUC_(AUTH|USR|KEY|ORG)"      # User domain
go test -v ./... -run "TestUC_(MLC|RTG|RCP|PLH)"       # Platform domain
go test -v ./... -run "TestUC_(INF|RSL|ANL)"           # Integration domain
```

---

## CI Workflows

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `uc-tests.yml` | PR, push to main/develop | Run UC tests against development |
| `nightly-uc.yml` | Daily 2 AM UTC | Run all UC tests against dev and staging |

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
- [ ] Tests use UC naming convention (TestUC_XXX_NNN)
- [ ] Each t.Run maps to an AC from the spec
- [ ] JSON struct tags verified against actual API responses
- [ ] Error assertions use flexible substring matching
- [ ] Skipped tests reference tracking beads
- [ ] Contract tests have `skipIfNoLiveAPI(t)` check
- [ ] Commits reference bead ID
- [ ] Bead closed with commit hash and summary

---

## Sources

| Component | Location |
|-----------|----------|
| UC Test Infrastructure | `tests/usecases/` |
| UC Specifications | `usecases/` |
| UC Schema | `usecases/SCHEMA.md` |
| CI Workflows | `.github/workflows/uc-tests.yml`, `.github/workflows/nightly-uc.yml` |
