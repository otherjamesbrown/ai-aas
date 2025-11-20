#!/bin/bash
#
# Setup GPU node on Linode LKE cluster
#
# This script helps you add a GPU node pool to your Linode LKE cluster
# for running vLLM inference workloads.
#

set -euo pipefail

# Colors
readonly GREEN='\033[0;32m'
readonly RED='\033[0;31m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

log_info() { printf "${BLUE}[INFO]${NC} %s\n" "$*"; }
log_success() { printf "${GREEN}[SUCCESS]${NC} %s\n" "$*"; }
log_error() { printf "${RED}[ERROR]${NC} %s\n" "$*"; }
log_warn() { printf "${YELLOW}[WARN]${NC} %s\n" "$*"; }

# Configuration
CLUSTER_ID="${CLUSTER_ID:-lke531921}"
REGION="${REGION:-fr-par-2}"
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/kubeconfigs/kubeconfig-development.yaml}"

log_info "GPU Node Setup for Linode LKE"
echo ""
log_info "Cluster: ${CLUSTER_ID}"
log_info "Region: ${REGION}"
echo ""

# Step 1: Check if linode-cli is installed
if ! command -v linode-cli >/dev/null 2>&1; then
    log_warn "linode-cli is not installed"
    echo ""
    log_info "To install linode-cli:"
    echo "  pip3 install linode-cli"
    echo "  linode-cli configure"
    echo ""
    log_info "Alternatively, use the Linode Cloud Console:"
    echo "  https://cloud.linode.com/kubernetes/clusters/${CLUSTER_ID}"
    echo ""
    exit 1
fi

# Step 2: Refresh kubeconfig
log_info "Refreshing kubeconfig..."
if linode-cli lke kubeconfig-view "${CLUSTER_ID}" --no-headers --text 2>/dev/null > /tmp/kubeconfig-new.yaml; then
    cp /tmp/kubeconfig-new.yaml "${KUBECONFIG_PATH}"
    log_success "Kubeconfig refreshed: ${KUBECONFIG_PATH}"
    rm /tmp/kubeconfig-new.yaml
else
    log_error "Failed to refresh kubeconfig. You may need to:"
    echo "  1. Run: linode-cli configure"
    echo "  2. Manually download from: https://cloud.linode.com/kubernetes/clusters/${CLUSTER_ID}"
    exit 1
fi

# Step 3: Test cluster connection
log_info "Testing cluster connection..."
export KUBECONFIG="${KUBECONFIG_PATH}"
if ! kubectl cluster-info >/dev/null 2>&1; then
    log_error "Cannot connect to cluster"
    exit 1
fi
log_success "Connected to cluster"

# Step 4: Check current nodes
log_info "Current nodes:"
kubectl get nodes -o wide

# Step 5: Check for existing GPU nodes
log_info "Checking for GPU nodes..."
GPU_NODES=$(kubectl get nodes -l node-type=gpu --no-headers 2>/dev/null | wc -l)
if [ "${GPU_NODES}" -gt 0 ]; then
    log_warn "Found ${GPU_NODES} GPU node(s) already labeled"
    kubectl get nodes -l node-type=gpu
    echo ""
    read -p "Do you want to add another GPU node? (y/N): " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        log_info "Exiting. No changes made."
        exit 0
    fi
fi

# Step 6: Show available GPU instance types
log_info "Available GPU instance types in Linode:"
echo ""
echo "┌─────────────────────┬──────────┬───────┬────────┬──────────────┐"
echo "│ Type                │ GPU      │ vCPUs │ RAM    │ Cost/Month*  │"
echo "├─────────────────────┼──────────┼───────┼────────┼──────────────┤"
echo "│ g1-gpu-rtx6000-1    │ RTX 6000 │ 8     │ 32GB   │ ~\$1,000     │"
echo "│ g1-gpu-rtx6000-2    │ RTX 6000 │ 16    │ 64GB   │ ~\$2,000     │"
echo "│ g1-gpu-rtx6000-4    │ RTX 6000 │ 32    │ 128GB  │ ~\$4,000     │"
echo "└─────────────────────┴──────────┴───────┴────────┴──────────────┘"
echo ""
echo "*Prices are approximate and vary by region"
echo ""
log_warn "Note: Linode doesn't have RTX 4000 Ada. RTX 6000 (24GB) is the closest option."
echo ""

# Step 7: Provide instructions
echo "════════════════════════════════════════════════════════════════"
log_info "To add a GPU node pool, choose ONE of these methods:"
echo "════════════════════════════════════════════════════════════════"
echo ""

echo "Method 1: Using linode-cli (Command Line)"
echo "─────────────────────────────────────────"
cat <<'EOF'

# Add a GPU node pool with RTX 6000 (24GB)
linode-cli lke pool-create \
  lke531921 \
  --type g1-gpu-rtx6000-1 \
  --count 1 \
  --tags gpu-pool

# Check pool creation
linode-cli lke pools-list lke531921

EOF

echo ""
echo "Method 2: Using Linode Cloud Console (Web UI)"
echo "──────────────────────────────────────────────"
echo ""
echo "1. Go to: https://cloud.linode.com/kubernetes/clusters/${CLUSTER_ID}"
echo "2. Click 'Add a Node Pool'"
echo "3. Select 'Dedicated GPU' tab"
echo "4. Choose 'RTX 6000' (24GB)"
echo "5. Set count: 1"
echo "6. Click 'Add Pool'"
echo ""

echo "════════════════════════════════════════════════════════════════"
log_info "After adding the GPU node pool:"
echo "════════════════════════════════════════════════════════════════"
echo ""
echo "1. Wait for node to be ready (3-5 minutes):"
echo "   watch kubectl get nodes"
echo ""
echo "2. Label the GPU node:"
echo "   GPU_NODE=\$(kubectl get nodes | grep gpu | awk '{print \$1}')"
echo "   kubectl label nodes \$GPU_NODE node-type=gpu"
echo "   kubectl taint nodes \$GPU_NODE gpu-workload=true:NoSchedule"
echo ""
echo "3. Install NVIDIA device plugin (if not installed):"
echo "   kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml"
echo ""
echo "4. Verify GPU is available:"
echo "   kubectl describe node \$GPU_NODE | grep nvidia.com/gpu"
echo ""
echo "5. Deploy vLLM:"
echo "   cd /home/dev/ai-aas"
echo "   ./scripts/deploy-vllm-linode.sh"
echo ""

log_success "Setup instructions complete!"
echo ""
log_info "For help or issues, see: docs/RTX4000_ADA_DEPLOYMENT.md"
