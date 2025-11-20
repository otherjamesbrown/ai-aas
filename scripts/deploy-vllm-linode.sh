#!/bin/bash
#
# Deploy vLLM on Linode LKE with RTX 6000 GPU
#
# This script deploys a vLLM model instance to your Linode cluster.
# Models are automatically downloaded from HuggingFace on first run.
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

# Default values
MODEL_PATH="${1:-meta-llama/Llama-2-7b-chat-hf}"
RELEASE_NAME="${2:-$(echo "$MODEL_PATH" | sed 's/.*\///' | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9-]/-/g')}"
NAMESPACE="${3:-system}"
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/kubeconfigs/kubeconfig-development.yaml}"

export KUBECONFIG="${KUBECONFIG_PATH}"

echo "════════════════════════════════════════════════════════════════"
log_info "vLLM Deployment on Linode LKE"
echo "════════════════════════════════════════════════════════════════"
echo ""
log_info "Model: ${MODEL_PATH}"
log_info "Release: ${RELEASE_NAME}"
log_info "Namespace: ${NAMESPACE}"
echo ""

# Pre-flight checks
log_info "Running pre-flight checks..."

# Check kubectl
if ! command -v kubectl >/dev/null 2>&1; then
    log_error "kubectl is not installed"
    exit 1
fi

# Check helm
if ! command -v helm >/dev/null 2>&1; then
    log_error "helm is not installed"
    exit 1
fi

# Check cluster connection
if ! kubectl cluster-info >/dev/null 2>&1; then
    log_error "Cannot connect to cluster. Run: ./scripts/setup-gpu-node.sh"
    exit 1
fi
log_success "Connected to cluster"

# Check for GPU nodes
GPU_NODES=$(kubectl get nodes -l node-type=gpu --no-headers 2>/dev/null | wc -l)
if [ "${GPU_NODES}" -eq 0 ]; then
    log_error "No GPU nodes found with label 'node-type=gpu'"
    echo ""
    log_info "To add GPU nodes, run:"
    echo "  ./scripts/setup-gpu-node.sh"
    exit 1
fi
log_success "Found ${GPU_NODES} GPU node(s)"

# Check NVIDIA device plugin
if ! kubectl get daemonset -n kube-system nvidia-device-plugin-daemonset >/dev/null 2>&1; then
    log_warn "NVIDIA device plugin not found"
    read -p "Install NVIDIA device plugin? (Y/n): " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        kubectl apply -f https://raw.githubusercontent.com/NVIDIA/k8s-device-plugin/v0.14.0/nvidia-device-plugin.yml
        log_success "NVIDIA device plugin installed"
        sleep 5
    fi
fi

# Verify GPU is allocatable
log_info "Verifying GPU availability..."
GPU_AVAILABLE=$(kubectl get nodes -l node-type=gpu -o json | \
    jq -r '.items[].status.allocatable."nvidia.com/gpu"' | \
    awk '{s+=$1} END {print s}')

if [ -z "${GPU_AVAILABLE}" ] || [ "${GPU_AVAILABLE}" -eq 0 ]; then
    log_error "No GPUs available for allocation"
    echo ""
    log_info "Check GPU nodes:"
    kubectl describe nodes -l node-type=gpu | grep -A 5 "Allocatable"
    exit 1
fi
log_success "GPU available for allocation: ${GPU_AVAILABLE}"

# Create namespace if it doesn't exist
if ! kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
    log_info "Creating namespace: ${NAMESPACE}"
    kubectl create namespace "${NAMESPACE}"
fi

# Deploy vLLM
echo ""
log_info "Deploying vLLM..."
echo ""
log_info "Model seeding information:"
echo "  - Model will be downloaded from HuggingFace on first run"
echo "  - Download size: ~13GB for 7B models, ~25GB for 13B models"
echo "  - Download time: 5-15 minutes depending on connection"
echo "  - Models are cached in the pod's storage"
echo ""
log_warn "First deployment will take longer due to model download!"
echo ""

# Determine model size for timeout
if echo "${MODEL_PATH}" | grep -qi "70b"; then
    MODEL_SIZE="large"
elif echo "${MODEL_PATH}" | grep -qi "13b\|30b\|34b"; then
    MODEL_SIZE="medium"
else
    MODEL_SIZE="small"
fi

log_info "Detected model size: ${MODEL_SIZE}"

