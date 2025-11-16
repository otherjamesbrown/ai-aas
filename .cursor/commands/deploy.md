# Deploy Changes to Environments

This command follows the **GitOps workflow** with code review via Pull Requests. All deployments go through ArgoCD for consistency.

## Quick Start

```bash
# Review what will be deployed
git status

# Run the deployment script (will prompt for missing info)
./scripts/dev/deploy.sh [development|production]
```

## What This Does

**If on feature branch:**
1. **Reviews changes** - Shows git status
2. **Commits changes** - Prompts for commit message
3. **Pushes to git** - Pushes to current branch
4. **Creates/updates PR** - Prompts for PR title/description
5. **CI runs automatically** - GitHub Actions runs tests (lint, unit, E2E)
6. **CI must pass** - With branch protection, PR cannot be merged until all tests pass
7. **Merge PR** - After CI passes and code review, merge to main
8. **ArgoCD auto-syncs** - Watches main branch and deploys

**If on main branch:**
1. **Reviews changes** - Shows git status
2. **Commits changes** - Prompts for commit message
3. **Pushes to main** - Pushes directly to main
4. **CI runs automatically** - GitHub Actions runs tests
5. **ArgoCD auto-syncs** - Watches main branch (development)
6. **Note**: If CI fails, no new Docker image is built, so ArgoCD won't deploy changes
7. **Verifies deployment** - Checks ingress and services

## Prerequisites

- Git repository initialized
- GitHub CLI (`gh`) authenticated: `gh auth login`
- Kubeconfigs configured: `~/kubeconfigs/kubeconfig-*.yaml`
- ArgoCD CLI installed (optional): `brew install argocd` or download from [ArgoCD releases](https://github.com/argoproj/argo-cd/releases)

**The script will prompt you for any missing information:**
- Commit message (if not provided)
- PR title (if creating PR)
- PR description (optional)

## GitOps Workflow (Recommended)

### Workflow Overview

```
Feature Branch
  ↓
Commit & Push
  ↓
Create PR (via deploy script)
  ↓
CI Runs Automatically (lint, test, E2E)
  ↓
CI Must Pass (blocks PR merge)
  ↓
Code Review
  ↓
Merge to main (only if CI passes)
  ↓
ArgoCD Auto-Sync (development)
  ↓
Deployment Complete
```

**Critical**: CI tests (including E2E tests) must pass before code can be merged. This prevents broken functionality from being deployed.

### Step-by-Step (Automated)

The `deploy.sh` script handles most steps automatically:

1. **Run the script:**
   ```bash
   ./scripts/dev/deploy.sh development
   ```

2. **Follow prompts:**
   - Enter commit message (or use default)
   - Enter PR title (if on feature branch)
   - Enter PR description (optional)

3. **Wait for CI:**
   - CI runs automatically on PR creation
   - Check CI status: `gh run list --workflow web-portal.yml` or GitHub UI
   - **CI must pass** before PR can be merged (with branch protection)

4. **Review PR:**
   - Script provides PR URL
   - Review changes in GitHub
   - Get approvals if required

5. **Merge PR:**
   - Merge via GitHub UI or CLI (only if CI passes)
   - ArgoCD auto-detects changes on main
   - Auto-syncs within 30-60 seconds

6. **Verify:**
   - Check ArgoCD UI: https://argocd.dev.ai-aas.local
   - Monitor deployment status
   - Verify CI passed: `gh run list --workflow web-portal.yml`

## Manual Steps (If Needed)

### Create PR Manually

```bash
# After committing and pushing
gh pr create --title "feat: My changes" --body "Description" --base main
```

### Check ArgoCD Sync Status

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get application -n argocd
kubectl get application web-portal-development -n argocd -o jsonpath='{.status.sync.status}'
```

### Force ArgoCD Sync (if needed)

```bash
# Via kubectl
kubectl patch application web-portal-development -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"main"}}}'

# Via ArgoCD CLI
argocd app sync web-portal-development
```

## Automated Deployment Script

The deployment script (`scripts/dev/deploy.sh`) follows GitOps best practices:

**Features:**
- ✅ Prompts for commit message (if not provided)
- ✅ Creates PR automatically (if on feature branch)
- ✅ Prompts for PR title/description
- ✅ Triggers CI pipeline
- ✅ Waits for ArgoCD sync (if on main)
- ✅ Verifies deployment status

**Usage:**
```bash
# Deploy to development
./scripts/dev/deploy.sh development

# Deploy to production
./scripts/dev/deploy.sh production
```

**The script will:**
- Check prerequisites (GitHub CLI, authentication)
- Guide you through each step
- Provide next steps after completion

## Quick Reference

### Branch Workflow

**Feature Branch (Recommended):**
1. Run `./scripts/dev/deploy.sh` → Creates PR
2. **Wait for CI to pass** → All tests must pass (lint, unit, E2E)
3. Review PR → Merge when CI passes and approved
4. ArgoCD auto-syncs from main → Deploys tested code

**Main Branch (Use with caution):**
1. Run `./scripts/dev/deploy.sh` → Pushes directly
2. CI runs automatically → Tests must pass
3. ArgoCD auto-syncs immediately → If CI fails, no new image is built
4. **Note**: Pushing directly to main bypasses PR review. Use only for urgent fixes.

### Environments

**Development:**
- Auto-sync: Enabled (watches `main`)
- ArgoCD UI: https://argocd.dev.ai-aas.local
- Manual sync: `argocd app sync web-portal-development`

**Production:**
- Auto-sync: Disabled (manual approval required)
- Manual sync: `argocd app sync web-portal-production`

### Monitoring

- **CI Status (Go services)**: `gh run list --workflow ci.yml`
- **CI Status (Web portal)**: `gh run list --workflow web-portal.yml`
- **ArgoCD Apps**: `kubectl get application -n argocd`
- **ArgoCD UI**: https://argocd.dev.ai-aas.local
- **Kubernetes**: `kubectl get all --all-namespaces`

**Check CI Status Before Merging:**
```bash
# Check web portal CI
gh run list --workflow web-portal.yml --limit 5

# Check specific PR CI status
gh pr checks <PR_NUMBER>
```

## Important Notes

⚠️ **Never manually patch Kubernetes resources managed by ArgoCD!**
- ArgoCD will revert manual changes
- Always update source files (Helm values, manifests) in git
- Commit → PR → Merge → ArgoCD syncs

✅ **Best Practices:**
- **Always use feature branches and PRs** - Ensures CI tests pass before merge
- **Never merge PRs with failing CI** - All tests (lint, unit, E2E) must pass
- **Monitor CI status** - Check GitHub Actions before merging
- Let ArgoCD handle all deployments
- Monitor ArgoCD UI for sync status
- Trust the GitOps workflow

⚠️ **Important**: With our CI setup, broken code (like a non-functional sign-in button) cannot be deployed because:
- E2E tests catch user workflow issues
- Build jobs depend on test jobs passing
- No Docker image is built if tests fail
- ArgoCD won't see changes to deploy if CI fails

## Documentation

- `docs/runbooks/argocd-deployment-workflow.md` - **GitOps workflow guide (READ THIS FIRST)**
- `docs/runbooks/deploy-to-environments.md` - Complete deployment guide
- `docs/platform/tls-ssl-setup.md` - TLS configuration
- `docs/platform/infrastructure-overview.md` - Infrastructure details
- `docs/runbooks/ci-remote.md` - Remote CI execution guide

