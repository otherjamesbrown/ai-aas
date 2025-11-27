#!/bin/bash
set -e

# Deploy Open WebUI on a Linode instance configured to use AI-AAS platform
# Usage: ./scripts/deploy-openwebui-linode.sh [OPTIONS]

# Default values
INSTANCE_LABEL="openwebui-dev"
INSTANCE_TYPE="g6-nanode-1"  # 1GB RAM, $5/month - sufficient for Open WebUI
REGION="us-east"
LINODE_TOKEN="${LINODE_TOKEN:-}"
AI_AAS_API_URL="${AI_AAS_API_URL:-http://api-router.ai-aas.dev:8080}"
SSH_KEY_PATH="${HOME}/.ssh/id_rsa.pub"
INSTALL_ONLY="false"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

show_help() {
  cat << EOF
${GREEN}Open WebUI Deployment on Linode${NC}

Deploys Open WebUI on a fresh Linode instance configured to use AI-AAS platform.

${YELLOW}Usage:${NC}
  $0 [OPTIONS]

${YELLOW}Required:${NC}
  --token TOKEN             Linode API token (or set LINODE_TOKEN env var)

${YELLOW}Instance Options:${NC}
  --label LABEL             Instance label (default: openwebui-dev)
  --type TYPE               Linode instance type (default: g6-nanode-1)
  --region REGION           Linode region (default: us-east)

${YELLOW}Configuration:${NC}
  --api-url URL             AI-AAS API Router URL (default: http://api-router.ai-aas.dev:8080)
  --ssh-key PATH            Path to SSH public key (default: ~/.ssh/id_rsa.pub)
  --install-only            Skip instance creation, install on existing instance

${YELLOW}Instance Types:${NC}
  g6-nanode-1     (1GB RAM, 1 CPU)    - \$5/month (recommended)
  g6-standard-1   (2GB RAM, 1 CPU)    - \$12/month
  g6-standard-2   (4GB RAM, 2 CPU)    - \$24/month

${YELLOW}Examples:${NC}
  # Create instance and deploy Open WebUI
  $0 --token \$LINODE_TOKEN

  # Deploy with custom API URL
  $0 --token \$LINODE_TOKEN \\
     --api-url http://172.232.49.84:8080

  # Deploy to specific region
  $0 --token \$LINODE_TOKEN --region us-west

${YELLOW}Environment Variables:${NC}
  LINODE_TOKEN              Linode API token
  AI_AAS_API_URL            AI-AAS API Router URL
EOF
}

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --token)
      LINODE_TOKEN="$2"
      shift 2
      ;;
    --label)
      INSTANCE_LABEL="$2"
      shift 2
      ;;
    --type)
      INSTANCE_TYPE="$2"
      shift 2
      ;;
    --region)
      REGION="$2"
      shift 2
      ;;
    --api-url)
      AI_AAS_API_URL="$2"
      shift 2
      ;;
    --ssh-key)
      SSH_KEY_PATH="$2"
      shift 2
      ;;
    --install-only)
      INSTALL_ONLY="true"
      shift
      ;;
    --help)
      show_help
      exit 0
      ;;
    *)
      echo -e "${RED}Unknown option: $1${NC}"
      echo "Use --help for usage information"
      exit 1
      ;;
  esac
done

# Validate required parameters
if [[ -z "${LINODE_TOKEN}" ]]; then
  echo -e "${RED}ERROR: Linode API token is required${NC}"
  echo "Provide via --token flag or LINODE_TOKEN environment variable"
  exit 1
fi

if [[ ! -f "${SSH_KEY_PATH}" ]]; then
  echo -e "${RED}ERROR: SSH public key not found: ${SSH_KEY_PATH}${NC}"
  echo "Generate one with: ssh-keygen -t rsa -b 4096"
  exit 1
fi

echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Open WebUI Deployment on Linode                               ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Configuration:${NC}"
echo "  Instance Label: ${INSTANCE_LABEL}"
echo "  Instance Type:  ${INSTANCE_TYPE}"
echo "  Region:         ${REGION}"
echo "  AI-AAS API:     ${AI_AAS_API_URL}"
echo ""