# Deploy with Helm (using RTX 6000 optimized values for Linode)
helm install "${RELEASE_NAME}" \
    infra/helm/charts/vllm-deployment \
    -f infra/helm/charts/vllm-deployment/values-rtx4000ada.yaml \
    --set model.path="${MODEL_PATH}" \
    --set model.size="${MODEL_SIZE}" \
    --set resources.limits.memory="22Gi" \
    --set resources.requests.memory="20Gi" \
    --set vllm.env[0].name=VLLM_GPU_MEMORY_UTILIZATION \
    --set vllm.env[0].value="0.85" \
    --namespace "${NAMESPACE}" \
    --timeout 20m \
    --wait

log_success "Deployment initiated!"

# Monitor deployment
echo ""
log_info "Monitoring deployment (this may take 10-20 minutes for first run)..."
log_info "Press Ctrl+C to stop monitoring (deployment will continue)"
echo ""

# Watch pod status
kubectl wait --for=condition=Ready \
    pod -l app.kubernetes.io/name=vllm-deployment,app.kubernetes.io/instance="${RELEASE_NAME}" \
    -n "${NAMESPACE}" \
    --timeout=20m || {
    log_error "Deployment timed out or failed"
    echo ""
    log_info "Check pod status:"
    kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/instance="${RELEASE_NAME}"
    echo ""
    log_info "Check logs:"
    kubectl logs -n "${NAMESPACE}" -l app.kubernetes.io/instance="${RELEASE_NAME}" --tail=50
    exit 1
}

# Deployment successful
echo ""
log_success "═══════════════════════════════════════════════════════════════"
log_success "vLLM Deployment Complete!"
log_success "═══════════════════════════════════════════════════════════════"
echo ""

# Show deployment info
log_info "Deployment Information:"
POD_NAME=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/instance="${RELEASE_NAME}" -o jsonpath='{.items[0].metadata.name}')
NODE_NAME=$(kubectl get pods -n "${NAMESPACE}" -l app.kubernetes.io/instance="${RELEASE_NAME}" -o jsonpath='{.items[0].spec.nodeName}')
SERVICE_NAME=$(kubectl get svc -n "${NAMESPACE}" -l app.kubernetes.io/instance="${RELEASE_NAME}" -o jsonpath='{.items[0].metadata.name}')

echo "  Pod: ${POD_NAME}"
echo "  Node: ${NODE_NAME}"
echo "  Service: ${SERVICE_NAME}"
echo "  Model: ${MODEL_PATH}"
echo ""

# Test the deployment
log_info "Testing deployment..."
echo ""
log_info "Port-forwarding to service..."
kubectl port-forward -n "${NAMESPACE}" "svc/${SERVICE_NAME}" 8000:8000 >/dev/null 2>&1 &
PF_PID=$!
trap "kill ${PF_PID} 2>/dev/null || true" EXIT

sleep 3

# Test health endpoint
if curl -sf http://localhost:8000/health >/dev/null; then
    log_success "Health check: OK"
else
    log_error "Health check: FAILED"
fi

# Test inference endpoint
log_info "Testing inference..."
RESPONSE=$(curl -sf -X POST http://localhost:8000/v1/chat/completions \
    -H "Content-Type: application/json" \
    -d "{
        \"model\": \"${MODEL_PATH}\",
        \"messages\": [{\"role\": \"user\", \"content\": \"Say 'hello' in one word\"}],
        \"max_tokens\": 5
    }" 2>/dev/null || echo "")

if [ -n "${RESPONSE}" ]; then
    ANSWER=$(echo "${RESPONSE}" | jq -r '.choices[0].message.content' 2>/dev/null || echo "")
    if [ -n "${ANSWER}" ]; then
        log_success "Inference test: OK"
        log_info "Response: ${ANSWER}"
    else
        log_warn "Inference test: Response received but couldn't parse"
    fi
else
    log_error "Inference test: FAILED"
fi

# Cleanup port-forward
kill ${PF_PID} 2>/dev/null || true
trap - EXIT

echo ""
log_success "═══════════════════════════════════════════════════════════════"
log_info "Next Steps:"
echo "═══════════════════════════════════════════════════════════════"
echo ""
echo "1. Run E2E test:"
echo "   export VLLM_BACKEND_URL=http://${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:8000"
echo "   export VLLM_MODEL_NAME=${MODEL_PATH}"
echo "   ./test-api-inference.sh cluster ${NAMESPACE} ${SERVICE_NAME} ${MODEL_PATH}"
echo ""
echo "2. View logs:"
echo "   kubectl logs -n ${NAMESPACE} ${POD_NAME} -f"
echo ""
echo "3. Access via port-forward:"
echo "   kubectl port-forward -n ${NAMESPACE} svc/${SERVICE_NAME} 8000:8000"
echo ""
echo "4. Configure API Router to use this backend"
echo ""
log_success "Deployment complete!"
