#!/usr/bin/env bash
# Apply labels and taints to GPU nodes after Terraform creates them
# Usage: ./scripts/infra/apply-gpu-node-labels.sh [kubeconfig-path]
#
# Supports:
#   - RTX6000 (g1-gpu-rtx6000*) - Labeled as gpu-class=rtx6000
#   - RTX4000 Ada (g2-gpu-rtx4000a*) - Labeled as gpu-class=rtx4000
#
# Model targeting example in Helm values:
#   nodeSelector:
#     ai-aas.io/gpu-class: rtx6000  # For larger models (13B+)
#   # or
#     ai-aas.io/gpu-class: rtx4000  # For smaller models (7B, embeddings)

set -euo pipefail

KUBECONFIG="${1:-${KUBECONFIG:-}}"
if [ -z "$KUBECONFIG" ]; then
  echo "Error: KUBECONFIG not set. Please provide kubeconfig path or set KUBECONFIG env var."
  exit 1
fi

export KUBECONFIG

echo "Applying labels and taints to GPU nodes..."

# Function to label GPU nodes by instance type pattern
label_gpu_nodes() {
  local pattern="$1"
  local gpu_class="$2"
  local gpu_product="$3"

  local nodes
  nodes=$(kubectl get nodes -o json | jq -r --arg pattern "$pattern" \
    '.items[] | select(.metadata.labels."node.kubernetes.io/instance-type" | startswith($pattern)) | .metadata.name')

  if [ -z "$nodes" ]; then
    echo "No $gpu_class nodes found (pattern: $pattern)"
    return 0
  fi

  for node in $nodes; do
    echo "Processing $gpu_class node: $node"

    # Apply common GPU labels
    kubectl label node "$node" \
      role=gpu \
      node-type=gpu \
      ai-aas.io/gpu-class="$gpu_class" \
      nvidia.com/gpu.product="$gpu_product" \
      --overwrite

    # Apply taint to prevent non-GPU workloads
    kubectl taint node "$node" gpu-workload=true:NoSchedule --overwrite 2>/dev/null || true

    echo "  ✓ Node $node configured as $gpu_class"
  done
}

# Label RTX6000 nodes (24GB VRAM - for larger models)
# Note: RTX6000 is NOT supported by LKE, but kept for standalone GPU nodes
label_gpu_nodes "g1-gpu-rtx6000" "rtx6000" "NVIDIA-RTX-6000"

# Label RTX4000 Ada Medium nodes (20GB VRAM, 32GB RAM)
label_gpu_nodes "g2-gpu-rtx4000a1-m" "rtx4000-medium" "NVIDIA-RTX-4000-Ada"

# Label RTX4000 Ada Large nodes (20GB VRAM, 64GB RAM)
label_gpu_nodes "g2-gpu-rtx4000a1-l" "rtx4000-large" "NVIDIA-RTX-4000-Ada"

# Label RTX4000 Ada XL nodes (20GB VRAM, 128GB RAM)
label_gpu_nodes "g2-gpu-rtx4000a1-xl" "rtx4000-xlarge" "NVIDIA-RTX-4000-Ada"

# Label RTX4000 Ada Small nodes (20GB VRAM, 16GB RAM) - fallback
label_gpu_nodes "g2-gpu-rtx4000a1-s" "rtx4000-small" "NVIDIA-RTX-4000-Ada"

# Generic fallback for any other RTX4000 variant
label_gpu_nodes "g2-gpu-rtx4000a" "rtx4000" "NVIDIA-RTX-4000-Ada"

echo ""
echo "GPU nodes configured:"
kubectl get nodes -l node-type=gpu -o custom-columns=\
NAME:.metadata.name,\
INSTANCE-TYPE:.metadata.labels.node\\.kubernetes\\.io/instance-type,\
GPU-CLASS:.metadata.labels.ai-aas\\.io/gpu-class,\
GPU:.status.allocatable.nvidia\\.com/gpu 2>/dev/null || \
kubectl get nodes -l node-type=gpu

