# Summary of CI/CD Issue with `web-portal.yml`

This document summarizes the ongoing issue with the `web-portal.yml` GitHub Actions workflow and the attempts made to resolve it.

## Goal

The primary goal is to create a robust and reliable CI/CD pipeline for the `web/portal` application. This includes linting, unit testing, and end-to-end testing.

## The Problem

The CI workflow is consistently failing on the `pnpm install --frozen-lockfile` step in all jobs (`lint`, `test`, and `test-e2e`). The error message is:

`ERR_PNPM_NO_LOCKFILE Cannot install with "frozen-lockfile" because pnpm-lock.yaml is absent`

This is accompanied by a warning:

`WARN Ignoring not compatible lockfile at /home/runner/work/ai-aas/ai-aas/pnpm-lock.yaml`

This indicates that while `pnpm` is finding the `pnpm-lock.yaml` file at the root of the repository, it is not using it correctly when running in the context of the `web/portal` workspace.

## Attempted Solutions

Several attempts have been made to resolve this issue, all of which have resulted in the same error:

1.  **Initial Refactoring**: The workflow was refactored to centralize Node.js and pnpm versions and to use a more robust E2E testing strategy.
2.  **Adjusting `cache-dependency-path`**: The `cache-dependency-path` in the `actions/setup-node` step was modified to point directly to the lockfile (`web/portal/pnpm-lock.yaml`) and then to a glob pattern (`**/pnpm-lock.yaml`).
3.  **Changing `working-directory`**: The `working-directory` for the `pnpm install` command was moved to the root of the repository.
4.  **Using `--lockfile-dir`**: The `--lockfile-dir` flag was added to the `pnpm install` command to explicitly point to the root directory.
5.  **Using `pnpm --filter`**: The workflow was refactored to use `pnpm --filter web/portal <command>` to run commands from the root of the repository.

## Root Cause

The issue was caused by **two problems**:

1. **Duplicate workspace configuration**: There were two `pnpm-workspace.yaml` files - one at the repository root and one in `web/portal/`. This caused pnpm to ignore the root lockfile with the warning: `WARN Ignoring not compatible lockfile at /home/runner/work/ai-aas/ai-aas/pnpm-lock.yaml`

2. **Incorrect filter syntax**: The workflow was using `--filter web/portal` to target the workspace package, but the actual package name in `web/portal/package.json` is `@ai-aas/web-portal`. The filter syntax must use the package name, not the directory path.

## Resolution

The issue was resolved by:

1. **Removing the duplicate workspace file**: Deleted `web/portal/pnpm-workspace.yaml`, keeping only the root-level `pnpm-workspace.yaml` which correctly defines both packages:
   ```yaml
   packages:
     - 'shared/ts'
     - 'web/portal'
   ```

2. **Updating filter syntax in workflow**: Changed all instances of `--filter web/portal` to `--filter @ai-aas/web-portal` in `.github/workflows/web-portal.yml` to use the correct package name.

## Verification

The fix was verified both locally and in CI:

**Local Testing:**
- ✅ `pnpm install --frozen-lockfile` completes successfully
- ✅ `pnpm --filter @ai-aas/web-portal <command>` correctly targets the web portal package
- ✅ Workspace recognizes all 2 packages (`@ai-aas/shared` and `@ai-aas/web-portal`)

**CI Testing (Run #19411572735):**
- ✅ All jobs successfully install dependencies
- ✅ Unit tests pass completely
- ✅ pnpm cache strategy works correctly
- ❌ Lint job fails due to missing ESLint configuration (separate issue)
- ❌ E2E tests fail due to webServer timeout (separate issue)

## Status

**RESOLVED** - The original pnpm workspace/lockfile issue is completely fixed. The CI can now successfully install dependencies from the pnpm workspace.

## Remaining Issues (Not Related to Original Problem)

1. **Missing ESLint Configuration**: The `web/portal` directory needs an ESLint config file (`.eslintrc.js`, `eslint.config.js`, or similar)
2. **E2E Test Web Server**: Playwright's `config.webServer` is timing out - may need environment configuration

These are separate from the pnpm workspace issue and should be addressed independently.

## Lessons Learned

- In a pnpm workspace monorepo, there should be **only one** `pnpm-workspace.yaml` file at the repository root
- The `--filter` flag should use the **package name** from `package.json`, not the directory path
- pnpm workspace package names should follow a consistent naming convention (e.g., `@scope/package-name`)
- **Critical**: The pnpm version in CI must match the lockfile format version:
  - pnpm v8 cannot read lockfile version 9.0 (created by pnpm v9+)
  - pnpm v10 can read lockfile version 9.0
  - Always align CI pnpm version with local development environment
