# GitOps Model Lifecycle Tests

This document provides comprehensive instructions for running GitOps integration tests (UC-MLC-010, UC-MLC-011) both locally and in CI.

## Overview

GitOps tests validate the complete end-to-end model lifecycle workflow:
1. Models are deployed by committing YAML to the ai-aas-config repository
2. ArgoCD detects changes and creates AIModel custom resources
3. The ai-model-operator reconciles AIModel to InferenceService
4. Removing config triggers ArgoCD prune and full cleanup

**Test Coverage:**
- **UC-MLC-010**: Deploy Model via GitOps (3 acceptance criteria)
- **UC-MLC-011**: Remove Model via GitOps (3 acceptance criteria)

**Test Duration:** 15-25 minutes (includes model deployment and cleanup timeouts)

## Prerequisites

### Required Components

1. **ai-aas-config repository** cloned locally
   ```bash
   git clone https://github.com/otherjamesbrown/ai-aas-config.git ~/ai-aas-config
   cd ~/ai-aas-config
   git checkout develop
   ```

2. **Development cluster kubeconfig**
   - Located at: `secrets/kubeconfigs/kubeconfig-development.yaml`
   - Or at: `~/kubeconfigs/kubeconfig-development.yaml`

3. **Git credentials** for ai-aas-config repository
   - Must have push access to `otherjamesbrown/ai-aas-config`
   - Configured via SSH key or GitHub Personal Access Token

4. **ArgoCD** configured to sync from ai-aas-config
   - Must watch `environments/development/models/` directory
   - Sync policy: automated with prune enabled

5. **Kyverno** (optional, for AC-03 of UC-MLC-010)
   - Required only if testing the "non-GitOps creation blocked" policy

### Verification Commands

Verify your setup before running tests:

```bash
# 1. Check ai-aas-config repository exists and is on develop branch
ls ~/ai-aas-config/environments/development/models/
git -C ~/ai-aas-config branch --show-current  # Should output: develop

# 2. Check kubeconfig works
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml cluster-info

# 3. Check git credentials (should not prompt for password)
git -C ~/ai-aas-config pull origin develop

# 4. Check ArgoCD is syncing
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  -n argocd get applications -l environment=development
```

## Running Tests Locally

### Quick Start

From the ai-aas repository root:

```bash
# Set required environment variables
export RUN_GITOPS_TESTS=1
export AI_AAS_CONFIG_PATH=~/ai-aas-config
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml

# Optional: Enable Kyverno policy test (UC-MLC-010/AC-03)
export KYVERNO_POLICY_DEPLOYED=1

# Optional: Set API Router URL for inference verification (UC-MLC-011/AC-03)
export AI_AAS_API_ROUTER_URL=https://api.dev.otherjamesbrown.com

# Run GitOps tests
cd tests/usecases
go test -v ./... -run "TestUC_MLC_01" -timeout 30m
```

### Running Specific Tests

```bash
# Run only UC-MLC-010 (Deploy Model)
go test -v ./... -run "TestUC_MLC_010" -timeout 20m

# Run only UC-MLC-011 (Remove Model)
go test -v ./... -run "TestUC_MLC_011" -timeout 20m

# Run only Kyverno policy test (UC-MLC-010/AC-03)
go test -v ./... -run "TestUC_MLC_010_AC03" -timeout 5m
```

### Full Command with All Options

```bash
RUN_GITOPS_TESTS=1 \
KYVERNO_POLICY_DEPLOYED=1 \
AI_AAS_CONFIG_PATH=~/ai-aas-config \
KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml \
AI_AAS_API_ROUTER_URL=https://api.dev.otherjamesbrown.com \
go test -v ./... -run "TestUC_MLC_01" -timeout 30m
```

## Environment Variables Reference

