#!/bin/bash
#
# Configure GPU node after it joins the cluster
#
# This script:
# 1. Identifies the newest node (GPU node)
# 2. Labels it with node-type=gpu
# 3. Taints it for GPU workloads only
# 4. Installs NVIDIA device plugin
# 5. Verifies GPU is allocatable
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

KUBECONFIG_PATH="${KUBECONFIG:-$HOME/kubeconfigs/kubeconfig-development.yaml}"
export KUBECONFIG="${KUBECONFIG_PATH}"

echo "════════════════════════════════════════════════════════════════"
log_info "GPU Node Configuration"
echo "════════════════════════════════════════════════════════════════"
echo ""

# Check cluster connection
log_info "Checking cluster connection..."
if ! kubectl cluster-info >/dev/null 2>&1; then
    log_error "Cannot connect to cluster"
    exit 1
fi
log_success "Connected to cluster"
echo ""

# Check number of nodes
NODE_COUNT=$(kubectl get nodes --no-headers | wc -l)
log_info "Current node count: ${NODE_COUNT}"

if [ "${NODE_COUNT}" -lt 4 ]; then
    log_error "Expected 4 nodes (3 original + 1 GPU), but found ${NODE_COUNT}"
    log_info "Current nodes:"
    kubectl get nodes
    exit 1
fi

# Identify the GPU node (newest node)
log_info "Identifying GPU node (newest node)..."
GPU_NODE=$(kubectl get nodes --sort-by=.metadata.creationTimestamp | tail -1 | awk '{print $1}')
log_success "GPU node identified: ${GPU_NODE}"
echo ""

# Show node details
log_info "Node details:"
kubectl get node "${GPU_NODE}" -o wide
echo ""

# Check if already labeled
EXISTING_LABEL=$(kubectl get node "${GPU_NODE}" -o jsonpath='{.metadata.labels.node-type}' 2>/dev/null || echo "")
if [ "${EXISTING_LABEL}" = "gpu" ]; then
    log_warn "Node already labeled with node-type=gpu"
else
    log_info "Labeling node with node-type=gpu..."
    kubectl label nodes "${GPU_NODE}" node-type=gpu
    log_success "Node labeled"
fi
echo ""

# Check if already tainted
EXISTING_TAINT=$(kubectl get node "${GPU_NODE}" -o jsonpath='{.spec.taints[?(@.key=="gpu-workload")].effect}' 2>/dev/null || echo "")
if [ "${EXISTING_TAINT}" = "NoSchedule" ]; then
    log_warn "Node already tainted with gpu-workload=true:NoSchedule"
else
    log_info "Tainting node for GPU workloads only..."
    kubectl taint nodes "${GPU_NODE}" gpu-workload=true:NoSchedule
    log_success "Node tainted"
fi
echo ""

# Verify labels and taints
log_info "Verifying node configuration..."
kubectl get node "${GPU_NODE}" -o json | jq '{
    name: .metadata.name,
    labels: {
        "node-type": .metadata.labels."node-type"
    },
    taints: [.spec.taints[] | select(.key == "gpu-workload")]
}'
echo ""

# Install NVIDIA device plugin
log_info "Checking NVIDIA device plugin..."
if kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset >/dev/null 2>&1; then
    log_warn "NVIDIA device plugin already installed"
else
    log_info "Installing NVIDIA device plugin..."
    kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
    log_success "NVIDIA device plugin installed"
fi
echo ""

# Wait for device plugin to start
log_info "Waiting for NVIDIA device plugin to start (30 seconds)..."
sleep 30

# Verify GPU is allocatable
log_info "Verifying GPU availability..."
GPU_AVAILABLE=$(kubectl get node "${GPU_NODE}" -o json | \
    jq -r '.status.allocatable."nvidia.com/gpu"' 2>/dev/null || echo "0")

if [ -z "${GPU_AVAILABLE}" ] || [ "${GPU_AVAILABLE}" = "null" ] || [ "${GPU_AVAILABLE}" -eq 0 ]; then
    log_error "GPU not yet allocatable"
    echo ""
    log_info "Checking device plugin logs..."
    kubectl logs -n kube-system -l name=nvidia-device-plugin-ds --tail=20
    echo ""
    log_warn "GPU may need more time to be detected. Try again in 1-2 minutes."
    exit 1
fi

log_success "GPU available for allocation: ${GPU_AVAILABLE}"
echo ""

# Show full node capacity and allocatable resources
log_info "Node resources:"
kubectl describe node "${GPU_NODE}" | grep -A 10 "Allocatable:"
echo ""

# Success!
echo "════════════════════════════════════════════════════════════════"
log_success "GPU Node Configuration Complete!"
echo "════════════════════════════════════════════════════════════════"
echo ""
log_info "GPU node ready for workloads:"
echo "  Node: ${GPU_NODE}"
echo "  Label: node-type=gpu"
echo "  Taint: gpu-workload=true:NoSchedule"
echo "  GPU: ${GPU_AVAILABLE}x nvidia.com/gpu"
echo ""
log_success "Next step: Deploy vLLM"
echo "  ./scripts/deploy-vllm-linode.sh meta-llama/Llama-2-7b-chat-hf"
echo ""
