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
5. **Triggers CI** - Kicks off GitHub Actions CI pipeline
6. **Waits for PR merge** - After merge, ArgoCD auto-syncs

**If on main branch:**
1. **Reviews changes** - Shows git status
2. **Commits changes** - Prompts for commit message
3. **Pushes to main** - Pushes directly to main
4. **ArgoCD auto-syncs** - Watches main branch (development)
5. **Verifies deployment** - Checks ingress and services

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
Code Review
  ↓
Merge to main
  ↓
ArgoCD Auto-Sync (development)
  ↓
Deployment Complete
```

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

3. **Review PR:**
   - Script provides PR URL
   - Review changes in GitHub
   - Get approvals if required

4. **Merge PR:**
   - Merge via GitHub UI or CLI
   - ArgoCD auto-detects changes on main
   - Auto-syncs within 30-60 seconds

5. **Verify:**
   - Check ArgoCD UI: https://argocd.dev.ai-aas.local
   - Monitor deployment status

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

**Feature Branch:**
1. Run `./scripts/dev/deploy.sh` → Creates PR
2. Review PR → Merge when ready
3. ArgoCD auto-syncs from main

**Main Branch:**
1. Run `./scripts/dev/deploy.sh` → Pushes directly
2. ArgoCD auto-syncs immediately

### Environments

**Development:**
- Auto-sync: Enabled (watches `main`)
- ArgoCD UI: https://argocd.dev.ai-aas.local
- Manual sync: `argocd app sync web-portal-development`

**Production:**
- Auto-sync: Disabled (manual approval required)
- Manual sync: `argocd app sync web-portal-production`

### Monitoring

- **CI Status**: `gh run list --workflow ci.yml`
- **ArgoCD Apps**: `kubectl get application -n argocd`
- **ArgoCD UI**: https://argocd.dev.ai-aas.local
- **Kubernetes**: `kubectl get all --all-namespaces`

## Important Notes

⚠️ **Never manually patch Kubernetes resources managed by ArgoCD!**
- ArgoCD will revert manual changes
- Always update source files (Helm values, manifests) in git
- Commit → PR → Merge → ArgoCD syncs

✅ **Best Practices:**
- Always use PRs for code review (unless emergency)
- Let ArgoCD handle all deployments
- Monitor ArgoCD UI for sync status
- Trust the GitOps workflow

## Documentation

- `docs/runbooks/argocd-deployment-workflow.md` - **GitOps workflow guide (READ THIS FIRST)**
- `docs/runbooks/deploy-to-environments.md` - Complete deployment guide
- `docs/platform/tls-ssl-setup.md` - TLS configuration
- `docs/platform/infrastructure-overview.md` - Infrastructure details
- `docs/runbooks/ci-remote.md` - Remote CI execution guide

