#!/bin/bash
# Simple script to set up admin key from secrets for E2E tests
# Usage: ./scripts/setup-admin-key.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$E2E_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

OUTPUT_FILE="${E2E_DIR}/.admin-key.env"
SECRETS_FILE="${REPO_ROOT}/secrets/env/.env"

echo -e "${GREEN}=== Setting up E2E Admin API Key ===${NC}"
echo ""

# Check if .admin-key.env already exists
if [ -f "$OUTPUT_FILE" ]; then
    echo -e "${YELLOW}Admin key file already exists: $OUTPUT_FILE${NC}"
    source "$OUTPUT_FILE"
    if [ -n "${ADMIN_API_KEY:-}" ]; then
        echo -e "${GREEN}✓ ADMIN_API_KEY is set (${ADMIN_API_KEY:0:10}...)${NC}"
        echo ""
        echo "To regenerate, delete the file first:"
        echo "  rm $OUTPUT_FILE"
        exit 0
    fi
fi

# Try to get key from secrets
if [ -f "$SECRETS_FILE" ]; then
    MASTER_KEY=$(grep "^MASTER_ADMIN_API_KEY=" "$SECRETS_FILE" | cut -d'=' -f2 | tr -d '"' | tr -d "'" || echo "")

    if [ -n "$MASTER_KEY" ]; then
        cat > "$OUTPUT_FILE" << EOF
# Admin API Key for E2E Tests
# Source: secrets/env/.env (MASTER_ADMIN_API_KEY)
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

export ADMIN_API_KEY="$MASTER_KEY"
EOF
        echo -e "${GREEN}✓ Admin key extracted from secrets and saved to:${NC}"
        echo "  $OUTPUT_FILE"
        echo ""
        echo "The key will be automatically loaded by test scripts."
        echo ""
        echo "To use manually:"
        echo "  source $OUTPUT_FILE"
        echo "  make test-dev-internet"
        exit 0
    fi
fi

# Fallback: check environment variable
if [ -n "${MASTER_ADMIN_API_KEY:-}" ]; then
    cat > "$OUTPUT_FILE" << EOF
# Admin API Key for E2E Tests
# Source: MASTER_ADMIN_API_KEY environment variable
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

export ADMIN_API_KEY="$MASTER_ADMIN_API_KEY"
EOF
    echo -e "${GREEN}✓ Admin key saved from environment variable${NC}"
    exit 0
fi

# No key found
echo -e "${RED}✗ Could not find admin API key${NC}"
echo ""
echo "Expected locations:"
echo "  1. $SECRETS_FILE (MASTER_ADMIN_API_KEY=...)"
echo "  2. Environment variable MASTER_ADMIN_API_KEY"
echo ""
echo "If you have the key, you can create the file manually:"
echo "  echo 'export ADMIN_API_KEY=\"your-key-here\"' > $OUTPUT_FILE"
exit 1
