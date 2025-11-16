#!/bin/bash
# Bootstrap script for Linode dev workspace
# Installs Docker Compose stack, Vector agent, and systemd units for dependency orchestration.
# This script runs during instance provisioning via StackScript.

set -euo pipefail

# StackScript data passed from Terraform
WORKSPACE_NAME="${UDF_WORKSPACE_NAME:-dev-workspace}"
OWNER="${UDF_OWNER:-unknown}"
TTL_HOURS="${UDF_TTL_HOURS:-24}"

# Logging
LOG_FILE="/var/log/workspace-bootstrap.log"
log() {
  echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "${LOG_FILE}"
}

log "Starting workspace bootstrap for ${WORKSPACE_NAME} (owner: ${OWNER}, TTL: ${TTL_HOURS}h)"

# Update system packages
log "Updating system packages..."
export DEBIAN_FRONTEND=noninteractive
apt-get update -qq
apt-get upgrade -y -qq
apt-get install -y -qq \
  curl \
  wget \
  git \
  jq \
  unzip \
  ca-certificates \
  gnupg \
  lsb-release \
  systemd \
  net-tools \
  iputils-ping \
  dnsutils

# Install Docker
if ! command -v docker >/dev/null 2>&1; then
  log "Installing Docker..."
  install -m 0755 -d /etc/apt/keyrings
  curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
  chmod a+r /etc/apt/keyrings/docker.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | \
    tee /etc/apt/sources.list.d/docker.list > /dev/null
  apt-get update -qq
  apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
  systemctl enable docker
  systemctl start docker
  log "Docker installed successfully"
else
  log "Docker already installed"
fi

# Verify Docker Compose v2
if docker compose version >/dev/null 2>&1; then
  log "Docker Compose v2 verified"
else
  log "ERROR: Docker Compose v2 not available"
  exit 1
fi

# Create workspace directory structure
WORKSPACE_DIR="/opt/ai-aas/dev-stack"
log "Creating workspace directory: ${WORKSPACE_DIR}"
mkdir -p "${WORKSPACE_DIR}"/{compose,data,logs,config}
chmod 755 "${WORKSPACE_DIR}"

# Install Vector agent (lightweight log shipper)
VECTOR_VERSION="0.38.1"
if ! command -v vector >/dev/null 2>&1; then
  log "Installing Vector agent..."
  curl -sSfL --proto '=https' --tlsv1.2 https://sh.vector.dev | bash -s -- -y --version "${VECTOR_VERSION}"
  mkdir -p /etc/vector /var/log/vector
  log "Vector agent installed"
else
  log "Vector agent already installed"
fi

# Create systemd service for dev stack
log "Creating systemd service for dev stack..."
cat > /etc/systemd/system/ai-aas-dev-stack.service <<'EOF'
[Unit]
Description=AI-AAS Development Stack (Docker Compose)
After=docker.service network-online.target
Requires=docker.service
Wants=network-online.target

[Service]
Type=oneshot
RemainAfterExit=yes
WorkingDirectory=/opt/ai-aas/dev-stack/compose
ExecStart=/usr/bin/docker compose -f compose.base.yaml -f compose.remote.yaml up -d
ExecStop=/usr/bin/docker compose -f compose.base.yaml -f compose.remote.yaml down
ExecReload=/usr/bin/docker compose -f compose.base.yaml -f compose.remote.yaml restart
TimeoutStartSec=300
TimeoutStopSec=60
StandardOutput=journal
StandardError=journal
SyslogIdentifier=ai-aas-dev-stack

[Install]
WantedBy=multi-user.target
EOF

# Create systemd service for Vector agent
log "Creating systemd service for Vector agent..."
cat > /etc/systemd/system/vector-agent.service <<'EOF'
[Unit]
Description=Vector Agent (Log Shipper)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/vector --config-dir /etc/vector
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=vector-agent

[Install]
WantedBy=multi-user.target
EOF

# Enable services (but don't start yet - wait for compose files)
systemctl daemon-reload
systemctl enable ai-aas-dev-stack.service
systemctl enable vector-agent.service

# Set up log rotation for workspace logs (90-day retention per data classification policy)
log "Configuring log rotation (90-day retention)..."
cat > /etc/logrotate.d/ai-aas-workspace <<EOF
/var/log/workspace-bootstrap.log
/var/log/ai-aas/*.log
/opt/ai-aas/dev-stack/logs/*.log {
    daily
    rotate 90
    compress
    delaycompress
    missingok
    notifempty
    create 0644 root root
    # Data classification: Internal, 90-day retention per docs/platform/data-classification.md
}
EOF

# Configure Vector agent with 90-day retention classification
log "Configuring Vector agent with retention policy..."
mkdir -p /etc/vector
if [[ -f /root/vector-agent.toml ]]; then
  cp /root/vector-agent.toml /etc/vector/vector-agent.toml
  # Ensure retention tags are present
  grep -q "retention.*90" /etc/vector/vector-agent.toml || echo "# Retention: 90 days (data classification: Internal)" >> /etc/vector/vector-agent.toml
fi

# Create cleanup script for TTL enforcement
log "Creating TTL cleanup script..."
cat > /usr/local/bin/workspace-cleanup.sh <<'EOF'
#!/bin/bash
# TTL-based cleanup script for workspace teardown
# Called by cron or systemd timer

TTL_HOURS="${UDF_TTL_HOURS:-24}"
WORKSPACE_NAME="${UDF_WORKSPACE_NAME:-unknown}"

# Calculate expiration time (simplified - in production, parse from tags/metadata)
# This is a placeholder; real implementation would check TTL from instance tags

log() {
  logger -t workspace-cleanup "[${WORKSPACE_NAME}] $*"
}

log "TTL cleanup check for workspace (TTL: ${TTL_HOURS}h)"
# Actual cleanup would trigger Terraform destroy or notify automation
log "Cleanup logic would run here"
EOF
chmod +x /usr/local/bin/workspace-cleanup.sh

# Create status script for health checks
log "Creating status check script..."
cat > /usr/local/bin/workspace-status.sh <<'EOF'
#!/bin/bash
# Workspace status check script
# Returns JSON with component health status

cd /opt/ai-aas/dev-stack/compose || exit 1

# Check Docker Compose services
if command -v docker >/dev/null 2>&1; then
  docker compose -f compose.base.yaml -f compose.remote.yaml ps --format json 2>/dev/null || echo '{"error": "compose not running"}'
else
  echo '{"error": "docker not available"}'
fi
EOF
chmod +x /usr/local/bin/workspace-status.sh

# Set permissions and ownership
chown -R root:root "${WORKSPACE_DIR}"
chmod 755 /usr/local/bin/workspace-*.sh

log "Bootstrap completed successfully"
log "Workspace: ${WORKSPACE_NAME}, Owner: ${OWNER}, TTL: ${TTL_HOURS}h"
log "Next steps:"
log "  1. Copy compose files to ${WORKSPACE_DIR}/compose/"
log "  2. Run: systemctl start ai-aas-dev-stack"
log "  3. Run: systemctl start vector-agent"

# Write completion marker
echo "bootstrap-completed-$(date +%s)" > "${WORKSPACE_DIR}/.bootstrap-complete"

exit 0

