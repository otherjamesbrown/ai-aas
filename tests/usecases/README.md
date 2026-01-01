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
| Environment variables | 1 (highest) | `AI_AAS_API_KEY`, `AI_AAS_API_ENDPOINT` |
| `secrets/env/.env` | 2 | Auto-loaded if env vars not set |

**Key environment variables:**
- `AI_AAS_API_ENDPOINT` - API endpoint URL (required, set by Makefile)
- `AI_AAS_API_KEY` - API key (auto-loaded from `MASTER_ADMIN_API_KEY` in .env)

The testconfig loader (`shared/go/testconfig`) parses `secrets/env/.env` and exports
credentials to environment variables for CLI subprocess execution.

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
