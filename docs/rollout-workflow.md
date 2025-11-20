# vLLM Deployment Rollout Workflow

**Feature**: `010-vllm-deployment` (User Story 3 - Safe operations)
**Last Updated**: 2025-01-27

## Overview

This document provides comprehensive guidance for safely rolling out vLLM model deployments across environments with validation gates and rollback triggers.

## Rollout Strategy

### Environment Progression

Models progress through environments in this order:

```
Development → Staging → Production
```

**Never skip staging** - it serves as the final validation gate before production.

## Pre-Deployment Checks

Before deploying to any environment, verify:

### 1. Configuration Validation

```bash
# Validate Helm chart syntax
helm lint infra/helm/charts/vllm-deployment

# Test template rendering
helm template test-release infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-<environment>.yaml \
  --dry-run

# Verify values file
cat infra/helm/charts/vllm-deployment/values-<environment>.yaml
```

### 2. Resource Availability

```bash
# Check GPU node capacity
kubectl describe nodes | grep -A 10 "Allocated resources" | grep nvidia.com/gpu

# Verify namespace exists
kubectl get namespace <namespace>

# Check resource quotas (if applicable)
kubectl get resourcequota -n <namespace>
```

### 3. Model Artifacts

```bash
# Verify model path/image is accessible
# For HuggingFace models:
# - Model ID is valid
# - Organization has access

# For custom models:
# - Image is available in registry
# - Volume mounts configured correctly
```

## Deployment Steps by Environment

### Development Deployment

**Purpose**: Rapid iteration and testing

```bash
# 1. Deploy to development
helm install llama-2-7b-development infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-development.yaml \
  --namespace system \
  --wait --timeout 15m

# 2. Verify deployment
./scripts/vllm/verify-deployment.sh llama-2-7b-development system

# 3. Register model
./scripts/vllm/register-model.sh llama-2-7b development system

# 4. Test inference
curl -X POST http://llama-2-7b-development.system.svc.cluster.local:8000/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{"model": "llama-2-7b", "prompt": "Hello", "max_tokens": 10}'
```

**Validation Gates:**
- ✅ Pods Running and Ready
- ✅ Health endpoint returns 200
- ✅ Sample inference completes successfully
- ✅ Response time < 5s (development SLO relaxed)

### Staging Deployment

**Purpose**: Pre-production validation with production-like config

```bash
# 1. Deploy to staging
helm install llama-2-7b-staging infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-staging.yaml \
  --namespace system \
  --wait --timeout 15m

# 2. Verify deployment
./scripts/vllm/verify-deployment.sh llama-2-7b-staging system

# 3. Register model
./scripts/vllm/register-model.sh llama-2-7b staging system

# 4. Run comprehensive tests
./test-vllm-staging.sh llama-2-7b

# 5. Load test (optional but recommended)
./test-vllm-load.sh llama-2-7b staging
```

**Validation Gates:**
- ✅ All development gates pass
- ✅ Integration tests pass
- ✅ Response time < 3s P95 (production SLO)
- ✅ Error rate < 0.1%
- ✅ Load test: handles expected production traffic
- ✅ Model outputs validated by QA/stakeholders

**Soak Period**: Run staging for 24-48 hours before promoting to production

### Production Deployment

**Purpose**: Serve production traffic

**Option 1: Automated Promotion (Recommended)**

```bash
# Promote validated staging deployment to production
./scripts/vllm/promote-deployment.sh llama-2-7b
```

**Option 2: Manual Deployment**

```bash
# 1. Deploy to production
helm install llama-2-7b-production infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-production.yaml \
  --namespace system \
  --wait --timeout 15m

# 2. Verify deployment
./scripts/vllm/verify-deployment.sh llama-2-7b-production system

# 3. Register model
./scripts/vllm/register-model.sh llama-2-7b production system

# 4. Smoke test
curl -X POST http://api-router:8080/v1/completions \
  -H 'Authorization: Bearer <production-key>' \
  -H 'Content-Type: application/json' \
  -d '{"model": "llama-2-7b", "prompt": "Test", "max_tokens": 5}'
```

**Validation Gates:**
- ✅ All staging gates pass
- ✅ Smoke test successful
- ✅ Metrics within SLO (first 5 minutes)
- ✅ No error spike in first 10 minutes
- ✅ Gradual traffic ramp successful (if using canary)

## Rollback Triggers

Initiate immediate rollback if:

| Trigger | Threshold | Action |
|---------|-----------|--------|
| Error rate spike | > 5% for > 5 min | Immediate rollback |
| P95 latency | > 6s for > 10 min | Immediate rollback |
| Complete outage | 100% errors | Immediate rollback |
| Pod crash loop | All pods failing | Immediate rollback |
| Health check failure | > 3 consecutive failures | Immediate rollback |

## Status Monitoring

### Real-Time Monitoring

```bash
# Watch pod status
kubectl get pods -l app.kubernetes.io/instance=llama-2-7b-production -n system -w

# Monitor logs
kubectl logs -f -l app.kubernetes.io/instance=llama-2-7b-production -n system

# Check deployment status
admin-cli deployment status --model-name llama-2-7b --environment production

# Watch registry status
watch -n 5 "admin-cli registry list --environment production | grep llama-2-7b"
```

### Metrics to Watch

