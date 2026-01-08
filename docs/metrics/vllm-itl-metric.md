# vLLM Inter-Token Latency (ITL) Metric

## Overview

The **Inter-Token Latency (ITL)** metric measures the time between successive tokens during LLM generation. This is critical for understanding streaming performance and user-perceived latency during interactive chat sessions.

## Metric Details

### Metric Name
```
vllm:inter_token_latency_seconds
```

### Type
**Histogram** - Provides distribution of inter-token latencies across percentiles

### Labels
- `engine`: vLLM engine ID (typically "0")
- `model_name`: Model identifier (e.g., "llama-3-1-8b-instruct-vllm")

### Histogram Buckets
```
le="0.01", le="0.025", le="0.05", le="0.075", le="0.1",
le="0.15", le="0.2", le="0.3", le="0.4", le="0.5",
le="0.75", le="1.0", le="2.5", le="5.0", le="7.5",
le="10.0", le="20.0", le="40.0", le="80.0", le="+Inf"
```

### Related Metrics
- `vllm:time_per_output_token_seconds` - **DEPRECATED** (replaced by inter_token_latency_seconds)
- `vllm:time_to_first_token_seconds` - TTFT metric (time until first token)
- `vllm:e2e_request_latency_seconds` - Total request latency

## Availability

### Automatic Exposure
- **No configuration required** - vLLM automatically exposes this metric on port 8000
- Available at: `http://<pod-ip>:8000/metrics`
- Scraped by Prometheus via PodMonitor: `kserve-vllm-metrics`

### Deployment Requirements
- vLLM version: 0.2.0+ (metric introduced to replace deprecated time_per_output_token_seconds)
- No environment variables or flags needed
- Works with all vLLM serving modes (OpenAI API, Triton)

## Querying Examples

### Basic Queries

#### Current ITL (Average)
```promql
vllm:inter_token_latency_seconds_sum / vllm:inter_token_latency_seconds_count
```

#### ITL p50 (Median)
```promql
histogram_quantile(0.50,
  rate(vllm:inter_token_latency_seconds_bucket{model_name="llama-3-1-8b-instruct-vllm"}[5m])
)
```

#### ITL p95
```promql
histogram_quantile(0.95,
  rate(vllm:inter_token_latency_seconds_bucket{model_name="llama-3-1-8b-instruct-vllm"}[5m])
)
```

#### ITL p99
```promql
histogram_quantile(0.99,
  rate(vllm:inter_token_latency_seconds_bucket{model_name="llama-3-1-8b-instruct-vllm"}[5m])
)
```

### Advanced Queries

#### ITL Across All Models
```promql
histogram_quantile(0.95,
  sum by (model_name, le) (rate(vllm:inter_token_latency_seconds_bucket[5m]))
)
```

#### ITL by GPU Type (Correlation)
```promql
histogram_quantile(0.95,
  sum by (model_name, le) (
    rate(vllm:inter_token_latency_seconds_bucket[5m])
    * on(pod) group_left(modelName)
    DCGM_FI_DEV_GPU_UTIL
  )
)
```

#### Tokens per Second (Inverse of ITL)
```promql
1 / (
  vllm:inter_token_latency_seconds_sum / vllm:inter_token_latency_seconds_count
)
```

## Grafana Dashboard Usage

### Panel: Inter-Token Latency (ITL) - Percentiles

**Type**: Time Series Graph

**Queries**:
```promql
# p50
histogram_quantile(0.50, rate(vllm:inter_token_latency_seconds_bucket{model_name="$model"}[5m]))

# p95
histogram_quantile(0.95, rate(vllm:inter_token_latency_seconds_bucket{model_name="$model"}[5m]))

# p99
histogram_quantile(0.99, rate(vllm:inter_token_latency_seconds_bucket{model_name="$model"}[5m]))
```

**Legend**:
- `p50 (median)`
- `p95`
- `p99`

**Y-Axis**:
- Unit: seconds (s)
- Min: 0

### Panel: ITL - Current Value (Stat)

**Type**: Stat Panel

**Query**:
```promql
histogram_quantile(0.95,
  rate(vllm:inter_token_latency_seconds_bucket{model_name="$model"}[5m])
)
```

**Display**:
- Unit: seconds (s)
- Thresholds:
  - Green: < 0.05s (< 50ms)
  - Yellow: 0.05s - 0.1s
  - Red: > 0.1s

### Panel: ITL Distribution Heatmap

**Type**: Heatmap

**Query**:
```promql
sum by (le) (
  increase(vllm:inter_token_latency_seconds_bucket{model_name="$model"}[$__interval])
)
```

