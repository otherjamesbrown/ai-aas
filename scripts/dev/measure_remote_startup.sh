#!/usr/bin/env bash
# Measure remote workspace startup latency
# Verifies that `make remote-up` completes in < 5 minutes and `make remote-status --json` in < 10 seconds
# Used in CI to validate SLA compliance

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/common.sh
# shellcheck disable=SC1091
source "${SCRIPT_DIR}/common.sh"

WORKSPACE_HOST="${WORKSPACE_HOST:-}"
WORKSPACE_NAME="${WORKSPACE_NAME:-}"
STARTUP_TIMEOUT="${STARTUP_TIMEOUT:-300}"  # 5 minutes
STATUS_TIMEOUT="${STATUS_TIMEOUT:-10}"     # 10 seconds

show_usage() {
  cat <<EOF
Usage: $0 [OPTIONS]

Measures remote workspace startup and status check latency.
Verifies SLA compliance: startup < 5 minutes, status < 10 seconds.

Options:
  --workspace-host HOST    Remote workspace host (required)
  --workspace, -w NAME     Workspace name (for logging)
  --startup-timeout SEC    Startup timeout in seconds (default: 300)
  --status-timeout SEC     Status check timeout in seconds (default: 10)
  --verbose, -v            Enable verbose output

Environment Variables:
  WORKSPACE_HOST           Remote workspace host
  WORKSPACE_NAME           Workspace name

Exit Codes:
  0   All checks passed
  1   Startup timeout exceeded
  2   Status check timeout exceeded
  3   Workspace not healthy after startup

EOF
}

parse_args "$@"

if [[ -z "${WORKSPACE_HOST}" ]]; then
  log_error "WORKSPACE_HOST required"
  show_usage
  exit 1
fi

# Measure startup time
log_info "Measuring remote workspace startup time..."
log_info "Workspace: ${WORKSPACE_NAME:-unknown} (${WORKSPACE_HOST})"
log_info "Startup timeout: ${STARTUP_TIMEOUT}s"
log_info "Status timeout: ${STATUS_TIMEOUT}s"

start_time=$(date +%s)
log_info "Starting dev stack at $(date)"

# Start the stack
if ! "${SCRIPT_DIR}/remote_lifecycle.sh" up --workspace-host "${WORKSPACE_HOST}" --workspace "${WORKSPACE_NAME}"; then
  log_error "Failed to start dev stack"
  exit 1
fi

startup_elapsed=$(($(date +%s) - start_time))
log_info "Startup completed in ${startup_elapsed}s"

# Check if startup exceeded timeout
if [[ ${startup_elapsed} -gt ${STARTUP_TIMEOUT} ]]; then
  log_error "Startup time ${startup_elapsed}s exceeds timeout ${STARTUP_TIMEOUT}s"
  exit 1
fi

log_success "Startup completed within timeout (${startup_elapsed}s < ${STARTUP_TIMEOUT}s)"

# Measure status check time
log_info "Measuring status check latency..."
status_start=$(date +%s)

# Run status check
if ! "${SCRIPT_DIR}/remote_lifecycle.sh" status --workspace-host "${WORKSPACE_HOST}" --workspace "${WORKSPACE_NAME}" --json >/dev/null 2>&1; then
  log_error "Status check failed"
  exit 3
fi

status_elapsed=$(($(date +%s) - status_start))
log_info "Status check completed in ${status_elapsed}s"

# Check if status check exceeded timeout
if [[ ${status_elapsed} -gt ${STATUS_TIMEOUT} ]]; then
  log_error "Status check time ${status_elapsed}s exceeds timeout ${STATUS_TIMEOUT}s"
  exit 2
fi

log_success "Status check completed within timeout (${status_elapsed}s < ${STATUS_TIMEOUT}s)"

# Verify workspace is healthy
log_info "Verifying workspace health..."
if ! "${SCRIPT_DIR}/remote_lifecycle.sh" status --workspace-host "${WORKSPACE_HOST}" --workspace "${WORKSPACE_NAME}" --json | jq -e '.overall == "healthy"' >/dev/null 2>&1; then
  log_error "Workspace is not healthy"
  "${SCRIPT_DIR}/remote_lifecycle.sh" status --workspace-host "${WORKSPACE_HOST}" --workspace "${WORKSPACE_NAME}" --json
  exit 3
fi

log_success "Workspace is healthy"

# Summary
log_success "=== Measurement Summary ==="
log_info "Startup time: ${startup_elapsed}s (limit: ${STARTUP_TIMEOUT}s)"
log_info "Status check: ${status_elapsed}s (limit: ${STATUS_TIMEOUT}s)"
log_info "Overall: PASSED"

exit 0

