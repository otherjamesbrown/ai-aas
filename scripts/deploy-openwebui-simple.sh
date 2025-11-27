#!/bin/bash
set -e

# Simplified Open WebUI deployment using Linode REST API
# No linode-cli required - uses curl directly

# Required environment variables (no defaults for sensitive values)
LINODE_TOKEN="${LINODE_TOKEN:-}"
AI_AAS_API_KEY="${AI_AAS_API_KEY:-}"
AI_AAS_API_IP="${AI_AAS_API_IP:-}"

# Optional configuration with defaults
LINODE_DEFAULT_FIREWALL="${LINODE_DEFAULT_FIREWALL:-akamai-home}"
INSTANCE_LABEL="${INSTANCE_LABEL:-openwebui-dev}"
INSTANCE_TYPE="${INSTANCE_TYPE:-g6-nanode-1}"
REGION="${REGION:-fr-par}"  # Paris region for proximity to AI-AAS cluster
AI_AAS_API_URL="${AI_AAS_API_URL:-https://api.dev.ai-aas.local}"
SSH_KEY_PATH="${SSH_KEY_PATH:-$HOME/.ssh/id_ed25519.pub}"

# Validate required variables
if [[ -z "$AI_AAS_API_KEY" ]]; then
  echo "ERROR: AI_AAS_API_KEY environment variable is required"
  echo "Usage: AI_AAS_API_KEY=<your-key> AI_AAS_API_IP=<cluster-ip> $0"
  exit 1
fi

if [[ -z "$AI_AAS_API_IP" ]]; then
  echo "ERROR: AI_AAS_API_IP environment variable is required"
  echo "Usage: AI_AAS_API_KEY=<your-key> AI_AAS_API_IP=<cluster-ip> $0"
  exit 1
fi

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${GREEN}==> Deploying Open WebUI to Linode${NC}"
echo ""

# Read SSH key
SSH_PUBLIC_KEY=$(cat "$SSH_KEY_PATH")

# Generate root password
ROOT_PASSWORD=$(openssl rand -base64 32)

# Create instance using Linode API
echo -e "${YELLOW}==> Creating Linode instance...${NC}"

RESPONSE=$(curl -s -X POST https://api.linode.com/v4/linode/instances \
  -H "Authorization: Bearer ${LINODE_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{
    \"type\": \"${INSTANCE_TYPE}\",
    \"region\": \"${REGION}\",
    \"image\": \"linode/ubuntu22.04\",
    \"label\": \"${INSTANCE_LABEL}\",
    \"root_pass\": \"${ROOT_PASSWORD}\",
    \"authorized_keys\": [\"${SSH_PUBLIC_KEY}\"]
  }")

INSTANCE_ID=$(echo "$RESPONSE" | jq -r '.id')
INSTANCE_IP=$(echo "$RESPONSE" | jq -r '.ipv4[0]')

if [[ -z "$INSTANCE_ID" ]] || [[ "$INSTANCE_ID" == "null" ]]; then
  echo "Error creating instance:"
  echo "$RESPONSE" | jq '.'
  exit 1
fi

echo -e "${GREEN}✓ Instance created${NC}"
echo "  ID: $INSTANCE_ID"
echo "  IP: $INSTANCE_IP"

# Attach default firewall
echo ""
echo -e "${YELLOW}==> Attaching '${LINODE_DEFAULT_FIREWALL}' firewall...${NC}"

# Get firewall ID
FIREWALL_ID=$(curl -s -H "Authorization: Bearer ${LINODE_TOKEN}" \
  https://api.linode.com/v4/networking/firewalls | \
  jq -r ".data[] | select(.label == \"${LINODE_DEFAULT_FIREWALL}\") | .id")

if [[ -z "$FIREWALL_ID" ]] || [[ "$FIREWALL_ID" == "null" ]]; then
  echo -e "${YELLOW}Warning: '${LINODE_DEFAULT_FIREWALL}' firewall not found, skipping...${NC}"
else
  # Attach firewall to instance
  curl -s -X POST "https://api.linode.com/v4/networking/firewalls/${FIREWALL_ID}/devices" \
    -H "Authorization: Bearer ${LINODE_TOKEN}" \
    -H "Content-Type: application/json" \
    -d "{\"id\": ${INSTANCE_ID}, \"type\": \"linode\"}" > /dev/null

  echo -e "${GREEN}✓ Firewall '${LINODE_DEFAULT_FIREWALL}' attached${NC}"
fi

# Wait for instance to boot
echo ""
echo "Waiting for instance to boot (60 seconds)..."
sleep 60

# Install Docker and Open WebUI
echo ""
echo -e "${YELLOW}==> Installing Docker and Open WebUI${NC}"

ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  -o ConnectTimeout=10 \
  root@${INSTANCE_IP} bash <<EOF
set -e

echo "==> Adding hosts entry for AI-AAS API"
echo "${AI_AAS_API_IP} api.dev.ai-aas.local" >> /etc/hosts

echo "==> Installing Docker"
curl -fsSL https://get.docker.com | sh
systemctl enable docker
systemctl start docker

echo "==> Starting Open WebUI with HTTPS and API key"
docker run -d \
  --name open-webui \
  --add-host api.dev.ai-aas.local:${AI_AAS_API_IP} \
  -p 3000:8080 \
  -v open-webui:/app/backend/data \
  -e OPENAI_API_BASE_URL=${AI_AAS_API_URL}/v1 \
  -e OPENAI_API_KEY=${AI_AAS_API_KEY} \
  -e WEBUI_AUTH=false \
  -e OPENAI_API_VERIFY_SSL=false \
  --restart unless-stopped \
  ghcr.io/open-webui/open-webui:main

echo "==> Waiting for container to start"
sleep 15
docker ps

echo "==> Installation complete!"
EOF

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Open WebUI Deployed Successfully!                     ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Access:${NC}"
echo "  Open WebUI: http://${INSTANCE_IP}:3000"
echo "  AI-AAS API: ${AI_AAS_API_URL}"
echo ""
echo -e "${BLUE}Instance Details:${NC}"
echo "  Instance ID: ${INSTANCE_ID}"
echo "  SSH: ssh root@${INSTANCE_IP}"
echo ""
echo -e "${BLUE}Cleanup:${NC}"
echo "  curl -X DELETE https://api.linode.com/v4/linode/instances/${INSTANCE_ID} \\"
echo "    -H \"Authorization: Bearer \$LINODE_TOKEN\""
echo ""

# Save info
cat > /tmp/openwebui-info.txt <<INFO
Open WebUI Instance
===================
Instance ID: ${INSTANCE_ID}
IP Address:  ${INSTANCE_IP}
Created:     $(date)

Access:
  http://${INSTANCE_IP}:3000

SSH:
  ssh root@${INSTANCE_IP}

Delete:
  curl -X DELETE https://api.linode.com/v4/linode/instances/${INSTANCE_ID} \\
    -H "Authorization: Bearer \$LINODE_TOKEN"
INFO

echo "Info saved to: /tmp/openwebui-info.txt"
