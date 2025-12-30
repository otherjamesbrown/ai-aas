# Test Runner

Interactive guide to run tests across the AI-AAS platform. Choose your test scope and environment.

## Instructions

**CRITICAL: Use `AskUserQuestion` tool to guide the user through test selection.**

### Step 1: Ask What to Do

Use AskUserQuestion with header "Action":

| Option | Description |
|--------|-------------|
| Run tests | Execute unit, E2E, integration, or other tests |
| Run UC tests | Run use case acceptance criteria tests |
| View open failures | Show test runs with unresolved failures |
| View UC coverage | Show use case test coverage report |

---

### If "View open failures" Selected

Show all test run beads that have open child beads (unresolved failures):

```bash
# Get test run beads with open child issues
bd list --label test-run
```

Then for each test run bead, check if it has open blocked issues:
```bash
bd show <test-run-bead-id>
```

**Present results in this format:**

#### Open Test Failures by Test Run

| Test Run | Date | Commit | Open Failures |
|----------|------|--------|---------------|
| aas-xxxx | 2025-12-29 | abc1234 | 3 |
| aas-yyyy | 2025-12-28 | def5678 | 1 |

**Failure Details:**

For each test run with failures, list the child beads:

**aas-xxxx** (E2E full @ abc1234):
| Failure Bead | Test Name | Priority |
|--------------|-----------|----------|
| aas-aaaa | TestAuthorizationDenial | P1 |
| aas-bbbb | TestStreamingCompletion | P2 |

**Quick Actions:**
- `bd show <bead-id>` - View failure details
- `bd close <bead-id> --reason="Fixed in commit..."` - Close resolved failure
- `/test-runner` - Run tests to verify fixes

**Also show standalone test failures** (not linked to a test run):
```bash
bd list --status open --label test-failure
bd list --status open --label e2e-failure
```

---

### If "View UC coverage" Selected

Run the UC coverage script and present results:

```bash
./scripts/uc-coverage.sh
```

**Present results in this format:**

#### Use Case Test Coverage Report

| Use Case | Title | ACs | Covered | Coverage |
|----------|-------|-----|---------|----------|
| UC-BM-001 | Create Benchmark Target | 4 | 4 | 100% |
| UC-BM-002 | Trigger Benchmark Run | 3 | 2 | 67% |
| UC-USR-001 | List Organization Users | 3 | 3 | 100% |

**Summary:**
- Total Use Cases: X
- Total Acceptance Criteria: Y
- Covered: Z (N%)

**Missing Coverage** (if any):

| Use Case | AC | Criterion | Status |
|----------|-----|-----------|--------|
| UC-BM-002 | AC-03 | Cancel running benchmark | NO TEST |

**Next Steps:**
- Run `/test-runner` → "Run UC tests" to execute covered tests
- Create missing tests in `tests/usecases/` following naming convention
- See `usecases/SCHEMA.md` for AC definitions

---

### If "Run UC tests" Selected

Use AskUserQuestion with header "UC Scope":

| Option | Description |
|--------|-------------|
| All UC tests | Run all use case acceptance criteria tests |
| Specific UC | Run tests for a single use case (e.g., UC-BM-001) |
| By feature | Run tests for a feature area (e.g., benchmarks, users) |

**For "All UC tests"**: Run immediately:
```bash
cd tests/usecases && go test -v -count=1 ./...
```

**For "Specific UC"**: Ask for the UC ID, then run:
```bash
# Example: UC-BM-001
cd tests/usecases && go test -v -count=1 -run "TestUC_BM_001" ./...
```

**For "By feature"**: Ask which feature:

| Feature | Test Pattern | UC File |
|---------|--------------|---------|
| Authentication | `TestUC_AUTH` | `usecases/authentication.yaml` |
| Users | `TestUC_USR` | `usecases/users.yaml` |
| API Keys | `TestUC_KEY` | `usecases/apikeys.yaml` |
| Models | `TestUC_MDL` | `usecases/models.yaml` |
| Organization | `TestUC_ORG` | `usecases/organization.yaml` |
| Usage | `TestUC_USG` | `usecases/usage.yaml` |
| Audit | `TestUC_AUD` | `usecases/audit.yaml` |
| Benchmarks | `TestUC_BM` | `usecases/benchmarks.yaml` |

