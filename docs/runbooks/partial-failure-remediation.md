# Runbook: Partial Failure Remediation for vLLM Deployments

**Feature**: `010-vllm-deployment` (User Story 3 - Safe operations)
**Last Updated**: 2025-01-27
**Severity**: P1 (Production impact)

## Overview

This runbook provides step-by-step procedures for identifying and remediating partial failures in vLLM model deployments where some components are working but others are degraded or failing.

## Quick Reference

| Symptom | Likely Cause | Quick Fix |
|---------|--------------|-----------|
| Some pods running, others pending | GPU resource shortage | Scale down non-critical workloads |
| Some pods crash looping | OOM or model loading failure | Check logs, increase memory limits |
| Health checks intermittent | Network issues or slow responses | Check network, increase timeout |
| Registry shows disabled | Manual disable or failed deployment | Re-enable: `admin-cli registry enable` |
| High latency on some requests | Pod overload or GPU throttling | Scale up replicas |

## Partial Failure Scenarios

### Scenario 1: Mixed Pod Status

**Symptoms:**
```bash
$ kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system
NAME                                    READY   STATUS             RESTARTS   AGE
llama-2-7b-production-abc123           1/1     Running            0          5m
llama-2-7b-production-def456           0/1     CrashLoopBackOff   3          5m
llama-2-7b-production-ghi789           0/1     Pending            0          5m
```

**Impact:**
- Degraded capacity (33% healthy)
- Increased latency on healthy pods
- Potential service disruption

**Diagnosis:**

```bash
# Step 1: Check deployment status
kubectl describe deployment llama-2-7b-production -n system

# Step 2: Check failing pod logs
kubectl logs llama-2-7b-production-def456 -n system --tail=100

# Step 3: Check pending pod events
kubectl describe pod llama-2-7b-production-ghi789 -n system | grep Events -A 20

# Step 4: Check resource availability
kubectl describe nodes | grep -A 5 "Allocated resources"
```

**Remediation:**

**For CrashLoopBackOff:**
```bash
# Check for OOM errors
kubectl logs llama-2-7b-production-def456 -n system --previous | grep -i "out of memory"

# If OOM:
# 1. Increase memory limits in values file
# 2. Upgrade deployment:
helm upgrade llama-2-7b-production infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-production.yaml \
  --set resources.limits.memory=64Gi \
  --namespace system
```

**For Pending:**
```bash
# If GPU resources insufficient:
# Option 1: Scale down non-critical GPU workloads
kubectl scale deployment <non-critical-gpu-workload> --replicas=0 -n <namespace>

# Option 2: Add GPU nodes (if infrastructure supports)
# Contact infrastructure team

# Option 3: Reduce replica count temporarily
kubectl scale deployment llama-2-7b-production --replicas=1 -n system
```

**For Mixed Status:**
```bash
# If some pods healthy, route traffic only to healthy pods
# This is automatic via readiness probes, but verify:
kubectl get endpoints llama-2-7b-production -n system

# Should only list ready pods
```

**Decision Tree:**
```
1. Are >50% of pods healthy?
   YES → Continue with reduced capacity, fix failing pods
   NO  → Consider rollback if degradation is severe

2. Is the issue resource-related?
   YES → Free up resources or scale down
   NO  → Check logs for application errors

3. Can the issue be fixed with config change?
   YES → Apply config change via helm upgrade
   NO  → Rollback to previous version

4. Is production traffic affected?
   YES → Consider immediate rollback
   NO  → Fix forward with monitoring
```

---

### Scenario 2: Intermittent Health Check Failures

**Symptoms:**
```bash
$ kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system
NAME                                    READY   STATUS    RESTARTS   AGE
llama-2-7b-production-abc123           1/1     Running   5          10m
llama-2-7b-production-def456           1/1     Running   3          10m
```

Pods are running but restarting frequently due to failed health checks.

**Impact:**
- Service interruptions during restarts
- Increased error rates
- Degraded performance

**Diagnosis:**

```bash
# Step 1: Check pod events for health check failures
kubectl describe pod llama-2-7b-production-abc123 -n system | grep -A 10 "Liveness\|Readiness"

# Step 2: Test health endpoint manually
kubectl run test-health --rm -i --restart=Never -n system \
  --image=curlimages/curl:latest -- \
  curl -v http://llama-2-7b-production.system.svc.cluster.local:8000/health

# Step 3: Check pod logs for errors
kubectl logs llama-2-7b-production-abc123 -n system | grep -i "error\|exception\|timeout"

# Step 4: Check resource utilization
kubectl top pod llama-2-7b-production-abc123 -n system
```

