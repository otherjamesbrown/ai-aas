#!/usr/bin/env bash
#
# Register vLLM Model Deployment
#
# This script automates the model registration process after Helm deployment.
# It waits for the deployment to be ready, verifies health, and registers the
# model in the model registry for API routing.
#
# Usage:
#   ./register-model.sh MODEL_NAME ENVIRONMENT NAMESPACE [OPTIONS]
#
# Arguments:
#   MODEL_NAME    - Model identifier (e.g., gpt-oss-20b, llama-2-7b)
#   ENVIRONMENT   - Deployment environment (development, staging, production)
#   NAMESPACE     - Kubernetes namespace (default: system)
#
# Options:
#   --skip-wait        Skip waiting for deployment readiness
#   --skip-health      Skip health check verification
#   --timeout DURATION Deployment wait timeout (default: 20m)
#   --dry-run          Simulate registration without making changes
#   --help             Show this help message
#
# Environment Variables:
#   DATABASE_URL    - PostgreSQL connection string (required)
#   KUBECONFIG      - Path to kubeconfig file (required)
#   ADMIN_CLI       - Path to admin-cli binary (default: admin-cli)
#
# Examples:
#   # Register model after deployment
#   ./register-model.sh gpt-oss-20b development system
#
#   # Register with custom timeout
#   ./register-model.sh llama-2-7b production production --timeout 30m
#
#   # Dry run to test without changes
#   ./register-model.sh gpt-oss-20b development system --dry-run
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_NAMESPACE="system"
DEFAULT_TIMEOUT="20m"
ADMIN_CLI="${ADMIN_CLI:-admin-cli}"

# Flags
SKIP_WAIT=false
SKIP_HEALTH=false
DRY_RUN=false
TIMEOUT="$DEFAULT_TIMEOUT"

#------------------------------------------------------------------------------
# Functions
#------------------------------------------------------------------------------

log_info() {
    echo -e "${BLUE}ℹ${NC} $*"
}

log_success() {
    echo -e "${GREEN}✓${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}⚠${NC} $*"
}

log_error() {
    echo -e "${RED}✗${NC} $*" >&2
}

show_usage() {
    sed -n '2,/^$/p' "$0" | sed 's/^# //; s/^#//'
    exit 0
}

