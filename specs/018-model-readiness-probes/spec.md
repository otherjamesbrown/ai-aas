# Feature Specification: Model Readiness Probes for KServe InferenceServices

**Feature Branch**: `018-model-readiness-probes`
**Created**: 2025-11-26
**Status**: Draft
**Input**: Implement proper readiness probes for KServe InferenceService deployments to prevent traffic routing to pods before model loading is complete, eliminating timeout errors during autoscaling and cold starts.

## Problem Statement

### Current Issue

When KServe autoscales or creates new replicas of InferenceServices:
1. Knative creates new pods to handle increased load
2. Pods are marked as "Ready" before the vLLM model finishes loading into GPU memory
3. Traffic is routed to these not-yet-ready pods via Knative activator
4. Requests fail with timeout errors: `context deadline exceeded`
5. Users receive `BACKEND_ERROR` responses until the model fully loads (2-5 minutes for large models)

### Root Cause

The current InferenceService configurations lack proper readiness probes. Kubernetes marks pods as ready based solely on the container starting, not on the application's actual readiness to serve traffic. For vLLM models:
- Container startup: ~10 seconds
- **Model loading to GPU: 2-5 minutes (7B models) to 5-15 minutes (20B+ models)**
- Without readiness probes, the pod receives traffic during the model loading phase

### Impact

- **User Experience**: Timeout errors during scaling events
- **System Reliability**: Failed requests cause retry storms and increased load
- **Resource Utilization**: Pods consuming GPU resources but unable to serve traffic
- **Monitoring**: False positive "ready" signals mask actual service unavailability

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configure Readiness Probes for vLLM Models (Priority: P1)

As a platform engineer, I can configure HTTP-based readiness probes that check vLLM's `/health` endpoint to ensure pods only receive traffic after models are fully loaded into GPU memory.

**Why this priority**: Directly addresses the timeout issue and prevents traffic to unready pods.

**Independent Test**: Deploy an InferenceService with readiness probe, verify pod doesn't become Ready until vLLM reports model loaded, send test request immediately after pod Ready status, confirm successful response.

**Acceptance Scenarios**:

1. **Given** a new InferenceService deployment, **When** the pod starts, **Then** Kubernetes does NOT mark it as Ready until the `/health` endpoint returns HTTP 200.
2. **Given** a pod loading a 20B model, **When** vLLM is still loading the model, **Then** the readiness probe fails and the pod status remains `1/2 Running` (queue-proxy ready, kserve-container not ready).
3. **Given** a pod that has completed model loading, **When** vLLM's `/health` endpoint returns 200, **Then** the readiness probe succeeds and Kubernetes marks the pod as `2/2 Running`.
4. **Given** a Ready pod, **When** a test request is sent, **Then** the request succeeds within SLA without timeout errors.

---

### User Story 2 - Apply Readiness Probes to All InferenceServices (Priority: P1)

As a platform engineer, I can apply standardized readiness probe configurations to all existing InferenceService manifests (gpt-oss-20b, mistral-7b-instruct, llama-2-7b, etc.) via GitOps.

**Why this priority**: Ensures consistent behavior across all models and environments.

**Independent Test**: Update each InferenceService YAML, apply via GitOps, verify rolling update completes with readiness probes active, test autoscaling behavior.

**Acceptance Scenarios**:

1. **Given** updated InferenceService manifests with readiness probes, **When** applied via GitOps, **Then** Knative performs rolling updates without downtime.
2. **Given** readiness probes configured on all models, **When** autoscaling creates new replicas, **Then** new pods do NOT receive traffic until models are loaded.
3. **Given** a scaling event, **When** monitoring pod status, **Then** no `BACKEND_ERROR` timeout errors occur in api-router-service logs.

---

### User Story 3 - Configure Liveness Probes for Pod Recovery (Priority: P2)

As a platform engineer, I can configure liveness probes that restart pods if vLLM becomes unresponsive, ensuring automatic recovery from GPU memory errors or process hangs.

**Why this priority**: Improves system resilience by auto-recovering from failure states.

**Independent Test**: Simulate vLLM hang/crash in a test pod, verify liveness probe triggers pod restart, confirm pod recovers to Ready state.

**Acceptance Scenarios**:

