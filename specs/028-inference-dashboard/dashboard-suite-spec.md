# Observability Dashboard Suite Specification

## Overview

A comprehensive set of Grafana dashboards providing visibility into the AI-as-a-Service platform. The dashboards are organized into three tiers based on use case: Operational (daily monitoring), Business (usage and costs), and Infrastructure (debugging and capacity).

**This spec replaces the existing dashboard suite.** The following dashboards will be removed:

| Dashboard to Remove | Replacement |
|---------------------|-------------|
| `fleet-overview.json` | Platform Overview + GPU Fleet |
| `per-model-performance.json` | Inference Performance |
| `per-gpu-type-analysis.json` | GPU Fleet |
| `api-performance.json` | API Performance |
| `inference-backends.json` | Inference Engine |
| `service-logs.json` | (Integrated into each dashboard as logs panels) |
| `request-tracing.json` | (Keep - tracing is orthogonal) |
| `node-cluster-view.json` | Kubernetes Resources |

**Migration**: Remove old dashboards after new ones are deployed and validated.

---

## Clarifications

### Session 2024-12-16

- **Q:** How do we know each dashboard is "done"?
  **A:** Query validation - All PromQL/LogQL queries must return data. Dashboards are complete when every panel query executes without error and returns meaningful data.

- **Q:** What should dashboards show when metrics are missing or unavailable?
  **A:** Show warning panel - Add a status/health panel at the top of each dashboard that highlights missing data sources (e.g., "DCGM Exporter: No data", "vLLM: 0 pods"). This makes it obvious what's missing without breaking the layout.

- **Q:** How should users navigate between dashboards?
  **A:** Drill-down links only - Users click on panels to navigate to related dashboards (e.g., click GPU utilization stat → GPU Fleet dashboard). Keeps dashboards clean with contextual navigation.

- **Q:** Should we standardize metric prefixes across dashboards?
  **A:** Keep as-is - Use actual metric names from each source (`vllm:*`, `api_router_*`, `DCGM_*`). Recording rules for aliasing add maintenance burden.

- **Q:** Should alerting configuration be part of this spec?
  **A:** Dashboards only - Alerts are a separate concern. Existing alerts in `platform-alerts.yaml` are sufficient. Keep this spec focused on dashboards.

---

## Non-Functional Requirements

| Requirement | Target | Notes |
|-------------|--------|-------|
| Default refresh rate | 10 seconds | Configurable per dashboard |
| Dashboard load time | < 3 seconds | With default time range |
| Query timeout | 30 seconds | Grafana default |
| Default time range | Last 1 hour | Adjustable via time picker |
| Max time range | 7 days | Prevent expensive queries |
| Panel data points | < 1000 per series | Use `$__interval` for aggregation |

---

## Acceptance Criteria

A dashboard is **complete** when:

1. **Queries execute** - All PromQL/LogQL queries return data without errors
2. **Data source status** - Health/status panel shows data source availability
3. **Drill-down links work** - Clicking relevant panels navigates to the correct dashboard
4. **Thresholds configured** - Color thresholds match spec (green/yellow/red)
5. **Variables work** - Dropdowns populate and filter correctly
6. **Refresh works** - Auto-refresh at 10s interval without errors

### Per-Dashboard Acceptance

| Dashboard | Key Validation Queries |
|-----------|------------------------|
| Platform Overview | `up{}`, `DCGM_FI_DEV_GPU_UTIL`, `api_router_backend_requests_total` |
| Inference Performance | `vllm:avg_generation_throughput_toks_per_s`, `vllm:time_to_first_token_seconds` |
| API Performance | `api_router_backend_requests_total`, `api_router_backend_request_duration_seconds` |
| Org Usage | `api_router_tokens_total` (requires Task 1) |
| Cost & Efficiency | `api_router_tokens_total`, `DCGM_FI_DEV_GPU_UTIL` (requires Task 1+2) |
| GPU Fleet | `DCGM_FI_DEV_GPU_UTIL`, `DCGM_FI_DEV_FB_USED`, `DCGM_FI_DEV_GPU_TEMP` |
| Kubernetes Resources | `kube_pod_status_phase`, `kube_node_status_condition` |
| Inference Engine | `vllm_num_requests_running`, `vllm_gpu_cache_usage_perc` |

---

## Design Principles

1. **Answer one question per dashboard**: Each dashboard should have a clear purpose
2. **Progressive disclosure**: Start with overview, drill down into details
3. **Consistent layout**: Same visual patterns across all dashboards
4. **Actionable metrics**: Every panel should inform a decision or action

---

## Dashboard Inventory

