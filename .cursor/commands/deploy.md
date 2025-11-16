# Deploy Changes to Environments

This command commits local changes, pushes to git, triggers CI/CD, and syncs ArgoCD for deployment.

## Quick Start

```bash
# Review what will be deployed
git status

# Run the deployment script
./scripts/dev/deploy.sh [development|production]

# Or follow manual steps below
```

## What This Does

1. **Reviews changes** - Shows git status
2. **Commits changes** - Creates commit with descriptive message
3. **Pushes to git** - Pushes to current branch or main
4. **Triggers CI** - Kicks off GitHub Actions CI pipeline
5. **Syncs ArgoCD** - Syncs applications for deployment
6. **Verifies deployment** - Checks ingress and services

## Prerequisites

- Git repository initialized
- GitHub CLI (`gh`) authenticated: `gh auth login`
- Kubeconfigs configured: `~/kubeconfigs/kubeconfig-*.yaml`
- ArgoCD CLI installed: `brew install argocd` or download from [ArgoCD releases](https://github.com/argoproj/argo-cd/releases)

## Manual Deployment Steps

### Step 1: Review and Stage Changes

```bash
# Check what will be committed
git status

# Stage all changes
git add .

# Or stage specific files
git add services/ web/ gitops/ docs/ infra/
```

### Step 2: Commit Changes

```bash
# Get current branch
BRANCH=$(git branch --show-current)

# Commit with message (customize as needed)
git commit -m "feat: Deploy changes

- Description of changes
- Related: T-S<spec>-P<phase>-<task>"
```

### Step 3: Push to Git

```bash
# Push current branch
git push origin "$(git branch --show-current)"

# Or push to main
git push origin main
```

### Step 4: Trigger CI Pipeline

#### Option A: Automatic (Push to main)

Pushing to `main` automatically triggers CI via GitHub Actions.

#### Option B: Manual Trigger (for branches)

```bash
# Trigger remote CI workflow
make ci-remote SERVICE=all NOTES="Deploying changes"

# Or use GitHub CLI directly
gh workflow run ci-remote.yml \
  --ref "$(git branch --show-current)" \
  --raw-field service=all \
  --raw-field notes="Deploying changes"
```

### Step 5: Sync ArgoCD (Development)

Development environment auto-syncs on push to `main`, but you can force sync:

```bash
# Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Login to ArgoCD (if needed)
argocd login argocd.dev.ai-aas.local --grpc-web --insecure
# Or use port-forward: kubectl port-forward svc/argocd-server -n argocd 8080:443

# Sync applications
argocd app sync web-portal-development
argocd app sync api-router-service-development

# Check status
argocd app list
```

### Step 6: Sync ArgoCD (Production)

Production requires manual sync:

```bash
# Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml

# Login to ArgoCD
argocd login argocd.prod.ai-aas.local --grpc-web --insecure

# Sync applications (manual approval)
argocd app sync web-portal-production
argocd app sync api-router-service-production
```

### Step 7: Verify Deployment

```bash
# Set kubeconfig for environment
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml  # or production

# Check ingress
kubectl get ingress --all-namespaces

# Check pods
kubectl get pods -n development
kubectl get pods -n default

# Test endpoints
curl -k https://api.dev.ai-aas.local/healthz
curl -k https://portal.dev.ai-aas.local
```

## Automated Deployment Script

The deployment script is available at `scripts/dev/deploy.sh`:

```bash
# Run deployment script
./scripts/dev/deploy.sh [development|production]

# Example: Deploy to development
./scripts/dev/deploy.sh development

# Example: Deploy to production
./scripts/dev/deploy.sh production
```

The script automates all steps: commit, push, CI trigger, and ArgoCD sync.

## Quick Reference

**Development**:
- Auto-sync: ArgoCD syncs automatically on push to `main`
- Manual sync: `argocd app sync web-portal-development`

**Production**:
- Manual sync required: `argocd app sync web-portal-production`

**CI/CD**:
- Automatic: Push to `main` triggers CI
- Manual: `make ci-remote SERVICE=all`

**Monitoring**:
- CI Status: `gh run list --workflow ci.yml`
- ArgoCD Apps: `argocd app list`
- Kubernetes Resources: `kubectl get all --all-namespaces`

## Documentation

- `docs/runbooks/deploy-to-environments.md` - Complete deployment guide
- `docs/platform/tls-ssl-setup.md` - TLS configuration
- `docs/platform/infrastructure-overview.md` - Infrastructure details
- `docs/runbooks/ci-remote.md` - Remote CI execution guide