1. **Given** a pod with liveness probe configured, **When** vLLM process hangs and stops responding to `/health`, **Then** Kubernetes restarts the pod after `failureThreshold * periodSeconds`.
2. **Given** a pod restarted by liveness probe, **When** the pod restarts, **Then** readiness probe prevents traffic until model is reloaded.

---

### User Story 4 - Configure Startup Probes for Large Models (Priority: P2)

As a platform engineer, I can configure startup probes with extended timeouts for large models (20B+) to prevent premature liveness probe failures during long initialization periods.

**Why this priority**: Prevents liveness probes from killing pods during legitimate long model loading times.

**Independent Test**: Deploy a 20B model with startup probe, verify pod completes loading without liveness probe interference, confirm startup probe hands off to readiness/liveness probes.

**Acceptance Scenarios**:

1. **Given** a 20B model with 15-minute load time, **When** startup probe is configured with appropriate timeout, **Then** the pod is not killed during model loading.
2. **Given** startup probe success, **When** model loading completes, **Then** readiness and liveness probes take over for ongoing health checks.

---

### User Story 5 - Document Readiness Probe Configuration Best Practices (Priority: P3)

As a developer, I can reference documentation that explains how to configure readiness, liveness, and startup probes for different model sizes and use cases.

**Why this priority**: Enables self-service configuration for new models.

**Independent Test**: Follow documentation to configure probes for a new model, verify correct behavior.

**Acceptance Scenarios**:

1. **Given** the documentation, **When** deploying a new 7B model, **Then** I can determine appropriate `initialDelaySeconds`, `periodSeconds`, and `failureThreshold` values.
2. **Given** probe configuration examples, **When** deploying a 70B model, **Then** I can adjust startup probe timeout accordingly.

---

### Edge Cases

- **Model loading slower than expected**: Increase `failureThreshold` or `periodSeconds` to accommodate variance
- **Network latency to `/health` endpoint**: Configure `timeoutSeconds` to account for internal network delays
- **Startup probe timeout for very large models**: Set `startupProbe.failureThreshold` × `startupProbe.periodSeconds` > maximum expected load time
- **Liveness probe false positives during heavy load**: Increase `failureThreshold` to tolerate temporary slowness
- **Cold start with scale-from-zero**: First replica experiences longer load time; startup probe must accommodate this
- **Rolling updates during high traffic**: Ensure `minReplicas` and probe settings prevent traffic disruption during updates
- **GPU memory errors during loading**: Liveness probe should catch and restart, but may require manual intervention for persistent issues
- **Concurrent replica startups**: Multiple pods loading simultaneously may exhaust GPU resources; resource quotas needed

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide readiness probe configuration for vLLM InferenceService containers checking `/health` endpoint on the container's application port (8000 for newer vLLM versions, 8080 for older versions).
- **FR-002**: Provide liveness probe configuration for vLLM InferenceService containers to detect and restart unhealthy pods.
- **FR-003**: Provide startup probe configuration for large models (20B+) with extended timeouts to accommodate long loading times.
- **FR-004**: Update all existing InferenceService manifests (gpt-oss-20b, mistral-7b-instruct, llama-2-7b) with probe configurations.
- **FR-005**: Provide probe configuration guidelines based on model size (7B, 13B, 20B, 70B+).
- **FR-006**: Ensure probe configurations are compatible with Knative Serving and queue-proxy sidecar.
- **FR-007**: Document probe configuration in runbooks and InferenceService templates.

### Non-Functional Requirements

- **NFR-001**: Readiness probe must detect model readiness within 30 seconds of vLLM becoming ready to serve.
- **NFR-002**: Liveness probe must not falsely trigger during normal heavy load (<1% false positive rate).
- **NFR-003**: Startup probe timeout must accommodate 95th percentile model loading times without premature pod kills.
- **NFR-004**: Probe HTTP requests must add <100ms overhead to model serving.
- **NFR-005**: Rolling updates with probes enabled must complete without service degradation or dropped requests.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Zero `context deadline exceeded` errors during autoscaling events after readiness probes deployed.
- **SC-002**: All InferenceService pods reach `2/2 Running` status only after models are fully loaded and ready.
- **SC-003**: No `BACKEND_ERROR` responses from api-router-service during scaling events (measured over 7 days post-deployment).
- **SC-004**: Liveness probe successfully restarts unhealthy pods within 2 minutes of vLLM failure detection.
- **SC-005**: Startup probe allows large models (20B+) to complete loading without premature pod termination (0 startup probe failures for legitimate loads).
- **SC-006**: Rolling updates complete with 100% success rate and zero downtime.
- **SC-007**: Documentation covers probe configuration for all supported model sizes with worked examples.

