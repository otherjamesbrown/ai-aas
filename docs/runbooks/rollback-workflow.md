## vLLM Deployment Rollback Workflow

This document provides comprehensive guidance for rolling back vLLM model deployments using Helm.

**Feature**: `010-vllm-deployment` (User Story 3 - Safe operations)
**Last Updated**: 2025-01-27

## Overview

Rollbacks restore a vLLM deployment to a previous Helm revision when issues are detected. The rollback process is safe, automated, and includes registry status management to prevent routing to unhealthy deployments.

## When to Rollback

### Rollback Triggers

Initiate a rollback when you observe:

1. **Deployment Failures**
   - Pods crash looping or failing to start
   - Health checks consistently failing
   - Model initialization timeouts

2. **Performance Degradation**
   - Response latency exceeds SLOs (>3s P95)
   - Error rates increase significantly
   - GPU memory issues or OOM errors

3. **Functional Issues**
   - Model producing incorrect outputs
   - API compatibility broken
   - Configuration errors detected

4. **Validation Failures**
   - Post-deployment smoke tests fail
   - Integration tests fail
   - User-reported issues

### Rollback Decision Criteria

**DO rollback if:**
- Critical functionality is broken
- Error rates > 5% for > 5 minutes
- P95 latency > 2x baseline for > 10 minutes
- Complete service outage
- Data integrity issues

**DON'T rollback if:**
- Minor non-critical issues
- Issues can be fixed with config-only changes
- Problem affects < 1% of requests
- Rollback would cause more disruption

## Rollback Methods

### Method 1: Automated Rollback Script (Recommended)

Use the rollback script for safe, automated rollbacks with registry updates:

```bash
# Rollback to previous revision
./scripts/vllm/rollback-deployment.sh llama-2-7b-development

# Rollback to specific revision
./scripts/vllm/rollback-deployment.sh llama-2-7b-development 3

# Rollback in different namespace
./scripts/vllm/rollback-deployment.sh llama-2-7b-production 0 production
```

**What the script does:**
1. Shows release history
2. Confirms rollback target
3. Disables model in registry (prevents routing)
4. Performs Helm rollback
5. Waits for deployment to be healthy
6. Re-enables model in registry
7. Verifies success

### Method 2: Manual Helm Rollback

For more control or troubleshooting:

```bash
# Step 1: Check release history
helm history llama-2-7b-development -n system

# Output shows revisions:
# REVISION  STATUS      DESCRIPTION
# 1         superseded  Install complete
# 2         superseded  Upgrade complete
# 3         deployed    Upgrade complete

# Step 2: Disable model in registry (prevent routing during rollback)
admin-cli registry disable \
  --model-name llama-2-7b \
  --environment development

# Step 3: Perform rollback
helm rollback llama-2-7b-development 2 -n system --wait --timeout 10m

# Step 4: Verify pods are healthy
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-development -n system
kubectl wait --for=condition=Ready pod -l app.kubernetes.io/instance=llama-2-7b-development \
  -n system --timeout=300s

# Step 5: Test health endpoint
kubectl run test-health --rm -i --restart=Never -n system \
  --image=curlimages/curl:latest -- \
  curl -f http://llama-2-7b-development.system.svc.cluster.local:8000/health

# Step 6: Re-enable model in registry
admin-cli registry enable \
  --model-name llama-2-7b \
  --environment development

# Step 7: Verify deployment status
admin-cli deployment status --model-name llama-2-7b --environment development
```

## Rollback Validation Steps

After rolling back, validate the deployment:

### 1. Pod Health Check

```bash
# Check all pods are running
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-development -n system

# Expected output:
# NAME                                    READY   STATUS    RESTARTS   AGE
# llama-2-7b-development-7b8f9d5c-x7k2m  1/1     Running   0          2m

# Check pod logs for errors
kubectl logs -l app.kubernetes.io/instance=llama-2-7b-development -n system --tail=50
```

### 2. Health Endpoint Test

```bash
# Test /health endpoint
curl http://llama-2-7b-development.system.svc.cluster.local:8000/health

# Expected response:
# {"status": "ok"}

# Test /ready endpoint
curl http://llama-2-7b-development.system.svc.cluster.local:8000/ready

# Expected response:
# {"status": "ready"}
```

### 3. Completion Test

```bash
# Test model inference
curl -X POST http://llama-2-7b-development.system.svc.cluster.local:8000/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "llama-2-7b",
    "prompt": "Hello, how are you?",
    "max_tokens": 10,
    "temperature": 0.7
  }'

# Verify response contains completion
# Check response time is < 3 seconds
```

### 4. Registry Status Check

```bash
# Verify model is enabled and ready
admin-cli registry list --environment development | grep llama-2-7b

# Expected output:
# ID  Model Name   Endpoint                                      Status  Environment  ...
# 1   llama-2-7b   llama-2-7b-development.system.svc.cluster... ready   development  ...
```

### 5. End-to-End Test via API Router

```bash
# Test routing through API Router
curl -X POST http://api-router:8080/v1/completions \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer <api-key>' \
  -d '{
    "model": "llama-2-7b",
    "prompt": "Test prompt",
    "max_tokens": 5
  }'

# Verify successful routing and response
```

## Common Rollback Scenarios

### Scenario 1: Pod Crash Loop

**Symptoms:**
```bash
$ kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-development -n system
NAME                                    READY   STATUS             RESTARTS   AGE
llama-2-7b-development-7b8f9d5c-x7k2m  0/1     CrashLoopBackOff   5          3m
```

