# Use Case Tests

This directory contains tests organized by use case feature areas. Each test file maps directly to a use case YAML file in `usecases/`.

## Purpose

Use case tests validate that the system behaves according to the acceptance criteria defined in each use case. These tests:

- **Provide traceability** - Each test maps to a specific UC and AC
- **Prevent scope drift** - Tests only what's defined in acceptance criteria
- **Enable coverage analysis** - `./scripts/uc-coverage.sh` reports which UCs have tests

## Test Naming Convention

### Test Functions

Test function names follow the pattern:

```
TestUC_{PREFIX}_{NNN}_{Title}
```

Where:
- `{PREFIX}` - Feature prefix (AUTH, USR, KEY, MDL, BM, AUD, USG, ORG)
- `{NNN}` - Three-digit use case number
- `{Title}` - Camel-case version of the UC title

Example:
```go
func TestUC_BM_001_CreateBenchmarkTarget(t *testing.T) {
    // ...
}
```

### Subtests (Acceptance Criteria)

Each acceptance criterion becomes a subtest:

```go
t.Run("AC-01: create target with required fields", func(t *testing.T) {
    // Given: authenticated org admin
    // When: create benchmark target
    // Then: target is created
})
```

The subtest name format is: `AC-{NN}: {criterion description}`

## Test Structure

Each test should follow the Given-When-Then pattern from the UC:

```go
func TestUC_BM_001_CreateBenchmarkTarget(t *testing.T) {
    t.Run("AC-01: create target with required fields", func(t *testing.T) {
        // Given: User is authenticated with org admin API key
        client := setupAuthenticatedClient(t)

        // When: User runs benchmark target add
        target, err := client.CreateBenchmarkTarget(ctx, &CreateTargetRequest{
            Model:    "llama-7b",
            Scenario: "standard",
        })

        // Then: Benchmark target is created
        require.NoError(t, err)
        assert.NotEmpty(t, target.ID)

        // Then: Target appears in list
        targets, err := client.ListBenchmarkTargets(ctx)
        require.NoError(t, err)
        assert.Contains(t, targetIDs(targets), target.ID)
    })

    t.Run("AC-02: reject unauthorized model", func(t *testing.T) {
        // Given: User specifies a model they don't have access to
        client := setupAuthenticatedClient(t)

        // When: User runs benchmark target add with restricted model
        _, err := client.CreateBenchmarkTarget(ctx, &CreateTargetRequest{
            Model:    "restricted-model",
            Scenario: "standard",
        })

        // Then: Command fails with auth error
        require.Error(t, err)
        assert.ErrorIs(t, err, ErrUnauthorizedModel)
    })
}
```

## File Organization

| File | UC Prefix | Use Case File |
|------|-----------|---------------|
| `auth_test.go` | UC-AUTH-* | `usecases/authentication.yaml` |
| `users_test.go` | UC-USR-* | `usecases/users.yaml` |
| `apikeys_test.go` | UC-KEY-* | `usecases/apikeys.yaml` |
| `models_test.go` | UC-MDL-* | `usecases/models.yaml` |
| `organization_test.go` | UC-ORG-* | `usecases/organization.yaml` |
| `usage_test.go` | UC-USG-* | `usecases/usage.yaml` |
| `audit_test.go` | UC-AUD-* | `usecases/audit.yaml` |
| `benchmarks_test.go` | UC-BM-* | `usecases/benchmarks.yaml` |
| `gitops_model_lifecycle_test.go` | UC-MLC-010, UC-MLC-011 | `usecases/model-lifecycle.yaml` |

## Special Test Categories

### GitOps Tests (UC-MLC-010, UC-MLC-011)

GitOps model lifecycle tests validate the end-to-end GitOps deployment flow. These tests require additional setup:

**Prerequisites:**
- `RUN_GITOPS_TESTS=1` - Enable GitOps tests (disabled by default)
- `AI_AAS_CONFIG_PATH` - Path to ai-aas-config repository (defaults to `~/ai-aas-config`)
- `KUBECONFIG` - Path to development cluster kubeconfig
- Git credentials configured for push access to ai-aas-config
- ArgoCD configured to sync from ai-aas-config develop branch

**Optional:**
- `KYVERNO_POLICY_DEPLOYED=1` - Enable Kyverno policy test (UC-MLC-010/AC-03)

