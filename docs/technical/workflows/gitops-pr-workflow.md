# GitOps PR Workflow

## Overview

This document describes the environment-based branch workflow that enables PRs while maintaining GitOps automation.

## Branch Strategy

```
feature/* → develop (auto-deploys to dev) → main (auto-deploys to prod)
```

### Branch Purposes

- **`develop`** - Development environment source of truth
  - Tracks latest development changes
  - Auto-syncs to development cluster via ArgoCD
  - All feature PRs merge here first

- **`main`** - Production environment source of truth
  - Tracks stable, production-ready code
  - Auto-syncs to production cluster via ArgoCD
  - Only receives changes promoted from `develop`

- **`feature/*`** - Feature/fix branches
  - Created from `develop`
  - PR targets `develop`
  - Deleted after merge

## Daily Development Workflow

### 1. Create Feature Branch

```bash
# Start from develop
git checkout develop
git pull origin develop

# Create feature branch
git checkout -b feature/my-feature
# or
git checkout -b fix/bug-description
```

### 2. Make Changes and Push

```bash
# Make your changes
vim services/api-router-service/...

# Commit changes
git add .
git commit -m "feat: add new feature"

# Push feature branch
git push origin feature/my-feature
```

###3. Create PR to `develop`

```bash
# Create PR targeting develop branch
gh pr create --base develop --head feature/my-feature \
  --title "feat: Add new feature" \
  --body "Description of changes..."
```

**OR** via GitHub UI: https://github.com/otherjamesbrown/ai-aas/pulls

### 4. Auto-Deploy to Development

Once the PR is merged to `develop`:

1. **ArgoCD detects the change** (polls every 3 minutes)
2. **Automatic sync** updates the development cluster
3. **Test in development environment**

```bash
# Check ArgoCD sync status
kubectl get application -n argocd api-router-service-development \
  -o jsonpath='{.status.sync.status}'

# Watch deployment progress
kubectl get pods -n development -l app.kubernetes.io/name=api-router-service -w
```

## Production Promotion Workflow

### When to Promote

Promote `develop` to `main` when:
- Features are tested and verified in development
- Ready for production release
- Typically done on a release schedule (weekly, bi-weekly, etc.)

### How to Promote

```bash
# 1. Ensure develop is up to date
git checkout develop
git pull origin develop

# 2. Create promotion PR from develop to main
gh pr create --base main --head develop \
  --title "Release: $(date +%Y-%m-%d)" \
  --body "Promoting changes from develop to production"

# 3. Review and merge PR (requires approval in production)

# 4. ArgoCD auto-syncs to production cluster

# 5. Monitor production deployment
kubectl get application -n argocd -l environment=production
```

## ArgoCD Configuration

### Development Applications

Track `develop` branch:

```yaml
# gitops/clusters/development/apps/api-router-service.yaml
spec:
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: develop  # ← Tracks develop branch
    path: services/api-router-service/deployments/helm/api-router-service
```

### Production Applications

Track `main` branch:

```yaml
# gitops/clusters/production/apps/api-router-service.yaml
spec:
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: main  # ← Tracks main branch
    path: services/api-router-service/deployments/helm/api-router-service
```

## Hotfix Workflow

For urgent production fixes:

```bash
# 1. Create hotfix from main
git checkout main
git pull origin main
git checkout -b hotfix/critical-bug

# 2. Make fix and push
git add .
git commit -m "fix: critical production bug"
git push origin hotfix/critical-bug

# 3. Create PR to main (expedited review)
gh pr create --base main --head hotfix/critical-bug \
  --title "HOTFIX: Critical bug description" \
  --body "Urgent fix for production issue"

# 4. After merge, backport to develop
git checkout develop
git merge main
git push origin develop
```

## Troubleshooting

### ArgoCD Not Syncing

```bash
# Check application status
kubectl get application -n argocd <app-name> -o yaml

# Manual sync if needed
argocd app sync <app-name>

# Force refresh
kubectl annotate application <app-name> -n argocd \
  argocd.argoproj.io/refresh=hard --overwrite
```

### Branch Out of Sync

```bash
# If develop is behind main
git checkout develop
git merge main
git push origin develop

# If main is behind develop (after promotion)
# Create a promotion PR (normal process)
```

### Checking What's Deployed

```bash
# View deployed git revision
kubectl get application -n argocd <app-name> \
  -o jsonpath='{.status.sync.revision}'

# View full sync status
argocd app get <app-name>
```

## Best Practices

1. **Always create PRs** - Never push directly to `develop` or `main`
2. **Test in development first** - All changes must pass through `develop`
3. **Small, focused PRs** - Easier to review and safer to deploy
4. **Clear commit messages** - Follow conventional commits format
5. **Review before promoting** - Production releases require extra scrutiny
6. **Monitor after deployment** - Check logs and metrics after each deployment

## Quick Reference

```bash
# Create feature
git checkout -b feature/name develop
git push origin feature/name
gh pr create --base develop

# Promote to production
gh pr create --base main --head develop

# Check deployment status
kubectl get application -n argocd
kubectl get pods -n <namespace> -w

# Manual sync (if needed)
argocd app sync <app-name>
```

## See Also

- [Deploy to Environments Runbook](./deploy-to-environments.md)
- [ArgoCD Documentation](https://argo-cd.readthedocs.io/)
- [CLAUDE.md](../../CLAUDE.md) - Development workflow overview
