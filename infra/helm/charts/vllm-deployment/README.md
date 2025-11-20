# vLLM Deployment Helm Chart

Production-ready Helm chart for deploying vLLM model inference engines on Kubernetes with GPU support.

## Overview

This Helm chart deploys vLLM (Very Large Language Model) inference servers on Kubernetes clusters with GPU nodes. It provides:

- GPU resource allocation and scheduling
- Health checks and readiness probes
- Service mesh integration
- Environment-specific configurations
- Network policies for security
- Prometheus metrics export

## Prerequisites

- Kubernetes 1.24+
- Helm 3.8+
- GPU nodes with NVIDIA drivers installed
- NVIDIA device plugin for Kubernetes
- Persistent storage (optional, for model caching)

## Installation

### Quick Start

```bash
# Install in development environment
helm install llama-2-7b-dev . \
  --values values-development.yaml \
  --namespace system \
  --create-namespace

# Install in production environment
helm install llama-2-7b-prod . \
  --values values-production.yaml \
  --namespace system
```

### With Custom Values

```bash
# Install with custom model
helm install my-model . \
  --set model.name=meta-llama/Llama-2-7b-hf \
  --set model.path=/models/llama-2-7b \
  --set replicaCount=2 \
  --namespace system
```

## Configuration

### Values Files

The chart includes environment-specific values files:

- `values.yaml` - Base configuration (defaults)
- `values-development.yaml` - Development environment
- `values-staging.yaml` - Staging environment
- `values-production.yaml` - Production environment

### Key Parameters

#### Model Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `model.name` | Model identifier | `meta-llama/Llama-2-7b-hf` |
| `model.path` | Path to model weights | `/models/llama-2-7b` |
| `model.revision` | Model revision/version | `main` |

#### Image Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `image.repository` | vLLM container image | `vllm/vllm-openai` |
| `image.tag` | Image tag | `latest` |
| `image.pullPolicy` | Image pull policy | `IfNotPresent` |

#### Resource Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `resources.requests.nvidia.com/gpu` | GPU request | `1` |
| `resources.limits.nvidia.com/gpu` | GPU limit | `1` |
| `resources.requests.memory` | Memory request | `24Gi` |
| `resources.limits.memory` | Memory limit | `32Gi` |
| `resources.requests.cpu` | CPU request | `4` |
| `resources.limits.cpu` | CPU limit | `8` |

#### Deployment Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `replicaCount` | Number of replicas | `1` |
| `environment` | Deployment environment | `development` |
| `namespace` | Kubernetes namespace | `system` |

#### vLLM Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `vllm.host` | Bind host | `0.0.0.0` |
| `vllm.port` | Service port | `8000` |
| `vllm.maxModelLen` | Max sequence length | `4096` |
| `vllm.tensorParallelSize` | Tensor parallel size | `1` |

#### Health Check Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `livenessProbe.enabled` | Enable liveness probe | `true` |
| `livenessProbe.initialDelaySeconds` | Initial delay | `60` |
| `livenessProbe.periodSeconds` | Check interval | `30` |
| `livenessProbe.timeoutSeconds` | Timeout | `5` |
| `livenessProbe.failureThreshold` | Failure threshold | `3` |
| `readinessProbe.enabled` | Enable readiness probe | `true` |
| `readinessProbe.initialDelaySeconds` | Initial delay | `30` |
| `readinessProbe.periodSeconds` | Check interval | `10` |
| `startupProbe.enabled` | Enable startup probe | `true` |
| `startupProbe.failureThreshold` | Failure threshold | `20` |

#### Node Selection

| Parameter | Description | Default |
|-----------|-------------|---------|
| `nodeSelector` | Node selector labels | `{"node-type": "gpu"}` |
| `tolerations` | Pod tolerations | GPU taint tolerations |
| `affinity` | Pod affinity rules | `{}` |

#### Service Configuration

| Parameter | Description | Default |
|-----------|-------------|---------|
| `service.type` | Service type | `ClusterIP` |
| `service.port` | Service port | `8000` |
| `service.annotations` | Service annotations | `{}` |

#### Network Policy

| Parameter | Description | Default |
|-----------|-------------|---------|
| `networkPolicy.enabled` | Enable network policy | `true` |
| `networkPolicy.allowedNamespaces` | Allowed namespaces | `["system"]` |

#### Monitoring

| Parameter | Description | Default |
|-----------|-------------|---------|
| `serviceMonitor.enabled` | Enable Prometheus scraping | `true` |
| `serviceMonitor.interval` | Scrape interval | `30s` |

## Examples

### Development Deployment