**Configuration**:
- Data format: Time series buckets
- Y-Axis: Latency (seconds)
- Color scheme: Blue-Green-Yellow-Red

## Interpretation

### What Does ITL Tell You?

- **Low ITL (< 50ms)**: Fast streaming, good user experience
- **Medium ITL (50-100ms)**: Acceptable for most use cases
- **High ITL (> 100ms)**: Slow streaming, noticeable delays

### Factors Affecting ITL

1. **GPU Performance**: Faster GPUs = lower ITL
2. **Model Size**: Larger models = higher ITL
3. **Batch Size**: Higher batching = higher ITL per request
4. **KV Cache Pressure**: Cache eviction = higher ITL
5. **Quantization**: FP8 typically faster than BF16

### ITL vs TTFT

| Metric | What It Measures | When It Matters |
|--------|------------------|-----------------|
| **TTFT** | Time until first token | User perceived "start" latency |
| **ITL** | Time between tokens | Streaming smoothness |

**Example**:
- TTFT = 200ms (acceptable)
- ITL = 20ms (smooth streaming at ~50 tokens/sec)

## Benchmark Integration

### GuideLLM Compatibility

GuideLLM benchmark tool tracks ITL automatically when testing vLLM backends:

```yaml
# In benchmark target
backend_type: openai
endpoint: https://api.dev.otherjamesbrown.com/v1/chat/completions

# GuideLLM measures:
# - time_to_first_token (TTFT)
# - inter_token_latency (ITL)
# - tokens_per_second (1/ITL)
```

### Recommended Scenarios

For ITL benchmarking, use scenarios with:
- **Long output lengths** (e.g., 200-500 tokens) to capture sustained generation
- **Streaming enabled** to measure real-time latency
- **Varied batch sizes** to understand ITL under load

Example scenario:
```yaml
scenario:
  type: standard
  max_tokens: 300
  temperature: 0.7
  stream: true
```

## Monitoring & Alerting

### Recommended Alerts

#### High ITL (p95)
```yaml
alert: HighInterTokenLatency
expr: |
  histogram_quantile(0.95,
    rate(vllm:inter_token_latency_seconds_bucket[5m])
  ) > 0.15
for: 5m
severity: warning
message: "High ITL p95: {{ $value }}s on {{ $labels.model_name }}"
```

#### ITL Spike
```yaml
alert: ITLSpike
expr: |
  (
    histogram_quantile(0.95,
      rate(vllm:inter_token_latency_seconds_bucket[5m])
    )
    /
    histogram_quantile(0.95,
      rate(vllm:inter_token_latency_seconds_bucket[1h] offset 1h)
    )
  ) > 2
for: 5m
severity: critical
message: "ITL doubled: {{ $value }}x on {{ $labels.model_name }}"
```

## Troubleshooting

### ITL is High

**Check**:
1. GPU utilization: `DCGM_FI_DEV_GPU_UTIL` - should be < 90%
2. KV cache usage: `vllm:gpu_cache_usage_perc` - should be < 80%
3. Request queue depth: `vllm:num_requests_running` - should be < max_batch_size
4. GPU memory: `DCGM_FI_DEV_FB_USED` - should not be at limit

**Solutions**:
- Reduce `max_batch_size` to improve per-request latency
- Increase `gpu_memory_utilization` for larger KV cache
- Scale to more replicas to reduce queue depth
- Use quantization (FP8) for faster generation

### ITL Metric Missing

**Check**:
1. vLLM version: Must be 0.2.0+
2. PodMonitor: `kubectl get podmonitor -n development kserve-vllm-metrics`
3. Prometheus targets: `http://localhost:9090/targets` - look for vLLM pods
4. Metrics endpoint: `kubectl exec -n development <pod> -- curl http://localhost:8000/metrics | grep inter_token`

### Deprecated Metric Still in Use

If you see `vllm:time_per_output_token_seconds` in queries:

**Replace with**:
```promql
# Old (deprecated)
histogram_quantile(0.95, rate(vllm:time_per_output_token_seconds_bucket[5m]))

# New (correct)
histogram_quantile(0.95, rate(vllm:inter_token_latency_seconds_bucket[5m]))
```

## References

- [vLLM Metrics Documentation](https://docs.vllm.ai/en/latest/serving/metrics.html)
- [Prometheus Histogram Queries](https://prometheus.io/docs/practices/histograms/)
- [AI-AAS vLLM Observability Guide](../platform/vllm-observability.md)

---

**Last Updated**: 2026-01-08
**Bead**: aas-r9402
**Author**: model-manager agent
