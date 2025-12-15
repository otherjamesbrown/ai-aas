# Inference Backend Dashboard Specification

## Overview

A comprehensive Grafana dashboard for monitoring ML inference backends (LLMs, vision models, embedding models) running across multiple GPUs. The dashboard should provide visibility into model performance, resource utilization, caching efficiency, and error rates.

## Requirements

### Functional Requirements

1. **Model Selection**: Dropdown to switch between deployed models
2. **GPU Aggregation**: Aggregate metrics across all GPUs serving a model, with drill-down capability
3. **Model Type Support**: Handle LLMs, vision models, embedding models, and multimodal models
4. **Real-time Monitoring**: Auto-refresh with configurable intervals (10s, 30s, 1m, 5m)

### Non-Functional Requirements

1. Dashboard should load in < 3 seconds
2. Queries should be efficient (use recording rules where needed)
3. Support for 10+ GPUs per model without performance degradation

---

## Dashboard Layout

### Top-Level Controls

| Variable | Type | Description |
|----------|------|-------------|
| `model` | Dropdown | Select model (e.g., `llama-70b`, `gpt-oss-20b`, `vision-v1`) |
| `aggregation` | Dropdown | View: `All GPUs` / `Per GPU` / `Per Replica` |
| `time_range` | Time picker | Standard Grafana time range |
| `refresh` | Interval | Auto-refresh rate |

---

## Section 1: Health Overview

**Purpose**: At-a-glance model health status

| Panel | Type | Metric/Query |
|-------|------|--------------|
| Model Status | Stat | `up{model="$model"}` - Running/Down indicator |
| Active Replicas | Stat | Count of healthy replicas (e.g., `3/3`) |
| GPU Count | Stat | Total GPUs serving this model |
| Current QPS | Stat | `rate(inference_requests_total{model="$model"}[1m])` |
| P50 Latency | Stat | `histogram_quantile(0.5, ...)` |
| P99 Latency | Stat | `histogram_quantile(0.99, ...)` |
| Error Rate | Stat | `rate(inference_errors_total[5m]) / rate(inference_requests_total[5m])` |

**Layout**: Single row of stat panels with color thresholds (green/yellow/red)

---

## Section 2: Throughput & Request Metrics

**Purpose**: Understand request volume and processing rates

### 2.1 Request Throughput

| Panel | Type | Metrics |
|-------|------|---------|
| Requests/sec | Time series | `rate(inference_requests_total{model="$model"}[1m])` by status |
| Request Queue Depth | Time series | `inference_queue_size{model="$model"}` |
| Concurrent Requests | Time series | `inference_requests_in_flight{model="$model"}` |
| Batch Size Distribution | Heatmap | `inference_batch_size_bucket{model="$model"}` |

### 2.2 Token Throughput (LLMs only)

| Panel | Type | Metrics |
|-------|------|---------|
| Input Tokens/sec | Time series | `rate(inference_prompt_tokens_total{model="$model"}[1m])` |
| Output Tokens/sec | Time series | `rate(inference_generation_tokens_total{model="$model"}[1m])` |
| Tokens per Request | Time series | Avg input/output tokens per request |

### 2.3 Image Throughput (Vision models only)

| Panel | Type | Metrics |
|-------|------|---------|
| Images/sec | Time series | `rate(inference_images_processed_total{model="$model"}[1m])` |
| Input Resolution Distribution | Heatmap | Resolution buckets |

---

## Section 3: Latency Metrics

**Purpose**: Identify latency bottlenecks and SLA compliance

### 3.1 End-to-End Latency

| Panel | Type | Metrics |
|-------|------|---------|
| Latency Percentiles | Time series | P50, P90, P95, P99 over time |
| Latency Distribution | Heatmap | `inference_request_duration_seconds_bucket` |
| Latency by Input Size | Heatmap | Latency vs input tokens (2D histogram) |

### 3.2 Latency Breakdown (LLMs)

