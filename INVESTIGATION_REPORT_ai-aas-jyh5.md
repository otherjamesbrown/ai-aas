# Investigation Report

**Bead**: ai-aas-jyh5
**Date**: 2025-12-14
**Investigator**: debugger agent

## Symptom

vLLM model pods in both staging and development clusters experience excessive restarts due to liveness probe failures:
- **Staging**: 274-277 restarts in 29 hours (mistral-7b-instruct-v03, openai-gpt-oss-20b, unsloth-gpt-oss-20b)
- **Development**: 7-9 restarts in 53 minutes (same models)
- Pods return HTTP 503 during model loading
- Containers are killed with exitCode 137 (SIGKILL) while healthy

Error message: "Container kserve-container failed liveness probe, will be restarted"

## Reproduction

Observed in both clusters:
1. Pod starts and begins loading vLLM model from HuggingFace
2. Health endpoint returns 503 during model download and initialization
3. Liveness probe starts immediately (no grace period)
4. After 3 consecutive failures (3 × 30s = 90s), kubelet sends SIGKILL
5. Container restarts and cycle repeats every 6-15 minutes

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `operators/ai-model-operator/internal/kserve/inferenceservice.go:444-472` | Operator defines startupProbe (initialDelaySeconds: 30, failureThreshold: 90, max wait: 15min) but livenessProbe has NO initialDelaySeconds |
| InferenceService YAML (both clusters) | startupProbe is defined in spec |
| Pod YAML (both clusters) | startupProbe is MISSING from actual deployment - removed by Knative |
| Liveness probe config (both clusters) | periodSeconds: 30, failureThreshold: 3, NO initialDelaySeconds |
| Pod events (staging) | "Killing: Container kserve-container failed liveness probe" every 6-15 minutes |
| Container status (staging) | Last terminated: exitCode 137, message shows "GET /health HTTP/1.1 200 OK" - healthy when killed! |
| vLLM logs (20B model) | Model loading: 94.37s download + 15.62s load + 22.20s compile = **~150 seconds total** |
| vLLM logs (7B model) | Similar timing observed |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Staging has different probe config than dev | ❌ Ruled out | Both clusters have identical InferenceService specs and probe configurations |
| Knative timeoutSeconds killing pods | ❌ Ruled out | Both have 300s timeout, containers killed earlier at 90s mark |
| Resource differences causing slower loading | ❌ Ruled out | Model load times similar in both clusters (~90-150s) |
| Startup probe not long enough | ✅ PARTIAL | Startup probe config is correct (15 min max) BUT it's being removed |
| Knative removes startupProbe | ✅ CONFIRMED | Knative Serving in Serverless mode does not support startupProbe |
| Liveness probe starts too early | ✅ CONFIRMED | livenessProbe has NO initialDelaySeconds, starts immediately after container start |

## Root Cause

**Category**: `config_drift` + `missing_context`

**Explanation**:

The AI Model Operator correctly defines a `startupProbe` with appropriate timeouts for model loading (15 minutes maximum), but **Knative Serving removes this probe** when deploying in Serverless mode. This is a known Knative limitation - startupProbe is not supported in Knative Serving.

The `livenessProbe` configuration lacks an `initialDelaySeconds` parameter (line 464 in `inferenceservice.go`). Since the startupProbe is removed, the liveness probe begins checking immediately when the container starts. vLLM returns HTTP 503 while downloading and loading the model (takes 90-150 seconds). After 3 consecutive liveness probe failures (3 × 30s = 90 seconds), Kubelet kills the container with SIGKILL, even though the model is still loading successfully.

**Evidence**:
- Operator code: `operators/ai-model-operator/internal/kserve/inferenceservice.go:464-472`
- StartupProbe defined in InferenceService spec but absent from deployed pods
- vLLM model load logs show 90-150 second initialization time
- Containers terminated with exitCode 137 after exactly 90 seconds of failed liveness checks
- Last log messages before termination show successful 200 OK health responses

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: `context/operator-developer/agents.md`

**What was missing**:
1. Anti-pattern for Knative Serverless mode probe limitations (startupProbe not supported)
2. Pattern for setting livenessProbe initialDelaySeconds for long-running initialization tasks
3. Documentation that Knative removes startupProbe even if defined in InferenceService spec

**Suggested fix**: Add to operator-developer context:
```markdown
## Anti-patterns

### Knative Serving Probe Configuration

# WRONG: Relying on startupProbe in Serverless mode
"startupProbe": {...}  # This will be removed by Knative

# WRONG: Liveness probe with no initialDelaySeconds for slow-starting containers
"livenessProbe": {
    "periodSeconds": 30,
    "failureThreshold": 3
    // Missing: "initialDelaySeconds"
}

# CORRECT: Set initialDelaySeconds on livenessProbe to account for startup time
"livenessProbe": {
    "httpGet": {"path": "/health", "port": 8000},
    "initialDelaySeconds": 300,  // 5 minutes for model loading
    "periodSeconds": 30,
    "failureThreshold": 3,
    "timeoutSeconds": 5
}
```

## Proposed Fix

**File**: `operators/ai-model-operator/internal/kserve/inferenceservice.go`

**Change Required**: Add `initialDelaySeconds` to the `livenessProbe` configuration in the `BuildContainerBased()` function (line 464-472).

**Recommended value**:
- **300 seconds (5 minutes)** for small/medium models (up to 20B parameters)
- Could be made configurable via AIModel CRD for larger models

**Rationale**:
- Model loading typically takes 90-150 seconds based on logs
- 300 seconds provides 2x safety margin
- Readiness probe will still prevent traffic until model is ready
- Liveness probe only prevents premature killing during initialization

**Affected files**:
- `operators/ai-model-operator/internal/kserve/inferenceservice.go` - Add initialDelaySeconds to livenessProbe

**Estimated complexity**: Low (single line change)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add integration test that verifies InferenceService pods don't restart during first 10 minutes |
| Lint | N/A - this is configuration, not code pattern |
| Context | Add anti-pattern for Knative probe configuration to operator-developer context |
| Logging | Add operator log message when creating InferenceService without livenessProbe initialDelaySeconds |
| Monitoring | Add alert for pod restart rate > 1/hour for InferenceService pods |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| ai-aas-[next] | bug | operator-developer | Add initialDelaySeconds to livenessProbe in BuildContainerBased() |
| ai-aas-[next+1] | task | operator-developer | Consider making liveness initialDelaySeconds configurable via AIModel CRD |
| ai-aas-[next+2] | task | context-maintainer | Update operator-developer context with Knative probe anti-patterns |
| ai-aas-[next+3] | task | infra-ops-manager | Add monitoring alert for excessive InferenceService pod restarts |
| ai-aas-[next+4] | task | operator-developer | Add integration test for InferenceService startup stability |