| Variable | Required | Default | Purpose |
|----------|----------|---------|---------|
| `RUN_GITOPS_TESTS` | Yes | (unset) | Must be `1` to enable GitOps tests |
| `AI_AAS_CONFIG_PATH` | Yes | `~/ai-aas-config` | Path to ai-aas-config repository |
| `KUBECONFIG` | Yes | `~/kubeconfigs/kubeconfig-development.yaml` | Path to development cluster kubeconfig |
| `KYVERNO_POLICY_DEPLOYED` | No | (unset) | Set to `1` to enable Kyverno policy test |
| `AI_AAS_API_ROUTER_URL` | No | (unset) | API Router URL for inference verification (UC-MLC-011/AC-03) |

## Test Workflow Details

### UC-MLC-010: Deploy Model via GitOps

**AC-01: AIModel config added to ai-aas-config creates deployment**
1. Generates unique TinyLlama model YAML with timestamp suffix
2. Commits file to `ai-aas-config/environments/development/models/tinyllama-gitops-test-<timestamp>.yaml`
3. Pushes to develop branch
4. Waits up to 3 minutes for ArgoCD to detect and create AIModel CR
5. Verifies AIModel CR matches the committed spec

**AC-02: Deployed model reaches Ready phase**
1. Waits up to 10 minutes for AIModel to reach Ready phase
2. Verifies InferenceService is created
3. Verifies pod is running

**AC-03: Non-GitOps AIModel creation is blocked** (requires `KYVERNO_POLICY_DEPLOYED=1`)
1. Attempts to create AIModel directly via `kubectl apply`
2. Verifies Kyverno policy rejects the request
3. Checks error message mentions GitOps policy

### UC-MLC-011: Remove Model via GitOps

**Setup:** Deploys a test model first (reuses UC-MLC-010 logic)

**AC-01: Removing config from ai-aas-config triggers deletion**
1. Deletes the model YAML file from ai-aas-config
2. Commits and pushes to develop branch
3. Waits up to 8 minutes for ArgoCD to prune the AIModel CR

**AC-02: Operator cleans up all resources**
1. Verifies InferenceService is deleted
2. Verifies all pods with the model label are deleted
3. Checks for any orphaned resources

**AC-03: Model no longer accessible via inference** (requires `AI_AAS_API_ROUTER_URL`)
1. Attempts inference request to the deleted model
2. Expects 404, 503, 502, or network error (model service is gone)
3. Fails if the model returns 200 OK (model should not be accessible)

## Cleanup

Tests automatically clean up their model files using Go's `t.Cleanup()` hook:
- Runs even if tests fail
- Removes test model YAML from ai-aas-config
- Commits and pushes cleanup changes

**Manual cleanup** if tests are interrupted:
```bash
# Remove orphaned test model files
cd ~/ai-aas-config
git pull origin develop
find environments/development/models -name 'tinyllama-gitops-test-*.yaml' -delete
git add -A
git commit -m "test cleanup: remove orphaned GitOps test models"
git push origin develop

# Delete orphaned AIModel CRs in cluster
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml \
  -n development delete aimodel -l test=gitops-lifecycle
```

## CI/CD Integration

GitOps tests run automatically as a **nightly job** at 3 AM UTC via `.github/workflows/nightly-gitops.yml`.

### GitHub Actions Workflow

The nightly workflow:
1. Checks out both ai-aas and ai-aas-config repositories
2. Configures kubeconfig from `DEV_KUBECONFIG_B64` secret
3. Sets up git credentials using `AI_AAS_CONFIG_PAT` token
4. Runs GitOps tests with 60-minute timeout
5. Uploads results to Linode Object Storage (`ai-aas-models` bucket, `gitops-results/` prefix)
6. Posts annotations to Grafana (if `GRAFANA_API_KEY` is configured)
7. Cleans up any orphaned test files from ai-aas-config

### Required GitHub Secrets

The workflow requires these secrets to be configured:

