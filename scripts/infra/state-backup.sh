#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

ENVIRONMENT="$(resolve_environment "")"
BACKUP_DIR="${STATE_BACKUP_DIR:-$REPO_ROOT/infra/terraform/backups}"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
STATE_FILE="$BACKUP_DIR/${ENVIRONMENT}-state-${TIMESTAMP}.tfstate"

mkdir -p "$BACKUP_DIR"
log "Pulling terraform state for ENV=$ENVIRONMENT"

run_make infra-state-pull ENV="$ENVIRONMENT"

if [[ -f "$REPO_ROOT/infra/terraform/environments/$ENVIRONMENT/terraform.tfstate" ]]; then
  mv "$REPO_ROOT/infra/terraform/environments/$ENVIRONMENT/terraform.tfstate" "$STATE_FILE"
else
  warn "Expected terraform state file not found after pull"
fi

echo "State saved to $STATE_FILE"

if [[ -n "${STATE_BACKUP_BUCKET:-}" ]]; then
  if ! command -v aws >/dev/null 2>&1; then
    warn "aws CLI not available; skipping upload"
  else
    DEST="s3://${STATE_BACKUP_BUCKET}/${ENVIRONMENT}/${ENVIRONMENT}-state-${TIMESTAMP}.tfstate"
    log "Uploading state to ${DEST}"
    aws s3 cp "$STATE_FILE" "$DEST"
  fi
fi
