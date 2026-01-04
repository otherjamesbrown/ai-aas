---
title: Routing Policy Drift Troubleshooting
last_updated: 2026-01-04
document_type: runbook
status: active
---

# Runbook: Routing Policy Drift Troubleshooting

**Feature**: Routing Policy Drift Detection & Auto-Healing
**Last Updated**: 2026-01-04
**Audience**: Platform Engineers, SRE

---

## Overview

### What is Routing Policy Drift?

Routing policy drift occurs when the `backend_id` in a routing policy (stored in the Admin API database) no longer matches the actual `InferenceServiceName` in Kubernetes. This mismatch causes requests to be routed to non-existent or incorrect backend services.

### Why Drift Matters

When drift occurs:
- API requests return 404 errors ("model not found")
- Requests may route to `localhost:8001` fallback (connection refused)
- 503 errors occur when the backend is unreachable
- Users experience model unavailability despite the model being deployed and ready

### How Drift Happens

Common causes of drift:
1. **InferenceService recreation** - KServe recreates the InferenceService with a different name suffix
2. **Manual policy edits** - Someone updates the routing policy without updating the backend_id
3. **Operator restarts during transition** - Operator misses the sync window during InferenceService creation
4. **Database restore** - Restoring from backup reintroduces stale backend_id values
5. **Failed auto-healing** - Admin API was unreachable when operator attempted to heal

### Automatic Healing

The ai-model-operator includes automatic drift detection and healing:
- **Detection**: Runs during every reconciliation loop (~30 seconds)
- **Healing**: Automatically updates routing policies when AIModel is Ready
- **Best-effort**: Healing failures don't block reconciliation

---

## Quick Reference Table

| Alert / Symptom | Likely Cause | Diagnostic Commands | Resolution |
|-----------------|--------------|---------------------|------------|
| `routing_policy_drift_detected_total` increasing | InferenceService recreated | `kubectl get aimodel -o yaml`, check `status.inferenceServiceName` | Wait for auto-heal or manual update |
| 404 "model not found" from vLLM | backend_id mismatch | `ai-aas-cli routing policy get --global --model <name>` | Update routing policy backend_id |
| 503 from api-router | Backend unreachable | Check BACKEND_ENDPOINTS mapping | Update BACKEND_ENDPOINTS in Helm values |
| `routing_policy_heal_attempts_total{result="failure"}` | Admin API unreachable | Check operator logs for API errors | Restart operator after Admin API recovered |

---

## Symptoms

### How Drift Manifests

1. **API returns 404 "model not found"**
   - vLLM receives a request for a model name that doesn't match its served model
   - Routing policy's `backend_id` doesn't match what vLLM is serving

2. **API returns 503 Service Unavailable**
   - api-router cannot reach the backend
   - BACKEND_ENDPOINTS doesn't contain the policy's `backend_id`
   - Falls back to `localhost:8001` which is unreachable

3. **Requests route to wrong model**
   - Rare but possible if backend_id points to a different model's InferenceService
   - Usually causes immediate 404 from vLLM

4. **Metrics show drift detected but not healed**
   - `routing_policy_drift_detected_total` increases
   - `routing_policy_heal_attempts_total{result="failure"}` increases
   - Admin API may be unreachable or authentication failing

---

## Detection

### Grafana Dashboard Queries

**Drift Detection Rate:**
```promql
rate(routing_policy_drift_detected_total[5m])
```

**Total Drift Events (last hour):**
```promql
sum(increase(routing_policy_drift_detected_total[1h])) by (model_name, namespace)
```

**Failed Healing Attempts:**
```promql
sum(increase(routing_policy_heal_attempts_total{result="failure"}[1h])) by (model_name)
```

**Successful Healing:**
```promql
sum(increase(routing_policy_heal_attempts_total{result="success"}[1h])) by (model_name)
```

**Drift Duration (time between detection and healing):**
```promql
histogram_quantile(0.95, rate(routing_policy_drift_duration_seconds_bucket[5m]))
```

### Loki Queries for Drift Logs

**Find drift detection events:**
```logql
{namespace="ai-model-operator-system"} | json | msg="Routing policy drift detected"
```

**Find healing events:**
```logql
{namespace="ai-model-operator-system"} | json | msg=~".*healed routing policy.*|.*heal.*routing.*"
```

