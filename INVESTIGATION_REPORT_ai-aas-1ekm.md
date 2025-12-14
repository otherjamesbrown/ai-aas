# Investigation Report

**Bead**: ai-aas-1ekm
**Date**: 2025-12-14
**Investigator**: debugger agent
**Related Issue**: ai-aas-53sw

## Symptom

When memory limits were changed from 32Gi → 24Gi in ai-aas-config repository:
1. New Knative Revisions were created (revisions 00150+)
2. Old Revisions with 32Gi limits remained running (revision 00001)
3. KServe rolling update appeared stuck - old revisions were not terminated
4. This caused ongoing OOM failures because traffic was split between old and new pods

Current state observed:

| Model | Old Revision (32Gi) | Restarts | New Revisions (Status) |
|-------|---------------------|----------|------------------------|
| mistral-7b-instruct-v03 | 00001 (Running) | 367 | 00153 (Pending) |
| openai-gpt-oss-20b | 00001 (Running) | 374 | 00173 (Pending) |
| unsloth-gpt-oss-20b | 00001 (Running) | 425 | 00159 (Pending) |

## Reproduction

1. Change memory limit in AIModel CR from 32Gi → 24Gi
2. Push to ai-aas-config repository
3. Wait for GitOps sync
4. Observe that new revisions are created but remain Pending
5. Old revisions continue running and serving traffic

## Evidence Gathered

### 1. InferenceService Status

All three InferenceServices show identical pattern:

```yaml
status:
  components:
    predictor:
      latestCreatedRevision: mistral-7b-instruct-v03-predictor-00153
      latestReadyRevision: mistral-7b-instruct-v03-predictor-00001
      latestRolledoutRevision: mistral-7b-instruct-v03-predictor-00001
      traffic:
      - latestRevision: true
        percent: 100
        revisionName: mistral-7b-instruct-v03-predictor-00001
```

**Key finding**: Despite `latestRevision: true`, traffic remains on 00001 because it's the ONLY ready revision.

### 2. Knative Revision Status

| Revision | Ready | Reason | Replicas |
|----------|-------|--------|----------|
| mistral-7b-instruct-v03-predictor-00001 | True | - | 1 |
| mistral-7b-instruct-v03-predictor-00153 | False | Unschedulable | 0 |
| openai-gpt-oss-20b-predictor-00001 | True | - | 1 |
| openai-gpt-oss-20b-predictor-00173 | False | Unschedulable | 0 |
| unsloth-gpt-oss-20b-predictor-00001 | True | - | 1 |
| unsloth-gpt-oss-20b-predictor-00159 | False | Unschedulable | 0 |

All new revisions fail with:
```
0/8 nodes are available: 8 Insufficient cpu, 8 Insufficient memory, 8 Insufficient nvidia.com/gpu
```

### 3. Cluster Resource Capacity

| Node | CPU | Memory | GPU | Current Pod |
|------|-----|--------|-----|-------------|
| lke531921-776664-2d6917d80000 | 8 | 32GB | 1 | (available) |
| lke531921-776664-37342cf80000 | 8 | 32GB | 1 | (available) |
| lke531921-776664-46225a090000 | 8 | 32GB | 1 | mistral-00001 |
| lke531921-776664-51386eeb0000 | 8 | 32GB | 1 | openai-00001 |
| lke531921-776664-59eb445b0000 | 8 | 32GB | 1 | unsloth-00001 |

**Key finding**: 5 GPU nodes, but only 3 GPUs occupied. Yet new pods cannot schedule.

### 4. AIModel CR Configuration

Current state in cluster:

```bash
kubectl get aimodel -n development -o yaml | grep -A 5 "memory:"
```

**All three AIModels show**: `memory: 32Gi` (NOT 24Gi as expected)

### 5. ArgoCD Application Conflict

```bash
kubectl get applications -n argocd | grep model
ai-model-operator-development       Synced        Healthy
aimodels-config-development         Synced        Healthy
aimodels-development                OutOfSync     Healthy
```

**SharedResourceWarning** detected:
```
AIModel/mistral-7b-instruct-v03 is part of applications argocd/aimodels-development and aimodels-config-development
```

Two ArgoCD applications managing the same resources:

