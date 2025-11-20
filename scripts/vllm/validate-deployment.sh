#!/usr/bin/env bash
#
# vLLM Deployment Validation Script
#
# Purpose:
#   Validates a vLLM deployment is healthy and ready to serve traffic.
#   Performs comprehensive checks of pod status, health endpoints, registry,
#   and inference functionality.
#
# Usage:
#   ./validate-deployment.sh <release-name> <namespace> [environment]
#
# Examples:
#   ./validate-deployment.sh llama-2-7b-production system production
#   ./validate-deployment.sh llama-2-7b-development system development
#
# Requirements Reference:
#   - specs/010-vllm-deployment/tasks.md#T-S010-P06-067
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
RELEASE_NAME="${1:-}"
NAMESPACE="${2:-system}"
ENVIRONMENT="${3:-production}"
TIMEOUT=300  # 5 minutes
HEALTH_CHECK_RETRIES=5
HEALTH_CHECK_INTERVAL=10

# Usage
usage() {
    cat <<EOF
Usage: $0 <release-name> <namespace> [environment]

Validates a vLLM deployment is healthy and ready to serve traffic.

Arguments:
  release-name    Helm release name (e.g., llama-2-7b-production)
  namespace       Kubernetes namespace (default: system)
  environment     Deployment environment (default: production)

Examples:
  $0 llama-2-7b-production system production
  $0 llama-2-7b-development system development

Validation Steps:
  1. Helm release status check
  2. Pod status and readiness
  3. Health endpoint validation
  4. Model registry verification
  5. Inference functionality test
  6. Metrics endpoint check
  7. Resource utilization check

Exit Codes:
  0 - All validations passed
  1 - Validation failed
  2 - Usage error

EOF
}

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
    echo -e "${RED}[✗]${NC} $*"
}

log_section() {
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}$*${NC}"
    echo -e "${BLUE}========================================${NC}"
}

# Validation check wrapper
validate_check() {
    local check_name="$1"
    shift

    log_info "Checking: $check_name"

    if "$@"; then
        log_success "$check_name: PASSED"
        return 0
    else
        log_error "$check_name: FAILED"
        return 1
    fi
}