## Architecture Overview

### Current State (No Probes)

```
┌─────────────────────────────────────────────────────────────┐
│                  Current Behavior (Broken)                   │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Pod Lifecycle:                                               │
│                                                               │
│  1. Container Starts                                          │
│     └─▶ Kubernetes marks pod Ready ✓ (PREMATURE!)            │
│                                                               │
│  2. vLLM begins loading model                                 │
│     ├─▶ Downloading model weights... (2-10 min)              │
│     ├─▶ Loading to GPU memory... (1-5 min)                   │
│     └─▶ Pod is Ready but vLLM NOT ready ✗                    │
│                                                               │
│  3. Knative routes traffic                                    │
│     └─▶ Requests sent to unready pod ✗                       │
│          └─▶ Timeout: "context deadline exceeded" ✗          │
│               └─▶ BACKEND_ERROR to client ✗                  │
│                                                               │
│  4. Model finally loaded (5-15 min after start)               │
│     └─▶ vLLM ready to serve ✓                                │
│          └─▶ But already received failed requests ✗          │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Target State (With Probes)

```
┌─────────────────────────────────────────────────────────────┐
│                  Target Behavior (Fixed)                     │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  Pod Lifecycle:                                               │
│                                                               │
│  1. Container Starts                                          │
│     ├─▶ Startup Probe begins checking /health                │
│     │    - initialDelaySeconds: 30s                           │
│     │    - periodSeconds: 10s                                 │
│     │    - failureThreshold: 60 (up to 10 min)               │
│     └─▶ Pod status: 1/2 Running (NOT Ready) ✓                │
│                                                               │
│  2. vLLM loading model                                        │
│     ├─▶ Downloading model weights... (2-10 min)              │
│     │    └─▶ /health returns 503 or connection refused       │
│     │         └─▶ Startup probe fails (expected) ✓           │
│     ├─▶ Loading to GPU memory... (1-5 min)                   │
│     │    └─▶ /health returns 503 (model not loaded)          │
│     │         └─▶ Startup probe continues failing ✓          │
│     └─▶ Knative does NOT route traffic yet ✓                 │
│                                                               │
│  3. Model loading complete                                    │
│     ├─▶ vLLM /health endpoint returns 200 OK ✓               │
│     ├─▶ Startup probe succeeds ✓                             │
│     ├─▶ Readiness probe takes over:                          │
│     │    - initialDelaySeconds: 0s (startup probe done)      │
│     │    - periodSeconds: 10s                                 │
│     │    - failureThreshold: 3                                │
│     └─▶ Pod marked Ready: 2/2 Running ✓                      │
│                                                               │
│  4. Traffic routing begins                                    │
│     ├─▶ Knative routes traffic to Ready pod ✓                │
│     ├─▶ First request succeeds immediately ✓                 │
│     └─▶ No timeout errors ✓                                  │
│                                                               │
│  5. Ongoing health monitoring                                 │
│     ├─▶ Readiness probe: Continues checking every 10s        │
│     │    - If fails: Remove from service endpoints           │
│     └─▶ Liveness probe: Detects crashes/hangs                │
│          - periodSeconds: 30s                                 │
│          - failureThreshold: 3                                │
│          - If fails 3 times: Restart pod                      │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

### Probe Configuration Matrix

| Model Size | Load Time | Startup Probe Timeout | Readiness Period | Liveness Period |
|:-----------|:----------|:---------------------|:----------------|:----------------|
| **7B** (e.g., mistral-7b) | 2-5 min | 6 min (30s init + 36×10s) | 10s | 30s |
| **13B** (e.g., llama-2-13b) | 5-8 min | 10 min (30s init + 60×10s) | 10s | 30s |
| **20B** (e.g., gpt-oss-20b) | 5-15 min | 15 min (30s init + 90×10s) | 10s | 30s |
| **70B+** (e.g., llama-2-70b) | 15-30 min | 30 min (60s init + 180×10s) | 15s | 60s |

