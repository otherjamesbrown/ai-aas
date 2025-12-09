# Branching Workflow

This document describes the branch promotion workflow used in the AI-AAS platform.

## Overview

```
develop (fast iteration, ArgoCD: development)
    ↓ PR with code review
staging (integration testing, ArgoCD: staging)
    ↓ PR with approval
main (production-ready, ArgoCD: production)
```

## Branch Purposes

| Branch | Environment | ArgoCD Sync | Protection |
|--------|-------------|-------------|------------|
| `develop` | development | Auto-sync | None (fast iteration) |
| `staging` | staging | Auto-sync | Require PR from develop |
| `main` | production | Manual sync | Require PR from staging |

## Daily Development Workflow

### 1. Direct Commits to Develop

For fast iteration and immediate ArgoCD deployment to development:

```bash
# Make changes
git add .
git commit -m "feat: your feature"
git push origin develop
```

ArgoCD will automatically sync to the development cluster.

### 2. Promoting to Staging (Code Review)

When ready for code review and staging deployment:

```bash
# Create PR from develop to staging
gh pr create --base staging --head develop --title "Release: description"
```

The PR will:
- Require at least 1 approval
- Run the branch flow enforcement check
- Allow you to review all changes since last staging release

### 3. Promoting to Production

After staging testing is complete:

```bash
# Create PR from staging to main
gh pr create --base main --head staging --title "Production Release: description"
```

The PR will:
- Require at least 1 approval
- Run the branch flow enforcement check
- Production ArgoCD requires manual sync after merge

## Branch Protection Rules

### Enforced via GitHub Actions

The `.github/workflows/branch-flow-enforcement.yml` workflow enforces:

- PRs to `staging` must come from `develop`
- PRs to `main` must come from `staging`

Violations will fail the PR check and add a comment explaining the correct flow.

### Configure in GitHub Settings

To fully enforce the workflow, configure these branch protection rules:

#### For `staging`:
1. Go to Settings → Branches → Add rule
2. Branch name pattern: `staging`
3. Enable:
   - Require a pull request before merging
   - Require approvals: 1
   - Require status checks to pass: `check-source-branch`
   - Do not allow bypassing the above settings

#### For `main`:
1. Go to Settings → Branches → Add rule
2. Branch name pattern: `main`
3. Enable:
   - Require a pull request before merging
   - Require approvals: 1
   - Require status checks to pass: `check-source-branch`
   - Do not allow bypassing the above settings

## ArgoCD Configuration

### Development Environment
- Watches: `develop` branch
- Sync: Automated (prune, self-heal)
- Location: `gitops/clusters/development/apps/`

### Staging Environment
- Watches: `staging` branch
- Sync: Automated (prune, self-heal)
- Location: `gitops/clusters/staging/apps/`

### Production Environment
- Watches: `main` branch
- Sync: Manual (requires explicit sync)
- Location: `gitops/clusters/production/apps/`

## Emergency Hotfix Process

For critical production issues that cannot wait for the normal flow:

1. Create hotfix branch from `main`:
   ```bash
   git checkout main
   git pull
   git checkout -b hotfix/critical-fix
   ```

2. Make the fix and push

3. Create PR to `main` (bypass enforcement if admin)

4. After merge, cherry-pick to `staging` and `develop`:
   ```bash
   git checkout staging && git cherry-pick <commit>
   git checkout develop && git cherry-pick <commit>
   ```

## FAQ

### Q: Can I push directly to staging or main?

No. Both branches are protected and require PRs. Direct pushes will be rejected.

### Q: Why can't I create a PR from my feature branch to staging?

The workflow requires: feature → develop → staging → main

Merge your feature branch to develop first, then promote to staging.

### Q: How often should I promote to staging?

Whenever you have a coherent set of features ready for review. Could be daily or after completing a feature set.

### Q: What if the branch flow check fails?

The PR will show a failed check. Read the error message - it will tell you which branch to create the PR from instead.
