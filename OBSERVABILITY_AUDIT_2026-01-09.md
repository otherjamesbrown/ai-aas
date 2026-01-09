# Observability Configuration Audit Report

**Date**: 2026-01-09
**Bead**: aas-sfxxr
**Auditor**: infra-ops-manager agent
**Scope**: Grafana dashboards, ServiceMonitors, Prometheus config, alert rules, datasources

---

## Executive Summary

The observability stack is **well-structured** with a clear hierarchy and GitOps deployment model. However, there are several **organizational issues**, **missing components**, and **documentation gaps** that need attention.

### Key Findings
1. **Dashboard duplication**: Legacy dashboards exist in `/dashboards/` alongside canonical ones in `/infra/k8s/monitoring/dashboards/`
2. **Missing ServiceMonitors**: Several deployed services lack Prometheus scraping configuration
3. **Inconsistent alert deployment**: Development has loki-alerts ArgoCD app, staging does not
4. **Missing Tempo in staging**: Tracing datasource only deployed to development
5. **Documentation inaccuracies**: Several docs reference outdated paths and missing components

---

## 1. Grafana Dashboards

### 1.1 Canonical Dashboard Location

**Primary location**: `/infra/k8s/monitoring/dashboards/`

This directory contains the **Dashboard Suite v2.0** (from spec028) and uses Kustomize to generate ConfigMaps with the `grafana_dashboard: "1"` label for automatic sidecar discovery.

**Deployment**:
- ArgoCD Application: `monitoring-dashboards-staging` (and `monitoring-dashboards-development`)
- Path: `infra/k8s/monitoring/dashboards`
- Kustomize generates ConfigMaps for each dashboard
- Grafana sidecar auto-discovers dashboards via label selector

**Dashboard Inventory** (12 dashboards):
| Dashboard | File | Purpose |
|-----------|------|---------|
| GPU Fleet | `gpu-fleet.json` | GPU inventory, utilization heatmaps |
| Kubernetes Resources | `kubernetes-resources.json` | Node/pod status, CPU/memory |
| API Performance v2 | `api-performance-v2.json` | Request rate, latency, errors by org |
| Inference Performance | `inference-performance.json` | Model latency, throughput, KV cache |
| Inference Engine | `inference-engine.json` | vLLM engine metrics, memory management |
| Organization Usage | `org-usage.json` | Org token consumption patterns |
| Cost Efficiency | `cost-efficiency.json` | Cost tracking, efficiency metrics |
| Service Logs | `service-logs.json` | Loki log viewer dashboard |
| Request Tracing | `request-tracing.json` | Tempo trace viewer dashboard |
| Inference Backends | `inference-backends.json` | Backend health and status |
| Model Deployment | `model-deployment.json` | Model deployment tracking |
| Routing Policy Health | `routing-policy-health.json` | Routing policy monitoring |
| Benchmark System Health | `benchmark-system-health.json` | Benchmark system monitoring |

**Status**: ✅ Well-organized, properly deployed via GitOps

### 1.2 Legacy/Duplicate Dashboards

**Problem**: Multiple dashboard locations create confusion and potential drift.

**Locations found**:

1. **`/dashboards/grafana/`** (4 dashboards):
   - `analytics-usage.json` - Possibly duplicate of `org-usage.json`?
   - `e2e-trends.json` - E2E test metrics dashboard
   - `platform-overview.json` - High-level platform overview
   - `shared-libraries.json` - Purpose unclear

2. **`/dashboards/analytics/`** (1 dashboard):
   - `usage.json` - Likely duplicate of `analytics-usage.json`

3. **`/services/api-router-service/deployments/helm/api-router-service/dashboards/`** (1 dashboard):
   - `api-router.json` - Service-specific dashboard (24KB)

4. **`/docs-humans/dashboards/`** (1 dashboard):
   - `vllm-deployment-dashboard.json` - Documentation/example dashboard?

**Issues**:
- ❌ `/dashboards/` directory is NOT deployed via any ArgoCD application
- ❌ Unclear relationship to canonical dashboards in `/infra/k8s/monitoring/dashboards/`
- ❌ No Kustomize configuration or ConfigMap generation
- ❌ These dashboards are orphaned - not loaded into Grafana
- ⚠️ Potential for confusion: developers may edit wrong file

**Questions to resolve**:
1. Are these legacy dashboards superseded by Dashboard Suite v2.0?
2. Should `e2e-trends.json` and `platform-overview.json` be migrated to canonical location?
3. Should service-specific dashboards (like `api-router.json`) be co-located with service Helm charts or centralized?

