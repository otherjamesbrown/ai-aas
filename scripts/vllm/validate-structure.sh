#!/usr/bin/env bash
# Validate vLLM deployment structure - check files, syntax, and basic validation
# Usage: ./scripts/vllm/validate-structure.sh

set -euo pipefail

CHART_PATH="infra/helm/charts/vllm-deployment"
MIGRATION_PATH="db/migrations/operational/20250127120000_add_deployment_metadata"

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

ERRORS=0
WARNINGS=0

log_info() {
  printf "${BLUE}[INFO]${NC} %s\n" "$*" >&2
}

log_success() {
  printf "${GREEN}[SUCCESS]${NC} %s\n" "$*" >&2
}

log_error() {
  printf "${RED}[ERROR]${NC} %s\n" "$*" >&2
  ((ERRORS++)) || true
}

log_warn() {
  printf "${YELLOW}[WARN]${NC} %s\n" "$*" >&2
  ((WARNINGS++)) || true
}

# Test 1: Check Helm chart structure
test_helm_chart_structure() {
  log_info "Test 1: Checking Helm chart structure..."
  
  local required_files=(
    "${CHART_PATH}/Chart.yaml"
    "${CHART_PATH}/values.yaml"
    "${CHART_PATH}/values-development.yaml"
    "${CHART_PATH}/values-staging.yaml"
    "${CHART_PATH}/values-production.yaml"
    "${CHART_PATH}/templates/_helpers.tpl"
    "${CHART_PATH}/templates/deployment.yaml"
    "${CHART_PATH}/templates/service.yaml"
    "${CHART_PATH}/templates/configmap.yaml"
    "${CHART_PATH}/templates/networkpolicy.yaml"
    "${CHART_PATH}/templates/servicemonitor.yaml"
    "${CHART_PATH}/templates/serviceaccount.yaml"
    "${CHART_PATH}/templates/job-pre-install-check.yaml"
  )
  
  for file in "${required_files[@]}"; do
    if [ -f "$file" ]; then
      log_success "Found: ${file}"
    else
      log_error "Missing: ${file}"
    fi
  done
}

