# Observability Stack Integration for vLLM Deployments

**Feature**: `010-vllm-deployment` (User Story 3 - Safe operations)
**Last Updated**: 2025-01-27

## Overview

This document provides integration guidance for connecting vLLM deployments to the AI-AAS observability stack (Prometheus, Grafana, Alertmanager, Loki).

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                        Observability Stack                       │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────┐     ┌──────────────┐     ┌──────────────┐   │
│  │  Prometheus  │────▶│  Grafana     │     │  Alertmanager│   │
│  │  (Metrics)   │     │ (Dashboards) │     │   (Alerts)   │   │
│  └──────┬───────┘     └──────────────┘     └───────┬──────┘   │
│         │                                            │           │
│         │  ┌──────────────┐                         │           │
│         └─▶│     Loki     │                         │           │
│            │    (Logs)    │                         │           │
│            └──────────────┘                         │           │
│                                                      │           │
└──────────────────────────────────────────────────────┼──────────┘
                                                       │
                                                       ▼
                    ┌─────────────────────────────────────────┐
                    │         Notification Channels            │
                    ├─────────────────────────────────────────┤
                    │  • PagerDuty (Critical)                 │
                    │  • Slack (#vllm-alerts)                 │
                    │  • Email (team@example.com)             │
                    └─────────────────────────────────────────┘

                                    ▲
                                    │
                    ┌───────────────┴────────────────┐
                    │                                 │
              ┌─────┴──────┐                  ┌──────┴─────┐
              │  vLLM Pod  │                  │  vLLM Pod  │
              │  (Model A) │                  │  (Model B) │
              └────────────┘                  └────────────┘
                - Metrics: :8000/metrics
                - Logs: stdout/stderr
                - Traces: (future)
```

## Prerequisites

Before integrating vLLM deployments, ensure:

- [ ] Prometheus deployed and accessible
- [ ] Grafana deployed and accessible
- [ ] Alertmanager deployed and configured
- [ ] Loki deployed (optional, for log aggregation)
- [ ] Prometheus Operator installed (for ServiceMonitor CRDs)
- [ ] Sufficient storage for metrics retention (recommended: 30 days)

### Verify Observability Stack

```bash
# Check Prometheus
kubectl get pods -n monitoring -l app=prometheus

# Check Grafana
kubectl get pods -n monitoring -l app=grafana

# Check Alertmanager
kubectl get pods -n monitoring -l app=alertmanager

# Check Loki (if installed)
kubectl get pods -n monitoring -l app=loki
```

## Prometheus Integration

### Method 1: ServiceMonitor (Recommended)

**Prerequisites**: Prometheus Operator installed

**Create ServiceMonitor for vLLM deployments:**

```yaml
# File: infra/k8s/monitoring/vllm-servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: vllm-deployments
  namespace: monitoring
  labels:
    app.kubernetes.io/name: vllm-deployment
    prometheus: kube-prometheus
spec:
  # Select all vLLM services
  selector:
    matchLabels:
      app.kubernetes.io/name: vllm-deployment

  # Scan all namespaces
  namespaceSelector:
    matchNames:
      - system

  # Scrape configuration
  endpoints:
    - port: http  # Service port name
      path: /metrics
      interval: 30s
      scrapeTimeout: 10s

      # Relabeling to add environment and model labels
      relabelings:
        - sourceLabels: [__meta_kubernetes_pod_label_environment]
          targetLabel: environment
        - sourceLabels: [__meta_kubernetes_pod_label_model]
          targetLabel: model
        - sourceLabels: [__meta_kubernetes_pod_name]
          targetLabel: pod
        - sourceLabels: [__meta_kubernetes_namespace]
          targetLabel: namespace
```

**Apply ServiceMonitor:**

```bash
kubectl apply -f infra/k8s/monitoring/vllm-servicemonitor.yaml

# Verify ServiceMonitor
kubectl get servicemonitor vllm-deployments -n monitoring

# Check if Prometheus picked it up
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Open http://localhost:9090/targets and look for vllm-deployments
```

### Method 2: Prometheus Scrape Config (Alternative)

If not using Prometheus Operator, add scrape config directly:

```yaml
# Add to Prometheus ConfigMap
scrape_configs:
  - job_name: 'vllm-deployments'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names:
            - system

    relabel_configs:
      # Only scrape pods with vllm-deployment label
      - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
        action: keep
        regex: vllm-deployment

      # Use pod IP and port 8000
      - source_labels: [__address__]
        action: replace
        regex: ([^:]+)(?::\d+)?
        replacement: ${1}:8000
        target_label: __address__

      # Add environment label
      - source_labels: [__meta_kubernetes_pod_label_environment]
        target_label: environment

      # Add model label
      - source_labels: [__meta_kubernetes_pod_label_model]
        target_label: model

      # Add pod name
      - source_labels: [__meta_kubernetes_pod_name]
        target_label: pod

      # Add namespace
      - source_labels: [__meta_kubernetes_namespace]
        target_label: namespace

    metric_path: /metrics
    scrape_interval: 30s
    scrape_timeout: 10s
```

**Apply configuration:**

```bash
# Update Prometheus ConfigMap
kubectl edit configmap prometheus-server -n monitoring

# Reload Prometheus config
kubectl exec -it prometheus-server-0 -n monitoring -- \
  curl -X POST http://localhost:9090/-/reload
```

### Verify Metrics Collection

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# Query vLLM metrics (in browser or using curl)
curl 'http://localhost:9090/api/v1/query?query=vllm_requests_total'

# Check for expected labels
curl 'http://localhost:9090/api/v1/query?query=vllm_requests_total{environment="production"}'
```

Expected metrics:
- `vllm_requests_total` - Total request count
- `vllm_errors_total` - Total error count
- `vllm_request_duration_seconds_bucket` - Request latency histogram
- `vllm_gpu_utilization` - GPU utilization percentage
- `nvidia_gpu_memory_used_bytes` - GPU memory usage

## Grafana Integration

### Import vLLM Dashboard

**Option 1: Import JSON (Recommended)**

```bash
# Copy dashboard JSON to clipboard or download
cat docs/dashboards/vllm-deployment-dashboard.json

# Import via Grafana UI:
# 1. Go to Grafana (http://grafana.example.com)
# 2. Click "+" → "Import"
# 3. Paste JSON content
# 4. Select Prometheus datasource
# 5. Click "Import"
```

**Option 2: Import via API**

```bash
# Get Grafana admin credentials
GRAFANA_PASSWORD=$(kubectl get secret -n monitoring grafana -o jsonpath='{.data.admin-password}' | base64 -d)

# Import dashboard via API
curl -X POST \
  -H "Content-Type: application/json" \
  -u admin:$GRAFANA_PASSWORD \
  -d @docs/dashboards/vllm-deployment-dashboard.json \
  http://grafana.example.com/api/dashboards/db
```

**Option 3: ConfigMap (GitOps)**

```yaml
# File: infra/k8s/monitoring/grafana-dashboard-configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: vllm-deployment-dashboard
  namespace: monitoring
  labels:
    grafana_dashboard: "1"
data:
  vllm-deployment-dashboard.json: |
    # Paste dashboard JSON here (indented)
```

```bash
kubectl apply -f infra/k8s/monitoring/grafana-dashboard-configmap.yaml
```

### Configure Datasources

Ensure Prometheus datasource is configured:

```yaml
# Grafana Prometheus datasource configuration
apiVersion: 1
datasources:
  - name: Prometheus
    type: prometheus
    access: proxy
    url: http://prometheus-operated.monitoring.svc.cluster.local:9090
    isDefault: true
    editable: false
```

### Setup Dashboard Variables

The vLLM dashboard uses these variables:

| Variable | Query | Description |
|----------|-------|-------------|
| `DS_PROMETHEUS` | Datasource | Prometheus datasource |
| `model` | `label_values(kube_pod_info{namespace="system"}, pod)` | Model name extracted from pod name |
| `environment` | `label_values(kube_pod_labels{namespace="system"}, label_environment)` | Environment (development/staging/production) |

These are auto-configured in the dashboard JSON.

## Alertmanager Integration

### Configure Alert Routes

```yaml
# File: infra/k8s/monitoring/alertmanager-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: alertmanager-config
  namespace: monitoring
data:
  alertmanager.yml: |
    global:
      resolve_timeout: 5m

    # Route configuration
    route:
      group_by: ['alertname', 'environment', 'model']
      group_wait: 10s
      group_interval: 10s
      repeat_interval: 12h
      receiver: 'default'

      routes:
        # Critical alerts → PagerDuty
        - match:
            severity: critical
            component: vllm-deployment
          receiver: 'vllm-pagerduty'
          continue: true  # Also send to Slack

        # High severity → Slack
        - match:
            severity: high
            component: vllm-deployment
          receiver: 'vllm-slack-high'

        # Medium severity → Slack (different channel)
        - match:
            severity: medium
            component: vllm-deployment
          receiver: 'vllm-slack-medium'

        # Info alerts → Email
        - match:
            severity: info
            component: vllm-deployment
          receiver: 'vllm-email'

    # Receivers
    receivers:
      - name: 'default'
        # Catch-all receiver

      - name: 'vllm-pagerduty'
        pagerduty_configs:
          - service_key: '<PAGERDUTY_SERVICE_KEY>'
            description: '{{ .GroupLabels.alertname }} - {{ .GroupLabels.environment }}'
            client: 'AI-AAS Alertmanager'
            client_url: 'https://grafana.example.com'

      - name: 'vllm-slack-high'
        slack_configs:
          - api_url: '<SLACK_WEBHOOK_URL>'
            channel: '#vllm-alerts-high'
            title: 'vLLM Alert: {{ .GroupLabels.alertname }}'
            text: |
              *Environment*: {{ .GroupLabels.environment }}
              *Model*: {{ .GroupLabels.model }}
              *Severity*: {{ .CommonLabels.severity }}

              {{ range .Alerts }}
              *Alert*: {{ .Labels.alertname }}
              *Description*: {{ .Annotations.description }}
              *Runbook*: {{ .Annotations.runbook }}
              {{ end }}
            color: 'danger'

      - name: 'vllm-slack-medium'
        slack_configs:
          - api_url: '<SLACK_WEBHOOK_URL>'
            channel: '#vllm-alerts'
            title: 'vLLM Alert: {{ .GroupLabels.alertname }}'
            text: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'
            color: 'warning'

      - name: 'vllm-email'
        email_configs:
          - to: 'vllm-team@example.com'
            from: 'alertmanager@example.com'
            smarthost: 'smtp.example.com:587'
            auth_username: 'alertmanager@example.com'
            auth_password: '<EMAIL_PASSWORD>'
            headers:
              Subject: '[{{ .Status | toUpper }}] {{ .GroupLabels.alertname }}'
```

**Apply Alertmanager configuration:**

```bash
kubectl apply -f infra/k8s/monitoring/alertmanager-config.yaml

# Reload Alertmanager
kubectl exec -it alertmanager-0 -n monitoring -- \
  curl -X POST http://localhost:9093/-/reload
```

### Install PrometheusRule

```bash
# Apply vLLM alert rules
kubectl apply -f docs/monitoring/vllm-alerts.yaml

# Verify rules are loaded
kubectl get prometheusrule vllm-deployment-alerts -n monitoring

# Check in Prometheus UI
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Open http://localhost:9090/rules and verify vllm-* rules appear
```

### Test Alerts

```bash
# Trigger a test alert (scale down to 0 replicas to trigger VLLMDeploymentDown)
kubectl scale deployment llama-2-7b-development --replicas=0 -n system

# Wait 2-3 minutes for alert to fire

# Check Alertmanager UI
kubectl port-forward -n monitoring svc/alertmanager 9093:9093
# Open http://localhost:9093 and verify alert appears

# Scale back up
kubectl scale deployment llama-2-7b-development --replicas=1 -n system
```

## Loki Integration (Log Aggregation)

### Install Promtail (Log Shipper)

```yaml
# File: infra/k8s/monitoring/promtail-daemonset.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: promtail
  namespace: monitoring
spec:
  selector:
    matchLabels:
      app: promtail
  template:
    metadata:
      labels:
        app: promtail
    spec:
      serviceAccountName: promtail
      containers:
        - name: promtail
          image: grafana/promtail:2.9.0
          args:
            - -config.file=/etc/promtail/promtail.yaml
          volumeMounts:
            - name: config
              mountPath: /etc/promtail
            - name: varlog
              mountPath: /var/log
            - name: varlibdockercontainers
              mountPath: /var/lib/docker/containers
              readOnly: true
      volumes:
        - name: config
          configMap:
            name: promtail-config
        - name: varlog
          hostPath:
            path: /var/log
        - name: varlibdockercontainers
          hostPath:
            path: /var/lib/docker/containers
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: promtail
  namespace: monitoring
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: promtail
rules:
  - apiGroups: [""]
    resources: ["nodes", "pods"]
    verbs: ["get", "list", "watch"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: promtail
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: promtail
subjects:
  - kind: ServiceAccount
    name: promtail
    namespace: monitoring
```

### Configure Promtail

```yaml
# File: infra/k8s/monitoring/promtail-config.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: promtail-config
  namespace: monitoring
data:
  promtail.yaml: |
    server:
      http_listen_port: 9080
      grpc_listen_port: 0

    positions:
      filename: /tmp/positions.yaml

    clients:
      - url: http://loki.monitoring.svc.cluster.local:3100/loki/api/v1/push

    scrape_configs:
      # Scrape vLLM deployment pods
      - job_name: vllm-deployments
        kubernetes_sd_configs:
          - role: pod
            namespaces:
              names:
                - system

        pipeline_stages:
          - docker: {}
          - regex:
              expression: '.*(?P<level>(INFO|WARN|ERROR|DEBUG)).*'
          - labels:
              level:

        relabel_configs:
          # Only scrape vLLM pods
          - source_labels: [__meta_kubernetes_pod_label_app_kubernetes_io_name]
            action: keep
            regex: vllm-deployment

          # Add environment label
          - source_labels: [__meta_kubernetes_pod_label_environment]
            target_label: environment

          # Add model label
          - source_labels: [__meta_kubernetes_pod_label_model]
            target_label: model

          # Add pod name
          - source_labels: [__meta_kubernetes_pod_name]
            target_label: pod

          # Add namespace
          - source_labels: [__meta_kubernetes_namespace]
            target_label: namespace
```

**Apply Promtail:**

```bash
kubectl apply -f infra/k8s/monitoring/promtail-config.yaml
kubectl apply -f infra/k8s/monitoring/promtail-daemonset.yaml

# Verify Promtail is running
kubectl get pods -n monitoring -l app=promtail
```

### Configure Loki Datasource in Grafana

```yaml
apiVersion: 1
datasources:
  - name: Loki
    type: loki
    access: proxy
    url: http://loki.monitoring.svc.cluster.local:3100
    editable: false
```

### Query Logs in Grafana

Example LogQL queries:

```logql
# All vLLM logs from production
{namespace="system", environment="production", app_kubernetes_io_name="vllm-deployment"}

# Error logs only
{namespace="system", app_kubernetes_io_name="vllm-deployment"} |= "ERROR"

# Logs for specific model
{namespace="system", model="llama-2-7b"}

# Logs with latency > 3s
{namespace="system", app_kubernetes_io_name="vllm-deployment"}
  | regexp "duration=(?P<duration>\\d+\\.\\d+)s"
  | duration > 3
```

## Observability Best Practices

### Metrics Retention

**Configure Prometheus retention:**

```yaml
# Prometheus StatefulSet configuration
spec:
  containers:
    - name: prometheus
      args:
        - --storage.tsdb.retention.time=30d
        - --storage.tsdb.retention.size=50GB
```

### High Availability

**Run multiple Prometheus replicas:**

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 2  # High availability
  retention: 30d
  storage:
    volumeClaimTemplate:
      spec:
        resources:
          requests:
            storage: 100Gi
```

### Secure Metrics Endpoints

**Enable authentication for metrics:**

```yaml
# Add authentication to vLLM deployment
env:
  - name: VLLM_METRICS_AUTH_TOKEN
    valueFrom:
      secretKeyRef:
        name: vllm-metrics-token
        key: token
```

### Cost Optimization

**Optimize metric cardinality:**

```yaml
# Reduce high-cardinality labels
metric_relabel_configs:
  # Drop user_id label (high cardinality)
  - source_labels: [__name__]
    regex: 'vllm_requests_total'
    action: labeldrop
    regex: 'user_id'
```

## Troubleshooting Observability

### Metrics Not Showing in Prometheus

```bash
# Check if target is being scraped
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Go to http://localhost:9090/targets

# Test metrics endpoint directly
kubectl port-forward -n system <vllm-pod> 8000:8000
curl http://localhost:8000/metrics

# Check Prometheus logs
kubectl logs -n monitoring prometheus-0 | grep -i error
```

### Alerts Not Firing

```bash
# Check PrometheusRule
kubectl get prometheusrule -n monitoring

# Check Prometheus rules are loaded
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090
# Go to http://localhost:9090/rules

# Check Alertmanager logs
kubectl logs -n monitoring alertmanager-0 | grep -i error
```

### Dashboard Not Loading

```bash
# Check Grafana datasource
kubectl port-forward -n monitoring svc/grafana 3000:3000
# Go to http://localhost:3000/datasources

# Test datasource connection
# In Grafana UI: Configuration → Data Sources → Prometheus → Save & Test

# Check Grafana logs
kubectl logs -n monitoring deployment/grafana | grep -i error
```

## See Also

- [Performance SLO Tracking](./performance-slo-tracking.md)
- [Prometheus Alert Rules](./vllm-alerts.yaml)
- [vLLM Deployment Dashboard](../dashboards/vllm-deployment-dashboard.json)
- [Troubleshooting Guide](../troubleshooting/vllm-deployment-troubleshooting.md)
