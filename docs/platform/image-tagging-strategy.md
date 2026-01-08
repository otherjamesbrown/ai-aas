---
title: Image Tagging Strategy
status: active
last_updated: 2026-01-08
changes: "Added automated build-once-promote-everywhere CI/CD workflow"
---

# Image Tagging Strategy

This document defines the container image tagging strategy for the AI-AAS platform.

## Overview

All platform services use **SHA-based tags** for container images to ensure:
- **Immutability**: Tags never change or get overwritten
- **Traceability**: Each tag maps to a specific git commit
- **Consistency**: Same tag across all environments means identical code
- **Rollback safety**: Any previous SHA can be deployed at any time

## Tag Format

**Standard format**: `sha-<first-7-chars-of-commit>`

**Example**: `sha-923f2ed`

This corresponds to git commit `923f2ed...` in the repository.

## Automated Tagging (CI/CD)

GitHub Actions implements a **build-once-promote-everywhere** workflow:

### Build Phase (develop branch)

When code is pushed to `develop`:
1. CI builds and pushes images with SHA tags (`sha-abc123f`)
2. CI automatically updates `values-development.yaml` with the new SHA
3. ArgoCD syncs the new images to the development cluster

### Promotion Phase (staging/main branches)

When code is merged to `staging` or `main`:
1. CI builds images as backup (for emergency hotfixes)
2. **CI automatically promotes images** from the source environment:
   - Staging: Reads SHA from `values-development.yaml` and updates `values-staging.yaml`
   - Production: Reads SHA from `values-staging.yaml` and updates `values-production.yaml`
3. ArgoCD syncs the promoted images

**Key points**:
- Images are built once on develop, promoted to staging/production
- No manual Helm values updates needed for staging/production
- Emergency hotfixes can build fresh images on staging/main if needed
- All commits use `[skip ci]` to prevent infinite loops

## Manual Promotion Workflow (Legacy/Override)

**Note**: As of 2026-01-08, CI/CD automatically promotes images. Manual promotion is only needed for:
- Overriding automated promotion (e.g., rollback to older SHA)
- Emergency scenarios where CI is unavailable

Use the `promote-images.sh` script to update all services to a specific SHA:

```bash
# Promote all environments to a specific SHA
./scripts/promote-images.sh sha-923f2ed

# Promote only staging
./scripts/promote-images.sh sha-923f2ed staging

# Promote only development
./scripts/promote-images.sh sha-923f2ed development
```

The script updates all Helm `values-<environment>.yaml` files with the specified SHA tag.

## Helm Values Configuration

Each service has environment-specific values files:

```
services/api-router-service/deployments/helm/api-router-service/
├── values.yaml                # Base values (defaults)
├── values-development.yaml    # Development overrides
├── values-staging.yaml        # Staging overrides
└── values-production.yaml     # Production overrides
```

**Example values file**:
```yaml
image:
  repository: ghcr.io/otherjamesbrown/ai-aas/api-router-service
  tag: "sha-923f2ed"  # Standardized SHA-based tag
  pullPolicy: Always
```

## Services Using SHA Tags

All platform services follow this strategy:

| Service | Helm Values Path |
|---------|------------------|
| admin-api-service | `services/admin-api-service/deployments/helm/admin-api-service/values-*.yaml` |
| analytics-service | `services/analytics-service/deployments/helm/analytics-service/values-*.yaml` |
| api-router-service | `services/api-router-service/deployments/helm/api-router-service/values-*.yaml` |
| user-org-service | `services/user-org-service/configs/helm/values-*.yaml` |
| ai-model-operator | `operators/ai-model-operator/deployments/helm/ai-model-operator/values-*.yaml` |

## Deployment Workflow

### Standard Promotion Flow (Automated)

```
1. Code merged to develop
   ↓
2. CI builds image with sha-XXXXXXX
   ↓
3. CI updates development values: sha-XXXXXXX (automatic)
   ↓
4. ArgoCD syncs to development cluster
   ↓
5. Test in development cluster
   ↓
6. Create PR: develop → staging
   ↓
7. Merge PR to staging
   ↓
8. CI promotes staging values to sha-XXXXXXX (automatic)
   ↓
9. ArgoCD syncs to staging cluster
   ↓
10. Test in staging cluster
   ↓
11. Create PR: staging → main
   ↓
12. Merge PR to main
   ↓
13. CI promotes production values to sha-XXXXXXX (automatic)
   ↓
14. ArgoCD syncs to production cluster (manual sync required)
```

