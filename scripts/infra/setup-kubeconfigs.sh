#!/usr/bin/env bash
# Setup kubeconfigs from 1Password
# Usage: ./scripts/infra/setup-kubeconfigs.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBECONFIG_DIR="${HOME}/kubeconfigs"

echo "🔐 Setting up kubeconfigs from 1Password"
echo ""

# Check for 1Password CLI
if ! command -v op >/dev/null 2>&1; then
  echo "❌ Error: 1Password CLI (op) not found"
  echo ""
  echo "Install it:"
  echo "  macOS: brew install --cask 1password-cli"
  echo "  Linux: See https://developer.1password.com/docs/cli/get-started"
  exit 1
fi

echo "✅ 1Password CLI found: $(op --version)"
echo ""

# Check if signed in
if ! op account list >/dev/null 2>&1; then
  echo "⚠️  Not signed in to 1Password. Signing in..."
  op signin
fi

# Create directory
mkdir -p "${KUBECONFIG_DIR}"

# Prompt for 1Password reference paths
echo "📋 Enter 1Password reference paths for kubeconfigs"
echo "   Format: op://vault/item-name"
echo ""

read -p "Development kubeconfig path [op://vault/kubeconfig-dev]: " DEV_PATH
DEV_PATH="${DEV_PATH:-op://vault/kubeconfig-dev}"

read -p "Production kubeconfig path [op://vault/kubeconfig-production]: " PROD_PATH
PROD_PATH="${PROD_PATH:-op://vault/kubeconfig-production}"

echo ""
echo "📥 Downloading kubeconfigs..."

# Download development kubeconfig
echo "   Downloading development kubeconfig..."
if op read "${DEV_PATH}" > "${KUBECONFIG_DIR}/kubeconfig-development.yaml" 2>/dev/null; then
  echo "   ✅ Development kubeconfig saved"
else
  echo "   ❌ Failed to read development kubeconfig from ${DEV_PATH}"
  echo "   Please check the path and try again"
  exit 1
fi

# Download production kubeconfig
echo "   Downloading production kubeconfig..."
if op read "${PROD_PATH}" > "${KUBECONFIG_DIR}/kubeconfig-production.yaml" 2>/dev/null; then
  echo "   ✅ Production kubeconfig saved"
else
  echo "   ❌ Failed to read production kubeconfig from ${PROD_PATH}"
  echo "   Please check the path and try again"
  exit 1
fi

# Set permissions
echo ""
echo "🔒 Setting file permissions..."
chmod 600 "${KUBECONFIG_DIR}/kubeconfig-development.yaml"
chmod 600 "${KUBECONFIG_DIR}/kubeconfig-production.yaml"
echo "   ✅ Permissions set to 600 (owner read/write only)"

# Verify kubeconfigs
echo ""
echo "🔍 Verifying kubeconfigs..."

# Check development
if kubectl --kubeconfig="${KUBECONFIG_DIR}/kubeconfig-development.yaml" config get-contexts >/dev/null 2>&1; then
  DEV_CTX=$(kubectl --kubeconfig="${KUBECONFIG_DIR}/kubeconfig-development.yaml" config current-context 2>/dev/null || echo "none")
  echo "   ✅ Development kubeconfig is valid"
  echo "      Current context: ${DEV_CTX}"
  if [ "$DEV_CTX" != "lke531921-ctx" ]; then
    echo "      ⚠️  Expected context: lke531921-ctx"
  fi
else
  echo "   ⚠️  Development kubeconfig may be invalid"
fi

# Check production
if kubectl --kubeconfig="${KUBECONFIG_DIR}/kubeconfig-production.yaml" config get-contexts >/dev/null 2>&1; then
  PROD_CTX=$(kubectl --kubeconfig="${KUBECONFIG_DIR}/kubeconfig-production.yaml" config current-context 2>/dev/null || echo "none")
  echo "   ✅ Production kubeconfig is valid"
  echo "      Current context: ${PROD_CTX}"
  if [ "$PROD_CTX" != "lke531922-ctx" ]; then
    echo "      ⚠️  Expected context: lke531922-ctx"
  fi
else
  echo "   ⚠️  Production kubeconfig may be invalid"
fi

echo ""
echo "✅ Setup complete!"
echo ""
echo "📋 Files created:"
echo "   ${KUBECONFIG_DIR}/kubeconfig-development.yaml"
echo "   ${KUBECONFIG_DIR}/kubeconfig-production.yaml"
echo ""
echo "💡 To use a kubeconfig:"
echo "   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml"
echo "   kubectl config use-context lke531921-ctx"
echo ""
echo "💡 Or use the scripts with --kube-context:"
echo "   ./scripts/infra/create-tls-secrets.sh --kube-context lke531921-ctx"