1. **Deployment Health**
   - Pod readiness
   - Restart count
   - Node GPU utilization

2. **Application Metrics**
   - Request rate
   - Error rate
   - P50/P95/P99 latency
   - Token throughput

3. **Resource Metrics**
   - GPU memory usage
   - CPU usage
   - Network I/O

## Common Issues and Resolutions

### Issue: Pods Pending Due to GPU Shortage

**Symptoms:**
```bash
$ kubectl get pods
NAME                    READY   STATUS    RESTARTS   AGE
llama-2-7b-prod-xxx     0/1     Pending   0          5m
```

**Resolution:**
```bash
# Check GPU availability
kubectl describe nodes | grep -A 5 "Allocated resources"

# Options:
# 1. Scale down non-critical GPU workloads
# 2. Add GPU nodes to cluster
# 3. Wait for resources (with timeout)
```

### Issue: Model Initialization Timeout

**Symptoms:**
- Startup probe failing
- Pods restarting repeatedly
- Timeout errors in logs

**Resolution:**
```bash
# Check startup probe configuration
kubectl get deployment llama-2-7b-production -n system -o yaml | grep -A 10 startupProbe

# Adjust failureThreshold for larger models:
# - 7B-13B models: failureThreshold: 20 (10 minutes)
# - 70B models: failureThreshold: 40 (20 minutes)

# Update and redeploy
helm upgrade llama-2-7b-production infra/helm/charts/vllm-deployment \
  --values infra/helm/charts/vllm-deployment/values-production.yaml \
  --set startupProbe.failureThreshold=40 \
  --namespace system
```

### Issue: High Error Rate Post-Deployment

**Symptoms:**
- Error rate > 1%
- 5xx responses from model
- Timeout errors

**Resolution:**
```bash
# 1. Check pod logs for errors
kubectl logs -l app.kubernetes.io/instance=llama-2-7b-production -n system --tail=100

# 2. Test health endpoint
curl http://llama-2-7b-production.system.svc.cluster.local:8000/health

# 3. If unhealthy, rollback immediately
./scripts/vllm/rollback-deployment.sh llama-2-7b-production
```

## Best Practices

### 1. Always Use Staging

Never deploy directly to production without staging validation.

### 2. Automated Testing in Staging

Create automated test suite:
```bash
#!/bin/bash
# test-vllm-staging.sh
MODEL=$1
ENVIRONMENT=${2:-staging}

# Health check
curl -f http://${MODEL}-${ENVIRONMENT}.system.svc.cluster.local:8000/health || exit 1

# Inference test
RESPONSE=$(curl -X POST http://${MODEL}-${ENVIRONMENT}.system.svc.cluster.local:8000/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{"model": "'$MODEL'", "prompt": "Hello", "max_tokens": 10}')

# Validate response
echo $RESPONSE | jq -e '.choices[0].text' || exit 1

echo "All tests passed"
```

### 3. Gradual Rollout Pattern

For critical models, consider:
1. Deploy to single pod
2. Monitor for 1 hour
3. Scale to 50% capacity
4. Monitor for 1 hour
5. Scale to 100%

### 4. Deployment Windows

Schedule production deployments during:
- Low traffic periods
- Business hours (for quick response to issues)
- When oncall team is available

**Avoid:**
- Fridays
- Holidays
- Peak traffic hours
- End of quarter/month

### 5. Communication

Before production deployment:
- Notify team in Slack/Teams
- Update status page (if applicable)
- Ensure oncall engineer is aware

### 6. Documentation

Document every production deployment:
```bash
# Create deployment record
cat > deployments/llama-2-7b-$(date +%Y%m%d-%H%M%S).md <<EOF
# Deployment: llama-2-7b to Production
Date: $(date)
Deployer: $USER
Version: $(helm list -n system -o json | jq -r '.[] | select(.name=="llama-2-7b-production") | .chart')
Reason: New model version with improved accuracy
Rollback: helm rollback llama-2-7b-production 0 -n system
EOF
```

## CI/CD Integration

Example GitLab CI/CD pipeline:

```yaml
deploy-development:
  stage: deploy
  environment: development
  script:
    - helm upgrade --install llama-2-7b-dev infra/helm/charts/vllm-deployment
        --values infra/helm/charts/vllm-deployment/values-development.yaml
        --namespace system --wait --timeout 15m
    - ./scripts/vllm/register-model.sh llama-2-7b development system
  only:
    - develop

deploy-staging:
  stage: deploy
  environment: staging
  script:
    - helm upgrade --install llama-2-7b-staging infra/helm/charts/vllm-deployment
        --values infra/helm/charts/vllm-deployment/values-staging.yaml
        --namespace system --wait --timeout 15m
    - ./scripts/vllm/register-model.sh llama-2-7b staging system
    - ./test-vllm-staging.sh llama-2-7b
  only:
    - main

deploy-production:
  stage: deploy
  environment: production
  when: manual  # Require manual approval
  script:
    - ./scripts/vllm/promote-deployment.sh llama-2-7b
  only:
    - main
```

## See Also

- [Rollback Workflow](./rollback-workflow.md)
- [Deployment Workflow](./deployment-workflow.md)
- [Registration Workflow](./vllm-registration-workflow.md)
- [Environment Separation](./environment-separation.md)