**Find healing failures:**
```logql
{namespace="ai-model-operator-system"} | json | level="error" | msg=~".*heal.*|.*routing policy.*"
```

**Drift events for specific model:**
```logql
{namespace="ai-model-operator-system"} | json | msg=~".*drift.*" | model="<model-name>"
```

**All drift-related logs (verbose):**
```logql
{namespace="ai-model-operator-system"} | json | msg=~".*drift.*|.*heal.*routing.*"
```

### kubectl Commands

**Check AIModel status and InferenceServiceName:**
```bash
kubectl get aimodel <model-name> -n <namespace> -o yaml | grep -A 20 "status:"
```

**List all AIModels with their InferenceServiceName:**
```bash
kubectl get aimodel -A -o jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.inferenceServiceName}{"\n"}{end}'
```

**Check InferenceService exists:**
```bash
kubectl get inferenceservice -n <namespace> | grep <model-name>
```

**Verify InferenceService name matches AIModel status:**
```bash
# Get expected name from AIModel
EXPECTED=$(kubectl get aimodel <model-name> -n <namespace> -o jsonpath='{.status.inferenceServiceName}')

# Check if InferenceService exists
kubectl get inferenceservice $EXPECTED -n <namespace>
```

**Check operator logs for drift events:**
```bash
kubectl logs -n ai-model-operator-system deploy/ai-model-operator-controller-manager --tail=100 | grep -i drift
```

---

## Manual Resolution

### Step 1: Identify the Correct Backend Service Name

```bash
# Get the actual InferenceService name from AIModel status
kubectl get aimodel <model-name> -n <namespace> -o jsonpath='{.status.inferenceServiceName}'
# Example output: my-model-isvc-00001

# Verify the InferenceService exists and is Ready
kubectl get inferenceservice <inference-service-name> -n <namespace>
```

### Step 2: Check Current Routing Policy

```bash
# List policies for the model
ai-aas-cli routing policy list --model <external-model-name>

# Get the global policy
ai-aas-cli routing policy get --global --model <external-model-name>
```

**Look for the `backend_id` field** - this should match the InferenceServiceName.

### Step 3: Update Routing Policy via Admin API

**Option A: Using CLI (recommended)**
```bash
# Delete and recreate with correct backend_id
ai-aas-cli routing policy delete --global --model <external-model-name>

ai-aas-cli routing policy create \
  --global \
  --model <external-model-name> \
  --backends <correct-inference-service-name>:100
```

**Option B: Using Admin API directly**
```bash
# Get the policy ID first
POLICY_ID=$(curl -s \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  "$ADMIN_API_ENDPOINT/v1/routing/policies?model=<external-model-name>&organization_id=*" \
  | jq -r '.policies[0].policy_id')

# Update the policy
curl -X PATCH \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  "$ADMIN_API_ENDPOINT/v1/routing/policies/$POLICY_ID" \
  -d '{
    "backends": [
      {"backend_id": "<correct-inference-service-name>", "weight": 100}
    ]
  }'
```

### Step 4: Trigger Operator Reconciliation

If auto-healing didn't work, manually trigger reconciliation:

```bash
# Add an annotation to force reconciliation
kubectl annotate aimodel <model-name> -n <namespace> \
  ai-aas.otherjamesbrown.com/force-reconcile=$(date +%s) --overwrite

# Or restart the operator
kubectl rollout restart deployment ai-model-operator-controller-manager \
  -n ai-model-operator-system
```

### Step 5: Update BACKEND_ENDPOINTS (if needed)

If the api-router's BACKEND_ENDPOINTS doesn't include the new InferenceService name:

```bash
# Check current BACKEND_ENDPOINTS
kubectl get deployment -n <api-router-namespace> api-router-service-<env>-api-router-service \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="BACKEND_ENDPOINTS")].value}'
```

**Update via Helm values (GitOps):**
```yaml
# services/api-router-service/deployments/helm/api-router-service/values-<environment>.yaml
backends:
  endpoints: "<backend-id>:http://<inference-service>.<namespace>.svc.cluster.local:8012"
```

Commit, push, and wait for ArgoCD to sync.

---

## Verification

### Confirm Drift is Resolved

**1. Check routing policy has correct backend_id:**
```bash
ai-aas-cli routing policy get --global --model <external-model-name>
# backend_id should match InferenceServiceName
```