```yaml
# values-custom-dev.yaml
replicaCount: 1
environment: development

model:
  name: meta-llama/Llama-2-7b-hf
  path: /models/llama-2-7b

resources:
  requests:
    nvidia.com/gpu: 1
    memory: 24Gi
    cpu: 4
  limits:
    nvidia.com/gpu: 1
    memory: 32Gi
    cpu: 8

startupProbe:
  failureThreshold: 20  # 10 minutes for model loading
```

```bash
helm install llama-2-7b-dev . \
  --values values-custom-dev.yaml \
  --namespace system
```

### Production Deployment with High Availability

```yaml
# values-custom-prod.yaml
replicaCount: 3
environment: production

model:
  name: meta-llama/Llama-2-70b-hf
  path: /models/llama-2-70b

resources:
  requests:
    nvidia.com/gpu: 2
    memory: 96Gi
    cpu: 16
  limits:
    nvidia.com/gpu: 2
    memory: 128Gi
    cpu: 32

startupProbe:
  failureThreshold: 40  # 20 minutes for large model

nodeSelector:
  node-type: gpu
  gpu-model: a100

tolerations:
  - key: nvidia.com/gpu
    operator: Exists
    effect: NoSchedule
```

```bash
helm install llama-2-70b-prod . \
  --values values-custom-prod.yaml \
  --namespace system
```

### Custom Model from S3

```yaml
# values-s3-model.yaml
model:
  name: my-custom-model
  path: s3://my-bucket/models/custom-model

env:
  - name: AWS_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: aws-credentials
        key: access-key-id
  - name: AWS_SECRET_ACCESS_KEY
    valueFrom:
      secretKeyRef:
        name: aws-credentials
        key: secret-access-key
```

## Upgrade

### Standard Upgrade

```bash
# Upgrade existing deployment
helm upgrade llama-2-7b-prod . \
  --values values-production.yaml \
  --namespace system \
  --wait
```

### Upgrade with Value Changes

```bash
# Increase replicas
helm upgrade llama-2-7b-prod . \
  --values values-production.yaml \
  --set replicaCount=3 \
  --namespace system
```

### Dry Run Before Upgrade

```bash
# Test upgrade without applying
helm upgrade llama-2-7b-prod . \
  --values values-production.yaml \
  --dry-run --debug
```

## Rollback

```bash
# View release history
helm history llama-2-7b-prod -n system

# Rollback to previous version
helm rollback llama-2-7b-prod -n system

# Rollback to specific revision
helm rollback llama-2-7b-prod 3 -n system
```

See [Rollback Workflow](../../../docs/rollback-workflow.md) for detailed procedures.

## Uninstall

```bash
# Uninstall release
helm uninstall llama-2-7b-prod -n system

# Uninstall and keep history
helm uninstall llama-2-7b-prod -n system --keep-history
```

## Troubleshooting

### Pods Stuck in Pending

**Symptom:**
```bash
$ kubectl get pods -n system
NAME                    READY   STATUS    RESTARTS   AGE
llama-2-7b-prod-xxx     0/1     Pending   0          5m
```

**Diagnosis:**
```bash
# Check pod events
kubectl describe pod llama-2-7b-prod-xxx -n system | grep Events -A 10

# Check GPU availability
kubectl describe nodes | grep -A 5 "Allocated resources"
```

**Solutions:**
1. Insufficient GPU resources - scale down other workloads or add nodes
2. Node selector mismatch - verify node labels
3. Tolerations missing - check node taints

### Model Loading Timeout

**Symptom:**
```bash
$ kubectl get pods -n system
NAME                    READY   STATUS    RESTARTS   AGE
llama-2-7b-prod-xxx     0/1     Running   5          10m
```

**Diagnosis:**
```bash
# Check startup probe configuration
kubectl get deployment llama-2-7b-prod -n system -o yaml | grep -A 10 startupProbe

# Check pod logs
kubectl logs llama-2-7b-prod-xxx -n system
```

**Solutions:**
1. Increase `startupProbe.failureThreshold`:
   - 7B-13B models: `failureThreshold: 20` (10 minutes)
   - 70B models: `failureThreshold: 40` (20 minutes)
2. Verify model path is accessible
3. Check GPU memory availability

### Health Check Failures

**Symptom:**
- Pods restarting frequently
- Readiness probe failures

**Diagnosis:**
```bash
# Test health endpoint manually
kubectl run test-health --rm -i --restart=Never -n system \
  --image=curlimages/curl:latest -- \
  curl -v http://llama-2-7b-prod.system.svc.cluster.local:8000/health
```

