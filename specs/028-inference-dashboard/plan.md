# Implementation Plan: Observability Dashboard Suite

**Feature Branch**: `028-inference-dashboard`
**Date**: 2024-12-16
**Spec**: [dashboard-suite-spec.md](./dashboard-suite-spec.md)

## Summary

Replace the existing 7 Grafana dashboards with a new suite of 8 purpose-driven dashboards organized into three tiers: Operational (daily monitoring), Business (usage and costs), and Infrastructure (debugging and capacity).

**Technical Approach**:
- Create Grafana dashboard JSON files
- Deploy via GitOps (ConfigMap → ArgoCD sync)
- Validate queries against live Prometheus/Loki

---

## Technical Context

| Aspect | Details |
|--------|---------|
| **Platform** | Grafana 11.x |
| **Data Sources** | Prometheus (via OTEL Collector), Loki, Tempo |
| **Deployment** | ConfigMap with `grafana_dashboard: "1"` label |
| **Location** | `infra/k8s/monitoring/dashboards/` |
| **Testing** | Query validation via Grafana/Loki APIs |

### Dependencies

| Dependency | Status | Notes |
|------------|--------|-------|
| Prometheus metrics | ✅ Available | Via OTEL Collector port 8889 |
| Loki logs | ✅ Available | loki.dev.otherjamesbrown.com |
| DCGM Exporter | ✅ Available | GPU metrics via PodMonitor |
| vLLM metrics | ✅ Available | Port 8000 via PodMonitor |
| Token count metrics | ❌ Missing | **Requires Task 1** |
| Cost configuration | ❌ Missing | **Requires Task 2** |

---

## Constitution Compliance

| Principle | How Plan Complies |
|-----------|-------------------|
| GitOps-First | Dashboards deployed via ArgoCD, no manual kubectl |
| Observable Systems | This spec improves observability |
| Single Source of Truth | Dashboard JSON in git, ConfigMap generated |

---

## Project Structure

### Files to Remove (after validation)

```
infra/k8s/monitoring/dashboards/
├── api-performance.json        # Remove
├── fleet-overview.json         # Remove
├── inference-backends.json     # Remove
├── node-cluster-view.json      # Remove
├── per-gpu-type-analysis.json  # Remove
├── per-model-performance.json  # Remove
└── service-logs.json           # Remove
```

### Files to Create

```
infra/k8s/monitoring/dashboards/
├── platform-overview.json      # NEW - Phase 1
├── gpu-fleet.json              # NEW - Phase 1
├── kubernetes-resources.json   # NEW - Phase 1
├── inference-performance.json  # NEW - Phase 2
├── api-performance-v2.json     # NEW - Phase 2
├── inference-engine.json       # NEW - Phase 2
├── org-usage.json              # NEW - Phase 3 (after Task 1)
└── cost-efficiency.json        # NEW - Phase 3 (after Task 1+2)
```

### Files to Keep

```
infra/k8s/monitoring/dashboards/
└── request-tracing.json        # Keep - orthogonal concern
```

### Files to Modify (Instrumentation)

```
services/api-router-service/internal/telemetry/exporters.go  # Task 1: Add token metrics
services/api-router-service/internal/api/public/openai.go    # Task 1: Call RecordTokens()
infra/k8s/monitoring/cost-config.yaml                        # Task 2: Create ConfigMap
```

---

## Implementation Phases

### Phase 1: Foundation Dashboards (No Blockers)

Create dashboards that work with existing metrics:

| Dashboard | File | Key Queries |
|-----------|------|-------------|
| Platform Overview | `platform-overview.json` | `up{}`, `api_router_*`, `DCGM_*` |
| GPU Fleet | `gpu-fleet.json` | `DCGM_FI_DEV_*` |
| Kubernetes Resources | `kubernetes-resources.json` | `kube_*`, `container_*` |

**Deliverables**:
- 3 dashboard JSON files
- Data source status panel template (reusable)
- Drill-down link patterns established

### Phase 2: Performance Dashboards (No Blockers)

| Dashboard | File | Key Queries |
|-----------|------|-------------|
| Inference Performance | `inference-performance.json` | `vllm:*` |
| API Performance | `api-performance-v2.json` | `api_router_*` |
| Inference Engine | `inference-engine.json` | `vllm_*` |

