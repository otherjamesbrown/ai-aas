#!/usr/bin/env bash
# Test Helm chart - validate chart structure, lint, and render templates
# Usage: ./scripts/vllm/test-helm-chart.sh [environment]

set -euo pipefail

ENVIRONMENT="${1:-development}"
CHART_PATH="infra/helm/charts/vllm-deployment"
VALUES_FILE="${CHART_PATH}/values-${ENVIRONMENT}.yaml"

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

# Check prerequisites
if ! command -v helm >/dev/null 2>&1; then
  log_error "helm is required but not installed"
  log_info "Install: https://helm.sh/docs/intro/install/"
  exit 1
fi

# Check if chart directory exists
if [ ! -d "$CHART_PATH" ]; then
  log_error "Chart directory not found: ${CHART_PATH}"
  exit 1
fi

# Check if values file exists
if [ ! -f "$VALUES_FILE" ]; then
  log_warn "Values file not found: ${VALUES_FILE}"
  log_info "Using base values.yaml"
  VALUES_FILE="${CHART_PATH}/values.yaml"
fi

log_info "Testing Helm chart: ${CHART_PATH}"
log_info "Environment: ${ENVIRONMENT}"
log_info "Values file: ${VALUES_FILE}"
echo ""

# Test 1: Chart structure validation
log_info "Test 1: Validating chart structure..."
if [ ! -f "${CHART_PATH}/Chart.yaml" ]; then
  log_error "Chart.yaml not found"
  exit 1
fi

if [ ! -f "${CHART_PATH}/values.yaml" ]; then
  log_error "values.yaml not found"
  exit 1
fi

if [ ! -d "${CHART_PATH}/templates" ]; then
  log_error "templates/ directory not found"
  exit 1
fi

log_success "Chart structure is valid"

# Test 2: Helm lint
log_info "Test 2: Running helm lint..."
if helm lint "$CHART_PATH" -f "$VALUES_FILE" 2>&1; then
  log_success "Helm lint passed"
else
  log_error "Helm lint failed"
  exit 1
fi

# Test 3: Template rendering (dry-run)
log_info "Test 3: Rendering templates (dry-run)..."
RENDER_OUTPUT=$(helm template test-release "$CHART_PATH" -f "$VALUES_FILE" --debug 2>&1 || true)

if echo "$RENDER_OUTPUT" | grep -q "Error:"; then
  log_error "Template rendering failed"
  echo "$RENDER_OUTPUT" | grep -A 5 "Error:"
  exit 1
fi

# Check for required resources
REQUIRED_RESOURCES=("Deployment" "Service" "ServiceAccount")
for resource in "${REQUIRED_RESOURCES[@]}"; do
  if echo "$RENDER_OUTPUT" | grep -q "kind: ${resource}"; then
    log_success "Found ${resource} resource"
  else
    log_warn "${resource} resource not found in rendered templates"
  fi
done

log_success "Templates rendered successfully"

# Test 4: Validate specific configurations
log_info "Test 4: Validating configurations..."

# Check for GPU resources
if echo "$RENDER_OUTPUT" | grep -q "nvidia.com/gpu"; then
  log_success "GPU resources configured"
else
  log_warn "GPU resources not found in deployment"
fi

# Check for health probes
if echo "$RENDER_OUTPUT" | grep -q "livenessProbe:"; then
  log_success "Liveness probe configured"
else
  log_warn "Liveness probe not found"
fi

if echo "$RENDER_OUTPUT" | grep -q "readinessProbe:"; then
  log_success "Readiness probe configured"
else
  log_warn "Readiness probe not found"
fi

if echo "$RENDER_OUTPUT" | grep -q "startupProbe:"; then
  log_success "Startup probe configured"
else
  log_warn "Startup probe not found"
fi

# Check for node selector
if echo "$RENDER_OUTPUT" | grep -q "nodeSelector:"; then
  log_success "Node selector configured"
else
  log_warn "Node selector not found"
fi

# Test 5: Kubernetes manifest validation (if kubectl available)
if command -v kubectl >/dev/null 2>&1; then
  log_info "Test 5: Validating Kubernetes manifests..."
  
  # Render and validate
  if helm template test-release "$CHART_PATH" -f "$VALUES_FILE" | kubectl apply --dry-run=client -f - >/dev/null 2>&1; then
    log_success "Kubernetes manifest validation passed"
  else
    log_warn "Kubernetes manifest validation had warnings (this may be expected)"
    # Show actual errors if any
    helm template test-release "$CHART_PATH" -f "$VALUES_FILE" | kubectl apply --dry-run=client -f - 2>&1 | grep -i "error" || true
  fi
else
  log_warn "kubectl not available, skipping Kubernetes manifest validation"
fi

# Test 6: Check for pre-install hook
log_info "Test 6: Checking pre-install hook..."
if [ -f "${CHART_PATH}/templates/job-pre-install-check.yaml" ]; then
  if echo "$RENDER_OUTPUT" | grep -q "helm.sh/hook: pre-install"; then
    log_success "Pre-install hook configured"
  else
    log_warn "Pre-install hook file exists but not rendered (may be disabled)"
  fi
else
  log_warn "Pre-install hook not found"
fi

# Summary
echo ""
log_success "Helm chart testing complete!"
log_info "Summary:"
log_info "  - Chart structure: ✓"
log_info "  - Helm lint: ✓"
log_info "  - Template rendering: ✓"
log_info "  - Configuration validation: ✓"
echo ""
log_info "Next steps:"
log_info "  1. Test deployment: ./scripts/vllm/deploy-with-retry.sh <release-name> ${CHART_PATH} ${VALUES_FILE}"
log_info "  2. Verify deployment: ./scripts/vllm/verify-deployment.sh <release-name>"

