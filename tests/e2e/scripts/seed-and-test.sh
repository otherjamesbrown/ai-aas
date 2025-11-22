#!/bin/bash
# Complete workflow: Seed admin user, get OAuth token, save it, and run tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_ROOT="$(cd "$E2E_DIR/../.." && pwd)"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# SSH helper for remote admin-cli execution
ssh_exec() {
    local host="$1"
    shift
    local cmd="$*"
    
    if [[ -z "${SSH_KEY:-}" ]]; then
        ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "${host}" "${cmd}" 2>&1
    else
        ssh -i "${SSH_KEY}" -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null "${host}" "${cmd}" 2>&1
    fi
}

# Execute admin-cli command (local or remote)
run_admin_cli() {
    local cmd="$*"
    
    if [[ "$ADMIN_CLI_PATH" =~ ^/home/dev/ ]]; then
        # Remote path - extract host and path
        # Assume default dev server user/host or use SSH_HOST env var
        local ssh_host="${SSH_HOST:-dev@172.232.58.222}"
        echo -e "${YELLOW}Running admin-cli remotely on ${ssh_host}...${NC}"
        ssh_exec "$ssh_host" "cd /home/dev/ai-aas && $ADMIN_CLI_PATH $cmd"
    else
        # Local path
        "$ADMIN_CLI_PATH" $cmd 2>&1
    fi
}

echo -e "${GREEN}=== Seeding Admin User and Running Tests ===${NC}"
echo ""

# Step 1: Seed admin user (if DATABASE_URL is available or admin-cli exists)
echo -e "${YELLOW}Step 1: Seeding admin user...${NC}"

# Check if admin-cli is available (local or remote)
ADMIN_CLI_PATH=""
SSH_HOST="${SSH_HOST:-dev@172.232.58.222}"

if command -v admin-cli >/dev/null 2>&1; then
    ADMIN_CLI_PATH="admin-cli"
    echo -e "${GREEN}✓ Found admin-cli in PATH${NC}"
elif [ -f "$REPO_ROOT/services/admin-cli/bin/admin-cli" ]; then
    ADMIN_CLI_PATH="$REPO_ROOT/services/admin-cli/bin/admin-cli"
    echo -e "${GREEN}✓ Found admin-cli locally at $ADMIN_CLI_PATH${NC}"
else
    # Check if admin-cli exists on remote dev server
    REMOTE_CLI_PATH="/home/dev/ai-aas/services/admin-cli/bin/admin-cli"
    if ssh_exec "$SSH_HOST" "test -f $REMOTE_CLI_PATH" >/dev/null 2>&1; then
        ADMIN_CLI_PATH="$REMOTE_CLI_PATH"
        echo -e "${GREEN}✓ Found admin-cli on remote server at $ADMIN_CLI_PATH${NC}"
    else
        echo -e "${YELLOW}⚠ admin-cli not found (checked local PATH, $REPO_ROOT/services/admin-cli/bin/admin-cli, and $SSH_HOST:$REMOTE_CLI_PATH)${NC}"
    fi
fi

# Initialize variables
SEED_OUTPUT=""
PASSWORD=""
ORG_ID=""

if [ -n "${DATABASE_URL:-}" ]; then
    # Use seed command with DATABASE_URL
    cd "$REPO_ROOT/services/user-org-service"
    
    # Run seed command
    SEED_OUTPUT=$(go run cmd/seed/main.go \
        -org-slug=e2e-test-admin \
        -org-name="E2E Test Admin Org" \
        -user-email=admin@e2e.test \
        -user-name="E2E Admin User" 2>&1)
    
    echo "$SEED_OUTPUT"
    
    # Extract password from output
    PASSWORD=$(echo "$SEED_OUTPUT" | grep -i "generated password" | sed 's/.*: *//' | tr -d '[:space:]' || echo "")
    
    if [ -z "$PASSWORD" ]; then
        PASSWORD=$(echo "$SEED_OUTPUT" | grep -i "password" | grep -oE '[a-z]+[0-9]+!' | head -1 || echo "")
    fi
    
    if [ -z "$PASSWORD" ]; then
        echo -e "${YELLOW}⚠ Could not extract password from seed output${NC}"
        read -p "Enter the generated password: " PASSWORD
    fi
    
    # Extract org ID from seed output
    ORG_ID=$(echo "$SEED_OUTPUT" | grep -i "organization" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1 || echo "")
    
    echo -e "${GREEN}✓ Admin user seeded${NC}"
    echo "  Email: admin@e2e.test"
    echo "  Password: [hidden]"
    if [ -n "$ORG_ID" ]; then
        echo "  Org ID: $ORG_ID"
    fi
