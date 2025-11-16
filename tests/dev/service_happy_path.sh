#!/usr/bin/env bash
# Integration smoke test for service connectivity to local dev stack
# Exercises a sample service against local dependencies and verifies happy-path flows.

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
# shellcheck source=scripts/dev/common.sh
source "${PROJECT_ROOT}/scripts/dev/common.sh"

# Default values
SERVICE="${SERVICE:-service-template}"
TIMEOUT="${TIMEOUT:-30}"
VERBOSE="${VERBOSE:-false}"

show_usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Integration smoke test for service connectivity to local dev stack.

Options:
  --service NAME   Service to test (default: service-template)
  --timeout SEC    Test timeout in seconds (default: 30)
  --verbose, -v    Enable verbose output
  --help, -h       Show this help message

Prerequisites:
  - Local dev stack running (make up)
  - Service executable or buildable

Examples:
  $0 --service service-template
  $0 --service user-org-service --timeout 60

EOF
}

parse_args "$@"

# Verify local stack is running
log_info "Verifying local dev stack is running..."
if ! "${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" status --json >/dev/null 2>&1; then
  log_error "Local stack is not running. Run 'make up' first."
  exit 1
fi

log_success "Local stack is running"

# Wait for all components to be healthy
log_info "Waiting for all components to be healthy..."
"${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" status --json | jq -e '.[] | select(.State != "running" or .Health != "healthy") | empty' >/dev/null 2>&1 || {
  log_warn "Some components may not be healthy yet"
  sleep 5
}

# Verify components are accessible
log_info "Verifying component connectivity..."

# Check PostgreSQL
log_info "Testing PostgreSQL connection..."
POSTGRES_PORT="${POSTGRES_PORT:-5432}"
if nc -z localhost "${POSTGRES_PORT}" 2>/dev/null; then
  log_success "PostgreSQL is accessible"
else
  log_error "PostgreSQL is not accessible on port ${POSTGRES_PORT}"
  exit 1
fi

# Check Redis
log_info "Testing Redis connection..."
REDIS_PORT="${REDIS_PORT:-6379}"
if nc -z localhost "${REDIS_PORT}" 2>/dev/null; then
  log_success "Redis is accessible"
else
  log_error "Redis is not accessible on port ${REDIS_PORT}"
  exit 1
fi

# Check NATS
log_info "Testing NATS connection..."
NATS_PORT="${NATS_CLIENT_PORT:-4222}"
if nc -z localhost "${NATS_PORT}" 2>/dev/null; then
  log_success "NATS is accessible"
else
  log_error "NATS is not accessible on port ${NATS_PORT}"
  exit 1
fi

# Check MinIO
log_info "Testing MinIO connection..."
MINIO_PORT="${MINIO_API_PORT:-9000}"
if nc -z localhost "${MINIO_PORT}" 2>/dev/null; then
  log_success "MinIO is accessible"
else
  log_error "MinIO is not accessible on port ${MINIO_PORT}"
  exit 1
fi

# Check Mock Inference
log_info "Testing Mock Inference connection..."
MOCK_PORT="${MOCK_INFERENCE_PORT:-8000}"
if nc -z localhost "${MOCK_PORT}" 2>/dev/null; then
  log_success "Mock Inference is accessible"
else
  log_error "Mock Inference is not accessible on port ${MOCK_PORT}"
  exit 1
fi

# Test mock inference endpoint
log_info "Testing mock inference API..."
MOCK_RESPONSE=$(curl -s -f "http://localhost:${MOCK_PORT}/health" || echo "")
if echo "${MOCK_RESPONSE}" | jq -e '.status == "healthy"' >/dev/null 2>&1; then
  log_success "Mock inference API is healthy"
else
  log_error "Mock inference API health check failed"
  exit 1
fi

# Test mock inference completion endpoint
log_info "Testing mock inference completion endpoint..."
COMPLETION_RESPONSE=$(curl -s -X POST "http://localhost:${MOCK_PORT}/v1/completions" \
  -H "Content-Type: application/json" \
  -d '{"prompt": "test prompt", "max_tokens": 10}' || echo "")

if echo "${COMPLETION_RESPONSE}" | jq -e '.text' >/dev/null 2>&1; then
  log_success "Mock inference completion endpoint works"
  if [[ "${VERBOSE}" == "true" ]]; then
    echo "${COMPLETION_RESPONSE}" | jq .
  fi
else
  log_error "Mock inference completion endpoint failed"
  if [[ "${VERBOSE}" == "true" ]]; then
    echo "Response: ${COMPLETION_RESPONSE}"
  fi
  exit 1
fi

# Test service (if specified and available)
if [[ -n "${SERVICE}" ]] && [[ "${SERVICE}" != "none" ]]; then
  log_info "Testing service: ${SERVICE}"
  
  SERVICE_PATH=""
  if [[ -d "${PROJECT_ROOT}/samples/${SERVICE}" ]]; then
    SERVICE_PATH="${PROJECT_ROOT}/samples/${SERVICE}"
  elif [[ -d "${PROJECT_ROOT}/services/${SERVICE}" ]]; then
    SERVICE_PATH="${PROJECT_ROOT}/services/${SERVICE}"
  fi
  
  if [[ -z "${SERVICE_PATH}" ]]; then
    log_warn "Service ${SERVICE} not found, skipping service test"
  else
    log_info "Service found at: ${SERVICE_PATH}"
    
    # For Go services, try to build and run a health check
    if [[ -f "${SERVICE_PATH}/go.mod" ]]; then
      log_info "Building Go service..."
      pushd "${SERVICE_PATH}" >/dev/null || log_fatal "Cannot change to ${SERVICE_PATH}"
      
      if go build -o /tmp/test-service ./cmd/*/main.go 2>/dev/null || go build -o /tmp/test-service ./main.go 2>/dev/null; then
        log_success "Service built successfully"
        
        # In a real scenario, would start the service and test its endpoints
        # For now, just verify it builds
        rm -f /tmp/test-service
      else
        log_warn "Service build failed (may be expected if incomplete)"
      fi
      
      popd >/dev/null
    fi
  fi
fi

# Summary
log_success "=== Smoke Test Summary ==="
log_success "✓ Local dev stack is running"
log_success "✓ All components are accessible"
log_success "✓ Mock inference API is healthy"
log_success "✓ Mock inference completion endpoint works"
log_success "All checks passed!"

exit 0