Then run:
```bash
cd tests/usecases && go test -v -count=1 -run "TestUC_<PREFIX>" ./...
```

**Important**: UC tests require a live API. Tests will skip if `AI_AAS_API_ENDPOINT` is not set.

To run against development cluster:
```bash
export AI_AAS_API_ENDPOINT="https://api.dev.otherjamesbrown.com"
export AI_AAS_API_KEY="<your-api-key>"
cd tests/usecases && go test -v -count=1 ./...
```

---

### If "Run tests" Selected

Use AskUserQuestion with header "Test":

| Option | Description | Duration |
|--------|-------------|----------|
| Unit tests (local) | Fast, isolated tests - all services, no dependencies | 1-5 min |
| E2E smoke (develop) | Quick smoke tests against development cluster | 2-5 min |
| E2E full (develop) | Complete E2E suite against development cluster | 10-30 min |
| Other | Integration, CLI, web, infra, or custom tests | varies |

### Step 2: Handle Selection

**For "Unit tests (local)"**: Run immediately:
```bash
cd /home/dev/worktrees/test-updates && make test SERVICE=all
```

**For "E2E smoke (develop)"**: Run immediately:
```bash
cd /home/dev/worktrees/test-updates/tests/e2e && make test-dev-ip TEST_PATTERN=TestSmoke
```

**For "E2E full (develop)"**: Run immediately:
```bash
cd /home/dev/worktrees/test-updates/tests/e2e && make test-dev-ip
```

**For "Other"**: Ask a follow-up question with header "Test Type":

| Option | Description |
|--------|-------------|
| Integration Tests | Multi-component tests with external dependencies |
| CLI Smoke Tests | CLI tool validation against environments |
| Web Portal Tests | Frontend unit and E2E tests |
| Performance / Benchmarks | Latency and throughput measurements |
| Infrastructure Tests | Terraform and Kubernetes validation |
| All Local Tests | Run all tests that don't require a cluster |
| Custom (specify) | User will describe what they want |

Then if needed, ask for service scope or environment (see reference tables below).

### Reference: Service Scope (for targeted tests)

| Option | Description |
|--------|-------------|
| All Services (Recommended) | Run across all services |
| api-router-service | Request routing and inference |
| admin-api-service | Admin operations and model registry |
| user-org-service | Users, orgs, API keys |
| analytics-service | Usage tracking and reporting |
| ai-model-operator | Kubernetes operator |
| Shared Libraries | shared/go and shared/ts |

### Reference: Environment (for E2E/Integration/CLI tests)

| Option | Description |
|--------|-------------|
| Local | Run against localhost services |
| Development Cluster | Run against dev.otherjamesbrown.com |
| Staging Cluster | Run against staging environment |
| Both Dev + Staging | Run against both remote environments |

### Step 3: Create Test Run Bead (Optional)

**For quick tests, skip this step. For longer runs or CI, create a tracking bead:**

```bash
# Get current git commit
COMMIT=$(git rev-parse --short HEAD)

# Create parent bead with labels
bd create --title="Test Run: <category> (<environment>) @ $COMMIT" \
  --type=task \
  --priority=3 \
  --label=test-run \
  --description="Test run tracking bead

**Category**: <category>
**Environment**: <environment>
**Commit**: $COMMIT
**Started**: $(date -Iseconds)

## Test Results
| Test | Status | Duration |
|------|--------|----------|
| (running...) | | |
"
```

Store the parent bead ID (e.g., `aas-xxxx`) for updating during the run.

### Step 4: Execute Tests and Track Results

**IMPORTANT**: Run the appropriate command and parse output to track each test.

For each test result:
1. **Parse test output** for test name, pass/fail, duration
2. **Update parent bead description** with results table
3. **For failures**, create child bead:

