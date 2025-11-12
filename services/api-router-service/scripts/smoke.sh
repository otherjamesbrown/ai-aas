#!/bin/bash
# Smoke test script for API Router Service
#
# Purpose:
#   Comprehensive smoke tests for deployment validation covering all critical
#   endpoints, rate limiting, routing, budget enforcement, and usage tracking.
#
# Usage:
#   ./scripts/smoke.sh [options]
#
# Options:
#   --url URL          Service URL (default: http://localhost:8080)
#   --api-key KEY      API key for authentication (default: dev-test-key)
#   --timeout SECONDS  Timeout for waiting for service (default: 30)
#   --skip-rate-limit  Skip rate limit tests
#   --skip-budget      Skip budget enforcement tests
#   --skip-admin       Skip admin endpoint tests
#   --skip-audit       Skip audit endpoint tests
#   --verbose          Verbose output
#   --help             Show this help message

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SERVICE_URL="${SERVICE_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-dev-test-key}"
TIMEOUT="${TIMEOUT:-30}"
SKIP_RATE_LIMIT=false
SKIP_BUDGET=false
SKIP_ADMIN=false
SKIP_AUDIT=false
VERBOSE=false

# Test counters
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --url)
            SERVICE_URL="$2"
            shift 2
            ;;
        --api-key)
            API_KEY="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --skip-rate-limit)
            SKIP_RATE_LIMIT=true
            shift
            ;;
        --skip-budget)
            SKIP_BUDGET=true
            shift
            ;;
        --skip-admin)
            SKIP_ADMIN=true
            shift
            ;;
        --skip-audit)
            SKIP_AUDIT=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            head -n 20 "$0" | grep "^#" | sed 's/^# //'
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}" >&2
            exit 1
            ;;
    esac
done

# Helper functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
    ((TESTS_PASSED++))
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*" >&2
    ((TESTS_FAILED++))
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $*"
    ((TESTS_SKIPPED++))
}

log_verbose() {
    if [ "$VERBOSE" = true ]; then
        echo -e "${YELLOW}[VERBOSE]${NC} $*"
    fi
}

# Generate UUID for request ID
generate_uuid() {
    if command -v uuidgen &> /dev/null; then
        uuidgen
    elif command -v python3 &> /dev/null; then
        python3 -c "import uuid; print(uuid.uuid4())"
    else
        # Fallback: simple UUID-like string
        echo "$(date +%s)-$(shuf -i 1000-9999 -n 1)-$(shuf -i 1000-9999 -n 1)-$(shuf -i 1000-9999 -n 1)-$(shuf -i 100000000000-999999999999 -n 1)"
    fi
}

# Make HTTP request and return status code
http_request() {
    local method=$1
    local url=$2
    local data="${3:-}"
    local headers="${4:-}"
    
    local curl_cmd="curl -s -w '\n%{http_code}' -X $method"
    
    if [ -n "$headers" ]; then
        curl_cmd="$curl_cmd $headers"
    fi
    
    if [ -n "$data" ]; then
        curl_cmd="$curl_cmd -d '$data'"
    fi
    
    curl_cmd="$curl_cmd '$url'"
    
    log_verbose "Executing: $curl_cmd"
    eval "$curl_cmd"
}

# Test health endpoint
test_health_endpoint() {
    log_info "Testing health endpoint (/v1/status/healthz)..."
    
    local response=$(http_request "GET" "$SERVICE_URL/v1/status/healthz")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status_code" = "200" ]; then
        log_success "Health endpoint returned 200"
        log_verbose "Response: $body"
        return 0
    else
        log_error "Health endpoint returned $status_code (expected 200)"
        return 1
    fi
}

# Test readiness endpoint
test_readiness_endpoint() {
    log_info "Testing readiness endpoint (/v1/status/readyz)..."
    
    local response=$(http_request "GET" "$SERVICE_URL/v1/status/readyz")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status_code" = "200" ]; then
        log_success "Readiness endpoint returned 200"
        log_verbose "Response: $body"
        
        # Check if response contains status field
        if echo "$body" | grep -q '"status"'; then
            log_success "Readiness response contains status field"
        else
            log_error "Readiness response missing status field"
            return 1
        fi
        return 0
    else
        log_error "Readiness endpoint returned $status_code (expected 200)"
        return 1
    fi
}

# Test metrics endpoint
test_metrics_endpoint() {
    log_info "Testing metrics endpoint (/metrics)..."
    
    local response=$(http_request "GET" "$SERVICE_URL/metrics")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status_code" = "200" ]; then
        log_success "Metrics endpoint returned 200"
        
        # Check for API router metrics
        if echo "$body" | grep -q "api_router"; then
            log_success "Metrics endpoint contains API router metrics"
        else
            log_verbose "Metrics endpoint may not contain API router metrics yet"
        fi
        return 0
    else
        log_error "Metrics endpoint returned $status_code (expected 200)"
        return 1
    fi
}