**2. Test API request:**
```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "<external-model-name>",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**3. Check operator logs show no drift:**
```bash
kubectl logs -n ai-model-operator-system deploy/ai-model-operator-controller-manager --tail=20 \
  | grep -i "drift detected: false\|No routing policy drift"
```

**4. Verify metrics:**
```promql
# Should show 0 or decreasing
rate(routing_policy_drift_detected_total{model_name="<model-name>"}[5m])
```

**5. Run pipeline diagnostic:**
```bash
ai-aas-cli model pipeline <model-name> -e <environment>
```

---

## Prevention Checklist

### Best Practices to Avoid Drift

1. **Never manually edit routing policies**
   - Let the operator manage backend_id
   - Use the Admin API or CLI for controlled updates

2. **Monitor drift metrics**
   - Set up alerts on `routing_policy_drift_detected_total`
   - Alert on `routing_policy_heal_attempts_total{result="failure"}` > 0

3. **Ensure Admin API availability**
   - Auto-healing requires Admin API to be reachable
   - Monitor Admin API health and uptime

4. **Use consistent InferenceService naming**
   - Avoid manual InferenceService edits
   - Let the operator manage InferenceService lifecycle

5. **After InferenceService recreation:**
   - Wait for operator reconciliation (~30 seconds)
   - Verify routing policy was auto-healed

6. **After database restore:**
   - Run operator reconciliation for all AIModels
   - Or restart the operator to trigger full reconciliation

### Recommended Alerts

**Drift Detected Alert:**
```yaml
- alert: RoutingPolicyDriftDetected
  expr: increase(routing_policy_drift_detected_total[5m]) > 0
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "Routing policy drift detected for {{ $labels.model_name }}"
    description: "Drift between routing policy and InferenceService. Auto-healing should resolve within 1 minute."
```

**Healing Failure Alert:**
```yaml
- alert: RoutingPolicyHealingFailed
  expr: increase(routing_policy_heal_attempts_total{result="failure"}[10m]) > 3
  for: 10m
  labels:
    severity: critical
  annotations:
    summary: "Routing policy auto-healing failing for {{ $labels.model_name }}"
    description: "Multiple healing attempts failed. Check Admin API connectivity and operator logs."
```

**Extended Drift Duration Alert:**
```yaml
- alert: RoutingPolicyDriftPersistent
  expr: routing_policy_drift_duration_seconds > 300
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Routing policy drift persisting for > 5 minutes"
    description: "Auto-healing not resolving drift. Manual intervention may be required."
```

---

## Escalation Matrix

| Severity | Trigger | Escalate To | Response SLA |
|----------|---------|-------------|--------------|
| Sev 1 | Production model inaccessible, auto-healing failing | Platform Director + On-call SRE | 5 minutes |
| Sev 2 | Drift detected, healing in progress but slow | Platform On-call | 15 minutes |
| Sev 3 | Drift metrics elevated in staging/dev | Platform Engineer | 1 hour |
| Sev 4 | Occasional drift auto-healed, no user impact | Backlog | Next business day |

---

## Related Documentation

- [Model Not Accessible](./model-not-accessible.md) - General model accessibility troubleshooting
- [Configure Routing Policies](./configure-routing-policies.md) - Routing policy configuration guide
- [AI Debugging Workflow](./ai-debugging-workflow.md) - General debugging with observability stack
- [Infrastructure Troubleshooting](./infrastructure-troubleshooting.md) - General infrastructure issues

### Source Code References

- Drift detection: `operators/ai-model-operator/controllers/aimodel_controller.go` - `detectRoutingPolicyDrift()`
- Auto-healing: `operators/ai-model-operator/controllers/aimodel_controller.go` - `healRoutingPolicy()`
- Metrics: `routing_policy_drift_detected_total`, `routing_policy_heal_attempts_total`, `routing_policy_drift_duration_seconds`

---

## Appendix: Metrics Reference

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `routing_policy_drift_detected_total` | Counter | `model_name`, `namespace` | Total drift events detected |
| `routing_policy_drift_duration_seconds` | Histogram | `model_name`, `namespace` | Time between drift detection and resolution |
| `routing_policy_heal_attempts_total` | Counter | `model_name`, `namespace`, `result` | Healing attempts (result: success/failure) |
| `routing_policy_heal_duration_seconds` | Histogram | `model_name`, `namespace` | Duration of healing operations |
