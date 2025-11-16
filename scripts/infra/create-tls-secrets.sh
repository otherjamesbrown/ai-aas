#!/usr/bin/env bash
# Create Kubernetes TLS secrets from certificate files
# Usage: ./scripts/infra/create-tls-secrets.sh [--cert-dir <dir>] [--namespace <ns>] [--kube-context <context>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

CERT_DIR="${PROJECT_ROOT}/infra/secrets/certs"
NAMESPACE="default"
KUBE_CONTEXT=""
SECRET_NAME="ai-aas-tls"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --cert-dir)
      CERT_DIR="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --kube-context)
      KUBE_CONTEXT="$2"
      shift 2
      ;;
    --secret-name)
      SECRET_NAME="$2"
      shift 2
      ;;
    --help|-h)
      cat <<USAGE
Usage: $(basename "$0") [--cert-dir <dir>] [--namespace <ns>] [--kube-context <context>] [--secret-name <name>]

Creates Kubernetes TLS secrets from certificate files.

Options:
  --cert-dir       Directory containing tls.crt and tls.key (default: infra/secrets/certs)
  --namespace      Kubernetes namespace for secrets (default: default)
  --kube-context   Kubernetes context to use
  --secret-name    Name for the TLS secret (default: ai-aas-tls)

Examples:
  # Create secrets with defaults
  $0

  # Create in specific namespace
  $0 --namespace ingress-nginx

  # Use custom certificate directory
  $0 --cert-dir /tmp/certs
USAGE
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

# Check if certificate files exist
TLS_CRT="${CERT_DIR}/tls.crt"
TLS_KEY="${CERT_DIR}/tls.key"

if [ ! -f "$TLS_CRT" ]; then
  echo "❌ Error: Certificate file not found: ${TLS_CRT}" >&2
  echo "   Run ./scripts/infra/generate-self-signed-certs.sh first" >&2
  exit 1
fi

if [ ! -f "$TLS_KEY" ]; then
  echo "❌ Error: Private key file not found: ${TLS_KEY}" >&2
  echo "   Run ./scripts/infra/generate-self-signed-certs.sh first" >&2
  exit 1
fi

echo "🔐 Creating Kubernetes TLS secrets..."
echo "   Certificate: ${TLS_CRT}"
echo "   Private Key: ${TLS_KEY}"
echo "   Namespace: ${NAMESPACE}"
echo "   Secret Name: ${SECRET_NAME}"
echo ""

# Check kubectl connectivity
if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "❌ Error: Cannot connect to Kubernetes cluster"
  echo ""
  echo "📋 Setup options:"
  echo ""
  echo "Option 1: Use setup script (recommended)"
  echo "   ./scripts/infra/setup-kubeconfigs.sh"
  echo ""
  echo "Option 2: Set KUBECONFIG environment variable"
  echo "   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml"
  echo "   $0"
  echo ""
  echo "Option 3: Specify kubeconfig via --kube-context"
  echo "   $0 --kube-context lke531921-ctx"
  echo ""
  echo "Option 4: Use 1Password CLI directly"
  echo "   op read \"op://vault/kubeconfig-dev\" > /tmp/kubeconfig.yaml"
  echo "   KUBECONFIG=/tmp/kubeconfig.yaml $0"
  echo ""
  echo "See tmp_md/KUBECONFIG_SETUP.md for detailed instructions"
  exit 1
fi

# Build kubectl command
KUBECTL_CMD="kubectl"
if [ -n "$KUBE_CONTEXT" ]; then
  KUBECTL_CMD="kubectl --context=$KUBE_CONTEXT"
fi

# Check if namespace exists
if ! $KUBECTL_CMD get namespace "$NAMESPACE" >/dev/null 2>&1; then
  echo "📦 Creating namespace: ${NAMESPACE}..."
  $KUBECTL_CMD create namespace "$NAMESPACE"
fi

# Delete existing secret if it exists
if $KUBECTL_CMD get secret "$SECRET_NAME" -n "$NAMESPACE" >/dev/null 2>&1; then
  echo "🗑️  Deleting existing secret: ${SECRET_NAME}"
  $KUBECTL_CMD delete secret "$SECRET_NAME" -n "$NAMESPACE"
fi

# Create TLS secret
echo "➕ Creating TLS secret: ${SECRET_NAME} in namespace ${NAMESPACE}..."
$KUBECTL_CMD create secret tls "$SECRET_NAME" \
  --cert="$TLS_CRT" \
  --key="$TLS_KEY" \
  -n "$NAMESPACE"

echo ""
echo "✅ TLS secret created successfully!"
echo ""
echo "📋 Secret details:"
$KUBECTL_CMD get secret "$SECRET_NAME" -n "$NAMESPACE" -o yaml | grep -E "name:|namespace:|type:" || true
echo ""
echo "💡 To use this secret in ingress, add to your Helm values:"
echo "   ingress:"
echo "     tls:"
echo "       - secretName: ${SECRET_NAME}"
echo "         hosts:"
echo "           - api.dev.ai-aas.local"
echo "           - portal.dev.ai-aas.local"
echo "           # ... etc"

