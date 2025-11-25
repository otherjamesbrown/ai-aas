# AI-AAS Observability Guide

This guide provides comprehensive documentation for the observability stack deployed on the AI-AAS platform, including metrics collection, visualization, and performance testing capabilities.

## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Available Metrics](#available-metrics)
- [Accessing Grafana](#accessing-grafana)
- [Dashboard Reference](#dashboard-reference)
- [Querying Prometheus](#querying-prometheus)
- [Performance Testing Use Cases](#performance-testing-use-cases)
- [Troubleshooting](#troubleshooting)
- [Adding New Metrics](#adding-new-metrics)

## Overview

The AI-AAS platform uses a comprehensive observability stack built on:

- **Prometheus**: Time-series metrics collection and storage
- **Grafana**: Visualization and dashboarding
- **DCGM Exporter**: NVIDIA GPU hardware metrics
- **vLLM Metrics**: LLM inference performance metrics
- **kube-prometheus-stack**: Complete Kubernetes monitoring solution

### Key Capabilities

- Real-time GPU utilization and performance tracking
- Per-model inference performance metrics
- Per-GPU-type performance comparison
- Node and cluster-level infrastructure monitoring
- API-level request rate and latency monitoring
- Correlation between GPU hardware metrics and model performance

## Architecture

### Components

```
┌─────────────────────────────────────────────────────────────┐
│                     Grafana (Port 80)                       │
│                  Visualization Layer                        │
└────────────────────────┬────────────────────────────────────┘
                         │
                         ▼
┌─────────────────────────────────────────────────────────────┐
│              Prometheus (Port 9090)                         │
│           Metrics Collection & Storage                      │
└─────┬──────────────────┬──────────────────┬────────────────┘
      │                  │                  │
      ▼                  ▼                  ▼
┌──────────┐      ┌─────────────┐    ┌──────────────────┐
│  vLLM    │      │    DCGM     │    │   Kubernetes     │
│ Metrics  │      │  Exporter   │    │    Metrics       │
│ :8000    │      │  (GPU data) │    │  (node, pod)     │
└──────────┘      └─────────────┘    └──────────────────┘
```

### Metric Scraping Configuration

#### vLLM Metrics (PodMonitor)

Location: `infra/k8s/kserve/base/podmonitor-vllm.yaml`

- **Target**: KServe InferenceService pods running vLLM
- **Port**: 8000 (user-port on kserve-container)
- **Interval**: 30 seconds
- **Labels Added**: pod, node, inferenceservice, model_label, model_version, environment

#### DCGM Metrics (ServiceMonitor)

Managed by: GPU Operator (pre-installed)

- **Target**: DCGM exporter pods in gpu-operator namespace
- **Labels Available**: exported_pod (links to model pod), modelName (GPU type), node, gpu

## Available Metrics

### vLLM Inference Metrics

All vLLM metrics use the `vllm:` prefix format (with colon).

#### Throughput Metrics

| Metric | Description | Type |
|--------|-------------|------|
| `vllm:avg_generation_throughput_toks_per_s` | Average token generation throughput | Gauge |
| `vllm:avg_prompt_throughput_toks_per_s` | Average prompt processing throughput | Gauge |
| `vllm:prompt_tokens_total` | Total prompt tokens processed | Counter |
| `vllm:generation_tokens_total` | Total generation tokens produced | Counter |

#### Latency Metrics

| Metric | Description | Type |
|--------|-------------|------|
| `vllm:time_to_first_token_seconds` | Time to first token (p50, p95, p99) | Histogram |
| `vllm:e2e_request_latency_seconds` | End-to-end request latency | Histogram |

#### Resource Utilization

| Metric | Description | Type |
|--------|-------------|------|
| `vllm:gpu_cache_usage_perc` | GPU KV cache usage percentage | Gauge |
| `vllm:gpu_prefix_cache_hit_rate` | Prefix cache hit rate | Gauge |
| `vllm:num_requests_running` | Currently running requests | Gauge |
| `vllm:num_requests_waiting` | Requests waiting in queue | Gauge |
| `vllm:num_requests_swapped` | Requests swapped to CPU | Gauge |

#### Quality Metrics

| Metric | Description | Type |
|--------|-------------|------|
| `vllm:request_success_total` | Successful requests | Counter |
| `vllm:request_params_best_of` | Best-of sampling parameter | Histogram |
| `vllm:request_params_n` | Number of completions | Histogram |

### DCGM GPU Hardware Metrics

All DCGM metrics use the `DCGM_` prefix.

#### GPU Utilization

| Metric | Description | Unit |
|--------|-------------|------|
| `DCGM_FI_DEV_GPU_UTIL` | GPU utilization | Percent (0-100) |
| `DCGM_FI_DEV_MEM_COPY_UTIL` | Memory bandwidth utilization | Percent |
| `DCGM_FI_DEV_SM_CLOCK` | SM clock frequency | MHz |
| `DCGM_FI_DEV_MEM_CLOCK` | Memory clock frequency | MHz |

#### Memory Metrics

| Metric | Description | Unit |
|--------|-------------|------|
| `DCGM_FI_DEV_FB_USED` | GPU framebuffer memory used | MiB |
| `DCGM_FI_DEV_FB_FREE` | GPU framebuffer memory free | MiB |

#### Power & Thermal

| Metric | Description | Unit |
|--------|-------------|------|
| `DCGM_FI_DEV_POWER_USAGE` | GPU power consumption | Watts |
| `DCGM_FI_DEV_GPU_TEMP` | GPU temperature | Celsius |

#### Key Labels

- `exported_pod`: The model pod name (links to vLLM metrics)
- `exported_namespace`: Kubernetes namespace
- `modelName`: GPU hardware type (e.g., "NVIDIA-GeForce-RTX-4090")
- `gpu`: GPU index on the node
- `node`: Kubernetes node name
- `UUID`: GPU UUID
- `Hostname`: Node hostname

### Kubernetes Metrics

Standard metrics from kube-state-metrics and node-exporter:

- `kube_node_info`: Node inventory
- `node_cpu_seconds_total`: CPU usage
- `node_memory_MemAvailable_bytes`: Available memory
- `node_memory_MemTotal_bytes`: Total memory

## Accessing Grafana

### Development Environment

```bash
# Option 1: Port-forward to Grafana service
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml
kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80

# Access at: http://localhost:3000
# Username: admin
# Password: retrieve from secret
kubectl get secret -n monitoring kube-prometheus-stack-grafana -o jsonpath="{.data.admin-password}" | base64 -d
```

### Production Environment

Access via ingress URL (configure ingress first):

```bash
# Example: https://grafana.prod.ai-aas.local
```

## Dashboard Reference

All dashboards are stored as JSON in: `infra/k8s/monitoring/dashboards/`

### 1. Fleet Overview Dashboard

**File**: `fleet-overview.json`
**UID**: `ai-aas-fleet-overview`
**Purpose**: High-level executive view of entire AI fleet

#### Panels

- **Total Models Deployed**: Count of unique models running
- **GPU Types in Use**: Number of different GPU hardware types
- **Total Throughput**: Aggregate tokens/sec across all models
- **Average GPU Utilization**: Fleet-wide GPU usage
- **Generation Throughput by Model**: Timeseries of per-model throughput
- **GPU Utilization by Type**: Timeseries of GPU usage by hardware type

#### Use Cases

- Executive reporting
- Capacity planning
- SLA monitoring
- Fleet health checks

### 2. Per-Model Performance Dashboard

**File**: `per-model-performance.json`
**UID**: `ai-aas-per-model-performance`
**Purpose**: Detailed performance analysis for individual models

#### Template Variables

- `$model`: Select model from dropdown (populated from `vllm:prompt_tokens_total`)

#### Panels

- **Generation Throughput**: Current tokens/sec for selected model
- **Time to First Token (p95)**: Latency to first token
- **GPU Cache Usage**: KV cache utilization
- **TTFT Latency Distribution**: p50, p95, p99 percentiles over time
- **Throughput Over Time**: Generation vs prompt processing
- **Token Distribution**: Histogram of tokens per request
- **Request Queue Status**: Running, waiting, swapped requests
- **Request Success Rate**: Success vs total requests

#### Use Cases

- Model optimization
- Performance troubleshooting
- A/B testing between model versions
- Capacity planning per model

### 3. Per-GPU-Type Analysis Dashboard

**File**: `per-gpu-type-analysis.json`
**UID**: `ai-aas-per-gpu-type-analysis`
**Purpose**: Compare performance across different GPU hardware types

#### Template Variables

- `$gpu_type`: Select GPU type from dropdown (e.g., "NVIDIA-GeForce-RTX-4090")

#### Panels

- **Average GPU Utilization**: Current utilization for selected GPU type
- **Average Power Draw**: Power consumption in watts
- **Average GPU Temperature**: Thermal monitoring
- **GPU Memory Usage**: Framebuffer usage percentage
- **GPU Utilization Over Time**: Historical utilization by GPU
- **Power Consumption Over Time**: Historical power usage
- **Tokens/sec by Model on this GPU**: Correlation of vLLM performance with GPU type
- **Tokens per Watt Efficiency**: Performance-per-watt calculation
- **Models Running on this GPU Type**: Table of models deployed

#### Key Correlation Query

This dashboard uses a critical join query to correlate vLLM performance with GPU hardware:

```promql
sum by (model_name) (
  vllm:avg_generation_throughput_toks_per_s
  * on(pod) group_right()
  DCGM_FI_DEV_GPU_UTIL{modelName="$gpu_type"}
)
```

#### Use Cases

- GPU procurement decisions
- Multi-GPU-type fleet optimization
- Cost-per-performance analysis
- Thermal and power budget planning

### 4. Node & Cluster View Dashboard

**File**: `node-cluster-view.json`
**UID**: `ai-aas-node-cluster-view`
**Purpose**: Infrastructure-level monitoring and pod placement

#### Panels

- **Total Nodes**: Cluster node count
- **Total GPUs**: Aggregate GPU count
- **GPU Types**: Number of GPU types deployed
- **Active Models**: Currently running models
- **Node CPU Utilization**: Per-node CPU usage over time
- **Node Memory Usage**: Per-node memory usage over time
- **Node GPU Inventory**: Table showing GPUs per node and average utilization
- **GPU Type Distribution**: Count of GPUs by hardware type
- **Model Pod to Node Mapping**: Which models run on which nodes
- **GPU Memory Usage by Pod**: Framebuffer usage per pod/GPU
- **GPU Power Usage by Pod**: Power consumption per pod/GPU

#### Use Cases

- Capacity planning
- Node provisioning decisions
- Pod placement optimization
- Infrastructure cost analysis
- Identifying underutilized nodes

### 5. API Performance Dashboard

**File**: `api-performance.json`
**UID**: `ai-aas-api-performance`
**Purpose**: API-level request monitoring and latency analysis

#### Panels

- **Request Rate (RPS)**: Current requests per second across all services
- **P95 Latency**: 95th percentile response time (milliseconds)
- **Success Rate**: Percentage of successful requests (2xx status codes)
- **Concurrent Requests**: Number of active requests being processed
- **Request Rate Over Time**: Timeseries graph of requests per second
- **Request Latency Percentiles**: p50, p95, p99 latency trends
- **Requests by Status Code**: Breakdown of 2xx, 4xx, 5xx responses over time
- **Concurrent Requests Over Time**: Concurrency trends
- **Service Performance Summary**: Table showing RPS, P95 Latency, and Success Rate per service

#### Data Source

This dashboard uses **Knative Serving metrics** (not Istio) since KServe InferenceServices are deployed in Serverless mode:

- `revision_app_request_count`: Request counts with status code labels
- `revision_app_request_latencies_bucket`: Request latency histograms
- `activator_request_concurrency`: Concurrent request tracking

#### Template Variables

- `$namespace`: Select namespace (default: development)

#### Use Cases

- API gateway performance monitoring
- Request rate and latency tracking during load tests
- Identifying error rate spikes (4xx, 5xx responses)
- Understanding system response characteristics at the API level
- Troubleshooting slow response times
- Monitoring autoscaling behavior via concurrency metrics

## Querying Prometheus

### Direct Access

```bash
# Port-forward to Prometheus
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090

# Access UI at: http://localhost:9090
```

### Common Query Patterns

#### List All Models

```promql
count by (model_name) (vllm:prompt_tokens_total)
```

#### GPU Utilization for Specific Model

```promql
DCGM_FI_DEV_GPU_UTIL{exported_pod=~"mistral-7b-instruct.*"}
```

#### Total Throughput Across Fleet

```promql
sum(vllm:avg_generation_throughput_toks_per_s)
```

#### Correlation: Throughput per GPU Type

```promql
sum by (model_name, modelName) (
  vllm:avg_generation_throughput_toks_per_s
  * on(pod) group_right()
  DCGM_FI_DEV_GPU_UTIL
)
```

#### Tokens per Watt Efficiency

```promql
sum by (model_name) (
  vllm:avg_generation_throughput_toks_per_s
  * on(pod) group_right()
  DCGM_FI_DEV_GPU_UTIL
) / ignoring(model_name) group_left() sum by (exported_pod) (DCGM_FI_DEV_POWER_USAGE)
```

#### Models on Specific Node

```promql
count by (model_name, node) (vllm:prompt_tokens_total{node="lke531921-776664-51386eeb0000"})
```

### API Queries

#### Get Available Metrics

```bash
curl -s http://localhost:9090/api/v1/label/__name__/values | jq -r '.data[]' | grep vllm
```

#### Get Metric Labels

```bash
curl -s 'http://localhost:9090/api/v1/series?match[]=vllm:prompt_tokens_total' | jq '.data[0]'
```

#### Query Current Values

```bash
curl -s 'http://localhost:9090/api/v1/query?query=vllm:avg_generation_throughput_toks_per_s' | jq
```

## Performance Testing Use Cases

### 1. GPU Engagement Tracking

**Question**: How many GPUs are actively engaged in inference?

**Query**:
```promql
count(DCGM_FI_DEV_GPU_UTIL > 0)
```

**Dashboard**: Node & Cluster View → Total GPUs panel

### 2. Performance Per GPU

**Question**: What's the throughput for each GPU?

**Query**:
```promql
sum by (exported_pod, gpu, node) (
  vllm:avg_generation_throughput_toks_per_s
  * on(pod) group_right()
  DCGM_FI_DEV_GPU_UTIL
)
```

**Dashboard**: Per-GPU-Type Analysis → Tokens/sec by Model panel

### 3. Tokens Per Second

**Question**: What's the current tokens/sec for a model?

**Query**:
```promql
vllm:avg_generation_throughput_toks_per_s{model_name="mistral-7b-instruct"}
```

**Dashboard**: Per-Model Performance → Generation Throughput panel

### 4. Time to First Token (TTFT)

**Question**: What's the p95 TTFT latency?

**Query**:
```promql
histogram_quantile(0.95,
  rate(vllm:time_to_first_token_seconds_bucket{model_name="mistral-7b-instruct"}[5m])
)
```

**Dashboard**: Per-Model Performance → Time to First Token (p95) panel

### 5. KV Cache Hit Rates

**Question**: What's the cache efficiency?

**Query**:
```promql
vllm:gpu_prefix_cache_hit_rate{model_name="mistral-7b-instruct"}
```

**Dashboard**: Per-Model Performance → GPU Cache Usage panel

### 6. Summary Per Model

**Question**: Show all metrics for one model

**Dashboard**: Per-Model Performance (select model from dropdown)

**Includes**:
- Throughput (tokens/sec)
- Latency (TTFT p50/p95/p99)
- Cache usage
- Request queue depth
- Success rate

### 7. Summary Per GPU Type

**Question**: Compare performance across GPU types

**Dashboard**: Per-GPU-Type Analysis (select GPU type from dropdown)

**Includes**:
- GPU utilization
- Power consumption
- Temperature
- Models running on this GPU type
- Tokens-per-watt efficiency

### 8. Multi-GPU-Type Comparison

**Query Example**: Compare RTX 4090 vs A100

```promql
# RTX 4090 performance
sum by (model_name) (
  vllm:avg_generation_throughput_toks_per_s
  * on(pod) group_right()
  DCGM_FI_DEV_GPU_UTIL{modelName="NVIDIA-GeForce-RTX-4090"}
)

# A100 performance
sum by (model_name) (
  vllm:avg_generation_throughput_toks_per_s
  * on(pod) group_right()
  DCGM_FI_DEV_GPU_UTIL{modelName="NVIDIA-A100-SXM4-80GB"}
)
```

**Dashboard**: Per-GPU-Type Analysis (switch between GPU types using dropdown)

## Troubleshooting

### No Metrics Appearing in Grafana

#### Check 1: Verify Prometheus is Scraping

```bash
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml
kubectl port-forward -n monitoring svc/kube-prometheus-stack-prometheus 9090:9090

# Check targets status
# Visit: http://localhost:9090/targets
# Look for: monitoring/kserve-vllm-metrics/* (should be UP)
```

#### Check 2: Verify PodMonitor is Discovered

```bash
kubectl get podmonitor -n development kserve-vllm-metrics -o yaml

# Verify labels include:
# release: kube-prometheus-stack
```

#### Check 3: Test Metrics Endpoint Directly

```bash
export KUBECONFIG=secrets/kubeconfigs/kubeconfig-development.yaml

# Get pod name
POD_NAME=$(kubectl get pods -n development -l app=vllm-inference -o jsonpath='{.items[0].metadata.name}')

# Port-forward and check metrics
kubectl port-forward -n development $POD_NAME 8000:8000 &
curl -s http://localhost:8000/metrics | grep vllm
```

### Missing DCGM Metrics

#### Check GPU Operator Status

```bash
kubectl get pods -n gpu-operator

# Verify dcgm-exporter pods are running
kubectl logs -n gpu-operator -l app=nvidia-dcgm-exporter
```

#### Query Prometheus for DCGM Metrics

```bash
curl -s http://localhost:9090/api/v1/label/__name__/values | jq -r '.data[]' | grep DCGM
```

### Dashboard Shows "No Data"

#### Verify Time Range

- Check Grafana dashboard time range (default: last 1 hour)
- Ensure models have been actively serving requests in that timeframe

#### Verify Template Variables

- For Per-Model dashboard: ensure model name exists in `vllm:prompt_tokens_total`
- For Per-GPU-Type dashboard: ensure GPU type exists in `DCGM_FI_DEV_GPU_UTIL`

#### Check Metric Label Matching

vLLM and DCGM metrics are joined on the `pod` label:

```promql
# Verify pod labels exist on both sides
vllm:avg_generation_throughput_toks_per_s
DCGM_FI_DEV_GPU_UTIL
```

### High Cardinality Issues

If Prometheus complains about high cardinality:

1. Check number of unique metric series:
   ```promql
   count({__name__=~"vllm.*"})
   ```

2. Review metricRelabelings in PodMonitor to drop unnecessary labels

3. Consider increasing Prometheus retention or storage

## Adding New Metrics

### Adding Custom vLLM Metrics

vLLM automatically exposes all its metrics. No additional configuration needed unless you want to:

1. Filter specific metrics via PodMonitor `metricRelabelings`
2. Add custom labels via `relabelings`

Edit: `infra/k8s/kserve/base/podmonitor-vllm.yaml`

### Adding Additional GPU Metrics

DCGM exports 30+ metrics by default. To expose more:

1. Check available fields: https://docs.nvidia.com/datacenter/dcgm/latest/dcgm-api/dcgm-api-field-ids.html

2. Edit GPU Operator DCGM exporter configuration (if needed)

### Creating New Dashboards

#### Option 1: Grafana UI (Recommended for Development)

1. Access Grafana UI
2. Create dashboard interactively
3. Save and export JSON
4. Place in `infra/k8s/monitoring/dashboards/`
5. Commit to git

#### Option 2: Direct JSON Creation

1. Copy existing dashboard JSON as template
2. Modify panels and queries
3. Test with Grafana API or import
4. Commit to git

#### Dashboard as Code Best Practices

- Set `"id": null` (Grafana assigns on import)
- Use meaningful UID: `"uid": "ai-aas-my-dashboard"`
- Include descriptive tags: `"tags": ["ai-aas", "category"]`
- Use template variables for dynamic filtering
- Document panel purposes in title and description

### Deploying Dashboard Changes

All dashboards are managed via GitOps:

```bash
# 1. Edit dashboard JSON
vim infra/k8s/monitoring/dashboards/my-dashboard.json

# 2. Commit changes
git add infra/k8s/monitoring/dashboards/
git commit -m "feat: Add new performance dashboard"

# 3. Push to trigger ArgoCD sync
git push origin develop

# 4. Verify in Grafana UI
# Dashboards should auto-reload (may require Grafana restart)
```

## Best Practices

### Query Performance

1. **Use recording rules** for frequently-used complex queries
2. **Limit time ranges** when querying high-resolution data
3. **Use rate() for counters** instead of raw values
4. **Aggregate before joining** to reduce series cardinality

### Dashboard Design

1. **Start with overview panels** (stats) before detailed timeseries
2. **Use consistent color schemes** across related panels
3. **Include both current values and trends** for key metrics
4. **Add descriptions** to complex panels explaining the metric

### Monitoring Strategy

1. **Fleet → Model → GPU** hierarchy for troubleshooting
2. **Monitor both resource (GPU) and application (vLLM) metrics**
3. **Set up alerts** for critical thresholds (not covered in this guide)
4. **Regularly review dashboards** during performance testing

## Reference Links

- [Prometheus Querying Documentation](https://prometheus.io/docs/prometheus/latest/querying/basics/)
- [Grafana Dashboard Documentation](https://grafana.com/docs/grafana/latest/dashboards/)
- [vLLM Metrics Documentation](https://docs.vllm.ai/en/latest/serving/metrics.html)
- [DCGM Exporter Documentation](https://docs.nvidia.com/datacenter/cloud-native/gpu-telemetry/dcgm-exporter.html)
- [kube-prometheus-stack Helm Chart](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)

## Support

For issues or questions:

1. Check Prometheus targets: `http://localhost:9090/targets`
2. Review pod logs: `kubectl logs -n development <pod-name>`
3. Verify metrics endpoint: `curl http://<pod-ip>:8000/metrics`
4. Consult troubleshooting section above

---

**Last Updated**: 2025-11-25
**Version**: 1.0
**Maintained By**: AI-AAS Platform Team
