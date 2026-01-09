---
title: Alertmanager Setup and Configuration
last_updated: 2026-01-09
status: Active
environment: all
---

# Alertmanager Setup and Configuration

## Overview

Alertmanager is deployed as part of kube-prometheus-stack and handles alert routing, grouping, and notification delivery to Slack and PagerDuty.

## Configuration Files

| File | Purpose |
|------|---------|
| `gitops/clusters/staging/apps/kube-prometheus-stack.yaml` | Staging Alertmanager config (inline in ArgoCD Application) |
| `gitops/clusters/development/apps/observability/values-configmap.yaml` | Development Alertmanager config (basic) |
| `infra/k8s/monitoring/alerts/alertmanager-config.yaml` | Template/reference configuration (not deployed) |

## Alert Routing Rules

### Staging Environment

Alerts are routed based on severity and category:

| Condition | Receiver | Slack Channel | Notes |
|-----------|----------|---------------|-------|
| `severity: critical` | pagerduty-critical | #platform-critical | Also sends to PagerDuty |
| `category: inference` | slack-ai-models | #ai-models | vLLM/model alerts |
| `alert_type: gpu-error` | slack-ai-models | #ai-models | GPU/CUDA errors |
| `category: infrastructure` | slack-platform-infra | #platform-infra | Infra warnings |
| `category: database\|connectivity` | slack-platform-infra | #platform-infra | DB/network issues |
| `category: security` | slack-platform-infra | #platform-infra | Auth/security alerts |
| `category: observability` | slack-platform-infra | #platform-infra | Monitoring health (low priority) |
| Default | default-receiver | #platform-alerts | Catch-all |

### Alert Grouping

- **Default grouping**: `['alertname', 'severity', 'service']`
- **Inference/GPU alerts**: `['alertname', 'inferenceservice', 'pod']` or `['alertname', 'node', 'pod']`
- **Group wait**: 30s (wait before sending first notification)
- **Group interval**: 5m (wait before sending subsequent alerts from same group)
- **Repeat interval**: 4h (resend alert if still firing)

### Inhibition Rules

Suppress redundant alerts:

1. **Critical suppresses warning**: If critical alert is firing, suppress warnings for same alertname+service
2. **ServiceDown suppresses HighServiceErrorRate**: If service is down, don't also alert on high error rate
3. **LokiIngestionDown suppresses LokiIngestionSlowdown**: If Loki is down, don't alert on slowdown

## Required Secrets

### Staging

The `alertmanager-secrets` secret must exist in the `monitoring` namespace before ArgoCD syncs kube-prometheus-stack.

**Create the secret:**

```bash
# Set your Slack webhook and PagerDuty key
SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
PAGERDUTY_KEY="your-pagerduty-integration-key"

kubectl create secret generic alertmanager-secrets \
  --from-literal=slack-webhook-url="$SLACK_WEBHOOK" \
  --from-literal=pagerduty-service-key="$PAGERDUTY_KEY" \
  -n monitoring \
  --kubeconfig=secrets/kubeconfigs/kubeconfig-staging.yaml
```

**Verify the secret:**

```bash
kubectl get secret alertmanager-secrets -n monitoring \
  --kubeconfig=secrets/kubeconfigs/kubeconfig-staging.yaml
```

### Development

Development uses a basic Alertmanager config defined in `gitops/clusters/development/apps/observability/values-configmap.yaml`. It sends all alerts to #platform-infra.

To add Slack notifications to development, update the ConfigMap with:

```bash
# Create the secret (same as staging)
kubectl create secret generic alertmanager-secrets \
  --from-literal=slack-webhook-url="$SLACK_WEBHOOK" \
  --from-literal=pagerduty-service-key="$PAGERDUTY_KEY" \
  -n monitoring \
  --kubeconfig=secrets/kubeconfigs/kubeconfig-development.yaml
```

## Obtaining Credentials

### Slack Webhook