# Check if linode-cli is installed
if ! command -v linode-cli &> /dev/null; then
  echo -e "${YELLOW}==> Installing linode-cli${NC}"
  pip3 install linode-cli --quiet || {
    echo -e "${RED}ERROR: Failed to install linode-cli${NC}"
    exit 1
  }
fi

# Configure linode-cli
export LINODE_CLI_TOKEN="${LINODE_TOKEN}"
linode-cli configure --token <<< "${LINODE_TOKEN}" > /dev/null 2>&1 || true

# Create instance if not install-only
if [[ "${INSTALL_ONLY}" == "false" ]]; then
  echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${YELLOW}STEP 1: Creating Linode Instance${NC}"
  echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
  echo ""

  # Read SSH public key
  SSH_PUBLIC_KEY=$(cat "${SSH_KEY_PATH}")

  # Generate random root password
  ROOT_PASSWORD=$(openssl rand -base64 32)

  # Create instance
  echo "Creating instance..."
  INSTANCE_JSON=$(linode-cli linodes create \
    --type "${INSTANCE_TYPE}" \
    --region "${REGION}" \
    --image linode/ubuntu22.04 \
    --label "${INSTANCE_LABEL}" \
    --root_pass "${ROOT_PASSWORD}" \
    --authorized_keys "${SSH_PUBLIC_KEY}" \
    --json)

  INSTANCE_ID=$(echo "${INSTANCE_JSON}" | jq -r '.[0].id')
  INSTANCE_IP=$(echo "${INSTANCE_JSON}" | jq -r '.[0].ipv4[0]')

  echo -e "${GREEN}✓ Instance created${NC}"
  echo "  Instance ID: ${INSTANCE_ID}"
  echo "  IP Address:  ${INSTANCE_IP}"

  # Wait for instance to be running
  echo ""
  echo "Waiting for instance to boot..."
  for i in {1..30}; do
    STATUS=$(linode-cli linodes list --json | \
      jq -r ".[] | select(.id == ${INSTANCE_ID}) | .status")

    if [[ "${STATUS}" == "running" ]]; then
      echo -e "${GREEN}✓ Instance is running${NC}"
      break
    fi

    echo "  Status: ${STATUS} (attempt $i/30)"
    sleep 5
  done

  # Wait additional time for SSH to be ready
  echo "Waiting for SSH to be ready..."
  sleep 30

else
  echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
  echo -e "${YELLOW}STEP 1: Using Existing Instance${NC}"
  echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
  echo ""

  # Get instance details
  INSTANCE_JSON=$(linode-cli linodes list --json | \
    jq ".[] | select(.label == \"${INSTANCE_LABEL}\")")

  if [[ -z "${INSTANCE_JSON}" ]]; then
    echo -e "${RED}ERROR: Instance '${INSTANCE_LABEL}' not found${NC}"
    exit 1
  fi

  INSTANCE_ID=$(echo "${INSTANCE_JSON}" | jq -r '.id')
  INSTANCE_IP=$(echo "${INSTANCE_JSON}" | jq -r '.ipv4[0]')

  echo "Using instance:"
  echo "  Instance ID: ${INSTANCE_ID}"
  echo "  IP Address:  ${INSTANCE_IP}"
fi

