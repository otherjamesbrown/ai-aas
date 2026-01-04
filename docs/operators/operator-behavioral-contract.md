---
title: Operator Behavioral Contract
last_updated: 2025-12-26
owner: operator-developer
---

# Operator Behavioral Contract

This document defines the behavioral contract for Kubernetes operators in the AI-AAS platform. It specifies how operators MUST behave in terms of phase transitions, timeouts, error handling, and health monitoring.

## Scope

This contract applies to:
- **AI Model Operator** (`aimodel.ai-aas.io/v1alpha1`)
- All future operators in the platform

## Phase Transition State Machine

### AIModel Phases

```
                                    ┌──────────────────────────────────┐
                                    │                                  │
                                    ▼                                  │
┌─────────┐    ┌─────────────┐    ┌────────────┐    ┌──────────┐    ┌─────┐
│ Pending │───▶│ Downloading │───▶│ Downloaded │───▶│ Deploying│───▶│Ready│
└─────────┘    └─────────────┘    └────────────┘    └──────────┘    └─────┘
     │              │                   │                │              │
     │              │                   │                │              │
     │              ▼                   │                ▼              │
     │         ┌─────────────┐          │           ┌────────┐          │
     │         │RetryPending │──────────┘           │ Failed │◀─────────┘
     │         └─────────────┘                      └────────┘
     │              │                                    ▲
     │              │ (max retries exceeded)             │
     │              └────────────────────────────────────┘
     │
     ▼
┌──────────┐
│ Disabled │
└──────────┘
```

### Valid Phase Transitions

| From | To | Trigger |
|------|-----|---------|
| `Pending` | `Downloading` | S3 artifacts not found, downloader job created |
| `Pending` | `Downloaded` | S3 artifacts already exist (skip download) |
| `Pending` | `Disabled` | `spec.enabled: false` |
| `Downloading` | `Downloaded` | Downloader job completed successfully |
| `Downloading` | `RetryPending` | Downloader job failed (retries remaining) |
| `Downloading` | `Failed` | Downloader job failed (no retries remaining) |
| `Downloaded` | `Deploying` | InferenceService created |
| `RetryPending` | `Downloading` | Retry delay elapsed, new job created |
| `RetryPending` | `Failed` | Max retries exceeded |
| `Deploying` | `Ready` | InferenceService ready and serving |
| `Deploying` | `Failed` | Deployment timeout or permanent error |
| `Ready` | `Deploying` | Spec changed, rollout in progress |
| `Ready` | `Degraded` | Health check failures (future) |
| `Ready` | `Failed` | Pod crash loop or unrecoverable error |
| `Disabled` | `Pending` | `spec.enabled: true` |
| `Failed` | `Pending` | Manual intervention (annotation/edit) |
| Any | `Disabled` | `spec.enabled: false` |

### Invalid Transitions

Operators MUST NOT allow these transitions:
- `Ready` → `Downloading` (cannot go backwards in lifecycle)
- `Failed` → `Ready` (must go through Pending)
- `Disabled` → `Ready` (must go through Pending)

## Phase Transition SLAs

Operators MUST enforce maximum durations for each phase. If a phase exceeds its SLA, the operator MUST take the specified action.

| From | To | Max Duration | On Timeout Action | Rationale |
|------|-----|-------------|-------------------|-----------|
| `Pending` | `Downloading` | 5m | → `Failed` | S3 check should be fast |
| `Downloading` | `Downloaded` | 60m | → `RetryPending` | Large models take time |
| `Downloaded` | `Deploying` | 5m | → `Failed` | InferenceService creation is fast |
| `Deploying` | `Ready` | 30m | → `Failed` | GPU scheduling + model loading |
| `RetryPending` | `Downloading` | (backoff delay) | (automatic) | Exponential backoff applies |

### Timeout Implementation

Status fields for timeout tracking:
```yaml
status:
  phase: Deploying
  phaseStartTime: "2025-12-26T10:00:00Z"  # When phase started
  message: "Waiting for InferenceService to be ready"
```

On each reconcile:
1. Check if current phase has exceeded its SLA
2. If exceeded, transition to timeout action phase
3. Emit a Kubernetes Event with reason `PhaseTimeout`
4. Update status message with timeout details

## Retry Strategy

### Download Retries

