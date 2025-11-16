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
mkdir -p /etc/ai-aas
chmod 755 "${WORKSPACE_DIR}"

# Create workspace metadata file with TTL information
log "Creating workspace metadata..."
cat > /etc/ai-aas/workspace-metadata.json <<EOF
{
  "workspace_name": "${WORKSPACE_NAME}",
  "owner": "${OWNER}",
  "ttl_hours": ${TTL_HOURS},
  "created_at": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "created_at_unix": $(date +%s),
  "expires_at_unix": $(($(date +%s) + TTL_HOURS * 3600))
}
EOF
chmod 644 /etc/ai-aas/workspace-metadata.json
log "Workspace metadata created: TTL=${TTL_HOURS}h"

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
# Called by systemd timer or cron

set -euo pipefail

METADATA_FILE="/etc/ai-aas/workspace-metadata.json"
LOG_TAG="workspace-cleanup"

log() {
  logger -t "${LOG_TAG}" "$*"
  echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" >> /var/log/workspace-cleanup.log
}

# Load workspace metadata
if [[ ! -f "${METADATA_FILE}" ]]; then
  log "ERROR: Workspace metadata file not found: ${METADATA_FILE}"
  exit 1
fi

WORKSPACE_NAME=$(jq -r '.workspace_name' "${METADATA_FILE}")
OWNER=$(jq -r '.owner' "${METADATA_FILE}")
TTL_HOURS=$(jq -r '.ttl_hours' "${METADATA_FILE}")
CREATED_AT=$(jq -r '.created_at_unix' "${METADATA_FILE}")
EXPIRES_AT=$(jq -r '.expires_at_unix' "${METADATA_FILE}")

log "TTL cleanup check for workspace ${WORKSPACE_NAME} (TTL: ${TTL_HOURS}h, Owner: ${OWNER})"

# Calculate current time and age
NOW=$(date +%s)
AGE_SECONDS=$((NOW - CREATED_AT))
AGE_HOURS=$((AGE_SECONDS / 3600))
TIME_UNTIL_EXPIRY=$((EXPIRES_AT - NOW))

# Check if workspace has expired
if [[ ${NOW} -ge ${EXPIRES_AT} ]]; then
  log "WARNING: Workspace TTL expired! (Age: ${AGE_HOURS}h, TTL: ${TTL_HOURS}h)"
  log "Initiating workspace destruction..."
  
  # Stop dev stack service
  if systemctl is-active --quiet ai-aas-dev-stack.service; then
    log "Stopping dev stack service..."
    systemctl stop ai-aas-dev-stack.service || true
  fi
  
  # Stop Docker Compose stack
  if command -v docker >/dev/null 2>&1 && [[ -d /opt/ai-aas/dev-stack/compose ]]; then
    log "Stopping Docker Compose stack..."
    cd /opt/ai-aas/dev-stack/compose
    docker compose -f compose.base.yaml -f compose.remote.yaml down -v || true
  fi
  
  # Get Linode instance ID from metadata service or tags
  # Linode instances can query their own metadata via http://169.254.169.254/latest/meta-data/
  INSTANCE_ID=""
  if command -v curl >/dev/null 2>&1; then
    INSTANCE_ID=$(curl -s http://169.254.169.254/latest/meta-data/instance-id 2>/dev/null || echo "")
  fi
  
  # If we have Linode API token, attempt to destroy instance
  # Otherwise, mark for manual cleanup
  if [[ -n "${LINODE_TOKEN:-}" ]] && [[ -n "${INSTANCE_ID}" ]]; then
    log "Attempting to destroy Linode instance ${INSTANCE_ID} via API..."
    # Use Linode API to delete instance
    # Note: This requires LINODE_TOKEN environment variable with appropriate permissions
    curl -s -H "Authorization: Bearer ${LINODE_TOKEN}" \
         -X DELETE \
         "https://api.linode.com/v4/linode/instances/${INSTANCE_ID}" \
         || log "WARNING: Failed to destroy instance via API (may require manual cleanup)"
  else
    log "WARNING: Cannot auto-destroy instance (missing LINODE_TOKEN or INSTANCE_ID)"
    log "Instance should be destroyed manually via Terraform or Linode API"
    log "Marking workspace as expired in metadata..."
    jq '.expired = true | .expired_at = now' "${METADATA_FILE}" > "${METADATA_FILE}.tmp" && \
      mv "${METADATA_FILE}.tmp" "${METADATA_FILE}"
  fi
  
  # Send notification (if notification system configured)
  log "Workspace ${WORKSPACE_NAME} has been cleaned up due to TTL expiration"
  
  exit 0
else
  # Workspace still valid
  HOURS_REMAINING=$((TIME_UNTIL_EXPIRY / 3600))
  log "Workspace still valid (Age: ${AGE_HOURS}h/${TTL_HOURS}h, ${HOURS_REMAINING}h remaining)"
  
  # Warn if approaching expiration (within 1 hour)
  if [[ ${TIME_UNTIL_EXPIRY} -lt 3600 ]]; then
    log "WARNING: Workspace expires in less than 1 hour!"
  fi
fi
EOF
chmod +x /usr/local/bin/workspace-cleanup.sh

# Create systemd timer for TTL cleanup (runs every hour)
log "Creating systemd timer for TTL cleanup..."
cat > /etc/systemd/system/workspace-cleanup.timer <<'EOF'
[Unit]
Description=Workspace TTL Cleanup Timer
After=network-online.target

[Timer]
OnBootSec=1h
OnUnitActiveSec=1h
AccuracySec=5m

[Install]
WantedBy=timers.target
EOF

cat > /etc/systemd/system/workspace-cleanup.service <<'EOF'
[Unit]
Description=Workspace TTL Cleanup Service
After=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/workspace-cleanup.sh
StandardOutput=journal
StandardError=journal
EOF

# Enable and start the timer
systemctl daemon-reload
systemctl enable workspace-cleanup.timer
systemctl start workspace-cleanup.timer
log "TTL cleanup timer enabled (runs every hour)"

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

