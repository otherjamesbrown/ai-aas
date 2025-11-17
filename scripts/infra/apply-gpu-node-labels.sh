#!/usr/bin/env bash
# Apply labels and taints to GPU nodes after Terraform creates them
# Usage: ./scripts/infra/apply-gpu-node-labels.sh [kubeconfig-path]

set -euo pipefail

KUBECONFIG="${1:-${KUBECONFIG:-}}"
if [ -z "$KUBECONFIG" ]; then
  echo "Error: KUBECONFIG not set. Please provide kubeconfig path or set KUBECONFIG env var."
  exit 1
fi

export KUBECONFIG

echo "Applying labels and taints to GPU nodes..."

# Get GPU nodes (nodes with g1-gpu instance type)
GPU_NODES=$(kubectl get nodes -o json | jq -r '.items[] | select(.metadata.labels."node.kubernetes.io/instance-type" == "g1-gpu-rtx6000") | .metadata.name')

if [ -z "$GPU_NODES" ]; then
  echo "No GPU nodes found. Waiting for nodes to be created..."
  echo "Run this script again after Terraform creates the GPU node pool."
  exit 0
fi

for node in $GPU_NODES; do
  echo "Processing node: $node"
  
  # Apply labels
  echo "  Applying labels: role=gpu, node-type=gpu"
  kubectl label node "$node" role=gpu node-type=gpu --overwrite
  
  # Apply taint (remove existing gpu-workload taints first, then add)
  echo "  Applying taint: gpu-workload=true:NoSchedule"
  kubectl taint node "$node" gpu-workload=true:NoSchedule --overwrite
  
  echo "  ✓ Node $node configured"
done

echo ""
echo "GPU nodes configured:"
kubectl get nodes -l node-type=gpu -o custom-columns=NAME:.metadata.name,TYPE:.metadata.labels.node\\.kubernetes\\.io/instance-type,GPU:.status.allocatable.nvidia\\.com/gpu,TAINTS:.spec.taints