elif [ -n "$ADMIN_CLI_PATH" ]; then
    # Try admin-cli bootstrap if available
    echo -e "${YELLOW}Trying admin-cli bootstrap...${NC}"
    if run_admin_cli bootstrap --dry-run >/dev/null 2>&1; then
        echo -e "${GREEN}✓ Admin CLI bootstrap available${NC}"
        # Run bootstrap (this should create first admin)
        BOOTSTRAP_OUTPUT=$(run_admin_cli bootstrap --org-slug=e2e-test-admin --user-email=admin@e2e.test)
        echo "$BOOTSTRAP_OUTPUT"
        
        # Extract credentials from bootstrap output
        PASSWORD=$(echo "$BOOTSTRAP_OUTPUT" | grep -i "password" | sed 's/.*: *//' | tr -d '[:space:]' | head -1 || echo "")
        ORG_ID=$(echo "$BOOTSTRAP_OUTPUT" | grep -i "organization\|org" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1 || echo "")
        
        if [ -n "$PASSWORD" ]; then
            echo -e "${GREEN}✓ Admin user bootstrapped via admin-cli${NC}"
            SEED_OUTPUT="$BOOTSTRAP_OUTPUT"
        else
            echo -e "${YELLOW}⚠ Could not extract password from bootstrap output${NC}"
            SEED_OUTPUT=""
            PASSWORD=""
            ORG_ID=""
        fi
    else
        echo -e "${YELLOW}⚠ Admin CLI bootstrap not available or failed${NC}"
        SEED_OUTPUT=""
        PASSWORD=""
        ORG_ID=""
    fi
