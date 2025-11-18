# Quickstart: Model Inference Deployment

**Feature**: `010-vllm-deployment`  
**Date**: 2025-01-27  
**Purpose**: Get started with deploying vLLM model inference engines

## Prerequisites

- Kubernetes cluster (LKE) with GPU node pool configured
- `kubectl` configured with cluster access
- `helm` 3.x installed
- ArgoCD installed and configured (for GitOps)
- Access to HuggingFace model repository (for model downloads)
- PostgreSQL database with `model_registry_entries` table

## Quick Start

### 1. Deploy a Model to Development

```bash
# Set environment variables
export MODEL_NAME="llama-7b"
export REVISION=1
export ENVIRONMENT="development"
export MODEL_PATH="meta-llama/Llama-2-7b-chat-hf"

# Deploy using Helm
helm install ${MODEL_NAME}-${ENVIRONMENT} \
  infra/helm/charts/vllm-deployment \
  --namespace system \
  --set model.name=${MODEL_NAME} \
  --set model.revision=${REVISION} \
  --set model.path=${MODEL_PATH} \
  --set environment=${ENVIRONMENT} \
  --set resources.gpu.count=1 \
  --set resources.memory.request="32Gi" \
  --set resources.memory.limit="48Gi"

# Wait for deployment to be ready
kubectl wait --for=condition=ready pod \
  -l app=vllm-deployment,model=${MODEL_NAME} \
  -n system \
  --timeout=10m
```

### 2. Register Model for Routing

```bash
# Get the service endpoint
ENDPOINT=$(kubectl get svc ${MODEL_NAME}-${ENVIRONMENT} -n system -o jsonpath='{.metadata.name}.{.metadata.namespace}.svc.cluster.local:8000')

# Register in model registry (via admin-cli or API)
admin-cli registry register \
  --model-name ${MODEL_NAME} \
  --revision ${REVISION} \
  --endpoint ${ENDPOINT} \
  --environment ${ENVIRONMENT} \
  --cost-per-1k-tokens 0.25
```

### 3. Verify Deployment

```bash
# Check deployment status
kubectl get deployment ${MODEL_NAME}-${ENVIRONMENT} -n system

# Check pod status
kubectl get pods -l app=vllm-deployment,model=${MODEL_NAME} -n system

# Check service endpoint
kubectl get svc ${MODEL_NAME}-${ENVIRONMENT} -n system

# Test health endpoint
kubectl port-forward svc/${MODEL_NAME}-${ENVIRONMENT} 8000:8000 -n system &
curl http://localhost:8000/health

# Test completion endpoint
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'${MODEL_NAME}'",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'
```

### 4. Verify Routing

```bash
# Test routing via API Router
curl -X POST https://api.example.com/v1/chat/completions \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "'${MODEL_NAME}'",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 50
  }'
```

## Environment-Specific Deployment

### Development

```bash
helm install ${MODEL_NAME}-development \
  infra/helm/charts/vllm-deployment \
  --namespace system \
  -f infra/helm/charts/vllm-deployment/values-development.yaml \
  --set model.name=${MODEL_NAME} \
  --set model.path=${MODEL_PATH}
```

### Staging

```bash
helm install ${MODEL_NAME}-staging \
  infra/helm/charts/vllm-deployment \
  --namespace system \
  -f infra/helm/charts/vllm-deployment/values-staging.yaml \
  --set model.name=${MODEL_NAME} \
  --set model.path=${MODEL_PATH}
```

### Production

```bash
# Production requires manual approval via ArgoCD
# 1. Create ArgoCD Application manifest
# 2. Submit PR with deployment configuration
# 3. After approval, ArgoCD syncs automatically

# Or deploy manually (not recommended for production)
helm install ${MODEL_NAME}-production \
  infra/helm/charts/vllm-deployment \
  --namespace system \
  -f infra/helm/charts/vllm-deployment/values-production.yaml \
  --set model.name=${MODEL_NAME} \
  --set model.path=${MODEL_PATH}
```

## Common Operations

### Check Deployment Status

```bash
# List all deployments
helm list -n system

# Get deployment details
helm status ${MODEL_NAME}-${ENVIRONMENT} -n system

# Check pod logs
kubectl logs -l app=vllm-deployment,model=${MODEL_NAME} -n system --tail=100
```

### Rollback Deployment

```bash
# List Helm release history
helm history ${MODEL_NAME}-${ENVIRONMENT} -n system

# Rollback to previous revision
helm rollback ${MODEL_NAME}-${ENVIRONMENT} -n system

# Rollback to specific revision
helm rollback ${MODEL_NAME}-${ENVIRONMENT} 2 -n system
```

