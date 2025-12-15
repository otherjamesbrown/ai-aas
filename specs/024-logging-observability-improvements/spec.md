# Spec 024: Logging & Observability Improvements

## Overview

This specification proposes improvements to the AI-AAS platform's logging and observability infrastructure to enable effective debugging, error capture, and production incident investigation.

## Problem Statement

### Current State

The platform has foundational logging in place:
- Go services use Uber Zap with structured JSON output
- Shared logging package provides consistency and sensitive data redaction
- OpenTelemetry instrumentation exists for distributed tracing
- ErrorBoundary catches React rendering errors

### Critical Gaps

| Gap | Impact | Severity |
|-----|--------|----------|
| No log aggregation backend | Cannot search historical logs or debug past issues | **Critical** |
| Logs not persisted | Lose all context when pods restart | **Critical** |
| No error tracking/alerting | Issues go unnoticed until user reports | **High** |
| Inconsistent frontend logging | Difficult to filter/debug browser issues | **Medium** |
| No request/response logging | Cannot replay or analyze failed API calls | **Medium** |
| No log sampling | Health check noise floods log output | **Low** |

### Impact

Without these capabilities, debugging production issues requires:
1. Hoping the relevant pod hasn't restarted
2. Manually tailing logs across multiple services
3. Guessing at request flows without correlation
4. No historical data for pattern analysis

## Proposed Solution

### Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           Kubernetes Cluster                            │
│                                                                         │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐                  │
│  │ api-router   │  │ user-org     │  │ analytics    │                  │
│  │   service    │  │   service    │  │   service    │                  │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘                  │
│         │                 │                 │                          │
│         │    stdout/stderr (JSON logs)      │                          │
│         ▼                 ▼                 ▼                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                    Promtail (DaemonSet)                         │   │
│  │         Collects logs from all pods via container runtime       │   │
│  └─────────────────────────────┬───────────────────────────────────┘   │
│                                │                                       │
│                                ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                         Loki                                    │   │
│  │              Log aggregation and storage                        │   │
│  └─────────────────────────────┬───────────────────────────────────┘   │
│                                │                                       │
│                                ▼                                       │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │                       Grafana                                   │   │
│  │         Dashboards, log exploration, alerting                   │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ┌──────────────┐                                                      │
│  │  Web Portal  │ ──────► Sentry (External SaaS)                       │
│  │  (Browser)   │         Error tracking & session replay              │
│  └──────────────┘                                                      │
└─────────────────────────────────────────────────────────────────────────┘
```

### Component Breakdown

#### 1. Log Aggregation Stack (Loki + Promtail + Grafana)

**Why Loki over ELK?**
- Native Prometheus ecosystem integration
- Lower resource footprint
- Label-based indexing matches our structured logging
- Already have Grafana patterns in the codebase

**Components:**
- **Promtail**: DaemonSet that tails container logs and pushes to Loki
- **Loki**: Horizontally-scalable log aggregation system
- **Grafana**: Visualization and alerting (may already exist for Prometheus)

#### 2. Frontend Logging Library

**Proposal: Implement structured browser logger**

Replace scattered `console.log` calls with a dedicated logger that:
- Supports log levels (debug, info, warn, error)
- Outputs structured JSON in production
- Integrates with OpenTelemetry for trace correlation
- Can be silenced in production for non-errors

#### 3. Request/Response Logging Middleware

**For Go services:**
- Log request metadata (method, path, headers minus sensitive ones)
- Log response status and duration
- Correlate with request_id and trace_id
- Configurable body logging for debugging

#### 4. Error Tracking (Sentry)

**Why Sentry?**
- ErrorBoundary already has Sentry integration scaffolded
- Industry standard for frontend error tracking
- Session replay helps reproduce issues
- Source map support for stack traces

**Scope:**
- Frontend: Unhandled exceptions, ErrorBoundary catches, API errors
- Backend: Optional integration for panic recovery

#### 5. Enhanced OTEL Collector Configuration

Update the existing collector to:
- Export traces to Jaeger or Tempo
- Export logs to Loki (via OTLP)
- Add service graph generation

#### 6. Inference Backend Logging (vLLM)

**Critical for debugging model deployment and inference issues.**

vLLM and other inference backends output logs to stdout/stderr. These logs are essential for debugging:
- Model loading failures (OOM, missing files, architecture mismatches)
- GPU allocation issues (CUDA errors, memory fragmentation)
- Inference errors (token limits, malformed inputs)
- Performance problems (queue depth, batch sizing)

**Components:**
- **Promtail scrape config for vLLM pods**: Collect logs from `ai-models` namespace
- **vLLM log parsing pipeline**: Extract model name, GPU metrics, inference timing
- **Dedicated Grafana dashboard**: Model health, inference latency, error rates

**vLLM Log Categories:**
| Log Pattern | Severity | Indicates |
|-------------|----------|-----------|
| `Loading model` | info | Model initialization starting |
| `Model loaded successfully` | info | Model ready for inference |
| `CUDA out of memory` | error | GPU memory exhaustion |
| `torch.cuda.OutOfMemoryError` | error | GPU OOM during inference |
| `KV cache` | info/warn | Memory allocation for context |
| `Request timed out` | warn | Inference exceeded timeout |
| `AsyncEngineDeadError` | error | vLLM engine crashed |
| `ValueError` | error | Invalid input/configuration |

**Required Promtail Labels for Inference Logs:**
- `namespace`: `ai-models` or inference namespace
- `model`: Model deployment name
- `pod`: Pod name for instance identification
- `container`: `vllm` or inference container name

## Design Decisions

### Decision 1: Loki vs ELK vs CloudWatch

| Option | Pros | Cons |
|--------|------|------|
| **Loki** | Low resource usage, Prometheus native, simple | Less powerful full-text search |
| ELK | Powerful search, mature | Heavy resource requirements |
| CloudWatch | Managed, no ops | Vendor lock-in, cost at scale |

**Decision: Loki** - Matches our Prometheus ecosystem, sufficient for our scale.

### Decision 2: Self-hosted vs SaaS for Error Tracking

| Option | Pros | Cons |
|--------|------|------|
| **Sentry SaaS** | No ops, session replay, mature | Monthly cost |
| Self-hosted Sentry | Free, data control | Complex to operate |
| GlitchTip | Simpler self-hosted | Fewer features |

**Decision: Sentry SaaS** - Free tier sufficient for development, worth the cost for production reliability.

### Decision 3: Log Retention Policy

| Environment | Debug/Info | Warn/Error | Audit |
|-------------|------------|------------|-------|
| Development | 7 days | 14 days | 30 days |
| Production | 14 days | 30 days | 90 days |

## Log Field Schema

All platform services MUST output structured JSON logs with consistent fields to enable automated debugging and correlation.

### Required Fields (Always Present)

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| `level` | string | Log level | `debug`, `info`, `warn`, `error` |
| `ts` | string | ISO8601 timestamp | `2024-01-15T10:30:00.123Z` |
| `msg` | string | Human-readable message | `request completed` |
| `service` | string | Service name | `api-router-service` |

### Contextual Fields (Present When Applicable)

| Field | Type | When Present | Description |
|-------|------|--------------|-------------|
| `request_id` | string | HTTP requests | Unique request correlation ID |
| `trace_id` | string | Traced requests | OpenTelemetry trace ID |
| `span_id` | string | Traced requests | OpenTelemetry span ID |
| `method` | string | HTTP requests | HTTP method |
| `path` | string | HTTP requests | Request path |
| `status` | int | Request completion | HTTP response status |
| `duration_ms` | float | Request completion | Request duration in milliseconds |
| `error` | string | On errors | Error message |
| `stack` | string | On panics/exceptions | Stack trace |
| `user_id` | string | Authenticated requests | User identifier |
| `org_id` | string | Authenticated requests | Organization identifier |

### Inference Backend Fields (vLLM/Model Serving)

| Field | Type | When Present | Description |
|-------|------|--------------|-------------|
| `model` | string | Model operations | Model name/ID |
| `model_version` | string | Model operations | Model version |
| `gpu_id` | int | GPU operations | GPU device ID |
| `gpu_memory_used_mb` | int | GPU operations | GPU memory usage |
| `tokens_in` | int | Inference requests | Input token count |
| `tokens_out` | int | Inference requests | Output token count |
| `inference_time_ms` | float | Inference completion | Model inference time |
| `queue_time_ms` | float | Inference completion | Time spent in queue |

### Error Context Fields (On 4xx/5xx Responses)

| Field | Type | Description |
|-------|------|-------------|
| `error_code` | string | Application error code |
| `error_type` | string | Error classification |
| `request_body_sample` | string | First 1KB of request body (redacted) |
| `response_body_sample` | string | First 1KB of response body |

## Success Criteria

1. **Log Search**: Can search logs across all services for the past 14 days
2. **Trace Correlation**: Can follow a request from frontend through all backend services
3. **Error Alerting**: Production errors trigger notifications within 5 minutes
4. **Debug Workflow**: Can reproduce and debug an issue reported by a user within 1 hour
5. **No Log Loss**: Logs persist through pod restarts and deployments
6. **Inference Visibility**: Can query vLLM logs for model loading, inference errors, and GPU issues

## Risks and Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Loki storage costs grow | Medium | Medium | Implement retention policies, log sampling |
| Sentry quota exceeded | Low | Low | Configure rate limiting, filter noise |
| Performance impact from logging | Low | Medium | Async logging, sampling for verbose endpoints |
| Sensitive data in logs | Medium | High | Extend redaction patterns, audit log output |

## Out of Scope

- Application Performance Monitoring (APM) - future enhancement
- Log-based anomaly detection - future enhancement
- Custom dashboards for business metrics - separate initiative
- Multi-region log replication - not needed at current scale

## References

- [Grafana Loki Documentation](https://grafana.com/docs/loki/latest/)
- [Sentry Documentation](https://docs.sentry.io/)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
- Current logging implementation: `shared/go/logging/`
- Current OTEL config: `infra/k8s/monitoring/otel-collector.yaml`
