#!/usr/bin/env bash
# Verify vLLM deployment - check pod status, health endpoints, and test completion
# Usage: ./scripts/vllm/verify-deployment.sh <release-name> [namespace] [timeout]

set -euo pipefail

RELEASE_NAME="${1:-}"
NAMESPACE="${2:-system}"
TIMEOUT="${3:-600}"  # 10 minutes default

if [ -z "$RELEASE_NAME" ]; then
  echo "Usage: $0 <release-name> [namespace] [timeout-seconds]"
  echo "Example: $0 llama-7b-production system 600"
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

# Check if kubectl is available
if ! command -v kubectl >/dev/null 2>&1; then
  log_error "kubectl is required but not installed"
  exit 1
fi

# Get pod name
log_info "Finding pod for release: ${RELEASE_NAME} in namespace: ${NAMESPACE}"
POD_NAME=$(kubectl get pods -n "${NAMESPACE}" -l "app.kubernetes.io/instance=${RELEASE_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$POD_NAME" ]; then
  log_error "No pod found for release: ${RELEASE_NAME}"
  exit 1
fi

log_info "Found pod: ${POD_NAME}"

# Check pod status
log_info "Checking pod status..."
POD_STATUS=$(kubectl get pod "${POD_NAME}" -n "${NAMESPACE}" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")

if [ "$POD_STATUS" != "Running" ]; then
  log_error "Pod is not running. Status: ${POD_STATUS}"
  log_info "Pod events:"
  kubectl describe pod "${POD_NAME}" -n "${NAMESPACE}" | grep -A 10 "Events:" || true
  exit 1
fi

log_success "Pod is running"

# Wait for pod to be ready
log_info "Waiting for pod to be ready (timeout: ${TIMEOUT}s)..."
if ! kubectl wait --for=condition=ready pod/"${POD_NAME}" -n "${NAMESPACE}" --timeout="${TIMEOUT}s" 2>/dev/null; then
  log_error "Pod did not become ready within ${TIMEOUT} seconds"
  log_info "Pod description:"
  kubectl describe pod "${POD_NAME}" -n "${NAMESPACE}" || true
  exit 1
fi

log_success "Pod is ready"

# Get service endpoint
SERVICE_NAME=$(kubectl get svc -n "${NAMESPACE}" -l "app.kubernetes.io/instance=${RELEASE_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
if [ -z "$SERVICE_NAME" ]; then
  log_error "No service found for release: ${RELEASE_NAME}"
  exit 1
fi

# Port forward to access the service
log_info "Setting up port forwarding to service: ${SERVICE_NAME}"
kubectl port-forward -n "${NAMESPACE}" "svc/${SERVICE_NAME}" 8000:8000 >/dev/null 2>&1 &
PORT_FORWARD_PID=$!

# Cleanup port forward on exit
trap "kill $PORT_FORWARD_PID 2>/dev/null || true" EXIT

# Wait for port forward to be ready
sleep 2

# Check /health endpoint
log_info "Checking /health endpoint..."
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8000/health 2>/dev/null || echo "000")
HEALTH_CODE=$(echo "$HEALTH_RESPONSE" | tail -n1)
HEALTH_BODY=$(echo "$HEALTH_RESPONSE" | head -n-1)

if [ "$HEALTH_CODE" != "200" ]; then
  log_error "Health check failed. HTTP code: ${HEALTH_CODE}"
  log_info "Response: ${HEALTH_BODY}"
  exit 1
fi

log_success "Health endpoint is healthy (HTTP ${HEALTH_CODE})"

# Check /ready endpoint
log_info "Checking /ready endpoint..."
READY_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8000/ready 2>/dev/null || echo "000")
READY_CODE=$(echo "$READY_RESPONSE" | tail -n1)
READY_BODY=$(echo "$READY_RESPONSE" | head -n-1)

if [ "$READY_CODE" != "200" ]; then
  log_error "Readiness check failed. HTTP code: ${READY_CODE}"
  log_info "Response: ${READY_BODY}"
  exit 1
fi

log_success "Ready endpoint is ready (HTTP ${READY_CODE})"

# Test completion endpoint
log_info "Testing /v1/chat/completions endpoint..."
START_TIME=$(date +%s)
COMPLETION_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 10
  }' 2>/dev/null || echo "000")
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

COMPLETION_CODE=$(echo "$COMPLETION_RESPONSE" | tail -n1)
COMPLETION_BODY=$(echo "$COMPLETION_RESPONSE" | head -n-1)

if [ "$COMPLETION_CODE" != "200" ]; then
  log_error "Completion test failed. HTTP code: ${COMPLETION_CODE}"
  log_info "Response: ${COMPLETION_BODY}"
  exit 1
fi

if [ "$DURATION" -gt 3 ]; then
  log_warn "Completion response time (${DURATION}s) exceeds target (≤3s)"
else
  log_success "Completion endpoint responded in ${DURATION}s (target: ≤3s)"
fi

log_success "Completion endpoint test passed (HTTP ${COMPLETION_CODE})"

# Summary
log_success "Deployment verification complete!"
log_info "Summary:"
log_info "  - Pod: ${POD_NAME} (${POD_STATUS})"
log_info "  - Service: ${SERVICE_NAME}"
log_info "  - Health: ✓"
log_info "  - Ready: ✓"
log_info "  - Completion: ✓ (${DURATION}s)"

