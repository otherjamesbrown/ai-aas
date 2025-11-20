#!/bin/bash
#
# Post-deployment registration hook for vLLM model deployments
#
# Purpose:
#   Automatically registers a vLLM model deployment in the model registry
#   after successful Helm deployment. This enables the API Router Service
#   to route requests to the newly deployed model.
#
# Usage:
#   ./scripts/vllm/register-model.sh <model-name> <environment> [namespace]
#
# Requirements:
#   - admin-cli binary in PATH
#   - DATABASE_URL environment variable set
#   - Model deployed and healthy in Kubernetes
#
# Requirements Reference:
#   - specs/010-vllm-deployment/tasks.md#T-S010-P04-040
#

set -euo pipefail

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
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

# Parse arguments
if [ $# -lt 2 ]; then
    log_error "Usage: $0 <model-name> <environment> [namespace]"
    log_error "  model-name: Name of the model (e.g., llama-2-7b)"
    log_error "  environment: Deployment environment (development, staging, production)"
    log_error "  namespace: Kubernetes namespace (default: system)"
    exit 1
fi

MODEL_NAME="$1"
ENVIRONMENT="$2"
NAMESPACE="${3:-system}"

# Validate environment
case "$ENVIRONMENT" in
    development|staging|production)
        ;;
    *)
        log_error "Invalid environment: $ENVIRONMENT"
        log_error "Must be one of: development, staging, production"
        exit 1
        ;;
esac

log_info "Registering model deployment..."
log_info "  Model: $MODEL_NAME"
log_info "  Environment: $ENVIRONMENT"
log_info "  Namespace: $NAMESPACE"

# Construct endpoint URL following the naming convention
# Format: {model-name}-{environment}.{namespace}.svc.cluster.local:8000
ENDPOINT="${MODEL_NAME}-${ENVIRONMENT}.${NAMESPACE}.svc.cluster.local:8000"

log_info "  Endpoint: $ENDPOINT"

# Check if admin-cli is available
if ! command -v admin-cli &> /dev/null; then
    log_error "admin-cli command not found in PATH"
    log_error "Build it with: cd services/admin-cli && go build -o \$HOME/bin/admin-cli ./cmd/admin-cli"
    exit 1
fi

# Check DATABASE_URL environment variable
if [ -z "${DATABASE_URL:-}" ]; then
    log_warn "DATABASE_URL not set, using default (postgres://postgres:postgres@localhost:5432/ai_aas_operational?sslmode=disable)"
    export DATABASE_URL="postgres://postgres:postgres@localhost:5432/ai_aas_operational?sslmode=disable"
fi

# Verify the deployment exists and is healthy (optional)
log_info "Verifying deployment..."
if command -v kubectl &> /dev/null; then
    if kubectl get deployment "${MODEL_NAME}-${ENVIRONMENT}" -n "$NAMESPACE" &> /dev/null; then
        READY=$(kubectl get deployment "${MODEL_NAME}-${ENVIRONMENT}" -n "$NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
        DESIRED=$(kubectl get deployment "${MODEL_NAME}-${ENVIRONMENT}" -n "$NAMESPACE" -o jsonpath='{.status.replicas}' 2>/dev/null || echo "1")

        if [ "$READY" -eq "$DESIRED" ] && [ "$READY" -gt 0 ]; then
            log_info "✓ Deployment is healthy ($READY/$DESIRED replicas ready)"
        else
            log_warn "Deployment not fully ready ($READY/$DESIRED replicas ready)"
            log_warn "Registration will proceed, but model may not be immediately available"
        fi
    else
        log_warn "Deployment not found in Kubernetes (namespace: $NAMESPACE)"
        log_warn "Registration will proceed, but verify deployment exists"
    fi
else
    log_warn "kubectl not found, skipping deployment verification"
fi

# Register the model
log_info "Registering model in registry..."

if admin-cli registry register \
    --model-name "$MODEL_NAME" \
    --endpoint "$ENDPOINT" \
    --environment "$ENVIRONMENT" \
    --namespace "$NAMESPACE" \
    --format json; then
    log_info "✓ Model registered successfully"
    log_info ""
    log_info "The model is now available for API routing."
    log_info "Test with:"
    log_info "  curl -X POST http://api-router:8080/v1/completions \\"
    log_info "    -H 'Content-Type: application/json' \\"
    log_info "    -d '{\"model\": \"$MODEL_NAME\", \"prompt\": \"Hello\", \"max_tokens\": 10}'"
    exit 0
else
    log_error "Failed to register model"
    log_error "Check DATABASE_URL and database connectivity"
    exit 1
fi
