# GitHub Actions Workflow Guide

This guide documents best practices and common pitfalls when working with GitHub Actions workflows in this repository.

## Directory Structure

```
.github/workflows/
├── ci.yml                    # Main CI on push/PR (Go services)
├── web-portal.yml           # Web portal CI (lint, test, e2e, build)
├── ci-remote.yml            # Manual dispatch workflow
├── reusable-build.yml       # Reusable workflow for build/test
└── (other workflows)
```

**CRITICAL**: Reusable workflows MUST be at the top level of `.github/workflows/`. GitHub does not support subdirectories for reusable workflows.

## Common Pitfalls & Solutions

### 1. Reusable Workflow Paths

❌ **WRONG** - Subdirectories don't work:
```yaml
uses: ./.github/workflows/reusable/build.yml  # Will fail!
```

✅ **CORRECT** - Top-level only:
```yaml
uses: ./.github/workflows/reusable-build.yml
```

**Error Message**: `invalid value workflow reference: workflows must be defined at the top level`

### 2. Environment Context in Reusable Workflows

❌ **WRONG** - `env` context not available in workflow call parameters:
```yaml
jobs:
  build:
    uses: ./.github/workflows/reusable-build.yml
    with:
      go-version: ${{ env.GO_VERSION }}  # Will fail!
```

✅ **CORRECT** - Use hardcoded values or pass via inputs:
```yaml
env:
  GO_VERSION: "1.21.x"

jobs:
  build:
    uses: ./.github/workflows/reusable-build.yml
    with:
      go-version: "1.21.x"  # Hardcoded in call
```

**Note**: The `env` context IS available within job steps, just not in the `with:` parameters of a workflow call.

**Error Message**: `Unrecognized named-value: 'env'`

### 3. Job Dependencies and Outputs

❌ **WRONG** - Missing dependency declaration:
```yaml
jobs:
  info:
    outputs:
      service: ${{ steps.collect.outputs.service }}
  
  test:
    needs: build  # Only depends on build
    steps:
      - run: echo ${{ needs.info.outputs.service }}  # Can't access!
```

✅ **CORRECT** - Declare all dependencies:
```yaml
jobs:
  info:
    outputs:
      service: ${{ steps.collect.outputs.service }}
  
  test:
    needs: [build, info]  # Explicitly list all dependencies
    steps:
      - run: echo ${{ needs.info.outputs.service }}  # Now accessible
```

**Rule**: If a job references another job's outputs via `needs.jobname.outputs.*`, that `jobname` MUST be in the `needs:` list.

### 4. workflow_dispatch on Feature Branches

**Issue**: `workflow_dispatch` requires the workflow file to exist on the default branch (`main`) before it can be triggered on feature branches.

**Solution**: Merge workflow files to `main` first, then you can dispatch them on any branch.

```bash
# This works once workflow exists on main
gh workflow run ci-remote.yml --ref feature-branch -f service=my-service
```

### 5. Script Permissions

**Issue**: Scripts checked out in Actions don't preserve executable permissions.

✅ **CORRECT** - Always set executable bit in git:
```bash
chmod +x scripts/metrics/upload.sh
git add scripts/metrics/upload.sh
git commit -m "Make script executable"
```

Or invoke with explicit interpreter:
```yaml
- run: bash ./scripts/metrics/upload.sh
```

## Testing Workflows

### Local Testing with act
```bash
make ci-local SERVICE=hello-service
```

### Remote Testing
```bash
# Test on feature branch (after workflow exists on main)
make ci-remote SERVICE=world-service NOTES="testing fix"

# Or use gh CLI directly
gh workflow run ci-remote.yml --ref $(git branch --show-current) \
  -f service=hello-service \
  -f notes="manual test"
```

### Debugging Failed Runs
```bash
# View run details
gh run list --workflow ci-remote.yml -L 5
gh run view RUN_ID

# View failed logs
gh run view RUN_ID --log-failed

# Watch running workflow
gh run watch RUN_ID
```

## Web Portal Workflow Pattern

The web portal workflow (`.github/workflows/web-portal.yml`) demonstrates a critical pattern: **test before build**.

```yaml
jobs:
  lint:
    name: Lint
    # Runs ESLint
  
  test:
    name: Unit Tests
    # Runs Vitest unit tests
  
  test-e2e:
    name: E2E Tests
    # Runs Playwright E2E tests (including critical user workflows)
  
  build:
    name: Build and Push Docker Image
    needs: [lint, test, test-e2e]  # ⚠️ CRITICAL: Build depends on all tests
    # Only builds if all tests pass
```

**Why This Matters**: Without this dependency, broken code (like a non-functional sign-in button) could be deployed if it compiles successfully. By making `build` depend on test jobs, we ensure:
- ✅ All tests must pass before building images
- ✅ Broken functionality cannot reach production
- ✅ PRs are blocked if tests fail

**Best Practice**: Always make build/deploy jobs depend on test jobs. Never build or deploy code that hasn't passed all tests.

## Workflow Design Patterns

### Pattern: Dispatch Info Collection
```yaml
jobs:
  dispatch-info:
    runs-on: ubuntu-latest
    outputs:
      service: ${{ steps.collect.outputs.service }}
    steps:
      - id: collect
        run: |
          SERVICE="${{ inputs.service }}"
          if [ -z "$SERVICE" ]; then
            SERVICE="all"
          fi
          echo "service=$SERVICE" >> "$GITHUB_OUTPUT"
```

### Pattern: Reusable Workflow Call
```yaml
jobs:
  build:
    needs: dispatch-info  # Declare dependency
    uses: ./.github/workflows/reusable-build.yml
    with:
      service: ${{ needs.dispatch-info.outputs.service }}
      target: build
      go-version: "1.21.x"  # Hardcoded, not from env
```

### Pattern: Conditional Execution
```yaml
jobs:
  metrics:
    needs: [lint, dispatch-info]
    if: ${{ always() }}  # Run even if previous jobs fail
    runs-on: ubuntu-latest
```

### Pattern: Test Before Build (Critical)
```yaml
jobs:
  lint:
    # Run linting
  
  test:
    # Run unit tests
  
  test-e2e:
    # Run E2E tests
  
  build:
    needs: [lint, test, test-e2e]  # Build only if all tests pass
    # Build and push Docker image
```

**Rule**: Never build or deploy without running tests first. Always use `needs:` to enforce test dependencies.

## Troubleshooting Checklist

When `workflow_dispatch` doesn't create runs:

1. ✓ Is the workflow file on the default branch (`main`)?
2. ✓ Are all reusable workflows at top level of `.github/workflows/`?
3. ✓ Are job dependencies correctly declared in `needs:`?
4. ✓ Is `env` context used only in job steps, not workflow call parameters?
5. ✓ Are scripts executable (`chmod +x`)?
6. ✓ Check GitHub UI for syntax errors (Actions tab → workflow → "invalid workflow file")

See `docs/troubleshooting/ci-remote-dispatch.md` for detailed resolution of a real incident.

## References

- [GitHub Actions Documentation](https://docs.github.com/en/actions)
- [Reusable Workflows](https://docs.github.com/en/actions/using-workflows/reusing-workflows)
- [Workflow Syntax](https://docs.github.com/en/actions/using-workflows/workflow-syntax-for-github-actions)
- [Contexts](https://docs.github.com/en/actions/learn-github-actions/contexts)

