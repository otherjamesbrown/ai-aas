# Validation Report: Model Readiness Probes

**Feature**: `018-model-readiness-probes`
**Date**: 2025-11-28
**Status**: Implementation Complete - Validation Ready

## Summary

This report documents the implementation and validation status of HTTP-based health probes for KServe InferenceService deployments. The probes prevent traffic routing to pods before vLLM models are fully loaded into GPU memory.

## Implementation Status

### Manifests Updated

| InferenceService | Startup Probe | Readiness Probe | Liveness Probe | minReplicas |
|:-----------------|:--------------|:----------------|:---------------|:------------|
| gpt-oss-20b | ✅ 90×10s (15 min) | ✅ 10s period | ✅ 30s period | ✅ 1 |
| mistral-7b-instruct | ✅ 36×10s (6 min) | ✅ 10s period | ✅ 30s period | ✅ 1 |
| llama-2-7b | ✅ 36×10s (6 min) | ✅ 10s period | ✅ 30s period | ✅ 1 |
| Template | ✅ Configurable | ✅ Configured | ✅ Configured | N/A |

### Documentation Created

| Document | Status | Path |
|:---------|:-------|:-----|
| Feature Spec | ✅ Complete | `specs/018-model-readiness-probes/spec.md` |
| Implementation Plan | ✅ Complete | `specs/018-model-readiness-probes/plan.md` |
| Quickstart Guide | ✅ Complete | `specs/018-model-readiness-probes/quickstart.md` |
| Probe Config Templates | ✅ Complete | `specs/018-model-readiness-probes/contracts/probe-config-templates.yaml` |
| Runbook | ✅ Complete | `docs/runbooks/enable-model-readiness-probes.md` |
| Best Practices Update | ✅ Complete | `docs/best-practices/vllm-deployment-best-practices.md` |
| Validation Report | ✅ Complete | This file |

---

## Validation Checklist

### Phase 1: Setup Verification

- [x] T-S018-P01-002: Reviewed gpt-oss-20b.yaml probe configuration
- [x] T-S018-P01-003: Reviewed mistral-7b-instruct.yaml probe configuration
- [x] T-S018-P01-004: Reviewed llama-2-7b.yaml probe configuration
- [x] T-S018-P01-005: Documented probe configuration differences per model size

### Phase 2: Environment Validation

Run these commands to verify deployment status:

```bash
# Set kubeconfig
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml

# T-S018-P02-006: Verify ArgoCD sync status
argocd app get kserve-models --server argocd.dev.ai-aas.local

# T-S018-P02-007-009: Check pod status for all models
kubectl get pods -n development -l app=vllm-inference

# T-S018-P02-010: Verify probe configuration visible in pods
kubectl describe pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b | grep -A 10 "Startup:"
```

| Task | Status | Notes |
|:-----|:-------|:------|
| T-S018-P02-006: ArgoCD synced | ⏳ Pending | Run validation command |
| T-S018-P02-007: gpt-oss-20b 2/2 Running | ⏳ Pending | Run validation command |
| T-S018-P02-008: mistral-7b-instruct 2/2 Running | ⏳ Pending | Run validation command |
| T-S018-P02-009: llama-2-7b 2/2 Running | ⏳ Pending | Run validation command |
| T-S018-P02-010: Probes visible in describe | ⏳ Pending | Run validation command |

---

## User Story Validation

### US1: Readiness Probes Gate Traffic (P1)

**Test**: Delete and recreate a pod, verify it doesn't become Ready until model is loaded.

```bash
# Delete pod to trigger recreation
kubectl delete pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b

# Watch status transition (should see 1/2 → 2/2)
kubectl get pods -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b -w

# Check events for probe failures during loading
kubectl describe pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b | grep -A 15 "Events:"

# Test inference immediately after 2/2 Ready
curl -X POST "https://api.172.232.58.222.nip.io/v1/chat/completions" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-oss-20b", "messages": [{"role": "user", "content": "Hello"}], "max_tokens": 10}'
```

| Acceptance Criteria | Status | Notes |
|:--------------------|:-------|:------|
| Pod shows 1/2 Running during model load | ⏳ Pending | |
| Pod transitions to 2/2 Running after /health returns 200 | ⏳ Pending | |
| First request after Ready succeeds without timeout | ⏳ Pending | |
| First request latency <5 seconds (NFR-002) | ⏳ Pending | |

### US2: Consistent Probe Configuration (P1)

**Test**: Verify all manifests have correct probe configurations.

| Model | Startup Threshold | Expected for Size | Status |
|:------|:------------------|:------------------|:-------|
| gpt-oss-20b (20B) | 90 | 90 (15 min) | ✅ Correct |
| mistral-7b-instruct (7B) | 36 | 36 (6 min) | ✅ Correct |
| llama-2-7b (7B) | 36 | 36 (6 min) | ✅ Correct |