---

## 2. ServiceMonitors (Prometheus Scraping)

### 2.1 Services WITH ServiceMonitors ✅

| Service | Location | Scrape Path | Interval |
|---------|----------|-------------|----------|
| api-router-service | `services/api-router-service/deployments/helm/api-router-service/templates/servicemonitor.yaml` | Configured via `values.prometheus.path` | 30s |
| admin-api-service | `services/admin-api-service/deployments/helm/admin-api-service/templates/servicemonitor.yaml` | Configured via `values.prometheus.path` | 30s |
| analytics-service | `services/analytics-service/deployments/helm/analytics-service/templates/servicemonitor.yaml` | Configured via `values.prometheus.path` | 30s |
| user-org-service | `services/user-org-service/configs/helm/templates/servicemonitor.yaml` | Configured via `values.prometheus.path` | 30s |

**Note**: All ServiceMonitors are conditionally created based on `.Values.prometheus.enabled` flag.

**Archived**:
- `archive/helm-charts/vllm-deployment/templates/servicemonitor.yaml` - Archived vLLM chart

### 2.2 Services WITHOUT ServiceMonitors ❌

The following services exist but lack ServiceMonitor definitions:

| Service | Type | Needs Metrics? |
|---------|------|----------------|
| hello-service | Example/demo service | ❓ Likely no - demo only |
| world-service | Example/demo service | ❓ Likely no - demo only |
| model-downloader | Job/utility | ⚠️ Maybe - download progress metrics? |
| preprocessor-service | Python gRPC service | ⚠️ Maybe - preprocessing metrics? |

**Note**: `hello-service` and `world-service` appear to be example services with no Kubernetes deployment manifests, so ServiceMonitors are not needed.

**Questions to resolve**:
1. Should `preprocessor-service` expose Prometheus metrics?
2. Should `model-downloader` expose job progress metrics?
3. Are there any vLLM/KServe InferenceServices that should have PodMonitors for GPU metrics?

---

## 3. Prometheus Configuration

### 3.1 kube-prometheus-stack Deployment

**Primary config locations**:
1. `/infra/helm/charts/kube-prometheus-stack-values-staging.yaml` - Separate values file (NOT USED)
2. `/gitops/clusters/staging/apps/kube-prometheus-stack.yaml` - ArgoCD Application with inline values

**Issue**: ⚠️ The separate values file (`kube-prometheus-stack-values-staging.yaml`) is **NOT referenced** by the ArgoCD Application. Values are inlined in the Application manifest instead.

**Current Configuration**:
- Chart source: `https://prometheus-community.github.io/helm-charts`
- Chart version: `80.4.1` (matches development)
- Namespace: `monitoring`
- Retention: 15 days
- Storage: 50Gi PVC
- Scrape interval: 15s

**ServiceMonitor Discovery** (CRITICAL):
```yaml
# These settings ensure Prometheus discovers ALL ServiceMonitors
serviceMonitorSelectorNilUsesHelmValues: false
podMonitorSelectorNilUsesHelmValues: false
ruleSelectorNilUsesHelmValues: false
```
✅ This is correctly configured - no label filtering on discovery.

**Grafana Configuration**:
- Enabled: ✅ Yes
- Persistence: ✅ 10Gi PVC
- Admin password: ⚠️ `admin123` (hardcoded - should be secret)
- Ingress: ✅ Enabled with TLS (staging: `grafana.staging.otherjamesbrown.com`)
- Dashboard sidecar: ✅ Enabled with label `grafana_dashboard: "1"`
- Datasources: ⚠️ Only Prometheus configured inline, Loki/Tempo via separate ConfigMap?

**Datasource Configuration Issue**:
The inline Grafana datasources section in the ArgoCD Application does NOT include Loki or Tempo:
```yaml
# grafana.datasources is NOT present in staging ArgoCD Application
# Only default Prometheus datasource is configured by the chart
```

However, the separate values file (`kube-prometheus-stack-values-staging.yaml`) DOES have datasources defined:
```yaml
datasources:
  datasources.yaml:
    apiVersion: 1
    datasources:
      - name: Prometheus
        type: prometheus
        url: http://kube-prometheus-stack-prometheus:9090
        isDefault: true
      - name: Loki
        url: http://loki-gateway.observability.svc.cluster.local
      - name: Tempo
        url: http://tempo.observability.svc.cluster.local:3100
```

**Problem**: ❌ Since the separate values file is not used, these datasources are NOT configured. Loki and Tempo must be added manually in Grafana or via ConfigMap.