# Step 2: Install Docker and Open WebUI
echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}STEP 2: Installing Docker and Open WebUI${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo ""

# Create installation script
INSTALL_SCRIPT=$(cat <<'SCRIPT'
#!/bin/bash
set -e

echo "==> Updating system packages"
apt-get update -qq
apt-get upgrade -y -qq

echo "==> Installing Docker"
curl -fsSL https://get.docker.com -o get-docker.sh
sh get-docker.sh
systemctl enable docker
systemctl start docker

echo "==> Installing Docker Compose"
apt-get install -y docker-compose-plugin

echo "==> Creating directory for Open WebUI"
mkdir -p /opt/openwebui

cat > /opt/openwebui/docker-compose.yml <<EOF
version: '3.8'

services:
  open-webui:
    image: ghcr.io/open-webui/open-webui:main
    container_name: open-webui
    ports:
      - "3000:8080"
    volumes:
      - open-webui-data:/app/backend/data
    environment:
      - OPENAI_API_BASE_URL=AI_AAS_API_URL_PLACEHOLDER/v1
      - OPENAI_API_KEY=required_but_unused
      - WEBUI_AUTH=false
    restart: unless-stopped

volumes:
  open-webui-data:
EOF

echo "==> Starting Open WebUI"
cd /opt/openwebui
docker compose up -d

echo "==> Waiting for Open WebUI to start"
sleep 10

echo "==> Installation complete!"
docker compose ps
SCRIPT
)

# Replace placeholder with actual API URL
INSTALL_SCRIPT="${INSTALL_SCRIPT//AI_AAS_API_URL_PLACEHOLDER/${AI_AAS_API_URL}}"

# Execute installation on remote instance
echo "Connecting to ${INSTANCE_IP}..."
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  root@${INSTANCE_IP} "${INSTALL_SCRIPT}"

# Step 3: Verify installation
echo ""
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo -e "${YELLOW}STEP 3: Verifying Installation${NC}"
echo -e "${YELLOW}═══════════════════════════════════════════════════════════════${NC}"
echo ""

# Check if Open WebUI is running
ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null \
  root@${INSTANCE_IP} "docker ps | grep open-webui"

echo ""
echo -e "${GREEN}╔════════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║  Open WebUI Deployed Successfully!                             ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}Access Information:${NC}"
echo ""
echo "  Open WebUI URL:  http://${INSTANCE_IP}:3000"
echo "  AI-AAS API:      ${AI_AAS_API_URL}"
echo ""
echo -e "${BLUE}SSH Access:${NC}"
echo ""
echo "  ssh root@${INSTANCE_IP}"
echo ""
echo -e "${BLUE}Docker Commands:${NC}"
echo ""
echo "  # View logs"
echo "  ssh root@${INSTANCE_IP} 'cd /opt/openwebui && docker compose logs -f'"
echo ""
echo "  # Restart"
echo "  ssh root@${INSTANCE_IP} 'cd /opt/openwebui && docker compose restart'"
echo ""
echo "  # Stop"
echo "  ssh root@${INSTANCE_IP} 'cd /opt/openwebui && docker compose stop'"
echo ""
echo -e "${BLUE}Next Steps:${NC}"
echo ""
echo "  1. Open http://${INSTANCE_IP}:3000 in your browser"
echo "  2. Start chatting - it will use your AI-AAS platform!"
echo "  3. API calls will go to: ${AI_AAS_API_URL}"
echo ""
echo -e "${BLUE}Cleanup:${NC}"
echo ""
echo "  # Delete the instance when done"
echo "  linode-cli linodes delete ${INSTANCE_ID}"
echo ""

# Save instance info
INFO_FILE="/tmp/openwebui-${INSTANCE_LABEL}-info.txt"
cat > "${INFO_FILE}" <<EOF
Open WebUI Instance Information
================================

Instance ID:     ${INSTANCE_ID}
Instance Label:  ${INSTANCE_LABEL}
IP Address:      ${INSTANCE_IP}
Region:          ${REGION}
Created:         $(date)

URLs:
  Open WebUI:    http://${INSTANCE_IP}:3000
  AI-AAS API:    ${AI_AAS_API_URL}

SSH Access:
  ssh root@${INSTANCE_IP}

Docker Logs:
  ssh root@${INSTANCE_IP} 'cd /opt/openwebui && docker compose logs -f'

Delete Instance:
  linode-cli linodes delete ${INSTANCE_ID}
EOF

echo -e "${GREEN}✓ Instance information saved to: ${INFO_FILE}${NC}"
echo ""
