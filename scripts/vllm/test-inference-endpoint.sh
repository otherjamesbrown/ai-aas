#!/usr/bin/env bash
# Test inference endpoint on dev cluster
# Usage: ./scripts/vllm/test-inference-endpoint.sh [api-key] [model-name]

set -euo pipefail

API_KEY="${1:-dev-key-123}"
MODEL_NAME="${2:-gpt-4o}"
KUBECONFIG="${KUBECONFIG:-~/kubeconfigs/kubeconfig-development.yaml}"

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

# Set kubeconfig
export KUBECONFIG

# Check if API Router Service is running
log_info "Checking API Router Service status..."
API_ROUTER_POD=$(kubectl get pods -n development -l app.kubernetes.io/name=api-router-service -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$API_ROUTER_POD" ]; then
  log_error "API Router Service pod not found"
  exit 1
fi

POD_STATUS=$(kubectl get pod "$API_ROUTER_POD" -n development -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")
if [ "$POD_STATUS" != "Running" ]; then
  log_error "API Router Service pod is not running. Status: ${POD_STATUS}"
  exit 1
fi

log_success "API Router Service pod is running: ${API_ROUTER_POD}"

# Setup port forwarding
log_info "Setting up port forwarding to API Router Service..."
kubectl port-forward -n development svc/api-router-service-development-api-router-service 8080:8080 >/dev/null 2>&1 &
PORT_FORWARD_PID=$!

# Cleanup port forward on exit
trap "kill $PORT_FORWARD_PID 2>/dev/null || true" EXIT

# Wait for port forward to be ready
sleep 2

# Test 1: Health endpoint
log_info "Test 1: Testing /v1/status/healthz endpoint..."
HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/v1/status/healthz 2>/dev/null || echo "000")
HEALTH_CODE=$(echo "$HEALTH_RESPONSE" | tail -n1)
HEALTH_BODY=$(echo "$HEALTH_RESPONSE" | head -n-1)

if [ "$HEALTH_CODE" = "200" ]; then
  log_success "Health endpoint is healthy (HTTP ${HEALTH_CODE})"
else
  log_error "Health endpoint failed. HTTP code: ${HEALTH_CODE}"
  log_info "Response: ${HEALTH_BODY}"
  exit 1
fi

# Test 2: Readiness endpoint
log_info "Test 2: Testing /v1/status/readyz endpoint..."
READY_RESPONSE=$(curl -s -w "\n%{http_code}" http://localhost:8080/v1/status/readyz 2>/dev/null || echo "000")
READY_CODE=$(echo "$READY_RESPONSE" | tail -n1)
READY_BODY=$(echo "$READY_RESPONSE" | head -n-1)

if [ "$READY_CODE" = "200" ]; then
  log_success "Readiness endpoint is ready (HTTP ${READY_CODE})"
else
  log_warn "Readiness endpoint returned HTTP ${READY_CODE}"
  log_info "Response: ${READY_BODY}"
fi

# Test 3: Inference endpoint (custom format)
log_info "Test 3: Testing /v1/inference endpoint (custom format)..."
INFERENCE_REQUEST='{
  "request_id": "test-'$(date +%s)'",
  "model": "'${MODEL_NAME}'",
  "payload": "Hello, this is a test inference request"
}'

START_TIME=$(date +%s)
INFERENCE_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/v1/inference \
  -H "X-API-Key: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d "${INFERENCE_REQUEST}" 2>/dev/null || echo "000")
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

INFERENCE_CODE=$(echo "$INFERENCE_RESPONSE" | tail -n1)
INFERENCE_BODY=$(echo "$INFERENCE_RESPONSE" | head -n-1)

if [ "$INFERENCE_CODE" = "200" ]; then
  log_success "Inference endpoint responded successfully (HTTP ${INFERENCE_CODE}, ${DURATION}s)"
  if command -v jq >/dev/null 2>&1; then
    echo "$INFERENCE_BODY" | jq . 2>/dev/null || echo "$INFERENCE_BODY"
  else
    echo "$INFERENCE_BODY"
  fi
else
  log_warn "Inference endpoint returned HTTP ${INFERENCE_CODE}"
  log_info "Response: ${INFERENCE_BODY}"
  log_info "Note: This may be expected if no backend is configured for model '${MODEL_NAME}'"
fi

# Test 4: Chat completions endpoint (OpenAI format)
log_info "Test 4: Testing /v1/chat/completions endpoint (OpenAI format)..."
CHAT_REQUEST='{
  "model": "'${MODEL_NAME}'",
  "messages": [{"role": "user", "content": "Hello, this is a test"}],
  "max_tokens": 50
}'

START_TIME=$(date +%s)
CHAT_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d "${CHAT_REQUEST}" 2>/dev/null || echo "000")
END_TIME=$(date +%s)
DURATION=$((END_TIME - START_TIME))

CHAT_CODE=$(echo "$CHAT_RESPONSE" | tail -n1)
CHAT_BODY=$(echo "$CHAT_RESPONSE" | head -n-1)

if [ "$CHAT_CODE" = "200" ]; then
  log_success "Chat completions endpoint responded successfully (HTTP ${CHAT_CODE}, ${DURATION}s)"
  if command -v jq >/dev/null 2>&1; then
    echo "$CHAT_BODY" | jq . 2>/dev/null || echo "$CHAT_BODY"
  else
    echo "$CHAT_BODY"
  fi
else
  log_warn "Chat completions endpoint returned HTTP ${CHAT_CODE}"
  log_info "Response: ${CHAT_BODY}"
  log_info "Note: This may be expected if no backend is configured for model '${MODEL_NAME}'"
fi

# Summary
echo ""
log_info "Test Summary:"
log_info "  - API Router Service: ✓ Running"
log_info "  - Health endpoint: ✓ (HTTP ${HEALTH_CODE})"
log_info "  - Readiness endpoint: ${READY_CODE}"
log_info "  - Inference endpoint: ${INFERENCE_CODE}"
log_info "  - Chat completions endpoint: ${CHAT_CODE}"
echo ""
log_info "Note: To test with a real vLLM deployment, you need:"
log_info "  1. GPU nodes in the cluster (currently none available)"
log_info "  2. Deploy a vLLM model instance using: ./scripts/vllm/deploy-with-retry.sh"
log_info "  3. Register the model in model_registry_entries"
log_info "  4. Configure API Router to route to the vLLM endpoint"

