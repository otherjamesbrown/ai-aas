# Research: Model Readiness Probes for KServe InferenceServices

**Feature**: `018-model-readiness-probes`
**Date**: 2025-11-28

## Research Summary

This document consolidates research findings for implementing Kubernetes health probes on vLLM InferenceServices to prevent traffic routing before model loading completes.

---

## R1: vLLM Health Endpoint Behavior

### Decision
Use vLLM's native `/health` endpoint on port 8000 (or 8080 for older versions) for all probe types.

### Rationale
- **Native Support**: vLLM provides `/health` endpoint that returns HTTP 200 only when model is fully loaded and ready for inference
- **Semantic Accuracy**: Unlike TCP probes, HTTP probes verify actual application readiness
- **Industry Standard**: Aligns with KServe and Kubernetes best practices
- **Low Overhead**: Simple HTTP GET with <10ms response time

### Alternatives Considered

| Alternative | Why Rejected |
|-------------|--------------|
| TCP Probe | Only checks if port is listening, not if model is loaded |
| Custom `/readyz` endpoint | Adds complexity; vLLM `/health` already provides correct semantics |
| Exec probe with script | Higher overhead, harder to maintain |
| gRPC health check | vLLM OpenAI-compatible server uses HTTP, not gRPC |

### vLLM Endpoint Behavior

| State | `/health` Response |
|-------|-------------------|
| Container starting | Connection refused |
| Downloading model | HTTP 503 |
| Loading to GPU | HTTP 503 |
| Model ready | HTTP 200 |
| Processing request | HTTP 200 |
| GPU memory error | HTTP 503 or connection refused |

---

## R2: Startup Probe Timeout Configuration

### Decision
Configure startup probe timeouts based on model size category:

| Model Size | Parameters | initialDelaySeconds | periodSeconds | failureThreshold | Total Timeout |
|------------|-----------|---------------------|---------------|------------------|---------------|
| 7B | 7B | 30 | 10 | 36 | ~6 minutes |
| 13B | 13B | 30 | 10 | 60 | ~10 minutes |
| 20B | 20B | 30 | 10 | 90 | ~15 minutes |
| 70B+ | 70B+ | 60 | 10 | 180 | ~30 minutes |

### Rationale
- **Empirical Data**: Based on observed model loading times on RTX 4000 Ada (20GB VRAM) and A100 GPUs
- **Safety Margin**: Configured for ~20% buffer above typical load times
- **Network Variability**: Accounts for HuggingFace download speed variations
- **Cold Cache**: Initial download takes longer than subsequent loads from cache

### Alternatives Considered

| Alternative | Why Rejected |
|-------------|--------------|
| Single timeout for all models | Would either be too long for small models or kill large models |
| Dynamic timeout based on model metadata | Adds complexity; static categories sufficient |
| Shorter periodSeconds (5s) | More probe traffic, minimal benefit |

### Model Loading Time Observations

| Model | GPU | First Load (cold) | Subsequent Load (cached) |
|-------|-----|-------------------|--------------------------|
| mistral-7b-instruct | RTX 4000 Ada | 3-5 min | 2-3 min |
| llama-2-7b | RTX 4000 Ada | 3-5 min | 2-3 min |
| gpt-oss-20b | RTX 4000 Ada | 8-12 min | 5-8 min |
| llama-2-70b (theoretical) | A100 80GB | 20-30 min | 15-20 min |

---

## R3: Readiness vs Liveness Probe Separation

### Decision
Use separate configurations for readiness and liveness probes:

**Readiness Probe**:
- `periodSeconds: 10` (frequent checks)
- `failureThreshold: 3` (remove from service quickly)
- Purpose: Traffic gating

**Liveness Probe**:
- `periodSeconds: 30` (less frequent)
- `failureThreshold: 3` (allow recovery time)
- Purpose: Crash detection and restart

### Rationale
- **Different Purposes**: Readiness controls traffic routing; liveness controls pod restart
- **Avoid False Positives**: Liveness with tight thresholds could restart healthy-but-busy pods
- **Industry Standard**: Kubernetes documentation recommends separate configurations
- **Knative Compatibility**: Works correctly with Knative activator and queue-proxy

### Alternatives Considered

| Alternative | Why Rejected |
|-------------|--------------|
| Same config for both | Liveness would be too aggressive, causing unnecessary restarts |
| Liveness only | Would lose ability to remove slow pods from service without restart |
| Readiness only | Would miss crash detection and auto-recovery |

---

## R4: Port Configuration for Different vLLM Versions

