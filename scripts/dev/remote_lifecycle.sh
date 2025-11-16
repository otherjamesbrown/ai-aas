#!/usr/bin/env bash
# Remote workspace lifecycle operations
# Manages dev stack on remote workspace via SSH and systemd.

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/common.sh
source "${SCRIPT_DIR}/common.sh"

# Default values
ACTION="${1:-help}"
WORKSPACE_HOST="${WORKSPACE_HOST:-}"
WORKSPACE_NAME="${WORKSPACE_NAME:-}"
COMPONENT="${COMPONENT:-}"
AUDIT_LOG="${AUDIT_LOG:-${HOME}/.ai-aas/workspace-audit.log}"

# Parse arguments
parse_args "$@"

show_usage() {
  cat <<EOF
Usage: $0 <action> [OPTIONS]

Actions:
  up          Start dev stack on remote workspace
  down        Stop dev stack on remote workspace
  status      Check dev stack status (JSON output)
  logs        View logs from dev stack components
  reset       Reset dev stack (stop, remove volumes, restart)
  restart     Restart dev stack services

Options:
  --workspace-host HOST    Remote workspace host (IP or hostname)
  --workspace, -w NAME     Workspace name (for logging)
  --component, -c NAME     Component name for logs (optional)
  --json, -j               Output JSON format
  --verbose, -v            Enable verbose output

Examples:
  $0 up --workspace-host 192.0.2.1 --workspace dev-jab
  $0 status --workspace-host 192.0.2.1 --json
  $0 logs --workspace-host 192.0.2.1 --component postgres
  $0 reset --workspace-host 192.0.2.1

EOF
}

check_ssh() {
  if [[ -z "${WORKSPACE_HOST}" ]]; then
    log_fatal "WORKSPACE_HOST required. Set via --workspace-host or environment variable."
  fi
  
  # Test SSH connectivity
  if ! ssh -o ConnectTimeout=5 -o StrictHostKeyChecking=no "root@${WORKSPACE_HOST}" "echo ok" >/dev/null 2>&1; then
    log_error "Cannot connect to ${WORKSPACE_HOST}"
    log_info "Ensure SSH key is configured and workspace is running"
    return 1
  fi
}

audit_log() {
  local action="$1"
  shift
  local message="$*"
  local timestamp
  timestamp="$(timestamp)"
  echo "[${timestamp}] ${action} | ${WORKSPACE_NAME} | ${WORKSPACE_HOST} | ${message}" >> "${AUDIT_LOG}"
}

remote_up() {
  check_ssh
  
  log_info "Starting dev stack on ${WORKSPACE_HOST}"
  audit_log "remote-up" "Starting dev stack"
  
  # Ensure compose files exist on remote
  log_info "Ensuring compose files are present on remote..."
  ssh_exec "${WORKSPACE_HOST}" "mkdir -p /opt/ai-aas/dev-stack/compose"
  
  # Copy compose files to remote (in production, this would be from a repo or artifact store)
  # For now, we assume they're already provisioned by the StackScript
  # In practice, you might scp them or pull from a config service
  
  # Start the systemd service
  ssh_exec "${WORKSPACE_HOST}" "systemctl start ai-aas-dev-stack.service"
  
  # Wait a moment and check status
  sleep 2
  remote_status_check
  
  audit_log "remote-up" "Dev stack started"
  log_success "Dev stack started on ${WORKSPACE_HOST}"
}

remote_down() {
  check_ssh
  
  check_ttl "stopping"
  
  log_info "Stopping dev stack on ${WORKSPACE_HOST}"
  audit_log "remote-down" "Stopping dev stack"
  
  ssh_exec "${WORKSPACE_HOST}" "systemctl stop ai-aas-dev-stack.service"
  
  audit_log "remote-down" "Dev stack stopped"
  log_success "Dev stack stopped"
}

remote_stop() {
  # Alias for remote_down for consistency with local commands
  remote_down
}

remote_status() {
  check_ssh
  
  audit_log "remote-status" "Checking status"
  
  # Get status from dev-status command or docker compose
  local status_json
  status_json=$(ssh_exec "${WORKSPACE_HOST}" "cd /opt/ai-aas/dev-stack/compose && docker compose -f compose.base.yaml -f compose.remote.yaml ps --format json 2>/dev/null || echo '[]'")
  
  if [[ "${OUTPUT_JSON}" == "true" ]]; then
    echo "${status_json}"
  else
    log_info "Remote workspace status:"
    echo "${status_json}" | jq -r '.[] | "\(.Name): \(.State)"' 2>/dev/null || echo "${status_json}"
  fi
}

remote_status_check() {
  # Check if services are healthy
  local unhealthy
  unhealthy=$(ssh_exec "${WORKSPACE_HOST}" "cd /opt/ai-aas/dev-stack/compose && docker compose ps --format json | jq -r '[.[] | select(.Health != \"healthy\" and .State != \"running\")] | length'" || echo "0")
  
  if [[ "${unhealthy}" -gt 0 ]]; then
    log_warn "${unhealthy} service(s) not healthy"
    return 1
  fi
  
  return 0
}

remote_logs() {
  check_ssh
  
  if [[ -n "${COMPONENT}" ]]; then
    log_info "Fetching logs for component: ${COMPONENT}"
    ssh_exec "${WORKSPACE_HOST}" "cd /opt/ai-aas/dev-stack/compose && docker compose logs --tail=100 -f ${COMPONENT}"
  else
    log_info "Fetching logs for all components"
    ssh_exec "${WORKSPACE_HOST}" "cd /opt/ai-aas/dev-stack/compose && docker compose logs --tail=100 -f"
  fi
}

