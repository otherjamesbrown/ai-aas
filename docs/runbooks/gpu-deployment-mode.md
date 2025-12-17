# GPU Deployment Mode Runbook

## Overview

This runbook covers deployment and troubleshooting of GPU-based ML model deployments using RawDeployment mode in KServe.

## Background

GPU workloads use RawDeployment mode instead of Knative Serverless because:
- Knative's admission webhook rejects nodeSelector (required for GPU node scheduling)
- Knative requires single-port containers (Triton needs multiple ports)
- Scale-to-zero is counterproductive for slow-loading GPU models (5-10 minute load times)

## Verifying Deployment Mode

Check the deployment mode annotation on an InferenceService:

```bash
kubectl get isvc <model-name> -n <namespace> -o jsonpath='{.metadata.annotations.serving\.kserve\.io/deploymentMode}'
```

Expected output for GPU workloads: `RawDeployment`

## Common Issues

### 1. Pod Not Scheduling to GPU Node

**Symptoms**: Pod stuck in Pending state

**Diagnosis**:
```bash
kubectl describe pod <pod-name> -n <namespace>
# Look for "FailedScheduling" events
```

**Common Causes**:
- nodeSelector not matching any GPU nodes
- GPU resources not available (check `nvidia.com/gpu` allocatable)
- Taints on GPU nodes without matching tolerations

**Resolution**:
1. Verify GPU nodes exist: `kubectl get nodes -l gpu=nvidia`
2. Check GPU availability: `kubectl describe node <gpu-node>`
3. Verify AIModel/Recipe has correct nodeSelector

### 2. Knative Admission Rejection

**Symptoms**: InferenceService creation fails with validation error about nodeSelector

**Diagnosis**:
```bash
kubectl describe isvc <model-name> -n <namespace>
# Look for admission webhook errors
```

**Resolution**:
- Ensure deploymentMode is set to "RawDeployment" for GPU workloads
- Check that the operator is using the updated version with explicit mode selection

### 3. Model Loading Timeout

**Symptoms**: Pod restarts repeatedly, logs show model loading timeout

**Diagnosis**:
```bash
kubectl logs <pod-name> -n <namespace> --previous
```

**Resolution**:
- Increase startup probe timeout in the recipe
- Verify sufficient GPU memory for the model size
- Check model download speed from S3/HuggingFace

## Deployment Commands

### Deploy a GPU Model

1. Create AIModel CR:
```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3-8b
  namespace: <namespace>
spec:
  modelName: llama-3-8b
  modelID: meta-llama/Meta-Llama-3-8B-Instruct
  runtime: vllm
  deploymentMode: RawDeployment
  resources:
    requests:
      nvidia.com/gpu: "1"
```

2. Apply and verify:
```bash
kubectl apply -f aimodel.yaml
kubectl get isvc -n <namespace>
kubectl get pods -n <namespace> -l serving.kserve.io/inferenceservice=llama-3-8b
```

## Monitoring

### Check GPU Utilization
```bash
kubectl exec -it <pod-name> -n <namespace> -- nvidia-smi
```

### View Model Logs
```bash
kubectl logs -f <pod-name> -n <namespace>
```

## Related Documentation

- [AI Model Operator](../operators/ai-model-operator.md)
- [vLLM Best Practices](../platform/vllm-best-practices.md)
