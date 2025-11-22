#!/bin/bash
# Bootstrap script to create an admin API key for e2e tests
# This script creates a test organization and admin API key if they don't exist
# Run this once before running e2e tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Configuration
BASE_URL="${USER_ORG_SERVICE_URL:-https://172.232.58.222}"
ORG_NAME="e2e-test-admin-org"
ORG_SLUG="e2e-test-admin-org-$(date +%s | tail -c 6)"  # Add timestamp to make unique
API_KEY_NAME="e2e-admin-key"
OUTPUT_FILE="${E2E_DIR}/.admin-key.env"

# Check if admin key already exists
if [ -f "$OUTPUT_FILE" ]; then
    echo -e "${YELLOW}Admin key file already exists: $OUTPUT_FILE${NC}"
    echo "Contents:"
    cat "$OUTPUT_FILE"
    echo ""
    read -p "Recreate admin key? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Using existing admin key."
        exit 0
    fi
fi

echo -e "${GREEN}=== Bootstrap Admin API Key for E2E Tests ===${NC}"
echo ""
echo "This script will:"
echo "  1. Create a test organization (if needed)"
echo "  2. Create an admin API key for that organization"
echo "  3. Save the key to $OUTPUT_FILE"
echo ""

# Step 1: Try to create organization (may fail if auth required)
echo -e "${YELLOW}Step 1: Creating test organization...${NC}"

