# CI Scripts

This directory contains scripts used by GitHub Actions CI/CD workflows.

## Scripts

### check-go-modules.sh

Verifies Go module path consistency across the repository to prevent module path mismatch bugs.

**Purpose**: Ensures all Go modules follow the correct naming conventions and import patterns. This prevents build failures caused by incorrect module paths or import statements.

**What it checks**:
1. **Module declarations** in `go.mod` files match expected patterns:
   - Services: `github.com/otherjamesbrown/ai-aas/services/<service-name>`
   - Shared: `github.com/ai-aas/shared-go`
   - Operators: `github.com/ai-aas/<operator-name>`

2. **Import statements** in `.go` files use correct module paths:
   - Services should import shared as `github.com/ai-aas/shared-go`
   - NOT `github.com/otherjamesbrown/ai-aas/shared/go`

3. **Replace directives** in service `go.mod` files:
   - Services importing shared must have: `replace github.com/ai-aas/shared-go => ../../shared/go`

**Usage**:
```bash
# Run locally from repo root
./scripts/ci/check-go-modules.sh

# Exit code 0 = success, 1 = errors found
```

**When it runs**:
- On push to `main`, `develop`, or `staging` branches
- On pull requests that modify `go.mod` files
- Can be triggered manually via workflow_dispatch
- Runs as part of the service-ci.yml lint job

**Example output**:
```
✓ services/admin-api-service: github.com/otherjamesbrown/ai-aas/services/admin-api-service
✓ shared/go: github.com/ai-aas/shared-go
ERROR: Incorrect shared module import in services/api-router-service
  Expected import: github.com/ai-aas/shared-go
  Found incorrect: github.com/otherjamesbrown/ai-aas/shared/go
```

**Related**:
- Bead: ai-aas-v91y
- Workflow: `.github/workflows/go-module-lint.yml`
- Workflow: `.github/workflows/service-ci.yml` (integrated into lint job)

### run-local.sh

Runs CI checks locally before pushing to GitHub.

### trigger-remote.sh

Triggers remote CI workflows via GitHub API.

### verify-workflow-triggered.sh

Verifies that a GitHub Actions workflow was triggered after a git push.

**Purpose**: Prevents deployments from using stale Docker images by ensuring CI workflows trigger successfully after code changes.

**Problem Solved**: Root cause from [aas-j20v](../../.beads/issues/aas-j20v.jsonl) - Code pushed to develop without CI workflow running, causing ArgoCD to deploy outdated images.

**What it does**:
1. Gets the latest commit SHA
2. Polls GitHub Actions API (via `gh` CLI) for up to 30 seconds
3. Checks if the specified workflow is running for the commit
4. Reports success/failure with actionable error messages

**Usage**:
```bash
# Verify CI triggered for current branch
./scripts/ci/verify-workflow-triggered.sh

# Verify specific branch
./scripts/ci/verify-workflow-triggered.sh develop

# Verify different workflow
./scripts/ci/verify-workflow-triggered.sh develop "Dev Environment CI"

# Common workflow after git push
git push origin develop
./scripts/ci/verify-workflow-triggered.sh develop CI
```

**Exit codes**:
- `0`: CI workflow triggered successfully
- `1`: CI workflow not found or failed
- `2`: Missing dependencies (GitHub CLI not installed)

**Dependencies**:
- [GitHub CLI (`gh`)](https://cli.github.com/) - Must be authenticated
- `git` - To get current branch/commit
- `jq` - JSON parsing

**When to use**:
- After every `git push` to develop/staging/main
- Before promoting deployments between environments
- When debugging "stale image" deployment issues

**Troubleshooting**:
If verification fails:
1. **Check path filters**: `.github/workflows/service-ci.yml` only triggers on paths like `services/**`, `shared/**`
2. **Check workflow syntax**: Run `yamllint .github/workflows/service-ci.yml`
3. **Manual trigger**: `gh workflow run "CI" --ref develop`
4. **View recent runs**: `gh run list --limit 10 --branch develop --workflow "CI"`

**Related**:
- Bead: aas-9j2p (this task), aas-j20v (root cause bug)
- Context: `context/go-services-developer/agents.md` (added to checklist)
- Workflow: `.github/workflows/service-ci.yml`