### vLLM Health Endpoint Behavior

vLLM's `/health` endpoint returns:
- **200 OK**: Model loaded and ready to serve
- **503 Service Unavailable**: Model still loading or not initialized
- **Connection Refused**: Container not yet listening on port 8000

Probe Logic:
- Startup probe: Tolerates failures until model loads
- Readiness probe: Removes pod from endpoints on failure, re-adds on success
- Liveness probe: Restarts pod only after repeated failures (3×30s = 90s default)

## Data Model

No database schema changes required. Configuration is declarative in InferenceService YAML manifests.

### InferenceService YAML Structure (Before)

```yaml
spec:
  predictor:
    containers:
    - name: kserve-container
      image: vllm/vllm-openai:v0.10.2
      args: [...]
      resources: {...}
      # MISSING: probes
```

### InferenceService YAML Structure (After)

```yaml
spec:
  predictor:
    containers:
    - name: kserve-container
      image: vllm/vllm-openai:v0.10.2
      args: [...]
      resources: {...}
      startupProbe:
        httpGet:
          path: /health
          port: 8000
        initialDelaySeconds: 30
        periodSeconds: 10
        failureThreshold: 90  # 30s + 90×10s = 15 min max
        timeoutSeconds: 5
      readinessProbe:
        httpGet:
          path: /health
          port: 8000
        periodSeconds: 10
        failureThreshold: 3
        timeoutSeconds: 5
      livenessProbe:
        httpGet:
          path: /health
          port: 8000
        periodSeconds: 30
        failureThreshold: 3
        timeoutSeconds: 5
```

## API Contracts

No external API changes. Health endpoint is provided by vLLM:

### vLLM /health Endpoint

```
GET http://localhost:8000/health
```

**Response (Ready)**:
```
HTTP/1.1 200 OK
```

**Response (Not Ready)**:
```
HTTP/1.1 503 Service Unavailable
```

## Implementation Tasks

### Phase 1: Create Readiness Probe Configuration Template
- [ ] Define probe parameters for each model size category
- [ ] Create InferenceService YAML template with all three probe types
- [ ] Document configuration decision rationale

### Phase 2: Update Existing InferenceServices
- [ ] Update gpt-oss-20b InferenceService manifest
- [ ] Update mistral-7b-instruct InferenceService manifest
- [ ] Update llama-2-7b InferenceService manifest (if deployed)
- [ ] Update any other deployed models

### Phase 3: GitOps Deployment
- [ ] Commit updated manifests to feature branch
- [ ] Create pull request to development
- [ ] Review and validate configuration
- [ ] Merge to development branch
- [ ] Monitor ArgoCD sync and rolling updates

### Phase 4: Validation Testing
- [ ] Trigger autoscaling event, verify new pods don't receive traffic until ready
- [ ] Monitor for timeout errors during scaling
- [ ] Test liveness probe by simulating vLLM hang
- [ ] Validate startup probe allows large models to load without termination

### Phase 5: Documentation
- [ ] Document probe configuration in runbooks
- [ ] Add probe configuration guide to InferenceService deployment docs
- [ ] Update troubleshooting guide with probe-related debugging steps

## Security Considerations

- **Health Endpoint Exposure**: The `/health` endpoint is exposed only within the cluster (ClusterIP service); no external access risk.
- **Probe Permissions**: Probes run from kubelet with no special RBAC requirements.
- **Information Disclosure**: `/health` endpoint reveals only readiness state, no sensitive model or user data.
- **DoS via Probes**: Probe frequency is bounded (every 10-30s); negligible overhead on vLLM.

## Observability

### Metrics

**Kubernetes Pod Metrics** (existing, now more accurate):
- `kube_pod_status_ready{pod=~".*-predictor-.*"}` - Pod readiness status (now reflects actual vLLM readiness)
- `kube_pod_container_status_ready{container="kserve-container"}` - Container readiness (improved accuracy)

