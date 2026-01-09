# AI-AAS Platform Alerting Rules

This directory contains Prometheus alerting rules for the AI-AAS platform, focusing on log-based alerts using Loki LogQL queries.

## Overview

The alerting infrastructure monitors:
- **Service error rates** - Application-level errors in Go services
- **Log ingestion health** - Loki and Promtail availability
- **Storage capacity** - Loki PersistentVolume disk usage
- **vLLM inference errors** - GPU errors, model loading failures, OOM conditions
- **API connectivity** - Backend connection failures, database errors
- **Authentication failures** - Potential security issues
- **Service crashes** - Panics, crash loops

## Alert Latency Target

**< 2 minutes** from log event to alert firing (per spec 024 success criteria)

## Files

| File | Purpose |
|------|---------|
| `loki-alerts.yaml` | Log-based alerting rules (LogQL queries) |
| `analytics-rollup-alerts.yaml` | Analytics rollup worker health monitoring |
| `analytics-service.yaml` | Analytics service SLO and reliability alerts |
| `user-org-service.yaml` | User-org service SLO and health alerts |
| `shared-libraries.yaml` | Shared libraries observability alerts |
| `alertmanager-config.yaml` | Alertmanager routing configuration and channel mappings |
| `README.md` | This documentation |

## Alert Categories

### Service Error Rates
- **HighServiceErrorRate**: > 10 errors/min for 2 minutes (WARNING)
- **CriticalErrorBurst**: > 5 fatal/panic/critical errors in 1 minute (CRITICAL)

### Log Ingestion Health
- **LokiIngestionDown**: No log ingestion for 5 minutes (CRITICAL)
- **LokiIngestionSlowdown**: < 1000 bytes/sec for 10 minutes (WARNING)
- **PromtailTargetDown**: Promtail instance not responding for 5 minutes (WARNING)

### Loki Storage
- **LokiDiskSpaceWarning**: PV > 80% full for 5 minutes (WARNING)
- **LokiDiskSpaceCritical**: PV > 90% full for 5 minutes (CRITICAL)

### vLLM Inference Errors
- **vLLMGPUErrorDetected**: GPU/CUDA errors detected in logs (CRITICAL)
- **vLLMModelLoadingFailure**: Model failed to load (CRITICAL)
- **vLLMHighOOMRate**: > 3 OOM errors in 5 minutes (WARNING)
- **vLLMRequestTimeouts**: > 5 timeout errors/min for 3 minutes (WARNING)

### API Service Errors
- **APIRouterBackendConnectionFailure**: > 10 connection failures in 5 min (CRITICAL)
- **AdminAPIDatabaseErrors**: > 5 database errors/min for 2 minutes (CRITICAL)

### Security
- **HighAuthenticationFailureRate**: > 20 auth failures/min for 5 minutes (WARNING)

### Crash Detection
- **ServicePanicDetected**: Go panic detected in logs (CRITICAL)
- **PodCrashLooping**: > 5 restarts in 10 minutes (CRITICAL) [Not yet implemented - see HighPodRestartRate]
- **HighPodRestartRate**: > 10 restarts in 1 hour (CRITICAL)

### Observability Health
- **LowCorrelationIDCoverage**: < 95% of logs have correlation IDs for 15 min (WARNING)

### Kubernetes Resource Health
- **NodeMemoryOvercommitment**: Node memory limits > 100% of allocatable (WARNING)

## Alert Routing

Alerts are routed to different channels based on severity and category:

| Severity/Category | Channel | Response Time |
|-------------------|---------|---------------|
| `severity: critical` | PagerDuty + Slack #platform-critical | Immediate |
| `category: inference` | Slack #ai-models | 15 minutes |
| `alert_type: gpu-error` | Slack #ai-models | Immediate |
| `category: infrastructure` | Slack #platform-infra | 30 minutes |
| `category: security` | Slack #platform-infra | 15 minutes |
| `category: observability` | Slack #platform-infra | 4 hours |

## Deployment

### Via ArgoCD (Recommended)

The alerts are deployed automatically via ArgoCD Application:

```yaml
# gitops/clusters/development/apps/observability/loki-alerts.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: loki-alerts-development
  namespace: argocd
spec:
  source:
    path: infra/k8s/monitoring/alerts
    targetRevision: develop
  destination:
    namespace: monitoring
```

### Manual Deployment

