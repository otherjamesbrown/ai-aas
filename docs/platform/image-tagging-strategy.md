---
title: Image Tagging Strategy
status: active
last_updated: 2026-01-08
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

GitHub Actions automatically builds and tags images on every push to `develop`, `staging`, or `main`:

```yaml
# .github/workflows/build-images.yml
- name: Build and push
  run: |
    SHORT_SHA=$(git rev-parse --short=7 HEAD)
    docker build -t ghcr.io/otherjamesbrown/ai-aas/service:sha-$SHORT_SHA .
    docker push ghcr.io/otherjamesbrown/ai-aas/service:sha-$SHORT_SHA
```

**Key points**:
- Images are built once per commit
- Same SHA tag is available in all environments
- No environment-specific tags (`dev`, `staging`, `latest`) are used

## Manual Promotion Workflow

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

### Standard Promotion Flow

```
1. Code merged to develop
   ↓
2. CI builds image with sha-XXXXXXX
   ↓
3. Update development values: sha-XXXXXXX
   ↓
4. Test in development cluster
   ↓
5. Promote to staging: ./scripts/promote-images.sh sha-XXXXXXX staging
   ↓
6. Test in staging cluster
   ↓
7. Promote to production: ./scripts/promote-images.sh sha-XXXXXXX production
```

### GitOps Integration

After updating values files:

```bash
# 1. Review changes
git diff

# 2. Commit
git add services/ operators/
git commit -m "chore: promote images to sha-923f2ed"

# 3. Push to appropriate branch
git push origin staging  # For staging changes
git push origin develop  # For development changes
git push origin main     # For production changes

# 4. ArgoCD auto-syncs (development) or manual sync (staging/production)
argocd app sync api-router-service-staging
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
