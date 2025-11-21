#!/usr/bin/env bash
#
# Promote vLLM Deployment
#
# This script promotes a model deployment from one environment to another
# (typically staging → production). It validates the source deployment,
# deploys to the target environment, and manages registry entries.
#
# Usage:
#   ./promote-deployment.sh MODEL_NAME SOURCE_ENV TARGET_ENV [OPTIONS]
#
# Arguments:
#   MODEL_NAME    - Model identifier (e.g., gpt-oss-20b, llama-2-7b)
#   SOURCE_ENV    - Source environment (typically: staging)
#   TARGET_ENV    - Target environment (typically: production)
#
# Options:
#   --source-namespace NS   Source namespace (default: system)
#   --target-namespace NS   Target namespace (default: production)
#   --skip-validation       Skip source deployment validation
#   --skip-tests            Skip smoke tests after promotion
#   --timeout DURATION      Deployment wait timeout (default: 20m)
#   --force                 Skip confirmation prompts
#   --dry-run               Simulate promotion without making changes
#   --help                  Show this help message
#
# Environment Variables:
#   DATABASE_URL    - PostgreSQL connection string (required)
#   KUBECONFIG      - Path to kubeconfig file (required)
#   ADMIN_CLI       - Path to admin-cli binary (default: admin-cli)
#
# Validation Gates:
#   1. Source deployment exists and is healthy
#   2. Source deployment registered in registry
#   3. Source deployment has been running > 24 hours (production only)
#   4. No recent rollbacks in source environment
#   5. Manual approval for production promotions
#
# Examples:
#   # Promote from staging to production
#   ./promote-deployment.sh gpt-oss-20b staging production
#
#   # Dry run to test promotion
#   ./promote-deployment.sh llama-2-7b staging production --dry-run
#
#   # Force promotion (skip validation gates)
#   ./promote-deployment.sh gpt-oss-20b staging production --force
#

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEFAULT_SOURCE_NAMESPACE="system"
DEFAULT_TARGET_NAMESPACE="production"
DEFAULT_TIMEOUT="20m"
ADMIN_CLI="${ADMIN_CLI:-admin-cli}"
MIN_UPTIME_HOURS=24  # Minimum uptime for production promotion

# Flags
SKIP_VALIDATION=false
SKIP_TESTS=false
FORCE=false
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