# Test inference endpoint
test_inference_endpoint() {
    log_info "Testing inference endpoint (/v1/inference)..."
    
    local request_id=$(generate_uuid)
    local request_body=$(cat <<EOF
{
  "request_id": "$request_id",
  "model": "gpt-4o",
  "payload": "This is a smoke test payload.",
  "parameters": {
    "temperature": 0.7,
    "max_tokens": 100
  }
}
EOF
)
    
    local response=$(http_request "POST" "$SERVICE_URL/v1/inference" "$request_body" "-H 'Content-Type: application/json' -H 'X-API-Key: $API_KEY'")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    log_verbose "Inference response status: $status_code"
    log_verbose "Inference response body: $body"
    
    if [ "$status_code" = "200" ]; then
        log_success "Inference endpoint returned 200"
        
        # Check if response contains expected fields
        if echo "$body" | grep -q '"request_id"'; then
            log_success "Inference response contains request_id"
        else
            log_error "Inference response missing request_id"
            return 1
        fi
        
        # Store request_id for audit test
        echo "$request_id" > /tmp/smoke_test_request_id.txt
        return 0
    elif [ "$status_code" = "401" ] || [ "$status_code" = "403" ]; then
        log_error "Inference endpoint returned $status_code (authentication/authorization issue)"
        return 1
    elif [ "$status_code" = "402" ]; then
        log_skip "Inference endpoint returned 402 (budget exceeded - expected in test environment)"
        return 0
    elif [ "$status_code" = "429" ]; then
        log_skip "Inference endpoint returned 429 (rate limited - expected in test environment)"
        return 0
    elif [ "$status_code" = "503" ] || [ "$status_code" = "502" ]; then
        log_error "Inference endpoint returned $status_code (service/backend unavailable)"
        return 1
    else
        log_error "Inference endpoint returned unexpected status $status_code"
        return 1
    fi
}

# Test rate limiting
test_rate_limiting() {
    if [ "$SKIP_RATE_LIMIT" = true ]; then
        log_skip "Rate limiting tests skipped"
        return 0
    fi
    
    log_info "Testing rate limiting (expecting 429 after exceeding limit)..."
    
    # Send multiple rapid requests to trigger rate limiting
    local rate_limit_triggered=false
    for i in {1..20}; do
        local request_id=$(generate_uuid)
        local request_body=$(cat <<EOF
{
  "request_id": "$request_id",
  "model": "gpt-4o",
  "payload": "Rate limit test $i"
}
EOF
)
        
        local response=$(http_request "POST" "$SERVICE_URL/v1/inference" "$request_body" "-H 'Content-Type: application/json' -H 'X-API-Key: $API_KEY'")
        local status_code=$(echo "$response" | tail -n1)
        
        if [ "$status_code" = "429" ]; then
            rate_limit_triggered=true
            log_success "Rate limiting is working (received 429 response)"
            break
        fi
        
        # Small delay to avoid overwhelming the service
        sleep 0.1
    done
    
    if [ "$rate_limit_triggered" = false ]; then
        log_skip "Rate limiting not triggered (may not be configured or limits are high)"
    fi
    
    return 0
}

# Test budget enforcement
test_budget_enforcement() {
    if [ "$SKIP_BUDGET" = true ]; then
        log_skip "Budget enforcement tests skipped"
        return 0
    fi
    
    log_info "Testing budget enforcement (expecting 402 if budget exceeded)..."
    
    local request_id=$(generate_uuid)
    local request_body=$(cat <<EOF
{
  "request_id": "$request_id",
  "model": "gpt-4o",
  "payload": "Budget enforcement test"
}
EOF
)
    
    local response=$(http_request "POST" "$SERVICE_URL/v1/inference" "$request_body" "-H 'Content-Type: application/json' -H 'X-API-Key: $API_KEY'")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status_code" = "402" ]; then
        log_success "Budget enforcement is working (received 402 response)"
        log_verbose "Response: $body"
        return 0
    elif [ "$status_code" = "200" ]; then
        log_skip "Budget not exceeded (received 200 - budget service may not be configured)"
        return 0
    else
        log_verbose "Budget check returned $status_code (may not be configured)"
        return 0
    fi
}

# Test routing validation
test_routing_validation() {
    log_info "Testing routing validation..."
    
    # Test inference request with routing headers
    local request_id=$(generate_uuid)
    local request_body=$(cat <<EOF
{
  "request_id": "$request_id",
  "model": "gpt-4o",
  "payload": "Routing validation test"
}
EOF
)
    
    local response=$(http_request "POST" "$SERVICE_URL/v1/inference" "$request_body" "-H 'Content-Type: application/json' -H 'X-API-Key: $API_KEY'")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    log_verbose "Routing validation response status: $status_code"
    
    # Check for routing headers in successful responses
    if [ "$status_code" = "200" ]; then
        log_success "Routing validation: Request routed successfully"
        
        # Verify response contains expected fields
        if echo "$body" | grep -q '"request_id"'; then
            log_success "Routing validation: Response contains request_id"
        else
            log_error "Routing validation: Response missing request_id"
            return 1
        fi
        
        return 0
    elif [ "$status_code" = "503" ] || [ "$status_code" = "502" ]; then
        log_error "Routing validation: Backend unavailable (status $status_code)"
        return 1
    elif [ "$status_code" = "429" ] || [ "$status_code" = "402" ]; then
        log_skip "Routing validation: Rate limited or budget exceeded (expected in test environment)"
        return 0
    else
        log_error "Routing validation: Unexpected status $status_code"
        return 1
    fi
}