| # | Dashboard | Purpose | Primary Audience |
|---|-----------|---------|------------------|
| 1 | Platform Overview | "Is everything working?" | Everyone |
| 2 | Inference Performance | "How are models performing?" | ML Engineers |
| 3 | API Performance | "How is the API performing?" | Backend Engineers |
| 4 | Org Usage & Analytics | "How are customers using the platform?" | Product/Business |
| 5 | Cost & Efficiency | "What does it cost to serve requests?" | Finance/Ops |
| 6 | GPU Fleet | "What's the state of our GPUs?" | Infrastructure |
| 7 | Kubernetes Resources | "What's the state of our cluster?" | Infrastructure |
| 8 | Inference Engine | "Why is inference slow?" | Debugging |

---

## Dashboard 1: Platform Overview

**Purpose**: Single pane of glass - know if everything is OK in 10 seconds

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Health Score]  [API Success%]  [Inference Success%]  [GPU Util]│
├─────────────────────────────────┬───────────────────────────────┤
│ Request Rate (24h)              │ Error Rate (24h)              │
│ [Time series graph]             │ [Time series graph]           │
├─────────────────────────────────┼───────────────────────────────┤
│ Inference Latency (P50/P99)     │ Active Models                 │
│ [Time series graph]             │ [Table: model, status, QPS]   │
├─────────────────────────────────┴───────────────────────────────┤
│ System Status                                                    │
│ [Service status tiles: API Router ✓, Admin API ✓, vLLM ✓, ...]  │
└─────────────────────────────────────────────────────────────────┘
```

### Panels

| Panel | Type | Metric/Query | Thresholds |
|-------|------|--------------|------------|
| Health Score | Stat | Composite of success rates | Green >99%, Yellow >95%, Red <95% |
| API Success Rate | Stat | `sum(rate(http_requests_total{status=~"2.."}[5m])) / sum(rate(http_requests_total[5m]))` | Green >99.5%, Yellow >99%, Red <99% |
| Inference Success Rate | Stat | `1 - (rate(inference_errors_total[5m]) / rate(inference_requests_total[5m]))` | Green >99%, Yellow >95%, Red <95% |
| GPU Utilization | Gauge | `avg(DCGM_FI_DEV_GPU_UTIL)` | Green <80%, Yellow <95%, Red >95% |
| Request Rate | Time series | `sum(rate(http_requests_total[1m]))` | - |
| Error Rate | Time series | `sum(rate(http_requests_total{status=~"5.."}[1m]))` | - |
| Inference Latency | Time series | P50 and P99 of `inference_request_duration_seconds` | - |
| Active Models | Table | Model name, status, requests/sec | - |
| Service Status | Stat repeat | `up{job=~"api-router|admin-api|vllm|..."}` | 1=green, 0=red |

### Links
- Click API metrics → API Performance dashboard
- Click Inference metrics → Inference Performance dashboard
- Click GPU metrics → GPU Fleet dashboard
- Click Model row → Inference Performance (filtered to model)

---

## Dashboard 2: Inference Performance

**Purpose**: Deep dive into model-level inference metrics

### Variables

| Variable | Type | Query |
|----------|------|-------|
| `model` | Dropdown | `label_values(inference_requests_total, model)` |

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Model: ▼ llama-70b    ]                                        │
├────────────┬────────────┬────────────┬────────────┬─────────────┤
│ Status     │ QPS        │ TTFT P50   │ Tokens/sec │ Error Rate  │
│ [Running]  │ [123.4]    │ [45ms]     │ [1,234]    │ [0.1%]      │
├────────────┴────────────┴────────────┴────────────┴─────────────┤
│ Throughput                                                       │
├─────────────────────────────────┬───────────────────────────────┤
│ Requests/sec                    │ Tokens/sec (Input vs Output)  │
│ [Time series]                   │ [Time series - stacked]       │
├─────────────────────────────────┴───────────────────────────────┤
│ Latency                                                          │
├─────────────────────────────────┬───────────────────────────────┤
│ Time to First Token (TTFT)      │ End-to-End Latency            │
│ [Time series: P50, P95, P99]    │ [Heatmap]                     │
├─────────────────────────────────┼───────────────────────────────┤
│ Inter-Token Latency             │ Latency by Input Length       │
│ [Time series]                   │ [Scatter plot]                │
├─────────────────────────────────┴───────────────────────────────┤
│ Caching                                                          │
├─────────────────────────────────┬───────────────────────────────┤
│ KV Cache Utilization            │ KV Cache Hit Rate             │
│ [Gauge + time series]           │ [Time series]                 │
├─────────────────────────────────┼───────────────────────────────┤
│ Batch Size Distribution         │ Queue Depth                   │
│ [Histogram]                     │ [Time series]                 │
├─────────────────────────────────┴───────────────────────────────┤
│ Errors                                                           │
├─────────────────────────────────┬───────────────────────────────┤
│ Errors by Type                  │ Recent Errors (Logs)          │
│ [Pie chart]                     │ [Loki logs panel]             │
└─────────────────────────────────┴───────────────────────────────┘
```