**Solution:**
```bash
# Immediate rollback
./scripts/vllm/rollback-deployment.sh llama-2-7b-development

# Check logs from failed version (optional, for debugging)
kubectl logs llama-2-7b-development-7b8f9d5c-x7k2m -n system --previous
```

### Scenario 2: Health Check Failures

**Symptoms:**
```bash
$ curl http://llama-2-7b-development.system.svc.cluster.local:8000/health
curl: (7) Failed to connect to llama-2-7b-development.system.svc.cluster.local port 8000: Connection refused
```

**Solution:**
```bash
# Check if pods are ready
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-development -n system

# If pods stuck in "Not Ready":
./scripts/vllm/rollback-deployment.sh llama-2-7b-development
```

### Scenario 3: Model Loading Timeout

**Symptoms:**
- Startup probe failing after extended period
- Pods show "Unhealthy" status
- Model initialization taking > configured timeout

**Solution:**
```bash
# Check events
kubectl describe deployment llama-2-7b-development -n system

# Look for startup probe timeout messages
# If timeout is too short, adjust in values and upgrade
# If model genuinely failing to load, rollback:
./scripts/vllm/rollback-deployment.sh llama-2-7b-development
```

### Scenario 4: Wrong Model Version Deployed

**Symptoms:**
- Model producing unexpected outputs
- Version mismatch in responses
- Compatibility issues with clients

**Solution:**
```bash
# Check current values
helm get values llama-2-7b-development -n system

# Rollback to known-good version
./scripts/vllm/rollback-deployment.sh llama-2-7b-development
```

## Troubleshooting Rollback Failures

### Rollback Command Fails

**Error**: `Error: release: llama-2-7b-development not found`

**Solution:**
```bash
# Verify release name and namespace
helm list -n system | grep llama

# Check all namespaces
helm list --all-namespaces | grep llama
```

**Error**: `Error: cannot rollback from 1`

**Solution:**
```bash
# Already at first revision, cannot rollback further
# Must redeploy or fix forward
helm upgrade llama-2-7b-development infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-development.yaml \
  --namespace system
```

### Pods Not Coming Up After Rollback

**Symptoms:**
- Pods stuck in Pending or ImagePullBackOff
- Previous version also failing

**Solution:**
```bash
# Check pod events
kubectl describe pod <pod-name> -n system

# Common issues:
# 1. GPU resources not available - check node capacity
kubectl describe nodes | grep -A 5 "Allocated resources"

# 2. Image pull failure - verify image exists
kubectl get events -n system | grep "Failed to pull image"

# 3. PVC issues - check persistent volumes
kubectl get pvc -n system
```

### Model Still Disabled After Rollback

**Symptoms:**
- Rollback succeeded but model not routing
- Registry shows status as "disabled"

**Solution:**
```bash
# Manually re-enable
admin-cli registry enable --model-name llama-2-7b --environment development

# Verify
admin-cli registry list --environment development | grep llama-2-7b
```

## Rollback Best Practices

1. **Always Check History First**
   ```bash
   helm history <release-name> -n <namespace>
   ```

2. **Use Automated Script for Safety**
   - Handles registry updates automatically
   - Includes validation steps
   - Provides clear rollback summary

3. **Disable During Rollback**
   - Prevents routing to unhealthy deployment
   - Reduces impact on clients
   - Allows safe validation before re-enabling

4. **Validate Before Re-Enabling**
   - Test all health endpoints
   - Run smoke tests
   - Check metrics and logs

5. **Document Rollback Reason**
   ```bash
   helm annotate release llama-2-7b-development \
     "rollback-reason=Pod crash loop due to OOM error" \
     -n system
   ```

6. **Monitor After Rollback**
   - Watch error rates
   - Check latency metrics
   - Verify client success rates

7. **Keep Rollback Window Open**
   - Don't immediately delete old revisions
   - Maintain history for forensics
   - Set Helm revision limit appropriately

## Preventing Need for Rollbacks

1. **Thorough Testing in Staging**
   - Deploy to staging first
   - Run full test suite
   - Load test with production-like traffic

2. **Gradual Rollouts**
   - Deploy to development → staging → production
   - Validate at each stage
   - Use promotion script with validation gates

3. **Proper Resource Configuration**
   - Test startup timeout settings
   - Verify GPU requirements
   - Validate memory limits

4. **Monitoring and Alerts**
   - Set up health check alerts
   - Monitor error rates
   - Track latency metrics

5. **Configuration Validation**
   - Use Helm dry-run before deploying
   - Validate values files
   - Test template rendering

## Emergency Rollback Procedure

For critical production issues requiring immediate rollback:

```bash
# 1. Disable model immediately (stops routing)
admin-cli registry disable --model-name <model> --environment production

# 2. Perform rollback (as fast as possible)
helm rollback <release-name> 0 -n production --wait=false

# 3. Wait for pods (monitor in parallel)
kubectl get pods -l app.kubernetes.io/instance=<release-name> -n production -w

# 4. When healthy, test quickly
curl http://<endpoint>/health

# 5. Re-enable if healthy
admin-cli registry enable --model-name <model> --environment production

# 6. Verify routing restored
curl -X POST http://api-router:8080/v1/completions \
  -H 'Authorization: Bearer <key>' \
  -d '{"model": "<model>", "prompt": "test", "max_tokens": 1}'
```

**Total time target**: < 5 minutes from detection to restored service

## See Also

- [Deployment Workflow](./deployment-workflow.md)
- [Rollout Workflow](./rollout-workflow.md)
- [Promotion Workflow](./vllm-registration-workflow.md#promotion)
- [Partial Failure Remediation Runbook](./runbooks/partial-failure-remediation.md)
- [Environment Separation Strategy](./environment-separation.md)