**Key Changes**:
- Steps 3, 8, 13 are now **fully automated** by CI/CD
- No manual `promote-images.sh` invocation needed
- Manual promotion script remains available for overrides/rollbacks

### GitOps Integration

**Automated (as of 2026-01-08)**:
CI/CD now handles all value file updates and commits. No manual git operations required.

**Manual override** (if using `promote-images.sh`):

```bash
# 1. Review changes
git diff

# 2. Commit
git add services/ operators/
git commit -m "chore: promote images to sha-923f2ed [skip ci]"

# 3. Push to appropriate branch
git push origin staging  # For staging changes
git push origin develop  # For development changes
git push origin main     # For production changes

# 4. ArgoCD auto-syncs (development/staging) or manual sync (production)
argocd app sync api-router-service-production
```

## Finding the Current SHA

To find which SHA is currently deployed in each environment:

```bash
# Check development
grep "tag:" services/*/deployments/helm/*/values-development.yaml
grep "tag:" operators/*/deployments/helm/*/values-development.yaml

# Check staging
grep "tag:" services/*/deployments/helm/*/values-staging.yaml
grep "tag:" operators/*/deployments/helm/*/values-staging.yaml

# Check production
grep "tag:" services/*/deployments/helm/*/values-production.yaml
grep "tag:" operators/*/deployments/helm/*/values-production.yaml
```

Or use the cluster directly:

```bash
kubectl get deploy -n <namespace> <service> -o jsonpath='{.spec.template.spec.containers[0].image}'
```

## Rollback Procedure

To rollback to a previous version:

1. **Find the previous SHA**:
   ```bash
   git log --oneline services/<service>/
   ```

2. **Promote to the old SHA**:
   ```bash
   ./scripts/promote-images.sh sha-abc123f staging
   ```

3. **Commit and push**:
   ```bash
   git add .
   git commit -m "chore: rollback staging to sha-abc123f"
   git push origin staging
   ```

4. **Verify deployment**:
   ```bash
   argocd app sync <service>-staging
   kubectl get pods -n <namespace>
   ```

## Anti-Patterns (AVOID)

### ❌ Environment-Specific Tags
```yaml
# DON'T use environment tags
tag: dev        # Bad
tag: staging    # Bad
tag: latest     # Bad
```

**Why**: These tags are mutable and can be overwritten, breaking traceability.

### ❌ Inline Image Overrides in ArgoCD
```yaml
# gitops/clusters/staging/apps/service.yaml
spec:
  source:
    helm:
      values: |
        image:
          tag: sha-123456  # Bad - creates duplication
```

**Why**: Tag should be defined once in `values-staging.yaml`, not in multiple places.

### ❌ Manual kubectl set image
```bash
kubectl set image deployment/api-router image=ghcr.io/.../service:sha-xyz
```

**Why**: Bypasses GitOps - changes will be reverted by ArgoCD self-heal.

## Migration from Environment Tags

If you find services using environment tags (`dev`, `staging`, `latest`):

1. **Identify current commit**:
   ```bash
   # Find the commit that matches the environment tag
   git log --oneline develop | head -1  # For dev tag
   git log --oneline staging | head -1  # For staging tag
   ```

2. **Update to SHA tag**:
   ```bash
   SHORT_SHA=$(git rev-parse --short=7 HEAD)
   ./scripts/promote-images.sh sha-$SHORT_SHA <environment>
   ```

3. **Commit and deploy**:
   ```bash
   git add .
   git commit -m "chore: migrate to SHA-based tags"
   git push origin <branch>
   ```

## Troubleshooting

### Image Not Found

**Symptom**: `ImagePullBackOff` with `manifest unknown` error

**Cause**: SHA tag doesn't exist in container registry

**Solution**:
1. Verify the commit exists: `git log --oneline | grep <sha>`
2. Check if CI built the image: GitHub Actions → Workflows → Build Images
3. If CI didn't run, manually trigger: `git commit --allow-empty -m "ci: rebuild images" && git push`

### Pods Not Updating

**Symptom**: Deployment has new SHA tag but pods still running old version

**Cause**: ArgoCD hasn't synced yet or sync failed

**Solution**:
```bash
# Check ArgoCD app status
argocd app get <service>-<environment>

# Manual sync
argocd app sync <service>-<environment>

# Force refresh
argocd app sync <service>-<environment> --force
```

## References

- **Promotion Script**: `scripts/promote-images.sh`
- **CI/CD Pipeline**: `.github/workflows/build-images.yml`
- **GitOps Workflow**: `docs/platform/branching-workflow.md`
- **ArgoCD Guide**: `docs/platform/argocd-testing-guide.md`