### Key Metrics

| Metric | Description | Source |
|--------|-------------|--------|
| `inference_time_to_first_token_seconds` | Time until first token generated | vLLM/custom |
| `inference_generation_tokens_total` | Output tokens generated | vLLM/custom |
| `inference_prompt_tokens_total` | Input tokens processed | vLLM/custom |
| `vllm_cache_usage_ratio` | KV cache utilization | vLLM |
| `vllm_num_requests_running` | Active requests | vLLM |
| `vllm_num_requests_waiting` | Queued requests | vLLM |

---

## Dashboard 3: API Performance

**Purpose**: Monitor the user-facing API layer (api-router-service)

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Success Rate]  [Requests/sec]  [P50 Latency]  [P99 Latency]    │
├─────────────────────────────────┬───────────────────────────────┤
│ Request Rate by Endpoint        │ Latency Percentiles           │
│ [Time series - stacked]         │ [Time series: P50, P95, P99]  │
├─────────────────────────────────┼───────────────────────────────┤
│ Error Rate                      │ Errors by Status Code         │
│ [Time series]                   │ [Bar chart: 4xx, 5xx, ...]    │
├─────────────────────────────────┼───────────────────────────────┤
│ Request Rate by Org             │ Latency by Org                │
│ [Time series - top 10]          │ [Time series - top 10]        │
├─────────────────────────────────┼───────────────────────────────┤
│ Requests by Model               │ Backend Response Time         │
│ [Pie chart]                     │ [Time series per backend]     │
├─────────────────────────────────┴───────────────────────────────┤
│ Request Details                                                  │
│ [Table: timestamp, org, endpoint, status, latency, model]       │
└─────────────────────────────────────────────────────────────────┘
```

### Key Metrics

| Metric | Description | Labels |
|--------|-------------|--------|
| `http_requests_total` | Request count | method, path, status, org_id |
| `http_request_duration_seconds` | Request latency | method, path, org_id |
| `http_request_size_bytes` | Request body size | method, path |
| `http_response_size_bytes` | Response body size | method, path |

---

## Dashboard 4: Org Usage & Analytics

**Purpose**: Understand how organizations are using the platform

### Variables

| Variable | Type | Query |
|----------|------|-------|
| `org` | Dropdown (multi) | `label_values(http_requests_total, org_id)` |

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Org: ▼ All / org-123  ]                                        │
├────────────┬────────────┬────────────┬────────────┬─────────────┤
│ Total Orgs │ Active Orgs│ Total Req  │ Total      │ Avg Tokens/ │
│ [42]       │ [38]       │ [1.2M]     │ Tokens[5B] │ Request[120]│
├────────────┴────────────┴────────────┴────────────┴─────────────┤
│ Usage Over Time                                                  │
├─────────────────────────────────┬───────────────────────────────┤
│ Requests by Org (Top 10)        │ Tokens by Org (Top 10)        │
│ [Time series - stacked]         │ [Time series - stacked]       │
├─────────────────────────────────┼───────────────────────────────┤
│ Org Leaderboard                 │ Model Usage by Org            │
│ [Table: org, requests, tokens]  │ [Heatmap: org × model]        │
├─────────────────────────────────┴───────────────────────────────┤
│ Usage Patterns                                                   │
├─────────────────────────────────┬───────────────────────────────┤
│ Requests by Hour of Day         │ Active Users per Org          │
│ [Heatmap: hour × day]           │ [Time series]                 │
├─────────────────────────────────┼───────────────────────────────┤
│ Avg Request Size by Org         │ Error Rate by Org             │
│ [Bar chart]                     │ [Bar chart]                   │
├─────────────────────────────────┴───────────────────────────────┤
│ Rate Limiting (Future)                                           │
├─────────────────────────────────┬───────────────────────────────┤
│ Rate Limit Hits by Org          │ Throttled Requests            │
│ [Time series]                   │ [Table: org, count, %]        │
└─────────────────────────────────┴───────────────────────────────┘
```

### Key Metrics

| Metric | Description | Labels |
|--------|-------------|--------|
| `api_tokens_total` | Tokens processed | org_id, model, direction (input/output) |
| `api_requests_total` | Request count | org_id, model, status |
| `api_active_users` | Unique users (gauge) | org_id |
| `api_rate_limit_hits_total` | Rate limit events | org_id, limit_type |

### Data Requirements

These metrics require instrumentation in the api-router-service:
- Token counting per request (input + output)
- Org ID attached to all request metrics
- User ID tracking for unique user counts

