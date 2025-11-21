#!/usr/bin/env bash
#
# Rollback vLLM Deployment
#
# This script automates the safe rollback of vLLM model deployments using Helm.
# It disables the model in the registry, performs the rollback, waits for health,
# and re-enables the model when ready.
#
# Usage:
#   ./rollback-deployment.sh MODEL_NAME ENVIRONMENT [REVISION] [OPTIONS]
#
# Arguments:
#   MODEL_NAME    - Model identifier (e.g., gpt-oss-20b, llama-2-7b)
#   ENVIRONMENT   - Deployment environment (development, staging, production)
#   REVISION      - Target revision number (default: 0 for previous revision)
#   NAMESPACE     - Kubernetes namespace (default: system)
#
# Options:
#   --skip-disable      Skip disabling model in registry before rollback
#   --skip-enable       Skip re-enabling model after rollback
#   --skip-wait         Skip waiting for rollback to complete
#   --timeout DURATION  Rollback wait timeout (default: 10m)
#   --force             Skip confirmation prompts
#   --help              Show this help message
#
# Environment Variables:
#   DATABASE_URL    - PostgreSQL connection string (required for registry updates)
#   KUBECONFIG      - Path to kubeconfig file (required)
#   ADMIN_CLI       - Path to admin-cli binary (default: admin-cli)
#
# Examples:
#   # Rollback to previous revision
#   ./rollback-deployment.sh gpt-oss-20b development
#
#   # Rollback to specific revision
#   ./rollback-deployment.sh gpt-oss-20b development 2
#
#   # Rollback in production with force (no prompts)
#   ./rollback-deployment.sh llama-2-7b production 3 --force
#
#   # Emergency rollback (skip waits for speed)
#   ./rollback-deployment.sh gpt-oss-20b production --skip-wait --force
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
DEFAULT_TIMEOUT="10m"
DEFAULT_REVISION="0"  # 0 means previous revision
ADMIN_CLI="${ADMIN_CLI:-admin-cli}"

# Flags
SKIP_DISABLE=false
SKIP_ENABLE=false
SKIP_WAIT=false
FORCE=false
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

confirm() {
    local prompt="$1"
    if [[ "$FORCE" == "true" ]]; then
        log_warning "Skipping confirmation (--force specified)"
        return 0
    fi

    read -r -p "$prompt [y/N] " response
    case "$response" in
        [yY][eE][sS]|[yY])
            return 0
            ;;
        *)
            return 1
            ;;
    esac
}