| Configuration | Default | Description |
|--------------|---------|-------------|
| `maxDownloadRetries` | 5 | Maximum retry attempts |
| `initialRetryDelay` | 1m | Initial backoff delay |
| `maxRetryDelay` | 16m | Maximum backoff delay |
| `backoffMultiplier` | 2 | Exponential multiplier |

**Backoff Schedule:**

| Attempt | Delay | Cumulative Time |
|---------|-------|-----------------|
| 1 | 1m | 1m |
| 2 | 2m | 3m |
| 3 | 4m | 7m |
| 4 | 8m | 15m |
| 5 | 16m | 31m |
| 6+ | (no more retries) | → Failed |

**Jitter**: Add 0-10% random jitter to prevent thundering herd.

### Deployment Retries

Deployment failures (InferenceService not becoming ready) do NOT automatically retry. Instead:
1. Phase transitions to `Failed`
2. Event emitted with failure details
3. Manual intervention required

**Rationale**: Deployment failures often indicate configuration issues (bad image, insufficient resources, missing secrets) that won't self-heal.

## Error Classification

### Transient Errors

Transient errors are temporary and should be retried with backoff.

| Error Type | Example | Action |
|------------|---------|--------|
| Network timeout | S3 connection timeout | Retry with backoff |
| API rate limit | HuggingFace 429 | Retry with backoff |
| Resource conflict | Optimistic locking conflict | Immediate requeue |
| Temporary unavailable | KServe webhook unavailable | Retry with backoff |
| Kubernetes API 5xx | etcd overload | Retry with backoff |

### Permanent Errors

Permanent errors require manual intervention and should NOT be retried.

| Error Type | Example | Action |
|------------|---------|--------|
| Invalid configuration | Unknown runtime `xyz` | → `Failed`, emit Event |
| Missing secret | `s3-credentials` not found | → `Failed`, emit Event |
| Authentication failure | Invalid HuggingFace token | → `Failed`, emit Event |
| Model not found | HuggingFace 404 | → `Failed`, emit Event |
| Insufficient quota | GPU quota exceeded | → `Failed`, emit Event |
| Image pull error | Invalid image reference | → `Failed`, emit Event |

### Error Detection Patterns

```go
// Permanent error detection
func isPermanentError(err error) bool {
    if errors.IsNotFound(err) {
        return true  // Resource doesn't exist
    }
    if errors.IsForbidden(err) {
        return true  // Permission denied
    }
    if errors.IsInvalid(err) {
        return true  // Invalid configuration
    }
    // Check for specific error messages
    msg := err.Error()
    permanentPatterns := []string{
        "image not found",
        "invalid model id",
        "authentication failed",
        "quota exceeded",
    }
    for _, pattern := range permanentPatterns {
        if strings.Contains(strings.ToLower(msg), pattern) {
            return true
        }
    }
    return false
}
```

## Health Monitoring (Post-Ready)

Once a model reaches `Ready` phase, the operator MUST continue monitoring its health.

### Health Check Configuration

| Parameter | Default | Description |
|-----------|---------|-------------|
| `healthCheckInterval` | 60s | Time between health checks |
| `unhealthyThreshold` | 5 | Consecutive failures before `Degraded` |
| `crashLoopThreshold` | 3 | Pod restarts in 10m before `Failed` |
| `degradedTimeout` | 10m | Time in `Degraded` before alerting |

### Health Signals

| Signal | Source | Healthy Condition |
|--------|--------|-------------------|
| Pod status | Kubernetes | All pods Running, Ready |
| Readiness probe | InferenceService | Returning 200 |
| Replica count | InferenceService | `readyReplicas >= 1` |
| Restart count | Pod | `restartCount < crashLoopThreshold` in window |

### Health State Machine

```
┌───────┐     5 consecutive      ┌──────────┐     10m timeout      ┌────────┐
│ Ready │──── health failures ──▶│ Degraded │──── or crash loop ──▶│ Failed │
└───────┘                        └──────────┘                       └────────┘
    ▲                                  │
    │                                  │
    └────── 3 healthy checks ──────────┘
```

### Status Fields

```yaml
status:
  phase: Ready
  healthStatus:
    lastCheckTime: "2025-12-26T10:30:00Z"
    consecutiveFailures: 0
    lastFailureReason: ""
  readyReplicas: 1
  conditions:
    - type: Ready
      status: "True"
      lastTransitionTime: "2025-12-26T10:00:00Z"
    - type: Healthy
      status: "True"
      lastTransitionTime: "2025-12-26T10:30:00Z"
```