```bash
# Create failure bead
bd create --title="FAIL: <TestName>" \
  --type=bug \
  --priority=<P1 for smoke/critical, P2 for others> \
  --label=test-failure \
  --description="Test failure from test run aas-<parent>

**Test**: <TestName>
**Category**: <category>
**Environment**: <environment>
**Commit**: $COMMIT

## Error Output
\`\`\`
<error message>
\`\`\`

## Suggested Investigation
- Check logs: <relevant log query>
- Related code: <file:line if available>
"

# Link failure to parent (parent blocks failure)
bd dep add <failure-bead> <parent-bead>
```

#### UC-Aware Failure Tracking

**IMPORTANT**: If the test name follows UC naming convention (`TestUC_XXX_NNN`), extract the UC ID and add context:

1. **Extract UC ID from test name**:
   - `TestUC_BM_001_CreateBenchmarkTarget` → `UC-BM-001`
   - `TestUC_USR_003_ShowUserDetails` → `UC-USR-003`

2. **Add UC label to failure bead**:
```bash
bd label add <failure-bead> uc:UC-BM-001
```

3. **Include UC context in description**:
```bash
# Read the UC definition
yq eval '.usecases[] | select(.id == "UC-BM-001")' usecases/benchmarks.yaml
```

**Enhanced failure bead for UC tests**:

```bash
bd create --title="FAIL: TestUC_BM_001_CreateBenchmarkTarget/AC-01" \
  --type=bug \
  --priority=2 \
  --label=test-failure \
  --label=uc:UC-BM-001 \
  --description="Use case test failure from test run aas-<parent>

**Test**: TestUC_BM_001_CreateBenchmarkTarget
**Subtest**: AC-01: create target with required fields
**Use Case**: UC-BM-001 - Create Benchmark Target
**Category**: UC Tests
**Environment**: <environment>
**Commit**: $COMMIT

## Use Case Context

**Acceptance Criteria Violated**:
- AC-01: Given an authenticated org admin, when they create a benchmark target with model and scenario, then the target is created with a unique ID

**Scope Boundaries**:
- in_scope: Creating target configurations, validation
- out_of_scope: Starting execution, modifying targets
- must_not: Auto-start benchmark, modify existing targets

## Error Output
\`\`\`
<error message>
\`\`\`

## Suggested Investigation
- Review UC definition: \`cat usecases/benchmarks.yaml\`
- Check if implementation matches AC requirements
- Verify must_not constraints aren't violated
"
```

This creates an audit trail linking test failures back to their requirements, enabling:
- `bd list --label uc:UC-BM-001 --type bug` - All bugs for a UC
- Pattern analysis: Which UCs have recurring failures?
- AC gap detection: Failures may reveal missing acceptance criteria

### Step 5: Finalize Test Run Bead

After all tests complete:

1. **Update parent bead** with final summary:
   - Total tests, passed, failed, skipped
   - Total duration
   - List of failure bead IDs (if any)

2. **Close or leave open**:
   - **All passed**: Close the parent bead with reason "All X tests passed in Xs"
   - **Any failures**: Leave open, update status to show "X failures - see blocked issues"

```bash
# If all passed
bd close <parent-bead> --reason="All <N> tests passed in <duration>"

# If failures, update description with summary (don't close)
# The "Blocks:" field will automatically show linked failure beads
```

### Priority Inference for Failures

| Test Category | Priority | Rationale |
|--------------|----------|-----------|
| Smoke Tests | P1 | Critical path, blocks deployment |
| E2E Critical Path | P1 | Core functionality broken |
| Unit Tests | P2 | Isolated failures |
| Integration Tests | P2 | Cross-service issues |
| Performance Tests | P3 | Non-blocking regressions |
| Infrastructure Tests | P1 | Deployment blockers |

---

## Test Commands Reference

### Unit Tests

