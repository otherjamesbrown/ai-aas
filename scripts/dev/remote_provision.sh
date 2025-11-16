#!/usr/bin/env bash
# Remote workspace provisioning wrapper
# Wraps Terraform operations for dev workspace lifecycle with audit logging.

set -euo pipefail

# Source common helper library
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/dev/common.sh
source "${SCRIPT_DIR}/common.sh"

# Default values
ACTION="${1:-help}"
WORKSPACE_NAME="${WORKSPACE_NAME:-}"
WORKSPACE_OWNER="${WORKSPACE_OWNER:-}"
TERRAFORM_DIR="${TERRAFORM_DIR:-infra/terraform/environments/development}"
AUDIT_LOG="${AUDIT_LOG:-${HOME}/.ai-aas/workspace-audit.log}"

# Ensure audit log directory exists
mkdir -p "$(dirname "${AUDIT_LOG}")"

# Audit logging function
audit_log() {
  local action="$1"
  shift
  local message="$*"
  local timestamp
  timestamp="$(timestamp)"
  echo "[${timestamp}] ${action} | ${WORKSPACE_NAME} | ${WORKSPACE_OWNER} | ${message}" >> "${AUDIT_LOG}"
}

# Parse arguments
parse_args "$@"

show_usage() {
  cat <<EOF
Usage: $0 <action> [OPTIONS]

Actions:
  init          Initialize Terraform working directory
  plan          Show Terraform execution plan
  apply         Provision workspace
  destroy       Destroy workspace
  status        Show workspace status
  outputs       Show Terraform outputs

Options:
  --workspace, -w NAME    Workspace name (required for apply/destroy)
  --owner, -o NAME        Workspace owner (required for apply/destroy)
  --terraform-dir DIR     Terraform directory (default: infra/terraform/environments/development)
  --verbose, -v           Enable verbose output

Examples:
  $0 init
  $0 plan --workspace dev-jab --owner jab
  $0 apply --workspace dev-jab --owner jab
  $0 destroy --workspace dev-jab --owner jab
  $0 status --workspace dev-jab
  $0 outputs --workspace dev-jab

EOF
}

check_terraform() {
  require_command terraform
  if ! terraform version >/dev/null 2>&1; then
    log_fatal "Terraform not available"
  fi
}

check_linode_token() {
  if [[ -z "${LINODE_TOKEN:-}" ]]; then
    log_error "LINODE_TOKEN environment variable not set"
    log_info "Set LINODE_TOKEN before running provisioning commands"
    log_info "See docs/platform/linode-access.md for details"
    return 1
  fi
}

init_terraform() {
  log_info "Initializing Terraform in ${TERRAFORM_DIR}"
  pushd "${PROJECT_ROOT}/${TERRAFORM_DIR}" >/dev/null || log_fatal "Cannot change to ${TERRAFORM_DIR}"
  
  audit_log "init" "Initializing Terraform workspace"
  
  terraform init \
    -backend-config="${PROJECT_ROOT}/infra/terraform/backend.hcl" \
    -reconfigure
  
  popd >/dev/null
  log_success "Terraform initialized"
}

plan_workspace() {
  if [[ -z "${WORKSPACE_NAME}" || -z "${WORKSPACE_OWNER}" ]]; then
    log_fatal "WORKSPACE_NAME and WORKSPACE_OWNER required for plan/apply"
  fi
  
  check_terraform
  check_linode_token
  
  log_info "Planning workspace: ${WORKSPACE_NAME} (owner: ${WORKSPACE_OWNER})"
  pushd "${PROJECT_ROOT}/${TERRAFORM_DIR}" >/dev/null || log_fatal "Cannot change to ${TERRAFORM_DIR}"
  
  audit_log "plan" "Planning workspace provisioning"
  
  terraform plan \
    -var="enable_dev_workspace=true" \
    -var="workspace_name=${WORKSPACE_NAME}" \
    -var="workspace_owner=${WORKSPACE_OWNER}" \
    -var="workspace_ttl_hours=24" \
    -out=tfplan \
    -detailed-exitcode
  
  popd >/dev/null
}

apply_workspace() {
  if [[ -z "${WORKSPACE_NAME}" || -z "${WORKSPACE_OWNER}" ]]; then
    log_fatal "WORKSPACE_NAME and WORKSPACE_OWNER required for apply"
  fi
  
  check_terraform
  check_linode_token
  
  log_info "Provisioning workspace: ${WORKSPACE_NAME} (owner: ${WORKSPACE_OWNER})"
  pushd "${PROJECT_ROOT}/${TERRAFORM_DIR}" >/dev/null || log_fatal "Cannot change to ${TERRAFORM_DIR}"
  
  audit_log "apply" "Starting workspace provisioning"
  
  terraform apply \
    -var="enable_dev_workspace=true" \
    -var="workspace_name=${WORKSPACE_NAME}" \
    -var="workspace_owner=${WORKSPACE_OWNER}" \
    -var="workspace_ttl_hours=24" \
    -auto-approve
  
  # Capture outputs
  local instance_id
  instance_id="$(terraform output -raw dev_workspace[0].instance_id 2>/dev/null || echo "unknown")"
  local public_ip
  public_ip="$(terraform output -raw dev_workspace[0].public_ip 2>/dev/null || echo "unknown")"
  
  audit_log "apply" "Workspace provisioned | instance_id=${instance_id} | ip=${public_ip}"
  
  log_success "Workspace provisioned"
  log_info "Instance ID: ${instance_id}"
  log_info "Public IP: ${public_ip}"
  log_info "SSH: ssh root@${public_ip}"
  
  popd >/dev/null
}

destroy_workspace() {
  if [[ -z "${WORKSPACE_NAME}" || -z "${WORKSPACE_OWNER}" ]]; then
    log_fatal "WORKSPACE_NAME and WORKSPACE_OWNER required for destroy"
  fi
  
  check_terraform
  check_linode_token
  
  log_warn "Destroying workspace: ${WORKSPACE_NAME}"
  pushd "${PROJECT_ROOT}/${TERRAFORM_DIR}" >/dev/null || log_fatal "Cannot change to ${TERRAFORM_DIR}"
  
  audit_log "destroy" "Starting workspace destruction"
  
  terraform destroy \
    -var="enable_dev_workspace=true" \
    -var="workspace_name=${WORKSPACE_NAME}" \
    -var="workspace_owner=${WORKSPACE_OWNER}" \
    -auto-approve
  
  audit_log "destroy" "Workspace destroyed"
  
  log_success "Workspace destroyed"
  popd >/dev/null
}

show_status() {
  if [[ -z "${WORKSPACE_NAME}" ]]; then
    log_fatal "WORKSPACE_NAME required for status"
  fi
  
  check_terraform
  
  pushd "${PROJECT_ROOT}/${TERRAFORM_DIR}" >/dev/null || log_fatal "Cannot change to ${TERRAFORM_DIR}"
  
  if [[ "${OUTPUT_JSON}" == "true" ]]; then
    terraform output -json 2>/dev/null | jq -r '.dev_workspace[0].value // empty' || echo '{}'
  else
    terraform output 2>/dev/null || log_warn "No outputs found (workspace may not be provisioned)"
  fi
  
  popd >/dev/null
}

show_outputs() {
  show_status
}

# Main dispatch
case "${ACTION}" in
  init)
    init_terraform
    ;;
  plan)
    plan_workspace
    ;;
  apply)
    apply_workspace
    ;;
  destroy)
    destroy_workspace
    ;;
  status)
    show_status
    ;;
  outputs)
    show_outputs
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

