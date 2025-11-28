# Data Model: Model Readiness Probes

**Feature**: `018-model-readiness-probes`
**Date**: 2025-11-28

## Overview

This feature is **configuration-only** and does not introduce new data models or database schemas.

## Configuration Schema

The probe configuration is defined in Kubernetes InferenceService YAML manifests using standard Kubernetes probe schema.

### Probe Configuration Structure

```yaml
# Standard Kubernetes probe schema used in InferenceService containers
containers:
  - name: kserve-container
    startupProbe:           # Required for model loading
      httpGet:
        path: /health       # vLLM health endpoint
        port: 8000          # vLLM default port (8080 for v0.3.x)
      initialDelaySeconds: 30
      periodSeconds: 10
      failureThreshold: 90  # Adjust by model size
      timeoutSeconds: 5
    
    readinessProbe:         # Required for traffic gating
      httpGet:
        path: /health
        port: 8000
      periodSeconds: 10
      failureThreshold: 3
      timeoutSeconds: 5
    
    livenessProbe:          # Required for crash recovery
      httpGet:
        path: /health
        port: 8000
      periodSeconds: 30
      failureThreshold: 3
      timeoutSeconds: 5
```

### Configuration Parameters by Model Size

| Parameter | 7B Models | 13B Models | 20B Models | 70B+ Models |
|-----------|-----------|------------|------------|-------------|
| `startupProbe.initialDelaySeconds` | 30 | 30 | 30 | 60 |
| `startupProbe.periodSeconds` | 10 | 10 | 10 | 10 |
| `startupProbe.failureThreshold` | 36 | 60 | 90 | 180 |
| `startupProbe.timeoutSeconds` | 5 | 5 | 5 | 5 |
| **Total Startup Timeout** | ~6 min | ~10 min | ~15 min | ~30 min |

## No Database Changes

- No new tables
- No schema migrations
- No entity changes
- All configuration is declarative in Kubernetes manifests

## Related Configurations

Probe configurations interact with these existing configurations:

1. **InferenceService Scaling**: `minReplicas`, `maxReplicas` affect pod lifecycle
2. **Knative Autoscaling**: `autoscaling.knative.dev/*` annotations control scaling behavior
3. **Resource Limits**: GPU memory and CPU affect model loading time

