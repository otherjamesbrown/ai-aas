#!/usr/bin/env bash
# Shared helper library for development environment lifecycle scripts.
# Source this file in scripts/dev/*.sh scripts for consistent logging, argument parsing, and SSH helpers.

set -euo pipefail

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Logging functions
log_info() {
  printf "${BLUE}[INFO]${NC} %s\n" "$*" >&2
}

log_success() {
  printf "${GREEN}[SUCCESS]${NC} %s\n" "$*" >&2
}

log_warn() {
  printf "${YELLOW}[WARN]${NC} %s\n" "$*" >&2
}

log_error() {
  printf "${RED}[ERROR]${NC} %s\n" "$*" >&2
}

log_fatal() {
  log_error "$@"
  exit 1
}

# Timestamp function
timestamp() {
  date +"%Y-%m-%dT%H:%M:%S%z"
}

# Argument parsing helper
# Usage: parse_args "$@"
# Sets global variables based on flags
parse_args() {
  local args=("$@")
  local i=0
  
  while [[ $i -lt ${#args[@]} ]]; do
    case "${args[$i]}" in
      --verbose|-v)
        VERBOSE=true
        ;;
      --json|-j)
        OUTPUT_JSON=true
        ;;
      --quiet|-q)
        QUIET=true
        ;;
      --help|-h)
        show_usage
        exit 0
        ;;
      --workspace|-w)
        if [[ $((i + 1)) -lt ${#args[@]} ]]; then
          WORKSPACE_NAME="${args[$((i + 1))]}"
          i=$((i + 1))
        else
          log_fatal "--workspace requires a value"
        fi
        ;;
      --component|-c)
        if [[ $((i + 1)) -lt ${#args[@]} ]]; then
          COMPONENT="${args[$((i + 1))]}"
          i=$((i + 1))
        else
          log_fatal "--component requires a value"
        fi
        ;;
      --remote)
        MODE="remote"
        ;;
      --local)
        MODE="local"
        ;;
      *)
        log_warn "Unknown flag: ${args[$i]}"
        ;;
    esac
    i=$((i + 1))
  done
}

# Default values (can be overridden)
VERBOSE="${VERBOSE:-false}"
OUTPUT_JSON="${OUTPUT_JSON:-false}"
QUIET="${QUIET:-false}"
MODE="${MODE:-local}"
WORKSPACE_NAME="${WORKSPACE_NAME:-}"
COMPONENT="${COMPONENT:-}"

# Get project root directory
get_project_root() {
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  echo "${script_dir}"
}

PROJECT_ROOT="$(get_project_root)"

# SSH helper functions for remote workspace operations
ssh_exec() {
  local host="$1"
  shift
  local cmd="$*"
  
  if [[ -z "${host}" ]]; then
    log_fatal "SSH host required"
  fi
  
  if [[ "${VERBOSE}" == "true" ]]; then
    log_info "Executing on ${host}: ${cmd}"
  fi
  
  ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
      "${host}" "${cmd}"
}

ssh_exec_with_env() {
  local host="$1"
  shift
  local env_vars=("$@")
  shift "${#env_vars[@]}"
  local cmd="$*"
  
  local env_string=""
  for var in "${env_vars[@]}"; do
    env_string="${env_string} ${var}"
  done
  
  ssh_exec "${host}" "${env_string} ${cmd}"
}

# Check if command exists
command_exists() {
  command -v "$1" >/dev/null 2>&1
}

require_command() {
  if ! command_exists "$1"; then
    log_fatal "Required command not found: $1"
  fi
}

# Check if Docker Compose v2 is available
check_docker_compose_v2() {
  if ! command_exists docker; then
    log_fatal "Docker CLI not found"
  fi
  
  if ! docker compose version >/dev/null 2>&1; then
    log_fatal "Docker Compose v2 plugin not found. Install: https://docs.docker.com/compose/install/"
  fi
  
  if [[ "${VERBOSE}" == "true" ]]; then
    local version
    version=$(docker compose version --short 2>/dev/null || docker compose version 2>&1 | head -1)
    log_info "Docker Compose v2: ${version}"
  fi
}

# Load environment variables from file
load_env_file() {
  local env_file="$1"
  
  if [[ ! -f "${env_file}" ]]; then
    log_warn "Environment file not found: ${env_file}"
    return 1
  fi
  
  if [[ "${VERBOSE}" == "true" ]]; then
    log_info "Loading environment from: ${env_file}"
  fi
  
  # Source the file, but skip comments and empty lines
  set -a
  # shellcheck source=/dev/null
  source "${env_file}"
  set +a
}

# Redact sensitive information from output
redact_secrets() {
  local input="$1"
  
  # Apply common redaction patterns
  echo "${input}" | sed -E \
    -e 's/LINODE_TOKEN[=:][[:space:]]*[A-Za-z0-9]{20,}/LINODE_TOKEN=***REDACTED***/gi' \
    -e 's/POSTGRES_PASSWORD[=:][[:space:]]*[^[:space:]]+/POSTGRES_PASSWORD=***REDACTED***/gi' \
    -e 's/password[=:][[:space:]]*[^[:space:]]+/password=***REDACTED***/gi' \
    -e 's/(Bearer )[A-Za-z0-9\-_.]{20,}/\1***REDACTED***/gi' \
    -e 's/(X-API-Key: )[A-Za-z0-9\-_]{20,}/\1***REDACTED***/gi'
}

# Wait for service to be ready
wait_for_service() {
  local service="$1"
  local endpoint="$2"
  local max_attempts="${3:-30}"
  local interval="${4:-2}"
  
  log_info "Waiting for ${service} to be ready..."
  
  local attempt=0
  while [[ $attempt -lt $max_attempts ]]; do
    if curl -f -s "${endpoint}" >/dev/null 2>&1; then
      log_success "${service} is ready"
      return 0
    fi
    
    attempt=$((attempt + 1))
    if [[ "${VERBOSE}" == "true" ]]; then
      log_info "Attempt ${attempt}/${max_attempts}..."
    fi
    sleep "${interval}"
  done
  
  log_error "${service} did not become ready after ${max_attempts} attempts"
  return 1
}

# Get container status
get_container_status() {
  local container="$1"
  
  if command_exists docker; then
    docker ps --filter "name=${container}" --format "{{.Status}}" 2>/dev/null || echo "not running"
  else
    echo "docker not available"
  fi
}

# Print usage (should be overridden by calling script)
show_usage() {
  cat <<EOF
Usage: $(basename "${BASH_SOURCE[1]}") [OPTIONS]

Common options:
  --verbose, -v          Enable verbose output
  --json, -j             Output JSON format
  --quiet, -q            Suppress non-error output
  --help, -h             Show this help message
  --workspace, -w NAME   Specify workspace name (remote mode)
  --component, -c NAME   Specify component name
  --remote               Use remote workspace
  --local                Use local workspace (default)

EOF
}

# Export functions for use in other scripts
export -f log_info log_success log_warn log_error log_fatal
export -f timestamp parse_args get_project_root
export -f ssh_exec ssh_exec_with_env
export -f command_exists require_command check_docker_compose_v2
export -f load_env_file redact_secrets wait_for_service
export -f get_container_status show_usage

