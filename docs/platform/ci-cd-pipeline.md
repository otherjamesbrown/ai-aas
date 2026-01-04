# CI/CD Pipeline

---
last_updated: 2026-01-04
document_type: overview
---

This document provides an overview of the CI/CD pipeline, Git strategy, and automated testing for the AI-as-a-Service platform.

## Philosophy

Our approach to CI/CD is guided by GitOps principles. All changes to the application and infrastructure are managed through Git, with GitHub Actions for Continuous Integration and ArgoCD for Continuous Deployment.

## Git Strategy

> **Source of Truth**: See [Branching Workflow](../development/branching-workflow.md) for complete details.

### Three-Branch Promotion Model

```
develop → staging → main
   ↓         ↓        ↓
development  staging  production
(auto-sync)  (auto-sync) (manual)
```

| Branch | Environment | ArgoCD Sync | Purpose |
|--------|-------------|-------------|---------|
| `develop` | development | Automated | Fast iteration, immediate feedback |
| `staging` | staging | Automated | Code review, integration testing |
| `main` | production | Manual | Production-ready releases |

### Promotion Flow

1. **develop → staging**: PR with code review required
2. **staging → main**: PR with approval required, production release

Branch protection is enforced via `.github/workflows/branch-flow-enforcement.yml`.

## Continuous Integration (CI)

### Workflow Files

All CI workflows are in `.github/workflows/`:

| Workflow | Purpose | Trigger |
|----------|---------|---------|
| `service-ci.yml` | Go microservices CI | Push to develop/staging/main, PRs |
| `web-portal.yml` | Web portal CI (lint, test, e2e, build) | Changes to web/portal/ |
| `shared-libraries-ci.yml` | Shared Go libraries | Changes to shared/go/ |
| `dev-environment-ci.yml` | Development environment validation | PRs |
| `remote-ci.yml` | Remote CI dispatch | Manual/API trigger |
| `branch-flow-enforcement.yml` | Enforce branch promotion rules | PRs to staging/main |

### Go Services CI (`service-ci.yml`)

Stages:
1. **Setup & Discover Services** - Dynamically discovers services in `services/`
2. **Build** - Compiles code for each microservice
3. **Test** - Runs unit and integration tests
4. **Lint** - Static analysis via `golangci-lint`
5. **Metrics Upload** - Archives build metrics
6. **Build & Push Images** - Builds and pushes Docker images with branch-specific tags (push events only)

#### Docker Image Tagging Strategy

CI automatically tags Docker images with multiple tags for flexibility:

| Tag Type | Format | Example | Purpose |
|----------|--------|---------|---------|
| **Commit SHA** (immutable) | `abc1234` (short) | `ghcr.io/.../admin-api-service:abc1234` | **Primary tag** - immutable, traceable |
| Branch + SHA | `develop-abc1234` | `ghcr.io/.../admin-api-service:develop-abc1234` | Debugging, rollback reference |
| Branch alias | `dev`, `staging`, `latest` | `ghcr.io/.../admin-api-service:dev` | Fallback for manual operations |

**Immutable Deployment Flow (Development)**:

1. **CI builds and pushes** image with commit SHA tag (e.g., `abc1234`)
2. **CI updates** `values-development.yaml` with new SHA: `image.tag: abc1234`
3. **CI commits** updated values file back to `develop` branch with `[skip ci]`
4. **ArgoCD detects** change in Git and syncs automatically
5. **Kubernetes restarts** pods with new image (tag changed in Git)

**Benefits**:
- ✅ Automatic deployments - no manual intervention needed
- ✅ Immutable tags - each commit gets unique image
- ✅ Easy rollback - change `image.tag` to previous SHA
- ✅ Full traceability - image tag = commit hash

**Helm Values Files**:

```yaml
# services/<name>/deployments/helm/<name>/values-development.yaml
# Updated automatically by CI with commit SHA
image:
  tag: abc1234  # Auto-updated by CI on each build

# services/<name>/deployments/helm/<name>/values-staging.yaml
# Updated manually or via promotion workflow
image:
  tag: staging  # OR specific SHA for controlled rollout

# services/<name>/deployments/helm/<name>/values.yaml (production default)
# Updated manually during production promotion
image:
  tag: latest  # OR specific SHA for production
```

