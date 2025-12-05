#!/usr/bin/env bash
# Teardown an AI-AAS environment completely
# ==========================================
# Usage: ./scripts/infra/teardown-environment.sh <environment>
#
# This script safely destroys an environment:
#   1. Confirms destruction with user
#   2. Deletes ArgoCD applications (to clean up resources)
#   3. Runs Terraform destroy
#   4. Removes local kubeconfig
#
# Examples:
#   ./scripts/infra/teardown-environment.sh staging

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_warning() {
  echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
  echo -e "${RED}✗ $1${NC}" >&2
}

print_success() {
  echo -e "${GREEN}✓ $1${NC}"
}

load_linode_token() {
  if [[ -n "${LINODE_TOKEN:-}" ]]; then
    return 0
  fi

  local env_file="$REPO_ROOT/secrets/env/.env"
  if [[ -f "$env_file" ]]; then
    local token
    token=$(grep -E "^LINODE_TOKEN=" "$env_file" | cut -d'=' -f2 | tr -d '"' | tr -d "'" || true)
    if [[ -n "$token" ]]; then
      export LINODE_TOKEN="$token"
      return 0
    fi
  fi

  print_error "LINODE_TOKEN not found"
  exit 1
}

main() {
  local environment
  environment=$(resolve_environment "${1:-}")
  local kubeconfig_file="$REPO_ROOT/secrets/kubeconfigs/kubeconfig-${environment}.yaml"
  local tf_dir="$REPO_ROOT/infra/terraform/environments/$environment"

  echo ""
  echo -e "${RED}╔════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${RED}║       AI-AAS Environment TEARDOWN Script                   ║${NC}"
  echo -e "${RED}║                                                            ║${NC}"
  echo -e "${RED}║  Environment: $environment                                       ║${NC}"
  echo -e "${RED}║                                                            ║${NC}"
  echo -e "${RED}║  ⚠️  THIS WILL DESTROY ALL RESOURCES IN THIS ENVIRONMENT   ║${NC}"
  echo -e "${RED}╚════════════════════════════════════════════════════════════╝${NC}"
  echo ""

  # Safety check - prevent destroying production
  if [[ "$environment" == "production" ]]; then
    print_error "REFUSING to destroy production environment"
    echo "If you really need to destroy production, do it manually with terraform."
    exit 1
  fi

  # Triple confirmation
  echo -e "${YELLOW}This will destroy:${NC}"
  echo "  - LKE Kubernetes cluster (ai-aas-$environment)"
  echo "  - All workloads, persistent volumes, and data"
  echo "  - All Terraform-managed resources"
  echo ""

  read -p "Type the environment name to confirm destruction [$environment]: " confirm1
  if [[ "$confirm1" != "$environment" ]]; then
    print_warning "Aborted - environment name did not match"
    exit 0
  fi

  read -p "Are you absolutely sure? (yes/no): " confirm2
  if [[ "$confirm2" != "yes" ]]; then
    print_warning "Aborted by user"
    exit 0
  fi

  load_linode_token

  # Try to clean up ArgoCD applications first (graceful cleanup)
  if [[ -f "$kubeconfig_file" ]]; then
    export KUBECONFIG="$kubeconfig_file"
    log "Attempting graceful ArgoCD cleanup..."

    # Delete all applications (this triggers finalizers to clean up resources)
    kubectl delete applications --all -n argocd --timeout=120s 2>/dev/null || true

    # Wait a bit for cleanup
    sleep 10
    print_success "ArgoCD applications deleted"
  fi

  # Run Terraform destroy
  if [[ -d "$tf_dir" ]]; then
    cd "$tf_dir"

    log "Initializing Terraform..."
    terraform init -input=false

    log "Destroying infrastructure..."
    terraform destroy -var-file=terraform.tfvars -auto-approve

    print_success "Terraform destroy completed"
    cd - > /dev/null
  else
    print_warning "Terraform directory not found: $tf_dir"
  fi

  # Remove kubeconfig
  if [[ -f "$kubeconfig_file" ]]; then
    rm -f "$kubeconfig_file"
    print_success "Removed kubeconfig: $kubeconfig_file"
  fi

  echo ""
  print_success "Environment $environment has been destroyed"
  echo ""
  echo "To recreate this environment:"
  echo "  ./scripts/infra/provision-environment.sh $environment"
}

main "$@"
