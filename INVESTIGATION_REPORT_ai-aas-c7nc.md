# Investigation Report

**Bead**: ai-aas-c7nc
**Date**: 2025-12-14
**Investigator**: debugger agent

## Symptom

istio-ingressgateway-staging pod stuck in ImagePullBackOff for 5 days with error:
```
Failed to pull image "auto": failed to pull and unpack image "docker.io/library/auto:latest":
failed to resolve image: pull access denied, repository does not exist or may require authorization
```

Pod shows `image: auto` in deployment spec, which should have been replaced by Istio's sidecar injector.

## Reproduction

Condition occurs on fresh Istio deployment when ArgoCD applications sync in parallel without ordering:

```bash
# Staging cluster shows the issue
export KUBECONFIG=/home/dev/kubeconfigs/kubeconfig-staging.yaml
kubectl get pod istio-ingressgateway-staging-798b8c5564-xvwfm -n istio-system
# Status: ImagePullBackOff (5d20h)

kubectl get deployment istio-ingressgateway-staging -n istio-system -o yaml | grep "image:"
# Output: image: auto
```

Development cluster doesn't show the issue because it was deployed 15 days ago (webhook was ready before pods were created).

## Evidence Gathered

| Source | Finding |
|--------|---------|
| Staging pod events | Continuously trying to pull "auto" since creation |
| Deployment spec (staging) | `image: auto` not replaced by injector |
| Deployment spec (development) | Also has `image: auto` but pod runs `istio/proxyv2:1.19.0` |
| Pod creation timestamps | istio-ingressgateway: `2025-12-08T19:17:46Z`<br>istiod: `2025-12-08T19:17:48Z` (2s later)<br>Webhook cert: `2025-12-08T19:17:56Z` (10s later) |
| Pod annotations | `sidecar.istio.io/inject: "true"` and `inject.istio.io/templates: gateway` present |
| MutatingWebhookConfiguration | Configured correctly, webhook exists |
| istiod logs | No errors |
| ArgoCD applications | No sync wave annotations - all apps sync in parallel |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Image "auto" is a typo or misconfiguration | ❌ Ruled out | "auto" is correct Istio pattern - should be replaced by injector |
| Sidecar injection disabled | ❌ Ruled out | Pod has `sidecar.istio.io/inject: "true"` annotation |
| istiod not running | ❌ Ruled out | istiod pod healthy and running |
| Webhook misconfigured | ❌ Ruled out | Webhook exists with correct selectors |
| Race condition during deployment | ✅ CONFIRMED | Gateway pod created 10 seconds BEFORE webhook cert was ready |

## Root Cause

**Category**: `config_drift` (ArgoCD deployment ordering issue)

**Explanation**:

The istio-ingressgateway pod was created before the Istio sidecar injector webhook was ready. When Kubernetes tried to create the pod with `image: auto`, the webhook that should replace "auto" with the actual `istio/proxyv2:1.19.0` image wasn't operational yet.

Timeline during staging deployment (2025-12-08):
1. **19:17:46Z** - istio-ingressgateway pod created (with `image: auto`)
2. **19:17:48Z** - istiod pod created (2 seconds later)
3. **19:17:56Z** - Webhook certificate generated and ready (10 seconds after gateway)

The three ArgoCD Applications (`istio-base-staging`, `istiod-staging`, `istio-ingressgateway-staging`) have no sync wave annotations, causing them to deploy in parallel. This creates a race condition where dependent resources may be created before their dependencies are ready.

**Why "auto" is correct**:
The Istio gateway Helm chart intentionally uses `image: auto` as a placeholder. Istio's sidecar injector webhook is supposed to mutate the pod spec during creation and replace "auto" with the actual image (`docker.io/istio/proxyv2:1.19.0`).

**Evidence**:
- In development, same deployment spec has `image: auto`, but pods run successfully with `docker.io/istio/proxyv2:1.19.0`
- Development deployed 15 days ago - webhook was ready when pods were created/restarted
- Webhook certificate validity start time confirms it wasn't ready during initial pod creation

## Context Gap Check

- [x] **YES** - This was caused by missing context

**Context file**: `context/infra-ops-manager/agents.md`

**What was missing**:
1. No documentation about ArgoCD sync waves for deployment ordering
2. No pattern showing when/how to use sync waves
3. No anti-pattern warning about parallel deployment of dependent infrastructure
4. No guidance that Istio components require ordered deployment (base → istiod → gateway)

**Suggested fix**: Add to `context/infra-ops-manager/agents.md`:

```yaml
patterns:
  argocd_sync_waves:
    purpose: Control deployment order for dependent resources
    when_to_use:
      - Infrastructure with webhooks (Istio, cert-manager, Knative)
      - CRDs that must exist before CR instances
      - Operators that must be ready before managed resources
    syntax: |
      metadata:
        annotations:
          argocd.argoproj.io/sync-wave: "0"  # Lower deploys first
    example_istio:
      wave_0: istio-base (CRDs, namespace, base resources)
      wave_1: istiod (control plane, webhooks)
      wave_2: istio-ingressgateway (resources needing injection)
    wait_for_readiness: Each wave waits for resources to be healthy

anti_patterns:
  parallel_deployment_of_dependent_resources:
    wrong: |
      # Three apps with no sync waves - race condition!
      istio-base, istiod, istio-ingressgateway all sync simultaneously
    right: |
      # Ordered deployment with sync waves
      istio-base (wave 0) → ready → istiod (wave 1) → ready → gateway (wave 2)
    symptoms:
      - Pods in ImagePullBackOff with webhook-replaced values
      - Webhooks not mutating resources as expected
      - CRD not found errors
```

## Proposed Fix

**High-level description**: Add ArgoCD sync wave annotations to Istio application manifests to ensure ordered deployment.

**Affected files**:
- `/home/dev/ai-aas/gitops/clusters/staging/apps/istio.yaml` - Add sync wave annotations
- `/home/dev/ai-aas/gitops/clusters/development/apps/istio.yaml` - Add sync wave annotations (preventive)
- `/home/dev/ai-aas/gitops/clusters/production/apps/istio.yaml` - Add sync wave annotations (if exists, preventive)

**Changes needed**:

1. **istio-base** applications: Add `argocd.argoproj.io/sync-wave: "0"` annotation
2. **istiod** applications: Add `argocd.argoproj.io/sync-wave: "1"` annotation
3. **istio-ingressgateway** applications: Add `argocd.argoproj.io/sync-wave: "2"` annotation

**Immediate recovery for staging**:
Delete the broken istio-ingressgateway pod to allow it to be recreated (webhook is now ready):
```bash
kubectl delete pod istio-ingressgateway-staging-798b8c5564-xvwfm -n istio-system
```

**Estimated complexity**: Low (annotation additions + pod restart)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add e2e test for fresh Istio deployment in CI |
| Lint | Add pre-commit check warning when ArgoCD apps reference webhooks without sync waves |
| Context | Add sync wave pattern and anti-pattern to infra-ops-manager context |
| Logging | N/A - not a logging issue |
| Documentation | Document sync wave requirements in ArgoCD application template |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| ai-aas-c7nd | bug | infra-ops-manager | Fix: Add sync waves to Istio ArgoCD apps and restart staging gateway |
| ai-aas-c7ne | task | infra-ops-manager | Add pre-commit lint check for missing sync waves on webhook-dependent apps |
| ai-aas-c7nf | task | context-maintainer | Update context: Add sync wave patterns to infra-ops-manager/agents.md |
