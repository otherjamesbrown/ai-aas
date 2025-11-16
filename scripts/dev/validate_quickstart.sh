#!/usr/bin/env bash
# Validate quickstart setup for local and remote dev environments
# Runs smoke checks to verify stack health and service connectivity

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=scripts/dev/common.sh
source "${SCRIPT_DIR}/common.sh"

# Default values
MODE="${MODE:-local}"
WORKSPACE_HOST="${WORKSPACE_HOST:-}"
VERBOSE="${VERBOSE:-false}"

show_usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Validates quickstart setup for local or remote dev environment.

Options:
  --mode MODE        Mode: local or remote (default: local)
  --host HOST        Remote workspace host (required for remote mode)
  --verbose, -v      Enable verbose output
  --help, -h         Show this help message

Prerequisites:
  - Local: Docker and Docker Compose v2 installed
  - Remote: Workspace provisioned and accessible via SSH

Examples:
  # Validate local setup
  $0 --mode local

  # Validate remote setup
  $0 --mode remote --host 192.0.2.1

EOF
}

parse_args "$@"

# Validation results
VALIDATION_ERRORS=0

log_validation() {
  local status="$1"
  shift
  local message="$*"
  
  if [[ "${status}" == "pass" ]]; then
    log_success "✓ ${message}"
  elif [[ "${status}" == "fail" ]]; then
    log_error "✗ ${message}"
    VALIDATION_ERRORS=$((VALIDATION_ERRORS + 1))
  else
    log_info "  ${message}"
  fi
}

# Validate local mode
validate_local() {
  log_info "Validating local dev environment setup..."
  
  # Check Docker
  if command_exists docker; then
    log_validation "pass" "Docker installed"
  else
    log_validation "fail" "Docker not found"
    return 1
  fi
  
  # Check Docker Compose v2
  if docker compose version >/dev/null 2>&1; then
    log_validation "pass" "Docker Compose v2 available"
  else
    log_validation "fail" "Docker Compose v2 not available"
    return 1
  fi
  
  # Check port conflicts
  log_info "Checking for port conflicts..."
  if "${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" diagnose >/dev/null 2>&1; then
    log_validation "pass" "No port conflicts detected"
  else
    log_validation "fail" "Port conflicts detected (run 'make diagnose' for details)"
  fi
  
  # Check compose files
  if [[ -f "${PROJECT_ROOT}/.dev/compose/compose.base.yaml" ]]; then
    log_validation "pass" "Compose base file exists"
  else
    log_validation "fail" "Compose base file not found"
  fi
  
  if [[ -f "${PROJECT_ROOT}/.dev/compose/compose.local.yaml" ]]; then
    log_validation "pass" "Compose local override exists"
  else
    log_validation "fail" "Compose local override not found"
  fi
  
  # Start stack if not running
  log_info "Starting dev stack..."
  if ! "${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" status --json >/dev/null 2>&1; then
    log_info "Stack not running, starting..."
    if "${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" up; then
      log_validation "pass" "Dev stack started"
      
      # Wait for components to be healthy
      log_info "Waiting for components to be healthy..."
      sleep 5
    else
      log_validation "fail" "Failed to start dev stack"
      return 1
    fi
  else
    log_validation "pass" "Dev stack already running"
  fi
  
  # Check component health
  log_info "Checking component health..."
  if "${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" status --json | jq -e '.[] | select(.State != "running" or .Health != "healthy") | empty' >/dev/null 2>&1; then
    log_validation "pass" "All components healthy"
  else
    log_warn "Some components may not be healthy yet"
    sleep 5
  fi
  
  # Run smoke test
  log_info "Running integration smoke test..."
  if "${PROJECT_ROOT}/tests/dev/service_happy_path.sh" --service service-template; then
    log_validation "pass" "Integration smoke test passed"
  else
    log_validation "fail" "Integration smoke test failed"
  fi
  
  # Test service template
  log_info "Testing service template connectivity..."
  if command_exists curl; then
    if curl -sf "http://localhost:8000/health" >/dev/null 2>&1; then
      log_validation "pass" "Mock inference service accessible"
    else
      log_validation "fail" "Mock inference service not accessible"
    fi
  else
    log_warn "curl not available, skipping HTTP checks"
  fi
  
  return 0
}

# Validate remote mode
validate_remote() {
  log_info "Validating remote dev environment setup..."
  
  if [[ -z "${WORKSPACE_HOST}" ]]; then
    log_validation "fail" "WORKSPACE_HOST required for remote mode"
    return 1
  fi
  
  # Check SSH connectivity
  log_info "Checking SSH connectivity..."
  if ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "root@${WORKSPACE_HOST}" "echo ok" >/dev/null 2>&1; then
    log_validation "pass" "SSH connectivity to ${WORKSPACE_HOST}"
  else
    log_validation "fail" "Cannot connect to ${WORKSPACE_HOST} via SSH"
    return 1
  fi
  
  # Check remote stack status
  log_info "Checking remote stack status..."
  if "${PROJECT_ROOT}/scripts/dev/remote_lifecycle.sh" status --workspace-host "${WORKSPACE_HOST}" --json >/dev/null 2>&1; then
    log_validation "pass" "Remote stack accessible"
  else
    log_validation "fail" "Remote stack not accessible"
    return 1
  fi
  
  # Check TTL status (if diagnose mode available)
  log_info "Checking workspace TTL status..."
  # Placeholder: Would check TTL via diagnose mode
  
  return 0
}

# Main validation
main() {
  log_info "=== Quickstart Validation ==="
  log_info "Mode: ${MODE}"
  if [[ "${MODE}" == "remote" ]]; then
    log_info "Workspace Host: ${WORKSPACE_HOST}"
  fi
  log_info ""
  
  if [[ "${MODE}" == "local" ]]; then
    validate_local
  elif [[ "${MODE}" == "remote" ]]; then
    validate_remote
  else
    log_error "Invalid mode: ${MODE} (must be 'local' or 'remote')"
    exit 1
  fi
  
  log_info ""
  log_info "=== Validation Summary ==="
  
  if [[ ${VALIDATION_ERRORS} -eq 0 ]]; then
    log_success "All validations passed!"
    exit 0
  else
    log_error "${VALIDATION_ERRORS} validation error(s) found"
    log_info "Review the errors above and refer to:"
    log_info "  - specs/002-local-dev-environment/quickstart.md"
    log_info "  - docs/runbooks/service-dev-connect.md"
    exit 1
  fi
}

main "$@"