### Update Deployment

```bash
# Update model configuration
helm upgrade ${MODEL_NAME}-${ENVIRONMENT} \
  infra/helm/charts/vllm-deployment \
  --namespace system \
  --set model.path=${NEW_MODEL_PATH} \
  --set resources.memory.limit="64Gi"
```

### Disable/Enable Model Routing

```bash
# Disable model (stops routing but keeps deployment)
admin-cli registry update ${MODEL_ID} \
  --status disabled

# Enable model (resumes routing)
admin-cli registry update ${MODEL_ID} \
  --status ready
```

### Delete Deployment

```bash
# Uninstall Helm release
helm uninstall ${MODEL_NAME}-${ENVIRONMENT} -n system

# Deregister from routing
admin-cli registry deregister ${MODEL_ID}
```

## Health Checks

### Manual Health Check

```bash
# Port forward to service
kubectl port-forward svc/${MODEL_NAME}-${ENVIRONMENT} 8000:8000 -n system

# Check health endpoint
curl http://localhost:8000/health

# Check readiness endpoint
curl http://localhost:8000/ready

# Check metrics endpoint
curl http://localhost:8000/metrics
```

### Automated Health Monitoring

Health checks are performed automatically by:
- Kubernetes liveness/readiness probes (every 30s)
- Health check service (updates `last_health_check_at` in database)
- Prometheus metrics scraping

## Troubleshooting

### Deployment Stuck in "Deploying" State

```bash
# Check pod events
kubectl describe pod -l app=vllm-deployment,model=${MODEL_NAME} -n system

# Check pod logs
kubectl logs -l app=vllm-deployment,model=${MODEL_NAME} -n system

# Check resource constraints
kubectl top pod -l app=vllm-deployment,model=${MODEL_NAME} -n system
```

### Model Not Loading

```bash
# Check GPU availability
kubectl get nodes -l node-type=gpu

# Check GPU allocation
kubectl describe node <gpu-node>

# Check model download progress
kubectl logs -l app=vllm-deployment,model=${MODEL_NAME} -n system | grep -i download
```

### Routing Not Working

```bash
# Verify model is registered
admin-cli registry list --model-name ${MODEL_NAME}

# Check API Router logs
kubectl logs -l app=api-router-service -n system --tail=100

# Verify endpoint is accessible
kubectl exec -it <api-router-pod> -n system -- \
  curl http://${MODEL_NAME}-${ENVIRONMENT}.system.svc.cluster.local:8000/health
```

### Health Check Failures

```bash
# Check health endpoint directly
kubectl exec -it <vllm-pod> -n system -- curl localhost:8000/health

# Check readiness probe configuration
kubectl get deployment ${MODEL_NAME}-${ENVIRONMENT} -n system -o yaml | grep -A 10 readinessProbe

# Check probe failures
kubectl describe pod -l app=vllm-deployment,model=${MODEL_NAME} -n system | grep -A 5 "Readiness\|Liveness"
```

## GitOps Workflow (ArgoCD)

### Create ArgoCD Application

```yaml
# argocd-apps/vllm-llama-7b-development.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: vllm-llama-7b-development
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/your-org/ai-aas
    targetRevision: main
    path: infra/helm/charts/vllm-deployment
    helm:
      valueFiles:
        - values-development.yaml
      values: |
        model:
          name: llama-7b
          revision: 1
          path: meta-llama/Llama-2-7b-chat-hf
        environment: development
  destination:
    server: https://kubernetes.default.svc
    namespace: system
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
```

### Apply ArgoCD Application

```bash
kubectl apply -f argocd-apps/vllm-llama-7b-development.yaml

# Check sync status
argocd app get vllm-llama-7b-development

# Manual sync (if needed)
argocd app sync vllm-llama-7b-development
```

## Next Steps

1. **Review Deployment**: Verify model is serving requests correctly
2. **Monitor Metrics**: Check Prometheus/Grafana dashboards
3. **Set Up Alerts**: Configure alerts for deployment failures
4. **Promote to Staging**: Follow promotion workflow
5. **Production Deployment**: Follow production approval process

## Additional Resources

- [Feature Specification](./spec.md)
- [Implementation Plan](./plan.md)
- [Data Model](./data-model.md)
- [API Contracts](./contracts/model-deployment-api.yaml)
- [Research & Decisions](./research.md)
- [Platform Architecture](../../usage-guide/architect/architecture-overview.md)