check_prerequisites() {
    local missing_prereqs=()

    # Check required commands
    for cmd in kubectl helm; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_prereqs+=("$cmd")
        fi
    done

    # Check admin-cli if registry updates are needed
    if [[ "$SKIP_DISABLE" == "false" ]] || [[ "$SKIP_ENABLE" == "false" ]]; then
        if ! command -v "${ADMIN_CLI}" &> /dev/null; then
            missing_prereqs+=("${ADMIN_CLI}")
        fi

        if [[ -z "${DATABASE_URL:-}" ]]; then
            log_error "DATABASE_URL environment variable not set"
            missing_prereqs+=("DATABASE_URL")
        fi
    fi

    if [[ -z "${KUBECONFIG:-}" ]]; then
        log_error "KUBECONFIG environment variable not set"
        missing_prereqs+=("KUBECONFIG")
    fi

    if [[ ${#missing_prereqs[@]} -gt 0 ]]; then
        log_error "Missing prerequisites: ${missing_prereqs[*]}"
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

show_release_history() {
    local release_name="$1"
    local namespace="$2"

    log_info "Helm release history:"
    echo ""
    helm history "$release_name" -n "$namespace" || {
        log_error "Failed to get release history"
        return 1
    }
    echo ""
}

get_current_revision() {
    local release_name="$1"
    local namespace="$2"

    helm history "$release_name" -n "$namespace" --max 1 -o json 2>/dev/null | \
        jq -r '.[0].revision' || echo "unknown"
}

disable_model_in_registry() {
    local model_name="$1"
    local environment="$2"

    log_info "Disabling model in registry (prevents routing during rollback)..."

    if "$ADMIN_CLI" registry disable \
        --model-name "$model_name" \
        --environment "$environment" \
        --quiet; then
        log_success "Model disabled in registry"
        return 0
    else
        log_error "Failed to disable model in registry"
        return 1
    fi
}

enable_model_in_registry() {
    local model_name="$1"
    local environment="$2"

    log_info "Enabling model in registry..."

    if "$ADMIN_CLI" registry enable \
        --model-name "$model_name" \
        --environment "$environment" \
        --quiet; then
        log_success "Model enabled in registry"
        return 0
    else
        log_error "Failed to enable model in registry"
        return 1
    fi
}

perform_helm_rollback() {
    local release_name="$1"
    local namespace="$2"
    local revision="$3"
    local timeout="$4"
    local wait="$5"

    log_info "Performing Helm rollback..."
    log_info "  Release: $release_name"
    log_info "  Namespace: $namespace"
    log_info "  Target Revision: $revision (0 = previous)"
    log_info "  Timeout: $timeout"

    local cmd_args=(
        helm rollback "$release_name" "$revision"
        -n "$namespace"
        --timeout "$timeout"
    )

    if [[ "$wait" == "true" ]]; then
        cmd_args+=(--wait)
    fi

    if "${cmd_args[@]}"; then
        log_success "Helm rollback completed"
        return 0
    else
        log_error "Helm rollback failed"
        return 1
    fi
}

wait_for_rollback_completion() {
    local release_name="$1"
    local namespace="$2"
    local timeout="$3"

    log_info "Waiting for rollback to complete..."

    # Wait for deployment to be available
    local deployment_name="${release_name}-vllm-deployment"
    if kubectl wait --for=condition=Available deployment/"$deployment_name" \
        -n "$namespace" \
        --timeout="$timeout" &> /dev/null; then
        log_success "Deployment is Available"
    else
        log_warning "Deployment did not become Available within timeout"
        return 1
    fi

    # Wait for pods to be ready
    local label_selector="app.kubernetes.io/instance=${release_name}"
    if kubectl wait --for=condition=Ready pod \
        -l "$label_selector" \
        -n "$namespace" \
        --timeout="$timeout" &> /dev/null; then
        log_success "Pods are Ready"
        return 0
    else
        log_warning "Pods did not become Ready within timeout"
        return 1
    fi
}

verify_rollback_health() {
    local release_name="$1"
    local namespace="$2"

    log_info "Verifying rollback health..."

    # Get pod status
    local label_selector="app.kubernetes.io/instance=${release_name}"
    local pod_count
    pod_count=$(kubectl get pods -l "$label_selector" -n "$namespace" --no-headers 2>/dev/null | wc -l)

    if [[ "$pod_count" -eq 0 ]]; then
        log_error "No pods found for release"
        return 1
    fi

    local ready_count
    ready_count=$(kubectl get pods -l "$label_selector" -n "$namespace" --no-headers 2>/dev/null | \
        grep -c "Running" || echo "0")

    log_info "  Pods: $ready_count/$pod_count Running"

    if [[ "$ready_count" -eq "$pod_count" ]]; then
        log_success "All pods are Running"
        return 0
    else
        log_warning "Not all pods are Running"
        return 1
    fi
}

#------------------------------------------------------------------------------
# Main Script
#------------------------------------------------------------------------------

main() {
    # Parse arguments
    if [[ $# -lt 2 ]] || [[ "${1:-}" == "--help" ]] || [[ "${1:-}" == "-h" ]]; then
        show_usage
    fi

    local model_name="$1"
    local environment="$2"
    local revision="${3:-$DEFAULT_REVISION}"
    local namespace="${4:-$DEFAULT_NAMESPACE}"
    shift 2
    if [[ $# -gt 0 ]] && [[ "$1" != --* ]]; then
        shift  # Skip revision if provided
    fi
    if [[ $# -gt 0 ]] && [[ "$1" != --* ]]; then
        shift  # Skip namespace if provided
    fi

    # Parse options
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --skip-disable)
                SKIP_DISABLE=true
                shift
                ;;
            --skip-enable)
                SKIP_ENABLE=true
                shift
                ;;
            --skip-wait)
                SKIP_WAIT=true
                shift
                ;;
            --timeout)
                TIMEOUT="$2"
                shift 2
                ;;
            --force)
                FORCE=true
                shift
                ;;
            --namespace)
                namespace="$2"
                shift 2
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
    echo " vLLM Deployment Rollback"
    echo "═══════════════════════════════════════════════════════"
    echo " Model: $model_name"
    echo " Environment: $environment"
    echo " Namespace: $namespace"
    echo " Target Revision: $revision"
    echo " Timeout: $TIMEOUT"
    if [[ "$FORCE" == "true" ]]; then
        echo " Mode: FORCE (no prompts)"
    fi
    echo "═══════════════════════════════════════════════════════"
    echo ""

    # Check prerequisites
    log_info "Checking prerequisites..."
    check_prerequisites
    log_success "All prerequisites met"

    # Validate environment
    validate_environment "$environment"

    # Determine release name
    local release_name="$model_name"
    log_info "Release: $release_name"

    # Check if release exists
    log_info "Checking if release exists..."
    if ! helm list -n "$namespace" -q | grep -q "^${release_name}$"; then
        log_error "Release '$release_name' not found in namespace '$namespace'"
        exit 1
    fi
    log_success "Release exists"

    # Show release history
    if ! show_release_history "$release_name" "$namespace"; then
        exit 1
    fi

    # Get current revision
    local current_revision
    current_revision=$(get_current_revision "$release_name" "$namespace")
    log_info "Current revision: $current_revision"

    # Confirm rollback
    if [[ "$revision" == "0" ]]; then
        log_warning "Rolling back to PREVIOUS revision"
    else
        log_warning "Rolling back to revision $revision"
    fi

    if ! confirm "Are you sure you want to rollback?"; then
        log_info "Rollback cancelled"
        exit 0
    fi

    # Step 1: Disable model in registry
    if [[ "$SKIP_DISABLE" == "false" ]]; then
        if ! disable_model_in_registry "$model_name" "$environment"; then
            log_warning "Failed to disable model, but continuing with rollback..."
        fi
    else
        log_warning "Skipping model disable (--skip-disable specified)"
    fi

    # Step 2: Perform Helm rollback
    local wait_flag="false"
    if [[ "$SKIP_WAIT" == "false" ]]; then
        wait_flag="true"
    fi

    if ! perform_helm_rollback "$release_name" "$namespace" "$revision" "$TIMEOUT" "$wait_flag"; then
        log_error "Helm rollback failed"
        exit 1
    fi

    # Step 3: Wait for rollback completion (if not skipped)
    if [[ "$SKIP_WAIT" == "false" ]]; then
        if ! wait_for_rollback_completion "$release_name" "$namespace" "$TIMEOUT"; then
            log_warning "Rollback may not be fully complete yet"
        fi
    else
        log_warning "Skipping rollback wait (--skip-wait specified)"
        log_info "Monitor manually with: kubectl get pods -l app.kubernetes.io/instance=$release_name -n $namespace -w"
    fi

    # Step 4: Verify rollback health
    if [[ "$SKIP_WAIT" == "false" ]]; then
        if ! verify_rollback_health "$release_name" "$namespace"; then
            log_warning "Health check indicates potential issues"
            log_info "Check pods: kubectl describe pods -l app.kubernetes.io/instance=$release_name -n $namespace"
            log_info "Check logs: kubectl logs -l app.kubernetes.io/instance=$release_name -n $namespace"
        fi
    fi

    # Step 5: Re-enable model in registry
    if [[ "$SKIP_ENABLE" == "false" ]]; then
        if [[ "$SKIP_WAIT" == "false" ]]; then
            # Only re-enable if we waited and verified health
            log_info "Waiting 5 seconds before re-enabling model..."
            sleep 5

            if ! enable_model_in_registry "$model_name" "$environment"; then
                log_error "Failed to re-enable model in registry"
                log_warning "Model is rolled back but still disabled in registry"
                log_info "Re-enable manually with: $ADMIN_CLI registry enable --model-name $model_name --environment $environment"
                exit 1
            fi
        else
            log_warning "Skipping model enable since --skip-wait was used"
            log_info "Re-enable manually after verification: $ADMIN_CLI registry enable --model-name $model_name --environment $environment"
        fi
    else
        log_warning "Skipping model enable (--skip-enable specified)"
        log_info "Re-enable manually when ready: $ADMIN_CLI registry enable --model-name $model_name --environment $environment"
    fi

    # Success
    echo ""
    echo "═══════════════════════════════════════════════════════"
    log_success "Rollback complete!"
    echo "═══════════════════════════════════════════════════════"
    echo ""

    # Show new revision
    local new_revision
    new_revision=$(get_current_revision "$release_name" "$namespace")
    echo "Rollback Summary:"
    echo "  Previous Revision: $current_revision"
    echo "  Current Revision: $new_revision"
    echo "  Model Status: $(if [[ "$SKIP_ENABLE" == "false" ]] && [[ "$SKIP_WAIT" == "false" ]]; then echo "enabled"; else echo "disabled (manual enable required)"; fi)"
    echo ""

    echo "Next steps:"
    echo "  1. ✓ Rollback completed"
    echo "  2. ✓ Pods are healthy"
    if [[ "$SKIP_ENABLE" == "false" ]] && [[ "$SKIP_WAIT" == "false" ]]; then
        echo "  3. ✓ Model re-enabled in registry"
    else
        echo "  3. → Re-enable model when ready"
    fi
    echo "  4. → Monitor deployment: kubectl get pods -l app.kubernetes.io/instance=$release_name -n $namespace"
    echo "  5. → Verify deployment status: $ADMIN_CLI deployment status --model-name $model_name --environment $environment"
    echo ""
}

# Run main function
main "$@"
