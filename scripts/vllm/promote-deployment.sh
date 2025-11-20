#!/bin/bash
#
# Promote vLLM model deployment from staging to production
#
# Purpose:
#   Safely promote a validated vLLM model deployment from staging to production
#   with validation gates and health checks.
#
# Usage:
#   ./scripts/vllm/promote-deployment.sh <model-name> [source-namespace] [target-namespace]
#
# Requirements:
#   - helm binary in PATH
#   - kubectl access to cluster
#   - admin-cli binary in PATH
#   - DATABASE_URL environment variable set
#   - Model already deployed and validated in staging
#
# Requirements Reference:
#   - specs/010-vllm-deployment/tasks.md#T-S010-P05-050
#   - specs/010-vllm-deployment/tasks.md#T-S010-P05-056
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
    log_error "Usage: $0 <model-name> [source-namespace] [target-namespace]"
    log_error "  model-name: Name of the model to promote (e.g., llama-2-7b)"
    log_error "  source-namespace: Source namespace (default: system)"
    log_error "  target-namespace: Target namespace (default: system)"
    log_error ""
    log_error "Examples:"
    log_error "  $0 llama-2-7b                    # Promote from staging to production"
    log_error "  $0 llama-2-7b system production  # Promote with explicit namespaces"
    exit 1
fi

MODEL_NAME="$1"
SOURCE_NAMESPACE="${2:-system}"
TARGET_NAMESPACE="${3:-system}"
SOURCE_ENV="staging"
TARGET_ENV="production"

SOURCE_RELEASE="${MODEL_NAME}-${SOURCE_ENV}"
TARGET_RELEASE="${MODEL_NAME}-${TARGET_ENV}"

log_info "Promotion Configuration:"
log_info "  Model: $MODEL_NAME"
log_info "  Source: $SOURCE_RELEASE (namespace: $SOURCE_NAMESPACE)"
log_info "  Target: $TARGET_RELEASE (namespace: $TARGET_NAMESPACE)"
log_info ""

# Check prerequisites
if ! command -v helm &> /dev/null; then
    log_error "helm command not found in PATH"
    exit 1
fi

if ! command -v kubectl &> /dev/null; then
    log_error "kubectl command not found in PATH"
    exit 1
fi

if ! command -v admin-cli &> /dev/null; then
    log_warn "admin-cli not found, some validation steps will be skipped"
fi

if [ -z "${DATABASE_URL:-}" ]; then
    log_warn "DATABASE_URL not set, registry validation will be skipped"
fi

# Step 1: Validate staging deployment
log_step "1/7: Validating staging deployment..."

# Check if staging release exists
if ! helm list -n "$SOURCE_NAMESPACE" -o json | jq -e ".[] | select(.name==\"$SOURCE_RELEASE\")" > /dev/null; then
    log_error "Staging release not found: $SOURCE_RELEASE (namespace: $SOURCE_NAMESPACE)"
    log_error "Deploy to staging first before promoting"
    exit 1
fi

STAGING_STATUS=$(helm list -n "$SOURCE_NAMESPACE" -o json | jq -r ".[] | select(.name==\"$SOURCE_RELEASE\") | .status")
log_info "Staging status: $STAGING_STATUS"

if [ "$STAGING_STATUS" != "deployed" ]; then
    log_error "Staging deployment is not in 'deployed' status: $STAGING_STATUS"
    log_error "Fix staging deployment before promoting"
    exit 1
fi

# Check staging pods are ready
STAGING_DEPLOYMENT="${SOURCE_RELEASE}"
if ! kubectl get deployment "$STAGING_DEPLOYMENT" -n "$SOURCE_NAMESPACE" &> /dev/null; then
    log_error "Staging deployment not found in Kubernetes"
    exit 1
fi

