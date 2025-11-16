#!/usr/bin/env bash
# Example runner script demonstrating service configuration usage for local & remote modes
# This script shows how to configure and run a service against the dev stack.

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
# shellcheck source=scripts/dev/common.sh
source "${PROJECT_ROOT}/scripts/dev/common.sh"

# Default values
SERVICE="${1:-service-template}"
MODE="${DEV_MODE:-local}"
WORKSPACE_HOST="${WORKSPACE_HOST:-}"
ENV_FILE="${ENV_FILE:-}"

show_usage() {
  cat <<EOF
Usage: $0 [SERVICE] [OPTIONS]

Run a sample service against the local or remote dev stack.

Arguments:
  SERVICE          Service name (default: service-template)

Options:
  --mode MODE      Mode: local or remote (default: local)
  --host HOST      Remote workspace host (required for remote mode)
  --env FILE       Environment file to load (default: auto-detect)
  --verbose, -v    Enable verbose output
  --help, -h       Show this help message

Examples:
  # Run against local stack
  $0 service-template --mode local

  # Run against remote workspace
  $0 service-template --mode remote --host 192.0.2.1

  # Use custom environment file
  $0 service-template --env .env.custom

EOF
}

parse_args "$@"

# Auto-detect environment file
if [[ -z "${ENV_FILE}" ]]; then
  if [[ "${MODE}" == "remote" ]]; then
    ENV_FILE="${PROJECT_ROOT}/.env.linode"
    if [[ ! -f "${ENV_FILE}" ]]; then
      log_warn ".env.linode not found, run 'make remote-secrets' first"
      ENV_FILE="${PROJECT_ROOT}/configs/dev/service-example.env.tpl"
    fi
  else
    ENV_FILE="${PROJECT_ROOT}/.env.local"
    if [[ ! -f "${ENV_FILE}" ]]; then
      log_info ".env.local not found, using template defaults"
      ENV_FILE="${PROJECT_ROOT}/configs/dev/service-example.env.tpl"
    fi
  fi
fi

# Load environment file if it exists
if [[ -f "${ENV_FILE}" ]]; then
  log_info "Loading environment from: ${ENV_FILE}"
  load_env_file "${ENV_FILE}"
else
  log_warn "Environment file not found: ${ENV_FILE}"
fi

# Set mode
export DEV_MODE="${MODE}"

# Override connection strings based on mode
if [[ "${MODE}" == "remote" ]]; then
  if [[ -z "${WORKSPACE_HOST}" ]]; then
    log_fatal "WORKSPACE_HOST required for remote mode"
  fi
  
  log_info "Configuring for remote workspace: ${WORKSPACE_HOST}"
  
  # Override with remote host addresses
  export POSTGRES_HOST="${WORKSPACE_HOST}"
  export REDIS_ADDR="${WORKSPACE_HOST}:6379"
  export NATS_URL="nats://${WORKSPACE_HOST}:4222"
  export MINIO_ENDPOINT="http://${WORKSPACE_HOST}:9000"
  export MOCK_INFERENCE_ENDPOINT="http://${WORKSPACE_HOST}:8000"
else
  log_info "Configuring for local stack"
  
  # Use localhost defaults (already set in template)
  export POSTGRES_HOST="${POSTGRES_HOST:-localhost}"
  export REDIS_ADDR="${REDIS_ADDR:-localhost:6379}"
  export NATS_URL="${NATS_URL:-nats://localhost:4222}"
  export MINIO_ENDPOINT="${MINIO_ENDPOINT:-http://localhost:9000}"
  export MOCK_INFERENCE_ENDPOINT="${MOCK_INFERENCE_ENDPOINT:-http://localhost:8000}"
fi

# Verify dev stack is running
log_info "Verifying dev stack is healthy..."
if [[ "${MODE}" == "remote" ]]; then
  if ! "${PROJECT_ROOT}/scripts/dev/remote_lifecycle.sh" status --workspace-host "${WORKSPACE_HOST}" --json >/dev/null 2>&1; then
    log_error "Remote stack is not healthy. Run 'make remote-up' first."
    exit 1
  fi
else
  if ! "${PROJECT_ROOT}/scripts/dev/local_lifecycle.sh" status --json >/dev/null 2>&1; then
    log_error "Local stack is not running. Run 'make up' first."
    exit 1
  fi
fi

log_success "Dev stack is healthy"

# Determine service path
SERVICE_PATH=""
if [[ -d "${PROJECT_ROOT}/samples/${SERVICE}" ]]; then
  SERVICE_PATH="${PROJECT_ROOT}/samples/${SERVICE}"
elif [[ -d "${PROJECT_ROOT}/services/${SERVICE}" ]]; then
  SERVICE_PATH="${PROJECT_ROOT}/services/${SERVICE}"
else
  log_fatal "Service not found: ${SERVICE}"
fi

# Run the service
log_info "Starting service: ${SERVICE} (mode: ${MODE})"
log_info "Service path: ${SERVICE_PATH}"

pushd "${SERVICE_PATH}" >/dev/null || log_fatal "Cannot change to ${SERVICE_PATH}"

# Check for Go service
if [[ -f "go.mod" ]]; then
  log_info "Running Go service..."
  
  # Find main.go
  MAIN_GO=""
  if [[ -f "cmd/${SERVICE}/main.go" ]]; then
    MAIN_GO="cmd/${SERVICE}/main.go"
  elif [[ -f "cmd/*/main.go" ]]; then
    MAIN_GO=$(find cmd -name "main.go" | head -1)
  elif [[ -f "main.go" ]]; then
    MAIN_GO="main.go"
  fi
  
  if [[ -z "${MAIN_GO}" ]]; then
    log_fatal "Cannot find main.go for Go service"
  fi
  
  if [[ "${VERBOSE}" == "true" ]]; then
    log_info "Environment variables:"
    env | grep -E "(SERVICE_|DATABASE_|REDIS_|NATS_|MINIO_|MOCK_|OTEL_|DEV_)" | sort || true
  fi
  
  log_info "Starting: go run ${MAIN_GO}"
  go run "${MAIN_GO}"
  
# Check for TypeScript/Node service
elif [[ -f "package.json" ]]; then
  log_info "Running TypeScript/Node service..."
  
  if [[ ! -d "node_modules" ]]; then
    log_info "Installing dependencies..."
    npm install || pnpm install || yarn install
  fi
  
  if [[ "${VERBOSE}" == "true" ]]; then
    log_info "Environment variables:"
    env | grep -E "(SERVICE_|DATABASE_|REDIS_|NATS_|MINIO_|MOCK_|OTEL_|DEV_)" | sort || true
  fi
  
  log_info "Starting: npm start"
  npm start || pnpm start || yarn start

else
  log_fatal "Cannot determine service type (no go.mod or package.json found)"
fi

popd >/dev/null

