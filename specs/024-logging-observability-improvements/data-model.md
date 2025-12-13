# Data Model: Logging & Observability

**Feature**: 024-logging-observability-improvements
**Date**: 2025-12-12

## Overview

This document defines the data structures for the observability system: log entry schemas, Loki labels, and query patterns.

---

## 1. Log Entry Schema

All platform services output structured JSON logs to stdout/stderr.

### 1.1 Base Log Entry

```json
{
  "level": "info",
  "ts": "2024-01-15T10:30:00.123Z",
  "msg": "request completed",
  "service": "api-router-service",
  "environment": "development"
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `level` | string | Yes | Log level: `debug`, `info`, `warn`, `error` |
| `ts` | string | Yes | ISO8601 timestamp with milliseconds |
| `msg` | string | Yes | Human-readable log message |
| `service` | string | Yes | Service name identifier |
| `environment` | string | Yes | Environment: `development`, `staging`, `production` |

### 1.2 HTTP Request Fields

Added by request logger middleware on HTTP requests.

```json
{
  "level": "info",
  "ts": "2024-01-15T10:30:00.123Z",
  "msg": "request_completed",
  "service": "api-router-service",
  "method": "POST",
  "path": "/v1/chat/completions",
  "status": 200,
  "duration_ms": 1523.45,
  "request_id": "req_abc123",
  "trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
  "span_id": "00f067aa0ba902b7",
  "response_bytes": 2048
}
```

| Field | Type | When Present | Description |
|-------|------|--------------|-------------|
| `method` | string | HTTP requests | HTTP method (GET, POST, etc.) |
| `path` | string | HTTP requests | Request path |
| `query` | string | HTTP requests | Query string (if present) |
| `status` | int | Request completion | HTTP response status code |
| `duration_ms` | float | Request completion | Request duration in milliseconds |
| `request_id` | string | HTTP requests | Unique request correlation ID |
| `trace_id` | string | Traced requests | OpenTelemetry trace ID (32 hex chars) |
| `span_id` | string | Traced requests | OpenTelemetry span ID (16 hex chars) |
| `response_bytes` | int | Request completion | Response body size |
| `remote_addr` | string | HTTP requests | Client IP address |

### 1.3 Authentication Context Fields

Added when request is authenticated.

```json
{
  "user_id": "usr_12345",
  "org_id": "org_67890",
  "api_key_id": "key_abcdef"
}
```

| Field | Type | When Present | Description |
|-------|------|--------------|-------------|
| `user_id` | string | Authenticated | User identifier |
| `org_id` | string | Authenticated | Organization identifier |
| `api_key_id` | string | API key auth | API key identifier (not the key itself) |

### 1.4 Error Context Fields

Added on 4xx/5xx responses when `LogBodyOnError` is enabled.

```json
{
  "level": "error",
  "msg": "request_completed",
  "status": 500,
  "error": "database connection timeout",
  "error_code": "DB_TIMEOUT",
  "error_type": "DatabaseError",
  "request_body_sample": "{\"model\": \"gpt-4\", \"messages\": [...]}",
  "response_body_sample": "{\"error\": {\"message\": \"Internal server error\"}}"
}
```

| Field | Type | When Present | Description |
|-------|------|--------------|-------------|
| `error` | string | On errors | Error message |
| `error_code` | string | On errors | Application error code |
| `error_type` | string | On errors | Error classification |
| `stack` | string | On panics | Stack trace |
| `request_body_sample` | string | On 4xx/5xx | First 1KB of request body (redacted) |
| `response_body_sample` | string | On 4xx/5xx | First 1KB of response body |

### 1.5 Inference Backend Fields (vLLM)

vLLM logs are mixed format. Promtail extracts these fields via regex.

```json
{
  "level": "info",
  "msg": "Model loaded successfully",
  "model": "gpt-oss-20b",
  "gpu_id": 0,
  "gpu_memory_used_mb": 45000,
  "tokens_in": 150,
  "tokens_out": 500,
  "inference_time_ms": 1200.5,
  "queue_time_ms": 50.2
}
```

| Field | Type | When Present | Description |
|-------|------|--------------|-------------|
| `model` | string | Model operations | Model name/ID |
| `model_version` | string | Model operations | Model version |
| `gpu_id` | int | GPU operations | GPU device ID |
| `gpu_memory_used_mb` | int | GPU operations | GPU memory usage in MB |
| `tokens_in` | int | Inference requests | Input token count |
| `tokens_out` | int | Inference requests | Output token count |
| `inference_time_ms` | float | Inference completion | Model inference time |
| `queue_time_ms` | float | Inference completion | Time spent in request queue |

---

## 2. Loki Label Schema

Labels are indexed by Loki for fast querying. Keep cardinality low.

### 2.1 Platform Service Labels

| Label | Source | Cardinality | Example Values |
|-------|--------|-------------|----------------|
| `namespace` | K8s metadata | Low (~5) | `default`, `system`, `ai-models` |
| `pod` | K8s metadata | Medium | `api-router-service-abc123` |
| `container` | K8s metadata | Low (~10) | `api-router`, `vllm` |
| `service` | Log JSON | Low (~6) | `api-router-service`, `user-org-service` |
| `level` | Log JSON | Very Low (4) | `debug`, `info`, `warn`, `error` |

### 2.2 Inference Backend Labels

| Label | Source | Cardinality | Example Values |
|-------|--------|-------------|----------------|
| `model` | Pod label | Low (~10) | `gpt-oss-20b`, `llama-7b` |
| `gpu_error` | Regex match | Very Low (3) | `oom`, `cuda`, `crash` |
| `model_status` | Regex match | Very Low (3) | `loading`, `loaded`, `failed` |

### 2.3 Label Extraction Rules

```yaml
# Promtail relabel_configs
relabel_configs:
  - source_labels: [__meta_kubernetes_namespace]
    target_label: namespace
  - source_labels: [__meta_kubernetes_pod_name]
    target_label: pod
  - source_labels: [__meta_kubernetes_pod_container_name]
    target_label: container
  - source_labels: [__meta_kubernetes_pod_label_app]
    target_label: service
  - source_labels: [__meta_kubernetes_pod_label_serving_kserve_io_inferenceservice]
    target_label: model