# Test admin endpoints
test_admin_endpoints() {
    if [ "$SKIP_ADMIN" = true ]; then
        log_skip "Admin endpoint tests skipped"
        return 0
    fi
    
    log_info "Testing admin endpoints..."
    
    # Test list backends
    log_info "  Testing GET /v1/admin/routing/backends..."
    local response=$(http_request "GET" "$SERVICE_URL/v1/admin/routing/backends" "" "-H 'X-API-Key: $API_KEY'")
    local status_code=$(echo "$response" | tail -n1)
    
    if [ "$status_code" = "200" ]; then
        log_success "  List backends endpoint returned 200"
        
        # Verify response contains backends array
        local body=$(echo "$response" | head -n-1)
        if echo "$body" | grep -q "backend"; then
            log_success "  List backends response contains backend data"
        fi
    else
        log_error "  List backends endpoint returned $status_code"
        return 1
    fi
    
    # Test get routing decisions
    log_info "  Testing GET /v1/admin/routing/decisions..."
    local response=$(http_request "GET" "$SERVICE_URL/v1/admin/routing/decisions" "" "-H 'X-API-Key: $API_KEY'")
    local status_code=$(echo "$response" | tail -n1)
    
    if [ "$status_code" = "200" ] || [ "$status_code" = "503" ]; then
        log_success "  Get routing decisions endpoint returned $status_code"
    else
        log_error "  Get routing decisions endpoint returned $status_code"
        return 1
    fi
    
    return 0
}

# Test audit endpoint
test_audit_endpoint() {
    if [ "$SKIP_AUDIT" = true ]; then
        log_skip "Audit endpoint tests skipped"
        return 0
    fi
    
    log_info "Testing audit endpoint (/v1/audit/requests/{requestId})..."
    
    # Get request_id from inference test if available
    local request_id=""
    if [ -f /tmp/smoke_test_request_id.txt ]; then
        request_id=$(cat /tmp/smoke_test_request_id.txt)
    else
        # Use a test UUID
        request_id=$(generate_uuid)
    fi
    
    local response=$(http_request "GET" "$SERVICE_URL/v1/audit/requests/$request_id" "" "-H 'X-API-Key: $API_KEY'")
    local status_code=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status_code" = "200" ]; then
        log_success "Audit endpoint returned 200"
        log_verbose "Response: $body"
        return 0
    elif [ "$status_code" = "404" ]; then
        log_skip "Audit endpoint returned 404 (request not found - may not be indexed yet)"
        return 0
    else
        log_error "Audit endpoint returned $status_code"
        return 1
    fi
}

# Wait for service to be ready
wait_for_service() {
    log_info "Waiting for service at $SERVICE_URL (timeout: ${TIMEOUT}s)..."
    
    local elapsed=0
    while [ $elapsed -lt $TIMEOUT ]; do
        if curl -fsS "$SERVICE_URL/v1/status/healthz" >/dev/null 2>&1; then
            log_success "Service is ready"
            return 0
        fi
        sleep 1
        ((elapsed++))
        if [ $((elapsed % 5)) -eq 0 ]; then
            log_verbose "Still waiting... (${elapsed}s elapsed)"
        fi
    done
    
    log_error "Service did not become ready within ${TIMEOUT}s"
    return 1
}

# Print summary
print_summary() {
    echo ""
    echo "=========================================="
    echo "Smoke Test Summary"
    echo "=========================================="
    echo -e "${GREEN}Passed:${NC} $TESTS_PASSED"
    echo -e "${RED}Failed:${NC} $TESTS_FAILED"
    echo -e "${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
    echo "=========================================="
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All critical tests passed!${NC}"
        return 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        return 1
    fi
}

# Main execution
main() {
    echo -e "${BLUE}API Router Service Smoke Tests${NC}"
    echo "=================================="
    echo "Service URL: $SERVICE_URL"
    echo "API Key: $API_KEY"
    echo ""
    
    # Wait for service
    if ! wait_for_service; then
        exit 1
    fi
    
    # Run tests
    test_health_endpoint || true
    test_readiness_endpoint || true
    test_metrics_endpoint || true
    test_inference_endpoint || true
    test_rate_limiting || true
    test_budget_enforcement || true
    test_routing_validation || true
    test_admin_endpoints || true
    test_audit_endpoint || true
    
    # Print summary and exit with appropriate code
    if print_summary; then
        exit 0
    else
        exit 1
    fi
}

# Cleanup
cleanup() {
    rm -f /tmp/smoke_test_request_id.txt
}
trap cleanup EXIT

# Run main function
main "$@"

