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
# Required for all UC tests
export AI_AAS_API_ENDPOINT=https://user-org.dev.otherjamesbrown.com
export AI_AAS_API_KEY=<your-admin-api-key>
export AI_AAS_ORG_ID=<optional-org-id>

# Required for GitOps model tests
export RUN_GITOPS_TESTS=1
export AI_AAS_CONFIG_PATH=/path/to/ai-aas-config
export KUBECONFIG=/path/to/kubeconfig-development.yaml
```

**GitOps Environment Variables:**

| Variable | Purpose | Example |
|----------|---------|---------|
| `RUN_GITOPS_TESTS` | Enable GitOps tests | `1` |
| `AI_AAS_CONFIG_PATH` | Path to ai-aas-config repo | `/home/dev/ai-aas-config` |
| `KUBECONFIG` | Path to kubeconfig file | `/home/dev/secrets/kubeconfigs/kubeconfig-development.yaml` |

**Why GitOps tests are opt-in:**
- Require access to ai-aas-config repository
- Require kubectl access to the cluster
- Commit and push to develop branch
- Take longer to run (wait for ArgoCD sync + model deployment)

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

#### GitOpsModelFixture

**Use this for creating test models via GitOps workflow** (commits to ai-aas-config).

```go
// Setup GitOps fixture (requires RUN_GITOPS_TESTS=1)
skipIfNoGitOpsConfig(t)

gitOpsConfig := GitOpsTestConfig{
    ConfigRepoPath: os.Getenv("AI_AAS_CONFIG_PATH"),
    Environment:    "development",
    Kubeconfig:     "/path/to/kubeconfig-development.yaml",
}

gitOpsFixture, err := NewGitOpsModelFixture(fm, gitOpsConfig)
require.NoError(t, err)

// Create a model deployment (commits to ai-aas-config)
deployed, err := gitOpsFixture.Create(ModelConfig{
    ModelName:    "test-tinyllama",
    ModelID:      "TinyLlama/TinyLlama-1.1B-Chat-v1.0",
    ExternalName: "tinyllama-chat",
    Runtime:      "vllm",
    ModelType:    "text",
    MinReplicas:  1,
    MaxReplicas:  1,
    GPUCount:     1,
    RuntimeArgs:  []string{"--max-model-len=2048"},
})
require.NoError(t, err)

// Wait for model to be ready (polls AIModel CRD status)
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
defer cancel()
err = gitOpsFixture.WaitForReady(ctx, deployed.ModelName)
require.NoError(t, err)

// Test uses the deployed model...

// Cleanup happens automatically via t.Cleanup()
// - Removes YAML file from ai-aas-config
// - Commits and pushes deletion
// - ArgoCD will reconcile and delete the AIModel resource
```

**How it works:**
1. Creates a model YAML in `ai-aas-config/environments/{env}/models/{model-name}.yaml`
2. Commits and pushes to `develop` branch
3. ArgoCD detects the change and creates the AIModel CRD
4. AI-AAS Operator reconciles the AIModel and creates the Kubernetes resources
5. Cleanup reverses the process (removes file, commits, ArgoCD reconciles)

**When to use:**
- Creating models for UC tests that need deployments
- Testing the full GitOps workflow
- Tests that need models to be deployed before running

#### PreExistingModelReference

**Use this for read-only tests against well-known models** (no creation/cleanup).

```go
// Get reference to a well-known model
modelRef, err := GetWellKnownModel("tinyllama")
require.NoError(t, err)

// Optionally verify it exists (recommended in test setup)
client := setupTestClient(t)
err = modelRef.VerifyModelExists(client)
require.NoError(t, err)

// Use the model for inference tests
resp := client.POST("/v1/chat/completions", map[string]interface{}{
    "model": modelRef.ModelName,
    "messages": []map[string]string{{"role": "user", "content": "Hello"}},
})
require.Equal(t, 200, resp.StatusCode)
```

**Well-known models** (must be pre-deployed):
- `"tinyllama"` → `tinyllama-1.1b-chat` in `development` namespace
- `"llama-3.1-8b"` → `llama-3.1-8b-instruct` in `system` namespace

**When to use:**
- Inference tests that just need a working model
- Tests that don't need to test deployment lifecycle
- Fast tests that want to skip deployment wait time

#### ModelDeploymentFixture (DEPRECATED)

**DEPRECATED**: Direct model deployment creation is blocked by Kyverno policy. Use `GitOpsModelFixture` instead.

```go
modelFixture := NewModelDeploymentFixture(fm, client)

// Create a deployment - NOW RETURNS ERROR
// ERROR: "direct model deployment creation is blocked by Kyverno policy; use GitOpsModelFixture instead"
deployment, err := modelFixture.Create("llama-7b", "development")

// Get a deployment (still works - read-only)
deployment, err := modelFixture.Get("llama-7b", "development")

// List deployments with filters (still works - read-only)
deployments, err := modelFixture.List("development", "")