READY_REPLICAS=$(kubectl get deployment "$STAGING_DEPLOYMENT" -n "$SOURCE_NAMESPACE" -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
DESIRED_REPLICAS=$(kubectl get deployment "$STAGING_DEPLOYMENT" -n "$SOURCE_NAMESPACE" -o jsonpath='{.status.replicas}' 2>/dev/null || echo "1")

if [ "$READY_REPLICAS" -ne "$DESIRED_REPLICAS" ] || [ "$READY_REPLICAS" -eq 0 ]; then
    log_error "Staging deployment not healthy: $READY_REPLICAS/$DESIRED_REPLICAS replicas ready"
    exit 1
fi

log_info "✓ Staging deployment is healthy ($READY_REPLICAS/$DESIRED_REPLICAS replicas ready)"
echo ""

# Step 2: Validate staging model in registry
log_step "2/7: Validating staging model in registry..."
if command -v admin-cli &> /dev/null && [ -n "${DATABASE_URL:-}" ]; then
    if ! admin-cli registry list --environment staging --format json | \
        jq -e ".[] | select(.model_name==\"$MODEL_NAME\" and .status==\"ready\")" > /dev/null 2>&1; then
        log_error "Model not found or not ready in staging registry"
        log_error "Check: admin-cli registry list --environment staging | grep $MODEL_NAME"
        exit 1
    fi
    log_info "✓ Model is registered and ready in staging"
else
    log_warn "Skipping registry validation (admin-cli or DATABASE_URL not configured)"
fi

echo ""

# Step 3: Test staging endpoint
log_step "3/7: Testing staging endpoint..."
STAGING_ENDPOINT="${SOURCE_RELEASE}.${SOURCE_NAMESPACE}.svc.cluster.local:8000"

log_info "Testing health endpoint: http://$STAGING_ENDPOINT/health"
if command -v curl &> /dev/null; then
    if kubectl run test-staging-health-$RANDOM -n "$SOURCE_NAMESPACE" --rm -i --restart=Never \
        --image=curlimages/curl:latest -- \
        curl -sf "http://$STAGING_ENDPOINT/health" > /dev/null 2>&1; then
        log_info "✓ Staging health check passed"
    else
        log_error "Staging health check failed"
        log_error "The model may not be responding correctly"
        exit 1
    fi
else
    log_warn "curl not available, skipping endpoint test"
fi

echo ""

# Step 4: Get staging values
log_step "4/7: Retrieving staging configuration..."
TEMP_DIR=$(mktemp -d)
trap "rm -rf $TEMP_DIR" EXIT

STAGING_VALUES="$TEMP_DIR/staging-values.yaml"
if ! helm get values "$SOURCE_RELEASE" -n "$SOURCE_NAMESPACE" > "$STAGING_VALUES"; then
    log_error "Failed to retrieve staging values"
    exit 1
fi

log_info "✓ Retrieved staging configuration"
log_info "Review staging values:"
cat "$STAGING_VALUES"
echo ""

# Step 5: Confirm promotion
read -p "$(echo -e ${YELLOW}Proceed with promotion to production? [y/N]:${NC} )" -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    log_info "Promotion cancelled by user"
    exit 0
fi

# Step 6: Deploy to production
log_step "5/7: Deploying to production..."

# Check if production release already exists
PROD_EXISTS=false
if helm list -n "$TARGET_NAMESPACE" -o json | jq -e ".[] | select(.name==\"$TARGET_RELEASE\")" > /dev/null; then
    PROD_EXISTS=true
    log_warn "Production release already exists, will upgrade"
fi

# Create production values by modifying environment
PROD_VALUES="$TEMP_DIR/production-values.yaml"
cp "$STAGING_VALUES" "$PROD_VALUES"

# Update environment in values (if not already set via values file)
# This assumes the values file might have environment-specific overrides

if [ "$PROD_EXISTS" = true ]; then
    log_info "Upgrading production deployment..."
    helm upgrade "$TARGET_RELEASE" infra/helm/charts/vllm-deployment \
        --values "$PROD_VALUES" \
        --values infra/helm/charts/vllm-deployment/values-production.yaml \
        --namespace "$TARGET_NAMESPACE" \
        --wait --timeout 15m
else
    log_info "Installing production deployment..."
    helm install "$TARGET_RELEASE" infra/helm/charts/vllm-deployment \
        --values "$PROD_VALUES" \
        --values infra/helm/charts/vllm-deployment/values-production.yaml \
        --namespace "$TARGET_NAMESPACE" \
        --wait --timeout 15m
fi

if [ $? -ne 0 ]; then
    log_error "Production deployment failed"
    log_error "Check Helm and Kubernetes logs"
    log_error "Rollback if needed: helm rollback $TARGET_RELEASE -n $TARGET_NAMESPACE"
    exit 1
fi

log_info "✓ Production deployment successful"
echo ""

# Step 7: Verify production deployment
log_step "6/7: Verifying production deployment..."

# Check pod status
PROD_DEPLOYMENT="${TARGET_RELEASE}"
log_info "Waiting for production deployment to be ready..."
if kubectl wait --for=condition=Available deployment/"$PROD_DEPLOYMENT" \
    -n "$TARGET_NAMESPACE" --timeout=300s 2>/dev/null; then
    log_info "✓ Production deployment is ready"
else
    log_error "Production deployment not ready after 5 minutes"
    log_error "Check pods: kubectl get pods -l app.kubernetes.io/instance=$TARGET_RELEASE -n $TARGET_NAMESPACE"
    exit 1
fi

# Test production endpoint
PROD_ENDPOINT="${TARGET_RELEASE}.${TARGET_NAMESPACE}.svc.cluster.local:8000"
log_info "Testing production health endpoint: http://$PROD_ENDPOINT/health"
if kubectl run test-prod-health-$RANDOM -n "$TARGET_NAMESPACE" --rm -i --restart=Never \
    --image=curlimages/curl:latest -- \
    curl -sf "http://$PROD_ENDPOINT/health" > /dev/null 2>&1; then
    log_info "✓ Production health check passed"
else
    log_error "Production health check failed"
    log_error "Consider rolling back: ./scripts/vllm/rollback-deployment.sh $TARGET_RELEASE $TARGET_NAMESPACE"
    exit 1
fi

echo ""

# Step 8: Register production model
log_step "7/7: Registering production model..."
if command -v admin-cli &> /dev/null && [ -n "${DATABASE_URL:-}" ]; then
    if ./scripts/vllm/register-model.sh "$MODEL_NAME" production "$TARGET_NAMESPACE"; then
        log_info "✓ Model registered in production"
    else
        log_error "Failed to register model in production"
        log_error "Manually register: ./scripts/vllm/register-model.sh $MODEL_NAME production $TARGET_NAMESPACE"
        exit 1
    fi
else
    log_warn "admin-cli or DATABASE_URL not configured, skipping registration"
    log_warn "Manually register: ./scripts/vllm/register-model.sh $MODEL_NAME production $TARGET_NAMESPACE"
fi

echo ""

# Summary
log_info "========================================="
log_info "Promotion Summary"
log_info "========================================="
log_info "Model: $MODEL_NAME"
log_info "Promoted from staging to production"
log_info "Source: $SOURCE_RELEASE (namespace: $SOURCE_NAMESPACE)"
log_info "Target: $TARGET_RELEASE (namespace: $TARGET_NAMESPACE)"
log_info ""
log_info "Verify the promotion:"
log_info "  helm list -n $TARGET_NAMESPACE | grep $TARGET_RELEASE"
log_info "  kubectl get pods -l app.kubernetes.io/instance=$TARGET_RELEASE -n $TARGET_NAMESPACE"
log_info "  admin-cli deployment status --model-name $MODEL_NAME --environment production"
log_info ""
log_info "✓ Promotion completed successfully"
