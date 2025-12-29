# Test Runner

Interactive guide to run tests across the AI-AAS platform. Choose your test scope and environment.

## Instructions

**CRITICAL: Use `AskUserQuestion` tool to guide the user through test selection.**

### Step 1: Ask Test Category

Use AskUserQuestion with header "Test Category":

| Option | Description | Duration |
|--------|-------------|----------|
| Unit Tests (Recommended) | Fast, isolated tests - no dependencies | 1-5 min |
| E2E / Smoke Tests | Full workflow tests against running services | 2-45 min |
| Integration Tests | Multi-component tests with external dependencies | 5-15 min |
| CLI Smoke Tests | CLI tool validation against environments | 2-5 min/env |
| Web Portal Tests | Frontend unit and E2E tests | 1-10 min |
| Performance / Benchmarks | Latency and throughput measurements | 1-5 min |
| Infrastructure Tests | Terraform and Kubernetes validation | 5-30 min |
| All Local Tests | Run all tests that don't require a cluster | 5-10 min |

### Step 2: Ask Service Scope (for Unit/Integration tests)

If user chose Unit Tests or Integration Tests, ask with header "Service":

| Option | Description |
|--------|-------------|
| All Services (Recommended) | Run across all services |
| api-router-service | Request routing and inference |
| admin-api-service | Admin operations and model registry |
| user-org-service | Users, orgs, API keys |
| analytics-service | Usage tracking and reporting |
| ai-model-operator | Kubernetes operator |
| Shared Libraries | shared/go and shared/ts |

### Step 3: Ask Environment (for E2E/Integration/CLI tests)

If user chose E2E, Integration, or CLI tests, ask with header "Environment":

| Option | Description |
|--------|-------------|
| Local (Recommended) | Run against localhost services |
| Development Cluster | Run against dev.otherjamesbrown.com |
| Staging Cluster | Run against staging environment |
| Both Dev + Staging | Run against both remote environments |

### Step 4: Create Test Run Bead

**CRITICAL: Before running tests, create a parent bead to track the test run.**

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

### Step 5: Execute Tests and Track Results

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

### Step 6: Finalize Test Run Bead

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

## Step 7: Present Results

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
