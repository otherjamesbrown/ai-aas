# Metrics Catalog

> **Single source of truth** for all Prometheus metrics available on the AI-AAS platform.

## Quick Reference

| Source | Prefix | ServiceMonitor | Primary Use |
|--------|--------|----------------|-------------|
| [vLLM](#vllm-inference-metrics) | `vllm:` | `monitoring/vllm-models` | Model inference performance |
| [DCGM](#dcgm-gpu-metrics) | `DCGM_` | `gpu-operator/nvidia-dcgm-exporter` | GPU hardware utilization |
| [API Router](#api-router-metrics) | `api_router_` | `staging/api-router-service-*` | Request routing & latency |
| [Analytics](#analytics-service-metrics) | `analytics_` | `analytics-service/analytics-service-*` | Usage rollups |
| [User/Org](#user-org-service-metrics) | `user_org_service_` | `user-org-service/*` | Auth & API keys |
| [GuideLLM](#guidellm-runner-metrics) | `guidellm_` | `staging/guidellm-runner` | Benchmark results |
| [Kubernetes](#kubernetes-infrastructure-metrics) | `kube_`, `node_` | kube-prometheus-stack | Infrastructure |

## How to Query

```bash
# Port-forward to Prometheus (staging)
kubectl --kubeconfig=secrets/kubeconfigs/kubeconfig-staging.yaml \
  port-forward -n monitoring svc/kube-prometheus-stack-stag-prometheus 9090:9090

# List all metrics with a prefix
curl -s "http://localhost:9090/api/v1/label/__name__/values" | jq -r '.data[]' | grep "^vllm:"

# Query specific metric
curl -s "http://localhost:9090/api/v1/query?query=vllm:request_success_total" | jq '.data.result'

# Check if metric exists (returns count)
curl -s "http://localhost:9090/api/v1/query?query=vllm:request_success_total" | jq '.data.result | length'
```

---

## vLLM Inference Metrics

**Source**: vLLM model pods
**ServiceMonitor**: `monitoring/vllm-models`
**Scrape Path**: `/metrics` on port 8000
**Documentation**: [vllm-observability.md](../platform/vllm-observability.md)

### Request Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `vllm:request_success_total` | Counter | Successful requests by `finished_reason` (stop, length, abort) |
| `vllm:num_requests_running` | Gauge | Currently executing requests |
| `vllm:num_requests_waiting` | Gauge | Requests queued waiting for execution |

**Key Labels**: `model_name`, `finished_reason`

### Latency Metrics (Histograms)

| Metric | Description | Key Percentiles |
|--------|-------------|-----------------|
| `vllm:time_to_first_token_seconds` | Time until first token generated | P50, P95, P99 |
| `vllm:inter_token_latency_seconds` | Time between tokens (streaming) | P50, P95, P99 |
| `vllm:e2e_request_latency_seconds` | Total request duration | P50, P95, P99 |
| `vllm:request_queue_time_seconds` | Time spent waiting in queue | P50, P95 |
| `vllm:request_prefill_time_seconds` | Prompt processing time | P50, P95 |
| `vllm:request_decode_time_seconds` | Token generation time | P50, P95 |

**Usage**:
```promql
# P95 Time to First Token
histogram_quantile(0.95, sum(rate(vllm:time_to_first_token_seconds_bucket[5m])) by (le, model_name))
```

### Throughput Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `vllm:prompt_tokens_total` | Counter | Total prompt tokens processed |
| `vllm:generation_tokens_total` | Counter | Total tokens generated |

**Usage**:
```promql
# Tokens per second (generation)
sum(rate(vllm:generation_tokens_total[5m])) by (model_name)
```

### Cache Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `vllm:gpu_cache_usage_perc` | Gauge | GPU KV cache utilization (0-1) |
| `vllm:kv_cache_usage_perc` | Gauge | KV cache utilization (0-1) |
| `vllm:gpu_prefix_cache_hits_total` | Counter | Prefix cache hits |
| `vllm:gpu_prefix_cache_queries_total` | Counter | Prefix cache queries |

**Usage**:
```promql
# Prefix cache hit rate
sum(rate(vllm:gpu_prefix_cache_hits_total[5m])) / sum(rate(vllm:gpu_prefix_cache_queries_total[5m]))
```

---

## DCGM GPU Metrics

**Source**: NVIDIA DCGM Exporter
**ServiceMonitor**: `gpu-operator/nvidia-dcgm-exporter`
**Namespace**: `gpu-operator`

### Utilization

| Metric | Unit | Description |
|--------|------|-------------|
| `DCGM_FI_DEV_GPU_UTIL` | % (0-100) | GPU compute utilization |
| `DCGM_FI_DEV_MEM_COPY_UTIL` | % (0-100) | Memory bandwidth utilization |
| `DCGM_FI_DEV_ENC_UTIL` | % (0-100) | Encoder utilization |
| `DCGM_FI_DEV_DEC_UTIL` | % (0-100) | Decoder utilization |

### Memory

| Metric | Unit | Description |
|--------|------|-------------|
| `DCGM_FI_DEV_FB_USED` | MiB | Framebuffer memory used |
| `DCGM_FI_DEV_FB_FREE` | MiB | Framebuffer memory free |
| `DCGM_FI_DEV_FB_RESERVED` | MiB | Framebuffer memory reserved |

### Power & Thermal

| Metric | Unit | Description |
|--------|------|-------------|
| `DCGM_FI_DEV_POWER_USAGE` | Watts | Current power draw |
| `DCGM_FI_DEV_GPU_TEMP` | Celsius | GPU temperature |
| `DCGM_FI_DEV_MEMORY_TEMP` | Celsius | Memory temperature |
| `DCGM_FI_DEV_TOTAL_ENERGY_CONSUMPTION` | mJ | Total energy consumed |

### Clocks

| Metric | Unit | Description |
|--------|------|-------------|
| `DCGM_FI_DEV_SM_CLOCK` | MHz | Streaming multiprocessor clock |
| `DCGM_FI_DEV_MEM_CLOCK` | MHz | Memory clock |

### PCIe

| Metric | Unit | Description |
|--------|------|-------------|
| `DCGM_FI_PROF_PCIE_TX_BYTES` | Bytes | PCIe transmit bandwidth |
| `DCGM_FI_PROF_PCIE_RX_BYTES` | Bytes | PCIe receive bandwidth |
| `DCGM_FI_DEV_PCIE_REPLAY_COUNTER` | Count | PCIe replay errors |

**Key Labels**: `gpu` (index), `UUID`, `modelName` (GPU type), `Hostname`, `exported_pod`, `exported_namespace`

**Usage**:
```promql
# Average GPU utilization by model
avg(DCGM_FI_DEV_GPU_UTIL) by (exported_pod)

# GPU memory usage percentage
DCGM_FI_DEV_FB_USED / (DCGM_FI_DEV_FB_USED + DCGM_FI_DEV_FB_FREE) * 100
```

---

## API Router Metrics

**Source**: api-router-service
**ServiceMonitor**: `staging/api-router-service-staging-api-router-service`
**Scrape Path**: `/metrics` on port 8080

| Metric | Type | Description |
|--------|------|-------------|
| `api_router_backend_requests_total` | Counter | Total requests to backends |
| `api_router_backend_request_duration_seconds` | Histogram | Backend request latency |
| `api_router_quota_denials_total` | Counter | Requests denied due to quota |

**Key Labels**: `backend`, `method`, `status_code`, `org_id`

**Usage**:
```promql
# Request rate by backend
sum(rate(api_router_backend_requests_total[5m])) by (backend)

# P95 latency by backend
histogram_quantile(0.95, sum(rate(api_router_backend_request_duration_seconds_bucket[5m])) by (le, backend))

# Error rate
sum(rate(api_router_backend_requests_total{status_code=~"5.."}[5m])) / sum(rate(api_router_backend_requests_total[5m]))
```

---

## Analytics Service Metrics

**Source**: analytics-service
**ServiceMonitor**: `analytics-service/analytics-service-staging-analytics-service`

| Metric | Type | Description |
|--------|------|-------------|
| `analytics_rollup_successes_total` | Counter | Successful rollup operations |
| `analytics_rollup_last_success_timestamp` | Gauge | Last successful rollup time |

---

## User/Org Service Metrics

**Source**: user-org-service
**ServiceMonitor**: `user-org-service/*`

| Metric | Type | Description |
|--------|------|-------------|
| `user_org_service_apikeys_issued_total` | Counter | API keys created |
| `user_org_service_apikeys_revoked_total` | Counter | API keys revoked |
| `user_org_service_auth_sessions_created_total` | Counter | Auth sessions created |
| `user_org_service_auth_sessions_revoked_total` | Counter | Auth sessions revoked |

---

## GuideLLM Runner Metrics

**Source**: guidellm-runner
**ServiceMonitor**: `staging/guidellm-runner`
**Scrape Path**: `/metrics` on port 8080

### Benchmark Counts

| Metric | Type | Description |
|--------|------|-------------|
| `guidellm_benchmark_runs_total` | Counter | Total benchmark executions |
| `guidellm_last_benchmark_timestamp` | Gauge | Unix timestamp of last run |

### Request Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `guidellm_requests_total` | Counter | Total inference requests |
| `guidellm_requests_successful_total` | Counter | Successful requests |
| `guidellm_requests_failed_total` | Counter | Failed requests |
| `guidellm_requests_per_second` | Gauge | Current request rate |

### Token Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `guidellm_prompt_tokens_total` | Counter | Total prompt tokens |
| `guidellm_output_tokens_total` | Counter | Total output tokens |
| `guidellm_output_tokens_per_second` | Gauge | Token generation rate |

### Latency

| Metric | Type | Description |
|--------|------|-------------|
| `guidellm_e2e_latency_seconds` | Histogram | End-to-end request latency |

**Key Labels**: `model`, `target`, `environment`

**Usage**:
```promql
# P95 benchmark latency
histogram_quantile(0.95, sum(rate(guidellm_e2e_latency_seconds_bucket[5m])) by (le, model))

# Success rate
sum(rate(guidellm_requests_successful_total[5m])) / sum(rate(guidellm_requests_total[5m]))
```

---

## Kubernetes Infrastructure Metrics

**Source**: kube-prometheus-stack (kube-state-metrics, node-exporter)

### Node Metrics

| Metric | Description |
|--------|-------------|
| `node_cpu_seconds_total` | CPU time by mode |
| `node_memory_MemTotal_bytes` | Total memory |
| `node_memory_MemAvailable_bytes` | Available memory |
| `node_filesystem_avail_bytes` | Available disk space |
| `node_network_receive_bytes_total` | Network RX |
| `node_network_transmit_bytes_total` | Network TX |

### Pod/Container Metrics

| Metric | Description |
|--------|-------------|
| `kube_pod_status_phase` | Pod phase (Running, Pending, etc.) |
| `kube_pod_container_status_restarts_total` | Container restarts |
| `container_cpu_usage_seconds_total` | Container CPU usage |
| `container_memory_usage_bytes` | Container memory usage |

### Kubernetes Resources

| Metric | Description |
|--------|-------------|
| `kube_deployment_status_replicas_ready` | Ready replicas |
| `kube_deployment_status_replicas_unavailable` | Unavailable replicas |
| `kube_node_status_condition` | Node conditions |

---

## Adding New Metrics

### For Go Services

1. Add Prometheus client to your service
2. Register metrics in `internal/metrics/metrics.go`
3. Ensure ServiceMonitor exists for your service
4. Document metrics in this catalog

```go
import "github.com/prometheus/client_golang/prometheus"

var requestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "myservice_requests_total",
        Help: "Total requests processed",
    },
    []string{"method", "status"},
)
```

### For New Services

1. Add `/metrics` endpoint exposing Prometheus format
2. Create ServiceMonitor in `infra/k8s/monitoring/servicemonitors/`
3. Verify scraping: check Prometheus targets
4. Add metrics to this catalog

### ServiceMonitor Template

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-service
  namespace: <service-namespace>
  labels:
    release: kube-prometheus-stack  # Required for discovery
spec:
  selector:
    matchLabels:
      app: my-service  # Must match Service labels
  endpoints:
    - port: metrics    # Must match Service port name
      path: /metrics
      interval: 15s
  namespaceSelector:
    matchNames:
      - <service-namespace>
```

---

## Troubleshooting

### Metric Not Found

```bash
# 1. Check if service is being scraped
curl -s "http://localhost:9090/api/v1/targets" | jq '.data.activeTargets[] | select(.labels.job=="<service>") | {health, lastError}'

# 2. Check ServiceMonitor exists
kubectl get servicemonitor -A | grep <service>

# 3. Check ServiceMonitor labels match Service
kubectl get svc <service> -n <ns> -o yaml | grep -A5 "labels:"
kubectl get servicemonitor <sm> -n <ns> -o yaml | grep -A5 "matchLabels:"

# 4. Check network policy allows scraping
kubectl get networkpolicy -n <ns> -o yaml | grep -A10 "ports:"
```

### Metric Exists But No Data

```bash
# Check if metric has any series
curl -s "http://localhost:9090/api/v1/query?query=<metric>" | jq '.data.result | length'

# Check label values
curl -s "http://localhost:9090/api/v1/label/<label>/values" | jq '.data'

# Check time range (metric might be stale)
curl -s "http://localhost:9090/api/v1/query?query=<metric>" | jq '.data.result[0].value'
```

---

## Related Documentation

| Document | Purpose |
|----------|---------|
| [Dashboard Spec Schema](../../dashboards/specs/SCHEMA.md) | How to specify dashboards |
| [vLLM Observability Guide](../platform/vllm-observability.md) | Detailed vLLM metrics |
| [Observability Architecture](../architecture/observability-architecture.md) | Stack architecture |
| [Observability Developer Context](../../context/observability-developer/agents.md) | Agent workflow |
