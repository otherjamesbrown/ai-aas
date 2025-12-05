#!/usr/bin/env bash
# Provision a complete AI-AAS environment from scratch
# =====================================================
# Usage: ./scripts/infra/provision-environment.sh <environment>
#
# This script automates the full provisioning process:
#   1. Validates prerequisites (terraform, kubectl, helm)
#   2. Runs Terraform to create LKE cluster
#   3. Saves kubeconfig to secrets/kubeconfigs/
#   4. Installs ArgoCD
#   5. Applies GPU node labels (if GPU nodes exist)
#   6. Bootstraps the ArgoCD applications
#
# Prerequisites:
#   - Terraform >= 1.0
#   - kubectl
#   - helm
#   - LINODE_TOKEN environment variable or in secrets/env/.env
#
# Examples:
#   ./scripts/infra/provision-environment.sh staging
#   ENV=staging ./scripts/infra/provision-environment.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

# Configuration
ARGOCD_VERSION="${ARGOCD_VERSION:-stable}"
ARGOCD_NAMESPACE="argocd"
WAIT_TIMEOUT="${WAIT_TIMEOUT:-300}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_step() {
  echo -e "\n${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
  echo -e "${BLUE}▶ $1${NC}"
  echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}\n"
}

print_success() {
  echo -e "${GREEN}✓ $1${NC}"
}

print_warning() {
  echo -e "${YELLOW}⚠ $1${NC}"
}

print_error() {
  echo -e "${RED}✗ $1${NC}" >&2
}

check_prerequisites() {
  print_step "Checking prerequisites"

  local missing=()

  # Check terraform
  if ! command -v terraform &> /dev/null; then
    missing+=("terraform")
  else
    print_success "terraform $(terraform version -json | jq -r '.terraform_version')"
  fi

  # Check kubectl
  if ! command -v kubectl &> /dev/null; then
    missing+=("kubectl")
  else
    print_success "kubectl $(kubectl version --client -o json 2>/dev/null | jq -r '.clientVersion.gitVersion')"
  fi

  # Check helm
  if ! command -v helm &> /dev/null; then
    missing+=("helm")
  else
    print_success "helm $(helm version --short)"
  fi

  # Check jq
  if ! command -v jq &> /dev/null; then
    missing+=("jq")
  else
    print_success "jq $(jq --version)"
  fi

  if [[ ${#missing[@]} -gt 0 ]]; then
    print_error "Missing required tools: ${missing[*]}"
    echo ""
    echo "Install missing tools:"
    for tool in "${missing[@]}"; do
      case $tool in
        terraform)
          echo "  terraform: https://developer.hashicorp.com/terraform/install"
          ;;
        kubectl)
          echo "  kubectl: https://kubernetes.io/docs/tasks/tools/"
          ;;
        helm)
          echo "  helm: https://helm.sh/docs/intro/install/"
          ;;
        jq)
          echo "  jq: apt install jq / brew install jq"
          ;;
      esac
    done
    exit 1
  fi
}

load_linode_token() {
  print_step "Loading Linode credentials"

  if [[ -n "${LINODE_TOKEN:-}" ]]; then
    print_success "LINODE_TOKEN found in environment"
    return 0
  fi

  local env_file="$REPO_ROOT/secrets/env/.env"
  if [[ -f "$env_file" ]]; then
    local token
    token=$(grep -E "^LINODE_TOKEN=" "$env_file" | cut -d'=' -f2 | tr -d '"' | tr -d "'" || true)
    if [[ -n "$token" ]]; then
      export LINODE_TOKEN="$token"
      print_success "LINODE_TOKEN loaded from secrets/env/.env"
      return 0
    fi
  fi

  print_error "LINODE_TOKEN not found. Set it in environment or secrets/env/.env"
  exit 1
}

run_terraform() {
  local environment="$1"
  local tf_dir="$REPO_ROOT/infra/terraform/environments/$environment"

  print_step "Running Terraform for $environment"

  if [[ ! -d "$tf_dir" ]]; then
    print_error "Terraform directory not found: $tf_dir"
    exit 1
  fi

  cd "$tf_dir"

  # Initialize
  log "Initializing Terraform..."
  terraform init -input=false

  # Plan
  log "Planning infrastructure changes..."
  terraform plan -var-file=terraform.tfvars -out=tfplan -input=false

  # Prompt for confirmation
  echo ""
  read -p "Apply this plan? (yes/no): " confirm
  if [[ "$confirm" != "yes" ]]; then
    print_warning "Aborted by user"
    exit 0
  fi

  # Apply
  log "Applying infrastructure changes..."
  terraform apply -input=false tfplan

  print_success "Terraform apply completed"

  cd - > /dev/null
}

save_kubeconfig() {
  local environment="$1"
  local tf_dir="$REPO_ROOT/infra/terraform/environments/$environment"
  local kubeconfig_dir="$REPO_ROOT/secrets/kubeconfigs"
  local kubeconfig_file="$kubeconfig_dir/kubeconfig-${environment}.yaml"

  print_step "Saving kubeconfig"

  mkdir -p "$kubeconfig_dir"

  cd "$tf_dir"

  # Get kubeconfig from Terraform output
  if terraform output -raw kubeconfig > "$kubeconfig_file" 2>/dev/null; then
    chmod 600 "$kubeconfig_file"
    print_success "Kubeconfig saved to $kubeconfig_file"
  else
    # Try to get from Linode API directly
    print_warning "Could not get kubeconfig from Terraform output, trying Linode API..."
    local cluster_id
    cluster_id=$(terraform output -raw cluster_id 2>/dev/null || true)
    if [[ -n "$cluster_id" ]]; then
      curl -s -H "Authorization: Bearer $LINODE_TOKEN" \
        "https://api.linode.com/v4/lke/clusters/${cluster_id}/kubeconfig" | \
        jq -r '.kubeconfig' | base64 -d > "$kubeconfig_file"
      chmod 600 "$kubeconfig_file"
      print_success "Kubeconfig saved from Linode API"
    else
      print_error "Could not retrieve kubeconfig"
      exit 1
    fi
  fi

  cd - > /dev/null

  export KUBECONFIG="$kubeconfig_file"
  echo "export KUBECONFIG=$kubeconfig_file"
}

