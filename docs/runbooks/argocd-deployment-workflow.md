# ArgoCD Deployment Workflow

**Last Updated**: 2025-11-16  
**Owner**: Platform Engineering

## Overview

This document explains the proper workflow for deploying changes via ArgoCD to ensure consistency and avoid manual drift.

## Key Principle: GitOps

**All deployments MUST go through ArgoCD**, which watches the `main` branch. Manual changes to Kubernetes resources will be reverted by ArgoCD's auto-sync.

## Workflow Comparison

### ❌ What `deploy.md` Does NOT Do

The `deploy.md` command (`./scripts/dev/deploy.sh`):
- ✅ Commits changes
- ✅ Pushes to git
- ✅ Triggers CI pipeline
- ✅ Can sync ArgoCD (if ArgoCD CLI available)
- ❌ **Does NOT merge to `main`**
- ❌ **Does NOT update ArgoCD if changes are on a branch**

### ✅ Proper Deployment Workflow

**Step 1: Make Changes on Feature Branch**
```bash
# Work on feature branch
git checkout -b feature/my-feature
# ... make changes ...
git add .
git commit -m "feat: My changes"
git push origin feature/my-feature
```

**Step 2: Merge to Main**
```bash
# Merge to main (via PR or direct merge)
git checkout main
git pull origin main
git merge feature/my-feature --no-ff -m "Merge feature/my-feature"
git push origin main
```

**Step 3: ArgoCD Auto-Syncs**
- ArgoCD watches `main` branch
- Auto-sync detects changes (usually within 30-60 seconds)
- ArgoCD syncs Helm charts and applies changes
- **No manual intervention needed**

**Step 4: Verify Deployment**
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get ingress --all-namespaces
kubectl get pods -n development
```

## When to Use `deploy.md`

Use `deploy.md` (`./scripts/dev/deploy.sh`) when:
- ✅ You're already on `main` branch
- ✅ You want to commit and push changes
- ✅ You want to trigger CI pipeline
- ✅ You want to manually sync ArgoCD (if needed)

**Do NOT use `deploy.md`** when:
- ❌ You're on a feature branch and expect ArgoCD to deploy
- ❌ You haven't merged to `main` yet
- ❌ You want to bypass ArgoCD

## ArgoCD Configuration

### Development Environment
- **Auto-sync**: Enabled
- **Watches**: `main` branch
- **Sync Policy**: `automated` with `selfHeal: true`
- **Location**: `gitops/clusters/development/apps/*.yaml`

### Production Environment
- **Auto-sync**: Disabled (manual sync required)
- **Watches**: `main` branch
- **Sync Policy**: Manual approval
- **Location**: `gitops/clusters/production/apps/*.yaml`

## Troubleshooting

### ArgoCD Not Syncing

**Check sync status:**
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get application -n argocd
kubectl get application web-portal-development -n argocd -o yaml | grep -A 10 "status:"
```

**Check if watching correct branch:**
```bash
kubectl get application web-portal-development -n argocd -o jsonpath='{.spec.source.targetRevision}'
# Should be: main
```

**Check current sync revision:**
```bash
kubectl get application web-portal-development -n argocd -o jsonpath='{.status.sync.revision}'
# Should match latest commit on main
```

**Force sync (if needed):**
```bash
# Via kubectl
kubectl patch application web-portal-development -n argocd \
  --type merge -p '{"operation":{"initiatedBy":{"username":"admin"},"sync":{"revision":"main"}}}'

# Via ArgoCD CLI
argocd app sync web-portal-development
```

### Manual Changes Being Reverted

**Symptom**: You manually patch a resource, but it gets reverted.

**Cause**: ArgoCD auto-sync is enabled and watching `main`.

**Solution**: 
1. Update the source files (Helm values, manifests) in git
2. Commit and push to `main`
3. Wait for ArgoCD to sync (or trigger manually)

**Never manually patch resources managed by ArgoCD!**

## Best Practices

1. **Always update source files first**
   - Helm values: `web/portal/deployments/helm/web-portal/values*.yaml`
   - Kubernetes manifests: `gitops/clusters/*/apps/*.yaml`
   - Infrastructure: `infra/terraform/` or `infra/helm/`

2. **Commit and push to main**
   - Changes must be on `main` for ArgoCD to see them
   - Use feature branches and PRs for review
   - Merge to `main` when ready

3. **Let ArgoCD handle deployment**
   - Don't manually `kubectl apply` resources managed by ArgoCD
   - Don't manually patch Helm releases
   - Trust the GitOps workflow

4. **Monitor ArgoCD sync status**
   - Check ArgoCD UI: `https://argocd.dev.ai-aas.local`
   - Use `kubectl get application` to check status
   - Watch for sync errors in ArgoCD logs

## Quick Reference

**Check ArgoCD apps:**
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl get application -n argocd
```

**Check sync status:**
```bash
kubectl get application web-portal-development -n argocd -o jsonpath='{.status.sync.status}'
```

**View ArgoCD UI:**
```
https://argocd.dev.ai-aas.local
Username: admin
Password: (get from secret: kubectl get secret argocd-initial-admin-secret -n argocd -o jsonpath='{.data.password}' | base64 -d)
```

## Related Documentation

- `docs/platform/endpoints-and-urls.md` - Complete endpoint and URL configuration guide
- `docs/runbooks/deploy-to-environments.md` - Complete deployment guide
- `docs/platform/infrastructure-overview.md` - Infrastructure details
- `.cursor/commands/deploy.md` - Deployment command reference

