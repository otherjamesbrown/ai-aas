#!/usr/bin/env bash
# Update hosts file with ingress IP addresses
# Usage: ./scripts/infra/update-hosts-file.sh [--ingress-ip <ip>] [--kube-context <context>] [--namespace <ns>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

INGRESS_IP=""
KUBE_CONTEXT=""
NAMESPACE="ingress-nginx"
INGRESS_SVC="ingress-nginx-controller"

# Domains to add
DOMAINS=(
  "api.dev.ai-aas.local"
  "portal.dev.ai-aas.local"
  "grafana.dev.ai-aas.local"
  "argocd.dev.ai-aas.local"
  "api.prod.ai-aas.local"
  "portal.prod.ai-aas.local"
  "grafana.prod.ai-aas.local"
  "argocd.prod.ai-aas.local"
)

while [[ $# -gt 0 ]]; do
  case "$1" in
    --ingress-ip)
      INGRESS_IP="$2"
      shift 2
      ;;
    --kube-context)
      KUBE_CONTEXT="$2"
      shift 2
      ;;
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --help|-h)
      cat <<USAGE
Usage: $(basename "$0") [--ingress-ip <ip>] [--kube-context <context>] [--namespace <ns>]

Updates the hosts file with ingress IP addresses for all services.

Options:
  --ingress-ip      Ingress controller IP (if not provided, will try to detect from k8s)
  --kube-context    Kubernetes context to use for detecting IP
  --namespace       Namespace where ingress controller is running (default: ingress-nginx)

Examples:
  # Auto-detect ingress IP from current kubeconfig
  $0

  # Specify IP manually
  $0 --ingress-ip 192.168.1.100

  # Use specific kube context
  $0 --kube-context lke531921-ctx
USAGE
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

# Detect ingress IP if not provided
if [ -z "$INGRESS_IP" ]; then
  echo "🔍 Detecting ingress controller IP..."
  
  KUBECTL_CMD="kubectl"
  if [ -n "$KUBE_CONTEXT" ]; then
    KUBECTL_CMD="kubectl --context=$KUBE_CONTEXT"
  fi
  
  INGRESS_IP=$($KUBECTL_CMD get svc -n "$NAMESPACE" "$INGRESS_SVC" \
    -o jsonpath='{.status.loadBalancer.ingress[0].ip}' 2>/dev/null || echo "")
  
  if [ -z "$INGRESS_IP" ]; then
    # Try hostname instead
    INGRESS_IP=$($KUBECTL_CMD get svc -n "$NAMESPACE" "$INGRESS_SVC" \
      -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")
  fi
  
  if [ -z "$INGRESS_IP" ]; then
    echo "❌ Error: Could not detect ingress IP. Please provide --ingress-ip" >&2
    exit 1
  fi
fi

echo "📝 Updating hosts file..."
echo "   Ingress IP: ${INGRESS_IP}"
echo "   Domains: ${DOMAINS[*]}"
echo ""

# Determine hosts file location
if [[ "$OSTYPE" == "darwin"* ]] || [[ "$OSTYPE" == "linux-gnu"* ]]; then
  HOSTS_FILE="/etc/hosts"
  BACKUP_FILE="/etc/hosts.backup.$(date +%Y%m%d_%H%M%S)"
elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
  HOSTS_FILE="C:\\Windows\\System32\\drivers\\etc\\hosts"
  BACKUP_FILE="C:\\Windows\\System32\\drivers\\etc\\hosts.backup.$(date +%Y%m%d_%H%M%S)"
else
  echo "❌ Error: Unsupported OS: $OSTYPE" >&2
  exit 1
fi

# Check if running as root/admin
if [[ "$OSTYPE" != "msys" ]] && [[ "$OSTYPE" != "win32" ]] && [ "$EUID" -ne 0 ]; then
  echo "⚠️  Warning: This script needs sudo privileges to modify ${HOSTS_FILE}"
  echo "   Please run: sudo $0 $*"
  exit 1
fi

# Backup hosts file
if [ -f "$HOSTS_FILE" ]; then
  echo "💾 Backing up hosts file to ${BACKUP_FILE}..."
  cp "$HOSTS_FILE" "$BACKUP_FILE"
fi

# Remove old entries (if they exist)
echo "🧹 Removing old AI-AAS entries..."
if [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
  # Windows: Use PowerShell to remove lines
  powershell -Command "(Get-Content '$HOSTS_FILE') | Where-Object { \$_ -notmatch 'ai-aas.local' } | Set-Content '$HOSTS_FILE'"
else
  # Linux/macOS: Use sed
  sed -i.bak '/ai-aas.local/d' "$HOSTS_FILE" 2>/dev/null || true
fi

# Add new entries
echo "➕ Adding new entries..."
{
  echo ""
  echo "# AI-AAS Platform - Added by update-hosts-file.sh on $(date)"
  echo "# Development Environment"
  for domain in "${DOMAINS[@]}"; do
    if [[ "$domain" == *.dev.* ]]; then
      echo "${INGRESS_IP}  ${domain}"
    fi
  done
  echo ""
  echo "# Production Environment"
  for domain in "${DOMAINS[@]}"; do
    if [[ "$domain" == *.prod.* ]]; then
      echo "${INGRESS_IP}  ${domain}"
    fi
  done
} >> "$HOSTS_FILE"

echo ""
echo "✅ Hosts file updated successfully!"
echo ""
echo "📋 Added entries:"
for domain in "${DOMAINS[@]}"; do
  echo "   ${INGRESS_IP}  ${domain}"
done
echo ""
echo "🧪 Test DNS resolution:"
echo "   ping ${DOMAINS[0]}"
echo "   curl -k https://${DOMAINS[0]}/healthz"