**Remediation:**

**If health check timeout too short:**
```bash
# Increase health check timeout
helm upgrade llama-2-7b-production infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-production.yaml \
  --set livenessProbe.timeoutSeconds=10 \
  --set readinessProbe.timeoutSeconds=10 \
  --namespace system
```

**If model is slow to respond:**
```bash
# Check if GPU is being utilized
kubectl exec -it llama-2-7b-production-abc123 -n system -- nvidia-smi

# If GPU utilization is low, model may be CPU-bound
# Increase CPU allocation:
helm upgrade llama-2-7b-production infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-production.yaml \
  --set resources.requests.cpu=8 \
  --set resources.limits.cpu=16 \
  --namespace system
```

**If network issues:**
```bash
# Test network connectivity from another pod
kubectl run test-network --rm -i --restart=Never -n system \
  --image=nicolaka/netshoot -- \
  ping llama-2-7b-production.system.svc.cluster.local

# Check DNS resolution
kubectl run test-dns --rm -i --restart=Never -n system \
  --image=nicolaka/netshoot -- \
  nslookup llama-2-7b-production.system.svc.cluster.local
```

---

### Scenario 3: Registry Status Mismatch

**Symptoms:**
```bash
$ admin-cli deployment status --model-name llama-2-7b --environment production
Model: llama-2-7b
Status: disabled  ← Model is disabled but deployment is healthy
Endpoint: llama-2-7b-production.system.svc.cluster.local:8000

$ kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system
NAME                                    READY   STATUS    RESTARTS   AGE
llama-2-7b-production-abc123           1/1     Running   0          10m
```

Deployment is healthy but registry shows model as disabled.

**Impact:**
- No traffic routed to healthy deployment
- Wasted resources
- Service appears down to clients

**Diagnosis:**

```bash
# Step 1: Check registry status
admin-cli registry list --environment production | grep llama-2-7b

# Step 2: Check deployment health
kubectl get deployment llama-2-7b-production -n system

# Step 3: Test endpoint directly
curl http://llama-2-7b-production.system.svc.cluster.local:8000/health
```

**Remediation:**

```bash
# If deployment is healthy but registry shows disabled:
# Re-enable the model
admin-cli registry enable \
  --model-name llama-2-7b \
  --environment production

# Verify
admin-cli registry list --environment production | grep llama-2-7b

# Test routing through API Router
curl -X POST http://api-router:8080/v1/completions \
  -H 'Authorization: Bearer <api-key>' \
  -H 'Content-Type: application/json' \
  -d '{"model": "llama-2-7b", "prompt": "test", "max_tokens": 1}'
```

**If endpoint mismatch:**
```bash
# Re-register with correct endpoint
admin-cli registry register \
  --model-name llama-2-7b \
  --endpoint llama-2-7b-production.system.svc.cluster.local:8000 \
  --environment production \
  --namespace system
```

---

### Scenario 4: Partial Rollback Needed

**Symptoms:**
- Rollback started but not completed
- Model still disabled after rollback
- Pods are healthy but not routing

**Diagnosis:**

```bash
# Check Helm release status
helm list -n system | grep llama-2-7b-production

# Check release history
helm history llama-2-7b-production -n system

# Check registry status
admin-cli registry list --environment production | grep llama-2-7b
```

**Remediation:**

```bash
# Complete the rollback
# 1. Verify deployment is healthy
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system

# 2. Re-enable in registry
admin-cli registry enable \
  --model-name llama-2-7b \
  --environment production

# 3. Clear Redis cache (force refresh)
kubectl exec -it <redis-pod> -n system -- redis-cli FLUSHDB

# 4. Verify routing works
admin-cli deployment status --model-name llama-2-7b --environment production
```

---

## General Remediation Procedures

### Procedure 1: Graceful Degradation

When partial failures occur, maintain service with reduced capacity:

```bash
# 1. Identify healthy pods
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system | grep Running

# 2. Ensure service only routes to healthy pods (automatic via readiness)
kubectl get endpoints llama-2-7b-production -n system

# 3. Monitor traffic distribution
# (Use API Router metrics or Prometheus)

# 4. Scale down to healthy replicas if needed
HEALTHY_COUNT=$(kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system | grep -c "1/1.*Running")
kubectl scale deployment llama-2-7b-production --replicas=$HEALTHY_COUNT -n system

# 5. Fix failing components while maintaining service
```

### Procedure 2: Identify Root Cause

