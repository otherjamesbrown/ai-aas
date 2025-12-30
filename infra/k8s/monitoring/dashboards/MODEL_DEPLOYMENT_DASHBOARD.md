# Model Deployment Dashboard

**Dashboard UID**: `model-deployment`
**Location**: `infra/k8s/monitoring/dashboards/model-deployment.json`
**Related Bead**: aas-gwxm2

## Overview

The Model Deployment dashboard provides centralized visibility into AIModel lifecycle, deployment phases, failure patterns, and per-model debugging.

## Dashboard Sections

### 1. Model Deployment Overview

**Panels:**
- **Models by Phase** (Pie Chart): Distribution of models across all deployment phases
- **Models in Error State** (Stat): Count of models in Failed phase (red alert when > 0)
- **Average Time to Ready** (Stat): Mean deployment duration (requires custom metric)
- **Models Stuck > 30min** (Stat): Models in non-Running phase for more than 30 minutes
- **Models by Phase Over Time** (Time Series): Historical view of phase distribution

### 2. Failure Analysis

**Panels:**
- **Failure Reasons Breakdown** (Pie Chart): Pod container waiting reasons (ImagePullBackOff, CrashLoopBackOff, etc.)
- **Scheduling Failures by Reason** (Bar Chart): Unschedulable pods grouped by reason (InsufficientGPU, NodeAffinity, etc.)
- **Recent Failures** (Logs): Last 10 errors from ai-model-operator logs

### 3. Per-Model Details

**Panels:**
- **Current Phase** (Stat): Deployment phase for selected model (color-coded by status)
- **Phase Duration** (Stat): Time spent in current phase
- **Replica Status** (Time Series): Ready vs Desired replicas over time
- **Model Events** (Logs): Operator logs filtered by selected model
- **Pod Logs** (Logs): Container logs from inference pod

## Variables

| Variable | Description | Source |
|----------|-------------|--------|
| `$namespace` | Filter by namespace (multi-select) | `kube_customresource_status_condition` labels |
| `$model` | Select specific AIModel | `kube_customresource_status_condition{customresource_name}` |

## Data Sources

### Current Implementation (kube-state-metrics)

The dashboard currently uses kube-state-metrics for AIModel CRD status:

```promql
# Model count by phase
count by (customresource_phase) (
  kube_customresource_status_condition{
    customresource_kind="AIModel",
    namespace=~"$namespace"
  }
)

# Failed models
count(
  kube_customresource_status_condition{
    customresource_kind="AIModel",
    customresource_phase="Failed",
    namespace=~"$namespace"
  }
)

# Pod-based phase duration (proxy metric)
time() - kube_pod_created{
  namespace=~"$namespace",
  pod=~"$model.*-predictor-.*"
}
```

### Recommended Custom Metrics (Future Enhancement)

The ai-model-operator should expose these Prometheus metrics for improved observability:

```go
// operators/ai-model-operator/internal/metrics/metrics.go

var (
    // Track current phase of each AIModel
    aimodelPhase = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "aimodel_status_phase",
            Help: "Current phase of AIModel (1=active phase, 0=inactive)",
        },
        []string{"name", "namespace", "phase"},
    )

    // Deployment duration histogram
    aimodelDeploymentDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "aimodel_deployment_duration_seconds",
            Help:    "Time taken from Pending to Ready",
            Buckets: []float64{60, 120, 300, 600, 900, 1200, 1800, 3600},
        },
        []string{"name", "namespace", "runtime"},
    )

    // Phase duration (time in current phase)
    aimodelPhaseDuration = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "aimodel_phase_duration_seconds",
            Help: "Time spent in current phase",
        },
        []string{"name", "namespace", "phase"},
    )

    // Blocked models
    aimodelBlocked = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "aimodel_blocked",
            Help: "Whether AIModel is blocked (1=blocked, 0=not blocked)",
        },
        []string{"name", "namespace", "reason"},
    )

    // Phase transitions counter
    aimodelPhaseTransitions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aimodel_phase_transitions_total",
            Help: "Total number of phase transitions",
        },
        []string{"name", "namespace", "from_phase", "to_phase"},
    )

    // Failure reasons
    aimodelFailureReasons = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "aimodel_failures_total",
            Help: "Total failures by reason",
        },
        []string{"name", "namespace", "reason"},
    )
)
```

