#!/bin/bash
# Setup /etc/hosts entries for remote development environment

REMOTE_IP="172.232.58.222"

# Check if entries already exist
if grep -q "ai-aas.local" /etc/hosts; then
    echo "⚠️  Found existing ai-aas.local entries in /etc/hosts"
    echo "Please review and remove old entries first, or run:"
    echo "  sudo sed -i.bak '/ai-aas.local/d' /etc/hosts"
    echo ""
fi

echo "Adding development cluster endpoints to /etc/hosts..."
echo ""

# Add all endpoints
sudo tee -a /etc/hosts > /dev/null << HOSTS

# AI-AAS Development Cluster (Added $(date))
$REMOTE_IP argocd.dev.ai-aas.local
$REMOTE_IP api.dev.ai-aas.local
$REMOTE_IP portal.dev.ai-aas.local
$REMOTE_IP user-org.dev.ai-aas.local
$REMOTE_IP etcd.dev.ai-aas.local
HOSTS

echo "✅ Hosts file updated successfully!"
echo ""
echo "Added endpoints:"
echo "  - https://argocd.dev.ai-aas.local (ArgoCD)"
echo "  - https://api.dev.ai-aas.local (API Router)"
echo "  - https://portal.dev.ai-aas.local (Web Portal)"
echo "  - https://user-org.dev.ai-aas.local (User-Org Service)"
echo "  - https://etcd.dev.ai-aas.local (etcd)"
echo ""
echo "You can now test with:"
echo "  curl -k https://portal.dev.ai-aas.local"
