# Research: End-to-End Test Framework

**Feature**: `012-e2e-tests`  
**Date**: 2025-01-27  
**Status**: Draft

## Overview

This document captures research on test frameworks, patterns, and best practices for building a comprehensive end-to-end test harness for the AI-as-a-Service platform.

## Test Framework Options

### Option 1: Go Test with Custom Harness

**Pros**:
- Native Go testing framework (`testing` package)
- Good integration with existing Go services
- Simple test execution and CI integration
- Direct HTTP client usage
- Easy to extend with custom utilities

**Cons**:
- Limited built-in test reporting
- Manual test orchestration required
- Less mature ecosystem for complex scenarios

**Use Case**: Best for service-level e2e tests, especially for Go services.

**Example**:
```go
func TestOrganizationLifecycle(t *testing.T) {
    client := NewTestClient(t)
    org := client.CreateOrganization(t, "test-org")
    defer client.CleanupOrganization(t, org.ID)
    // ... test steps
}
```

### Option 2: Testcontainers

**Pros**:
- Isolated test environments with containers
- Real dependencies (databases, services)
- Deterministic test setup
- Good for integration testing

**Cons**:
- Slower test execution
- Requires Docker
- More complex setup
- Resource intensive

**Use Case**: Best for local testing with real dependencies, less suitable for CI against deployed services.

**Example**:
```go
func TestWithDatabase(t *testing.T) {
    postgres, _ := testcontainers.StartPostgresContainer(ctx)
    defer postgres.Terminate(ctx)
    // ... test with real database
}
```

### Option 3: Playwright / Cypress (Browser Testing)

**Pros**:
- Real browser automation
- Good for web portal testing
- Rich debugging tools
- Visual regression testing

**Cons**:
- Slower execution
- Requires browser runtime
- Not suitable for API-only testing
- More flaky due to browser behavior

**Use Case**: Best for web portal e2e tests, not suitable for API testing.

### Option 4: REST Assured / HTTP Client Libraries

**Pros**:
- Domain-specific language for API testing
- Built-in assertions
- Request/response logging
- Good documentation

**Cons**:
- Language-specific (Java for REST Assured)
- May require language bridge for Go services
- Less flexible than raw HTTP clients

**Use Case**: Best for API-focused testing in Java/Kotlin ecosystems.

### Option 5: Custom Test Harness (Recommended)

**Approach**: Build a custom test harness using Go's `testing` package with reusable utilities.

**Components**:
- Test client wrapper for HTTP requests
- Fixture management (orgs, users, keys)
- Test context and isolation
- Artifact collection
- Report generation (JUnit XML, JSON)

**Pros**:
- Tailored to platform needs
- Full control over test execution
- Easy integration with existing codebase
- Language consistency (Go)
- Customizable reporting and diagnostics

**Cons**:
- Requires development effort
- Need to maintain test utilities
- Less community support

## Test Patterns

### Fixture Pattern

**Purpose**: Create test data (orgs, users, keys) with predictable state.

**Implementation**:
```go
type FixtureManager struct {
    client *TestClient
    fixtures []Fixture
}

func (fm *FixtureManager) CreateOrganization(name string) *Organization {
    org := fm.client.CreateOrganization(name)
    fm.fixtures = append(fm.fixtures, Fixture{Type: "organization", ID: org.ID})
    return org
}

func (fm *FixtureManager) Cleanup() {
    for _, fixture := range fm.fixtures {
        fm.client.DeleteFixture(fixture)
    }
}
```

### Page Object Pattern (for API Testing)

**Purpose**: Encapsulate API interactions in reusable objects.

**Implementation**:
```go
type OrganizationAPI struct {
    client *TestClient
}

func (api *OrganizationAPI) Create(name string) (*Organization, error) {
    return api.client.POST("/v1/organizations", CreateOrgRequest{Name: name})
}

func (api *OrganizationAPI) Get(id string) (*Organization, error) {
    return api.client.GET("/v1/organizations/" + id)
}
```

### Test Context Pattern

**Purpose**: Share test state and configuration across test steps.

**Implementation**:
```go
type TestContext struct {
    RunID string
    Environment string
    Client *TestClient
    Fixtures *FixtureManager
    Artifacts *ArtifactCollector
}

func NewTestContext(t *testing.T) *TestContext {
    return &TestContext{
        RunID: generateRunID(),
        Environment: os.Getenv("TEST_ENV"),
        Client: NewTestClient(t),
        Fixtures: NewFixtureManager(),
        Artifacts: NewArtifactCollector(),
    }
}
```

### Retry Pattern

**Purpose**: Handle transient failures and timing issues.

**Implementation**:
```go
func Retry(t *testing.T, maxAttempts int, fn func() error) error {
    var lastErr error
    for i := 0; i < maxAttempts; i++ {
        if err := fn(); err == nil {
            return nil
        }
        lastErr = err
        time.Sleep(time.Duration(i+1) * time.Second)
    }
    return lastErr
}
```

## Test Isolation Strategies

### Namespace Isolation

**Approach**: Use unique prefixes for test resources.