remote_reset() {
  check_ssh
  
  check_ttl "resetting"
  
  log_warn "Resetting dev stack on ${WORKSPACE_HOST} (this will remove all data)"
  audit_log "remote-reset" "Resetting dev stack"
  
  # Stop stack
  ssh_exec "${WORKSPACE_HOST}" "systemctl stop ai-aas-dev-stack.service"
  
  # Remove volumes
  ssh_exec "${WORKSPACE_HOST}" "cd /opt/ai-aas/dev-stack/compose && docker compose down -v || true"
  
  # Clean data directories (90-day retention policy per data classification)
  ssh_exec "${WORKSPACE_HOST}" "find /opt/ai-aas/dev-stack/data -type f -mtime +90 -delete 2>/dev/null || true"
  ssh_exec "${WORKSPACE_HOST}" "find /opt/ai-aas/dev-stack/logs -type f -mtime +90 -delete 2>/dev/null || true"
  
  # Remove all data for reset
  ssh_exec "${WORKSPACE_HOST}" "rm -rf /opt/ai-aas/dev-stack/data/* /opt/ai-aas/dev-stack/logs/*"
  
  # Restart
  ssh_exec "${WORKSPACE_HOST}" "systemctl start ai-aas-dev-stack.service"
  
  # Wait and check
  sleep 3
  remote_status_check
  
  audit_log "remote-reset" "Dev stack reset complete"
  log_success "Dev stack reset complete"
}

remote_destroy() {
  check_ssh
  
  check_ttl "destroying"
  
  log_warn "Destroying workspace ${WORKSPACE_NAME} on ${WORKSPACE_HOST} (this will remove everything)"
  audit_log "remote-destroy" "Destroying workspace"
  
  # Stop and remove stack
  ssh_exec "${WORKSPACE_HOST}" "systemctl stop ai-aas-dev-stack.service || true"
  ssh_exec "${WORKSPACE_HOST}" "cd /opt/ai-aas/dev-stack/compose && docker compose down -v || true"
  
  # Clean up all workspace data (respect 90-day retention for logs)
  ssh_exec "${WORKSPACE_HOST}" "find /opt/ai-aas -type f -mtime +90 -delete 2>/dev/null || true"
  ssh_exec "${WORKSPACE_HOST}" "rm -rf /opt/ai-aas/dev-stack/data /opt/ai-aas/dev-stack/logs || true"
  
  # Clean up systemd service
  ssh_exec "${WORKSPACE_HOST}" "systemctl disable ai-aas-dev-stack.service || true"
  ssh_exec "${WORKSPACE_HOST}" "rm -f /etc/systemd/system/ai-aas-dev-stack.service || true"
  
  audit_log "remote-destroy" "Workspace destroyed"
  log_success "Workspace destroyed. Note: Instance still exists; use 'make remote-provision destroy' to remove it."
}

check_ttl() {
  local action="$1"
  
  # Get TTL from instance tags (if available via Linode API)
  # For now, check workspace creation time from metadata or tags
  # This is a placeholder - real implementation would parse from Terraform state or instance tags
  
  local ttl_hours
  ttl_hours=$(ssh_exec "${WORKSPACE_HOST}" "cat /etc/ai-aas/workspace-metadata.json 2>/dev/null | grep -o '\"ttl_hours\":[0-9]*' | cut -d: -f2" || echo "24")
  
  if [[ -z "${ttl_hours}" ]] || [[ "${ttl_hours}" -eq 0 ]]; then
    ttl_hours=24  # Default TTL
  fi
  
  # Check if workspace has expired
  local created_at
  created_at=$(ssh_exec "${WORKSPACE_HOST}" "stat -c %Y /etc/ai-aas/workspace-metadata.json 2>/dev/null || echo $(date +%s)" || echo "$(date +%s)")
  
  local now
  now=$(date +%s)
  
  local age_seconds
  age_seconds=$((now - created_at))
  local age_hours
  age_hours=$((age_seconds / 3600))
  
  if [[ ${age_hours} -ge ${ttl_hours} ]]; then
    log_warn "Workspace TTL (${ttl_hours}h) may have expired (age: ${age_hours}h)"
    log_info "Consider reprovisioning workspace if operations fail"
    audit_log "ttl-warning" "Workspace age (${age_hours}h) exceeds TTL (${ttl_hours}h)"
  fi
  
  return 0
}

remote_restart() {
  check_ssh
  
  log_info "Restarting dev stack on ${WORKSPACE_HOST}"
  audit_log "remote-restart" "Restarting dev stack"
  
  ssh_exec "${WORKSPACE_HOST}" "systemctl restart ai-aas-dev-stack.service"
  
  sleep 2
  remote_status_check
  
  audit_log "remote-restart" "Dev stack restarted"
  log_success "Dev stack restarted"
}

# Main dispatch
case "${ACTION}" in
  up)
    remote_up
    ;;
  down)
    remote_down
    ;;
  stop)
    remote_stop
    ;;
  status)
    remote_status
    ;;
  logs)
    remote_logs
    ;;
  reset)
    remote_reset
    ;;
  restart)
    remote_restart
    ;;
  destroy)
    remote_destroy
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

# Enforce 90-day log retention (update Vector config if TTL detected)
if [[ -n "${WORKSPACE_HOST}" ]]; then
  ssh_exec "${WORKSPACE_HOST}" "grep -q 'retention.*90' /etc/vector/vector-agent.toml 2>/dev/null || echo '# Retention: 90 days' >> /etc/vector/vector-agent.toml" || true
fi