1. Go to your Slack workspace settings
2. Navigate to **Apps** → **Custom Integrations** → **Incoming Webhooks**
3. Create a new webhook or use existing
4. Select the target channel (e.g., #platform-alerts, #platform-critical, #ai-models)
5. Copy the webhook URL (format: `https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXX`)

**Recommended channels to create:**
- `#platform-alerts` - Default alerts
- `#platform-critical` - Critical alerts (also sent to PagerDuty)
- `#ai-models` - Inference/GPU/model alerts
- `#platform-infra` - Infrastructure warnings

### PagerDuty Integration Key

1. Log in to PagerDuty
2. Go to **Services** → Select your service (or create "AI-AAS Platform")
3. Navigate to **Integrations** tab
4. Click **Add Integration**
5. Select **Events API v2**
6. Copy the **Integration Key**

## Deployment

### Staging

1. **Create the secret** (see above)
2. **Commit and push** the updated `kube-prometheus-stack.yaml`:
   ```bash
   git add gitops/clusters/staging/apps/kube-prometheus-stack.yaml
   git commit -m "feat(staging): deploy Alertmanager custom configuration"
   git push origin staging
   ```
3. **ArgoCD will auto-sync** (or manually sync via UI/CLI)
4. **Verify deployment**:
   ```bash
   kubectl get pods -n monitoring -l app.kubernetes.io/name=alertmanager
   kubectl logs -n monitoring alertmanager-kube-prometheus-stack-alertmanager-0 -c alertmanager
   ```

### Development

Development currently uses a basic config. To deploy the full routing configuration:

1. Update `gitops/clusters/development/apps/observability/values-configmap.yaml` (or create separate file)
2. Create the `alertmanager-secrets` secret
3. Commit and push to `develop` branch
4. ArgoCD will sync

## Verification

### Test Alertmanager Configuration

Check if the config is loaded correctly:

```bash
# Port-forward to Alertmanager
kubectl port-forward -n monitoring svc/kube-prometheus-stack-alertmanager 9093:9093

# Check status (in browser)
open http://localhost:9093/#/status

# Check config via API
curl http://localhost:9093/api/v2/status | jq
```

### Send Test Alert

Create a test PrometheusRule to trigger an alert:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: test-alert
  namespace: monitoring
spec:
  groups:
    - name: test
      interval: 30s
      rules:
        - alert: TestAlert
          expr: vector(1)
          labels:
            severity: warning
            category: infrastructure
          annotations:
            summary: "Test alert"
            description: "This is a test alert to verify Alertmanager routing"
```

Apply and watch for notification in Slack.

### Check Alert Routing

View current alerts and their routing:

```bash
# View all alerts
curl http://localhost:9093/api/v2/alerts | jq

# View silences
curl http://localhost:9093/api/v2/silences | jq
```

## Troubleshooting

### Alerts Not Sending to Slack

1. **Check Alertmanager logs**:
   ```bash
   kubectl logs -n monitoring alertmanager-kube-prometheus-stack-alertmanager-0 -c alertmanager --tail=100
   ```

2. **Verify secret is mounted**:
   ```bash
   kubectl exec -n monitoring alertmanager-kube-prometheus-stack-alertmanager-0 -c alertmanager -- ls -la /etc/alertmanager/secrets/alertmanager-secrets/
   ```

3. **Test Slack webhook manually**:
   ```bash
   curl -X POST -H 'Content-type: application/json' \
     --data '{"text":"Test message from Alertmanager"}' \
     "$SLACK_WEBHOOK"
   ```

4. **Check Alertmanager config**:
   ```bash
   kubectl get secret -n monitoring alertmanager-kube-prometheus-stack-alertmanager -o jsonpath='{.data.alertmanager\.yaml}' | base64 -d
   ```

### Configuration Not Applied

If ArgoCD sync succeeds but config doesn't change:

1. **Check ArgoCD Application sync status**:
   ```bash
   argocd app get kube-prometheus-stack-staging
   ```

2. **Force refresh and sync**:
   ```bash
   argocd app sync kube-prometheus-stack-staging --force
   ```

3. **Restart Alertmanager pods**:
   ```bash
   kubectl rollout restart statefulset -n monitoring alertmanager-kube-prometheus-stack-alertmanager
   ```

### PagerDuty Not Receiving Alerts

1. **Verify integration key** in secret
2. **Check PagerDuty service status** in PagerDuty web UI
3. **Test integration** using PagerDuty's test event feature
4. **Check Alertmanager logs** for PagerDuty API errors

## Alert Tuning

### Adjust Repeat Interval

To change how often alerts are resent, edit the `repeat_interval` in the route:

```yaml
route:
  repeat_interval: 4h  # Default: resend every 4 hours
```

### Add Custom Receiver

To add a new receiver (e.g., email):

```yaml
receivers:
  - name: 'email-ops'
    email_configs:
      - to: 'ops@example.com'
        from: 'alertmanager@ai-aas.example.com'
        smarthost: 'smtp.example.com:587'
        auth_username: 'alertmanager'
        auth_password_file: /etc/alertmanager/secrets/alertmanager-secrets/smtp-password
```

Add routing rule:

```yaml
routes:
  - match:
      team: ops
    receiver: 'email-ops'
```

## Related Documentation

- [Alertmanager Configuration Guide](https://prometheus.io/docs/alerting/latest/configuration/)
- [Alertmanager Routing Tree Editor](https://prometheus.io/webtools/alerting/routing-tree-editor/)
- [kube-prometheus-stack Helm Chart Values](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack)
- [AI-AAS Observability Architecture](../architecture/observability-architecture.md)
- [Prometheus Alerting Rules](./prometheus-alerting-rules.md)
