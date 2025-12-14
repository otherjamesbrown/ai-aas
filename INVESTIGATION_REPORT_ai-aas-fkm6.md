# Investigation Report

**Bead**: ai-aas-fkm6
**Date**: 2025-12-14
**Investigator**: debugger agent
**Environment**: Staging

## Symptom

analytics-service pod stuck in Init:ImagePullBackOff for 5+ days in staging cluster. Error message: Cannot pull 'ghcr.io/otherjamesbrown/ai-aas/analytics-service:staging'

## Reproduction

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-staging.yaml
kubectl get pods -n analytics-service
# Shows: analytics-service-staging-analytics-service-d5df7f465-9c4c9   0/1     Init:ImagePullBackOff
```

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `kubectl get pods -n analytics-service` | 2 pods Running (old RS), 1 pod Init:ImagePullBackOff (new RS) |
| `kubectl get rs -n analytics-service` | Old RS (778c8dbcc6) uses `ghcr.io/otherjamesbrown/analytics-service:latest` |
| | New RS (d5df7f465) uses `ghcr.io/otherjamesbrown/ai-aas/analytics-service:staging` |
| `kubectl describe pod <failing-pod>` | Error: "401 Unauthorized - failed to fetch anonymous token" |
| `kubectl get sa -n analytics-service -o yaml` | ServiceAccount has NO imagePullSecrets configured |
| `kubectl get secrets -n analytics-service` | No ghcr-pull-secret exists in namespace |
| `kubectl get secrets -A \| grep ghcr` (staging) | NO ghcr-pull-secret anywhere in cluster |
| `kubectl get secrets -A \| grep ghcr` (development) | ghcr-pull-secret exists in 3 namespaces |
| `values-development.yaml:22-23` | Has `pullSecrets: [ghcr-pull-secret]` |
| `values-staging.yaml` | MISSING pullSecrets configuration |
| GitHub Actions workflow run 20210479563 | Image successfully built and pushed at 2025-12-14 16:05:38 UTC |
| `docker pull ghcr.io/otherjamesbrown/ai-aas/analytics-service:staging` | Image pulls successfully from local machine |
| Test pod in staging cluster | 401 Unauthorized - repository requires authentication |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Image doesn't exist in GHCR | ❌ Ruled out | GH Actions shows successful push; local docker pull succeeds |
| Image tag is wrong | ❌ Ruled out | Deployment spec matches pushed tag exactly |
| Repository is private, needs auth | ✅ CONFIRMED | 401 error from staging cluster; development has ghcr-pull-secret |
| imagePullSecrets not configured | ✅ CONFIRMED | ServiceAccount lacks imagePullSecrets; staging has no secret |
| Config drift between dev/staging | ✅ CONFIRMED | values-development.yaml has pullSecrets; values-staging.yaml missing it |

## Root Cause

**Category**: `config_drift`

**Explanation**: The staging Helm values file (`values-staging.yaml`) is missing the `image.pullSecrets` configuration that exists in `values-development.yaml`. The GHCR repository `ghcr.io/otherjamesbrown/ai-aas/*` requires authentication for pulls, but staging cluster has:

1. NO `ghcr-pull-secret` secret in the analytics-service namespace (or anywhere in cluster)
2. NO `imagePullSecrets` configured in values-staging.yaml
3. NO `imagePullSecrets` in the ServiceAccount

When ArgoCD deployed the updated analytics-service Helm chart to staging, the Deployment template (lines 21-26) only adds `imagePullSecrets` if `.Values.image.pullSecrets` is set. Since staging values don't set this, pods cannot authenticate to GHCR and receive 401 Unauthorized errors.

**Evidence**:
- Development values-development.yaml line 22-23: `pullSecrets: [ghcr-pull-secret]` ✓
- Staging values-staging.yaml: No pullSecrets configuration ✗
- Test pod in staging: "401 Unauthorized - failed to fetch anonymous token"

**Why old pods still work**: Previous ReplicaSet (778c8dbcc6) uses old image path `ghcr.io/otherjamesbrown/analytics-service:latest` which may have been pulled before repository became private, or was from a different (public) repository.

## Context Gap Check

- [X] Was this caused by missing context? **PARTIAL**

**Context file**: Multiple files need updates

**What was missing**:
1. No documentation on how to set up GHCR authentication for new environments
2. No validation in CI to ensure pullSecrets are configured consistently across all environment values files
3. No runbook for ImagePullBackOff issues related to private registries

**Suggested fix**:
1. Add to `docs/runbooks/` - "GHCR Authentication Setup" runbook
2. Add to `context/infra-ops-manager/agents.md` - Pattern for ensuring imagePullSecrets in all env values
3. Add lint check to verify critical fields (pullSecrets, resources, etc.) exist in all values-{env}.yaml files

## Proposed Fix

**High-level description**:

### Fix 1: Add ghcr-pull-secret to staging cluster
1. Create GHCR token with read:packages permission
2. Create Kubernetes secret in analytics-service namespace:
   ```bash
   kubectl create secret docker-registry ghcr-pull-secret \
     --docker-server=ghcr.io \
     --docker-username=<GITHUB_USERNAME> \
     --docker-password=<GHCR_TOKEN> \
     -n analytics-service
   ```
3. Repeat for other namespaces that deploy from GHCR

### Fix 2: Add pullSecrets to values-staging.yaml
Update `services/analytics-service/deployments/helm/analytics-service/values-staging.yaml`:
```yaml
image:
  repository: ghcr.io/otherjamesbrown/ai-aas/analytics-service
  tag: staging
  pullPolicy: Always
  pullSecrets:
    - ghcr-pull-secret
```

### Fix 3 (Long-term): Automate secret creation
- Add ghcr-pull-secret creation to infrastructure automation
- Use SealedSecrets or External Secrets Operator for GitOps-friendly secret management
- Document in environment setup runbook

**Affected files**:
- `services/analytics-service/deployments/helm/analytics-service/values-staging.yaml` - add pullSecrets
- Staging cluster - create ghcr-pull-secret in analytics-service namespace
- (Optional) Other service values-staging.yaml files - verify they have pullSecrets

**Estimated complexity**: Low (for immediate fix), Medium (for long-term automation)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add integration test that verifies images can be pulled in each environment |
| Lint | Add CI check to validate pullSecrets field exists in all values-{env}.yaml files |
| Context | Add runbook: "Setting up GHCR authentication for new environments" |
| Context | Add anti-pattern: "Never omit pullSecrets when using private registries" |
| Observability | Alert on ImagePullBackOff events lasting >5 minutes |
| Automation | Use SealedSecrets/External Secrets for consistent secret deployment |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| TBD | bug | infra-ops-manager | Implement Fix 1 & 2: Add ghcr-pull-secret to staging and update values-staging.yaml |
| TBD | task | infra-ops-manager | Add Helm values linter to check pullSecrets consistency |
| TBD | task | infra-ops-manager | Create runbook: GHCR authentication setup |
| TBD | task | infra-ops-manager | Audit all services' values-staging.yaml for missing pullSecrets |

---

## Summary

**Root Cause**: Missing `image.pullSecrets` configuration in `values-staging.yaml` combined with missing `ghcr-pull-secret` in staging cluster prevents pods from authenticating to private GHCR repository.

**Impact**: New deployments stuck in ImagePullBackOff; service continues running on old pods with cached images.

**Fix**: Add ghcr-pull-secret to staging cluster and update values-staging.yaml to include pullSecrets configuration.

**Prevention**: Add CI validation for pullSecrets consistency and document GHCR authentication setup process.