```bash
# Check all potential failure points
echo "=== Pod Status ==="
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system

echo "=== Recent Events ==="
kubectl get events -n system --sort-by='.lastTimestamp' | grep llama-2-7b-production | tail -20

echo "=== Pod Logs (errors) ==="
kubectl logs -l app.kubernetes.io/instance=llama-2-7b-production -n system --tail=100 | grep -i "error\|exception\|fatal"

echo "=== Resource Usage ==="
kubectl top pods -l app.kubernetes.io/instance=llama-2-7b-production -n system

echo "=== Node Status ==="
kubectl get nodes -o wide

echo "=== Registry Status ==="
admin-cli registry list --environment production | grep llama-2-7b
```

### Procedure 3: Progressive Remediation

Fix issues incrementally to avoid compounding problems:

```bash
# 1. Stabilize (stop the bleeding)
#    - Disable auto-scaling if causing issues
#    - Route traffic only to healthy pods
#    - Increase resource limits if OOM

# 2. Isolate (identify the problem)
#    - Check logs
#    - Test health endpoints
#    - Verify configuration

# 3. Remediate (fix the issue)
#    - Apply configuration changes
#    - Restart failing pods
#    - Upgrade deployment if needed

# 4. Validate (ensure fix works)
#    - Test all endpoints
#    - Monitor metrics
#    - Verify registry status

# 5. Scale (return to normal capacity)
#    - Scale up to desired replicas
#    - Enable auto-scaling
#    - Monitor for stability
```

---

## Rollback Decision Matrix

| Condition | Healthy Pods | Error Rate | Decision |
|-----------|--------------|------------|----------|
| < 25% healthy | 0-25% | Any | **Immediate rollback** |
| 25-50% healthy | 25-50% | > 10% | **Rollback recommended** |
| 50-75% healthy | 50-75% | 5-10% | Fix forward with monitoring |
| > 75% healthy | > 75% | < 5% | Fix forward, investigate failures |

---

## Monitoring and Alerts

### Key Metrics to Watch

```promql
# Pod availability
sum(kube_pod_status_ready{namespace="system", pod=~"llama-2-7b-production-.*"}) /
sum(kube_pod_status_phase{namespace="system", pod=~"llama-2-7b-production-.*"})

# Error rate
rate(vllm_errors_total{model="llama-2-7b", environment="production"}[5m])

# P95 latency
histogram_quantile(0.95, rate(vllm_request_duration_seconds_bucket{model="llama-2-7b"}[5m]))

# Restart count
rate(kube_pod_container_status_restarts_total{namespace="system", pod=~"llama-2-7b-production-.*"}[10m])
```

### Alert Thresholds

```yaml
# Partial failure alert
- alert: VLLMPartialFailure
  expr: |
    (sum(kube_pod_status_ready{namespace="system", pod=~"llama-2-7b-production-.*"}) /
     sum(kube_pod_status_phase{namespace="system", pod=~"llama-2-7b-production-.*"})) < 0.75
  for: 5m
  annotations:
    summary: "vLLM deployment {{ $labels.pod }} has < 75% pods ready"
    runbook: "docs/runbooks/partial-failure-remediation.md"
```

---

## Post-Incident

### After Remediation Checklist

- [ ] Verify all pods are healthy
- [ ] Confirm registry status is correct
- [ ] Test end-to-end routing through API Router
- [ ] Check error rates returned to baseline
- [ ] Verify latency within SLOs
- [ ] Document root cause
- [ ] Create postmortem if production impacted
- [ ] Update runbook if new failure mode discovered

### Postmortem Template

```markdown
# Postmortem: Partial vLLM Deployment Failure

**Date**: YYYY-MM-DD
**Duration**: X hours
**Impact**: Degraded service, X% of requests affected

## Summary
Brief description of what happened.

## Timeline
- HH:MM - Issue detected
- HH:MM - Investigation started
- HH:MM - Root cause identified
- HH:MM - Remediation applied
- HH:MM - Service restored

## Root Cause
Detailed explanation of what caused the issue.

## Resolution
Steps taken to resolve the issue.

## Action Items
- [ ] Prevent similar issues (link to ticket)
- [ ] Update monitoring/alerts (link to ticket)
- [ ] Update documentation (link to PR)
```

---

## See Also

- [Rollback Workflow](../rollback-workflow.md)
- [Rollout Workflow](../rollout-workflow.md)
- [Deployment Status Inspection](../vllm-registration-workflow.md#deployment-status)
- [Environment Separation](../environment-separation.md)