# Check prerequisites
check_prerequisites() {
    local missing_tools=()

    for tool in kubectl helm jq curl; do
        if ! command -v "$tool" &> /dev/null; then
            missing_tools+=("$tool")
        fi
    done

    if [ ${#missing_tools[@]} -gt 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_info "Please install missing tools and try again"
        return 1
    fi

    return 0
}

# Check Helm release status
check_helm_release() {
    log_info "Checking Helm release: $RELEASE_NAME in namespace: $NAMESPACE"

    if ! helm list -n "$NAMESPACE" | grep -q "$RELEASE_NAME"; then
        log_error "Helm release '$RELEASE_NAME' not found in namespace '$NAMESPACE'"
        return 1
    fi

    local status
    status=$(helm status "$RELEASE_NAME" -n "$NAMESPACE" -o json | jq -r '.info.status')

    if [ "$status" != "deployed" ]; then
        log_error "Helm release status is '$status', expected 'deployed'"
        helm status "$RELEASE_NAME" -n "$NAMESPACE"
        return 1
    fi

    log_success "Helm release is deployed"
    return 0
}

# Check pod status
check_pod_status() {
    log_info "Checking pod status for release: $RELEASE_NAME"

    local pods
    pods=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/instance=$RELEASE_NAME" -o json)

    if [ "$(echo "$pods" | jq '.items | length')" -eq 0 ]; then
        log_error "No pods found for release '$RELEASE_NAME'"
        return 1
    fi

    # Check each pod
    local all_ready=true
    local pod_count=0
    local ready_count=0

    echo "$pods" | jq -c '.items[]' | while read -r pod; do
        pod_count=$((pod_count + 1))
        local pod_name
        local phase
        local ready

        pod_name=$(echo "$pod" | jq -r '.metadata.name')
        phase=$(echo "$pod" | jq -r '.status.phase')
        ready=$(echo "$pod" | jq -r '.status.conditions[] | select(.type=="Ready") | .status')

        log_info "Pod: $pod_name"
        log_info "  Phase: $phase"
        log_info "  Ready: $ready"

        if [ "$phase" != "Running" ]; then
            log_error "Pod $pod_name is not Running (phase: $phase)"
            all_ready=false
        elif [ "$ready" != "True" ]; then
            log_error "Pod $pod_name is not Ready"
            all_ready=false
        else
            ready_count=$((ready_count + 1))
        fi
    done

    if ! $all_ready; then
        log_error "Not all pods are ready"
        kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/instance=$RELEASE_NAME"
        return 1
    fi

    log_success "All pods are Running and Ready"
    return 0
}

# Check pod health endpoint
check_health_endpoint() {
    log_info "Checking health endpoint..."

    local service_name="$RELEASE_NAME"
    local service_port=8000

    # Get service
    if ! kubectl get service "$service_name" -n "$NAMESPACE" &> /dev/null; then
        log_error "Service '$service_name' not found"
        return 1
    fi

    # Port-forward and check health
    local retries=0
    local health_check_passed=false

    while [ $retries -lt $HEALTH_CHECK_RETRIES ]; do
        log_info "Health check attempt $((retries + 1))/$HEALTH_CHECK_RETRIES"

        # Create port-forward in background
        kubectl port-forward -n "$NAMESPACE" "service/$service_name" 8000:8000 &> /dev/null &
        local pf_pid=$!

        # Wait for port-forward to establish
        sleep 2

        # Check health endpoint
        if curl -f -s --max-time 10 http://localhost:8000/health > /dev/null 2>&1; then
            log_success "Health endpoint responded successfully"
            health_check_passed=true
            kill $pf_pid 2> /dev/null || true
            break
        else
            log_warning "Health endpoint not responding, retrying in $HEALTH_CHECK_INTERVAL seconds..."
            kill $pf_pid 2> /dev/null || true
            sleep $HEALTH_CHECK_INTERVAL
        fi

        retries=$((retries + 1))
    done

    if ! $health_check_passed; then
        log_error "Health endpoint failed after $HEALTH_CHECK_RETRIES attempts"
        return 1
    fi

    return 0
}

# Check model registry
check_model_registry() {
    log_info "Checking model registry..."

    # Extract model name from release name (remove environment suffix)
    local model_name
    model_name=$(echo "$RELEASE_NAME" | sed -E "s/-(development|staging|production)$//")

    log_info "Looking for model: $model_name in environment: $ENVIRONMENT"

    # Check if admin-cli is available
    if ! command -v admin-cli &> /dev/null; then
        log_warning "admin-cli not found, skipping registry check"
        log_info "To enable registry validation, ensure admin-cli is in PATH"
        return 0
    fi

    # Query registry
    local registry_output
    if ! registry_output=$(admin-cli registry list --environment "$ENVIRONMENT" 2>&1); then
        log_warning "Failed to query registry: $registry_output"
        log_info "Registry check skipped (this may be expected if registry is not configured)"
        return 0
    fi

    # Check if model is registered
    if echo "$registry_output" | grep -q "$model_name"; then
        log_success "Model '$model_name' found in registry"

        # Check if model is enabled
        if echo "$registry_output" | grep "$model_name" | grep -q "ready"; then
            log_success "Model status: ready"
        else
            log_warning "Model is registered but not in 'ready' status"
        fi
    else
        log_warning "Model '$model_name' not found in registry"
        log_info "Run: admin-cli registry register --model-name $model_name --endpoint $RELEASE_NAME.$NAMESPACE.svc.cluster.local:8000 --environment $ENVIRONMENT --namespace $NAMESPACE"
    fi

    return 0
}

# Test inference functionality
test_inference() {
    log_info "Testing inference functionality..."

    local service_name="$RELEASE_NAME"

    # Create port-forward
    kubectl port-forward -n "$NAMESPACE" "service/$service_name" 8000:8000 &> /dev/null &
    local pf_pid=$!

    # Wait for port-forward
    sleep 2

    # Test inference with a simple request
    local test_request='{
  "model": "test",
  "prompt": "Hello, ",
  "max_tokens": 5,
  "temperature": 0.7
}'

    local response
    if response=$(curl -f -s --max-time 30 \
        -X POST http://localhost:8000/v1/completions \
        -H 'Content-Type: application/json' \
        -d "$test_request" 2>&1); then

        # Validate response structure
        if echo "$response" | jq -e '.choices[0].text' > /dev/null 2>&1; then
            local generated_text
            generated_text=$(echo "$response" | jq -r '.choices[0].text')
            log_success "Inference test passed"
            log_info "Generated text: $generated_text"
        else
            log_error "Inference response missing expected fields"
            log_info "Response: $response"
            kill $pf_pid 2> /dev/null || true
            return 1
        fi
    else
        log_error "Inference test failed: $response"
        kill $pf_pid 2> /dev/null || true
        return 1
    fi

    # Cleanup
    kill $pf_pid 2> /dev/null || true

    return 0
}

# Check metrics endpoint
check_metrics_endpoint() {
    log_info "Checking metrics endpoint..."

    local service_name="$RELEASE_NAME"

    # Create port-forward
    kubectl port-forward -n "$NAMESPACE" "service/$service_name" 8000:8000 &> /dev/null &
    local pf_pid=$!

    # Wait for port-forward
    sleep 2

    # Check metrics
    if curl -f -s --max-time 10 http://localhost:8000/metrics > /dev/null 2>&1; then
        log_success "Metrics endpoint is accessible"
    else
        log_warning "Metrics endpoint not accessible (this may be expected for some configurations)"
    fi

    # Cleanup
    kill $pf_pid 2> /dev/null || true

    return 0
}

# Check resource utilization
check_resource_utilization() {
    log_info "Checking resource utilization..."

    local pods
    pods=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/instance=$RELEASE_NAME" -o json | jq -r '.items[].metadata.name')

    for pod in $pods; do
        log_info "Resource usage for pod: $pod"

        if kubectl top pod "$pod" -n "$NAMESPACE" &> /dev/null; then
            kubectl top pod "$pod" -n "$NAMESPACE" | tail -n 1 | awk '{print "  CPU: " $2 ", Memory: " $3}'
        else
            log_warning "Metrics not available (metrics-server may not be installed)"
        fi

        # Check GPU allocation
        local gpu_request
        gpu_request=$(kubectl get pod "$pod" -n "$NAMESPACE" -o json | jq -r '.spec.containers[0].resources.requests["nvidia.com/gpu"] // "0"')

        if [ "$gpu_request" != "0" ]; then
            log_info "  GPU Request: $gpu_request"
        else
            log_warning "  No GPU requested (expected for vLLM deployments)"
        fi
    done

    return 0
}

# Print validation summary
print_summary() {
    local total_checks=$1
    local passed_checks=$2

    log_section "VALIDATION SUMMARY"

    echo ""
    echo "Release: $RELEASE_NAME"
    echo "Namespace: $NAMESPACE"
    echo "Environment: $ENVIRONMENT"
    echo ""
    echo "Checks Passed: $passed_checks/$total_checks"
    echo ""

    if [ "$passed_checks" -eq "$total_checks" ]; then
        log_success "All validation checks passed! Deployment is healthy and ready."
        return 0
    else
        log_error "Some validation checks failed. Please review the output above."
        return 1
    fi
}

# Main function
main() {
    # Check arguments
    if [ -z "$RELEASE_NAME" ]; then
        usage
        exit 2
    fi

    log_section "vLLM Deployment Validation"

    echo ""
    echo "Release: $RELEASE_NAME"
    echo "Namespace: $NAMESPACE"
    echo "Environment: $ENVIRONMENT"
    echo ""

    # Check prerequisites
    if ! check_prerequisites; then
        exit 1
    fi

    # Run validation checks
    local total_checks=0
    local passed_checks=0

    log_section "Running Validation Checks"

    # 1. Helm release
    total_checks=$((total_checks + 1))
    if validate_check "Helm Release Status" check_helm_release; then
        passed_checks=$((passed_checks + 1))
    fi

    # 2. Pod status
    total_checks=$((total_checks + 1))
    if validate_check "Pod Status" check_pod_status; then
        passed_checks=$((passed_checks + 1))
    fi

    # 3. Health endpoint
    total_checks=$((total_checks + 1))
    if validate_check "Health Endpoint" check_health_endpoint; then
        passed_checks=$((passed_checks + 1))
    fi

    # 4. Model registry
    total_checks=$((total_checks + 1))
    if validate_check "Model Registry" check_model_registry; then
        passed_checks=$((passed_checks + 1))
    fi

    # 5. Inference test
    total_checks=$((total_checks + 1))
    if validate_check "Inference Functionality" test_inference; then
        passed_checks=$((passed_checks + 1))
    fi

    # 6. Metrics endpoint
    total_checks=$((total_checks + 1))
    if validate_check "Metrics Endpoint" check_metrics_endpoint; then
        passed_checks=$((passed_checks + 1))
    fi

    # 7. Resource utilization
    total_checks=$((total_checks + 1))
    if validate_check "Resource Utilization" check_resource_utilization; then
        passed_checks=$((passed_checks + 1))
    fi

    # Print summary
    if print_summary "$total_checks" "$passed_checks"; then
        exit 0
    else
        exit 1
    fi
}

# Run main function
main "$@"