// Delete - NOW RETURNS ERROR
// ERROR: "direct model deployment deletion is blocked by Kyverno policy; use GitOpsModelFixture instead"
err := modelFixture.Delete("llama-7b", "development")
```

**Migration path:**
- Replace `ModelDeploymentFixture.Create()` → `GitOpsModelFixture.Create()`
- Replace `ModelDeploymentFixture.Delete()` → automatic cleanup via `t.Cleanup()`
- Keep `ModelDeploymentFixture.Get()` and `List()` for read-only queries

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

## Migration Guide: ModelDeploymentFixture → GitOpsModelFixture

As of Phase 5 of the GitOps refactoring, direct model deployment creation/deletion is blocked by Kyverno policy. Tests must use the GitOps workflow instead.

### Before (Old Pattern - BROKEN)

```go
func TestModelDeployment_Old(t *testing.T) {
    skipIfNoLiveAPI(t)

    client, _ := NewTestClientFromEnv()
    fm := NewFixtureManager(t, client)

    modelFixture := NewModelDeploymentFixture(fm, client)

    // BROKEN: Returns error "blocked by Kyverno policy"
    deployment, err := modelFixture.Create("llama-7b", "development")
    require.NoError(t, err)

    // Test uses deployment...

    // BROKEN: Returns error "blocked by Kyverno policy"
    err = modelFixture.Delete("llama-7b", "development")
}
```

### After (New Pattern - GitOps)

```go
func TestModelDeployment_New(t *testing.T) {
    skipIfNoLiveAPI(t)
    skipIfNoGitOpsConfig(t)

    client, _ := NewTestClientFromEnv()
    fm := NewFixtureManager(t, client)

    // Setup GitOps fixture
    gitOpsConfig := GitOpsTestConfig{
        ConfigRepoPath: os.Getenv("AI_AAS_CONFIG_PATH"),
        Environment:    "development",
        Kubeconfig:     os.Getenv("KUBECONFIG"),
    }
    gitOpsFixture, err := NewGitOpsModelFixture(fm, gitOpsConfig)
    require.NoError(t, err)

    // Create model via GitOps
    deployed, err := gitOpsFixture.Create(ModelConfig{
        ModelName:   "test-llama-7b",
        ModelID:     "meta-llama/Llama-2-7b-hf",
        Runtime:     "vllm",
        GPUCount:    1,
        MinReplicas: 1,
        MaxReplicas: 1,
    })
    require.NoError(t, err)

    // Wait for deployment to be ready
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()
    err = gitOpsFixture.WaitForReady(ctx, deployed.ModelName)
    require.NoError(t, err)

    // Test uses deployment...

    // Cleanup happens automatically via t.Cleanup()
    // No manual deletion needed
}
```

### After (Alternative - Use Pre-Existing Models)

For tests that just need a working model and don't need to test deployment lifecycle:

```go
func TestInference_WithPreExistingModel(t *testing.T) {
    skipIfNoLiveAPI(t)

    client, _ := NewTestClientFromEnv()

    // Get reference to well-known model
    modelRef, err := GetWellKnownModel("tinyllama")
    require.NoError(t, err)

    // Verify model exists (optional but recommended)
    err = modelRef.VerifyModelExists(client)
    require.NoError(t, err)

    // Use model for inference tests
    resp, err := client.POST("/v1/chat/completions", map[string]interface{}{
        "model": modelRef.ModelName,
        "messages": []map[string]string{
            {"role": "user", "content": "Hello"},
        },
    })
    require.NoError(t, err)
    require.Equal(t, 200, resp.StatusCode)
}
```

### Migration Checklist

- [ ] Add `skipIfNoGitOpsConfig(t)` to tests that create models
- [ ] Replace `ModelDeploymentFixture.Create()` with `GitOpsModelFixture.Create()`
- [ ] Add `WaitForReady()` call after creating models
- [ ] Remove manual `Delete()` calls (automatic cleanup via `t.Cleanup()`)
- [ ] Set `RUN_GITOPS_TESTS=1` in your environment
- [ ] Set `AI_AAS_CONFIG_PATH` to your ai-aas-config repo path
- [ ] Ensure `KUBECONFIG` points to your development kubeconfig
- [ ] Consider using `PreExistingModelReference` for inference-only tests

### Decision Tree: Which Fixture to Use?

```
Do you need to CREATE a model deployment?
├─ NO → Use PreExistingModelReference
│       - Fastest option
│       - Read-only tests (inference, queries)
│       - No GitOps setup required
│
└─ YES → Do you need to test the GitOps deployment workflow?
    ├─ YES → Use GitOpsModelFixture
    │        - Tests deployment lifecycle
    │        - Tests GitOps integration
    │        - Requires RUN_GITOPS_TESTS=1
    │
    └─ NO → Consider redesigning test to use PreExistingModelReference
             - Most inference tests don't need deployment control
             - Faster test execution
             - Fewer dependencies
```

## Related Documentation

- [Use Case Schema](../../usecases/SCHEMA.md) - Use case YAML format
- [Contract Tests](./CONTRACT_TESTS.md) - CLI-to-API contract testing
- [E2E Tests](../e2e/README.md) - Full platform E2E tests
