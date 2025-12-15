# Investigation Report: 502 Backend Errors and Model Pod Restarts in Staging

**Bead**: ai-aas-9s4z
**Date**: 2025-12-14
**Investigator**: debugger agent
**Environment**: staging

---

## Symptom

- guidellm-runner benchmarks failing with 502 backend errors
- Model pods restarting frequently (13-26 restarts each)
- All InferenceServices showing READY=False
- 500 ROUTING_ERROR: "no routing policy configured" for some requests

---

## Reproduction

**Pod Status** (as of investigation):
```
mistral-7b-instruct-v03-predictor-00001: 2/2 Running, 24 restarts
openai-gpt-oss-20b-predictor-00001: 2/2 Running, 26 restarts
unsloth-gpt-oss-20b-predictor-00001: 1/2 Running, 22 restarts
```

**Error Messages**:
- `502 BACKEND_ERROR: backend request failed: Post http://mistral-7b-instruct-v03-predictor-00001-private.staging.svc.cluster.local:8012/v1/completions: dial tcp: connect: connection refused`
- `500 ROUTING_ERROR: no routing policy configured`

---

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `kubectl get events` | "Container kserve-container failed liveness probe, will be restarted" |
| vLLM container logs | Model startup takes ~90 seconds (download: 54s, init: 30s, CUDA graphs: 7s) |
| Pod probe config | livenessProbe: NO initialDelaySeconds, periodSeconds=30, failureThreshold=3 |
| Pod probe config | startupProbe: NOT PRESENT (null) |
| Pod probe config | readinessProbe: NOT PRESENT (null) |
| Operator code | BuildContainerBased() sets livenessProbe.initialDelaySeconds=300 (5 min) |
| Operator code | BuildContainerBased() sets startupProbe with failureThreshold=90 |
| Pod last termination | exitCode=137 (SIGKILL), finishedAt=2025-12-14T20:03:03Z |
| api-router logs | 502 errors starting at 20:04:28Z (85s after pod killed) |
| api-router logs | "no routing policy found" for org_id=b6fc81af-a245-4599-b3e1-7d2b8745c148 |

---

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| OOM kills causing restarts | ❌ Ruled out | No OOM events; memory limit 32Gi, actual usage ~13.5 GiB |
| GPU memory errors | ❌ Ruled out | No CUDA OOM in logs; KV cache fits in available GPU memory |
| Liveness probe timeout too aggressive | ✅ CONFIRMED | Probe starts immediately, model takes 90s to load, killed after 90s (3 failures) |
| KServe overriding operator probe config | ✅ CONFIRMED | Operator sets initialDelaySeconds=300, deployed pod has none |
| Missing routing policies in database | ✅ CONFIRMED | api-router queries database for (org_id, model), returns 500 when not found |
| Network connectivity issues | ❌ Ruled out | Service endpoints exist, IPs match pod IPs, port 8012 exposed |

---

## Root Cause

**Category**: `config_drift` + `missing_context`

### Primary Root Cause: KServe Overriding Health Probe Configuration

**What happened:**
1. The ai-model-operator creates InferenceServices with safe health probe configuration:
   - `startupProbe`: initialDelaySeconds=30, periodSeconds=10, failureThreshold=90 (max 15 min)
   - `livenessProbe`: initialDelaySeconds=300 (5 min), periodSeconds=30, failureThreshold=3
   - `readinessProbe`: no initialDelay, periodSeconds=10, failureThreshold=3

2. KServe's admission webhook or queue-proxy injection **overrides these probes** with defaults:
   - `startupProbe`: REMOVED (not present)
   - `livenessProbe`: NO initialDelaySeconds, periodSeconds=30, failureThreshold=3
   - `readinessProbe`: REMOVED (not present)

3. With no initialDelaySeconds on the liveness probe:
   - First probe attempt: t=0s (pod just started) → FAIL (vLLM not ready)
   - Second probe: t=30s → FAIL (vLLM still loading model)
   - Third probe: t=60s → FAIL (vLLM still loading model)
   - After 3 failures (90s), Kubernetes sends SIGKILL (exit code 137)

4. vLLM model startup time in staging:
   - Model weight download: ~54 seconds
   - Engine initialization: ~30 seconds
   - CUDA graph capture: ~7 seconds
   - **Total: ~90 seconds**

5. The probe kills the pod at exactly 90 seconds, which is when vLLM would become ready.

6. Pod restarts → vLLM starts loading again → killed at 90s → infinite restart loop.

**Evidence:**
- Operator code: `/home/dev/ai-aas/operators/ai-model-operator/internal/kserve/inferenceservice.go` lines 465-494
- Deployed pod: `kubectl get pod -o jsonpath` shows NO initialDelaySeconds
- Pod events: "Container kserve-container failed liveness probe, will be restarted"
- Last termination: exitCode=137 (SIGKILL)