---

## Dashboard 5: Cost & Efficiency

**Purpose**: Track infrastructure costs and map to per-token costs

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Total Cost]  [Cost/1K Tokens] [GPU Hours]   [Efficiency %]     │
│ [$1,234/day]  [$0.0012]        [96 hrs]      [78%]              │
├─────────────────────────────────┬───────────────────────────────┤
│ Daily Cost Trend                │ Cost Breakdown                │
│ [Time series: GPU, Compute,     │ [Pie: GPU, CPU, Storage,      │
│  Storage, Network]              │  Network, Other]              │
├─────────────────────────────────┼───────────────────────────────┤
│ Cost per Model                  │ Tokens per GPU-Hour           │
│ [Bar chart]                     │ [Bar chart by model]          │
├─────────────────────────────────┼───────────────────────────────┤
│ Cost per Org                    │ GPU Idle Time                 │
│ [Table: org, tokens, cost]      │ [Gauge + trend]               │
├─────────────────────────────────┴───────────────────────────────┤
│ Efficiency Metrics                                               │
├─────────────────────────────────┬───────────────────────────────┤
│ GPU Utilization vs Cost         │ Throughput per Watt           │
│ [Scatter: util% vs $/hour]      │ [Time series by GPU]          │
├─────────────────────────────────┼───────────────────────────────┤
│ Batch Efficiency                │ Cache Efficiency              │
│ [Actual vs optimal batch size]  │ [Cache hits saved compute]    │
└─────────────────────────────────┴───────────────────────────────┘
```

### Cost Calculation

```
Cost per Token = (GPU Cost per Hour × GPU Hours) / Total Tokens

Where:
- GPU Cost per Hour: Configured per GPU type (e.g., A100=$3.50/hr, H100=$5.00/hr)
- GPU Hours: Sum of (GPU utilization × time) across all GPUs
- Total Tokens: Sum of input + output tokens processed
```

### Configuration Required

```yaml
# Cost configuration (to be stored in ConfigMap or database)
gpu_costs:
  nvidia-a100-80gb: 3.50  # $/hour
  nvidia-h100-80gb: 5.00  # $/hour
  nvidia-l40s: 1.50       # $/hour

other_costs:
  cpu_per_core_hour: 0.05
  memory_per_gb_hour: 0.01
  storage_per_gb_month: 0.10
  network_egress_per_gb: 0.05
```

### Key Metrics

| Metric | Description | Calculation |
|--------|-------------|-------------|
| `platform_cost_usd` | Running cost counter | Recording rule from GPU hours × cost |
| `platform_tokens_per_gpu_hour` | Efficiency metric | tokens / GPU-hours |
| `platform_cost_per_1k_tokens` | Unit cost | (cost / tokens) × 1000 |

---

## Dashboard 6: GPU Fleet

**Purpose**: Monitor GPU inventory, health, and utilization

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Total GPUs]  [Healthy]   [Avg Util%]  [Avg Mem%]  [Avg Temp]   │
│ [8]           [8/8]       [72%]        [85%]       [62°C]       │
├─────────────────────────────────────────────────────────────────┤
│ GPU Inventory                                                    │
│ [Table: GPU, Node, Model Serving, Util%, Mem%, Temp, Power]     │
├─────────────────────────────────┬───────────────────────────────┤
│ GPU Utilization Over Time       │ Memory Utilization Over Time  │
│ [Time series per GPU]           │ [Time series per GPU]         │
├─────────────────────────────────┼───────────────────────────────┤
│ GPU Utilization Heatmap         │ Memory Utilization Heatmap    │
│ [Heatmap: GPU × time]           │ [Heatmap: GPU × time]         │
├─────────────────────────────────┴───────────────────────────────┤
│ GPU Health                                                       │
├─────────────────────────────────┬───────────────────────────────┤
│ Temperature                     │ Power Draw                    │
│ [Time series with threshold]    │ [Time series with limit]      │
├─────────────────────────────────┼───────────────────────────────┤
│ Thermal Throttling Events       │ ECC Errors                    │
│ [Stat + time series]            │ [Stat + time series]          │
├─────────────────────────────────┴───────────────────────────────┤
│ GPU to Model Mapping                                             │
│ [Table: GPU, Node, Assigned Model, Since, Requests Served]      │
└─────────────────────────────────────────────────────────────────┘
```

### Key Metrics (from DCGM Exporter)

