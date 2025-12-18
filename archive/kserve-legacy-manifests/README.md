# Archived: Legacy KServe InferenceService Manifests

**Archive Date**: 2025-12-13

## Purpose of This Archive

This directory contains legacy KServe InferenceService YAML manifests that were previously used for model deployments. These files have been archived because they have been superseded by a more modern and maintainable approach.

## What Changed

**Previous Approach** (archived here):
- Direct KServe InferenceService manifests in `infra/k8s/kserve/models/`
- Manual YAML management for each model deployment
- Applied directly with `kubectl apply -f`

**Current Approach** (active):
- Custom AIModel Custom Resources (CRs) managed by the ai-model-operator
- Declarative model management with automatic reconciliation
- GitOps-driven deployment via ArgoCD
- Environment-specific configurations in `infra/k8s/aimodels/<environment>/`

## Archived Files

- `llama-2-7b.yaml` - Llama 2 7B InferenceService manifest
- `gpt-oss-20b.yaml` - GPT OSS 20B InferenceService manifest
- `mistral-7b-instruct.yaml` - Mistral 7B Instruct (development) manifest
- `mistral-7b-instruct-staging.yaml` - Mistral 7B Instruct (staging) manifest
- `ORIGINAL_README.md` - Original README from legacy location

## Migration Information

**New Model Deployment Locations:**
- Development: `infra/k8s/aimodels/development/`
- Staging: `infra/k8s/aimodels/staging/`
- Production: `infra/k8s/aimodels/production/`

**AI Model Operator:**
- Source: `services/ai-model-operator/`
- Documentation: `services/ai-model-operator/README.md`
- CRD: `services/ai-model-operator/config/crd/bases/ai.ai-aas.io_aimodels.yaml`

**Key Benefits of New Approach:**
- Automatic status tracking and reconciliation
- Built-in health checking and error recovery
- Consistent resource management across environments
- Integration with platform observability stack
- Simplified model lifecycle management

## How to Use AIModel CRs

Instead of creating InferenceService manifests, create AIModel resources:

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-2-7b
  namespace: system
spec:
  modelName: llama-2-7b-chat-hf
  modelSource: hf://meta-llama/Llama-2-7b-chat-hf
  servingRuntime: vllm
  resources:
    limits:
      nvidia.com/gpu: "1"
      memory: "16Gi"
    requests:
      cpu: "2"
      memory: "8Gi"
  replicas: 1
  minReplicas: 0
  maxReplicas: 3
  scaleMetric: concurrency
  scaleTarget: 10
```

The ai-model-operator automatically creates the corresponding InferenceService and manages its lifecycle.

## Reference Documentation

- AI Model Operator Guide: `services/ai-model-operator/README.md`
- Model Deployment Runbook: `docs/runbooks/model-deployment.md`
- AIModel CR Examples: `infra/k8s/aimodels/development/`
- Platform Architecture: `docs/platform/infrastructure-overview.md`

## Restoration

These archived manifests are kept for historical reference only. They should NOT be used for new deployments. If you need to reference the original configuration, consult these files but translate them to AIModel CRs using the examples above.

For questions about the migration or AIModel CR usage, refer to the documentation links above or consult the platform team.