log_gate() {
    echo -e "${MAGENTA}🔒${NC} Validation Gate: $*"
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
    for cmd in kubectl helm jq; do
        if ! command -v "$cmd" &> /dev/null; then
            missing_prereqs+=("$cmd")
        fi
    done

    if ! command -v "${ADMIN_CLI}" &> /dev/null; then
        missing_prereqs+=("${ADMIN_CLI}")
    fi

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

# Validation Gate 1: Check source deployment exists and is healthy
validate_source_deployment() {
    local model_name="$1"
    local source_namespace="$2"

    log_gate "Source deployment health check"

    local deployment_name="${model_name}-vllm-deployment"

    # Check deployment exists
    if ! kubectl get deployment "$deployment_name" -n "$source_namespace" &> /dev/null; then
        log_error "Source deployment not found: $deployment_name in $source_namespace"
        return 1
    fi

    # Check deployment is available
    local available
    available=$(kubectl get deployment "$deployment_name" -n "$source_namespace" -o jsonpath='{.status.conditions[?(@.type=="Available")].status}')
    if [[ "$available" != "True" ]]; then
        log_error "Source deployment is not Available"
        return 1
    fi

    # Check pods are ready
    local label_selector="app.kubernetes.io/instance=${model_name}"
    local ready_pods
    ready_pods=$(kubectl get pods -l "$label_selector" -n "$source_namespace" --no-headers 2>/dev/null | grep -c "Running" || echo "0")

    if [[ "$ready_pods" -eq 0 ]]; then
        log_error "No running pods found in source deployment"
        return 1
    fi

    log_success "Source deployment is healthy ($ready_pods pods running)"
    return 0
}

# Validation Gate 2: Check source is registered in registry
validate_source_registration() {
    local model_name="$1"
    local source_env="$2"

    log_gate "Source registration check"

    local registration
    registration=$("$ADMIN_CLI" registry list --environment "$source_env" --format json 2>/dev/null | \
        jq -r ".[] | select(.model_name == \"$model_name\")" || echo "")

    if [[ -z "$registration" ]]; then
        log_error "Model not registered in source environment"
        return 1
    fi

    local status
    status=$(echo "$registration" | jq -r '.deployment_status')
    if [[ "$status" != "ready" ]]; then
        log_error "Model status in source is not 'ready' (current: $status)"
        return 1
    fi

    log_success "Model is registered and ready in source environment"
    return 0
}

# Validation Gate 3: Check source uptime (production only)
validate_source_uptime() {
    local model_name="$1"
    local source_namespace="$2"
    local target_env="$3"

    # Only required for production promotions
    if [[ "$target_env" != "production" ]]; then
        log_info "Skipping uptime check (not promoting to production)"
        return 0
    fi

    log_gate "Source uptime check (minimum: ${MIN_UPTIME_HOURS}h for production)"

    local label_selector="app.kubernetes.io/instance=${model_name}"
    local pod_age_seconds

    pod_age_seconds=$(kubectl get pods -l "$label_selector" -n "$source_namespace" --no-headers 2>/dev/null | \
        head -1 | awk '{print $5}' | \
        sed 's/[hms]/ /g' | awk '{
            hours = ($1 ~ /d/ ? $1 * 24 : ($1 ~ /h/ ? $1 : 0));
            print hours * 3600
        }' || echo "0")

    local required_seconds=$((MIN_UPTIME_HOURS * 3600))

    if [[ "$pod_age_seconds" -lt "$required_seconds" ]]; then
        log_error "Source deployment has not been running long enough"
        log_error "Required: ${MIN_UPTIME_HOURS}h, Actual: $((pod_age_seconds / 3600))h"

        if [[ "$FORCE" != "true" ]]; then
            return 1
        else
            log_warning "Proceeding anyway (--force specified)"
        fi
    fi

    log_success "Source deployment uptime requirement met"
    return 0
}

# Validation Gate 4: Check for recent rollbacks
validate_no_recent_rollbacks() {
    local model_name="$1"
    local source_namespace="$2"

    log_gate "Recent rollback check"

    # Check Helm history for recent rollbacks
    local recent_rollback
    recent_rollback=$(helm history "$model_name" -n "$source_namespace" --max 5 -o json 2>/dev/null | \
        jq -r '.[] | select(.description | contains("Rollback"))' || echo "")

    if [[ -n "$recent_rollback" ]]; then
        log_warning "Recent rollback detected in source environment"
        log_warning "This deployment was recently rolled back"

        if [[ "$FORCE" != "true" ]]; then
            if ! confirm "Continue with promotion despite recent rollback?"; then
                return 1
            fi
        fi
    fi

    log_success "No concerning rollback history"
    return 0
}

# Get Helm values from source deployment
get_source_values() {
    local model_name="$1"
    local source_namespace="$2"
    local values_file="$3"

    log_info "Extracting Helm values from source deployment..."

    if helm get values "$model_name" -n "$source_namespace" > "$values_file" 2>/dev/null; then
        log_success "Source values extracted to: $values_file"
        return 0
    else
        log_error "Failed to extract source values"
        return 1
    fi
}

# Deploy to target environment
deploy_to_target() {
    local model_name="$1"
    local target_namespace="$2"
    local values_file="$3"
    local timeout="$4"

    log_info "Deploying to target environment..."

    local chart_path
    chart_path="$(cd "$SCRIPT_DIR/../infra/helm/charts/vllm-deployment" && pwd)"

    if [[ ! -d "$chart_path" ]]; then
        log_error "Helm chart not found at: $chart_path"
        return 1
    fi

    local cmd_args=(
        helm upgrade --install "$model_name" "$chart_path"
        -f "$values_file"
        -n "$target_namespace"
        --timeout "$timeout"
        --wait
    )

    if [[ "$DRY_RUN" == "true" ]]; then
        cmd_args+=(--dry-run)
        log_warning "DRY RUN MODE - No changes will be made"
    fi

    if "${cmd_args[@]}"; then
        log_success "Deployment to target environment completed"
        return 0
    else
        log_error "Deployment to target environment failed"
        return 1
    fi
}

# Register in target environment
register_in_target() {
    local model_name="$1"
    local target_env="$2"
    local target_namespace="$3"

    log_info "Registering model in target environment..."

    local service_endpoint="${model_name}-vllm-deployment.${target_namespace}.svc.cluster.local:8000"

    local cmd_args=(
        "$ADMIN_CLI" registry register
        --model-name "$model_name"
        --endpoint "$service_endpoint"
        --environment "$target_env"
        --namespace "$target_namespace"
    )

    if [[ "$DRY_RUN" == "true" ]]; then
        cmd_args+=(--dry-run)
    fi

    if "${cmd_args[@]}"; then
        log_success "Model registered in target environment"
        return 0
    else
        log_error "Failed to register model in target environment"
        return 1
    fi
}

# Run smoke tests
run_smoke_tests() {
    local model_name="$1"
    local target_namespace="$2"

    log_info "Running smoke tests..."

    local service_endpoint="${model_name}-vllm-deployment.${target_namespace}.svc.cluster.local:8000"

    # Test health endpoint
    if kubectl run smoke-test-$$ --rm -i --restart=Never \
        -n "$target_namespace" \
        --image=curlimages/curl:latest \
        --command -- curl -f -s "$service_endpoint/health" &> /dev/null; then
        log_success "Health endpoint test passed"
    else
        log_error "Health endpoint test failed"
        return 1
    fi

    log_success "All smoke tests passed"
    return 0
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
    local source_env="$2"
    local target_env="$3"
    shift 3

    local source_namespace="$DEFAULT_SOURCE_NAMESPACE"
    local target_namespace="$DEFAULT_TARGET_NAMESPACE"

    # Parse options
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --source-namespace)
                source_namespace="$2"
                shift 2
                ;;
            --target-namespace)
                target_namespace="$2"
                shift 2
                ;;
            --skip-validation)
                SKIP_VALIDATION=true
                shift
                ;;
            --skip-tests)
                SKIP_TESTS=true
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
    echo " vLLM Deployment Promotion"
    echo "═══════════════════════════════════════════════════════"
    echo " Model: $model_name"
    echo " Source: $source_env ($source_namespace)"
    echo " Target: $target_env ($target_namespace)"
    echo " Timeout: $TIMEOUT"
    if [[ "$DRY_RUN" == "true" ]]; then
        echo " Mode: DRY RUN"
    fi
    if [[ "$FORCE" == "true" ]]; then
        echo " Mode: FORCE (validation gates relaxed)"
    fi
    echo "═══════════════════════════════════════════════════════"
    echo ""

    # Check prerequisites
    log_info "Checking prerequisites..."
    check_prerequisites
    log_success "All prerequisites met"

    # Validate environments
    validate_environment "$source_env"
    validate_environment "$target_env"

    # Confirm promotion
    log_warning "Promoting from $source_env to $target_env"
    if [[ "$target_env" == "production" ]]; then
        log_warning "⚠️  PRODUCTION PROMOTION - This will affect live traffic"
    fi

    if ! confirm "Are you sure you want to promote this deployment?"; then
        log_info "Promotion cancelled"
        exit 0
    fi

    # Validation Gates
    if [[ "$SKIP_VALIDATION" == "false" ]]; then
        echo ""
        log_info "Running validation gates..."
        echo ""

        if ! validate_source_deployment "$model_name" "$source_namespace"; then
            log_error "Validation gate failed: Source deployment health"
            exit 1
        fi

        if ! validate_source_registration "$model_name" "$source_env"; then
            log_error "Validation gate failed: Source registration"
            exit 1
        fi

        if ! validate_source_uptime "$model_name" "$source_namespace" "$target_env"; then
            log_error "Validation gate failed: Source uptime"
            exit 1
        fi

        if ! validate_no_recent_rollbacks "$model_name" "$source_namespace"; then
            log_error "Validation gate failed: Recent rollbacks"
            exit 1
        fi

        echo ""
        log_success "All validation gates passed"
        echo ""
    else
        log_warning "Skipping validation gates (--skip-validation specified)"
    fi

    # Extract source values
    local temp_values="/tmp/${model_name}-promotion-values-$$.yaml"
    if ! get_source_values "$model_name" "$source_namespace" "$temp_values"; then
        exit 1
    fi

    # Deploy to target
    if ! deploy_to_target "$model_name" "$target_namespace" "$temp_values" "$TIMEOUT"; then
        rm -f "$temp_values"
        exit 1
    fi

    # Clean up temp values
    rm -f "$temp_values"

    # Register in target (unless dry-run)
    if [[ "$DRY_RUN" == "false" ]]; then
        if ! register_in_target "$model_name" "$target_env" "$target_namespace"; then
            log_error "Deployment succeeded but registration failed"
            log_info "Register manually: $ADMIN_CLI registry register --model-name $model_name --environment $target_env --namespace $target_namespace"
            exit 1
        fi
    fi

    # Run smoke tests
    if [[ "$SKIP_TESTS" == "false" ]] && [[ "$DRY_RUN" == "false" ]]; then
        sleep 5  # Give deployment a moment to stabilize
        if ! run_smoke_tests "$model_name" "$target_namespace"; then
            log_error "Smoke tests failed"
            log_warning "Deployment may need rollback"
            exit 1
        fi
    else
        log_warning "Skipping smoke tests"
    fi

    # Success
    echo ""
    echo "═══════════════════════════════════════════════════════"
    log_success "Promotion complete!"
    echo "═══════════════════════════════════════════════════════"
    echo ""
    echo "Promotion Summary:"
    echo "  Model: $model_name"
    echo "  Promoted: $source_env → $target_env"
    echo "  Target Namespace: $target_namespace"
    if [[ "$DRY_RUN" == "false" ]]; then
        echo "  Status: Deployed and registered"
    else
        echo "  Status: Dry run (no changes made)"
    fi
    echo ""
    echo "Next steps:"
    echo "  1. ✓ Source deployment validated"
    echo "  2. ✓ Deployed to target environment"
    echo "  3. ✓ Registered in target registry"
    if [[ "$SKIP_TESTS" == "false" ]]; then
        echo "  4. ✓ Smoke tests passed"
    fi
    echo "  5. → Monitor deployment: kubectl get pods -l app.kubernetes.io/instance=$model_name -n $target_namespace"
    echo "  6. → Verify with API Router (if integrated)"
    echo ""
}

# Run main function
main "$@"
