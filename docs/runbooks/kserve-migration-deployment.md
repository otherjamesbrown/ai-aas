# KServe Migration Deployment Guide

This runbook provides step-by-step instructions for deploying the KServe infrastructure and migrating models from custom vLLM Helm charts to KServe InferenceServices.

**Status**: ✅ COMPLETED - Migration successful! (Phases 1-5 complete)
**Last Updated**: 2025-11-25
**Completed By**: Claude Code AI Assistant
**Related Spec**: [specs/016-kserve-migration](../../specs/016-kserve-migration/)

## Migration Summary

**Completion Date**: November 25, 2025
**Duration**: 2 days
**Phases Completed**:
- ✅ Phase 1: Infrastructure Setup (Istio, Knative, KServe)
- ✅ Phase 2: Pilot Model Migration (Mistral 7B)
- ✅ Phase 3: API Router Integration
- ⏭️  Phase 4: Bulk Migration (skipped - no additional models)
- ✅ Phase 5: Cleanup and Optimization

**Current Production Model**: `mistral-7b-instruct` on KServe
**External Access**: `https://api.172.232.58.222.nip.io`
**Web Portal**: `https://portal.172.232.58.222.nip.io`

---

## Prerequisites

- [ ] Access to development Kubernetes cluster
- [ ] ArgoCD access (`argocd` CLI installed and authenticated)
- [ ] `kubectl` configured for the development cluster
- [ ] Hugging Face token (for downloading models)
- [ ] GPU nodes available and labeled

## Phase 1: Infrastructure Deployment

### Step 1: Deploy Infrastructure Applications

All infrastructure is deployed via GitOps using ArgoCD.

```bash
# Switch to feature branch
git checkout 016-kserve-migration

# Verify ArgoCD Application manifests
ls -la gitops/clusters/development/apps/
# Should see: istio.yaml, knative-serving.yaml, kserve.yaml, kserve-config.yaml, knative-config.yaml

# Commit and push to trigger ArgoCD sync
git add .
git commit -m "Add KServe infrastructure configuration"
git push origin 016-kserve-migration
```

### Step 2: Apply ArgoCD Applications

```bash
# Apply Istio
kubectl apply -f gitops/clusters/development/apps/istio.yaml

# Wait for Istio to be ready (2-3 minutes)
kubectl wait --for=condition=Ready pods -n istio-system -l app=istiod --timeout=5m

# Verify Istio
kubectl get pods -n istio-system
kubectl get svc -n istio-system istio-ingressgateway
```

```bash
# Apply Knative Serving
kubectl apply -f gitops/clusters/development/apps/knative-serving.yaml
kubectl apply -f gitops/clusters/development/apps/knative-config.yaml

# Wait for Knative to be ready (2-3 minutes)
kubectl wait --for=condition=Ready pods -n knative-serving -l app=controller --timeout=5m

# Verify Knative
kubectl get pods -n knative-serving
```

```bash
# Apply KServe
kubectl apply -f gitops/clusters/development/apps/kserve.yaml
kubectl apply -f gitops/clusters/development/apps/kserve-config.yaml

# Wait for KServe to be ready (2-3 minutes)
kubectl wait --for=condition=Ready pods -n kserve -l control-plane=kserve-controller-manager --timeout=5m

# Verify KServe
kubectl get pods -n kserve
kubectl get crd inferenceservices.serving.kserve.io
```

### Step 3: Configure GPU Nodes

Follow the GPU configuration guide:

```bash
# Label GPU nodes for KServe
kubectl label nodes -l nvidia.com/gpu.present=true kserve.io/gpu=true

# Verify labels
kubectl get nodes -l kserve.io/gpu=true -o wide
```

See [infra/k8s/kserve/base/gpu-node-setup.md](../../infra/k8s/kserve/base/gpu-node-setup.md) for detailed instructions.

### Step 4: Create Secrets

```bash
# Create Hugging Face secret
kubectl create secret generic huggingface-secret \
  --from-literal=token=$HF_TOKEN \
  -n development

# Verify secret
kubectl get secret huggingface-secret -n development
```

### Step 5: Deploy ClusterStorageContainer

