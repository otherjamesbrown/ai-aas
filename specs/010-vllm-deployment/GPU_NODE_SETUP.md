# GPU Node Setup for Development Cluster

**Date**: 2025-01-27  
**Status**: Configuration updated, ready for Terraform apply

## Configuration Changes

### Updated Development Cluster Node Pools

**File**: `infra/terraform/environments/_shared/locals.tf`

**Changes**:
1. **Reduced CPU nodes**: From 3 to 2 nodes (`g6-standard-4`)
2. **Added GPU node pool**: 1 node (`g1-gpu-rtx6000`)

**New Configuration**:
```hcl
development = {
  node_pools = [
    {
      type  = "g6-standard-4"      # CPU nodes
      count = 2                     # Reduced from 3
      autoscaler = {
        min = 2
        max = 4
      }
    },
    {
      type  = "g1-gpu-rtx6000"      # GPU node
      count = 1                     # Single GPU node for dev
      autoscaler = {
        min = 1
        max = 1                     # No autoscaling (cost control)
      }
    }
  ]
}
```

## Deployment Steps

### Step 1: Apply Terraform Changes

```bash
cd infra/terraform/environments/development
terraform init
terraform plan
terraform apply
```

**Expected Result**:
- 2 CPU nodes (`g6-standard-4`)
- 1 GPU node (`g1-gpu-rtx6000`)

### Step 2: Apply Labels and Taints

**Note**: The Linode Terraform provider may not support labels/taints in node pools. Apply them manually after node creation:

```bash
# Set kubeconfig
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Run the script
./scripts/infra/apply-gpu-node-labels.sh ~/kubeconfigs/kubeconfig-development.yaml
```

**Or manually**:
```bash
# Get GPU node names (handles multiple nodes)
GPU_NODES=$(kubectl get nodes -o json | jq -r '.items[] | select(.metadata.labels."node.kubernetes.io/instance-type" == "g1-gpu-rtx6000") | .metadata.name')

# Apply labels and taints to each GPU node
for node in $GPU_NODES; do
  # Apply labels
  kubectl label node "$node" role=gpu node-type=gpu --overwrite
  
  # Apply taint
  kubectl taint node "$node" gpu-workload=true:NoSchedule --overwrite
done
```

### Step 3: Verify GPU Node

```bash
# Check node exists
kubectl get nodes -l node-type=gpu

# Check GPU resources
kubectl get nodes -o custom-columns=NAME:.metadata.name,TYPE:.metadata.labels.node\\.kubernetes\\.io/instance-type,GPU:.status.allocatable.nvidia\\.com/gpu

# Verify taints
kubectl describe node <gpu-node-name> | grep -A 5 Taints
```

**Expected Output**:
- Node should show `nvidia.com/gpu: 1` (or similar)
- Node should have `node-type=gpu` label
- Node should have `gpu-workload=true:NoSchedule` taint

### Step 4: Install NVIDIA Device Plugin (if needed)

Linode LKE should automatically install the NVIDIA device plugin, but verify:

```bash
kubectl get daemonset -n kube-system | grep nvidia
```

If not present, install it:
```bash
kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.1/nvidia-device-plugin.yml
```

## Cost Estimate

**GPU Node** (`g1-gpu-rtx6000`):
- **Monthly Cost**: ~$1,200/month
- **Hourly Cost**: ~$1.67/hour

**Total Dev Cluster**:
- 2x `g6-standard-4`: ~$60/month
- 1x `g1-gpu-rtx6000`: ~$1,200/month
- **Total**: ~$1,260/month

## Testing vLLM Deployment

Once GPU node is ready:

```bash
# Deploy vLLM model
./scripts/vllm/deploy-with-retry.sh test-llama-7b \
  infra/helm/charts/vllm-deployment \
  infra/helm/charts/vllm-deployment/values-development.yaml \
  system 10

# Verify deployment
kubectl get pods -n system -l app.kubernetes.io/name=vllm-deployment

# Test inference
./scripts/vllm/test-inference-endpoint.sh dev-key-123 test-llama-7b
```

## Troubleshooting

### GPU Node Not Showing GPU Resources

1. **Check NVIDIA device plugin**:
   ```bash
   kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset
   ```

2. **Check node conditions**:
   ```bash
   kubectl describe node <gpu-node-name> | grep -A 10 Conditions
   ```

3. **Check node allocatable resources**:
   ```bash
   kubectl get node <gpu-node-name> -o jsonpath='{.status.allocatable.nvidia\.com/gpu}'
   ```

### Pods Not Scheduling on GPU Node

1. **Check node selector**:
   ```bash
   kubectl get pods -n system -o yaml | grep nodeSelector
   ```

2. **Check tolerations**:
   ```bash
   kubectl get pods -n system -o yaml | grep -A 5 tolerations
   ```

3. **Check node taints**:
   ```bash
   kubectl describe node <gpu-node-name> | grep Taints
   ```

## Notes

- **Autoscaling**: GPU node pool has `max: 1` to prevent cost overruns
- **Labels**: `node-type=gpu` matches vLLM chart requirements
- **Taints**: `gpu-workload=true:NoSchedule` matches vLLM chart tolerations
- **Cost Control**: Single GPU node is sufficient for dev/testing

