#!/bin/bash
# Access Argo CD Web UI for Remote Dev Cluster
# Usage: 
#   ./scripts/access-argocd-ui.sh [development|production] [kubeconfig-path]
#   KUBECONFIG=/path/to/kubeconfig.yaml ./scripts/access-argocd-ui.sh [development|production]
#   ./scripts/access-argocd-ui.sh development <(op read "op://vault/kubeconfig-dev")

set -euo pipefail

ENVIRONMENT="${1:-development}"
KUBECONFIG_DIR="${HOME}/kubeconfigs"
TEMP_KUBECONFIG=""

# Determine context based on environment
if [[ "$ENVIRONMENT" == "development" ]]; then
  DEFAULT_KUBECONFIG="${KUBECONFIG_DIR}/kubeconfig-development.yaml"
  CONTEXT="lke531921-ctx"
  CLUSTER_ID="531921"
elif [[ "$ENVIRONMENT" == "production" ]]; then
  DEFAULT_KUBECONFIG="${KUBECONFIG_DIR}/kubeconfig-production.yaml"
  CONTEXT="lke531922-ctx"
  CLUSTER_ID="531922"
else
  echo "Error: Environment must be 'development' or 'production'"
  exit 1
fi

# Check for kubeconfig in environment variable first, then argument, then default location
if [[ -n "${KUBECONFIG:-}" ]] && [[ -f "$KUBECONFIG" ]]; then
  KUBECONFIG_FILE="$KUBECONFIG"
elif [[ -n "${2:-}" ]]; then
  KUBECONFIG_FILE="$2"
else
  KUBECONFIG_FILE="$DEFAULT_KUBECONFIG"
fi

# Handle special case: reading from stdin/pipe (e.g., from 1Password CLI)
if [[ "$KUBECONFIG_FILE" == "/dev/stdin" ]] || [[ "$KUBECONFIG_FILE" == "-" ]] || [[ ! -f "$KUBECONFIG_FILE" ]] && [[ -p /dev/stdin ]]; then
  TEMP_KUBECONFIG=$(mktemp /tmp/kubeconfig-XXXXXX.yaml)
  cat > "$TEMP_KUBECONFIG"
  KUBECONFIG_FILE="$TEMP_KUBECONFIG"
  trap "rm -f $TEMP_KUBECONFIG" EXIT INT TERM
fi

# Check if kubeconfig exists
if [[ ! -f "$KUBECONFIG_FILE" ]]; then
  echo "❌ Error: Kubeconfig not found at $KUBECONFIG_FILE"
  echo ""
  echo "📋 Options to provide kubeconfig:"
  echo ""
  echo "Option 1: Use 1Password CLI (Recommended - no local storage)"
  echo "  # Install 1Password CLI: https://developer.1password.com/docs/cli"
  echo "  op read \"op://vault/kubeconfig-${ENVIRONMENT}\" | ./scripts/access-argocd-ui.sh ${ENVIRONMENT} -"
  echo ""
  echo "Option 2: Use environment variable (temporary)"
  echo "  export KUBECONFIG=/path/to/kubeconfig.yaml"
  echo "  ./scripts/access-argocd-ui.sh ${ENVIRONMENT}"
  echo ""
  echo "Option 3: Use script argument"
  echo "  ./scripts/access-argocd-ui.sh ${ENVIRONMENT} /path/to/kubeconfig.yaml"
  echo ""
  echo "Option 4: Save to expected location (if you prefer local storage)"
  echo "  mkdir -p ${KUBECONFIG_DIR}"
  echo "  cp <your-kubeconfig> ${DEFAULT_KUBECONFIG}"
  echo "  ./scripts/access-argocd-ui.sh ${ENVIRONMENT}"
  echo ""
  echo "Option 5: Get from Linode CLI"
  echo "  linode-cli lke kubeconfig-view ${CLUSTER_ID} --text | ./scripts/access-argocd-ui.sh ${ENVIRONMENT} -"
  echo ""
  echo "💡 Best Practice: Keep kubeconfigs in 1Password and use Option 1 to avoid storing secrets on disk."
  exit 1
fi

# Set kubeconfig
export KUBECONFIG="$KUBECONFIG_FILE"
kubectl config use-context "$CONTEXT" >/dev/null 2>&1 || {
  echo "Error: Failed to set context $CONTEXT"
  exit 1
}

echo "🔍 Checking Argo CD status..."
if ! kubectl get namespace argocd >/dev/null 2>&1; then
  echo "❌ Argo CD namespace not found. Argo CD may not be installed."
  echo "   Install with: ./scripts/gitops/bootstrap_argocd.sh $ENVIRONMENT $CONTEXT"
  exit 1
fi

# Check if Argo CD server is running
if ! kubectl -n argocd get deployment argocd-server >/dev/null 2>&1; then
  echo "❌ Argo CD server not found. Argo CD may not be installed."
  exit 1
fi

echo "✅ Argo CD is installed"

# Get admin password
echo ""
echo "🔑 Getting Argo CD admin password..."
PASSWORD=$(kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" 2>/dev/null | base64 -d 2>/dev/null || echo "")

if [[ -z "$PASSWORD" ]]; then
  echo "⚠️  Could not retrieve password. You may need to get it manually:"
  echo "   kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath='{.data.password}' | base64 -d"
else
  echo "✅ Password retrieved"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "  Argo CD Web UI Access"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
  echo "  📍 URL:      http://localhost:8080"
  echo "  👤 Username: admin"
  echo "  🔐 Password: $PASSWORD"
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo ""
fi

# Start port-forward
echo "🚀 Starting port-forward to Argo CD server..."
echo "   Keep this terminal open to maintain the connection."
echo "   Press Ctrl+C to stop."
echo ""
echo "   Opening browser in 3 seconds..."
sleep 3

# Try to open browser (works on macOS and Linux with xdg-open)
if command -v open >/dev/null 2>&1; then
  open http://localhost:8080 2>/dev/null || true
elif command -v xdg-open >/dev/null 2>&1; then
  xdg-open http://localhost:8080 2>/dev/null || true
fi

# Port-forward (this will block)
kubectl -n argocd port-forward svc/argocd-server 8080:80

