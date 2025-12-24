# Quickstart: Logging & Observability

**Feature**: 024-logging-observability-improvements
**Time to complete**: ~30 minutes

## Prerequisites

- [ ] Kubernetes cluster access (kubeconfig configured)
- [ ] ArgoCD access for deployment
- [ ] kubectl and curl installed
- [ ] Grafana already deployed (verify: `kubectl get svc -n monitoring grafana`)

## Step 1: Deploy Loki

### 1.1 Create Loki manifests

```bash
mkdir -p infra/k8s/monitoring/loki
```

Create the StatefulSet, ConfigMap, Service, and Ingress per `architecture.md`.

### 1.2 Create ArgoCD Application

```yaml
# gitops/clusters/development/apps/loki.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: loki
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: develop
    path: infra/k8s/monitoring/loki
  destination:
    server: https://kubernetes.default.svc
    namespace: monitoring
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

### 1.3 Deploy and verify

```bash
# Commit and push
git add infra/k8s/monitoring/loki gitops/clusters/development/apps/loki.yaml
git commit -m "feat(observability): Add Loki deployment"
git push

# Wait for ArgoCD sync (or manual sync)
# Verify pod is running
kubectl get pods -n monitoring -l app=loki

# Test Loki API
curl -s https://loki.dev.otherjamesbrown.com/ready
# Expected: "ready"
```

## Step 2: Deploy Promtail

### 2.1 Create Promtail manifests

```bash
mkdir -p infra/k8s/monitoring/promtail
```

Create DaemonSet, ConfigMap, ServiceAccount, ClusterRole per `architecture.md`.

### 2.2 Create ArgoCD Application and deploy

```bash
git add infra/k8s/monitoring/promtail gitops/clusters/development/apps/promtail.yaml
git commit -m "feat(observability): Add Promtail deployment"
git push

# Verify DaemonSet running on all nodes
kubectl get pods -n monitoring -l app=promtail
```

### 2.3 Verify log collection

```bash
# Check Promtail is shipping logs
kubectl logs -n monitoring -l app=promtail --tail=20 | grep -i "loki"

# Query Loki for recent logs
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={}' \
  --data-urlencode 'limit=10' | jq '.data.result | length'
# Expected: > 0
```

## Step 3: Configure Grafana

### 3.1 Add Loki datasource

In Grafana UI (`https://grafana.dev.otherjamesbrown.com`):

1. Go to **Configuration > Data Sources**
2. Click **Add data source**
3. Select **Loki**
4. URL: `http://loki.monitoring.svc.cluster.local:3100`
5. Click **Save & Test**

Or via API:

```bash
curl -X POST https://grafana.dev.otherjamesbrown.com/api/datasources \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Loki",
    "type": "loki",
    "url": "http://loki.monitoring.svc.cluster.local:3100",
    "access": "proxy"
  }'
```

### 3.2 Test in Grafana Explore

1. Go to **Explore**
2. Select **Loki** datasource
3. Enter query: `{namespace="default"}`
4. Should see logs from platform services

## Step 4: Add Request Logger Middleware

### 4.1 Add middleware to a service

In `services/api-router-service/cmd/router/main.go`:

```go
import "github.com/ai-aas/shared/go/middleware"

// In router setup
r.Use(middleware.RequestLogger(logger, middleware.DefaultConfig()))
```

### 4.2 Deploy and verify

```bash
# Commit change
git add services/api-router-service
git commit -m "feat(api-router): Add request logger middleware"
git push

# After deployment, make a test request
curl -i https://api.dev.otherjamesbrown.com/v1/models

# Check logs in Loki
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={service="api-router-service"} |= "request_completed"' \
  --data-urlencode 'limit=5' | jq '.data.result[].values[][1]'
```

## Step 5: Verify vLLM Log Collection

### 5.1 Check inference backend logs

```bash
# Query vLLM logs
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"}' \
  --data-urlencode 'limit=20' | jq '.data.result'

# Check for GPU errors
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={container=~"vllm|kserve-container"} |~ "(?i)cuda|oom"' \
  --data-urlencode 'limit=10' | jq '.data.result'
```

## Step 6: Create Basic Dashboard

### 6.1 Import dashboard JSON

Create `infra/k8s/monitoring/grafana/dashboards/logs-overview.json` with panels for:
- Log volume by service
- Error rate
- Recent errors table

### 6.2 Verify dashboard

1. Go to Grafana **Dashboards**
2. Find "AI-AAS Service Logs" dashboard
3. Verify data is populated

## Verification Checklist

- [ ] Loki pod running: `kubectl get pods -n monitoring -l app=loki`
- [ ] Promtail DaemonSet running: `kubectl get ds -n monitoring promtail`
- [ ] Loki API responds: `curl https://loki.dev.otherjamesbrown.com/ready`
- [ ] Logs visible in Grafana Explore
- [ ] Platform service logs have `service` label
- [ ] vLLM logs have `container` label
- [ ] Request logs include `request_id` field
- [ ] Errors include `request_body_sample` field

## Troubleshooting

### No logs in Loki

```bash
# Check Promtail is scraping
kubectl logs -n monitoring -l app=promtail | grep -i "error\|failed"

# Check Loki is receiving
kubectl logs -n monitoring -l app=loki | grep -i "push"
```

### Missing service label

Ensure Promtail config extracts from pod label:
```yaml
- source_labels: [__meta_kubernetes_pod_label_app]
  target_label: service
```

### vLLM logs not parsed

Check regex patterns match actual vLLM output:
```bash
kubectl logs -n system -l app=vllm --tail=50
```

## Next Steps

1. [ ] Add alerting rules (Phase 4)
2. [ ] Create service-specific dashboards (Phase 4)
3. [ ] Integrate Sentry for frontend (Phase 3)
4. [ ] Update CLAUDE.md with debugging guide (Phase 5)
