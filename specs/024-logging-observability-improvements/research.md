# Research: Logging & Observability Improvements

**Feature**: 024-logging-observability-improvements
**Date**: 2025-12-12
**Status**: Complete

## Overview

This document captures technology decisions and research findings for the observability stack implementation.

## Decision 1: Log Aggregation Stack

### Decision
**Loki + Promtail + Grafana** (Grafana Labs stack)

### Rationale
- Native integration with existing Prometheus/Grafana ecosystem
- Lower resource footprint than ELK (no JVM overhead)
- Label-based indexing matches our structured JSON logging approach
- LogQL query language similar to PromQL (team familiarity)
- HTTP API enables programmatic access for AI assistants

### Alternatives Considered

| Alternative | Pros | Cons | Why Rejected |
|-------------|------|------|--------------|
| ELK (Elasticsearch/Logstash/Kibana) | Powerful full-text search, mature ecosystem | Heavy resource requirements (JVM), complex operations, expensive at scale | Resource overhead too high for our scale |
| CloudWatch Logs | Fully managed, no ops burden | Vendor lock-in, cost scales poorly, limited query capabilities | Platform is cloud-agnostic; want to avoid AWS lock-in |
| Fluentd + ClickHouse | High performance, SQL queries | More complex setup, less ecosystem integration | Additional operational complexity not justified |

### References
- [Grafana Loki vs ELK comparison](https://grafana.com/docs/loki/latest/)
- Existing Grafana deployment at `grafana.dev.ai-aas.local`

---

## Decision 2: Frontend Error Tracking

### Decision
**Sentry SaaS** (cloud-hosted)

### Rationale
- ErrorBoundary already has Sentry integration scaffolded in codebase
- Session replay feature helps reproduce browser issues
- Source map support for readable stack traces
- Free tier sufficient for development; predictable costs for production
- No operational overhead

### Alternatives Considered

| Alternative | Pros | Cons | Why Rejected |
|-------------|------|------|--------------|
| Self-hosted Sentry | Free, full data control | Complex to operate (Redis, PostgreSQL, Kafka, etc.), upgrade burden | Operational complexity not worth cost savings |
| GlitchTip | Simpler self-hosted alternative | Fewer features, smaller community | Missing session replay, less mature |
| Rollbar | Good error grouping | Higher cost, less frontend-focused | Sentry better fits React ecosystem |
| Browser logs to Loki | Unified stack | No session replay, poor browser error context | Loses key debugging capabilities |

### Configuration
- Environment: Differentiate dev/staging/prod in Sentry projects
- Sample rates: 100% errors, 10% traces in prod
- PII scrubbing: Remove Authorization headers, redact API keys

---

## Decision 3: vLLM Log Collection Strategy

### Decision
**Promtail with regex-based parsing** for mixed-format vLLM logs

### Rationale
- vLLM outputs mixed format: some JSON, some plain text with Python logging format
- Need to extract: log level, model name, GPU errors, loading status
- Regex patterns provide flexibility for format variations across vLLM versions
- Labels enable efficient querying by model, error type, namespace

### Implementation Approach

1. **Namespace targeting**: Scrape `ai-models`, `system`, `kserve` namespaces
2. **Container filtering**: Match `vllm`, `kserve-container`, `inference`, `transformer`
3. **Two-pass parsing**:
   - First: Try JSON extraction for structured logs
   - Second: Regex fallback for Python logging format (`INFO:`, `ERROR:`, etc.)
4. **Label extraction**:
   - `model`: From pod label `serving.kserve.io/inferenceservice`
   - `gpu_error`: Pattern match for CUDA/OOM/GPU keywords
   - `model_status`: Pattern match for loading/loaded/failed

### Key Patterns to Extract

| Pattern | Label | Purpose |
|---------|-------|---------|
| `CUDA out of memory` | `gpu_error=oom` | GPU memory exhaustion |
| `torch.cuda.OutOfMemoryError` | `gpu_error=oom` | PyTorch OOM |
| `AsyncEngineDeadError` | `gpu_error=crash` | vLLM engine crash |
| `Loading model` | `model_status=loading` | Model initialization |
| `Model loaded successfully` | `model_status=loaded` | Model ready |
| `Failed to load` | `model_status=failed` | Load failure |

### Risks & Mitigations
- **Risk**: vLLM log format changes between versions
- **Mitigation**: Use flexible regex patterns, pin vLLM version, add version label

---

## Decision 4: Log Retention Policy

### Decision
**Tiered retention by environment and severity**

| Environment | Debug/Info | Warn/Error | Audit |
|-------------|------------|------------|-------|
| Development | 7 days | 14 days | 30 days |
| Production | 14 days | 30 days | 90 days |

### Rationale
- Balance storage costs with debugging needs
- Longer retention for errors (investigation often delayed)
- Audit logs retained longer for compliance
- Development can have shorter retention (more frequent deployments)

### Implementation
- Loki `table_manager.retention_period` for global retention
- Promtail drop rules for debug logs in production
- Separate compaction policies by label

---

## Decision 5: Request Body Logging

### Decision
**Log request/response body ONLY on errors (4xx/5xx)** with redaction

### Rationale
- Full body logging too expensive (storage, performance)
- Error debugging requires seeing what payload caused failure
- Sensitive fields must be redacted (passwords, tokens, API keys)
- 1KB limit prevents large payload storage

### Redaction Strategy
Fields to redact:
- `password`, `token`, `secret`, `api_key`, `apiKey`
- `credit_card`, `ssn`, `authorization`

Implementation: Regex replacement before logging

---

## Decision 6: Correlation ID Strategy

### Decision
**Dual ID system**: `request_id` (per-request) + `trace_id` (distributed trace)

### Rationale
- `request_id`: Simple UUID, always present, easy to grep
- `trace_id`: OpenTelemetry standard, enables cross-service tracing
- Both propagated via headers: `X-Request-ID`, `traceparent`
- Logs include both for maximum queryability

### Propagation
1. API Router generates `request_id` if not present
2. OpenTelemetry SDK generates `trace_id`
3. Both passed to downstream services via headers
4. All services log both IDs

---

## Open Questions (Resolved)

| Question | Resolution |
|----------|------------|
| Use Tempo or Jaeger for traces? | Tempo (Grafana ecosystem consistency) |
| Where to store Loki data? | PersistentVolume on Kubernetes |
| How to handle log cardinality? | Label allowlist, drop high-cardinality fields |
| Grafana authentication? | Existing Grafana auth, add Loki datasource |

---

## Next Steps

1. Proceed to Phase 1: Infrastructure deployment (Loki, Promtail)
2. Create Helm charts or raw manifests based on architecture.md
3. Configure ArgoCD Applications for GitOps deployment