```bash
# All services
make test SERVICE=all

# Specific service
make test SERVICE=api-router-service
make test SERVICE=admin-api-service
make test SERVICE=user-org-service
make test SERVICE=analytics-service

# Specific package (more targeted)
cd services/api-router-service && go test -v ./internal/router/...
cd services/user-org-service && go test -v ./internal/server/...

# Shared libraries
make shared-go-test      # Go shared libs (80% coverage target)
make shared-ts-test      # TypeScript shared libs
make shared-test         # All shared libs
```

**Requirements**: Local only, no external dependencies
**Duration**: 1-5 minutes

---

### E2E / Smoke Tests

```bash
# Smoke tests (fastest, ~2 min)
cd tests/e2e && go test -v ./suites -run TestSmoke -timeout 10m

# Happy path tests
cd tests/e2e && go test -v ./suites -run TestHappyPath -timeout 15m

# Full E2E suite (local)
cd tests/e2e && make test-local

# E2E against development cluster (via public ingress)
cd tests/e2e && make test-dev-internet

# E2E against development cluster (via port-forwarding)
cd tests/e2e && make test-dev-remote

# Test tiers (using build tags)
cd tests/e2e && go test -v ./suites -tags="smoke,e2e_tier" -timeout 10m      # Quick
cd tests/e2e && go test -v ./suites -tags="nightly,e2e_tier" -timeout 20m    # Daily
cd tests/e2e && go test -v ./suites -tags="full,e2e_tier" -timeout 45m       # Complete
```

**Requirements**: Running services (local or cluster)
**Duration**: 2-45 minutes depending on tier
**Setup**: See `tests/e2e/SETUP.md` for initial configuration

---

### Integration Tests

```bash
# API Router integration tests
cd services/api-router-service && go test -v ./test/integration/...

# Analytics reconciliation tests
make analytics-verify

# Cross-service integration (requires Docker Compose)
cd tests/go/integration && go test -v ./...

# TypeScript integration
cd tests/ts/integration && pnpm test
```

**Requirements**: Docker Compose or running services
**Duration**: 5-15 minutes

---

### CLI Smoke Tests

```bash
# Both environments (recommended)
./scripts/cli-smoke-test.sh

# Development only
./scripts/cli-smoke-test.sh --dev-only

# Staging only
./scripts/cli-smoke-test.sh --staging-only

# Verbose output (shows models, Q&A responses)
./scripts/cli-smoke-test.sh --verbose
./scripts/cli-smoke-test.sh --dev-only --verbose
```

**Requirements**: Network access to target environment
**Duration**: 2-5 minutes per environment

---

### Web Portal Tests

```bash
cd web/portal

# Unit tests (Jest + Testing Library)
pnpm test

# E2E tests (Playwright, headless)
pnpm test:e2e

# E2E tests with visible browser
PLAYWRIGHT_HEADLESS=false pnpm test:e2e

# E2E tests with Playwright UI mode
pnpm test:e2e:ui

# E2E against existing dev server
SKIP_WEBSERVER=true PLAYWRIGHT_BASE_URL=http://localhost:5173 pnpm test:e2e

# E2E against remote dev cluster
PLAYWRIGHT_BASE_URL=https://portal.dev.otherjamesbrown.com \
SKIP_WEBSERVER=true \
API_ROUTER_URL=https://api.dev.otherjamesbrown.com \
pnpm test:e2e

# Accessibility tests
pnpm test:a11y

# All web tests
pnpm test:all
```

**Requirements**:
- Unit tests: Local only
- E2E tests: Running web server + API services
**Duration**: 1-10 minutes

---

### Performance / Benchmarks

```bash
# Go benchmarks (shared libs)
go test -bench=. -benchmem ./tests/go/perf/

# Specific benchmark
go test -bench=BenchmarkRequestContextMiddleware ./tests/go/perf/

# Analytics benchmarks
cd tests/analytics && go test -bench=. ./perf/

# CLI help command latency
cd tests/perf && ./measure_help.sh

# Infrastructure performance
cd tests/infra/perf && go test -v ./...
```

**Requirements**: Mostly local; infra benchmarks may need cluster
**Duration**: 1-5 minutes

---

### Infrastructure Tests