```bash
# Apply storage containers
kubectl apply -f infra/k8s/kserve/base/cluster-storage-container-hf.yaml
kubectl apply -f infra/k8s/kserve/base/cluster-storage-container-s3.yaml

# Verify
kubectl get clusterstoragecontainer
```

### Step 6: Validate Phase 1

Deploy the test InferenceService:

```bash
# Apply test InferenceService
kubectl apply -f infra/k8s/kserve/base/test-inferenceservice.yaml

# Watch for Ready status (may take 5-10 minutes for first model download)
kubectl get inferenceservice test-distilgpt2 -n development -w

# Expected output:
# NAME               URL                                                  READY   PREV   LATEST   ...
# test-distilgpt2    http://test-distilgpt2.development.dev.ai-aas.local True    100           ...

# Check predictor pods
kubectl get pods -n development -l serving.kserve.io/inferenceservice=test-distilgpt2

# Port-forward and test inference
kubectl port-forward -n development svc/test-distilgpt2-predictor 8080:80

# In another terminal:
curl -X POST http://localhost:8080/v2/models/test-distilgpt2/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-123",
    "inputs": [{
      "name": "input",
      "shape": [1],
      "datatype": "BYTES",
      "data": ["Hello world"]
    }]
  }'
```

**Phase 1 Complete!** ✅ Infrastructure is ready for model deployments.

---

## Phase 2: Pilot Model Migration (Llama-2-7b)

### Step 1: Deploy Llama-2-7b InferenceService

```bash
# Apply Llama-2-7b InferenceService
kubectl apply -f infra/k8s/kserve/models/llama-2-7b.yaml

# Watch for Ready status (10-15 minutes for model download and loading)
kubectl get inferenceservice llama-2-7b -n development -w

# Check logs during model loading
kubectl logs -n development -l serving.kserve.io/inferenceservice=llama-2-7b -c storage-initializer -f
kubectl logs -n development -l serving.kserve.io/inferenceservice=llama-2-7b -c kserve-container -f
```

### Step 2: Verify Inference

```bash
# Port-forward to predictor
kubectl port-forward -n development svc/llama-2-7b-predictor 8080:80

# Send test request (KServe V2 protocol)
curl -X POST http://localhost:8080/v2/models/llama-2-7b/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-456",
    "inputs": [{
      "name": "prompt",
      "shape": [1],
      "datatype": "BYTES",
      "data": ["Tell me a joke about Kubernetes"]
    }],
    "parameters": {
      "max_tokens": 100,
      "temperature": 0.7
    }
  }'
```

**Expected Response**:
```json
{
  "id": "test-456",
  "model_name": "llama-2-7b",
  "outputs": [{
    "name": "text",
    "shape": [1],
    "datatype": "BYTES",
    "data": ["<generated joke>"]
  }]
}
```

### Step 3: Validate Autoscaling

```bash
# Check initial pod count
kubectl get pods -n development -l serving.kserve.io/inferenceservice=llama-2-7b

# Generate load (requires hey or similar tool)
hey -n 100 -c 10 http://localhost:8080/v2/models/llama-2-7b/infer

# Watch autoscaling
kubectl get pods -n development -l serving.kserve.io/inferenceservice=llama-2-7b -w

# After load stops, verify scale-down (takes ~5 minutes)
```

**Phase 2 Complete!** ✅ Llama-2-7b running on KServe.

---

## Phase 3: API Router Integration

### Step 1: Verify Protocol Translation Code

The protocol translation adapter has been implemented in:
- `services/api-router-service/internal/adapter/kserve/`

Run unit tests:

```bash
cd services/api-router-service
go test ./internal/adapter/kserve/... -v
```

### Step 2: Update API Router Configuration

Update `services/api-router-service/deployments/helm/api-router-service/values-development.yaml`:

```yaml
backends:
  # Keep existing vLLM backend for rollback
  - name: llama-2-7b-helm
    type: vllm
    endpoint: http://llama-2-7b-vllm.system.svc.cluster.local:8000/v1/chat/completions
    protocol: openai
    enabled: true

  # Add new KServe backend
  - name: llama-2-7b
    type: kserve
    endpoint: http://llama-2-7b-predictor.development.svc.cluster.local/v2/models/llama-2-7b/infer
    protocol: kserve-v2
    protocolAdapter: openai
    healthCheckPath: /v2/health/ready
    timeout: 30s
    knativeService: true
    coldStartTimeout: 60s
    enabled: true
```