## Alerting Requirements

Operators MUST emit Kubernetes Events for significant state changes. Monitoring systems SHOULD alert on these conditions:

### Critical Alerts (P1)

| Condition | Event Reason | Alert Threshold |
|-----------|--------------|-----------------|
| Phase=Failed | `PhaseFailed` | Immediate |
| Pod crash loop | `CrashLoopDetected` | Immediate |
| All replicas down | `AllReplicasDown` | Immediate |

### Warning Alerts (P2)

| Condition | Event Reason | Alert Threshold |
|-----------|--------------|-----------------|
| Phase=Deploying > 30m | `DeploymentStuck` | After 30m |
| Phase=Degraded | `HealthDegraded` | After 10m |
| Download retry | `DownloadRetry` | After 3rd retry |

### Informational Events

| Condition | Event Reason | Notes |
|-----------|--------------|-------|
| Phase transition | `PhaseTransition` | Log all transitions |
| Download started | `DownloadStarted` | Include model ID |
| Download completed | `DownloadCompleted` | Include duration |
| InferenceService ready | `ModelReady` | Include endpoint |

### Event Format

```yaml
apiVersion: v1
kind: Event
metadata:
  name: my-model.abc123
  namespace: development
involvedObject:
  apiVersion: aimodel.ai-aas.io/v1alpha1
  kind: AIModel
  name: my-model
  namespace: development
type: Warning  # or Normal
reason: DeploymentStuck
message: "Model stuck in Deploying phase for 35m (SLA: 30m)"
firstTimestamp: "2025-12-26T10:00:00Z"
lastTimestamp: "2025-12-26T10:35:00Z"
count: 1
source:
  component: ai-model-operator
```

## Recovery Procedures

### Automatic Recovery

| Condition | Automatic Action |
|-----------|------------------|
| Download job failed | Retry with exponential backoff (up to max retries) |
| Transient API error | Requeue with backoff |
| Resource conflict | Immediate requeue |

### Manual Recovery Required

| Condition | Recovery Steps |
|-----------|---------------|
| Phase=Failed (download) | 1. Check job logs for error<br>2. Fix root cause (secrets, network, quota)<br>3. Delete failed job<br>4. Update AIModel annotation to force reconcile |
| Phase=Failed (deployment) | 1. Check InferenceService events<br>2. Check pod logs and events<br>3. Fix root cause (image, resources, tolerations)<br>4. Update AIModel to trigger reconcile |
| Crash loop | 1. Check pod logs for crash reason<br>2. Check resource limits<br>3. Check for OOM or GPU errors<br>4. Update AIModel with fixed configuration |
| Stuck in Deploying | 1. Check if InferenceService exists<br>2. Check Knative pods<br>3. Check for GPU scheduling issues<br>4. May need to delete/recreate InferenceService |
| Model Ready but not in /v1/models | 1. Check if deployment record exists in Admin API<br>2. Verify operator has Admin API client configured<br>3. Force reconcile (periodic sync will recreate record)<br>4. Or wait for next 5-minute periodic sync |

### Force Reconciliation

To force an immediate reconciliation:
```bash
kubectl annotate aimodel <name> -n <namespace> \
  ai-aas.io/reconcile-at="$(date +%s)" --overwrite
```

## Implementation Checklist

When implementing or modifying an operator, verify:

- [ ] All phase transitions are valid per state machine
- [ ] Phase SLAs are enforced with timeouts
- [ ] `phaseStartTime` is set on phase transitions
- [ ] Transient errors use exponential backoff
- [ ] Permanent errors transition to Failed immediately
- [ ] Events are emitted for all phase transitions
- [ ] Events include actionable error messages
- [ ] Health monitoring runs after Ready phase
- [ ] Crash loop detection is implemented
- [ ] Recovery procedures are documented

## Related Documents

| Document | Purpose |
|----------|---------|
| [AI Model Operator](./ai-model-operator.md) | CRD reference and reconciliation flow |
| [Operator Patterns](./operator-patterns.md) | Code patterns for controller-runtime |
| [Debugging Workflow](../runbooks/ai-debugging-workflow.md) | Observability and debugging |

## Changelog

| Date | Change |
|------|--------|
| 2025-12-26 | Initial version created (aas-eeyj) |