### Decision
Configure probes for port 8000 (newer vLLM v0.6+) as default, with awareness of port 8080 for older versions.

### Rationale
- **Version Alignment**: All deployed models use vLLM v0.6.x or v0.10.x (port 8000)
- **Backward Compatibility**: llama-2-7b uses v0.3.0 with port 8080 (documented in manifest)
- **Explicit Configuration**: Port is explicitly set in each manifest to avoid ambiguity

### Port Configuration by vLLM Version

| vLLM Version | Default Port | Models Using |
|--------------|-------------|--------------|
| v0.3.x | 8080 | llama-2-7b |
| v0.6.x | 8000 | mistral-7b-instruct |
| v0.10.x | 8000 | gpt-oss-20b |

---

## R5: minReplicas Configuration for Cold Start Prevention

### Decision
Set `minReplicas: 1` (minimum) for all production InferenceServices to prevent cold start delays.

### Rationale
- **Cold Start Penalty**: Model loading takes 5-15 minutes; unacceptable for first request
- **User Experience**: First user request must not wait for model to load
- **Cost Tradeoff**: GPU cost is acceptable vs. 15-minute latency penalty
- **Knative Default**: Knative allows scale-to-zero, which we explicitly disable

### Alternatives Considered

| Alternative | Why Rejected |
|-------------|--------------|
| minReplicas: 0 (scale-to-zero) | 5-15 minute cold start unacceptable for production |
| Higher minReplicas (2+) | Unnecessary cost for development; appropriate only for high-traffic production |

### Environment Configuration

| Environment | Recommended minReplicas | Rationale |
|-------------|------------------------|-----------|
| Development | 1 | Avoid cold starts during testing |
| Staging | 1 | Mirror production behavior |
| Production | 1-3 | Based on baseline traffic, ensure availability |

---

## R6: Knative/KServe Compatibility

### Decision
Configure probes on the `kserve-container` only, relying on Knative's queue-proxy for traffic management.

### Rationale
- **Knative Architecture**: queue-proxy sidecar handles traffic gating based on kserve-container readiness
- **Pod Status**: Pod shows `1/2 Running` when queue-proxy ready but kserve-container not ready
- **Traffic Routing**: Knative activator respects readiness; won't route traffic until `2/2 Running`
- **No queue-proxy Probes Needed**: Knative manages queue-proxy health internally

### Pod Status Interpretation

| queue-proxy | kserve-container | Pod Status | Traffic Routed |
|-------------|------------------|------------|----------------|
| Ready | Not Ready | 1/2 Running | No |
| Ready | Ready | 2/2 Running | Yes |
| Not Ready | Any | 0/2 Running | No |

---

## R7: Existing Implementation Analysis

### Decision
The probe configuration has already been implemented in all InferenceService manifests. Remaining work is validation and documentation.

### Current State (as of 2025-11-28)

| File | Probes Configured | Correct Settings |
|------|-------------------|------------------|
| `infra/k8s/kserve/models/gpt-oss-20b.yaml` | ✅ Yes | ✅ Yes (90×10s = 15min) |
| `infra/k8s/kserve/models/mistral-7b-instruct.yaml` | ✅ Yes | ✅ Yes (36×10s = 6min) |
| `infra/k8s/kserve/models/llama-2-7b.yaml` | ✅ Yes | ✅ Yes (36×10s = 6min) |
| `infra/k8s/kserve/templates/inference-service-vllm-template.yaml` | ✅ Yes | ✅ Template with placeholders |

### Existing Documentation

| Document | Probe Coverage | Status |
|----------|---------------|--------|
| `docs/best-practices/vllm-deployment-best-practices.md` | ✅ Detailed guidance | Complete |
| `docs/model-initialization.md` | ✅ Timeout strategy | Complete |
| `docs/runbooks/enable-model-readiness-probes.md` | ❌ Does not exist | Needs creation |

---

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| TCP vs HTTP probes? | HTTP - provides semantic accuracy |
| Appropriate failureThreshold for 70B? | 180 (30 minutes) |
| Custom health endpoint needed? | No - vLLM /health is sufficient |
| Port differences across versions? | Documented per-version; explicit in manifests |

---

## References

- [Kubernetes Configure Liveness, Readiness and Startup Probes](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
- [KServe InferenceService Spec](https://kserve.github.io/website/0.11/modelserving/v1beta1/custom/custom_model/)
- [vLLM OpenAI-Compatible Server](https://docs.vllm.ai/en/latest/serving/openai_compatible_server.html)
- [Knative Pod Autoscaling](https://knative.dev/docs/serving/autoscaling/)