**Running GitOps tests:**
```bash
# Run all GitOps tests
RUN_GITOPS_TESTS=1 \
AI_AAS_CONFIG_PATH=~/ai-aas-config \
KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml \
go test -v ./... -run "TestUC_MLC_01"

# Run with Kyverno policy test
RUN_GITOPS_TESTS=1 \
KYVERNO_POLICY_DEPLOYED=1 \
go test -v ./... -run "TestUC_MLC_010_AC03"
```

**What these tests do:**
1. Commit TinyLlama model config to ai-aas-config/environments/development/models/
2. Wait for ArgoCD to sync and create AIModel CR
3. Wait for model to reach Ready phase
4. Verify InferenceService and pod are created
5. Remove model config from ai-aas-config
6. Wait for ArgoCD to prune the AIModel CR
7. Verify all resources are cleaned up (InferenceService, pods, services)
8. Verify model is no longer accessible via inference endpoint

**Cleanup:**
Tests automatically clean up their model files on completion or failure using `t.Cleanup()`.

## Running Tests

### Using Makefile (Recommended)

```bash
cd tests/usecases

# Run all tests against development cluster
make test-dev

# Run all tests against staging cluster
make test-staging

# Run specific UC test
make test-uc UC=UC-ORG-001
```

### Manual Execution

```bash
# Run with explicit environment variables
AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com \
AI_AAS_API_KEY=<your-api-key> \
go test -v ./...

# Run tests for a specific feature
go test ./... -run "TestUC_BM_"

# Run a specific use case
go test ./... -run "TestUC_BM_001"

# Run a specific acceptance criterion
go test ./... -run "TestUC_BM_001/AC-01"
```

## Configuration

Tests automatically load credentials from `secrets/env/.env` via the testconfig loader:

| Source | Priority | Description |
|--------|----------|-------------|
| Environment variables | 1 (highest) | `AI_AAS_API_KEY`, `AI_AAS_API_ENDPOINT`, etc. |
| `secrets/env/.env` | 2 | Auto-loaded if env vars not set |

**Key environment variables:**
- `AI_AAS_API_ENDPOINT` - Admin API endpoint URL (e.g., `https://admin-api.dev.otherjamesbrown.com`)
- `AI_AAS_API_KEY` - API key (auto-loaded from `MASTER_ADMIN_API_KEY` in .env)
- `AI_AAS_ORG_ID` - Organization ID (auto-loaded from `MASTER_ADMIN_ORG_ID` in .env)
- `AI_AAS_API_ROUTER_URL` - API Router URL (e.g., `https://api.dev.otherjamesbrown.com`)

The testconfig loader (`shared/go/testconfig`) parses `secrets/env/.env` and:
1. Loads `MASTER_ADMIN_API_KEY` → sets `AI_AAS_API_KEY`
2. Loads `MASTER_ADMIN_ORG_ID` → sets `AI_AAS_ORG_ID`
3. Sets `AI_AAS_API_ENDPOINT` to development admin API (if not already set)
4. Sets `AI_AAS_API_ROUTER_URL` to development API router (if not already set)

This configuration is handled by `config_test.go` init() function and runs automatically when tests are loaded.

## Coverage Analysis

Use the coverage script to see which UCs have tests:

```bash
./scripts/uc-coverage.sh
```

This will output a report showing:
- UCs with full test coverage
- UCs with partial coverage (some ACs missing)
- UCs with no tests

## Related Documentation

- [usecases/SCHEMA.md](../../usecases/SCHEMA.md) - Use case YAML schema and examples
- [CLAUDE.md](../../CLAUDE.md) - Agent instructions for UC workflow
- [context/agents.md](../../context/agents.md) - Core agent rules

## Adding New Tests

When implementing a new UC:

1. **Read the use case file** to understand acceptance criteria
2. **Create stub test** with `t.Skip("TODO: Implement - see usecases/{feature}.yaml")`
3. **Implement each AC as a subtest** following Given-When-Then
4. **Run tests** to verify they pass
5. **Update coverage** with `./scripts/uc-coverage.sh`

When a UC YAML file is updated:
1. Check if new ACs were added
2. Add corresponding subtests
3. Update existing subtests if criteria changed
