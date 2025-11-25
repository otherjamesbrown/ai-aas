# GPU Node Configuration for KServe

This document describes how to configure GPU nodes for KServe InferenceServices.

## Prerequisites

- NVIDIA GPU Operator installed and running
- GPU nodes available in the cluster

## Verify GPU Operator

```bash
# Check GPU operator pods
kubectl get pods -n gpu-operator-resources

# Verify driver installation
kubectl get nodes -o=jsonpath='{range .items[*]}{.metadata.name}{"\t"}{.status.allocatable.nvidia\.com/gpu}{"\n"}{end}'
```

## Label GPU Nodes for KServe

KServe uses node labels to schedule InferenceServices on appropriate nodes.

### Label all GPU nodes

```bash
# Label GPU nodes for KServe
kubectl label nodes -l nvidia.com/gpu.present=true kserve.io/gpu=true

# Verify labels
kubectl get nodes -l kserve.io/gpu=true
```

### Label nodes with specific GPU types (optional)

```bash
# Example: Label nodes with A100 GPUs
kubectl label nodes <node-name> nvidia.com/gpu.product=NVIDIA-A100-SXM4-40GB

# Example: Label nodes with V100 GPUs
kubectl label nodes <node-name> nvidia.com/gpu.product=NVIDIA-Tesla-V100-SXM2-16GB
```

## Test GPU Scheduling

Deploy a test pod to verify GPU scheduling:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: gpu-test
spec:
  restartPolicy: Never
  containers:
    - name: cuda
      image: nvidia/cuda:11.8.0-base-ubuntu22.04
      command:
        - nvidia-smi
      resources:
        limits:
          nvidia.com/gpu: 1
  nodeSelector:
    kserve.io/gpu: "true"
  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
```

Apply and verify:

```bash
kubectl apply -f gpu-test.yaml
kubectl logs gpu-test
# Should show GPU information
```

## Tolerations for GPU Nodes

If GPU nodes have taints, ensure InferenceServices include appropriate tolerations:

```yaml
spec:
  predictor:
    tolerations:
      - key: nvidia.com/gpu
        operator: Exists
        effect: NoSchedule
```

## Storage for Model Caching (Optional)

Create a StorageClass for fast model loading:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: fast-ssd
provisioner: kubernetes.io/gce-pd  # Or appropriate provisioner
parameters:
  type: pd-ssd
  replication-type: none
allowVolumeExpansion: true
```

## Verification Checklist

- [ ] GPU Operator pods healthy
- [ ] GPU nodes labeled with `kserve.io/gpu=true`
- [ ] Test pod can access GPU via `nvidia-smi`
- [ ] GPU resource limits visible: `kubectl describe node <gpu-node> | grep nvidia.com/gpu`
