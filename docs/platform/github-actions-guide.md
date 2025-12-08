# GitHub Actions Workflow Guide

---
last_updated: 2025-12-08
document_type: guide
---

This guide documents best practices and common pitfalls when working with GitHub Actions workflows.

## Workflow Files

All workflows are in `.github/workflows/`. Current workflows:

| Workflow | Purpose |
|----------|---------|
| `service-ci.yml` | Go microservices CI (build, test, lint) |
| `web-portal.yml` | Web portal CI (lint, test, e2e, build) |
| `shared-libraries-ci.yml` | Shared Go libraries CI |
| `shared-libraries-release.yml` | Shared libraries release |
| `dev-environment-ci.yml` | Development environment validation |
| `remote-ci.yml` | Manual dispatch workflow |
| `reusable-build.yml` | Reusable workflow for build/test |
| `branch-flow-enforcement.yml` | Enforce branch promotion rules |
| `api-router-validation.yml` | API router specific validation |
| `branch-sync-check.yml` | Branch synchronization checks |
| `db-guardrails.yml` | Database migration guardrails |
| `e2e.yml` | End-to-end tests |
| `infra-availability.yml` | Infrastructure availability checks |
| `infra-terraform.yml` | Terraform validation |
| `post-deploy-health-check.yml` | Post-deployment health checks |

**Note**: Reusable workflows MUST be at top level of `.github/workflows/`. GitHub doesn't support subdirectories.

## Common Pitfalls & Solutions

### 1. Reusable Workflow Paths

```yaml
# WRONG - Subdirectories don't work
uses: ./.github/workflows/reusable/build.yml  # Will fail!

# CORRECT - Top-level only
uses: ./.github/workflows/reusable-build.yml
```

### 2. Environment Context in Reusable Workflows

```yaml
# WRONG - env context not available in workflow call parameters
with:
  go-version: ${{ env.GO_VERSION }}  # Will fail!

# CORRECT - Use hardcoded values
with:
  go-version: "1.21.x"
```

### 3. Job Dependencies and Outputs

```yaml
# WRONG - Can't access outputs from jobs not in needs
test:
  needs: build  # Only depends on build
  steps:
    - run: echo ${{ needs.info.outputs.service }}  # Can't access!

# CORRECT - Declare all dependencies
test:
  needs: [build, info]  # Explicitly list all dependencies
  steps:
    - run: echo ${{ needs.info.outputs.service }}
```

### 4. workflow_dispatch on Feature Branches

`workflow_dispatch` requires the workflow file to exist on `main` before it can be triggered on feature branches.

```bash
# Works once workflow exists on main
gh workflow run remote-ci.yml --ref feature-branch -f service=my-service
```

### 5. Script Permissions

```bash
# Set executable bit in git
chmod +x scripts/metrics/upload.sh
git add scripts/metrics/upload.sh

# Or invoke with explicit interpreter
bash ./scripts/metrics/upload.sh
```

## Test Before Build Pattern

The web portal workflow demonstrates the critical pattern:

```yaml
jobs:
  lint:
    # Run linting

  test:
    # Run unit tests

  test-e2e:
    # Run E2E tests

  build:
    needs: [lint, test, test-e2e]  # Build ONLY if all tests pass
```

**Why This Matters**: Without this dependency, broken code could be deployed if it compiles. Always make build/deploy jobs depend on test jobs.

## Testing Workflows

### Local Testing with act
```bash
make ci-local SERVICE=hello-service
```

### Remote Testing
```bash
# Use make target
make ci-remote SERVICE=world-service NOTES="testing fix"

# Or gh CLI directly
gh workflow run remote-ci.yml --ref $(git branch --show-current) \
  -f service=hello-service \
  -f notes="manual test"
```

### Debugging Failed Runs
```bash
# View run details
gh run list --workflow remote-ci.yml -L 5
gh run view RUN_ID

# View failed logs
gh run view RUN_ID --log-failed

# Watch running workflow
gh run watch RUN_ID
```

## Workflow Design Patterns

### Dispatch Info Collection
```yaml
jobs:
  dispatch-info:
    outputs:
      service: ${{ steps.collect.outputs.service }}
    steps:
      - id: collect
        run: echo "service=${{ inputs.service || 'all' }}" >> "$GITHUB_OUTPUT"
```

### Reusable Workflow Call
```yaml
jobs:
  build:
    needs: dispatch-info
    uses: ./.github/workflows/reusable-build.yml
    with:
      service: ${{ needs.dispatch-info.outputs.service }}
      go-version: "1.21.x"  # Hardcoded, not from env
```

### Conditional Execution
```yaml
jobs:
  metrics:
    needs: [lint, dispatch-info]
    if: ${{ always() }}  # Run even if previous jobs fail
```

## Troubleshooting Checklist

When `workflow_dispatch` doesn't create runs:

1. Is the workflow file on `main` branch?
2. Are reusable workflows at top level of `.github/workflows/`?
3. Are job dependencies correctly declared in `needs:`?
4. Is `env` context used only in job steps, not workflow call parameters?
5. Are scripts executable (`chmod +x`)?
6. Check GitHub UI for syntax errors (Actions tab)

## Related Documentation

- [CI/CD Pipeline](ci-cd-pipeline.md) - Overall pipeline architecture
- [CI Remote CLI](ci-remote-cli.md) - Remote dispatch usage
- [GitHub Actions Documentation](https://docs.github.com/en/actions)
