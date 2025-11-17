#!/usr/bin/env bash
# Deploy vLLM with retry logic - check GPU availability, wait with exponential backoff
# Usage: ./scripts/vllm/deploy-with-retry.sh <release-name> <chart-path> [values-file] [namespace] [max-wait-minutes]

set -euo pipefail

RELEASE_NAME="${1:-}"
CHART_PATH="${2:-}"
VALUES_FILE="${3:-}"
NAMESPACE="${4:-system}"
MAX_WAIT_MINUTES="${5:-30}"

if [ -z "$RELEASE_NAME" ] || [ -z "$CHART_PATH" ]; then
  echo "Usage: $0 <release-name> <chart-path> [values-file] [namespace] [max-wait-minutes]"
  echo "Example: $0 llama-7b-production infra/helm/charts/vllm-deployment values-development.yaml system 30"
  exit 1
fi

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

log_info() {
  printf "${BLUE}[INFO]${NC} %s\n" "$*" >&2
}

log_success() {
  printf "${GREEN}[SUCCESS]${NC} %s\n" "$*" >&2
}

log_error() {
  printf "${RED}[ERROR]${NC} %s\n" "$*" >&2
}

log_warn() {
  printf "${YELLOW}[WARN]${NC} %s\n" "$*" >&2
}

# Check prerequisites
if ! command -v kubectl >/dev/null 2>&1; then
  log_error "kubectl is required but not installed"
  exit 1
fi

if ! command -v helm >/dev/null 2>&1; then
  log_error "helm is required but not installed"
  exit 1
fi

# Check GPU node availability
check_gpu_availability() {
  log_info "Checking GPU node availability..."
  
  # Get GPU nodes
  GPU_NODES=$(kubectl get nodes -l node-type=gpu --no-headers 2>/dev/null | wc -l || echo "0")
  
  if [ "$GPU_NODES" -eq 0 ]; then
    log_error "No GPU nodes found with label 'node-type=gpu'"
    return 1
  fi
  
  log_info "Found ${GPU_NODES} GPU node(s)"
  
  # Check available GPU resources
  AVAILABLE_GPUS=$(kubectl describe nodes -l node-type=gpu 2>/dev/null | grep -c "nvidia.com/gpu.*[0-9]" || echo "0")
  
  if [ "$AVAILABLE_GPUS" -eq 0 ]; then
    log_warn "No available GPUs found (may be allocated)"
    return 1
  fi
  
  log_success "GPU resources available"
  return 0
}

# Wait for GPU availability with exponential backoff
wait_for_gpu() {
  local max_wait_seconds=$((MAX_WAIT_MINUTES * 60))
  local elapsed=0
  local backoff=10  # Start with 10 seconds
  
  log_info "Waiting for GPU availability (max: ${MAX_WAIT_MINUTES} minutes)..."
  
  while [ $elapsed -lt $max_wait_seconds ]; do
    if check_gpu_availability; then
      return 0
    fi
    
    log_warn "GPU not available, waiting ${backoff}s (elapsed: ${elapsed}s)..."
    sleep $backoff
    
    elapsed=$((elapsed + backoff))
    backoff=$((backoff * 2))  # Exponential backoff, max 60s
    if [ $backoff -gt 60 ]; then
      backoff=60
    fi
  done
  
  log_error "Timeout waiting for GPU availability after ${MAX_WAIT_MINUTES} minutes"
  return 1
}

# Deploy with Helm
deploy_helm() {
  local helm_args=("install" "$RELEASE_NAME" "$CHART_PATH" "-n" "$NAMESPACE" "--create-namespace")
  
  if [ -n "$VALUES_FILE" ]; then
    helm_args+=("-f" "$VALUES_FILE")
  fi
  
  log_info "Deploying with Helm: helm ${helm_args[*]}"
  
  if helm "${helm_args[@]}"; then
    log_success "Helm deployment initiated"
    return 0
  else
    log_error "Helm deployment failed"
    return 1
  fi
}

# Main deployment flow
main() {
  log_info "Starting deployment with retry logic"
  log_info "  Release: ${RELEASE_NAME}"
  log_info "  Chart: ${CHART_PATH}"
  log_info "  Namespace: ${NAMESPACE}"
  log_info "  Max wait: ${MAX_WAIT_MINUTES} minutes"
  
  # Check if release already exists
  if helm list -n "$NAMESPACE" | grep -q "^${RELEASE_NAME}\s"; then
    log_warn "Release ${RELEASE_NAME} already exists"
    read -p "Do you want to upgrade it? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
      log_info "Deployment cancelled"
      exit 0
    fi
    log_info "Upgrading existing release..."
    helm upgrade "$RELEASE_NAME" "$CHART_PATH" -n "$NAMESPACE" ${VALUES_FILE:+-f "$VALUES_FILE"}
    log_success "Release upgraded"
    return 0
  fi
  
  # Wait for GPU if needed
  if ! check_gpu_availability; then
    if ! wait_for_gpu; then
      log_error "Cannot proceed without GPU availability"
      exit 1
    fi
  fi
  
  # Deploy
  if ! deploy_helm; then
    log_error "Deployment failed"
    exit 1
  fi
  
  log_success "Deployment complete!"
  log_info "Next steps:"
  log_info "  1. Wait for pod to be ready: kubectl wait --for=condition=ready pod -l app.kubernetes.io/instance=${RELEASE_NAME} -n ${NAMESPACE} --timeout=600s"
  log_info "  2. Verify deployment: ./scripts/vllm/verify-deployment.sh ${RELEASE_NAME} ${NAMESPACE}"
}

main "$@"

