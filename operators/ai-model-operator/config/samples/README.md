# AIModel Sample CRs

This directory contains example AIModel Custom Resources (CRs) that demonstrate how to use the AI Model Operator.

## Overview

The AI Model Operator manages AI model deployments by creating KServe InferenceServices from AIModel CRs. This provides a GitOps-friendly, declarative way to manage model deployments.

## Samples

### mistral-7b-instruct-staging.yaml

Example AIModel CR for Mistral-7B-Instruct-v0.2 in staging environment.

**Key features:**
- Public HuggingFace model (no S3 required)
- vLLM runtime with GPU acceleration
- Keeps 1 replica warm (minReplicas=1) for fast testing
- Autoscales to 3 replicas under load (maxReplicas=3)
- GPU node scheduling with appropriate tolerations

**To apply:**
```bash
kubectl apply -f mistral-7b-instruct-staging.yaml
```

**To verify:**
```bash
# Check AIModel status
kubectl get aimodel mistral-7b-instruct -n staging

# Check created InferenceService
kubectl get inferenceservice mistral-7b-instruct -n staging

# Check inference endpoint
kubectl get aimodel mistral-7b-instruct -n staging -o jsonpath='{.status.inferenceEndpoint}'
```

## AIModel → InferenceService Mapping

The operator translates AIModel CRs into KServe InferenceService resources:

| AIModel Field | InferenceService Field | Notes |
|---------------|------------------------|-------|
| `spec.modelID` | `containers[0].args[--model]` | HuggingFace model ID |
| `spec.runtime` | `containers[0].image` | Selects runtime image (vllm, triton, tgi) |
| `spec.minReplicas` | `predictor.minReplicas` | Minimum replicas for autoscaling |
| `spec.maxReplicas` | `predictor.maxReplicas` | Maximum replicas for autoscaling |
| `spec.resources` | `containers[0].resources` | CPU, memory, GPU requests/limits |
| `spec.tolerations` | `predictor.tolerations` | Node scheduling tolerations |
| `spec.runtimeArgs` | `containers[0].args` | Runtime-specific CLI arguments |
| `spec.runtimeEnv` | `containers[0].env` | Runtime-specific environment variables |
| `spec.enabled=false` | `predictor.minReplicas=0` | Scales to zero when disabled |

## Migration from Manual InferenceService

If you have existing manual InferenceService deployments, you can migrate them to AIModel CRs:

1. **Read the existing InferenceService**:
   ```bash
   kubectl get inferenceservice <name> -n <namespace> -o yaml > old-isvc.yaml
   ```

2. **Create AIModel CR** mapping the InferenceService fields (see table above)

3. **Apply the AIModel CR**:
   ```bash
   kubectl apply -f <aimodel-cr>.yaml
   ```

4. **Verify the operator creates the InferenceService**:
   ```bash
   kubectl get inferenceservice <name> -n <namespace>
   ```

5. **Delete the old manual InferenceService** (only after verifying the new one works):
   ```bash
   kubectl delete -f old-isvc.yaml
   ```

## Related Documentation

- AI Model Operator controller: `operators/ai-model-operator/controllers/aimodel_controller.go`
- AIModel CRD: `operators/ai-model-operator/api/v1alpha1/aimodel_types.go`
- KServe InferenceService builder: `operators/ai-model-operator/internal/kserve/inferenceservice.go`