| Metric | Description |
|--------|-------------|
| `DCGM_FI_DEV_GPU_UTIL` | GPU compute utilization % |
| `DCGM_FI_DEV_MEM_COPY_UTIL` | Memory bandwidth utilization % |
| `DCGM_FI_DEV_FB_USED` | Framebuffer memory used (bytes) |
| `DCGM_FI_DEV_FB_FREE` | Framebuffer memory free (bytes) |
| `DCGM_FI_DEV_GPU_TEMP` | GPU temperature (°C) |
| `DCGM_FI_DEV_POWER_USAGE` | Power consumption (W) |
| `DCGM_FI_DEV_THERMAL_VIOLATION` | Thermal throttling events |
| `DCGM_FI_DEV_ECC_SBE_VOL_TOTAL` | Single-bit ECC errors |
| `DCGM_FI_DEV_ECC_DBE_VOL_TOTAL` | Double-bit ECC errors |

---

## Dashboard 7: Kubernetes Resources

**Purpose**: Monitor cluster health and resource allocation

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Nodes]    [Pods Running] [Pods Pending] [CPU Used%] [Mem Used%]│
│ [3/3]      [45]           [0]            [65%]       [78%]      │
├─────────────────────────────────────────────────────────────────┤
│ Node Status                                                      │
│ [Table: Node, Status, CPU, Memory, Pods, GPUs, Conditions]      │
├─────────────────────────────────┬───────────────────────────────┤
│ Pod Status by Namespace         │ Pod Restarts (24h)            │
│ [Bar chart: Running/Pending/    │ [Table: Pod, Restarts, Last]  │
│  Failed by namespace]           │                               │
├─────────────────────────────────┼───────────────────────────────┤
│ CPU Usage by Namespace          │ Memory Usage by Namespace     │
│ [Time series - stacked]         │ [Time series - stacked]       │
├─────────────────────────────────┼───────────────────────────────┤
│ Resource Requests vs Limits     │ Resource Actual vs Requests   │
│ [Bar chart comparison]          │ [Scatter: request vs actual]  │
├─────────────────────────────────┴───────────────────────────────┤
│ Workload Health                                                  │
│ [Table: Deployment, Replicas, Available, Ready, Age]            │
├─────────────────────────────────────────────────────────────────┤
│ Recent Events                                                    │
│ [Table: Time, Type, Reason, Object, Message]                    │
└─────────────────────────────────────────────────────────────────┘
```

### Key Metrics (from kube-state-metrics)

| Metric | Description |
|--------|-------------|
| `kube_node_status_condition` | Node health conditions |
| `kube_pod_status_phase` | Pod phase (Running, Pending, etc.) |
| `kube_pod_container_status_restarts_total` | Container restart count |
| `kube_deployment_status_replicas_available` | Available replicas |
| `container_cpu_usage_seconds_total` | Container CPU usage |
| `container_memory_usage_bytes` | Container memory usage |

---

## Dashboard 8: Inference Engine (vLLM/Triton)

**Purpose**: Debug inference performance issues with engine-level metrics

### Variables

| Variable | Type | Query |
|----------|------|-------|
| `engine` | Dropdown | `label_values(up{job=~"vllm.*"}, job)` |

### Layout

```
┌─────────────────────────────────────────────────────────────────┐
│ [Engine: ▼ vllm-llama-70b  ]                                    │
├────────────┬────────────┬────────────┬────────────┬─────────────┤
│ Status     │ Running Req│ Waiting Req│ GPU Blocks │ CPU Blocks  │
│ [Up]       │ [12]       │ [3]        │ [85%]      │ [45%]       │
├────────────┴────────────┴────────────┴────────────┴─────────────┤
│ Request Processing                                               │
├─────────────────────────────────┬───────────────────────────────┤
│ Running vs Waiting Requests     │ Request Processing Time       │
│ [Time series: running, waiting] │ [Time series: prefill, decode]│
├─────────────────────────────────┼───────────────────────────────┤
│ Batch Size Over Time            │ Sequence Length Distribution  │
│ [Time series]                   │ [Histogram]                   │
├─────────────────────────────────┴───────────────────────────────┤
│ Memory Management                                                │
├─────────────────────────────────┬───────────────────────────────┤
│ GPU Cache Blocks (Used/Total)   │ CPU Cache Blocks (Used/Total) │
│ [Gauge + time series]           │ [Gauge + time series]         │
├─────────────────────────────────┼───────────────────────────────┤
│ Cache Hit Rate                  │ Cache Evictions               │
│ [Time series]                   │ [Time series]                 │
├─────────────────────────────────┴───────────────────────────────┤
│ Performance Breakdown                                            │
├─────────────────────────────────┬───────────────────────────────┤
│ Time in Prefill vs Decode       │ Tokens Generated per Batch    │
│ [Stacked area chart]            │ [Time series]                 │
├─────────────────────────────────┼───────────────────────────────┤
│ Scheduling Overhead             │ Model Forward Pass Time       │
│ [Time series]                   │ [Time series]                 │
├─────────────────────────────────┴───────────────────────────────┤
│ Errors & Issues                                                  │
├─────────────────────────────────┬───────────────────────────────┤
│ CUDA Errors                     │ OOM Events                    │
│ [Stat + time series]            │ [Stat + time series]          │
├─────────────────────────────────┴───────────────────────────────┤
│ Engine Logs                                                      │
│ [Loki logs panel with error highlighting]                       │
└─────────────────────────────────────────────────────────────────┘
```

### Key Metrics (vLLM-specific)

| Metric | Description |
|--------|-------------|
| `vllm_num_requests_running` | Currently processing requests |
| `vllm_num_requests_waiting` | Queued requests |
| `vllm_gpu_cache_usage_perc` | GPU KV cache utilization |
| `vllm_cpu_cache_usage_perc` | CPU KV cache utilization |
| `vllm_num_preemptions_total` | Request preemption count |
| `vllm_prompt_tokens_total` | Input tokens processed |
| `vllm_generation_tokens_total` | Output tokens generated |
| `vllm_request_success_total` | Successful requests |
| `vllm_request_failure_total` | Failed requests |

---

## Data Source Requirements

### Required Exporters

| Exporter | Purpose | Deployment | Status |
|----------|---------|------------|--------|
| kube-state-metrics | Kubernetes object metrics | DaemonSet | ✅ Deployed |
| node-exporter | Node-level metrics | DaemonSet | ✅ Deployed |
| DCGM Exporter | GPU metrics | DaemonSet on GPU nodes | ✅ Deployed (via GPU Operator) |
| vLLM built-in | Inference metrics | Part of vLLM deployment | ✅ Enabled, scraped via PodMonitor |

### Current Instrumentation Status

| Service | Metrics Available | Status |
|---------|-------------------|--------|
| api-router-service | Request metrics with `organization_id` label | ✅ Available |
| api-router-service | Token count metrics | ❌ **Missing - needs implementation** |
| admin-api-service | Request metrics | ✅ Available |
| vLLM deployments | Throughput, latency, cache metrics | ✅ Available |
| DCGM Exporter | GPU utilization, memory, temperature | ✅ Available |

### Loki Labels (Verified)

Currently available labels in Loki:

```yaml
labels:
  service: "api-router-service"  # ✅ Available
  namespace: "ai-aas"            # ✅ Available
  pod: "${POD_NAME}"             # ✅ Available
  container: "${CONTAINER}"      # ✅ Available
  node: "${NODE}"                # ✅ Available
  app: "${APP}"                  # ✅ Available