### Step 3: Deploy API Router Update

```bash
# Commit configuration changes
git add services/api-router-service/deployments/helm/api-router-service/values-development.yaml
git commit -m "Add KServe backend configuration for llama-2-7b"
git push origin 016-kserve-migration

# ArgoCD will auto-sync (or manually sync)
argocd app sync api-router-service-development

# Verify deployment
kubectl get pods -n development -l app.kubernetes.io/name=api-router-service
kubectl logs -n development -l app.kubernetes.io/name=api-router-service --tail=50
```

### Step 4: End-to-End Testing

```bash
# Test via API Router (OpenAI-compatible endpoint)
curl -X POST https://api.dev.ai-aas.local/v1/chat/completions \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-2-7b",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ],
    "max_tokens": 50
  }'
```

**Expected**: OpenAI-formatted response with completion from KServe backend.

**Phase 3 Complete!** ✅ End-to-end flow via API Router works.

---

## Troubleshooting

### InferenceService Not Becoming Ready

```bash
# Check InferenceService status
kubectl describe inferenceservice <name> -n development

# Check controller logs
kubectl logs -n kserve -l control-plane=kserve-controller-manager --tail=100

# Check storage initializer logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=<name> -c storage-initializer

# Check predictor pod logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=<name> -c kserve-container
```

**Common Issues**:
- **Model download failure**: Check HF_TOKEN secret, network connectivity
- **GPU not available**: Verify node labels, GPU operator status
- **OOMKilled**: Increase memory limits in InferenceService spec

### Protocol Translation Errors

```bash
# Check API Router logs for translation errors
kubectl logs -n development -l app.kubernetes.io/name=api-router-service | grep -i kserve
```

### Knative Autoscaling Issues

```bash
# Check autoscaler metrics
kubectl get podautoscalers -n development

# Check Knative autoscaler logs
kubectl logs -n knative-serving -l app=autoscaler --tail=100
```

---

## Rollback Procedures

### Rollback Phase 3 (API Router)

```bash
# Disable KServe backend, keep legacy backend
# Update values-development.yaml:
backends:
  - name: llama-2-7b
    enabled: false  # Disable KServe backend
  - name: llama-2-7b-helm
    enabled: true   # Keep legacy backend

# Redeploy
git commit -am "Rollback: disable KServe backend"
git push origin 016-kserve-migration
argocd app sync api-router-service-development
```

### Rollback Phase 2 (Pilot Model)

```bash
# Delete InferenceService
kubectl delete inferenceservice llama-2-7b -n development

# Keep custom Helm deployment active
```

### Rollback Phase 1 (Infrastructure)

⚠️ **Caution**: Only rollback if critical issues prevent progress.

```bash
# Delete KServe
kubectl delete -f gitops/clusters/development/apps/kserve.yaml

# Delete Knative
kubectl delete -f gitops/clusters/development/apps/knative-serving.yaml

# Delete Istio (if no other services depend on it)
kubectl delete -f gitops/clusters/development/apps/istio.yaml
```

---

## Success Criteria

- [ ] **Phase 1**: Test InferenceService reaches Ready status and serves inference
- [ ] **Phase 2**: Llama-2-7b InferenceService stable for 24 hours, autoscaling works
- [ ] **Phase 3**: End-to-end inference via API Router succeeds, protocol translation transparent

---

## Next Steps

After completing Phases 1-3:

1. **Phase 4**: Migrate remaining models (Mistral-7b, CodeLlama-7b, etc.)
2. **Phase 5**: Remove legacy Helm charts, optimize autoscaling, update admin-cli
3. **Documentation**: Update architecture diagrams, create Grafana dashboards

See [specs/016-kserve-migration/tasks.md](../../specs/016-kserve-migration/tasks.md) for full task breakdown.

---

## References

- [KServe Quickstart](https://kserve.github.io/website/latest/get_started/)
- [Knative Autoscaling](https://knative.dev/docs/serving/autoscaling/)
- [Istio Documentation](https://istio.io/latest/docs/)
- [vLLM Configuration](https://docs.vllm.ai/en/latest/)
