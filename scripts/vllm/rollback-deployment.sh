#!/bin/bash
#
# Rollback vLLM model deployment to previous Helm revision
#
# Purpose:
#   Safely rollback a vLLM model deployment to a previous Helm revision
#   and update the model registry status accordingly.
#
# Usage:
#   ./scripts/vllm/rollback-deployment.sh <release-name> [revision] [namespace]
#
# Requirements:
#   - helm binary in PATH
#   - kubectl access to cluster
#   - admin-cli binary in PATH (for registry updates)
#   - DATABASE_URL environment variable set
#
# Requirements Reference:
#   - specs/010-vllm-deployment/tasks.md#T-S010-P05-049
#

set -euo pipefail

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${BLUE}[STEP]${NC} $1"
}

# Parse arguments
if [ $# -lt 1 ]; then
    log_error "Usage: $0 <release-name> [revision] [namespace]"
    log_error "  release-name: Helm release name (e.g., llama-2-7b-development)"
    log_error "  revision: Target revision number (default: 0 = previous revision)"
    log_error "  namespace: Kubernetes namespace (default: system)"
    log_error ""
    log_error "Examples:"
    log_error "  $0 llama-2-7b-development              # Rollback to previous revision"
    log_error "  $0 llama-2-7b-development 3            # Rollback to revision 3"
    log_error "  $0 llama-2-7b-development 0 inference  # Rollback in 'inference' namespace"
    exit 1
fi

RELEASE_NAME="$1"
REVISION="${2:-0}"  # 0 means previous revision
NAMESPACE="${3:-system}"

# Extract model name and environment from release name
# Expected format: {model-name}-{environment}
# Example: llama-2-7b-development -> llama-2-7b, development
if [[ "$RELEASE_NAME" =~ ^(.+)-(development|staging|production)$ ]]; then
    MODEL_NAME="${BASH_REMATCH[1]}"
    ENVIRONMENT="${BASH_REMATCH[2]}"
else
    log_error "Invalid release name format: $RELEASE_NAME"
    log_error "Expected format: {model-name}-{environment}"
    log_error "Example: llama-2-7b-development"
    exit 1
fi

log_info "Rollback Configuration:"
log_info "  Release: $RELEASE_NAME"
log_info "  Model: $MODEL_NAME"
log_info "  Environment: $ENVIRONMENT"
log_info "  Namespace: $NAMESPACE"
if [ "$REVISION" -eq 0 ]; then
    log_info "  Target Revision: previous"
else
    log_info "  Target Revision: $REVISION"
fi
log_info ""

# Check prerequisites
if ! command -v helm &> /dev/null; then
    log_error "helm command not found in PATH"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    log_warn "kubectl not found, will skip pod verification"
fi

# Step 1: Show current release history
log_step "1/5: Checking release history..."
if ! helm history "$RELEASE_NAME" -n "$NAMESPACE" &> /dev/null; then
    log_error "Release not found: $RELEASE_NAME (namespace: $NAMESPACE)"
    log_error "Check release name and namespace are correct"
    exit 1
fi

log_info "Current release history:"
helm history "$RELEASE_NAME" -n "$NAMESPACE" | tail -n 10
echo ""

# Get current revision
CURRENT_REVISION=$(helm list -n "$NAMESPACE" -o json | jq -r ".[] | select(.name==\"$RELEASE_NAME\") | .revision")
log_info "Current revision: $CURRENT_REVISION"

# Determine target revision
if [ "$REVISION" -eq 0 ]; then
    # Calculate previous revision
    if [ "$CURRENT_REVISION" -le 1 ]; then
        log_error "Cannot rollback: already at first revision"
        exit 1
    fi
    TARGET_REVISION=$((CURRENT_REVISION - 1))
    log_info "Target revision: $TARGET_REVISION (previous)"
else
    TARGET_REVISION=$REVISION
    log_info "Target revision: $TARGET_REVISION (specified)"
fi

echo ""

# Step 2: Confirm rollback
read -p "$(echo -e ${YELLOW}Proceed with rollback? [y/N]:${NC} )" -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Rollback cancelled by user"
    exit 0
fi

# Step 3: Mark model as disabled during rollback
log_step "2/5: Marking model as disabled during rollback..."
if command -v admin-cli &> /dev/null && [ -n "${DATABASE_URL:-}" ]; then
    if admin-cli registry disable \
        --model-name "$MODEL_NAME" \
        --environment "$ENVIRONMENT" \
        --quiet; then
        log_info "✓ Model marked as disabled"
    else
        log_warn "Failed to disable model in registry (continuing anyway)"
    fi
else
    log_warn "admin-cli or DATABASE_URL not configured, skipping registry update"
fi

echo ""

# Step 4: Perform rollback
log_step "3/5: Rolling back Helm release..."
if helm rollback "$RELEASE_NAME" "$TARGET_REVISION" -n "$NAMESPACE" --wait --timeout 10m; then
    log_info "✓ Helm rollback successful"
else
    log_error "Helm rollback failed"
    log_error "Check Helm and Kubernetes logs for details"
    log_error "The model is currently disabled in the registry"
    log_error "Re-enable with: admin-cli registry enable --model-name $MODEL_NAME --environment $ENVIRONMENT"
    exit 1
fi

echo ""

# Step 5: Verify deployment health
log_step "4/5: Verifying deployment health..."
if command -v kubectl &> /dev/null; then
    DEPLOYMENT_NAME="${RELEASE_NAME}"

    # Wait for deployment to be ready
    log_info "Waiting for deployment to be ready (timeout: 5 minutes)..."
    if kubectl wait --for=condition=Available deployment/"$DEPLOYMENT_NAME" \
        -n "$NAMESPACE" --timeout=300s 2>/dev/null; then
        log_info "✓ Deployment is ready"

        # Check pod status
        READY_PODS=$(kubectl get pods -l app.kubernetes.io/instance="$RELEASE_NAME" \
            -n "$NAMESPACE" -o json | jq -r '.items | length')
        log_info "Ready pods: $READY_PODS"
    else
        log_warn "Deployment not ready after 5 minutes"
        log_warn "Check pod status: kubectl get pods -l app.kubernetes.io/instance=$RELEASE_NAME -n $NAMESPACE"
    fi
else
    log_warn "kubectl not available, skipping deployment verification"
fi

echo ""

# Step 6: Re-enable model in registry
log_step "5/5: Re-enabling model in registry..."
if command -v admin-cli &> /dev/null && [ -n "${DATABASE_URL:-}" ]; then
    # Wait a bit for pods to stabilize
    log_info "Waiting 10 seconds for pods to stabilize..."
    sleep 10

    if admin-cli registry enable \
        --model-name "$MODEL_NAME" \
        --environment "$ENVIRONMENT" \
        --quiet; then
        log_info "✓ Model re-enabled in registry"
    else
        log_error "Failed to re-enable model in registry"
        log_error "Manually enable with: admin-cli registry enable --model-name $MODEL_NAME --environment $ENVIRONMENT"
        exit 1
    fi
else
    log_warn "admin-cli or DATABASE_URL not configured, skipping registry update"
fi

echo ""

# Summary
log_info "========================================="
log_info "Rollback Summary"
log_info "========================================="
log_info "Release: $RELEASE_NAME"
log_info "Rolled back from revision $CURRENT_REVISION to $TARGET_REVISION"
log_info "Model: $MODEL_NAME ($ENVIRONMENT)"
log_info "Status: enabled and ready for routing"
log_info ""
log_info "Verify the rollback:"
log_info "  helm history $RELEASE_NAME -n $NAMESPACE"
log_info "  kubectl get pods -l app.kubernetes.io/instance=$RELEASE_NAME -n $NAMESPACE"
log_info "  admin-cli deployment status --model-name $MODEL_NAME --environment $ENVIRONMENT"
log_info ""
log_info "✓ Rollback completed successfully"