**Example**:
```go
func GenerateTestResourceName(prefix string) string {
    timestamp := time.Now().Format("20060102-150405")
    runID := os.Getenv("TEST_RUN_ID")
    return fmt.Sprintf("e2e-%s-%s-%s", prefix, runID, timestamp)
}
```

### Database Isolation

**Approach**: Use separate databases or schemas per test run.

**Example**:
```go
func SetupTestDatabase(t *testing.T) *sql.DB {
    dbName := fmt.Sprintf("e2e_test_%s", generateRunID())
    db := createDatabase(dbName)
    t.Cleanup(func() {
        dropDatabase(dbName)
    })
    return db
}
```

### Parallel Execution Isolation

**Approach**: Use worker IDs and distinct namespaces.

**Example**:
```go
func TestParallel(t *testing.T) {
    t.Parallel()
    workerID := os.Getenv("TEST_WORKER_ID")
    namespace := fmt.Sprintf("e2e-worker-%s", workerID)
    // ... use namespace for all resources
}
```

## Test Reporting

### JUnit XML Format

**Purpose**: CI/CD integration (Jenkins, GitHub Actions, etc.).

**Format**:
```xml
<testsuites>
  <testsuite name="e2e-tests" tests="15" failures="1">
    <testcase name="TestOrganizationLifecycle" time="5.0"/>
    <testcase name="TestBudgetEnforcement" time="3.0">
      <failure message="Expected status 403, got 200"/>
    </testcase>
  </testsuite>
</testsuites>
```

### JSON Report Format

**Purpose**: Programmatic analysis and custom dashboards.

**Format**:
```json
{
  "test_run_id": "run-xyz",
  "summary": {
    "total": 15,
    "passed": 14,
    "failed": 1
  },
  "test_cases": [...]
}
```

## Test Execution Strategies

### Sequential Execution

**Pros**: Simple, no resource conflicts, easier debugging

**Cons**: Slower, doesn't utilize available resources

**Use Case**: Initial implementation, debugging, critical path tests

### Parallel Execution

**Pros**: Faster execution, better resource utilization

**Cons**: Requires careful isolation, potential resource conflicts

**Use Case**: Full test suite, CI/CD pipelines

**Implementation**:
```go
func TestSuite(t *testing.T) {
    workers := 4
    sem := make(chan struct{}, workers)
    var wg sync.WaitGroup
    
    for _, test := range tests {
        wg.Add(1)
        go func(test TestCase) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            runTest(t, test)
        }(test)
    }
    wg.Wait()
}
```

## Test Data Management

### Deterministic Test Data

**Approach**: Use predictable, timestamped identifiers.

**Example**:
```go
func GenerateTestEmail() string {
    timestamp := time.Now().Unix()
    return fmt.Sprintf("e2e-user-%d@test.example.com", timestamp)
}
```

### Test Data Cleanup

**Approach**: Tag resources with test run ID and cleanup after tests.

**Implementation**:
```go
func (tc *TestContext) Cleanup() {
    resources := tc.Client.ListResources(tc.RunID)
    for _, resource := range resources {
        tc.Client.DeleteResource(resource.ID)
    }
}
```

## Best Practices

### 1. Test Independence

- Each test should be independently runnable
- Tests should not depend on execution order
- Tests should clean up after themselves

### 2. Deterministic Tests

- Use fixed or predictable test data
- Avoid random values that affect test outcomes
- Use timestamps for uniqueness, not randomness

### 3. Clear Failure Messages

- Include request/response details in failures
- Include correlation IDs for debugging
- Include timestamps and durations

### 4. Configurable Timeouts

- Use environment-specific timeouts
- Allow longer timeouts for slower environments
- Fail fast on obvious errors

### 5. Artifact Collection

- Capture request/response bodies for failures
- Include correlation IDs in artifacts
- Store artifacts for post-test analysis

### 6. Test Organization

- Group related tests in test suites
- Use descriptive test names
- Document test prerequisites and assumptions

## Recommendations

### Recommended Approach

**Custom Go Test Harness** with the following components:

1. **Test Client**: HTTP client wrapper with logging and retries
2. **Fixture Manager**: Create and cleanup test resources
3. **Test Context**: Share state and configuration
4. **Artifact Collector**: Capture test artifacts
5. **Report Generator**: Generate JUnit XML and JSON reports

### Implementation Phases

1. **Phase 1**: Basic test harness with sequential execution
2. **Phase 2**: Add fixture management and cleanup
3. **Phase 3**: Add parallel execution support
4. **Phase 4**: Add artifact collection and reporting
5. **Phase 5**: Add advanced features (retries, timeouts, etc.)

### Technology Stack

- **Language**: Go (consistent with services)
- **Testing Framework**: `testing` package
- **HTTP Client**: `net/http` with custom wrapper
- **Reporting**: Custom JUnit XML and JSON generators
- **Configuration**: Environment variables and config files

## References

- [Go Testing Package](https://pkg.go.dev/testing)
- [Testcontainers Go](https://testcontainers.com/modules/go/)
- [JUnit XML Format](https://github.com/junit-team/junit5/blob/main/platform-tests/src/test/resources/jenkins-junit.xsd)
- [Testing Best Practices](https://testing.googleblog.com/)

