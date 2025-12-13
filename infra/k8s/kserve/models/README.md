# Legacy KServe Manifests - ARCHIVED

**Notice**: The contents of this directory have been archived as of **2025-12-13**.

## What Happened

The KServe InferenceService YAML manifests that were previously located in this directory have been moved to:

```
archive/kserve-legacy-manifests/
```

These legacy manifests are no longer actively used for model deployments.

## Current Approach: AIModel Custom Resources

The AI-AAS platform now uses **AIModel Custom Resources (CRs)** managed by the **ai-model-operator** for all model deployments. This provides:

- Declarative model management with automatic reconciliation
- GitOps-driven deployment via ArgoCD
- Integrated health checking and error recovery
- Consistent lifecycle management across environments

## Where to Find Active Model Configurations

**Current model deployment locations:**

| Environment | Directory |
|-------------|-----------|
| Development | `infra/k8s/aimodels/development/` |
| Staging | `infra/k8s/aimodels/staging/` |
| Production | `infra/k8s/aimodels/production/` |

## How to Deploy Models Now

**1. Create an AIModel CR** (example):

```yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: my-model
  namespace: system
spec:
  modelName: my-model-name
  modelSource: hf://organization/model-name
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
```

**2. Apply via GitOps:**
- Add your AIModel CR to the appropriate environment directory
- Commit and push to the repository
- ArgoCD will automatically sync and deploy

**3. The ai-model-operator handles:**
- Creating the corresponding KServe InferenceService
- Managing model lifecycle and health
- Scaling based on demand
- Status reporting and error recovery

## Documentation and Resources

**AI Model Operator:**
- Source code: `services/ai-model-operator/`
- README: `services/ai-model-operator/README.md`
- CRD definition: `services/ai-model-operator/config/crd/bases/ai.ai-aas.io_aimodels.yaml`

**Platform Documentation:**
- Model Deployment Runbook: `docs/runbooks/model-deployment.md`
- Infrastructure Overview: `docs/platform/infrastructure-overview.md`
- AIModel CR Examples: `infra/k8s/aimodels/development/`

**Archived Legacy Manifests:**
- Location: `archive/kserve-legacy-manifests/`
- Archive README: `archive/kserve-legacy-manifests/README.md`

## Using the AI-AAS CLI

You can also manage models using the AI-AAS CLI:

```bash
# Register a model
ai-aas-cli model registry add hf://meta-llama/Llama-2-7b-hf --name llama-7b

# Cache a model
ai-aas-cli model cache pull llama-7b

# Deploy a model
ai-aas-cli model deploy create llama-7b -e development

# Check deployment status
ai-aas-cli model deploy status llama-7b
```

## Questions?

For questions about:
- **AIModel CRs**: See `services/ai-model-operator/README.md`
- **Model deployment workflow**: See `docs/runbooks/model-deployment.md`
- **Legacy manifests**: See `archive/kserve-legacy-manifests/README.md`
- **Platform architecture**: See `docs/platform/infrastructure-overview.md`