| Acceptance Criteria | Status | Notes |
|:--------------------|:-------|:------|
| All manifests have startup/readiness/liveness probes | ✅ Verified | |
| Probe timeouts match model size | ✅ Verified | |
| All production models have minReplicas: 1 | ✅ Verified | |
| ArgoCD sync completed successfully | ⏳ Pending | Requires cluster check |

### US3: Liveness Probes Detect Failures (P2)

```bash
# Check liveness probe configuration
kubectl get pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b \
  -o jsonpath='{.items[0].spec.containers[0].livenessProbe}'

# Check restart count (should be 0 under normal operation)
kubectl get pods -n development -l app=vllm-inference \
  -o jsonpath='{range .items[*]}{.metadata.name}: restarts={.status.containerStatuses[0].restartCount}{"\n"}{end}'
```

| Acceptance Criteria | Status | Notes |
|:--------------------|:-------|:------|
| Liveness probe configured: 30s period, 3 failures | ✅ Verified in manifests | |
| Restart count = 0 under normal operation | ⏳ Pending | Requires cluster check |
| False positive rate <1% (NFR-003) | ⏳ Pending | Monitor over time |

### US4: Startup Probes for Large Models (P2)

```bash
# Verify gpt-oss-20b startup probe allows 15 minutes
kubectl get pod -n development -l serving.kserve.io/inferenceservice=gpt-oss-20b \
  -o jsonpath='{.items[0].spec.containers[0].startupProbe}'
```

| Acceptance Criteria | Status | Notes |
|:--------------------|:-------|:------|
| gpt-oss-20b has 15 min startup timeout | ✅ Verified (90×10s) | |
| Pod not killed during model loading | ⏳ Pending | Requires cold start test |
| Startup probe hands off to readiness/liveness after success | ⏳ Pending | |

---

## Success Criteria Validation

| Criteria | Target | Status | Notes |
|:---------|:-------|:-------|:------|
| SC-001: Zero `context deadline exceeded` during scaling | 0 errors | ⏳ Pending | Monitor after deployment |
| SC-002: Pods 2/2 only after model loaded | All pods | ⏳ Pending | Observe pod transitions |
| SC-003: No BACKEND_ERROR during scaling (7 days) | 0 errors | ⏳ Pending | Long-term monitoring |
| SC-004: Liveness restarts unhealthy pods <2 min | <120s | N/A | No failures to test |
| SC-005: Zero startup probe kills for legitimate loads | 0 kills | ⏳ Pending | Monitor restarts |
| SC-006: Rolling updates 100% success, zero downtime | 100% | ⏳ Pending | Test rolling update |
| SC-007: Documentation complete | All docs | ✅ Complete | See Documentation section |

---

## Monitoring Queries

### Check for Timeout Errors (Post-Deployment)

```bash
# Search api-router-service logs for errors
kubectl logs -n system -l app=api-router-service --tail=500 | grep -i "BACKEND_ERROR\|timeout\|deadline"

# Check for probe-related pod events
kubectl get events -n development --field-selector reason=Unhealthy --sort-by='.lastTimestamp'
```

### Check Pod Readiness Metrics

```promql
# Grafana/Prometheus query for pod readiness
kube_pod_status_ready{namespace="development", pod=~".*-predictor-.*"}
```

---

## Risk Assessment

| Risk | Mitigation | Status |
|:-----|:-----------|:-------|
| Probe timeout too short | Used spec-recommended values per model size | ✅ Mitigated |
| Liveness kills pod during load | Startup probe configured to disable liveness | ✅ Mitigated |
| Cold start delays | minReplicas: 1 configured on all production models | ✅ Mitigated |
| Probe overhead | HTTP GET to /health is lightweight (<10ms) | ✅ Acceptable |

---

## Conclusion

**Implementation Status**: ✅ Complete

All InferenceService manifests have been updated with:
- Startup probes with model-size-appropriate timeouts
- Readiness probes to gate traffic until models are loaded
- Liveness probes to detect and restart unhealthy pods
- minReplicas: 1 to prevent cold boot delays

**Validation Status**: ⏳ Cluster validation pending

Documentation is complete. Cluster validation tests require access to the development environment to verify:
1. Pods transition correctly from 1/2 to 2/2 Running
2. No timeout errors occur during autoscaling
3. First request after Ready succeeds within 5 seconds

---

## Next Steps

1. Run cluster validation commands in the development environment
2. Update this report with validation results
3. Monitor for 7 days to verify SC-003 (no BACKEND_ERROR during scaling)
4. Update spec.md status from "Validation In Progress" to "Complete"