# Check if we need to use Host header
CURL_HOST_HEADER=""
if [[ "$BASE_URL" =~ ^https?://[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    CURL_HOST_HEADER="-H Host:api.dev.ai-aas.local"
fi

# Try to create org (this may require existing auth or bootstrap endpoint)
TEMP_FILE=$(mktemp)
if [ -n "$CURL_HOST_HEADER" ]; then
    curl -s -k "$CURL_HOST_HEADER" \
        -X POST "$BASE_URL/v1/orgs" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"$ORG_NAME\",\"slug\":\"$ORG_SLUG\"}" \
        -w "\n%{http_code}" \
        -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
else
    curl -s -k \
        -X POST "$BASE_URL/v1/orgs" \
        -H "Content-Type: application/json" \
        -d "{\"name\":\"$ORG_NAME\",\"slug\":\"$ORG_SLUG\"}" \
        -w "\n%{http_code}" \
        -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
fi

HTTP_CODE=$(tail -1 "$TEMP_FILE")
ORG_BODY=$(head -n -1 "$TEMP_FILE" 2>/dev/null || sed '$d' "$TEMP_FILE" 2>/dev/null || cat "$TEMP_FILE")
rm -f "$TEMP_FILE"

if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "200" ]; then
    # Try different field names for org ID
    ORG_ID=$(echo "$ORG_BODY" | grep -o '"orgId":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
    if [ -z "$ORG_ID" ]; then
        ORG_ID=$(echo "$ORG_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
    fi
    echo -e "${GREEN}✓ Organization created: $ORG_ID${NC}"
elif [ "$HTTP_CODE" = "401" ] || [ "$HTTP_CODE" = "403" ]; then
    echo -e "${YELLOW}⚠ Organization creation requires authentication${NC}"
    echo ""
    echo "To bootstrap an admin key, you need an existing admin key or user credentials."
    echo ""
    echo "Options:"
    echo "  1. If you have an admin API key, set it:"
    echo "     export ADMIN_API_KEY=your-existing-key"
    echo "     Then re-run this script"
    echo ""
    echo "  2. If you have user credentials, log in via web portal and create an API key"
    echo ""
    echo "  3. Use the seed command to create initial admin user:"
    echo "     cd services/user-org-service"
    echo "     go run cmd/seed/main.go -org-slug=e2e-admin -user-email=admin@e2e.test"
    echo ""
    
    # Check if ADMIN_API_KEY is already set in environment
    if [ -n "${ADMIN_API_KEY:-}" ]; then
        echo -e "${GREEN}Found ADMIN_API_KEY in environment, using it...${NC}"
        ADMIN_KEY="$ADMIN_API_KEY"
    else
        read -p "Do you have an existing admin API key to use? (y/N) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            read -p "Enter admin API key: " ADMIN_KEY
            export ADMIN_API_KEY="$ADMIN_KEY"
        else
            echo ""
            echo -e "${RED}✗ Cannot proceed without authentication${NC}"
            echo ""
            echo "Please either:"
            echo "  1. Set ADMIN_API_KEY environment variable with an existing admin key"
            echo "  2. Or create an admin user via seed command first"
            echo ""
            echo "Then re-run: make setup"
            exit 1
        fi
    fi
        
        # Retry with auth
        TEMP_FILE=$(mktemp)
        if [ -n "$CURL_HOST_HEADER" ]; then
            curl -s -k "$CURL_HOST_HEADER" \
                -X POST "$BASE_URL/v1/orgs" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $ADMIN_KEY" \
                -H "X-API-Key: $ADMIN_KEY" \
                -d "{\"name\":\"$ORG_NAME\",\"slug\":\"$ORG_SLUG\"}" \
                -w "\n%{http_code}" \
                -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
        else
            curl -s -k \
                -X POST "$BASE_URL/v1/orgs" \
                -H "Content-Type: application/json" \
                -H "Authorization: Bearer $ADMIN_KEY" \
                -H "X-API-Key: $ADMIN_KEY" \
                -d "{\"name\":\"$ORG_NAME\",\"slug\":\"$ORG_SLUG\"}" \
                -w "\n%{http_code}" \
                -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
        fi
        
        HTTP_CODE=$(tail -1 "$TEMP_FILE")
        ORG_BODY=$(head -n -1 "$TEMP_FILE" 2>/dev/null || sed '$d' "$TEMP_FILE" 2>/dev/null || cat "$TEMP_FILE")
        rm -f "$TEMP_FILE"
        
        if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "200" ]; then
            ORG_ID=$(echo "$ORG_BODY" | grep -o '"id":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
            echo -e "${GREEN}✓ Organization created: $ORG_ID${NC}"
        else
            echo -e "${RED}✗ Failed to create organization: $HTTP_CODE${NC}"
            echo "Response: $ORG_BODY"
            exit 1
        fi
    else
        echo -e "${RED}✗ Cannot proceed without authentication${NC}"
        exit 1
    fi
else
    echo -e "${RED}✗ Failed to create organization: $HTTP_CODE${NC}"
    echo "Response: $ORG_BODY"
    exit 1
fi

# Step 2: Create admin API key
echo ""
echo -e "${YELLOW}Step 2: Creating admin API key...${NC}"

TEMP_FILE=$(mktemp)
if [ -n "$CURL_HOST_HEADER" ]; then
    curl -s -k "$CURL_HOST_HEADER" \
        -X POST "$BASE_URL/v1/api-keys" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${ADMIN_API_KEY:-}" \
        -H "X-API-Key: ${ADMIN_API_KEY:-}" \
        -d "{\"name\":\"$API_KEY_NAME\",\"organization_id\":\"$ORG_ID\",\"scopes\":[\"admin\",\"inference:read\",\"inference:write\"]}" \
        -w "\n%{http_code}" \
        -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
else
    curl -s -k \
        -X POST "$BASE_URL/v1/api-keys" \
        -H "Content-Type: application/json" \
        -H "Authorization: Bearer ${ADMIN_API_KEY:-}" \
        -H "X-API-Key: ${ADMIN_API_KEY:-}" \
        -d "{\"name\":\"$API_KEY_NAME\",\"organization_id\":\"$ORG_ID\",\"scopes\":[\"admin\",\"inference:read\",\"inference:write\"]}" \
        -w "\n%{http_code}" \
        -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
fi

HTTP_CODE=$(tail -1 "$TEMP_FILE")
API_KEY_BODY=$(head -n -1 "$TEMP_FILE" 2>/dev/null || sed '$d' "$TEMP_FILE" 2>/dev/null || cat "$TEMP_FILE")
rm -f "$TEMP_FILE"

if [ "$HTTP_CODE" = "201" ] || [ "$HTTP_CODE" = "200" ]; then
    # Extract the API key (it's usually in the "key" field)
    NEW_API_KEY=$(echo "$API_KEY_BODY" | grep -o '"key":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
    
    if [ -z "$NEW_API_KEY" ]; then
        # Try alternative field names
        NEW_API_KEY=$(echo "$API_KEY_BODY" | grep -o '"secret":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
        NEW_API_KEY=$(echo "$API_KEY_BODY" | grep -o '"api_key":"[^"]*"' | head -1 | cut -d'"' -f4 || echo "")
    fi
    
    if [ -z "$NEW_API_KEY" ]; then
        echo -e "${YELLOW}⚠ Could not extract API key from response${NC}"
        echo "Response: $API_KEY_BODY"
        echo "Please extract the key manually and set ADMIN_API_KEY"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Admin API key created${NC}"
    
    # Save to file
    cat > "$OUTPUT_FILE" << EOF
# Admin API Key for E2E Tests
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
# Organization: $ORG_ID
# Key Name: $API_KEY_NAME

export ADMIN_API_KEY="$NEW_API_KEY"
export E2E_TEST_ORG_ID="$ORG_ID"
EOF
    
    echo ""
    echo -e "${GREEN}✓ Admin key saved to: $OUTPUT_FILE${NC}"
    echo ""
    echo "To use it, run:"
    echo "  source $OUTPUT_FILE"
    echo "  make test-dev-ip"
    echo ""
    echo -e "${YELLOW}⚠ Keep this key secure and don't commit it to version control!${NC}"
    
else
    echo -e "${RED}✗ Failed to create API key: $HTTP_CODE${NC}"
    echo "Response: $API_KEY_BODY"
    exit 1
fi