```

**Note**: `org_id` and `level` are not currently indexed as labels. They are available in log content via JSON parsing.

---

## Required Instrumentation Work

### Task 1: Token Count Prometheus Metrics (REQUIRED)

**Why**: The Org Usage & Analytics and Cost & Efficiency dashboards require token counts as Prometheus metrics. Currently, tokens are tracked in usage records (published to Kafka) but NOT exposed as Prometheus metrics.

**Location**: `services/api-router-service/internal/telemetry/exporters.go`

**Current State**:
- Token counting IS implemented in `internal/usage/record.go` (lines 35-36)
- Tokens ARE extracted from OpenAI responses in `internal/api/public/openai.go`
- Tokens are NOT exposed as Prometheus metrics

**Required Changes**:

Add the following metrics to `exporters.go`:

```go
// TokensProcessedTotal tracks total tokens processed with direction label.
// This metric enables org usage analytics and cost calculation dashboards.
var TokensProcessedTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "api_router_tokens_total",
        Help: "Total number of tokens processed",
    },
    []string{"organization_id", "model", "direction"}, // direction: "input", "output"
)

// TokensPerRequest tracks token distribution per request for capacity planning.
var TokensPerRequest = promauto.NewHistogramVec(
    prometheus.HistogramOpts{
        Name:    "api_router_tokens_per_request",
        Help:    "Distribution of tokens per request",
        Buckets: []float64{10, 50, 100, 250, 500, 1000, 2000, 4000, 8000, 16000, 32000},
    },
    []string{"organization_id", "model", "direction"},
)
```

**Helper function to add**:

```go
// RecordTokens records token metrics for a request.
func RecordTokens(organizationID, model string, inputTokens, outputTokens int) {
    TokensProcessedTotal.WithLabelValues(organizationID, model, "input").Add(float64(inputTokens))
    TokensProcessedTotal.WithLabelValues(organizationID, model, "output").Add(float64(outputTokens))

    if inputTokens > 0 {
        TokensPerRequest.WithLabelValues(organizationID, model, "input").Observe(float64(inputTokens))
    }
    if outputTokens > 0 {
        TokensPerRequest.WithLabelValues(organizationID, model, "output").Observe(float64(outputTokens))
    }
}
```

**Integration point**: Call `RecordTokens()` from `internal/api/public/openai.go` after extracting token counts from the response (around line 195 for chat completions, line 308 for text completions).

**Example PromQL queries enabled by this metric**:

```promql
# Total tokens by org (last 24h)
sum by (organization_id) (increase(api_router_tokens_total[24h]))

