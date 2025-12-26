# Quickstart: Observability Dashboard Suite

## Accessing Dashboards

**Grafana URL**: https://grafana.dev.otherjamesbrown.com

Dashboards are accessible without login (anonymous access enabled).

---

## Dashboard Navigation

| Dashboard | Purpose | When to Use |
|-----------|---------|-------------|
| **Platform Overview** | Is everything working? | First stop, daily check |
| **Inference Performance** | How are models performing? | Investigating model issues |
| **API Performance** | How is the API layer? | Debugging request failures |
| **GPU Fleet** | GPU health and utilization | Capacity planning, hardware issues |
| **Kubernetes Resources** | Cluster health | Pod failures, resource issues |
| **Inference Engine** | vLLM internals | Deep debugging inference |
| **Org Usage** | Customer usage patterns | Business metrics |
| **Cost & Efficiency** | Cost per token | Financial planning |

---

## Testing Queries

### Via Grafana Explore

1. Go to https://grafana.dev.otherjamesbrown.com/explore
2. Select data source (Prometheus or Loki)
3. Enter query and run

### Via API

```bash
# Test Prometheus query
curl -G "https://grafana.dev.otherjamesbrown.com/api/datasources/proxy/uid/prometheus/api/v1/query" \
  --data-urlencode 'query=up{}'

# Test Loki query
curl -G "https://loki.dev.otherjamesbrown.com/loki/api/v1/query" \
  --data-urlencode 'query={service="api-router-service"}'
```

---

## Editing Dashboards

### Option 1: Edit JSON Directly

```bash
# Edit dashboard file
vim infra/k8s/monitoring/dashboards/platform-overview.json

# Commit and push
git add -A && git commit -m "Update platform overview dashboard"
git push

# ArgoCD will sync automatically
```

### Option 2: Export from Grafana UI

1. Edit dashboard in Grafana UI
2. Settings → JSON Model → Copy
3. Paste into JSON file in repo
4. Commit and push

---

## Validating Dashboards

### Check All Queries Return Data

```bash
# Key metrics that must return data
curl -s "https://grafana.dev.otherjamesbrown.com/api/datasources/proxy/uid/prometheus/api/v1/query" \
  --data-urlencode 'query=DCGM_FI_DEV_GPU_UTIL' | jq '.data.result | length'

curl -s "https://grafana.dev.otherjamesbrown.com/api/datasources/proxy/uid/prometheus/api/v1/query" \
  --data-urlencode 'query=api_router_backend_requests_total' | jq '.data.result | length'
```

### Check Dashboard Loads

```bash
# Should return dashboard JSON
curl -s "https://grafana.dev.otherjamesbrown.com/api/dashboards/uid/platform-overview" | jq '.dashboard.title'
```

---

## Common Issues

| Issue | Solution |
|-------|----------|
| "No data" on panels | Check data source status panel, verify metrics exist |
| Dashboard not updating | Check ArgoCD sync status, verify ConfigMap deployed |
| Slow dashboard load | Reduce time range, check query complexity |
| Missing GPU metrics | Verify DCGM exporter pods are running |
| Missing vLLM metrics | Verify vLLM pods have `/metrics` endpoint |

---

## Key Metrics Reference

| Metric | Source | Description |
|--------|--------|-------------|
| `up{}` | Prometheus | Target availability |
| `DCGM_FI_DEV_GPU_UTIL` | DCGM Exporter | GPU utilization % |
| `DCGM_FI_DEV_FB_USED` | DCGM Exporter | GPU memory used |
| `api_router_backend_requests_total` | API Router | Request count |
| `api_router_backend_request_duration_seconds` | API Router | Request latency |
| `vllm:avg_generation_throughput_toks_per_s` | vLLM | Token throughput |
| `vllm:time_to_first_token_seconds` | vLLM | TTFT latency |
| `kube_pod_status_phase` | kube-state-metrics | Pod status |
