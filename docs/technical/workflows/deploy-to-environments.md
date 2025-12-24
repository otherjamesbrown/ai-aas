# Deploy to Environments - Runbook

**Last Updated**: 2025-11-16  
**Owner**: Platform Engineering

## Overview

This runbook covers the complete deployment workflow: committing changes, pushing to git, triggering CI/CD, and syncing ArgoCD for both development and production environments.

## Deployment Flow

```
Local Changes
  ↓
Commit & Push to Git
  ↓
GitHub Actions CI (build/test)
  ↓
Merge to main (if on branch)
  ↓
ArgoCD Auto-Sync (development)
  ↓
Manual ArgoCD Sync (production)
  ↓
Services Deployed
```

## Prerequisites

- Git repository access
- GitHub CLI (`gh`) authenticated
- Kubeconfigs configured (`~/kubeconfigs/kubeconfig-*.yaml`)
- ArgoCD CLI installed (`argocd`)
- Helm 3.x installed

## Quick Deploy Command

Use the Cursor command: `.cursor/commands/deploy.md`

Or follow the manual steps below.

## Step-by-Step Deployment

### Step 1: Review Changes

```bash
# Check what will be committed
git status

# Review diffs
git diff

# Check for uncommitted changes
git status --short
```

### Step 2: Stage Changes

```bash
# Stage all changes (or specific files)
git add .

# Or stage specific directories
git add services/ web/ gitops/ docs/ infra/
```

### Step 3: Commit

```bash
# Commit with descriptive message
git commit -m "feat: <description>

- Change 1
- Change 2
- Change 3

Related: T-S<spec>-P<phase>-<task>"
```

### Step 4: Push to Git

```bash
# Get current branch
BRANCH=$(git branch --show-current)

# Push current branch
git push origin "$BRANCH"

# Or push to main directly
git push origin main
```

### Step 5: Trigger CI/CD

#### Option A: Automatic (Push to main)

If pushing to `main`, GitHub Actions will automatically:
- Run CI pipeline (build, test, lint)
- ArgoCD will auto-sync (development only)

#### Option B: Manual CI Trigger

```bash
# Trigger remote CI workflow
make ci-remote SERVICE=all NOTES="Deploying ingress TLS changes"

# Or use script directly
./scripts/ci/trigger-remote.sh SERVICE=all NOTES="Deploying changes"
```

### Step 6: ArgoCD Sync

#### Development (Auto-Sync)

ArgoCD should auto-sync, but you can force:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Login to ArgoCD
argocd login argocd.dev.otherjamesbrown.com --grpc-web --insecure
# Or use port-forward: kubectl port-forward svc/argocd-server -n argocd 8080:443

# Sync applications
argocd app sync web-portal-development
argocd app sync api-router-service-development
argocd app sync kserve-llama-2-7b-development

# Or sync all apps in development
argocd app sync -l environment=development
```

#### Production (Manual Sync Required)

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml

# Login to ArgoCD
argocd login argocd.prod.otherjamesbrown.com --grpc-web --insecure

# Sync applications (manual approval required)
argocd app sync web-portal-production
argocd app sync api-router-service-production
argocd app sync kserve-llama-2-7b-production
```

### Step 7: Verify Deployment

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Check ingress resources
kubectl get ingress --all-namespaces

# Check pods
kubectl get pods -n development
kubectl get pods -n default

# Check ArgoCD app status
argocd app list

# Test endpoints
curl -k https://api.dev.otherjamesbrown.com/healthz
curl -k https://portal.dev.otherjamesbrown.com
```

## Deploying a Model (KServe)

Deploying a model follows the same GitOps principles.

### Step 1: Create the `InferenceService` Manifest

Create a new directory for your model under `infra/kserve/models/` and add an `inferenceservice.yaml` file.

```yaml
# infra/kserve/models/my-new-model/base/inferenceservice.yaml
apiVersion: "serving.kserve.io/v1beta1"
kind: "InferenceService"
metadata:
  name: "my-new-model"
spec:
  predictor:
    model:
      modelFormat:
        name: vllm
      storageUri: "hf://user/my-new-model"
```

Use overlays for environment-specific configurations (e.g., `minReplicas`, resources).

### Step 2: Create the ArgoCD Application

Create a new ArgoCD application manifest for your model in `argocd-apps/`.

### Step 3: Commit and Push

Commit the new `InferenceService` and ArgoCD application manifests to Git.

### Step 4: Sync in ArgoCD

ArgoCD will detect the new application and deploy the `InferenceService`.

### Step 5: Verify the Model Deployment

```bash
# Check the status of the InferenceService
kubectl get inferenceservice my-new-model -n system

# Test the model endpoint (if exposed via Istio)
curl -k https://my-new-model.system.otherjamesbrown.com/v2/models/my-new-model/infer -d @- <<EOF
{
  "inputs": []
}
EOF
```

## Environment-Specific Steps

### Development Environment

**Auto-Deploy**: ArgoCD auto-syncs on git push to `main`

**Manual Sync**:
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
argocd app sync web-portal-development
argocd app sync api-router-service-development
```

### Production Environment

**Manual Sync Required**: Production requires explicit ArgoCD sync

**Steps**:
1. Push changes to `main`
2. Wait for CI to pass
3. Manually sync ArgoCD:
   ```bash
   export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml
   argocd app sync web-portal-production
   ```

## Troubleshooting

### CI Pipeline Fails

```bash
# Check GitHub Actions
gh run list --workflow ci.yml

# View failed run
gh run view <run-id> --log-failed

# Re-run failed workflow
gh run rerun <run-id>
```

### ArgoCD Not Syncing

```bash
# Check app status
argocd app get <app-name>

# Check for sync errors
argocd app history <app-name>

# Force sync
argocd app sync <app-name> --force

# Check events
kubectl get events -n argocd --field-selector involvedObject.name=<app-name>
```

### Services Not Deploying

```bash
# Check Helm release
helm list -n <namespace>

# Check pod status
kubectl get pods -n <namespace>

# Check logs
kubectl logs -n <namespace> <pod-name>

# Check ingress
kubectl describe ingress -n <namespace> <ingress-name>
```

## Related Documentation

- `docs/technical/platform/endpoints-and-urls.md` - Complete endpoint and URL configuration guide
- `docs/technical/platform/tls-ssl-setup.md` - TLS/SSL configuration
- `docs/technical/platform/infrastructure-overview.md` - Infrastructure details
- `docs/technical/workflows/ci-remote.md` - Remote CI execution
- `usage-guide/architect/ci-cd-architecture.md` - CI/CD architecture

