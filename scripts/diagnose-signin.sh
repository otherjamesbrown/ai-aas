#!/bin/bash
# Diagnostic script for signin button issues
# Checks logs, service health, and tests login endpoint

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "=== Signin Button Diagnostic ==="
echo ""

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Check 1: Service Health
echo -e "${GREEN}[1] Checking user-org-service health...${NC}"
if curl -s -f http://localhost:8081/healthz > /dev/null 2>&1; then
    echo "✓ Service is responding on http://localhost:8081"
else
    echo -e "${RED}✗ Service not responding on http://localhost:8081${NC}"
    echo "  Checking alternative ports..."
    
    # Check if running on different port
    for port in 8080 8082 8443; do
        if curl -s -f http://localhost:${port}/healthz > /dev/null 2>&1; then
            echo "  ✓ Found service on port ${port}"
        fi
    done
fi
echo ""

# Check 2: Test Login Endpoint
echo -e "${GREEN}[2] Testing login endpoint...${NC}"
LOGIN_URL="${VITE_USER_ORG_SERVICE_URL:-http://localhost:8081}/v1/auth/login"
echo "  Testing: ${LOGIN_URL}"

RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${LOGIN_URL}" \
    -H "Content-Type: application/json" \
    -d '{"email":"test@example.com","password":"test"}' 2>&1 || echo "CONNECTION_ERROR")

HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | head -n-1)

if [[ "$HTTP_CODE" == "CONNECTION_ERROR" ]] || [[ "$HTTP_CODE" == "000" ]]; then
    echo -e "${RED}✗ Cannot connect to login endpoint${NC}"
    echo "  This suggests the service is not running or not accessible"
else
    echo "  HTTP Status: ${HTTP_CODE}"
    if [[ "$HTTP_CODE" == "401" ]] || [[ "$HTTP_CODE" == "400" ]]; then
        echo "  ✓ Endpoint is responding (expected error for invalid credentials)"
    elif [[ "$HTTP_CODE" == "200" ]]; then
        echo -e "${YELLOW}  ⚠ Endpoint returned 200 (unexpected for test credentials)${NC}"
    else
        echo -e "${RED}  ✗ Unexpected status code${NC}"
    fi
    echo "  Response: ${BODY:0:200}"
fi
echo ""

# Check 3: Check for running services
echo -e "${GREEN}[3] Checking for running services...${NC}"
if command -v docker >/dev/null 2>&1; then
    echo "  Docker containers:"
    docker ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" | grep -E "(user-org|portal|web)" || echo "    No relevant containers found"
fi

if command -v kubectl >/dev/null 2>&1; then
    echo ""
    echo "  Kubernetes pods (development):"
    if kubectl --kubeconfig=~/kubeconfigs/kubeconfig-development.yaml get pods -A 2>/dev/null | grep -E "(user-org|portal)" | head -5; then
        echo "    Found pods in development cluster"
    else
        echo "    No pods found or cluster not accessible"
    fi
    
    echo ""
    echo "  Kubernetes pods (production):"
    if kubectl --kubeconfig=~/kubeconfigs/kubeconfig-production.yaml get pods -A 2>/dev/null | grep -E "(user-org|portal)" | head -5; then
        echo "    Found pods in production cluster"
    else
        echo "    No pods found or cluster not accessible"
    fi
fi
echo ""

# Check 4: Recent logs (if available)
echo -e "${GREEN}[4] Checking recent logs...${NC}"
if [[ -d "${PROJECT_ROOT}/.dev/compose" ]]; then
    echo "  Attempting to get logs from docker compose..."
    cd "${PROJECT_ROOT}"
    if docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml ps 2>/dev/null | grep -q "user-org"; then
        echo "  Recent user-org-service logs:"
        docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml logs --tail=20 user-org-service 2>/dev/null | grep -i "login\|error\|auth" | tail -10 || echo "    No relevant logs found"
    fi
fi
echo ""

# Check 5: Environment variables
echo -e "${GREEN}[5] Checking environment configuration...${NC}"
echo "  VITE_USER_ORG_SERVICE_URL: ${VITE_USER_ORG_SERVICE_URL:-http://localhost:8081 (default)}"
echo "  VITE_OAUTH_CLIENT_ID: ${VITE_OAUTH_CLIENT_ID:-not set}"
echo "  VITE_OAUTH_ISSUER_URL: ${VITE_OAUTH_ISSUER_URL:-not set}"
echo ""

# Recommendations
echo -e "${YELLOW}=== Recommendations ===${NC}"
echo ""
echo "1. Check browser console (F12) for JavaScript errors"
echo "2. Check Network tab for failed requests to /v1/auth/login"
echo "3. Verify the service is running:"
echo "   - Local: make up"
echo "   - Kubernetes: kubectl get pods -n user-org-service"
echo ""
echo "4. To view logs:"
echo "   - Local: make logs-tail SERVICE=user-org-service"
echo "   - Kubernetes: kubectl logs -l app=user-org-service -n user-org-service --tail=50"
echo ""
echo "5. Test login endpoint manually:"
echo "   curl -X POST http://localhost:8081/v1/auth/login \\"
echo "     -H 'Content-Type: application/json' \\"
echo "     -d '{\"email\":\"admin@example-acme.com\",\"password\":\"AcmeAdmin2024!Secure\"}'"
echo ""