```bash
# Apply PrometheusRule
kubectl apply -f loki-alerts.yaml

# Apply Alertmanager config (after setting secrets)
kubectl apply -f alertmanager-config.yaml
```

## Configuration

### Alertmanager Secrets

Create secrets for Slack and PagerDuty integration:

```bash
kubectl create secret generic alertmanager-secrets \
  --from-literal=slack-webhook-url='https://hooks.slack.com/services/YOUR/WEBHOOK' \
  --from-literal=pagerduty-service-key='YOUR_PAGERDUTY_KEY' \
  -n monitoring
```

### Testing Alerts

To test alert firing:

```bash
# Generate errors in a service
kubectl exec -n development deployment/api-router-service -- \
  curl -X POST http://localhost:8080/api/test-error

# Check alert status in Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Visit http://localhost:9090/alerts

# Check Alertmanager
kubectl port-forward -n monitoring svc/alertmanager 9093:9093
# Visit http://localhost:9093
```

### Querying Alerts in Prometheus

```promql
# Show all firing alerts
ALERTS{alertstate="firing"}

# Show alerts by severity
ALERTS{alertstate="firing", severity="critical"}

# Show vLLM alerts
ALERTS{alertstate="firing", category="inference"}
```

## LogQL Query Patterns

The alerts use these LogQL patterns:

### Error Detection
```logql
{namespace="development"} |= "error" | json | level="error"
```

### Critical Error Burst
```logql
{namespace="development"} |= "fatal" or "panic" or "critical"
```

### GPU Errors
```logql
{namespace=~"ai-models|system|kserve"} |~ "(?i)(gpu error|cuda error|out of memory)"
```

### Authentication Failures
```logql
{namespace="development"} |~ "(?i)(authentication failed|unauthorized|invalid.*token)"
```

## Maintenance

### Silencing Alerts

During maintenance windows:

```bash
# Via Alertmanager UI
kubectl port-forward -n monitoring svc/alertmanager 9093:9093
# Create silence at http://localhost:9093/#/silences

# Via annotation (for planned maintenance)
kubectl annotate namespace development maintenance.ai-aas.dev/enabled=true
```

### Adjusting Thresholds

Edit `loki-alerts.yaml` and adjust the `expr` or `for` duration:

```yaml
- alert: HighServiceErrorRate
  expr: ... > 10  # Change threshold here
  for: 2m         # Change duration here
```

Then commit and push - ArgoCD will sync automatically.

## Troubleshooting

### Alert Not Firing

1. Check if the PrometheusRule is loaded:
   ```bash
   kubectl get prometheusrule -n monitoring loki-log-alerts
   ```

2. Test the LogQL query in Grafana Explore:
   ```bash
   kubectl port-forward -n monitoring svc/grafana 3000:80
   # Visit http://localhost:3000 and use Explore tab
   ```

3. Check Prometheus rule evaluation:
   ```bash
   kubectl port-forward -n monitoring svc/prometheus 9090:9090
   # Visit http://localhost:9090/rules
   ```

### Alert Firing Too Often

- Adjust the `for` duration to add tolerance
- Use inhibit rules in Alertmanager to suppress dependent alerts
- Consider log sampling for verbose endpoints

### Missing Labels

Ensure pods have the correct labels for alert grouping:

```yaml
metadata:
  labels:
    app: your-service
    environment: development
```

## Related Documentation

- [Observability Guide](../../../../docs/platform/observability-guide.md) - Overall observability architecture
- [vLLM Observability](../../../../docs/platform/vllm-observability.md) - vLLM-specific metrics
- [Loki Configuration](../loki/README.md) - Loki deployment and configuration
- [Grafana Dashboards](../dashboards/README.md) - Dashboard documentation

## Success Criteria Verification

Per spec 024, verify these success criteria:

- [ ] Can search logs across all services for the past 14 days
- [ ] Production errors trigger notifications within 5 minutes (2 min alert + 3 min routing)
- [ ] Can query vLLM logs for model loading, inference errors, and GPU issues
- [ ] Alert latency < 2 minutes from log event to alert state change
- [ ] Alerts include service name, summary, and sample errors in annotations
- [ ] Critical alerts reach PagerDuty within 2 minutes
- [ ] vLLM alerts reach #ai-models Slack channel
- [ ] Correlation ID coverage maintained at 95%+
