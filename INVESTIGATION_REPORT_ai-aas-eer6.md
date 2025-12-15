# Investigation Report

**Bead**: ai-aas-eer6
**Date**: 2025-12-14
**Investigator**: debugger agent

## Symptom

ArgoCD applications showing OutOfSync in staging cluster:
- `kserve-staging` - CRD `inferenceservices.serving.kserve.io` OutOfSync
- `kserve-config-staging` - Multiple PodMonitor resources failing to sync
- `gpu-operator-staging` - ClusterPolicy `cluster-policy` OutOfSync

## Reproduction

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-staging.yaml
kubectl get application -n argocd | grep -E "kserve|gpu-operator"
# Shows OutOfSync status for all three applications
```

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `kserve-staging` app status | CRD inferenceservices.serving.kserve.io marked OutOfSync |
| CRD in cluster | Has `caBundle` field injected in `.spec.conversion.webhook.clientConfig` |
| Git source `infra/k8s/kserve/install/kserve.yaml:3067` | Does NOT have `caBundle` field |
| CRD annotations | `cert-manager.io/inject-ca-from: kserve/serving-cert` present |
| autoHealAttemptsCount | 7 attempts - indicates continuous sync thrashing |
| kserve-config-staging sync result | `PodMonitor.monitoring.coreos.com "" not found` |
| kubectl get crd podmonitors | CRD does not exist in staging cluster |
| gpu-operator ClusterPolicy | Has `.status` field with reconciliation state |
| ArgoCD version staging | v3.2.1 |
| ArgoCD version development | v2.11.0 |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Manual changes to cluster | ❌ Ruled out | Changes are systematic (webhook injections, controller status updates) |
| cert-manager injecting caBundle | ✅ CONFIRMED | Annotation present, caBundle field added after sync |
| Webhook modifying after sync | ✅ CONFIRMED | cert-manager mutating webhook active |
| PodMonitor CRD missing | ✅ CONFIRMED | CRD podmonitors.monitoring.coreos.com not found |
| Controller updating status | ✅ CONFIRMED | gpu-operator adds `.status` field to ClusterPolicy |
| ArgoCD v3 behavior change | ✅ CONFIRMED | Development (v2.11) shows Synced, Staging (v3.2.1) shows OutOfSync with same config |

## Root Cause

**Category**: `config_drift` + `architecture`

**Primary Cause - kserve-staging**: ArgoCD v3 sync loop with cert-manager webhook

ArgoCD v3.2.1 in staging has stricter drift detection than v2.11.0 in development. The cert-manager mutating webhook injects `caBundle` into the InferenceService CRD immediately after ArgoCD syncs it from Git. ArgoCD v3 detects this as drift and marks OutOfSync, whereas v2.11 ignores or handles this injection differently.

**Sync loop**:
1. ArgoCD syncs CRD from Git (no caBundle)
2. cert-manager sees annotation `cert-manager.io/inject-ca-from: kserve/serving-cert`
3. cert-manager mutating webhook injects caBundle for TLS verification
4. ArgoCD v3 detects drift, marks OutOfSync
5. ArgoCD auto-heals (with selfHeal: true), removes caBundle
6. Loop repeats (7 attempts observed)

**Secondary Cause - kserve-config-staging**: Missing Prometheus Operator

PodMonitor CRD (`monitoring.coreos.com`) does not exist in staging cluster. The kserve-config application tries to create PodMonitor resources for:
- `nvidia-dcgm-exporter` (gpu-operator namespace)
- `kserve-queue-proxy-metrics` (development namespace)
- `kserve-vllm-metrics` (development namespace)

Prometheus Operator must be installed to provide the PodMonitor CRD.

**Tertiary Cause - gpu-operator-staging**: Controller status updates

The gpu-operator controller adds `.status` field to the ClusterPolicy resource at runtime. ArgoCD v3 detects this as drift because Git source only has `.spec`. This is expected Kubernetes controller behavior but ArgoCD v3 is more strict about detecting it.

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: `context/infra-ops-manager/agents.md` and `docs/runbooks/argocd-best-practices.md`

**What was missing**:
1. No documentation about ArgoCD v3 behavior changes with mutating webhooks
2. No pattern for using `ignoreDifferences` to handle cert-manager injections
3. No warning about ArgoCD version differences between environments
4. No guidance on handling controller status updates in GitOps

**Suggested fix**: Add patterns for:
- Using `ignoreDifferences` for cert-manager caBundle injections
- Using `ignoreDifferences` for controller status fields
- Environment-specific considerations for ArgoCD versions
- Prometheus Operator as prerequisite for PodMonitor resources

## Proposed Fix

**High-level approach**:

### 1. Fix kserve-staging OutOfSync (cert-manager caBundle)

Add `ignoreDifferences` to ArgoCD Application to ignore cert-manager injections:

```yaml
# gitops/clusters/staging/apps/kserve.yaml
spec:
  ignoreDifferences:
  - group: apiextensions.k8s.io
    kind: CustomResourceDefinition
    name: inferenceservices.serving.kserve.io
    jsonPointers:
    - /spec/conversion/webhook/clientConfig/caBundle
```

### 2. Fix kserve-config-staging PodMonitor failures

**Option A**: Install Prometheus Operator in staging
**Option B**: Make PodMonitor resources conditional/optional
**Option C**: Move PodMonitor resources to separate app with dependencies

Recommended: **Option A** - Install Prometheus Operator for full observability parity with development.

### 3. Fix gpu-operator-staging OutOfSync (status field)

Add `ignoreDifferences` for status subresource:

```yaml
# gitops/clusters/staging/apps/gpu-operator.yaml
spec:
  ignoreDifferences:
  - group: nvidia.com
    kind: ClusterPolicy
    jsonPointers:
    - /status
```

### 4. Document ArgoCD v3 patterns

Create runbook or context doc explaining:
- ArgoCD v3 stricter drift detection
- Common `ignoreDifferences` patterns for webhooks
- Standard approach for status fields
- Environment version alignment considerations

**Affected files**:
- `gitops/clusters/staging/apps/kserve.yaml`
- `gitops/clusters/staging/apps/kserve-config.yaml`
- `gitops/clusters/staging/apps/gpu-operator.yaml`
- `docs/runbooks/argocd-best-practices.md` (new or update existing)
- `context/infra-ops-manager/agents.md`

**Estimated complexity**: Low-Medium
- ignoreDifferences changes: Low (simple YAML edits)
- Prometheus Operator install: Medium (new infrastructure component)
- Documentation: Low

## Prevention

How to prevent this class of issue in future:

| Type | Action |
|------|--------|
| Test | Add pre-deployment checks for CRD prerequisites (PodMonitor, etc.) |
| Lint | Validate ignoreDifferences configured for known webhook injections |
| Context | Add ArgoCD v3 patterns and cert-manager handling to infra-ops docs |
| Architecture | Standardize ArgoCD versions across environments OR document version-specific behavior |
| CI/CD | Add ArgoCD app validation in CI to detect missing ignoreDifferences |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| TBD | bug | infra-ops-manager | Add ignoreDifferences to kserve-staging for caBundle |
| TBD | bug | infra-ops-manager | Add ignoreDifferences to gpu-operator-staging for status |
| TBD | task | infra-ops-manager | Install Prometheus Operator in staging OR make PodMonitors optional |
| TBD | task | infra-ops-manager | Update ArgoCD best practices docs with v3 patterns |
| TBD | task | context-maintainer | Add ArgoCD v3 and cert-manager patterns to infra-ops context |
| TBD | task | infra-ops-manager | Consider standardizing ArgoCD versions across environments |