**Deliverables**:
- 3 dashboard JSON files
- Model selector variable template
- Logs panel integration

### Phase 3: Instrumentation Work (Blockers)

#### Task 1: Token Metrics (Required for Org Usage)

**Files**:
- `services/api-router-service/internal/telemetry/exporters.go`
- `services/api-router-service/internal/api/public/openai.go`

**Changes**:
```go
// Add to exporters.go
var TokensProcessedTotal = promauto.NewCounterVec(...)
var TokensPerRequest = promauto.NewHistogramVec(...)
func RecordTokens(orgID, model string, input, output int) {...}

// Call from openai.go after extracting token counts
telemetry.RecordTokens(orgID, model, promptTokens, completionTokens)
```

**Validation**: `curl` Prometheus to verify `api_router_tokens_total` metric exists

#### Task 2: Cost Configuration (Required for Cost Dashboard)

**File**: `infra/k8s/monitoring/cost-config.yaml`

**Content**: ConfigMap with GPU costs per hour

### Phase 4: Business Dashboards (After Phase 3)

| Dashboard | File | Requires |
|-----------|------|----------|
| Org Usage | `org-usage.json` | Task 1 complete |
| Cost & Efficiency | `cost-efficiency.json` | Task 1 + Task 2 complete |

### Phase 5: Cleanup

1. Validate all new dashboards work
2. Remove old dashboard files
3. Update kustomization.yaml if needed

---

## Dashboard Template Standards

All dashboards should follow these patterns:

### 1. Status Panel (Top Row)

Every dashboard includes a data source status panel:

```json
{
  "title": "Data Source Status",
  "type": "stat",
  "gridPos": {"h": 2, "w": 24, "x": 0, "y": 0},
  "targets": [
    {"expr": "up{job=\"prometheus\"}", "legendFormat": "Prometheus"},
    {"expr": "count(DCGM_FI_DEV_GPU_UTIL)", "legendFormat": "DCGM"}
  ]
}
```

### 2. Refresh Rate

```json
{
  "refresh": "10s",
  "time": {"from": "now-1h", "to": "now"}
}
```

### 3. Drill-Down Links

```json
{
  "links": [
    {
      "title": "GPU Fleet",
      "url": "/d/gpu-fleet?var-gpu=${__field.labels.gpu}",
      "targetBlank": false
    }
  ]
}
```

### 4. Color Thresholds

```json
{
  "thresholds": {
    "mode": "absolute",
    "steps": [
      {"color": "green", "value": null},
      {"color": "yellow", "value": 80},
      {"color": "red", "value": 95}
    ]
  }
}
```

---

## Validation Approach

### Query Validation Script

```bash
#!/bin/bash
# Test all dashboard queries against live Prometheus

QUERIES=(
  "up{}"
  "DCGM_FI_DEV_GPU_UTIL"
  "api_router_backend_requests_total"
  "vllm:avg_generation_throughput_toks_per_s"
  "kube_pod_status_phase"
)

for q in "${QUERIES[@]}"; do
  result=$(curl -s "https://grafana.dev.otherjamesbrown.com/api/datasources/proxy/uid/prometheus/api/v1/query" \
    --data-urlencode "query=$q" | jq '.data.result | length')
  echo "$q: $result results"
done
```

### Dashboard Load Test

```bash
# Verify dashboard loads in <3s
time curl -s "https://grafana.dev.otherjamesbrown.com/api/dashboards/uid/platform-overview" > /dev/null
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Breaking existing dashboards | Keep old dashboards until new ones validated |
| Missing metrics | Status panel shows data availability |
| Query performance | Use `$__interval` for aggregation, limit to 7 days |
| Token metrics not deployed | Phase 3 dashboards blocked until Task 1 complete |

---

## Next Steps

1. Run `/speckit.tasks` to create task breakdown and beads
2. Phase 1 can start immediately (no blockers)
3. Task 1 (token metrics) can run in parallel with Phase 1-2

---

## References

- [Grafana Dashboard JSON Model](https://grafana.com/docs/grafana/latest/dashboards/json-model/)
- [PromQL Functions](https://prometheus.io/docs/prometheus/latest/querying/functions/)
- [Existing dashboards](../../infra/k8s/monitoring/dashboards/)