| Panel | Type | Metrics | Description |
|-------|------|---------|-------------|
| Time to First Token (TTFT) | Time series | `inference_time_to_first_token_seconds` | User-perceived responsiveness |
| Inter-Token Latency | Time series | `inference_inter_token_latency_seconds` | Streaming smoothness |
| Prefill Time | Time series | `inference_prefill_duration_seconds` | Prompt processing time |
| Decode Time | Time series | `inference_decode_duration_seconds` | Token generation time |
| Queue Wait Time | Time series | `inference_queue_wait_seconds` | Time waiting before processing |

### 3.3 Latency SLA Compliance

| Panel | Type | Metrics |
|-------|------|---------|
| % Requests < 100ms | Gauge | Percentage meeting threshold |
| % Requests < 500ms | Gauge | Percentage meeting threshold |
| % Requests < 1s | Gauge | Percentage meeting threshold |
| SLA Violations/min | Time series | Count of requests exceeding SLA |

---

## Section 4: GPU Resource Utilization

**Purpose**: Monitor GPU health and identify resource bottlenecks

### 4.1 Compute Utilization

| Panel | Type | Metrics |
|-------|------|---------|
| GPU Utilization % | Time series | `DCGM_FI_DEV_GPU_UTIL{model="$model"}` aggregated |
| GPU Utilization Heatmap | Heatmap | Per-GPU utilization over time |
| SM Occupancy | Time series | `DCGM_FI_PROF_SM_OCCUPANCY` |

### 4.2 Memory Utilization

| Panel | Type | Metrics |
|-------|------|---------|
| GPU Memory Used (GB) | Time series | `DCGM_FI_DEV_FB_USED` |
| GPU Memory Utilization % | Time series | Used / Total |
| Memory per GPU | Bar gauge | Per-GPU memory breakdown |

### 4.3 GPU Health

| Panel | Type | Metrics |
|-------|------|---------|
| GPU Temperature | Time series | `DCGM_FI_DEV_GPU_TEMP` with threshold lines |
| Power Draw (W) | Time series | `DCGM_FI_DEV_POWER_USAGE` |
| Thermal Throttling Events | Stat | Count of throttling events |

### 4.4 Multi-GPU View

| Panel | Type | Description |
|-------|------|-------------|
| GPU Status Table | Table | All GPUs: utilization, memory, temp, power |
| GPU Topology | Node graph | Visual representation of GPU interconnects (optional) |

---

## Section 5: KV Cache & Model Caching

**Purpose**: Monitor cache efficiency (critical for LLM performance)

### 5.1 KV Cache Metrics

| Panel | Type | Metrics |
|-------|------|---------|
| KV Cache Utilization % | Gauge | `vllm_cache_usage_ratio` or equivalent |
| KV Cache Size (GB) | Time series | Current cache size |
| KV Cache Hit Rate | Time series | `rate(kv_cache_hits[1m]) / rate(kv_cache_requests[1m])` |
| KV Cache Evictions/sec | Time series | `rate(kv_cache_evictions_total[1m])` |

### 5.2 Prefix Caching (if enabled)

| Panel | Type | Metrics |
|-------|------|---------|
| Prefix Cache Hit Rate | Time series | Reused prefix ratio |
| Prefix Cache Size | Time series | Cached prefix tokens |

### 5.3 Context Length

| Panel | Type | Metrics |
|-------|------|---------|
| Context Length Distribution | Histogram | Distribution of request context lengths |
| Max Active Context | Stat | Longest active sequence |
| Avg Context Length | Stat | Mean context length |

---

## Section 6: Errors & Reliability

**Purpose**: Track errors and identify reliability issues

### 6.1 Error Overview

| Panel | Type | Metrics |
|-------|------|---------|
| Error Rate % | Gauge | `rate(errors) / rate(requests)` with thresholds |
| Errors/min | Time series | `rate(inference_errors_total[1m])` |
| Error Rate by Model | Bar chart | Compare error rates across models |

### 6.2 Error Breakdown

| Panel | Type | Metrics |
|-------|------|---------|
| Errors by Type | Pie chart | OOM, Timeout, Validation, Backend, Unknown |
| HTTP Status Codes | Time series | 2xx, 4xx, 5xx over time |
| Error Log Stream | Logs panel | Recent error logs from Loki |

