#!/bin/bash
# Complete setup script for e2e test environment
# This script handles all setup steps to make tests repeatable

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=== E2E Test Environment Setup ===${NC}"
echo ""

# Step 1: Check prerequisites
echo -e "${YELLOW}Step 1: Checking prerequisites...${NC}"
MISSING=0

if ! command -v curl &> /dev/null; then
    echo -e "${RED}✗ curl is not installed${NC}"
    MISSING=1
else
    echo -e "${GREEN}✓ curl found${NC}"
fi

if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ go is not installed${NC}"
    MISSING=1
else
    echo -e "${GREEN}✓ go found${NC}"
fi

if [ $MISSING -eq 1 ]; then
    echo -e "${RED}Please install missing prerequisites${NC}"
    exit 1
fi

# Step 2: Set environment URLs
echo ""
echo -e "${YELLOW}Step 2: Configuring service URLs...${NC}"

if [ -z "${USER_ORG_SERVICE_URL:-}" ]; then
    export USER_ORG_SERVICE_URL="https://172.232.58.222"
    echo -e "${GREEN}✓ Set USER_ORG_SERVICE_URL=$USER_ORG_SERVICE_URL${NC}"
else
    echo -e "${GREEN}✓ USER_ORG_SERVICE_URL already set${NC}"
fi

if [ -z "${API_ROUTER_SERVICE_URL:-}" ]; then
    export API_ROUTER_SERVICE_URL="https://172.232.58.222"
    echo -e "${GREEN}✓ Set API_ROUTER_SERVICE_URL=$API_ROUTER_SERVICE_URL${NC}"
else
    echo -e "${GREEN}✓ API_ROUTER_SERVICE_URL already set${NC}"
fi

if [ -z "${ANALYTICS_SERVICE_URL:-}" ]; then
    export ANALYTICS_SERVICE_URL="https://172.232.58.222"
    echo -e "${GREEN}✓ Set ANALYTICS_SERVICE_URL=$ANALYTICS_SERVICE_URL${NC}"
else
    echo -e "${GREEN}✓ ANALYTICS_SERVICE_URL already set${NC}"
fi

# Step 3: Bootstrap admin key
echo ""
echo -e "${YELLOW}Step 3: Setting up admin API key...${NC}"

ADMIN_KEY_FILE="${E2E_DIR}/.admin-key.env"
if [ -f "$ADMIN_KEY_FILE" ]; then
    echo -e "${GREEN}✓ Admin key file exists${NC}"
    source "$ADMIN_KEY_FILE"
    if [ -n "${ADMIN_API_KEY:-}" ]; then
        echo -e "${GREEN}✓ Admin API key loaded from $ADMIN_KEY_FILE${NC}"
    else
        echo -e "${YELLOW}⚠ Admin key file exists but ADMIN_API_KEY not set${NC}"
        echo "Running bootstrap script..."
        "$SCRIPT_DIR/bootstrap-admin-key.sh"
        source "$ADMIN_KEY_FILE"
    fi
else
    echo "Admin key file not found. Running bootstrap..."
    "$SCRIPT_DIR/bootstrap-admin-key.sh"
    source "$ADMIN_KEY_FILE"
fi

# Step 4: Verify connectivity
echo ""
echo -e "${YELLOW}Step 4: Verifying service connectivity...${NC}"

if curl -s -k --max-time 5 -H "Host: api.dev.ai-aas.local" "$USER_ORG_SERVICE_URL/health" > /dev/null 2>&1 || \
   curl -s -k --max-time 5 -H "Host: api.dev.ai-aas.local" "$USER_ORG_SERVICE_URL/v1/health" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Services are reachable${NC}"
else
    echo -e "${YELLOW}⚠ Could not verify service health (may be expected)${NC}"
fi

# Step 5: Summary
echo ""
echo -e "${GREEN}=== Setup Complete ===${NC}"
echo ""
echo "Environment configured:"
echo "  USER_ORG_SERVICE_URL=$USER_ORG_SERVICE_URL"
echo "  API_ROUTER_SERVICE_URL=$API_ROUTER_SERVICE_URL"
echo "  ANALYTICS_SERVICE_URL=$ANALYTICS_SERVICE_URL"
echo "  ADMIN_API_KEY=${ADMIN_API_KEY:0:20}... (hidden)"
echo ""
echo "To run tests:"
echo "  cd $E2E_DIR"
echo "  source .admin-key.env  # if not already sourced"
echo "  make test-dev-ip"
echo ""
echo "Or run this script before each test session:"
echo "  source $SCRIPT_DIR/setup-test-env.sh"