**Probe Success/Failure Metrics** (via kube-state-metrics):
- `kube_pod_container_status_restarts_total{container="kserve-container"}` - Liveness probe-triggered restarts
- Probe failures visible in pod events (`kubectl describe pod`)

### Logging

**Probe Failure Events**:
```
Events:
  Type     Reason     Age   Message
  ----     ------     ----  -------
  Warning  Unhealthy  2m    Startup probe failed: HTTP probe failed with statuscode: 503
  Warning  Unhealthy  1m    Readiness probe failed: Get "http://10.2.1.5:8000/health": context deadline exceeded
  Normal   Started    30s   Started container kserve-container
```

**vLLM Logs** (correlated with probe events):
```
INFO 11-26 03:34:47 Starting to load model openai/gpt-oss-20b...
INFO 11-26 03:37:22 Model loaded successfully, server ready
```

### Dashboards

**Grafana Panel: Pod Readiness Lag**
- Metric: Time between container start and pod ready status
- Target: <15 minutes for 20B models
- Alert: >20 minutes indicates probe misconfiguration or loading issues

**Grafana Panel: Probe Failure Rate**
- Metric: Probe failures per pod
- Target: <5% false positive rate for liveness probes
- Alert: >10% suggests need for probe tuning

## Testing Strategy

### Integration Tests

- **Test 1: New Pod Readiness**: Deploy InferenceService, verify pod transitions from Not Ready → Ready only after vLLM `/health` returns 200.
- **Test 2: Traffic Routing**: Send request immediately after pod Ready, verify no timeout errors.
- **Test 3: Autoscaling**: Simulate load to trigger scale-up, verify new replicas don't receive traffic until ready.
- **Test 4: Liveness Probe Restart**: Kill vLLM process inside pod, verify liveness probe triggers restart.
- **Test 5: Startup Probe Timeout**: Deploy 20B model, verify startup probe doesn't kill pod during legitimate 10+ minute load.

### Performance Tests

- **Test 6: Probe Overhead**: Measure CPU/memory overhead of probes running every 10s/30s.
- **Test 7: Rolling Update**: Perform rolling update of InferenceService, measure downtime (target: 0s).

## Migration Runbook

See `docs/runbooks/enable-model-readiness-probes.md` for:
- Pre-deployment validation steps
- Probe configuration per model
- GitOps deployment procedure
- Rollback steps if issues arise

## Risk Mitigation

| Risk | Impact | Probability | Mitigation |
|:-----|:-------|:-----------|:-----------|
| Probe timeout too short for large models | High | Medium | Use startup probe with model-size-specific timeouts |
| Liveness probe kills pod during heavy load | Medium | Low | Set failureThreshold=3, periodSeconds=30s for tolerance |
| Probe configuration error breaks deployments | High | Low | Test in development first; use phased rollout |
| Startup probe too long delays recovery | Low | Medium | Balance timeout vs model size; document tuning guidelines |
| Probes add excessive overhead | Low | Low | HTTP GET to /health is lightweight (<10ms); monitor metrics |

## Open Questions

- **Q1**: Should we use TCP probes instead of HTTP probes for lower overhead?
  - **Recommendation**: Use HTTP probes against `/health` for semantic accuracy; overhead is negligible.

- **Q2**: What is the appropriate `failureThreshold` for startup probes on 70B+ models?
  - **Recommendation**: Start with 180 (30 minutes); adjust based on observed load times.

- **Q3**: Should we implement a custom health endpoint that checks GPU memory status?
  - **Recommendation**: vLLM's `/health` already validates model readiness; custom endpoint adds complexity without benefit.

## Future Enhancements

- **Advanced Health Checks**: Implement `/readyz` and `/livez` endpoints with more granular health signals
- **Probe Metrics Exporter**: Export probe success/failure rates to Prometheus for alerting
- **Dynamic Probe Configuration**: Adjust probe parameters based on model metadata (size, quantization, etc.)
- **Graceful Shutdown Hooks**: Implement `preStop` hooks to drain in-flight requests before pod termination

## References

- [Kubernetes Pod Lifecycle](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/)
- [Kubernetes Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [vLLM Health Endpoint](https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html)
- [Knative Readiness Probes](https://knative.dev/docs/serving/configuration/feature-flags/#kubernetes-deployment-with-probes)