else
    # Try to port-forward to remote database if kubectl is available
    if command -v kubectl >/dev/null 2>&1; then
        KUBECONFIG="${KUBECONFIG:-$HOME/kubeconfigs/kubeconfig-development.yaml}"
        if [ -f "$KUBECONFIG" ]; then
            echo "Attempting to port-forward to remote database..."
            export KUBECONFIG
            
            # Try to find postgres service in development namespace
            PG_SVC=$(kubectl get svc -n development -o name | grep -i postgres | head -1 || echo "")
            if [ -n "$PG_SVC" ]; then
                echo "Found postgres service: $PG_SVC"
                echo "Starting port-forward in background..."
                
                # Kill any existing port-forward on 5432
                lsof -ti:5432 | xargs kill -9 2>/dev/null || true
                
                # Start port-forward in background
                kubectl port-forward -n development "$PG_SVC" 5432:5432 >/dev/null 2>&1 &
                PF_PID=$!
                sleep 2
                
                # Check if port-forward is working
                if kill -0 $PF_PID 2>/dev/null; then
                    echo -e "${GREEN}✓ Port-forward established${NC}"
                    # Try to construct DATABASE_URL (may need actual credentials)
                    # For now, use a default that might work
                    export DATABASE_URL="postgres://ai_aas:dev_password@localhost:5432/ai_aas?sslmode=disable"
                    echo "Using: $DATABASE_URL"
                    echo "Note: You may need to adjust credentials"
                    PORT_FORWARD_PID=$PF_PID
                    # Retry with DATABASE_URL now set
                    cd "$REPO_ROOT/services/user-org-service"
                    SEED_OUTPUT=$(go run cmd/seed/main.go \
                        -org-slug=e2e-test-admin \
                        -org-name="E2E Test Admin Org" \
                        -user-email=admin@e2e.test \
                        -user-name="E2E Admin User" 2>&1)
                    echo "$SEED_OUTPUT"
                    PASSWORD=$(echo "$SEED_OUTPUT" | grep -i "generated password" | sed 's/.*: *//' | tr -d '[:space:]' || echo "")
                    ORG_ID=$(echo "$SEED_OUTPUT" | grep -i "organization" | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' | head -1 || echo "")
                else
                    echo -e "${YELLOW}⚠ Port-forward failed${NC}"
                fi
            else
                echo -e "${YELLOW}⚠ Postgres service not found in development namespace${NC}"
            fi
        else
            echo -e "${YELLOW}⚠ Kubeconfig not found: $KUBECONFIG${NC}"
        fi
    else
        echo -e "${YELLOW}⚠ kubectl not found${NC}"
    fi
    
    if [ -z "${DATABASE_URL:-}" ] && [ -z "$PASSWORD" ]; then
        echo ""
        echo "Proceeding with existing credentials or API-based setup..."
    fi
fi

# Clean up port-forward if we started one
if [ -n "${PORT_FORWARD_PID:-}" ]; then
    trap "kill $PORT_FORWARD_PID 2>/dev/null || true" EXIT
fi

echo ""

# Step 2: Get OAuth token via login or use existing admin key
echo -e "${YELLOW}Step 2: Getting authentication credentials...${NC}"

BASE_URL="${USER_ORG_SERVICE_URL:-https://172.232.58.222}"
CURL_HOST_HEADER=""
if [[ "$BASE_URL" =~ ^https?://[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+ ]]; then
    CURL_HOST_HEADER="-H Host:api.dev.ai-aas.local"
fi

# Check if we have a password from seed, or use existing admin key
if [ -n "$PASSWORD" ] && [ -n "${PASSWORD:-}" ]; then
    # Login to get OAuth token
    TEMP_FILE=$(mktemp)
    if [ -n "$CURL_HOST_HEADER" ]; then
        curl -s -k "$CURL_HOST_HEADER" \
            -X POST "$BASE_URL/v1/auth/login" \
            -H "Content-Type: application/json" \
            -d "{\"email\":\"admin@e2e.test\",\"password\":\"$PASSWORD\"}" \
            -w "\n%{http_code}" \
            -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
    else
        curl -s -k \
            -X POST "$BASE_URL/v1/auth/login" \
            -H "Content-Type: application/json" \
            -d "{\"email\":\"admin@e2e.test\",\"password\":\"$PASSWORD\"}" \
            -w "\n%{http_code}" \
            -o "$TEMP_FILE" 2>&1 || echo "ERROR" > "$TEMP_FILE"
    fi
    
    HTTP_CODE=$(tail -1 "$TEMP_FILE")
    AUTH_BODY=$(head -n -1 "$TEMP_FILE" 2>/dev/null || sed '$d' "$TEMP_FILE" 2>/dev/null || cat "$TEMP_FILE")
    rm -f "$TEMP_FILE"
    
    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
        # Extract access_token from OAuth response
        ACCESS_TOKEN=$(echo "$AUTH_BODY" | grep -o '"access_token":"[^"]*"' | cut -d'"' -f4 || echo "")
        
        if [ -z "$ACCESS_TOKEN" ]; then
            ACCESS_TOKEN=$(echo "$AUTH_BODY" | grep -o 'access_token[^,}]*' | cut -d'"' -f4 || echo "")
        fi
        
        if [ -n "$ACCESS_TOKEN" ]; then
            echo -e "${GREEN}✓ OAuth access token obtained${NC}"
            ADMIN_API_KEY="$ACCESS_TOKEN"
        else
            echo -e "${RED}✗ Could not extract access_token from login response${NC}"
            echo "Response: $AUTH_BODY"
            exit 1
        fi
    else
        echo -e "${RED}✗ Login failed${NC}"
        echo "HTTP Code: $HTTP_CODE"
        echo "Response: $AUTH_BODY"
        exit 1
    fi
elif [ -f "$E2E_DIR/.admin-key.env" ]; then
    # Use existing admin key
    echo -e "${GREEN}✓ Using existing admin key from .admin-key.env${NC}"
    source "$E2E_DIR/.admin-key.env"
    if [ -z "${ADMIN_API_KEY:-}" ]; then
        echo -e "${RED}✗ ADMIN_API_KEY not found in .admin-key.env${NC}"
        exit 1
    fi
elif [ -n "${ADMIN_API_KEY:-}" ]; then
    # Use environment variable
    echo -e "${GREEN}✓ Using ADMIN_API_KEY from environment${NC}"
else
    # Try admin-cli to create API key if available
    if [ -n "$ADMIN_CLI_PATH" ] && [ -n "$ORG_ID" ]; then
        echo -e "${YELLOW}Trying admin-cli to create API key...${NC}"
        # Use admin-cli to create API key
        API_KEY_OUTPUT=$(run_admin_cli apikeys create \
            --org-id="$ORG_ID" \
            --name="e2e-test-admin-key" \
            --scopes="admin,inference:read,inference:write")
        
        if [ $? -eq 0 ]; then
            ADMIN_API_KEY=$(echo "$API_KEY_OUTPUT" | grep -oE 'key_[a-zA-Z0-9_-]+' | head -1 || echo "")
            if [ -n "$ADMIN_API_KEY" ]; then
                echo -e "${GREEN}✓ API key created via admin-cli${NC}"
            else
                # Try alternative extraction
                ADMIN_API_KEY=$(echo "$API_KEY_OUTPUT" | grep -i "secret\|key" | sed 's/.*: *//' | tr -d '[:space:]' | head -1 || echo "")
            fi
        fi
    fi
    
    # Fall back to bootstrap script if admin-cli didn't work
    if [ -z "${ADMIN_API_KEY:-}" ]; then
        echo -e "${YELLOW}No credentials found, trying bootstrap script...${NC}"
        cd "$E2E_DIR"
        
        # Check if user wants to provide an existing admin key
        if [ -z "${ADMIN_API_KEY:-}" ]; then
            echo ""
            echo "To proceed, you need an admin API key. Options:"
            echo "  1. Provide an existing admin key (recommended for remote dev):"
            echo "     export ADMIN_API_KEY=your-existing-key"
            echo "     Then re-run this script"
            echo ""
            if [ -n "$ADMIN_CLI_PATH" ]; then
                echo "  2. Use admin-cli to bootstrap (if not already done):"
                echo "     $ADMIN_CLI_PATH bootstrap"
                echo ""
            fi
            echo "  3. Seed a user in the database (requires DATABASE_URL):"
            echo "     export DATABASE_URL=postgres://user:pass@host:port/db"
            echo "     Then re-run this script"
            echo ""
            read -p "Do you have an admin API key to provide now? (y/N) " -n 1 -r
            echo
            if [[ $REPLY =~ ^[Yy]$ ]]; then
                read -p "Enter admin API key: " ADMIN_API_KEY
                export ADMIN_API_KEY
            else
                echo -e "${RED}✗ Cannot proceed without admin key${NC}"
                exit 1
            fi
        fi
        
        # Now try bootstrap with the provided key
        export ADMIN_API_KEY
        if "$SCRIPT_DIR/bootstrap-admin-key.sh"; then
            source "$E2E_DIR/.admin-key.env"
            if [ -z "${ADMIN_API_KEY:-}" ]; then
                echo -e "${RED}✗ Bootstrap failed to create admin key${NC}"
                exit 1
            fi
            echo -e "${GREEN}✓ Admin key ready${NC}"
        else
            echo -e "${RED}✗ Bootstrap failed${NC}"
            exit 1
        fi
    fi
fi

# Step 3: Save to .admin-key.env
echo ""
echo -e "${YELLOW}Step 3: Saving credentials...${NC}"
cd "$E2E_DIR"

cat > .admin-key.env << EOF
# Admin API Key for E2E Tests (OAuth Access Token)
# Generated: $(date -u +"%Y-%m-%d %H:%M:%S UTC")
# Organization: ${ORG_ID:-unknown}
# User: admin@e2e.test
# Note: This is an OAuth access token, not a permanent API key

export ADMIN_API_KEY="$ADMIN_API_KEY"
export E2E_TEST_ORG_ID="${ORG_ID:-}"
EOF

echo -e "${GREEN}✓ Credentials saved to .admin-key.env${NC}"
echo ""

# Step 4: Run tests
echo -e "${YELLOW}Step 4: Running e2e tests...${NC}"
echo ""

source .admin-key.env
export TEST_ENV=development
export USER_ORG_SERVICE_URL="${USER_ORG_SERVICE_URL:-https://172.232.58.222}"
export API_ROUTER_SERVICE_URL="${API_ROUTER_SERVICE_URL:-https://172.232.58.222}"
export ANALYTICS_SERVICE_URL="${ANALYTICS_SERVICE_URL:-https://172.232.58.222}"

cd "$E2E_DIR"
go test -v ./suites -timeout 30m

TEST_EXIT_CODE=$?

if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo ""
    echo -e "${GREEN}✓ All tests passed!${NC}"
else
    echo ""
    echo -e "${RED}✗ Some tests failed${NC}"
fi

exit $TEST_EXIT_CODE

