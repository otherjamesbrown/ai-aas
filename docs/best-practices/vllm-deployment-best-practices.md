# vLLM Deployment Best Practices

**Feature**: `010-vllm-deployment`
**Last Updated**: 2025-01-27

## Overview

This document outlines production-grade best practices for deploying, operating, and maintaining vLLM model inference deployments on the AI-AAS platform.

## Table of Contents

1. [Deployment Practices](#deployment-practices)
2. [Configuration Management](#configuration-management)
3. [Resource Management](#resource-management)
4. [Security](#security)
5. [Monitoring and Observability](#monitoring-and-observability)
6. [Operations](#operations)
7. [Performance Optimization](#performance-optimization)
8. [Cost Optimization](#cost-optimization)
9. [Disaster Recovery](#disaster-recovery)

---

## Deployment Practices

### Always Test in Lower Environments First

**✅ DO**:
```bash
# Deploy to development first
helm install llama-2-7b-dev infra/helm/charts/vllm-deployment \
  --values values-development.yaml \
  -n system

# Validate
./scripts/vllm/validate-deployment.sh llama-2-7b-dev system development

# Promote to staging after validation
./scripts/vllm/promote-deployment.sh llama-2-7b development staging

# Finally, promote to production
./scripts/vllm/promote-deployment.sh llama-2-7b staging production
```

**❌ DON'T**:
```bash
# Never deploy directly to production
helm install llama-2-7b-prod infra/helm/charts/vllm-deployment \
  --values values-production.yaml \
  -n system
```

**Why**: Lower environments catch configuration errors, resource issues, and model loading problems before they impact production traffic.

---

### Use GitOps for All Deployments

**✅ DO**:
```yaml
# argocd-apps/vllm-llama-2-7b-production.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: vllm-llama-2-7b-production
spec:
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas.git
    path: infra/helm/charts/vllm-deployment
    helm:
      valueFiles:
        - values-production.yaml
  syncPolicy:
    # Manual sync for production
    syncOptions:
      - CreateNamespace=true
```

**❌ DON'T**:
```bash
# Don't make manual changes directly with kubectl
kubectl edit deployment llama-2-7b-production -n system
```

**Why**: GitOps provides:
- Audit trail of all changes
- Easy rollback to previous configurations
- Prevention of configuration drift
- Declarative infrastructure as code

---

### Pin Image Versions

**✅ DO**:
```yaml
# values-production.yaml
image:
  repository: vllm/vllm-openai
  tag: "v0.2.6"  # Specific version
  pullPolicy: IfNotPresent
```

**❌ DON'T**:
```yaml
# values-production.yaml
image:
  repository: vllm/vllm-openai
  tag: "latest"  # Unpredictable
```

**Why**: Pinned versions ensure:
- Reproducible deployments
- Controlled upgrades
- No surprise breaking changes
- Easier troubleshooting

---

## Configuration Management

### Use Environment-Specific Values Files

**✅ DO**:
```bash
infra/helm/charts/vllm-deployment/
├── values.yaml                 # Base configuration
├── values-development.yaml     # Dev overrides
├── values-staging.yaml         # Staging overrides
└── values-production.yaml      # Production overrides
```

```yaml
# values-production.yaml
replicaCount: 3
environment: production

resources:
  requests:
    nvidia.com/gpu: 1
    memory: 48Gi
    cpu: 8
  limits:
    nvidia.com/gpu: 1
    memory: 48Gi  # Same as request to avoid OOM
    cpu: 16

startupProbe:
  failureThreshold: 30  # Longer timeout for production

serviceMonitor:
  enabled: true  # Enable monitoring
```

**❌ DON'T**:
```bash
# Don't use command-line overrides for production
helm install my-model . --set replicaCount=3 --set resources.requests.memory=48Gi
```

**Why**: Values files provide:
- Clear documentation of environment differences
- Version control of configuration
- Easy review in pull requests
- Reduced error-prone manual inputs

---

### Set Resource Limits Equal to Requests

**✅ DO**:
```yaml
resources:
  requests:
    memory: 48Gi
    cpu: 8
  limits:
    memory: 48Gi  # Same as request
    cpu: 16       # 2x for burst capacity
```

**❌ DON'T**:
```yaml
resources:
  requests:
    memory: 24Gi
  limits:
    memory: 96Gi  # Huge gap allows OOM
```

**Why**:
- Prevents OOM kills by guaranteeing memory
- Ensures QoS class "Guaranteed" (higher priority)
- Predictable resource allocation
- CPU can have some headroom (2x) for burst traffic

---

### Configure Appropriate Health Probes

**✅ DO**:
```yaml
# Startup probe: For model loading (can take 5-20 minutes)
startupProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 30
  periodSeconds: 30
  failureThreshold: 40  # 20 minutes total for 70B models
  timeoutSeconds: 5

# Liveness probe: Restart if hung
livenessProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 60
  periodSeconds: 30
  failureThreshold: 3
  timeoutSeconds: 5

# Readiness probe: Remove from service if slow
readinessProbe:
  httpGet:
    path: /health
    port: 8000
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 2
  timeoutSeconds: 5
```

**❌ DON'T**:
```yaml
# Don't use same tight timeouts for all models
startupProbe:
  failureThreshold: 10  # Only 5 minutes - too short for 70B models
```

**Why**: Model size affects loading time:
- 7B models: 5-10 minutes
- 13B models: 10-15 minutes
- 70B models: 15-20 minutes

---

## Resource Management

### Size GPUs Appropriately for Models

| Model Size | GPU Type | Min VRAM | Recommended |
|------------|----------|----------|-------------|
| 7B params  | RTX 4000 Ada | 20GB | 24GB |
| 13B params | A10 | 24GB | 48GB |
| 70B params | A100 | 40GB | 80GB |

**✅ DO**:
```yaml
# For 70B model
resources:
  requests:
    nvidia.com/gpu: 2  # Use 2 GPUs with tensor parallelism
    memory: 96Gi
```

**❌ DON'T**:
```yaml
# Don't try to fit 70B model on single 24GB GPU
resources:
  requests:
    nvidia.com/gpu: 1
    memory: 32Gi
```

---

### Use Node Selectors and Taints

**✅ DO**:
```yaml
nodeSelector:
  node-type: gpu
  gpu-model: a100  # Specific GPU for large models

tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
```

**Why**: Ensures pods land on appropriate GPU nodes and prevents non-GPU workloads from consuming GPU resources.

---

### Set Resource Quotas per Environment

**✅ DO**:
```yaml
# Development quota (limited resources)
apiVersion: v1
kind: ResourceQuota
metadata:
  name: vllm-development-quota
  namespace: system
spec:
  hard:
    requests.nvidia.com/gpu: "2"
    requests.memory: "64Gi"
    requests.cpu: "16"

# Production quota (more resources)
apiVersion: v1
kind: ResourceQuota
metadata:
  name: vllm-production-quota
  namespace: system
spec:
  hard:
    requests.nvidia.com/gpu: "8"
    requests.memory: "256Gi"
    requests.cpu: "64"
```

**Why**: Prevents one environment from consuming all cluster resources.

---

## Security

### Use NetworkPolicies for Traffic Isolation

**✅ DO**:
```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: vllm-production-ingress
  namespace: system
spec:
  podSelector:
    matchLabels:
      environment: production
  policyTypes:
    - Ingress
  ingress:
    # Only allow API Router Service
    - from:
      - namespaceSelector:
          matchLabels:
            name: system
        podSelector:
          matchLabels:
            app: api-router-service
      ports:
        - protocol: TCP
          port: 8000
```

**Why**: Limits blast radius of security issues and enforces service mesh boundaries.

---

### Store Secrets in Kubernetes Secrets

**✅ DO**:
```bash
# Create secret for HuggingFace token (secure method to avoid shell history)
echo -n "hf_xxxxxxxxxxxxx" > /tmp/hf_token
kubectl create secret generic hf-token \
  --from-file=token=/tmp/hf_token \
  -n system
rm /tmp/hf_token

# Reference in deployment
env:
  - name: HF_TOKEN
    valueFrom:
      secretKeyRef:
        name: hf-token
        key: token
```

**❌ DON'T**:
```yaml
# Don't hardcode secrets in values
env:
  - name: HF_TOKEN
    value: "hf_xxxxxxxxxxxxx"  # Visible in git!
```

**Why**: Secrets in Kubernetes are:
- Encrypted at rest (if configured)
- Not visible in git history
- Rotatable without code changes
- Access-controlled via RBAC

---

### Use RBAC to Restrict Access

**✅ DO**:
```yaml
# Production: Only deployment team can deploy
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: vllm-production-deployers
  namespace: system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: vllm-deployer
subjects:
  - kind: Group
    name: deployment-team
```

**Why**: Prevents accidental changes to production by limiting who can deploy.

---

## Monitoring and Observability

### Enable ServiceMonitors for Prometheus

**✅ DO**:
```yaml
# values-production.yaml
serviceMonitor:
  enabled: true
  interval: 30s
  scrapeTimeout: 10s
```

**Why**: Automatic metrics collection enables:
- SLO tracking
- Performance monitoring
- Capacity planning
- Alerting on issues

---

### Set Up Alerts for Critical Issues

**✅ DO**:
```yaml
# Apply alert rules
kubectl apply -f docs/monitoring/vllm-alerts.yaml

# Configure Alertmanager routing
route:
  routes:
    - match:
        severity: critical
      receiver: 'pagerduty'
```

**Why**: Alerts provide early warning of:
- Deployment failures
- Performance degradation
- Resource exhaustion
- SLO violations

---

### Use Structured Logging

**✅ DO**:
```python
# In application code
import logging
import json

logger = logging.getLogger(__name__)

logger.info(json.dumps({
    "event": "inference_request",
    "model": "llama-2-7b",
    "duration_ms": 2500,
    "tokens": 100
}))
```

**Why**: Structured logs enable:
- Easy parsing and querying
- Metrics extraction
- Correlation across services
- Better troubleshooting

---

## Operations

### Document Runbooks for Common Scenarios

**✅ DO**:
- Create runbooks for: deployment, rollback, scaling, troubleshooting
- Keep runbooks in git alongside code
- Test runbooks regularly
- Update after incidents

**Reference**:
- [Rollback Workflow](../rollback-workflow.md)
- [Rollout Workflow](../rollout-workflow.md)
- [Troubleshooting Guide](../troubleshooting/vllm-deployment-troubleshooting.md)
- [Partial Failure Remediation](../runbooks/partial-failure-remediation.md)

---

### Use Validation Scripts

**✅ DO**:
```bash
# After every deployment
./scripts/vllm/validate-deployment.sh llama-2-7b-production system production

# Check passes before marking deployment complete
```

**Why**: Catches issues early:
- Pod health problems
- Misconfigured endpoints
- Registry sync issues
- Inference failures

---

### Implement Gradual Rollouts

**✅ DO**:
```bash
# 1. Deploy to dev
helm install llama-2-7b-dev . --values values-development.yaml -n system

# 2. Validate in dev for 1 hour
# Monitor metrics, test functionality

# 3. Deploy to staging
helm install llama-2-7b-staging . --values values-staging.yaml -n system

# 4. Soak test in staging for 24-48 hours
# Run load tests, validate SLOs

# 5. Deploy to production with canary (single pod first)
helm install llama-2-7b-prod . --set replicaCount=1 --values values-production.yaml -n system

# 6. Monitor for 1 hour

# 7. Scale to full capacity
helm upgrade llama-2-7b-prod . --set replicaCount=3 --values values-production.yaml -n system
```

---

### Maintain Rollback Plans

**✅ DO**:
```bash
# Before production deployment
echo "Rollback command: helm rollback llama-2-7b-prod 0 -n system" > rollback-plan.txt

# Include in deployment notes
# Test rollback in staging first
```

---

## Performance Optimization

### Tune Batch Size for Throughput

**Recommendation**:
- 7B models: batch_size=32
- 13B models: batch_size=16
- 70B models: batch_size=8

**✅ DO**:
```yaml
vllm:
  maxBatchSize: 16  # Tune based on model size and GPU memory
```

**Why**: Larger batches increase throughput but consume more GPU memory.

---

### Use Tensor Parallelism for Large Models

**✅ DO**:
```yaml
# For 70B model
resources:
  requests:
    nvidia.com/gpu: 2

vllm:
  tensorParallelSize: 2  # Split model across 2 GPUs
```

**Why**: Enables deploying models larger than single GPU VRAM.

---

### Optimize max_model_len

**✅ DO**:
```yaml
vllm:
  maxModelLen: 2048  # Match your use case
```

**Guidance**:
- 512: Short responses (chatbot)
- 2048: Medium responses (QA, summaries)
- 4096: Long responses (document generation)

**Why**: Shorter max length reduces memory usage and improves throughput.

---

### Use Model Caching

**✅ DO**:
```yaml
persistence:
  enabled: true
  size: 200Gi
  storageClass: fast-ssd
  mountPath: /models
```

**Why**: Avoids re-downloading model weights on pod restarts.

---

## Cost Optimization

### Right-Size GPU Allocations

**✅ DO**:
- Use smallest GPU that fits model
- Share GPUs across small models (if supported)
- Scale down non-production environments overnight

```bash
# Scale down dev at night
kubectl scale deployment llama-2-7b-dev --replicas=0 -n system

# Scale back up in morning
kubectl scale deployment llama-2-7b-dev --replicas=1 -n system
```

---

### Use Spot/Preemptible Instances for Dev

**✅ DO**:
```yaml
# values-development.yaml
nodeSelector:
  node-type: gpu-spot  # Use spot instances for dev
```

**Why**: 60-80% cost savings for non-critical workloads.

---

### Monitor GPU Utilization

**✅ DO**:
```promql
# Alert on low GPU utilization
avg(nvidia_gpu_utilization{pod=~".*-production-.*"}) < 50
```

**Why**: Underutilized GPUs waste money. Consider:
- Consolidating models
- Reducing replica count
- Using smaller GPUs

---

## Disaster Recovery

### Regular Backups

**✅ DO**:
```bash
# Backup Helm values
helm get values llama-2-7b-prod -n system > backup-values-$(date +%Y%m%d).yaml

# Backup model registry (use label selector for robustness)
PG_POD=$(kubectl get pod -n system -l app.kubernetes.io/name=postgresql -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it $PG_POD -n system -- \
  pg_dump -U postgres -d ai_aas_operational -t model_registry_entries > registry-backup-$(date +%Y%m%d).sql
```

---

### Document Recovery Procedures

**✅ DO**:
```markdown
# Disaster Recovery Plan

## Scenario: Complete Cluster Loss

1. Provision new cluster
2. Install NVIDIA device plugin
3. Restore Helm releases from git
4. Restore model registry from backup
5. Validate deployments
6. Update DNS/load balancers

RTO: 2 hours
RPO: 1 hour (hourly backups)
```

---

### Test Recovery Regularly

**✅ DO**:
- Quarterly DR drills
- Test restoring to new cluster
- Validate all services work
- Update DR plan based on findings

---

## Quick Reference Checklist

### Pre-Deployment

- [ ] Values file reviewed and approved
- [ ] Image version pinned
- [ ] Resource limits appropriate for model size
- [ ] Health probe timeouts match model size
- [ ] Secrets created in Kubernetes
- [ ] NetworkPolicies defined
- [ ] Rollback plan documented

### Post-Deployment

- [ ] Validation script passed
- [ ] Model registered in registry
- [ ] Metrics appearing in Prometheus
- [ ] Dashboard shows healthy status
- [ ] Alerts configured and firing (test)
- [ ] Inference test successful
- [ ] Documentation updated

### Production Deployment

- [ ] Tested in development
- [ ] Validated in staging (24-48 hour soak)
- [ ] Change request approved
- [ ] Oncall engineer notified
- [ ] Deployed during low-traffic window
- [ ] Monitored for 1 hour post-deployment
- [ ] Rollback plan tested in staging
- [ ] Postmortem template prepared (if needed)

---

## See Also

- [Helm Chart README](../../infra/helm/charts/vllm-deployment/README.md)
- [Rollout Workflow](../rollout-workflow.md)
- [Rollback Workflow](../rollback-workflow.md)
- [Troubleshooting Guide](../troubleshooting/vllm-deployment-troubleshooting.md)
- [Performance SLO Tracking](../monitoring/performance-slo-tracking.md)
- [Environment Separation](../environment-separation.md)