```

---

## 3. Query Patterns (LogQL)

### 3.1 Basic Queries

```logql
# All errors
{level="error"}

# Errors for specific service
{service="api-router-service", level="error"}

# Logs containing text
{service="api-router-service"} |= "timeout"

# Logs matching regex
{service="api-router-service"} |~ "status=[45][0-9]{2}"
```

### 3.2 JSON Parsing Queries

```logql
# Parse JSON and filter by field
{service="api-router-service"} | json | status >= 400

# Extract specific fields
{service="api-router-service"} | json | line_format "{{.method}} {{.path}} {{.status}} {{.duration_ms}}ms"

# Filter by duration
{service="api-router-service"} | json | duration_ms > 1000
```

### 3.3 Inference Backend Queries

```logql
# All vLLM logs
{container=~"vllm|kserve-container"}

# GPU errors
{container=~"vllm|kserve-container"} |~ "(?i)cuda|out.?of.?memory|oom"

# Model loading
{container=~"vllm|kserve-container"} |~ "(?i)loading model|model loaded|failed to load"

# Specific model
{model="gpt-oss-20b"}
```

### 3.4 Correlation Queries

```logql
# Trace request across services by request_id
{} |= "req_abc123"

# Find all logs for a trace
{} | json | trace_id="4bf92f3577b34da6a3ce929d0e0e4736"
```

### 3.5 Aggregation Queries (Metrics from Logs)

```logql
# Error count by service (last hour)
sum by(service) (count_over_time({level="error"}[1h]))

# Error rate over time
sum(rate({level="error"}[5m]))

# Top error messages
topk(10, sum by(msg) (count_over_time({level="error"} | json [1h])))

# P99 latency from logs (requires numeric extraction)
quantile_over_time(0.99, {service="api-router-service"} | json | unwrap duration_ms [5m])
```

---

## 4. Retention Configuration

### 4.1 Loki Retention Settings

```yaml
# loki-config.yaml
limits_config:
  retention_period: 336h  # 14 days default

table_manager:
  retention_deletes_enabled: true
  retention_period: 336h

chunk_store_config:
  max_look_back_period: 336h
```

### 4.2 Per-Environment Overrides

| Environment | Retention | Storage Estimate |
|-------------|-----------|------------------|
| Development | 14 days | ~50GB |
| Staging | 14 days | ~50GB |
| Production | 30 days | ~200GB |

---

## 5. Sensitive Data Handling

### 5.1 Fields Never Logged

- Passwords (any field containing `password`)
- Full API keys (log only `api_key_id`)
- Session tokens
- Credit card numbers
- SSN/government IDs

### 5.2 Redaction Patterns

```go
var sensitiveFields = []string{
    "password",
    "token",
    "secret",
    "api_key",
    "apiKey",
    "credit_card",
    "ssn",
    "authorization",
}
```

### 5.3 Header Filtering

Headers excluded from logging:
- `Authorization`
- `X-API-Key`
- `Cookie`
- `Set-Cookie`

---

## 6. Entity Relationships

```
┌─────────────────────────────────────────────────────────────────┐
│                        Log Entry                                 │
├─────────────────────────────────────────────────────────────────┤
│ Always: level, ts, msg, service, environment                    │
│                                                                  │
│ HTTP Request: method, path, status, duration_ms, request_id     │
│              trace_id, span_id                                   │
│                                                                  │
│ Auth Context: user_id, org_id, api_key_id                       │
│                                                                  │
│ Error Context: error, error_code, error_type, stack             │
│               request_body_sample, response_body_sample          │
│                                                                  │
│ Inference: model, gpu_id, tokens_in, tokens_out,                │
│           inference_time_ms, queue_time_ms                       │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Collected by
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                        Promtail                                  │
├─────────────────────────────────────────────────────────────────┤
│ Extracts Labels: namespace, pod, container, service, level,     │
│                 model, gpu_error, model_status                   │
│                                                                  │
│ Parses: JSON fields, regex patterns for vLLM                    │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Pushes to
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                          Loki                                    │
├─────────────────────────────────────────────────────────────────┤
│ Indexes: Labels only (low cardinality)                          │
│ Stores: Full log lines (compressed chunks)                      │
│ Queries: LogQL                                                  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              │ Queried by
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Query Interfaces                              │
├─────────────────────────────────────────────────────────────────┤
│ Grafana: Dashboards, Explore, Alerts                            │
│ HTTP API: curl queries for AI assistants                        │
│ LogCLI: Command-line queries                                    │
└─────────────────────────────────────────────────────────────────┘
```
