# Investigation Report

**Bead**: ai-aas-e6e3
**Date**: 2025-12-14
**Investigator**: debugger agent

## Symptom

AI Model Operator in staging environment showing 401 authentication errors when attempting to sync deployment status to Admin API:

```
Failed to sync deployment to Admin API: API error (status 401): {"type":"https://docs.ai-aas.local/errors/unauthorized","title":"Unauthorized","status":401,"detail":"invalid or revoked API key"}
```

This works correctly in development environment.

## Reproduction

1. Deploy AIModel CR in staging
2. Operator attempts to sync deployment status to Admin API
3. Admin API returns 401 with "invalid or revoked API key"
4. Error repeats on every reconciliation attempt

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `kubectl get pods -n admin-api-service` (staging) | Admin API running healthy (2/2 pods) |
| `kubectl get pods -n ai-model-system` (staging) | Operator running (1/1 pod, restart due to other issues) |
| `kubectl get secret admin-api-credentials -n ai-model-system` (staging) | Secret exists with key `ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M` |
| `kubectl get secret admin-api-secrets -n admin-api-service` (staging) | Secret exists with master key `Tnwhgd7xKwMXGORtKwt8rBJC4aNQNE94LygW0usA` |
| `services/admin-api-service/internal/api/router.go:137` | Admin API uses `masterKeyValidator` that only accepts a single hardcoded master key from config |
| `kubectl logs operator` (staging) | Multiple 401 errors when calling Admin API |
| `kubectl logs admin-api` (staging) | NO 401 errors logged - health checks succeed, but no operator requests logged |
| Development cluster comparison | Operator key and Admin API master key **MATCH** (`VXDzIauNfwRdmUDowO37plULPXbf1fUBr-69oqSEWEA`) |
| Port-forward test with staging key | Direct test confirms: staging operator key returns 401 "invalid or revoked API key" |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Admin API not running in staging | ❌ Ruled out | 2/2 pods healthy, responding to health checks |
| Operator secret missing | ❌ Ruled out | Secret exists in ai-model-system namespace |
| Wrong authentication format | ❌ Ruled out | Operator sends correct `Authorization: Bearer <key>` format |
| API key not registered in database | ❌ Ruled out | Admin API doesn't use database for auth - uses hardcoded master key |
| **Configuration mismatch between secrets** | ✅ **CONFIRMED** | Operator key != Admin API master key in staging |

## Root Cause

**Category**: `config_drift`

**Explanation**:

The Admin API service uses a simple `masterKeyValidator` that validates all incoming requests against a single master admin API key stored in its configuration (`MASTER_ADMIN_API_KEY` environment variable, sourced from `admin-api-secrets.master-admin-api-key` in the `admin-api-service` namespace).

The AI Model Operator authenticates to the Admin API using an API key stored in the `admin-api-credentials.api-key` secret in the `ai-model-system` namespace.

In the staging environment, these two secrets contain **different values**:
- Admin API expects: `Tnwhgd7xKwMXGORtKwt8rBJC4aNQNE94LygW0usA`
- Operator provides: `ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M`

In the development environment, they **match** (`VXDzIauNfwRdmUDowO37plULPXbf1fUBr-69oqSEWEA`), which is why authentication succeeds there.

Both secrets appear to have been created manually (imperative `kubectl create secret`) rather than through GitOps, leading to configuration drift when one was updated without updating the other.

**Evidence**:
- Code analysis: `services/admin-api-service/internal/api/router.go:137-149`
- Direct comparison of secret values via `kubectl get secret -o jsonpath`
- No GitOps manifests or scripts found for creating/syncing these secrets

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: `docs/platform/environment-access.md` or deployment runbooks

**What was missing**:
1. No documentation that Admin API uses a master key validator (not database-backed auth)
2. No documentation of the relationship between `admin-api-secrets` and `admin-api-credentials`
3. No documented procedure for creating/syncing operator API keys across environments
4. No GitOps management of these secrets (they're created imperatively)

**Suggested fix**:
- Document the Admin API authentication model in `docs/platform/environment-access.md`
- Add to `docs/runbooks/deploy-to-environments.md` a section on secret dependencies
- Consider adding validation step to deployment checklist: "Verify operator API key matches Admin API master key"

## Proposed Fix

**High-level description**: Update the staging operator's API key secret to match the Admin API's master key.

**Affected secrets**:
- `admin-api-credentials` in namespace `ai-model-system` (staging cluster)
  - Current value: `ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M`
  - Should be: `Tnwhgd7xKwMXGORtKwt8rBJC4aNQNE94LygW0usA`

**Steps**:
1. Delete existing operator secret: `kubectl delete secret admin-api-credentials -n ai-model-system`
2. Recreate with correct value: `kubectl create secret generic admin-api-credentials -n ai-model-system --from-literal=api-key=<MASTER_KEY_FROM_ADMIN_API>`
3. Restart operator to pick up new secret: `kubectl rollout restart deployment -n ai-model-system -l app.kubernetes.io/name=ai-model-operator`
4. Verify: Check operator logs for successful sync

**Estimated complexity**: Low (simple secret update and pod restart)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add integration test that verifies operator can authenticate to Admin API in staging |
| Lint | Add CI check that validates secret references exist and are consistent across environments |
| Context | Add documentation of Admin API authentication model and secret dependencies |
| Logging | Admin API should log failed auth attempts with key prefix (not full key) for debugging |
| Architecture | Consider replacing hardcoded master key with proper API key management (database-backed, per-service keys) |
| GitOps | Manage these secrets via Sealed Secrets or External Secrets Operator instead of imperative creation |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| (to be created) | bug | infra-ops-manager | Fix staging operator API key to match Admin API master key |
| (to be created) | task | infra-ops-manager | Add secret sync validation to deployment checklist |
| (to be created) | task | context-maintainer | Document Admin API auth model and secret dependencies |
| (to be created) | feature | go-services-developer | Replace master key validator with proper API key management (long-term) |