# Test 2: Validate YAML syntax
test_yaml_syntax() {
  log_info "Test 2: Validating YAML syntax..."
  
  if command -v yamllint >/dev/null 2>&1; then
    for file in "${CHART_PATH}"/*.yaml "${CHART_PATH}/templates"/*.yaml; do
      if [ -f "$file" ]; then
        if yamllint "$file" >/dev/null 2>&1; then
          log_success "YAML valid: $(basename "$file")"
        else
          log_warn "YAML issues in: $(basename "$file")"
        fi
      fi
    done
  else
    log_warn "yamllint not available, skipping YAML syntax validation"
    log_info "Install: pip install yamllint"
  fi
}

# Test 3: Check Chart.yaml content
test_chart_yaml() {
  log_info "Test 3: Validating Chart.yaml..."
  
  if [ -f "${CHART_PATH}/Chart.yaml" ]; then
    if grep -q "name: vllm-deployment" "${CHART_PATH}/Chart.yaml"; then
      log_success "Chart name is correct"
    else
      log_error "Chart name not found or incorrect"
    fi
    
    if grep -q "apiVersion: v2" "${CHART_PATH}/Chart.yaml"; then
      log_success "Chart API version is v2"
    else
      log_warn "Chart API version may be incorrect"
    fi
  fi
}

# Test 4: Check template syntax (basic)
test_template_syntax() {
  log_info "Test 4: Checking template syntax (basic)..."
  
  local templates=(
    "${CHART_PATH}/templates/deployment.yaml"
    "${CHART_PATH}/templates/service.yaml"
  )
  
  for template in "${templates[@]}"; do
    if [ -f "$template" ]; then
      # Check for basic Helm template syntax
      if grep -q "{{" "$template" && grep -q "}}" "$template"; then
        log_success "Template syntax found in: $(basename "$template")"
      else
        log_warn "No template syntax in: $(basename "$template")"
      fi
      
      # Check for required Kubernetes fields
      if grep -q "apiVersion:" "$template" && grep -q "kind:" "$template"; then
        log_success "Kubernetes resource structure in: $(basename "$template")"
      else
        log_error "Missing Kubernetes resource structure in: $(basename "$template")"
      fi
    fi
  done
}

# Test 5: Check database migrations
test_database_migrations() {
  log_info "Test 5: Checking database migrations..."
  
  if [ -f "${MIGRATION_PATH}.up.sql" ]; then
    log_success "Up migration found"
    
    # Check for required columns
    local required_columns=(
      "deployment_endpoint"
      "deployment_status"
      "deployment_environment"
      "deployment_namespace"
      "last_health_check_at"
    )
    
    for column in "${required_columns[@]}"; do
      if grep -qi "$column" "${MIGRATION_PATH}.up.sql"; then
        log_success "Column found: ${column}"
      else
        log_error "Column missing: ${column}"
      fi
    done
  else
    log_error "Up migration not found: ${MIGRATION_PATH}.up.sql"
  fi
  
  if [ -f "${MIGRATION_PATH}.down.sql" ]; then
    log_success "Down migration found"
  else
    log_error "Down migration not found: ${MIGRATION_PATH}.down.sql"
  fi
}

# Test 6: Check deployment scripts
test_deployment_scripts() {
  log_info "Test 6: Checking deployment scripts..."
  
  local scripts=(
    "scripts/vllm/deploy-with-retry.sh"
    "scripts/vllm/verify-deployment.sh"
    "scripts/vllm/test-helm-chart.sh"
  )
  
  for script in "${scripts[@]}"; do
    if [ -f "$script" ]; then
      if [ -x "$script" ]; then
        log_success "Script exists and is executable: ${script}"
      else
        log_warn "Script exists but not executable: ${script}"
      fi
      
      # Check for shebang
      if head -n1 "$script" | grep -q "^#!/"; then
        log_success "Script has shebang: ${script}"
      else
        log_warn "Script missing shebang: ${script}"
      fi
    else
      log_error "Script not found: ${script}"
    fi
  done
}

# Test 7: Check documentation
test_documentation() {
  log_info "Test 7: Checking documentation..."
  
  local docs=(
    "docs/deployment-workflow.md"
    "docs/model-initialization.md"
    "specs/010-vllm-deployment/PROGRESS.md"
  )
  
  for doc in "${docs[@]}"; do
    if [ -f "$doc" ]; then
      log_success "Documentation found: ${doc}"
    else
      log_warn "Documentation missing: ${doc}"
    fi
  done
}

# Test 8: Check values files for environment-specific fields
test_values_files() {
  log_info "Test 8: Checking values files..."
  
  # Only check fields that should be present in environment-specific files
  # Other fields are inherited from base values.yaml
  local env_specific_fields=(
    "environment:"
    "namespace:"
  )
  
  for env in development staging production; do
    local values_file="${CHART_PATH}/values-${env}.yaml"
    if [ -f "$values_file" ]; then
      log_success "Values file found: values-${env}.yaml"
      
      for field in "${env_specific_fields[@]}"; do
        if grep -q "^${field}" "$values_file" || grep -q "  ${field}" "$values_file"; then
          log_success "Field found in ${env}: ${field}"
        else
          log_warn "Field missing in ${env}: ${field}"
        fi
      done
    else
      log_error "Values file missing: values-${env}.yaml"
    fi
  done
}

# Run all tests
main() {
  log_info "Starting structure validation..."
  echo ""
  
  test_helm_chart_structure
  echo ""
  
  test_yaml_syntax
  echo ""
  
  test_chart_yaml
  echo ""
  
  test_template_syntax
  echo ""
  
  test_database_migrations
  echo ""
  
  test_deployment_scripts
  echo ""
  
  test_documentation
  echo ""
  
  test_values_files
  echo ""
  
  # Summary
  log_info "Validation complete!"
  echo ""
  log_info "Summary:"
  log_info "  Errors: ${ERRORS}"
  log_info "  Warnings: ${WARNINGS}"
  echo ""
  
  if [ $ERRORS -eq 0 ]; then
    log_success "All critical checks passed!"
    if [ $WARNINGS -gt 0 ]; then
      log_warn "Some warnings were found, but structure is valid"
    fi
    exit 0
  else
    log_error "Validation failed with ${ERRORS} error(s)"
    exit 1
  fi
}

main "$@"