# Token rate by model
sum by (model) (rate(api_router_tokens_total[5m]))

# Input/Output ratio by org
sum by (organization_id) (rate(api_router_tokens_total{direction="output"}[1h]))
/
sum by (organization_id) (rate(api_router_tokens_total{direction="input"}[1h]))

# Cost calculation (with recording rule)
sum by (organization_id) (
  increase(api_router_tokens_total{direction="input"}[24h]) * 0.00001 +
  increase(api_router_tokens_total{direction="output"}[24h]) * 0.00003
)
```

---

### Task 2: Cost Configuration (REQUIRED for Cost Dashboard)

**Why**: To calculate cost per token, we need GPU cost configuration.

**Recommended approach**: ConfigMap in the `monitoring` namespace.

**File**: `infra/k8s/monitoring/cost-config.yaml`

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: platform-cost-config
  namespace: monitoring
data:
  costs.yaml: |
    # GPU costs ($/hour)
    gpu_costs:
      nvidia-a100-80gb: 3.50
      nvidia-a100-40gb: 2.80
      nvidia-h100-80gb: 5.00
      nvidia-l40s: 1.50
      nvidia-rtx-4000-ada: 0.80

    # Overhead costs ($/hour)
    overhead:
      cpu_per_core: 0.05
      memory_per_gb: 0.01

    # Token pricing (for reference/validation)
    token_pricing:
      default_input_per_1k: 0.01
      default_output_per_1k: 0.03
```

**Recording rules for cost metrics** (add to `infra/k8s/prometheus-rules/development/platform-alerts.yaml`):

```yaml
groups:
  - name: cost_metrics
    interval: 1m
    rules:
      # GPU hours by model (from DCGM utilization)
      - record: platform:gpu_hours:1h
        expr: |
          sum by (model_name) (
            avg_over_time(DCGM_FI_DEV_GPU_UTIL[1h]) / 100
          )

      # Estimated cost per 1K tokens (simplified)
      - record: platform:cost_per_1k_tokens:1h
        expr: |
          (sum(increase(api_router_tokens_total[1h])) > 0)
          and
          (
            # Placeholder: actual cost calculation requires GPU cost config
            3.50 * sum(avg_over_time(DCGM_FI_DEV_GPU_UTIL[1h])) / 100
          ) / (sum(increase(api_router_tokens_total[1h])) / 1000)
```

---

### Task 3: Active Users Metric (OPTIONAL - for Org Dashboard)

**Why**: To show unique active users per organization.

**Location**: `services/api-router-service/internal/telemetry/exporters.go`

```go
// ActiveUsersGauge tracks unique active users per organization.
// Reset periodically (e.g., hourly) via a background goroutine.
var ActiveUsersGauge = promauto.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "api_router_active_users",
        Help: "Number of unique active users per organization (rolling window)",
    },
    []string{"organization_id"},
)
```

**Note**: This requires tracking unique user IDs in a time window, which adds complexity. Consider implementing this in Phase 3.

---

### Task 4: Rate Limit Metrics (FUTURE - when rate limiting is implemented)

**Placeholder metrics for future rate limiting feature**:

```go
// RateLimitHitsTotal tracks rate limit violations.
var RateLimitHitsTotal = promauto.NewCounterVec(
    prometheus.CounterOpts{
        Name: "api_router_rate_limit_hits_total",
        Help: "Total number of rate limit hits",
    },
    []string{"organization_id", "limit_type"}, // limit_type: "requests_per_minute", "tokens_per_day"
)

// RateLimitRemainingGauge tracks remaining quota.
var RateLimitRemainingGauge = promauto.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "api_router_rate_limit_remaining",
        Help: "Remaining rate limit quota",
    },
    []string{"organization_id", "limit_type"},
)
```

---

## Gap Analysis Summary

| Dashboard | Data Status | Blockers |
|-----------|-------------|----------|
| Platform Overview | ✅ Ready | None |
| Inference Performance | ✅ Ready | None |
| API Performance | ✅ Ready | None |
| Org Usage & Analytics | ⚠️ Partial | **Task 1: Token metrics required** |
| Cost & Efficiency | ❌ Blocked | **Task 1 + Task 2 required** |
| GPU Fleet | ✅ Ready | None |
| Kubernetes Resources | ✅ Ready | None |
| Inference Engine | ✅ Ready | None |

---

## Implementation Priority

