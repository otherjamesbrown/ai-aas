#!/usr/bin/env bash
# Local development stack lifecycle operations
# Manages dev stack on local machine via Docker Compose v2.

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/common.sh
source "${SCRIPT_DIR}/common.sh"

# Default values
ACTION="${1:-help}"
COMPONENT="${COMPONENT:-}"
COMPOSE_DIR="${PROJECT_ROOT}/.dev/compose"

# Parse arguments (skip first argument which is the action)
if [[ $# -gt 1 ]]; then
  parse_args "${@:2}"
fi

show_usage() {
  cat <<EOF
Usage: $0 <action> [OPTIONS]

Actions:
  up          Start dev stack locally
  down        Stop dev stack
  status      Check dev stack status (JSON output)
  logs        View logs from dev stack components
  reset       Reset dev stack (stop, remove volumes, restart)
  restart     Restart dev stack services
  diagnose    Show diagnostic information (port conflicts, etc.)

Options:
  --component, -c NAME     Component name for logs (optional)
  --json, -j               Output JSON format
  --verbose, -v            Enable verbose output

Examples:
  $0 up
  $0 status --json
  $0 logs --component postgres
  $0 reset

EOF
}

check_docker_compose_v2() {
  if ! docker compose version >/dev/null 2>&1; then
    log_fatal "Docker Compose v2 required"
  fi
}

check_compose_files() {
  if [[ ! -f "${COMPOSE_DIR}/compose.base.yaml" ]]; then
    log_fatal "Compose base file not found: ${COMPOSE_DIR}/compose.base.yaml"
  fi
  if [[ ! -f "${COMPOSE_DIR}/compose.local.yaml" ]]; then
    log_fatal "Compose local override not found: ${COMPOSE_DIR}/compose.local.yaml"
  fi
}

local_up() {
  check_docker_compose_v2
  check_compose_files
  
  log_info "Starting local dev stack..."
  
  # Create network if it doesn't exist
  if ! docker network inspect ai-aas-dev-network >/dev/null 2>&1; then
    log_info "Creating dev network..."
    docker network create ai-aas-dev-network || true
  fi
  
  # Start services
  pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
  
  docker compose \
    -f compose.base.yaml \
    -f compose.local.yaml \
    up -d
  
  popd >/dev/null
  
  log_info "Waiting for services to be ready..."
  sleep 5
  
  # Check health
  local_health_check
  
  log_success "Local dev stack started"
}

local_down() {
  check_docker_compose_v2
  check_compose_files
  
  log_info "Stopping local dev stack..."
  
  pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
  
  docker compose \
    -f compose.base.yaml \
    -f compose.local.yaml \
    down
  
  popd >/dev/null
  
  log_success "Local dev stack stopped"
}

local_status() {
  check_docker_compose_v2
  check_compose_files
  
  pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
  
  local status_json
  status_json=$(docker compose \
    -f compose.base.yaml \
    -f compose.local.yaml \
    ps --format json 2>/dev/null || echo '[]')
  
  popd >/dev/null
  
  if [[ "${OUTPUT_JSON}" == "true" || "${jsonOutput}" == "true" ]]; then
    echo "${status_json}"
  else
    log_info "Local dev stack status:"
    echo "${status_json}" | jq -r '.[] | "\(.Name): \(.State) \(if .Health then "(\(.Health))" else "" end)"' 2>/dev/null || echo "${status_json}"
  fi
}

local_health_check() {
  # Wait for services to be healthy
  local max_attempts=30
  local attempt=0
  local unhealthy=0
  
  while [[ $attempt -lt $max_attempts ]]; do
    pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
    
    unhealthy=$(docker compose \
      -f compose.base.yaml \
      -f compose.local.yaml \
      ps --format json | jq -r '[.[] | select(.Health != "healthy" and .State == "running")] | length' 2>/dev/null || echo "0")
    
    popd >/dev/null
    
    if [[ ${unhealthy} -eq 0 ]]; then
      return 0
    fi
    
    attempt=$((attempt + 1))
    if [[ "${VERBOSE}" == "true" ]]; then
      log_info "Waiting for services to be healthy... (attempt ${attempt}/${max_attempts})"
    fi
    sleep 2
  done
  
  log_warn "Some services may not be healthy yet"
  return 1
}

local_logs() {
  check_docker_compose_v2
  check_compose_files
  
  pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
  
  if [[ -n "${COMPONENT}" ]]; then
    log_info "Fetching logs for component: ${COMPONENT}"
    docker compose \
      -f compose.base.yaml \
      -f compose.local.yaml \
      logs --tail=100 -f "${COMPONENT}"
  else
    log_info "Fetching logs for all components"
    docker compose \
      -f compose.base.yaml \
      -f compose.local.yaml \
      logs --tail=100 -f
  fi
  
  popd >/dev/null
}

local_reset() {
  check_docker_compose_v2
  check_compose_files
  
  log_warn "Resetting local dev stack (this will remove all data)"
  
  # Stop and remove volumes
  pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
  
  docker compose \
    -f compose.base.yaml \
    -f compose.local.yaml \
    down -v
  
  popd >/dev/null
  
  # Seed sample data if available
  seed_local_data
  
  # Restart
  local_up
  
  log_success "Local dev stack reset complete"
}

local_restart() {
  check_docker_compose_v2
  check_compose_files
  
  log_info "Restarting local dev stack..."
  
  pushd "${COMPOSE_DIR}" >/dev/null || log_fatal "Cannot change to ${COMPOSE_DIR}"
  
  docker compose \
    -f compose.base.yaml \
    -f compose.local.yaml \
    restart
  
  popd >/dev/null
  
  sleep 2
  local_health_check
  
  log_success "Local dev stack restarted"
}

seed_local_data() {
  local seed_file="${PROJECT_ROOT}/.dev/data/seed.sql"
  
  if [[ ! -f "${seed_file}" ]]; then
    log_info "No seed file found at ${seed_file}, skipping data seeding"
    return 0
  fi
  
  log_info "Waiting for PostgreSQL to be ready for seeding..."
  
  # Wait for PostgreSQL
  local postgres_port="${POSTGRES_PORT:-5432}"
  local max_attempts=30
  local attempt=0
  
  while [[ $attempt -lt $max_attempts ]]; do
    if nc -z localhost "${postgres_port}" 2>/dev/null; then
      break
    fi
    attempt=$((attempt + 1))
    sleep 1
  done
  
  if [[ $attempt -eq $max_attempts ]]; then
    log_warn "PostgreSQL not ready, skipping seed"
    return 1
  fi
  
  log_info "Seeding sample data..."
  
  local dsn="postgres://postgres:postgres@localhost:${postgres_port}/ai_aas?sslmode=disable"
  
  # Use docker exec if postgres container is running, otherwise use psql directly
  if docker ps --format "{{.Names}}" | grep -q "^dev-postgres$"; then
    docker exec -i dev-postgres psql -U postgres -d ai_aas < "${seed_file}" || {
      log_warn "Seed failed (tables may not exist yet)"
      return 1
    }
  elif command_exists psql; then
    PGPASSWORD=postgres psql -h localhost -p "${postgres_port}" -U postgres -d ai_aas -f "${seed_file}" || {
      log_warn "Seed failed (tables may not exist yet)"
      return 1
    }
  else
    log_warn "Cannot seed data: PostgreSQL client not available"
    return 1
  fi
  
  log_success "Sample data seeded"
}

local_diagnose() {
  log_info "Diagnosing local dev stack..."
  
  # Check Docker
  if ! command_exists docker; then
    log_error "Docker not found"
    return 1
  fi
  
  # Check Docker Compose v2
  if ! docker compose version >/dev/null 2>&1; then
    log_error "Docker Compose v2 not available"
    return 1
  fi
  
  # Check port conflicts
  log_info "Checking for port conflicts..."
  local ports=("5432:postgres" "6379:redis" "4222:nats" "8222:nats-http" "9000:minio-api" "9001:minio-console" "8000:mock-inference")
  local conflicts=0
  
  for port_info in "${ports[@]}"; do
    IFS=':' read -r port service <<<"${port_info}"
    if command_exists nc && nc -z localhost "${port}" 2>/dev/null; then
      if ! docker ps --format "{{.Ports}}" | grep -q ":${port}"; then
        log_warn "Port ${port} (${service}) is in use by non-Docker process"
        conflicts=$((conflicts + 1))
      fi
    fi
  done
  
  if [[ ${conflicts} -gt 0 ]]; then
    log_warn "${conflicts} port conflict(s) detected"
    log_info ""
    log_info "Port Conflict Remediation:"
    log_info "  1. Identify the conflicting process:"
    log_info "     sudo lsof -i :<port>  # or: sudo netstat -tlnp | grep :<port>"
    log_info ""
    log_info "  2. Stop the conflicting service (if safe to do so):"
    log_info "     sudo systemctl stop <service>  # or kill the process"
    log_info ""
    log_info "  3. Override ports via environment variables:"
    log_info "     export POSTGRES_PORT=5433"
    log_info "     export REDIS_PORT=6380"
    log_info "     export NATS_CLIENT_PORT=4223"
    log_info "     export MINIO_API_PORT=9002"
    log_info "     export MOCK_INFERENCE_PORT=8001"
    log_info "     make up"
    log_info ""
    log_info "  4. Or update .specify/local/ports.yaml and restart stack"
    log_info ""
    log_info "See docs/runbooks/service-dev-connect.md for more details"
    return 1
  else
    log_success "No port conflicts detected"
  fi
  
  # Check compose files
  if [[ ! -f "${COMPOSE_DIR}/compose.base.yaml" ]]; then
    log_error "Compose base file not found"
    return 1
  fi
  
  if [[ ! -f "${COMPOSE_DIR}/compose.local.yaml" ]]; then
    log_error "Compose local override not found"
    return 1
  fi
  
  log_success "Compose files present"
  
  # Check network
  if ! docker network inspect ai-aas-dev-network >/dev/null 2>&1; then
    log_warn "Dev network not created (will be created on 'up')"
  else
    log_success "Dev network exists"
  fi
  
  return 0
}

# Main dispatch
case "${ACTION}" in
  up)
    local_up
    ;;
  down|stop)
    local_down
    ;;
  status)
    local_status
    ;;
  logs)
    local_logs
    ;;
  reset)
    local_reset
    ;;
  restart)
    local_restart
    ;;
  diagnose)
    local_diagnose
    ;;
  seed-data)
    seed_local_data
    ;;
  help|--help|-h)
    show_usage
    exit 0
    ;;
  *)
    log_error "Unknown action: ${ACTION}"
    show_usage
    exit 1
    ;;
esac