### 3.2 Recording Rules

**Search result**: ❌ No Prometheus recording rules found in the codebase.

**Common use cases for recording rules**:
- Pre-aggregated request rate metrics
- Cost calculation based on token usage
- GPU utilization aggregates

**Recommendation**: Consider adding recording rules for frequently queried metrics to improve dashboard performance.

---

## 4. Alert Rules

### 4.1 PrometheusRule Deployments

**Deployment structure**:

| Environment | PrometheusRule File | ArgoCD Application | Status |
|-------------|---------------------|---------------------|--------|
| development | `infra/k8s/prometheus-rules/development/platform-alerts.yaml` | `prometheus-alerts-development` | ✅ Deployed |
| development | `infra/k8s/monitoring/alerts/*.yaml` | `loki-alerts-development` | ✅ Deployed |
| staging | `infra/k8s/prometheus-rules/staging/platform-alerts.yaml` | `prometheus-alerts-staging` | ✅ Deployed |
| staging | `infra/k8s/monitoring/alerts/*.yaml` | ❌ NO ArgoCD APP | ❌ NOT deployed |

**Issue**: ⚠️ Staging lacks the `loki-alerts-staging` ArgoCD application, so the following alert files are NOT deployed to staging:
- `infra/k8s/monitoring/alerts/loki-alerts.yaml` - Loki ingestion health
- `infra/k8s/monitoring/alerts/analytics-rollup-alerts.yaml` - Analytics rollup job health

**Result**: Staging has fewer alerts than development, creating a coverage gap.

### 4.2 Alert Rule Inventory

**Platform Alerts** (`infra/k8s/prometheus-rules/staging/platform-alerts.yaml`):

| Alert Group | Alerts | Purpose |
|-------------|--------|---------|
| api-router-service | 3 alerts | High error rate, high latency, pod not ready |
| admin-api-service | 1 alert | High error rate |
| gpu-inference | 5 alerts | vLLM latency, GPU memory, pod restarts, GPU node health, NVIDIA driver |
| infrastructure | 2 alerts | High memory usage, high CPU usage |
| guidellm-runner | 1 alert | Stuck benchmark runner |

**Loki Infrastructure Alerts** (`infra/k8s/monitoring/alerts/loki-alerts.yaml` - NOT in staging):

| Alert Group | Alerts | Purpose |
|-------------|--------|---------|
| loki-ingestion | 3 alerts | Ingestion down, ingestion slowdown, Promtail down |
| loki-storage | 2 alerts | Disk space warning/critical |
| kubernetes-pod-health | 1 alert | High pod restart rate |
| kubernetes-node-health | 1 alert | Node memory overcommitment |

**Analytics Rollup Alerts** (`infra/k8s/monitoring/alerts/analytics-rollup-alerts.yaml` - NOT in staging):

| Alert Group | Alerts | Purpose |
|-------------|--------|---------|
| analytics-rollup-health | 3 alerts | Rollup failure rate, stale rollup, service down |

### 4.3 Alertmanager Configuration

**Location**: `infra/k8s/monitoring/alerts/alertmanager-config.yaml`

**Status**: ⚠️ This ConfigMap is NOT deployed via any ArgoCD Application. It's a standalone file with no deployment mechanism.

**Contents**:
- Alert routing based on severity and category
- Receiver definitions (Slack, PagerDuty)
- Inhibition rules to suppress dependent alerts
- Templated webhook URLs (requires secrets)

**Issue**: ❌ This configuration is not applied to the cluster. Alertmanager is using default config from kube-prometheus-stack chart.

**Alertmanager Secret Template**:
The file includes a Secret template for Slack/PagerDuty credentials, but with empty values:
```yaml
stringData:
  slack-webhook-url: ""
  pagerduty-service-key: ""
```

---

## 5. Datasource Configuration

### 5.1 Prometheus

**Configuration**: ✅ Auto-configured by kube-prometheus-stack chart
**URL**: `http://kube-prometheus-stack-prometheus:9090`
**Status**: ✅ Working

### 5.2 Loki

**Deployment**:
- ArgoCD Application: `loki-staging` (and `loki-development`)
- Chart: `https://grafana.github.io/helm-charts` (chart: `loki`, version: `6.16.0`)
- Namespace: `observability`
- Mode: SingleBinary with 50Gi persistence
- Gateway: Enabled

**Datasource Configuration**:
- ❌ NOT auto-configured in kube-prometheus-stack
- ⚠️ Must be added manually in Grafana or via ConfigMap
- Expected URL: `http://loki-gateway.observability.svc.cluster.local`