| Application | Source Repo | Path | Memory Config | Status |
|-------------|-------------|------|---------------|--------|
| aimodels-development | ai-aas | infra/k8s/aimodels/development | 32Gi (STALE) | OutOfSync |
| aimodels-config-development | ai-aas-config | environments/development/models | 24Gi (NEW) | Synced |

### 6. Git Repository State

**ai-aas-config** (commit febe54a):
```yaml
# environments/development/models/mistral-7b-instruct-v03.yaml
  limits:
    memory: "24Gi"  # ✅ Updated
```

**ai-aas** (current):
```yaml
# infra/k8s/aimodels/development/mistral-7b-instruct-v03.yaml
  limits:
    memory: "32Gi"  # ❌ Stale
```

### 7. Knative Garbage Collection Config

```yaml
# config-gc ConfigMap in knative-serving namespace
max-non-active-revisions: "2"
min-non-active-revisions: "1"
retain-since-create-time: 10m
retain-since-last-active-time: 5m
```

**GC is properly configured** but cannot act because revision 00001 is ACTIVE (serving traffic).

### 8. KServe Controller Behavior

Controller logs show repeated configuration drift detection:

```json
{
  "logger":"KsvcReconciler",
  "msg":"knative service configuration diff (-desired, +observed)",
  "diff":"memory: 32Gi (observed) vs 24Gi (desired)"
}
```

Controller creates new revisions but they remain unschedulable. **No errors in controller** - it's working correctly.

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Knative GC disabled or misconfigured | ❌ Ruled out | GC config is correct: max-non-active-revisions=2 |
| Traffic not shifting to new revisions | ❌ Ruled out | Traffic CANNOT shift - new revisions not ready |
| GPU resource exhaustion blocking rollout | ⚠️ PARTIAL | GPUs available but pods request 32Gi (wrong spec) |
| KServe/Knative operator errors | ❌ Ruled out | Operators working correctly, no errors |
| Configuration issue in InferenceService | ❌ Ruled out | InferenceService is managed by AIModel operator |
| Configuration conflict in GitOps | ✅ CONFIRMED | Two ArgoCD apps managing same resources |
| AIModel CR not updated with new memory limits | ✅ CONFIRMED | AIModel still shows 32Gi, not 24Gi |

## Root Cause

**Category**: `config_drift`

**Explanation**:

The system is stuck in a rollout deadlock caused by **conflicting ArgoCD Applications** managing the same AIModel CRs. Here's the exact sequence:

1. **Conflicting Sources**: Two ArgoCD applications manage the same AIModel resources:
   - `aimodels-development` syncs from `ai-aas/infra/k8s/aimodels/development/` (contains stale 32Gi config)
   - `aimodels-config-development` syncs from `ai-aas-config/environments/development/models/` (contains new 24Gi config)

2. **Config Battle**: Both applications continuously sync, creating a race condition:
   - User updates ai-aas-config to 24Gi → `aimodels-config-development` syncs → AIModel CR updated to 24Gi
   - Moments later → `aimodels-development` syncs → AIModel CR reverted to 32Gi
   - This happens repeatedly, with the stale config "winning" most of the time

3. **Operator Responds**: AIModel operator detects the 32Gi spec and updates InferenceService accordingly

4. **KServe Creates Revisions**: KServe creates new Knative Revisions with 32Gi memory limit

5. **Scheduling Deadlock**:
   - New pods request: 8 CPU + 32Gi memory + 1 GPU
   - Old pods occupy: 3 GPUs (one per model)
   - Available nodes: 2 GPUs free, but each has only 32GB total RAM
   - 32Gi memory request exceeds node capacity (needs ~20% headroom for system)
   - New pods remain Pending: "Insufficient memory"

6. **Traffic Stuck**: Traffic stays on old revision 00001 (only ready revision)

7. **GC Blocked**: Knative GC cannot delete old revisions because they're ACTIVE (serving traffic)

**Evidence**:

- ArgoCD shows `SharedResourceWarning` for all three AIModels
- `aimodels-development` application is OutOfSync
- AIModel CRs in cluster show 32Gi (stale value from ai-aas repo)
- ai-aas-config repo shows 24Gi (correct value, but overridden)
- New revisions created with 32Gi (not 24Gi as intended)
- Scheduler rejects 32Gi pods: node capacity is 32GB total, need headroom