### 6.3 Specific Error Types

| Panel | Type | Metrics |
|-------|------|---------|
| OOM Events | Stat + Time series | GPU out-of-memory count |
| Timeout Events | Stat + Time series | Requests exceeding deadline |
| Request Rejections | Time series | Rejected due to overload |
| CUDA Errors | Stat | CUDA-specific failures |

### 6.4 Reliability Indicators

| Panel | Type | Metrics |
|-------|------|---------|
| Availability % | Gauge | Uptime percentage |
| Mean Time Between Failures | Stat | MTBF calculation |
| Circuit Breaker Status | Stat | Open/Closed/Half-open |

---

## Section 7: Efficiency & Cost (Optional)

**Purpose**: Track resource efficiency and cost metrics

| Panel | Type | Metrics |
|-------|------|---------|
| Tokens per GPU-second | Time series | Throughput efficiency |
| GPU Idle Time % | Gauge | Wasted compute |
| Cost per 1K Tokens | Stat | If cost tracking enabled |
| Throughput per Watt | Time series | Power efficiency |

---

## Section 8: Model-Specific Panels

### 8.1 LLM-Specific

| Panel | Type | Metrics |
|-------|------|---------|
| Input/Output Token Ratio | Time series | Prompt vs generation balance |
| Streaming vs Non-streaming | Pie chart | Request type distribution |
| Stop Reason Distribution | Pie chart | EOS, max_tokens, stop_sequence |
| Generation Speed (tokens/sec) | Stat | Per-request generation rate |

### 8.2 Vision Model-Specific

| Panel | Type | Metrics |
|-------|------|---------|
| Input Resolution Distribution | Histogram | Image sizes processed |
| Preprocessing Time | Time series | Image prep latency |
| Detection Confidence | Histogram | For object detection models |
| Classes Detected | Bar chart | Top detected classes |

### 8.3 Embedding Model-Specific

| Panel | Type | Metrics |
|-------|------|---------|
| Embeddings/sec | Time series | Embedding throughput |
| Input Sequence Length | Histogram | Token counts |
| Batch Efficiency | Gauge | Actual vs optimal batch size |

---

## Data Sources Required

### Prometheus Metrics

| Source | Metrics |
|--------|---------|
| vLLM / TGI / Triton | Inference metrics (requests, tokens, latency) |
| DCGM Exporter | GPU metrics (utilization, memory, temp, power) |
| NVIDIA GPU Operator | GPU health and status |
| Application metrics | Custom business metrics |

### Loki (Logs)

- Inference service logs for error details
- GPU driver logs for hardware issues

---

## Implementation Notes

### Recording Rules (for performance)

Consider creating recording rules for expensive queries:

```yaml
groups:
  - name: inference_metrics
    rules:
      - record: inference:request_rate:1m
        expr: rate(inference_requests_total[1m])
      - record: inference:error_rate:5m
        expr: rate(inference_errors_total[5m]) / rate(inference_requests_total[5m])
      - record: inference:p99_latency:5m
        expr: histogram_quantile(0.99, rate(inference_request_duration_seconds_bucket[5m]))
```

### Dashboard Variables

```yaml
variables:
  - name: model
    type: query
    query: label_values(inference_requests_total, model)

  - name: gpu
    type: query
    query: label_values(DCGM_FI_DEV_GPU_UTIL{model="$model"}, gpu)
    includeAll: true

  - name: aggregation
    type: custom
    options: [All GPUs, Per GPU, Per Replica]
```

---

## Open Questions

1. **What inference engine is used?** (vLLM, TGI, Triton, custom) - affects available metrics
2. **Is DCGM exporter deployed?** - required for GPU metrics
3. **What are the SLA thresholds?** - for compliance panels
4. **Is cost tracking needed?** - requires GPU pricing configuration
5. **Which models are currently deployed?** - for model dropdown population

---

## References

- [vLLM Metrics](https://docs.vllm.ai/en/latest/serving/metrics.html)
- [DCGM Exporter](https://github.com/NVIDIA/dcgm-exporter)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