**Status**: ⚠️ Loki is deployed but datasource may not be configured in Grafana

### 5.3 Tempo

**Deployment**:
- ✅ Deployed to **development**: `gitops/clusters/development/apps/tempo.yaml`
- ❌ NOT deployed to **staging**: No corresponding `tempo.yaml` in staging apps

**Issue**: ❌ Staging lacks distributed tracing capability. Tempo datasource cannot be configured because Tempo is not deployed.

**Result**: Request tracing dashboard (`request-tracing.json`) will not work in staging.

---

## 6. Documentation Issues

### 6.1 Observability Architecture Doc

**File**: `docs/architecture/observability-architecture.md`

**Issues found**:
1. ⚠️ `last_updated: 2025-12-12` - Date is in the past (should be current)
2. ✅ Architecture diagram is accurate for development
3. ⚠️ Does not mention staging lacks Tempo
4. ⚠️ References "Promtail (DaemonSet)" but does not specify how it's deployed (part of Loki chart? Separate?)

### 6.2 Debugging Workflow Runbook

**File**: `docs/runbooks/ai-debugging-workflow.md`

**Issues found**:
1. ⚠️ `last_updated: 2025-12-15` - Date is in the past
2. ✅ Commands are accurate
3. ⚠️ References `/home/dev/ai-aas-024-observability/` path which is workspace-specific
4. ⚠️ Does not document differences between development and staging observability
5. ✅ LogQL examples are correct

---

## 7. Cross-Environment Comparison

| Component | Development | Staging | Production | Notes |
|-----------|-------------|---------|------------|-------|
| **kube-prometheus-stack** | ✅ v80.4.1 | ✅ v80.4.1 | ❓ Unknown | - |
| **Grafana dashboards** | ✅ 12 dashboards | ✅ 12 dashboards | ❓ Unknown | Via monitoring-dashboards app |
| **Loki** | ✅ v6.16.0 | ✅ v6.16.0 | ❓ Unknown | Logs aggregation |
| **Tempo** | ✅ Deployed | ❌ NOT deployed | ❓ Unknown | Staging lacks tracing |
| **Platform alerts** | ✅ Deployed | ✅ Deployed | ❓ Unknown | Via prometheus-alerts app |
| **Loki/analytics alerts** | ✅ Deployed | ❌ NOT deployed | ❓ Unknown | Staging lacks loki-alerts app |
| **Alertmanager config** | ❌ Not applied | ❌ Not applied | ❓ Unknown | ConfigMap exists but not deployed |
| **Loki datasource** | ⚠️ Manual? | ⚠️ Manual? | ❓ Unknown | Not in kube-prometheus-stack values |
| **Tempo datasource** | ⚠️ Manual? | ❌ N/A (Tempo not deployed) | ❓ Unknown | - |

**Key Gap**: Staging is missing several observability components compared to development.

---

## 8. Issues Summary

### Critical Issues (P1)

1. **Staging lacks Tempo** - No distributed tracing in staging environment
2. **Staging lacks loki-alerts ArgoCD app** - Missing Loki health and analytics rollup alerts
3. **Alertmanager config not deployed** - Custom alert routing not applied

### High Priority Issues (P2)

4. **Orphaned dashboards in `/dashboards/`** - Duplicate/legacy dashboards not deployed, causing confusion
5. **Loki/Tempo datasources not auto-configured** - Manual Grafana configuration required
6. **Hardcoded Grafana admin password** - Security issue (`admin123` in git)
7. **Unused values file** - `kube-prometheus-stack-values-staging.yaml` not referenced by ArgoCD

### Medium Priority Issues (P3)

8. **Missing ServiceMonitors** - preprocessor-service, model-downloader lack metrics
9. **No recording rules** - Performance optimization opportunity
10. **Documentation dates outdated** - Several docs show past dates in `last_updated`
11. **Documentation gaps** - Missing coverage of staging differences

### Low Priority Issues (P4)

12. **api-router.json dashboard location** - Service-specific dashboard co-located with Helm chart (evaluate pattern)
13. **docs-humans dashboard** - Purpose unclear (example? documentation?)

---

## 9. Recommendations

### Immediate Actions

1. **Deploy Tempo to staging** - Create `gitops/clusters/staging/apps/tempo.yaml`
2. **Deploy loki-alerts to staging** - Create `gitops/clusters/staging/apps/observability/loki-alerts.yaml`
3. **Deploy Alertmanager config** - Create ArgoCD Application for `infra/k8s/monitoring/alerts/alertmanager-config.yaml`
4. **Configure Loki/Tempo datasources** - Add to kube-prometheus-stack values or create separate ConfigMap
5. **Move Grafana password to Secret** - Remove hardcoded password from git