**Usage in controller:**

```go
// When updating AIModel status
func (r *AIModelReconciler) updatePhase(ctx context.Context, aiModel *aimodelv1alpha1.AIModel, newPhase aimodelv1alpha1.AIModelPhase) error {
    oldPhase := aiModel.Status.Phase

    // Update status
    aiModel.Status.Phase = newPhase
    aiModel.Status.PhaseStartTime = metav1.Now()

    // Update metrics
    metrics.RecordPhaseTransition(aiModel.Name, aiModel.Namespace, string(oldPhase), string(newPhase))
    metrics.SetPhase(aiModel.Name, aiModel.Namespace, string(newPhase))

    // Track deployment duration when reaching Ready
    if newPhase == aimodelv1alpha1.AIModelPhaseReady && aiModel.Status.DeploymentStartedAt != nil {
        duration := time.Since(aiModel.Status.DeploymentStartedAt.Time).Seconds()
        metrics.RecordDeploymentDuration(aiModel.Name, aiModel.Namespace, aiModel.Spec.Runtime, duration)
    }

    return r.Status().Update(ctx, aiModel)
}
```

## Deployment

The dashboard is automatically deployed via Kustomize:

```bash
# Generate ConfigMap
kubectl kustomize infra/k8s/monitoring/dashboards/ > /tmp/dashboards.yaml

# Apply to cluster
kubectl apply -f /tmp/dashboards.yaml
```

Grafana sidecar will auto-load any ConfigMap with label `grafana_dashboard: "1"` in the `monitoring` namespace.

## Grafana Access

- **Development**: https://grafana.dev.otherjamesbrown.com
- **Production**: https://grafana.prod.otherjamesbrown.com

Navigate to: **Dashboards** → **Model Deployment**

## Troubleshooting

### Dashboard shows "No data"

**Check kube-state-metrics is scraping AIModel CRDs:**
```bash
kubectl get --raw /apis/metrics.k8s.io/v1beta1/customresources/aimodels.aimodel.ai-aas.io
```

**Verify Prometheus is scraping kube-state-metrics:**
```promql
up{job="kube-state-metrics"}
```

**Check if AIModel CRD status is populated:**
```bash
kubectl get aimodels -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.phase}{"\n"}{end}'
```

### Panels show "requires custom metric"

These panels are placeholders for future operator metrics. To implement:
1. Add Prometheus metrics to ai-model-operator (see "Recommended Custom Metrics" above)
2. Expose metrics endpoint on `:8080/metrics`
3. Add ServiceMonitor to scrape operator metrics
4. Update dashboard queries to use custom metrics

### Logs panels are empty

**Check Loki is receiving operator logs:**
```bash
kubectl logs -n system -l app=ai-model-operator | head -20
```

**Verify Promtail is running:**
```bash
kubectl get pods -n system -l app=promtail
```

**Test Loki query:**
```bash
curl -G https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range \
  --data-urlencode 'query={app="ai-model-operator"}' \
  --data-urlencode 'limit=10'
```

## Related Documentation

- [Observability Guide](../../../docs/platform/observability-guide.md) - Full observability stack documentation
- [AI Model Operator](../../../operators/ai-model-operator/README.md) - Operator architecture
- [Grafana Dashboards](./README.md) - Dashboard provisioning and standards
- [Epic: aas-j1q4](../../../.beads/issues/aas-j1q4.yaml) - Model deployment observability improvements

## Future Enhancements

1. **Custom Operator Metrics** (requires aas-j1q4 implementation):
   - Accurate deployment duration tracking
   - Phase-specific duration breakdowns
   - Retry attempt tracking
   - Failure reason classification

2. **Alert Integration**:
   - Panel annotations from Alertmanager
   - Link to related alerts in failure panels
   - Automatic alert creation for stuck deployments

3. **Cost Tracking**:
   - GPU-hours per model deployment
   - Failed deployment cost waste
   - Resource efficiency metrics

4. **Deployment Comparison**:
   - Side-by-side model comparison
   - Historical deployment trends
   - Performance regression detection