check_prerequisites() {
    local missing_prereqs=()

    # Check required commands
    for cmd in kubectl "${ADMIN_CLI}"; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_prereqs+=("$cmd")
        fi
    done

    # Check required environment variables
    if [[ -z "${DATABASE_URL:-}" ]]; then
        log_error "DATABASE_URL environment variable not set"
        missing_prereqs+=("DATABASE_URL")
    fi

    if [[ -z "${KUBECONFIG:-}" ]]; then
        log_error "KUBECONFIG environment variable not set"
        missing_prereqs+=("KUBECONFIG")
    fi

    if [[ ${#missing_prereqs[@]} -gt 0 ]]; then
        log_error "Missing prerequisites: ${missing_prereqs[*]}"
        echo ""
        echo "Please ensure the following are available:"
        echo "  - kubectl CLI tool"
        echo "  - admin-cli binary (from services/admin-cli)"
        echo "  - DATABASE_URL environment variable"
        echo "  - KUBECONFIG environment variable"
        exit 1
    fi
}

validate_environment() {
    local env="$1"
    case "$env" in
        development|staging|production)
            return 0
            ;;
        *)
            log_error "Invalid environment: $env"
            log_error "Environment must be one of: development, staging, production"
            exit 1
            ;;
    esac
}

wait_for_deployment_ready() {
    local deployment_name="$1"
    local namespace="$2"
    local timeout="$3"

    log_info "Waiting for deployment to be ready (timeout: $timeout)..."

    if kubectl wait --for=condition=Available deployment/"$deployment_name" \
        -n "$namespace" \
        --timeout="$timeout" &> /dev/null; then
        log_success "Deployment is Available"
        return 0
    else
        log_error "Deployment did not become Available within $timeout"
        return 1
    fi
}

wait_for_pod_ready() {
    local label_selector="$1"
    local namespace="$2"
    local timeout="$3"

    log_info "Waiting for pod to be Ready..."

    if kubectl wait --for=condition=Ready pod \
        -l "$label_selector" \
        -n "$namespace" \
        --timeout="$timeout" &> /dev/null; then
        log_success "Pod is Ready"
        return 0
    else
        log_error "Pod did not become Ready within $timeout"
        return 1
    fi
}

verify_health_endpoints() {
    local endpoint="$1"
    local namespace="$2"

    log_info "Verifying health endpoints..."

    # Create a temporary pod to test the endpoints
    local test_pod_name="test-health-$$"
    local health_url="http://${endpoint}/health"
    local models_url="http://${endpoint}/v1/models"

    # Test /health endpoint
    log_info "Testing /health endpoint: $health_url"
    if kubectl run "$test_pod_name" --rm -i --restart=Never \
        -n "$namespace" \
        --image=curlimages/curl:latest \
        --command -- curl -f -s "$health_url" &> /dev/null; then
        log_success "/health endpoint is accessible"
    else
        log_error "/health endpoint check failed"
        return 1
    fi

    # Test /v1/models endpoint
    sleep 1
    log_info "Testing /v1/models endpoint: $models_url"
    if kubectl run "$test_pod_name" --rm -i --restart=Never \
        -n "$namespace" \
        --image=curlimages/curl:latest \
        --command -- curl -f -s "$models_url" &> /dev/null; then
        log_success "/v1/models endpoint is accessible"
    else
        log_warning "/v1/models endpoint check failed (may be normal during startup)"
    fi

    return 0
}

register_model() {
    local model_name="$1"
    local endpoint="$2"
    local environment="$3"
    local namespace="$4"

    log_info "Registering model in registry..."
    log_info "  Model Name: $model_name"
    log_info "  Endpoint: $endpoint"
    log_info "  Environment: $environment"
    log_info "  Namespace: $namespace"

    local cmd_args=(
        "$ADMIN_CLI" registry register
        --model-name "$model_name"
        --endpoint "$endpoint"
        --environment "$environment"
        --namespace "$namespace"
    )

    if [[ "$DRY_RUN" == "true" ]]; then
        cmd_args+=(--dry-run)
        log_warning "DRY RUN MODE - No changes will be made"
    fi

    if "${cmd_args[@]}"; then
        log_success "Model registered successfully"
        return 0
    else
        log_error "Model registration failed"
        return 1
    fi
}

verify_registration() {
    local model_name="$1"
    local environment="$2"

    log_info "Verifying registration..."

    if "$ADMIN_CLI" registry list --environment "$environment" --format json | \
        jq -e ".[] | select(.model_name == \"$model_name\" and .deployment_status == \"ready\")" &> /dev/null; then
        log_success "Model is registered and ready"

        # Show registration details
        echo ""
        echo "Registration Details:"
        "$ADMIN_CLI" registry list --environment "$environment" | grep "$model_name" || true

        return 0
    else
        log_error "Model not found in registry or not ready"
        return 1
    fi
}

#------------------------------------------------------------------------------
# Main Script
#------------------------------------------------------------------------------

main() {
    # Parse arguments
    if [[ $# -lt 3 ]] || [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
        show_usage
    fi

    local model_name="$1"
    local environment="$2"
    local namespace="${3:-$DEFAULT_NAMESPACE}"
    shift 3

    # Parse options
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-wait)
                SKIP_WAIT=true
                shift
                ;;
            --skip-health)
                SKIP_HEALTH=true
                shift
                ;;
            --timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            --dry-run)
                DRY_RUN=true
                shift
                ;;
            --help|-h)
                show_usage
                ;;
            *)
                log_error "Unknown option: $1"
                show_usage
                ;;
        esac
    done

    # Print banner
    echo ""
    echo "═══════════════════════════════════════════════════════"
    echo " vLLM Model Registration"
    echo "═══════════════════════════════════════════════════════"
    echo " Model: $model_name"
    echo " Environment: $environment"
    echo " Namespace: $namespace"
    echo " Timeout: $TIMEOUT"
    if [[ "$DRY_RUN" == "true" ]]; then
        echo " Mode: DRY RUN"
    fi
    echo "═══════════════════════════════════════════════════════"
    echo ""

    # Check prerequisites
    log_info "Checking prerequisites..."
    check_prerequisites
    log_success "All prerequisites met"

    # Validate environment
    validate_environment "$environment"

    # Determine deployment and endpoint names
    local deployment_name="${model_name}-vllm-deployment"
    local service_endpoint="${deployment_name}.${namespace}.svc.cluster.local:8000"
    local label_selector="app.kubernetes.io/instance=${model_name}"

    log_info "Deployment: $deployment_name"
    log_info "Service Endpoint: $service_endpoint"
    log_info "Label Selector: $label_selector"

    # Check if deployment exists
    log_info "Checking if deployment exists..."
    if ! kubectl get deployment "$deployment_name" -n "$namespace" &> /dev/null; then
        log_error "Deployment '$deployment_name' not found in namespace '$namespace'"
        log_error "Please deploy the model first using Helm"
        exit 1
    fi
    log_success "Deployment exists"

    # Wait for deployment readiness (unless skipped)
    if [[ "$SKIP_WAIT" == "false" ]]; then
        if ! wait_for_deployment_ready "$deployment_name" "$namespace" "$TIMEOUT"; then
            log_error "Deployment did not become ready"
            log_info "Check deployment status: kubectl describe deployment $deployment_name -n $namespace"
            exit 1
        fi

        if ! wait_for_pod_ready "$label_selector" "$namespace" "$TIMEOUT"; then
            log_error "Pod did not become ready"
            log_info "Check pod status: kubectl get pods -l $label_selector -n $namespace"
            log_info "Check pod logs: kubectl logs -l $label_selector -n $namespace"
            exit 1
        fi
    else
        log_warning "Skipping deployment readiness wait (--skip-wait specified)"
    fi

    # Verify health endpoints (unless skipped)
    if [[ "$SKIP_HEALTH" == "false" ]]; then
        if ! verify_health_endpoints "$service_endpoint" "$namespace"; then
            log_error "Health endpoint verification failed"
            log_info "Service may not be fully ready yet"
            exit 1
        fi
    else
        log_warning "Skipping health check (--skip-health specified)"
    fi

    # Register model
    if ! register_model "$model_name" "$service_endpoint" "$environment" "$namespace"; then
        log_error "Model registration failed"
        exit 1
    fi

    # Verify registration (unless dry-run)
    if [[ "$DRY_RUN" == "false" ]]; then
        sleep 2  # Give database a moment to commit
        if ! verify_registration "$model_name" "$environment"; then
            log_warning "Registration verification failed"
            log_info "The model may have been registered but verification timed out"
            log_info "Check manually with: admin-cli registry list --environment $environment"
        fi
    fi

    # Success
    echo ""
    echo "═══════════════════════════════════════════════════════"
    log_success "Model registration complete!"
    echo "═══════════════════════════════════════════════════════"
    echo ""
    echo "Next steps:"
    echo "  1. ✓ Model deployed via Helm"
    echo "  2. ✓ Deployment is healthy and ready"
    echo "  3. ✓ Model registered in registry"
    echo "  4. → Verify API routing (if API Router is integrated)"
    echo ""
    echo "To verify routing:"
    echo "  curl -X POST https://api.${environment}.ai-aas.local/v1/chat/completions \\"
    echo "    -H \"X-API-Key: your-api-key\" \\"
    echo "    -H \"Content-Type: application/json\" \\"
    echo "    -d '{\"model\": \"$model_name\", \"messages\": [{\"role\": \"user\", \"content\": \"Hello\"}]}'"
    echo ""
}

# Run main function
main "$@"