wait_for_nodes() {
  print_step "Waiting for nodes to be ready"

  local timeout=$WAIT_TIMEOUT
  local interval=10
  local elapsed=0

  while [[ $elapsed -lt $timeout ]]; do
    local ready_nodes
    ready_nodes=$(kubectl get nodes --no-headers 2>/dev/null | grep -c " Ready" || echo "0")
    local total_nodes
    total_nodes=$(kubectl get nodes --no-headers 2>/dev/null | wc -l || echo "0")

    if [[ $ready_nodes -gt 0 && $ready_nodes -eq $total_nodes ]]; then
      print_success "All $ready_nodes nodes are ready"
      kubectl get nodes
      return 0
    fi

    log "Waiting for nodes... ($ready_nodes/$total_nodes ready, ${elapsed}s elapsed)"
    sleep $interval
    elapsed=$((elapsed + interval))
  done

  print_error "Timeout waiting for nodes after ${timeout}s"
  kubectl get nodes
  exit 1
}

install_argocd() {
  print_step "Installing ArgoCD"

  # Create namespace
  kubectl create namespace $ARGOCD_NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

  # Install ArgoCD
  log "Deploying ArgoCD ($ARGOCD_VERSION)..."
  kubectl apply -n $ARGOCD_NAMESPACE \
    -f "https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"

  # Wait for ArgoCD to be ready
  log "Waiting for ArgoCD server to be ready..."
  kubectl wait --for=condition=available --timeout=300s \
    deployment/argocd-server -n $ARGOCD_NAMESPACE

  # Get initial admin password
  local admin_password
  admin_password=$(kubectl -n $ARGOCD_NAMESPACE get secret argocd-initial-admin-secret \
    -o jsonpath="{.data.password}" 2>/dev/null | base64 -d || echo "")

  if [[ -n "$admin_password" ]]; then
    print_success "ArgoCD installed successfully"
    echo ""
    echo "ArgoCD Admin Credentials:"
    echo "  Username: admin"
    echo "  Password: $admin_password"
    echo ""
    echo "Access ArgoCD:"
    echo "  kubectl port-forward svc/argocd-server -n argocd 8080:443"
    echo "  Open: https://localhost:8080"
  else
    print_warning "ArgoCD installed but could not retrieve admin password"
  fi
}

apply_gpu_labels() {
  print_step "Applying GPU node labels"

  "$SCRIPT_DIR/apply-gpu-node-labels.sh" "$KUBECONFIG"
}

bootstrap_applications() {
  local environment="$1"

  print_step "Bootstrapping ArgoCD applications"

  # Apply the project first
  local project_file="$REPO_ROOT/gitops/clusters/$environment/projects/platform-project.yaml"
  if [[ -f "$project_file" ]]; then
    log "Applying ArgoCD project..."
    kubectl apply -f "$project_file"
    print_success "Applied platform-$environment project"
  fi

  # Apply all applications
  local apps_dir="$REPO_ROOT/gitops/clusters/$environment/apps"
  if [[ -d "$apps_dir" ]]; then
    log "Applying ArgoCD applications..."
    kubectl apply -f "$apps_dir/" --recursive
    print_success "Applied applications from $apps_dir"
  else
    print_warning "No applications directory found: $apps_dir"
  fi

  # List applications
  echo ""
  log "ArgoCD Applications:"
  kubectl get applications -n argocd 2>/dev/null || true
}

print_summary() {
  local environment="$1"

  print_step "Provisioning Complete"

  echo "Environment: $environment"
  echo "Kubeconfig:  $KUBECONFIG"
  echo ""
  echo "Cluster Status:"
  kubectl get nodes -o wide
  echo ""
  echo "ArgoCD Applications:"
  kubectl get applications -n argocd 2>/dev/null || echo "  (none yet)"
  echo ""
  echo "Next Steps:"
  echo "  1. Push changes to the '$environment' branch to trigger ArgoCD sync"
  echo "  2. Access ArgoCD: kubectl port-forward svc/argocd-server -n argocd 8080:443"
  echo "  3. Monitor GPU nodes: kubectl get nodes -l node-type=gpu"
  echo "  4. Check GPU operator: kubectl get pods -n gpu-operator"
}

main() {
  local environment
  environment=$(resolve_environment "${1:-}")

  echo ""
  echo "╔════════════════════════════════════════════════════════════╗"
  echo "║       AI-AAS Environment Provisioning Script               ║"
  echo "║                                                            ║"
  echo "║  Environment: $environment                                       ║"
  echo "╚════════════════════════════════════════════════════════════╝"
  echo ""

  check_prerequisites
  load_linode_token
  run_terraform "$environment"
  save_kubeconfig "$environment"
  wait_for_nodes
  install_argocd
  apply_gpu_labels
  bootstrap_applications "$environment"
  print_summary "$environment"
}

# Run main with all arguments
main "$@"
