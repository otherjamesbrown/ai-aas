#!/bin/bash
#
# Unified test script for API Router inference endpoints
#
# This script supports both mock tests (fast) and E2E tests (real backends)
# to provide a layered testing approach.
#
# Usage:
#   ./test-api-inference.sh [mode] [options]
#
# Modes:
#   fast        - Run mock tests only (CI/CD, development)
#   e2e         - Run E2E tests with real vLLM backend
#   all         - Run both mock and E2E tests
#   cluster     - Run E2E tests with port-forward to cluster
#
# Examples:
#   ./test-api-inference.sh fast
#   ./test-api-inference.sh e2e http://localhost:8000
#   ./test-api-inference.sh cluster
#

set -euo pipefail

# Colors
readonly GREEN='\033[0;32m'
readonly RED='\033[0;31m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly BOLD='\033[1m'
readonly NC='\033[0m'

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
API_ROUTER_DIR="${SCRIPT_DIR}/services/api-router-service"

log_info() {
    printf "${BLUE}[INFO]${NC} %s\n" "$*"
}

log_success() {
    printf "${GREEN}[SUCCESS]${NC} %s\n" "$*"
}

log_error() {
    printf "${RED}[ERROR]${NC} %s\n" "$*"
}

log_warn() {
    printf "${YELLOW}[WARN]${NC} %s\n" "$*"
}

log_section() {
    printf "\n${BOLD}═══════════════════════════════════════${NC}\n"
    printf "${BOLD}%s${NC}\n" "$*"
    printf "${BOLD}═══════════════════════════════════════${NC}\n\n"
}

show_usage() {
    cat <<EOF
Usage: $0 [mode] [options]

Modes:
  fast        Run mock tests only (default)
  e2e         Run E2E tests with real vLLM backend
  all         Run both mock and E2E tests
  cluster     Run E2E tests with port-forward to cluster

Options for 'e2e' mode:
  $0 e2e <backend-url> [model-name]

  backend-url   vLLM endpoint (e.g., http://localhost:8000)
  model-name    Model name (e.g., meta-llama/Llama-2-7b-chat-hf)

Options for 'cluster' mode:
  $0 cluster [namespace] [service-name] [model-name]

  namespace     Kubernetes namespace (default: system)
  service-name  vLLM service name (default: vllm-service)
  model-name    Model name (default: gpt-4o)

Examples:
  # Fast tests (CI/CD)
  $0 fast

  # E2E test with local vLLM
  $0 e2e http://localhost:8000 meta-llama/Llama-2-7b-chat-hf

  # E2E test with cluster vLLM (auto port-forward)
  $0 cluster

  # E2E test with specific cluster service
  $0 cluster system my-vllm-service my-model

  # Run both mock and E2E
  $0 all
EOF
}

run_mock_tests() {
    log_section "Running Mock Tests (Fast)"

    log_info "Purpose: Test API router logic, validation, response formatting"
    log_info "Speed: ~20ms per test"
    log_info "Confidence: Medium (tests logic, not real integration)"
    echo ""

    cd "${API_ROUTER_DIR}"

    if go test -short -v ./test/integration -run "TestOpenAI.*[^E2E]$" 2>&1 | tee /tmp/test-output.txt; then
        log_success "✓ Mock tests passed!"
        echo ""
        log_info "What was tested:"
        log_info "  ✓ Request validation"
        log_info "  ✓ Response formatting"
        log_info "  ✓ Routing logic"
        log_info "  ✓ Error handling"
        echo ""
        log_warn "Note: These tests use mocks. Run 'e2e' mode for real backend validation."
        return 0
    else
        log_error "✗ Mock tests failed!"
        log_error "This indicates a logic error in the API router code."
        return 1
    fi
}

run_e2e_tests() {
    local backend_url="$1"
    local model_name="${2:-gpt-4o}"

    log_section "Running E2E Tests (Real Backend)"

    log_info "Backend: ${backend_url}"
    log_info "Model: ${model_name}"
    log_info "Purpose: Test actual inference, catch real integration issues"
    log_info "Speed: ~2-10s per test"
    log_info "Confidence: High (tests real integration)"
    echo ""

    # Check if backend is reachable
    log_info "Checking if vLLM backend is healthy..."
    if ! curl -sf "${backend_url}/health" >/dev/null; then
        log_error "vLLM backend is not reachable at ${backend_url}"
        log_error "Make sure vLLM is running and accessible"
        return 1
    fi
    log_success "✓ vLLM backend is healthy"
    echo ""

    # Run E2E tests
    cd "${API_ROUTER_DIR}"
    export VLLM_BACKEND_URL="${backend_url}"
    export VLLM_MODEL_NAME="${model_name}"

    if go test -v ./test/integration -run E2E 2>&1 | tee /tmp/test-e2e-output.txt; then
        log_success "✓ E2E tests passed!"
        echo ""
        log_info "What was tested:"
        log_info "  ✓ Real model inference"
        log_info "  ✓ Token counting accuracy"
        log_info "  ✓ Actual latency and performance"
        log_info "  ✓ GPU integration"
        log_info "  ✓ The 'capital of France' question returned 'Paris'"
        echo ""
        log_success "🎉 System is working correctly with real backend!"
        return 0
    else
        log_error "✗ E2E tests failed!"
        echo ""
        log_error "This indicates a REAL integration issue:"
        log_error "  - Model behavior issue"
        log_error "  - Token counting error"
        log_error "  - Timeout problem"
        log_error "  - GPU error"
        echo ""
        log_error "Check the test output above for details."
        return 1
    fi
}

run_cluster_tests() {
    local namespace="${1:-system}"
    local service_name="${2:-vllm-service}"
    local model_name="${3:-gpt-4o}"
    local port=8000

    log_section "Running E2E Tests with Cluster (Port-Forward)"

    # Check kubectl
    if ! command -v kubectl >/dev/null 2>&1; then
        log_error "kubectl is not installed"
        return 1
    fi

    # Check if service exists
    log_info "Checking if service exists in cluster..."
    if ! kubectl get svc -n "${namespace}" "${service_name}" >/dev/null 2>&1; then
        log_error "Service '${service_name}' not found in namespace '${namespace}'"
        log_info "Available services:"
        kubectl get svc -n "${namespace}"
        return 1
    fi
    log_success "✓ Service found: ${namespace}/${service_name}"
    echo ""

    # Setup port-forward
    log_info "Setting up port-forward to ${namespace}/${service_name}:${port}..."
    kubectl port-forward -n "${namespace}" "svc/${service_name}" "${port}:${port}" >/dev/null 2>&1 &
    PORT_FORWARD_PID=$!

    # Cleanup on exit
    trap "log_info 'Stopping port-forward...'; kill ${PORT_FORWARD_PID} 2>/dev/null || true" EXIT

    # Wait for port-forward
    sleep 3

    if ! curl -sf "http://localhost:${port}/health" >/dev/null; then
        log_error "Port-forward failed or service not healthy"
        kill ${PORT_FORWARD_PID} 2>/dev/null || true
        return 1
    fi
    log_success "✓ Port-forward established"
    echo ""

    # Run E2E tests
    run_e2e_tests "http://localhost:${port}" "${model_name}"
    local result=$?

    # Cleanup
    kill ${PORT_FORWARD_PID} 2>/dev/null || true
    trap - EXIT

    return ${result}
}

# Main
main() {
    local mode="${1:-fast}"

    case "${mode}" in
        fast)
            run_mock_tests
            ;;
        e2e)
            if [ -z "${2:-}" ]; then
                log_error "Backend URL required for E2E mode"
                echo ""
                show_usage
                exit 1
            fi
            run_e2e_tests "$2" "${3:-gpt-4o}"
            ;;
        cluster)
            run_cluster_tests "${2:-system}" "${3:-vllm-service}" "${4:-gpt-4o}"
            ;;
        all)
            log_section "Running All Tests"
            run_mock_tests
            echo ""
            if [ -n "${2:-}" ]; then
                run_e2e_tests "$2" "${3:-gpt-4o}"
            else
                log_warn "Skipping E2E tests (no backend URL provided)"
                log_info "To run E2E tests: $0 all <backend-url>"
            fi
            ;;
        -h|--help|help)
            show_usage
            exit 0
            ;;
        *)
            log_error "Unknown mode: ${mode}"
            echo ""
            show_usage
            exit 1
            ;;
    esac
}

main "$@"