### Phase 1: Foundation (Start Here)
1. **Platform Overview** - Get visibility immediately
2. **GPU Fleet** - Understand hardware state
3. **Kubernetes Resources** - Basic cluster health

### Phase 2: Performance
4. **Inference Performance** - Model-level metrics
5. **API Performance** - User-facing metrics
6. **Inference Engine** - Debugging capability

### Phase 3: Business (Requires Instrumentation Work First)
7. **Implement Task 1** - Add token count Prometheus metrics to api-router-service
8. **Implement Task 2** - Create cost configuration ConfigMap
9. **Org Usage & Analytics** - Build dashboard after Task 1 complete
10. **Cost & Efficiency** - Build dashboard after Tasks 1 & 2 complete

---

## Resolved Questions

| Question | Answer | Evidence |
|----------|--------|----------|
| Token counting implemented? | ✅ Yes (internal) | `usage/record.go:35-36`, `openai.go:179-195` |
| Token metrics exposed? | ❌ No | Not in `exporters.go` - **Task 1 required** |
| DCGM Exporter deployed? | ✅ Yes | GPU Operator + PodMonitor in `kserve/monitoring/` |
| vLLM metrics scraped? | ✅ Yes | PodMonitor at `podmonitor-vllm.yaml`, port 8000 |
| org_id in metrics? | ✅ Yes | All metrics in `exporters.go` have `organization_id` label |
| Cost data source? | ConfigMap | Recommended: `infra/k8s/monitoring/cost-config.yaml` |

---

## Development & Debugging Access

### API Endpoints for Dashboard Development

Claude Code can query these endpoints directly for testing and debugging:

| Service | Endpoint | Purpose |
|---------|----------|---------|
| Loki | `https://loki.dev.otherjamesbrown.com/loki/api/v1/query` | Test LogQL queries |
| Loki | `https://loki.dev.otherjamesbrown.com/loki/api/v1/labels` | List available labels |
| Grafana | `https://grafana.dev.otherjamesbrown.com/api/datasources` | List datasources |
| Grafana | `https://grafana.dev.otherjamesbrown.com/api/search` | List dashboards |
| Grafana | `https://grafana.dev.otherjamesbrown.com/api/dashboards/uid/{uid}` | Get dashboard JSON |

### Example Debug Commands

```bash
# Test Loki query
curl -G "https://loki.dev.otherjamesbrown.com/loki/api/v1/query" \
  --data-urlencode 'query={service="api-router-service"} |= "error"' \
  --data-urlencode 'limit=10'

# List Loki labels
curl "https://loki.dev.otherjamesbrown.com/loki/api/v1/labels"

# Test Prometheus query via Grafana proxy
curl "https://grafana.dev.otherjamesbrown.com/api/datasources/proxy/uid/prometheus/api/v1/query" \
  --data-urlencode 'query=up'

# Get dashboard by UID
curl "https://grafana.dev.otherjamesbrown.com/api/dashboards/uid/api-performance"

# List all dashboards
curl "https://grafana.dev.otherjamesbrown.com/api/search?type=dash-db"
```

### Dashboard JSON Location

Dashboard JSON files are in the repository:

```
infra/k8s/monitoring/dashboards/

# TO BE REMOVED (replaced by this spec):
├── api-performance.json        → replaced by: api-performance-v2.json
├── fleet-overview.json         → replaced by: platform-overview.json + gpu-fleet.json
├── inference-backends.json     → replaced by: inference-engine.json
├── node-cluster-view.json      → replaced by: kubernetes-resources.json
├── per-gpu-type-analysis.json  → replaced by: gpu-fleet.json
├── per-model-performance.json  → replaced by: inference-performance.json
├── service-logs.json           → (logs integrated into dashboards)

# TO KEEP:
├── request-tracing.json        → (keep - tracing is orthogonal)

# NEW DASHBOARDS (this spec):
├── platform-overview.json      → NEW
├── inference-performance.json  → NEW
├── api-performance-v2.json     → NEW (replaces api-performance.json)
├── org-usage.json              → NEW (requires Task 1)
├── cost-efficiency.json        → NEW (requires Task 1 + 2)
├── gpu-fleet.json              → NEW
├── kubernetes-resources.json   → NEW
└── inference-engine.json       → NEW
```

Dashboards are deployed via ConfigMap with Grafana's dashboard provider auto-discovery.

---

## References

- [vLLM Metrics Documentation](https://docs.vllm.ai/en/latest/serving/metrics.html)
- [DCGM Exporter Metrics](https://github.com/NVIDIA/dcgm-exporter)
- [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics)
- [Grafana Dashboard Best Practices](https://grafana.com/docs/grafana/latest/dashboards/build-dashboards/best-practices/)