**Solutions:**
1. Increase probe timeout: `livenessProbe.timeoutSeconds: 10`
2. Increase CPU allocation if model is CPU-bound
3. Check for OOM errors in logs

### Service Not Accessible

**Symptom:**
- Cannot reach service endpoint
- Connection refused errors

**Diagnosis:**
```bash
# Check service exists
kubectl get service llama-2-7b-prod -n system

# Check endpoints
kubectl get endpoints llama-2-7b-prod -n system

# Test from another pod
kubectl run test-svc --rm -i --restart=Never -n system \
  --image=curlimages/curl:latest -- \
  curl -f http://llama-2-7b-prod.system.svc.cluster.local:8000/health
```

**Solutions:**
1. Verify service selector matches pod labels
2. Check NetworkPolicy allows ingress
3. Verify pods are in Ready state

## Best Practices

### Production Deployments

1. **Use Specific Image Tags**
   ```yaml
   image:
     tag: "v0.2.6"  # Don't use 'latest'
   ```

2. **Set Resource Limits**
   ```yaml
   resources:
     requests:
       memory: 48Gi
       cpu: 8
     limits:
       memory: 48Gi  # Same as request to avoid OOM kills
       cpu: 16
   ```

3. **Configure Appropriate Timeouts**
   ```yaml
   startupProbe:
     failureThreshold: 30  # Based on model size
   ```

4. **Enable Monitoring**
   ```yaml
   serviceMonitor:
     enabled: true
   ```

5. **Use NetworkPolicies**
   ```yaml
   networkPolicy:
     enabled: true
     allowedNamespaces: ["system"]
   ```

### Development Best Practices

1. Use `values-development.yaml` for consistency
2. Test startup probes match production
3. Validate resource requests match production ratios
4. Use same image versions as production when possible

### Multi-Environment Strategy

Deploy to environments in order:
1. Development - rapid iteration
2. Staging - production validation
3. Production - with promotion script

See [Environment Separation](../../../docs/environment-separation.md) for details.

## Monitoring

### Prometheus Metrics

The vLLM service exposes metrics at `/metrics`:

```promql
# Request rate
rate(vllm_requests_total[5m])

# Error rate
rate(vllm_errors_total[5m]) / rate(vllm_requests_total[5m])

# P95 latency
histogram_quantile(0.95, rate(vllm_request_duration_seconds_bucket[5m]))

# GPU utilization
nvidia_gpu_utilization{pod=~"llama-2-7b-prod-.*"}
```

### Health Endpoints

- `/health` - Liveness check
- `/ready` - Readiness check
- `/metrics` - Prometheus metrics
- `/v1/models` - List available models

## Security

### Network Policies

The chart includes NetworkPolicies that:
- Allow ingress only from API Router Service
- Deny all other ingress by default
- Allow egress for model downloads

### RBAC

The chart creates a ServiceAccount with minimal permissions:
- Read access to ConfigMaps
- Read access to Secrets (if needed)

### Secrets Management

Store sensitive data in Kubernetes Secrets:

```bash
# Create secret for model access
kubectl create secret generic model-credentials \
  --from-literal=api-key=xxx \
  -n system

# Reference in values
env:
  - name: HF_TOKEN
    valueFrom:
      secretKeyRef:
        name: model-credentials
        key: api-key
```

## Performance Tuning

### GPU Optimization

```yaml
vllm:
  tensorParallelSize: 2  # Use 2 GPUs for large models
  maxModelLen: 4096      # Adjust based on use case
```

### Memory Optimization

```yaml
resources:
  limits:
    memory: 64Gi  # Increase for larger models
```

### CPU Optimization

```yaml
resources:
  limits:
    cpu: 16  # Increase if CPU-bound
```

## Contributing

See main repository [CONTRIBUTING.md](../../../CONTRIBUTING.md) for contribution guidelines.

## License

See main repository [LICENSE](../../../LICENSE).

## Support

- Documentation: [docs/](../../../docs/)
- Issues: [GitHub Issues](https://github.com/otherjamesbrown/ai-aas/issues)
- Runbooks: [docs/runbooks/](../../../docs/runbooks/)

## Related Documentation

- [Deployment Workflow](../../../docs/deployment-workflow.md)
- [Registration Workflow](../../../docs/vllm-registration-workflow.md)
- [Rollback Workflow](../../../docs/rollback-workflow.md)
- [Rollout Workflow](../../../docs/rollout-workflow.md)
- [Environment Separation](../../../docs/environment-separation.md)
- [Partial Failure Remediation](../../../docs/runbooks/partial-failure-remediation.md)