### Organizational Improvements

6. **Consolidate dashboards** - Move or delete orphaned dashboards in `/dashboards/`
7. **Document dashboard policy** - Decide: centralized vs. service-specific dashboard location
8. **Remove unused values file** - Delete or reference `kube-prometheus-stack-values-staging.yaml`
9. **Update documentation dates** - Set `last_updated` to current date

### Feature Enhancements

10. **Add preprocessor-service metrics** - Implement Prometheus endpoint and ServiceMonitor
11. **Add model-downloader metrics** - Job progress tracking
12. **Create recording rules** - Pre-aggregate frequently queried metrics
13. **Document cross-environment differences** - Add staging/dev comparison to docs

---

## 10. File Locations Reference

### Dashboards
- **Canonical**: `/infra/k8s/monitoring/dashboards/` (12 dashboards, deployed via Kustomize)
- **Legacy**: `/dashboards/grafana/` (4 dashboards, NOT deployed)
- **Legacy**: `/dashboards/analytics/` (1 dashboard, NOT deployed)
- **Service-specific**: `/services/api-router-service/deployments/helm/api-router-service/dashboards/` (1 dashboard, NOT deployed)
- **Docs**: `/docs-humans/dashboards/` (1 dashboard, example/docs)

### ServiceMonitors
- `services/api-router-service/deployments/helm/api-router-service/templates/servicemonitor.yaml`
- `services/admin-api-service/deployments/helm/admin-api-service/templates/servicemonitor.yaml`
- `services/analytics-service/deployments/helm/analytics-service/templates/servicemonitor.yaml`
- `services/user-org-service/configs/helm/templates/servicemonitor.yaml`

### Prometheus Configuration
- `infra/helm/charts/kube-prometheus-stack-values-staging.yaml` (NOT USED)
- `gitops/clusters/staging/apps/kube-prometheus-stack.yaml` (inline values)
- `gitops/clusters/development/apps/kube-prometheus-stack.yaml` (development config - compare)

### Alert Rules
- `infra/k8s/prometheus-rules/staging/platform-alerts.yaml` (deployed)
- `infra/k8s/prometheus-rules/development/platform-alerts.yaml` (deployed)
- `infra/k8s/monitoring/alerts/loki-alerts.yaml` (NOT deployed to staging)
- `infra/k8s/monitoring/alerts/analytics-rollup-alerts.yaml` (NOT deployed to staging)
- `infra/k8s/monitoring/alerts/alertmanager-config.yaml` (NOT deployed)

### ArgoCD Applications
- `gitops/clusters/staging/apps/kube-prometheus-stack.yaml`
- `gitops/clusters/staging/apps/monitoring-dashboards.yaml`
- `gitops/clusters/staging/apps/loki.yaml`
- `gitops/clusters/staging/apps/observability/prometheus-alerts.yaml`
- `gitops/clusters/development/apps/observability/loki-alerts.yaml` (MISSING IN STAGING)
- `gitops/clusters/development/apps/tempo.yaml` (MISSING IN STAGING)

### Documentation
- `docs/architecture/observability-architecture.md`
- `docs/runbooks/ai-debugging-workflow.md`
- `docs/platform/agent-infra-ops-manager.md` (may need updates)

---

## Appendix: Metrics Exposed by Services

Based on ServiceMonitor configurations and alert rules:

### API Router Service
- `http_requests_total{job="api-router-service", status}` - Request counter
- `http_request_duration_seconds_bucket{job="api-router-service"}` - Latency histogram

### Admin API Service
- `http_requests_total{job="admin-api-service", status}` - Request counter

### Analytics Service
- `analytics_rollup_failures_total{rollup_type}` - Rollup failure counter
- `analytics_rollup_successes_total{rollup_type}` - Rollup success counter
- `analytics_rollup_last_success_timestamp{rollup_type}` - Last success timestamp
- `up{job="analytics-service"}` - Service health

### vLLM (InferenceServices)
- `vllm_request_duration_seconds_bucket` - Request latency histogram
- `vllm_gpu_memory_used_bytes` - GPU memory usage
- `vllm_gpu_memory_total_bytes` - GPU memory total

### Guidellm Runner
- `guidellm_runner_up` - Runner health
- `guidellm_requests_total` - Benchmark request counter

---

**End of Audit Report**
