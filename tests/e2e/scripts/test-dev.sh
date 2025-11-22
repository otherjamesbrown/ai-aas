#!/bin/bash
# Script to run e2e tests against remote development environment
# Sets up port-forwarding and runs tests

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
NAMESPACE="${NAMESPACE:-development}"
KUBECONFIG="${KUBECONFIG:-${HOME}/kubeconfigs/kubeconfig-development.yaml}"

# Service ports (local:remote)
USER_ORG_PORT="${USER_ORG_PORT:-8081:8081}"
API_ROUTER_PORT="${API_ROUTER_PORT:-8082:8080}"
ANALYTICS_PORT="${ANALYTICS_PORT:-8083:8083}"

# Check prerequisites
check_prerequisites() {
    echo -e "${YELLOW}Checking prerequisites...${NC}"
    
    if ! command -v kubectl &> /dev/null; then
        echo -e "${RED}Error: kubectl is not installed${NC}"
        exit 1
    fi
    
    if [ ! -f "$KUBECONFIG" ]; then
        echo -e "${YELLOW}Warning: KUBECONFIG not found at $KUBECONFIG${NC}"
        echo "Using default kubectl config"
        KUBECONFIG=""
    else
        export KUBECONFIG
    fi
    
    # Check if we can access the cluster
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        echo -e "${RED}Error: Cannot access namespace '$NAMESPACE'${NC}"
        echo "Please check your kubeconfig and cluster access"
        exit 1
    fi
    
    echo -e "${GREEN}✓ Prerequisites check passed${NC}"
}

# Start port-forwarding
start_port_forwards() {
    echo -e "${YELLOW}Starting port-forwarding...${NC}"
    
    # Kill any existing port-forwards
    pkill -f "kubectl.*port-forward.*user-org-service" || true
    pkill -f "kubectl.*port-forward.*api-router-service" || true
    pkill -f "kubectl.*port-forward.*analytics-service" || true
    
    # Start port-forwards in background
    kubectl port-forward -n "$NAMESPACE" svc/user-org-service $USER_ORG_PORT > /dev/null 2>&1 &
    USER_ORG_PID=$!
    
    kubectl port-forward -n "$NAMESPACE" svc/api-router-service $API_ROUTER_PORT > /dev/null 2>&1 &
    API_ROUTER_PID=$!
    
    kubectl port-forward -n "$NAMESPACE" svc/analytics-service $ANALYTICS_PORT > /dev/null 2>&1 &
    ANALYTICS_PID=$!
    
    # Wait for port-forwards to be ready
    echo "Waiting for port-forwards to be ready..."
    sleep 3
    
    # Verify port-forwards are working
    if ! kill -0 $USER_ORG_PID 2>/dev/null; then
        echo -e "${RED}Error: Failed to start user-org-service port-forward${NC}"
        exit 1
    fi
    
    if ! kill -0 $API_ROUTER_PID 2>/dev/null; then
        echo -e "${RED}Error: Failed to start api-router-service port-forward${NC}"
        kill $USER_ORG_PID 2>/dev/null || true
        exit 1
    fi
    
    if ! kill -0 $ANALYTICS_PID 2>/dev/null; then
        echo -e "${RED}Error: Failed to start analytics-service port-forward${NC}"
        kill $USER_ORG_PID $API_ROUTER_PID 2>/dev/null || true
        exit 1
    fi
    
    echo -e "${GREEN}✓ Port-forwards started${NC}"
    echo "  - user-org-service: localhost:${USER_ORG_PORT%%:*}"
    echo "  - api-router-service: localhost:${API_ROUTER_PORT%%:*}"
    echo "  - analytics-service: localhost:${ANALYTICS_PORT%%:*}"
}

# Stop port-forwarding
stop_port_forwards() {
    echo -e "${YELLOW}Stopping port-forwards...${NC}"
    kill $USER_ORG_PID $API_ROUTER_PID $ANALYTICS_PID 2>/dev/null || true
    pkill -f "kubectl.*port-forward.*user-org-service" || true
    pkill -f "kubectl.*port-forward.*api-router-service" || true
    pkill -f "kubectl.*port-forward.*analytics-service" || true
    echo -e "${GREEN}✓ Port-forwards stopped${NC}"
}

# Cleanup on exit
cleanup() {
    stop_port_forwards
}
trap cleanup EXIT INT TERM

# Main execution
main() {
    echo -e "${GREEN}=== Running E2E Tests Against Development Environment ===${NC}"
    echo ""
    
    check_prerequisites
    start_port_forwards
    
    # Set environment variables for tests
    export TEST_ENV=development
    export USER_ORG_SERVICE_URL="http://localhost:${USER_ORG_PORT%%:*}"
    export API_ROUTER_SERVICE_URL="http://localhost:${API_ROUTER_PORT%%:*}"
    export ANALYTICS_SERVICE_URL="http://localhost:${ANALYTICS_PORT%%:*}"
    
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