| Secret | Purpose |
|--------|---------|
| `AI_AAS_CONFIG_PAT` | GitHub Personal Access Token with `repo` scope for ai-aas-config access |
| `DEV_KUBECONFIG_B64` | Base64-encoded development cluster kubeconfig |
| `DEV_KUBE_CONTEXT` | Kubernetes context name (e.g., `lke258421-ctx`) |
| `LINODE_OBJECT_STORAGE_ACCESS_KEY` | Linode Object Storage access key for test results |
| `LINODE_OBJECT_STORAGE_SECRET_KEY` | Linode Object Storage secret key |
| `GRAFANA_API_KEY` | (Optional) Grafana API key for annotations |
| `GRAFANA_URL` | (Optional) Grafana instance URL |

See [docs/platform/github-secrets-setup.md](../../docs/platform/github-secrets-setup.md) for secret setup instructions.

### Manual CI Trigger

Trigger GitOps tests manually via GitHub Actions:

```bash
# Using GitHub CLI
gh workflow run nightly-gitops.yml -f environment=development

# Using GitHub web UI
# 1. Go to: https://github.com/otherjamesbrown/ai-aas/actions/workflows/nightly-gitops.yml
# 2. Click "Run workflow"
# 3. Select "develop" branch
# 4. Choose environment: "development"
# 5. Click "Run workflow"
```

## Troubleshooting

### Tests Skip with "RUN_GITOPS_TESTS not set"

**Cause:** Environment variable `RUN_GITOPS_TESTS` is not set to `1`

**Solution:**
```bash
export RUN_GITOPS_TESTS=1
go test -v ./... -run "TestUC_MLC_01"
```

### "Failed to add model config to ai-aas-config"

**Cause:** Git credentials not configured or ai-aas-config repo not found

**Solution:**
```bash
# Verify repo exists
ls ~/ai-aas-config

# Test git push access
git -C ~/ai-aas-config pull origin develop
git -C ~/ai-aas-config push --dry-run origin develop
```

### "AIModel was not created by ArgoCD sync"

**Cause:** ArgoCD is not syncing from ai-aas-config or sync is disabled

**Solution:**
```bash
# Check ArgoCD applications
kubectl -n argocd get applications -l environment=development

# Force sync the models application
argocd app sync ai-models-development --force
```

### "AIModel did not reach Ready phase"

**Cause:** Model deployment failed, GPU not available, or operator not running

**Solution:**
```bash
# Check AIModel status
kubectl get aimodel tinyllama-gitops-test-<timestamp> -n development -o yaml

# Check operator logs
kubectl -n ai-aas-system logs -l app=ai-model-operator --tail=50

# Check InferenceService status
kubectl get inferenceservice tinyllama-gitops-test-<timestamp> -n development -o yaml
```

### "timeout waiting for AIModel to be deleted"

**Cause:** ArgoCD prune is disabled or finalizers are blocking deletion

**Solution:**
```bash
# Check ArgoCD sync policy
kubectl -n argocd get application ai-models-development -o jsonpath='{.spec.syncPolicy.automated.prune}'

# Check AIModel finalizers
kubectl get aimodel tinyllama-gitops-test-<timestamp> -n development -o jsonpath='{.metadata.finalizers}'

# Force delete if stuck (last resort)
kubectl delete aimodel tinyllama-gitops-test-<timestamp> -n development --grace-period=0 --force
```

### Test Results Location

**Local:**
- Test output: stdout
- No artifacts saved locally

**CI (GitHub Actions):**
- GitHub Artifacts: `gitops-results-development-YYYY-MM-DD` (90 day retention)
- Object Storage: `s3://ai-aas-models/gitops-results/nightly/development/YYYY-MM-DD/`
- Latest results: `s3://ai-aas-models/gitops-results/nightly/development/latest/metrics.json`

## Related Documentation

- [usecases/model-lifecycle.yaml](../../usecases/model-lifecycle.yaml) - UC-MLC-010 and UC-MLC-011 definitions
- [tests/usecases/README.md](README.md) - Use case test overview
- [docs/platform/github-secrets-setup.md](../../docs/platform/github-secrets-setup.md) - GitHub secrets configuration
- [.github/workflows/nightly-gitops.yml](../../.github/workflows/nightly-gitops.yml) - CI workflow definition