### Contributing Factor #1: vLLM Startup Time vs Probe Timing

vLLM requires downloading model weights on first startup (if not cached), which takes significant time:
- 7B model: ~50-60 seconds
- 20B model: potentially longer

The probe configuration must account for this, but KServe's defaults do not.

### Contributing Factor #2: Missing Routing Policies (Separate Issue)

The 500 ROUTING_ERROR is a separate configuration issue:
- api-router queries database for routing policies by (org_id, model)
- Staging database does not have policies configured for org `b6fc81af-a245-4599-b3e1-7d2b8745c148`
- This causes 500 errors BEFORE attempting backend connection
- This is a data/configuration issue, not a deployment issue

---

## Context Gap Check

✅ **YES - This was caused by missing context**

**Context file**: `context/operator-developer/agents.md`

**What was missing**:
- No documented pattern showing that KServe overrides health probe configuration
- No anti-pattern warning that probe configuration in operator may be ignored
- No guidance on how to set KServe-compatible probe configuration
- No pattern for using PodSpec annotations to prevent probe override

**Suggested fix**: Add to `context/operator-developer/agents.md`:
```yaml
kserve_patterns:
  health_probes:
    problem: "KServe admission webhook overrides container health probes"
    solution: "Use KServe-specific annotations instead of pod spec"
    example: |
      annotations:
        serving.knative.dev/startup-probe: '{"periodSeconds": 10, "failureThreshold": 90}'
        serving.knative.dev/liveness-probe: '{"initialDelaySeconds": 300}'
```

---

## Proposed Fix

### Fix #1: Configure KServe-Compatible Health Probes (HIGH PRIORITY)

**Affected files**:
- `operators/ai-model-operator/internal/kserve/inferenceservice.go` (BuildContainerBased method)

**Required changes**:
1. Research KServe v0.13+ health probe configuration method
2. Determine if probes should be set via:
   - Knative Service annotations (serving.knative.dev/*)
   - KServe-specific annotations (serving.kserve.io/*)
   - PodSpec with specific labels to prevent override
3. Update BuildContainerBased() to use correct method
4. Ensure startupProbe OR livenessProbe.initialDelaySeconds allows 15+ min for model load

**Validation**:
- Deploy to development environment
- Verify deployed pod has correct probe configuration
- Verify vLLM completes startup without restarts
- Test with mistral-7b (fast load) and gpt-oss-20b (slower load)

**Estimated complexity**: Medium

---

### Fix #2: Configure Routing Policies in Staging Database (MEDIUM PRIORITY)

**Affected files**:
- Staging database (manual data configuration)

**Required changes**:
1. Use ai-aas-cli or Admin API to create routing policies
2. For org `b6fc81af-a245-4599-b3e1-7d2b8745c148`:
   - Add policy for model `openai/gpt-oss-20b` → backend `openai/gpt-oss-20b`
   - Add policy for model `unsloth/gpt-oss-20b` → backend `unsloth/gpt-oss-20b`
   - Add policy for model `mistral-7b-instruct` → backend `mistral-7b-instruct-v03`
3. Verify policies via API

**Validation**:
- Send completion request to staging api-router
- Verify 200 response (not 500 ROUTING_ERROR)

**Estimated complexity**: Low (configuration task, not code change)

---

## Prevention

How to prevent this class of bug in the future:

| Type | Action |
|------|--------|
| Test | Add integration test: Deploy InferenceService, verify probe config in running pod |
| Test | Add test: vLLM startup must complete within probe failure threshold |
| Lint | N/A |
| Context | Document KServe probe override behavior and correct configuration method |
| Monitoring | Alert on pod restart count > 3 in 30 minutes |
| CI/CD | Add validation step: Compare operator-generated manifest vs deployed pod spec |

---

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| TBD | bug | operator-developer | Fix KServe health probe configuration in ai-model-operator |
| TBD | task | go-services-developer | Configure routing policies in staging database |
| TBD | task | context-maintainer | Add KServe probe anti-pattern to operator-developer context |
| TBD | task | infra-ops-manager | Add pod restart monitoring/alerting for staging |

---

## Summary

**Primary Issue**: KServe admission webhook overrides the health probe configuration set by the ai-model-operator, removing initialDelaySeconds from livenessProbe and removing startupProbe entirely. This causes pods to be killed at 90 seconds (3 x 30s probe failures) when vLLM needs ~90 seconds to start, creating an infinite restart loop.

**Secondary Issue**: Staging database lacks routing policy configuration for the test organization, causing 500 errors before backend requests are even attempted.

**Immediate Impact**:
- All model pods in staging are unstable and restarting frequently
- guidellm benchmarks cannot complete
- InferenceServices report READY=False

**Fix Priority**: HIGH - This blocks all staging testing and validation.
