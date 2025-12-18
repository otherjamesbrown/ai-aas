# Grafana Dashboard Context

## Locations

```
infra/k8s/monitoring/dashboards/     # Main dashboards (kustomize deployed)
dashboards/grafana/                   # Additional dashboards
```

## Dashboard Suite (v2.0)

| File | Purpose |
|------|---------|
| gpu-fleet.json | GPU inventory, utilization, health |
| kubernetes-resources.json | Node/pod status, CPU/memory |
| api-performance-v2.json | Request rate, latency, errors by org |
| inference-performance.json | Model latency, throughput, KV cache |
| inference-engine.json | vLLM engine metrics |
| org-usage.json | Organization token consumption |
| cost-efficiency.json | Cost tracking, efficiency metrics |
| platform-overview.json | Health score, API success (in dashboards/grafana/) |

## Key Metrics

**Token metrics** (api-router-service):
- `tokens_processed_total{org_id, model, request_type}` - counter
- `tokens_per_request{org_id, model, request_type}` - histogram

**API metrics** (api-router-service):
- `api_router_backend_requests_total{organization_id, backend, model}`
- `api_router_backend_request_duration_seconds_bucket{...}`
- `api_router_backend_errors_total{organization_id, error_type}`

**GPU metrics** (DCGM exporter):
- `DCGM_FI_DEV_GPU_UTIL` - GPU utilization %
- `DCGM_FI_DEV_FB_USED` / `DCGM_FI_DEV_FB_FREE` - GPU memory

**vLLM metrics**:
- `vllm:prompt_tokens_total`, `vllm:generation_tokens_total`
- `vllm:request_latency_seconds`, `vllm:time_to_first_token_seconds`

## Adding a Dashboard

1. Create JSON in `infra/k8s/monitoring/dashboards/`
2. Add to `kustomization.yaml`:
```yaml
- name: grafana-dashboard-<name>
  files:
    - <name>.json
  options:
    labels:
      grafana_dashboard: "1"
```
3. Validate: `cat <file>.json | jq empty`

## Conventions

- Template variables: `${datasource}` (prometheus), `${organization}` (org filter)
- Tags: `["ai-aas", "<category>"]` for drill-down links
- Schema version: 39
- Refresh: 30s default
