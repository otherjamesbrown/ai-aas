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