**Why 24Gi would work**:

- With 24Gi limit, pods could schedule on available GPU nodes
- Old pods would be replaced one at a time (rolling update)
- Traffic would shift to new revisions
- Knative GC would delete old revisions after 5m inactive

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: `context/infra-ops-manager/agents.md`

**What was missing**:
1. **Anti-pattern**: Never create duplicate ArgoCD Applications managing the same resources
2. **Pattern**: Use one of these strategies:
   - Single source of truth (one repo, one path)
   - Helm with values files from different repos
   - Kustomize overlays with remote bases
3. **Warning**: ArgoCD SharedResourceWarning indicates a serious configuration error
4. **Rule**: When migrating from one repo to another, DELETE the old ArgoCD Application first

**Suggested fix**: Add anti-pattern section for "Duplicate ArgoCD Applications" showing:
- How to detect (SharedResourceWarning)
- Why it's dangerous (race conditions, config drift)
- How to fix (delete old app, ensure single source)

## Proposed Fix

**HIGH-LEVEL APPROACH** (not implementation code):

### Option 1: Remove Duplicate Application (RECOMMENDED)

Delete the `aimodels-development` ArgoCD Application and keep only `aimodels-config-development`:

1. Delete ArgoCD Application:
   ```
   kubectl delete application aimodels-development -n argocd
   ```

2. This leaves only `aimodels-config-development` managing the AIModels

3. Sync will apply the 24Gi limits from ai-aas-config

4. AIModel operator will update InferenceServices with 24Gi

5. KServe will create new revisions with 24Gi

6. New pods will schedule successfully (24Gi fits on nodes)

7. Traffic will shift to new revisions

8. Knative GC will delete old revisions after 5m inactive

### Option 2: Remove Old Files (ALTERNATIVE)

Delete the stale files in ai-aas repo:

1. Remove `ai-aas/infra/k8s/aimodels/development/*.yaml`

2. This makes `aimodels-development` application have no resources

3. Delete the `aimodels-development` Application

4. Keep only `aimodels-config-development`

### Option 3: Consolidate to Single Repo (LONG-TERM)

Pick ONE source of truth:
- Either: Move all AIModel manifests to ai-aas-config (environment-specific configs)
- Or: Move all AIModel manifests to ai-aas (monorepo approach)

**DO NOT** keep them in both repos.

**Affected files**:
- ArgoCD Application: `gitops/clusters/development/apps/aimodels-development.yaml` (delete)
- Stale configs: `infra/k8s/aimodels/development/*.yaml` (delete or update)

**Estimated complexity**: Low (delete ArgoCD app) to Medium (consolidate repos)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| **Lint** | Add ArgoCD Application validation: detect SharedResourceWarning in CI |
| **Test** | Pre-deployment check: kubectl get applications -A -o yaml \| grep SharedResourceWarning |
| **Context** | Add anti-pattern for duplicate ArgoCD Applications in infra-ops-manager context |
| **Monitoring** | Alert on ArgoCD SharedResourceWarning conditions |
| **Architecture** | Establish single source of truth policy for CRDs (either ai-aas or ai-aas-config, not both) |
| **Documentation** | Document repo separation: ai-aas (operators, services) vs ai-aas-config (environment configs) |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| TBD | bug | infra-ops-manager | Delete duplicate aimodels-development ArgoCD Application |
| TBD | task | infra-ops-manager | Add ArgoCD SharedResourceWarning detection to CI |
| TBD | task | context-maintainer | Add anti-pattern for duplicate ArgoCD apps |
| TBD | task | infra-ops-manager | Establish and document single-source-of-truth policy for CRDs |

---

## Summary

**This is NOT a KServe or Knative bug.** The operators are working correctly.

The root cause is a **GitOps configuration conflict** where two ArgoCD Applications manage the same AIModel resources with different configurations. The stale configuration (32Gi) wins the race condition, causing new pods to request more memory than nodes can provide, creating a scheduling deadlock. Old pods cannot be replaced because new pods cannot schedule, and Knative GC correctly refuses to delete active pods.

**Immediate fix**: Delete the `aimodels-development` ArgoCD Application to eliminate the conflict.

**Long-term fix**: Establish single source of truth for all CRDs - use ai-aas-config for environment-specific configs.