```bash
# Terraform validation (Terratest)
cd tests/infra/terratest && go test -v ./...

# vLLM deployment tests (requires GPU cluster)
export KUBECONFIG=/home/dev/secrets/kubeconfigs/kubeconfig-development.yaml
export RUN_VLLM_E2E_TESTS=1
go test -v ./tests/infra/vllm -run TestVLLMDeploymentE2E

# vLLM completion endpoint
go test -v ./tests/infra/vllm -run TestCompletionEndpoint
```

**Requirements**: Kubernetes cluster (GPU nodes for vLLM tests)
**Duration**: 5-30 minutes

---

### All Local Tests (No Cluster)

```bash
# Full local test suite
make check SERVICE=all          # fmt + lint + security + test
make shared-check               # Shared libraries full check

# Or individually:
make test SERVICE=all           # Unit tests
make shared-go-test             # Shared Go libs
make shared-ts-test             # Shared TS libs
cd web/portal && pnpm test      # Web unit tests
```

**Requirements**: Local only
**Duration**: 5-10 minutes

---

## Quick Reference: Local vs Cluster

| Test Type | Local | Docker Compose | Dev Cluster | Staging |
|-----------|-------|----------------|-------------|---------|
| Unit Tests | Yes | - | - | - |
| Shared Libs | Yes | - | - | - |
| Web Unit | Yes | - | - | - |
| Integration | Partial | Yes | Yes | Yes |
| E2E Smoke | With services | Yes | Yes | Yes |
| E2E Full | - | Yes | Yes | Yes |
| CLI Smoke | - | - | Yes | Yes |
| Web E2E | With services | - | Yes | Yes |
| Performance | Yes | - | Optional | - |
| Infrastructure | - | - | Yes | Yes |
| vLLM E2E | - | - | Yes (GPU) | - |

---

## Environment Setup

### Local Development Stack
```bash
make up                # Start local services via Docker Compose
make diagnose          # Check local setup
```

### Cluster Access
```bash
# Development
export KUBECONFIG=/home/dev/secrets/kubeconfigs/kubeconfig-development.yaml

# Staging
export KUBECONFIG=/home/dev/secrets/kubeconfigs/kubeconfig-staging.yaml
```

### E2E Test Configuration
See `tests/e2e/SETUP.md` for:
- Setting up `.admin-key.env`
- Configuring test endpoints
- Database URLs

---

## Step 6: Present Results

After running tests, present results in this format:

### Test Results Summary

| Metric | Value |
|--------|-------|
| **Test Run Bead** | aas-xxxx |
| **Category** | [Category] |
| **Environment** | [Environment] |
| **Commit** | [short hash] |
| **Duration** | Xs |

| Status | Count |
|--------|-------|
| Passed | X |
| Failed | X |
| Skipped | X |
| **Total** | X |

### Failed Tests (if any)

| Test | Failure Bead | Error Summary |
|------|--------------|---------------|
| TestFoo | aas-yyyy | Connection refused |
| TestBar | aas-zzzz | Assertion failed |

### Next Steps

- **All passed**: "All tests passed. Test run bead aas-xxxx closed."
- **Failures**: "X failures tracked. See `bd show aas-xxxx` for blocked issues. Run `bd list --label test-failure --status open` to see all open test failures."

---

## Troubleshooting

### Git-Crypt Issues
```bash
git crypt unlock ~/.config/git-crypt/ai-aas-key
```

### Missing Dependencies
```bash
make diagnose          # Check what's missing
make deps             # Install dependencies
```

### E2E Test Setup
```bash
cat tests/e2e/SETUP.md           # Initial setup
cat tests/e2e/TROUBLESHOOTING.md # Common issues
```

### Check for Existing Test Failures
```bash
bd list --status open --label test-failure    # Open test failures
bd list --label test-run                       # Recent test runs
bd show <bead-id>                              # See blocked failures
```

---

## Closing Failure Beads

When fixing a test failure, close the bead with reference to the fix:

```bash
bd close <failure-bead> --reason="Fixed in commit abc1234: <description>"
```

This maintains an audit trail from failure → fix.
