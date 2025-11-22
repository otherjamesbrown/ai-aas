#!/bin/bash
# Script to run e2e tests against remote development environment via public internet
# Uses ingress URLs instead of port-forwarding

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Default public URLs (can be overridden via environment variables)
# These assume ingress is configured and accessible
USER_ORG_URL="${USER_ORG_SERVICE_URL:-https://user-org.api.ai-aas.dev}"
API_ROUTER_URL="${API_ROUTER_SERVICE_URL:-https://router.api.ai-aas.dev}"
ANALYTICS_URL="${ANALYTICS_SERVICE_URL:-https://analytics.api.ai-aas.dev}"

# Check if URLs are accessible
check_service_connectivity() {
    echo -e "${YELLOW}Checking service connectivity...${NC}"
    
    local services=(
        "$USER_ORG_URL"
        "$API_ROUTER_URL"
        "$ANALYTICS_URL"
    )
    
    local failed=0
    
    for url in "${services[@]}"; do
        # Extract hostname from URL
        local hostname=$(echo "$url" | sed -E 's|https?://([^/]+).*|\1|')
        
        echo -n "  Checking $hostname... "
        
        # Try to connect (with timeout)
        if curl -s --max-time 5 --head "$url" > /dev/null 2>&1 || \
           curl -s --max-time 5 -k "$url/health" > /dev/null 2>&1 || \
           curl -s --max-time 5 -k "$url/v1/health" > /dev/null 2>&1; then
            echo -e "${GREEN}✓${NC}"
        else
            echo -e "${YELLOW}⚠ (may not be accessible or health endpoint not available)${NC}"
            failed=$((failed + 1))
        fi
    done
    
    if [ $failed -eq ${#services[@]} ]; then
        echo -e "${RED}Error: None of the services appear to be accessible${NC}"
        echo "Please verify:"
        echo "  1. Ingress is enabled for the services"
        echo "  2. DNS is configured correctly"
        echo "  3. Services are running in the cluster"
        echo "  4. URLs are correct (check with: kubectl get ingress -n development)"
        echo ""
        echo "You can override URLs with environment variables:"
        echo "  export USER_ORG_SERVICE_URL=https://your-user-org-url"
        echo "  export API_ROUTER_SERVICE_URL=https://your-router-url"
        echo "  export ANALYTICS_SERVICE_URL=https://your-analytics-url"
        echo ""
        read -p "Continue anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            exit 1
        fi
    else
        echo -e "${GREEN}✓ Service connectivity check passed${NC}"
    fi
}

# Get ingress URLs from cluster (if kubectl is available)
get_ingress_urls() {
    if ! command -v kubectl &> /dev/null; then
        return
    fi
    
    local namespace="${NAMESPACE:-development}"
    local kubeconfig="${KUBECONFIG:-${HOME}/kubeconfigs/kubeconfig-development.yaml}"
    
    if [ -f "$kubeconfig" ]; then
        export KUBECONFIG="$kubeconfig"
    fi
    
    echo -e "${YELLOW}Attempting to discover ingress URLs from cluster...${NC}"
    
    # Try to get ingress URLs
    if kubectl get ingress -n "$namespace" &> /dev/null; then
        echo "Found ingress resources:"
        kubectl get ingress -n "$namespace" -o custom-columns=NAME:.metadata.name,HOSTS:.spec.rules[*].host --no-headers 2>/dev/null || true
        
        # Try to extract URLs (this is a best-effort attempt)
        local user_org_ingress=$(kubectl get ingress -n "$namespace" -o jsonpath='{.items[?(@.metadata.name=="user-org-service")].spec.rules[0].host}' 2>/dev/null || echo "")
        local router_ingress=$(kubectl get ingress -n "$namespace" -o jsonpath='{.items[?(@.metadata.name=="api-router-service")].spec.rules[0].host}' 2>/dev/null || echo "")
        local analytics_ingress=$(kubectl get ingress -n "$namespace" -o jsonpath='{.items[?(@.metadata.name=="analytics-service")].spec.rules[0].host}' 2>/dev/null || echo "")
        
        if [ -n "$user_org_ingress" ]; then
            USER_ORG_URL="https://$user_org_ingress"
            echo "  Using user-org-service: $USER_ORG_URL"
        fi
        
        if [ -n "$router_ingress" ]; then
            API_ROUTER_URL="https://$router_ingress"
            echo "  Using api-router-service: $API_ROUTER_URL"
        fi
        
        if [ -n "$analytics_ingress" ]; then
            ANALYTICS_URL="https://$analytics_ingress"
            echo "  Using analytics-service: $ANALYTICS_URL"
        fi
    else
        echo "  No ingress resources found or cannot access cluster"
    fi
}

# Main execution
main() {
    echo -e "${GREEN}=== Running E2E Tests Against Development Environment (Internet) ===${NC}"
    echo ""
    
    # Try to discover URLs from cluster
    get_ingress_urls
    
    echo ""
    echo "Using service URLs:"
    echo "  - user-org-service: $USER_ORG_URL"
    echo "  - api-router-service: $API_ROUTER_URL"
    echo "  - analytics-service: $ANALYTICS_URL"
    echo ""
    
    # Check connectivity (non-blocking)
    check_service_connectivity
    
    # Set environment variables for tests
    export TEST_ENV=development
    export USER_ORG_SERVICE_URL="$USER_ORG_URL"
    export API_ROUTER_SERVICE_URL="$API_ROUTER_URL"
    export ANALYTICS_SERVICE_URL="$ANALYTICS_URL"
    
    # Check if ADMIN_API_KEY is set
    if [ -z "${ADMIN_API_KEY:-}" ]; then
        echo -e "${YELLOW}Warning: ADMIN_API_KEY not set${NC}"
        echo "Some tests may fail without admin credentials"
        echo "Set ADMIN_API_KEY environment variable to provide admin access"
    else
        export ADMIN_API_KEY
    fi
    
    echo ""
    echo -e "${GREEN}Running tests...${NC}"
    echo ""
    
    # Change to e2e directory and run tests
    cd "$E2E_DIR"
    go test -v ./suites/... -timeout 30m "$@"
    
    TEST_EXIT_CODE=$?
    
    if [ $TEST_EXIT_CODE -eq 0 ]; then
        echo ""
        echo -e "${GREEN}✓ All tests passed${NC}"
    else
        echo ""
        echo -e "${RED}✗ Some tests failed${NC}"
    fi
    
    exit $TEST_EXIT_CODE
}

# Run main function
main "$@"