**Manual Rollback**:

```bash
# Roll back to previous commit
cd services/api-router-service/deployments/helm/api-router-service
yq eval '.image.tag = "xyz9876"' -i values-development.yaml
git add values-development.yaml
git commit -m "rollback(api-router): revert to xyz9876"
git push origin develop
# ArgoCD will sync automatically
```

See `.github/workflows/service-ci.yml` (lines 199-274) for the complete implementation.

### Web Portal CI (`web-portal.yml`)

Stages:
1. **Lint** - ESLint checks
2. **Unit Tests** - Vitest tests
3. **E2E Tests** - Playwright end-to-end tests
4. **Build** - Docker image build **only if all tests pass**

**Critical**: Build depends on all tests passing. Broken code cannot be deployed.

### Required Secrets

See [Environment Access](environment-access.md#github-secrets) for secret configuration.

Key secrets:
- `GHCR_TOKEN` - GitHub Container Registry authentication

## Automated Testing Strategy

| Test Type | Location | Purpose |
|-----------|----------|---------|
| Unit Tests | Alongside source code | Individual component verification |
| Integration Tests | `make test` | Component interaction verification |
| E2E Tests | `web/portal/e2e/` | Full user workflow verification |
| Performance Tests | `tests/perf/` | Performance and scalability |
| Infrastructure Tests | `tests/infra/` | Infrastructure-as-code validation |

## Continuous Deployment (CD) with ArgoCD

### GitOps Structure

```
gitops/
├── clusters/
│   ├── development/    # Watches: develop branch
│   │   ├── apps/       # ArgoCD Application definitions
│   │   └── projects/   # AppProject definitions
│   ├── staging/        # Watches: staging branch
│   │   ├── apps/
│   │   └── projects/
│   └── production/     # Watches: main branch
│       ├── apps/
│       └── projects/
```

### Deployment by Environment

#### Development
- **Trigger**: Push to `develop` branch
- **Process**: CI validates → ArgoCD auto-syncs
- **Apps location**: `gitops/clusters/development/apps/`

#### Staging
- **Trigger**: PR merge from `develop` to `staging`
- **Process**: CI validates → ArgoCD auto-syncs
- **Apps location**: `gitops/clusters/staging/apps/`

#### Production
- **Trigger**: PR merge from `staging` to `main`
- **Process**: CI validates → Manual ArgoCD sync required
- **Apps location**: `gitops/clusters/production/apps/`

```bash
# Manual production sync
argocd app sync <app-name>
```

### ArgoCD Application Standards

All Applications MUST include:

```yaml
syncPolicy:
  automated:
    prune: true
    selfHeal: true
    allowEmpty: false
  syncOptions:
    - CreateNamespace=true
    - PrunePropagationPolicy=foreground
    - PruneLast=true
  retry:
    limit: 5
    backoff:
      duration: 5s
      factor: 2
      maxDuration: 3m
```

#### Branch Targeting

| Environment | targetRevision |
|-------------|---------------|
| development | `develop` |
| staging | `staging` |
| production | `main` |

**Never** reference feature branches in Applications.

### Bootstrap & Configuration

```bash
# Bootstrap ArgoCD for an environment
./scripts/gitops/bootstrap_argocd.sh <environment> <kube-context>

# Customize via
gitops/templates/argocd-values.yaml
```

## Branch Protection

Configure in GitHub Settings → Branches:

### For `staging`:
- Require PR before merging
- Require 1 approval
- Require status check: `check-source-branch`

### For `main`:
- Require PR before merging
- Require 1 approval
- Require status check: `check-source-branch`

## Related Documentation

- [Branching Workflow](../development/branching-workflow.md) - Complete branch promotion details
- [ArgoCD Deployment Workflow](../runbooks/argocd-deployment-workflow.md)
- [ArgoCD Bootstrap](../runbooks/argocd-bootstrap.md)
- [ArgoCD Testing Guide](argocd-testing-guide.md)
- [Environment Access](environment-access.md)
