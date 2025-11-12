#!/bin/bash
# Smoke test script for Analytics Service
#
# Purpose:
#   Comprehensive smoke tests for deployment validation covering health checks,
#   usage API, reliability API, and export job management.
#
# Usage:
#   ./smoke.sh [options]
#
# Options:
#   --url URL          Service URL (default: http://localhost:8084)
#   --org-id ID        Organization ID for testing (default: generates test UUID)
#   --actor-subject    Actor subject for RBAC (default: test-user)
#   --actor-roles      Actor roles for RBAC (default: admin)
#   --timeout SECONDS  Timeout for waiting for service (default: 30)
#   --skip-exports     Skip export job tests
#   --skip-reliability Skip reliability API tests
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
SERVICE_URL="${SERVICE_URL:-http://localhost:8084}"
ORG_ID="${ORG_ID:-}"
ACTOR_SUBJECT="${ACTOR_SUBJECT:-test-user}"
ACTOR_ROLES="${ACTOR_ROLES:-admin}"
TIMEOUT="${TIMEOUT:-30}"
SKIP_EXPORTS=false
SKIP_RELIABILITY=false
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
        --org-id)
            ORG_ID="$2"
            shift 2
            ;;
        --actor-subject)
            ACTOR_SUBJECT="$2"
            shift 2
            ;;
        --actor-roles)
            ACTOR_ROLES="$2"
            shift 2
            ;;
        --timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        --skip-exports)
            SKIP_EXPORTS=true
            shift
            ;;
        --skip-reliability)
            SKIP_RELIABILITY=true
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

# Generate UUID
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

# Wait for service to be ready
wait_for_service() {
    log_info "Waiting for service at $SERVICE_URL (timeout: ${TIMEOUT}s)..."
    
    local elapsed=0
    while [ $elapsed -lt $TIMEOUT ]; do
        if curl -s -f "${SERVICE_URL}/analytics/v1/status/healthz" > /dev/null 2>&1; then
            log_success "Service is ready"
            return 0
        fi
        sleep 1
        ((elapsed++))
    done
    
    log_error "Service did not become ready within ${TIMEOUT}s"
    return 1
}

# Test health endpoint
test_health() {
    log_info "Testing health endpoint..."
    
    local response=$(http_request "GET" "${SERVICE_URL}/analytics/v1/status/healthz")
    local status=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status" = "200" ] && [ "$body" = "OK" ]; then
        log_success "Health check passed"
        return 0
    else
        log_error "Health check failed: status=$status, body=$body"
        return 1
    fi
}

# Test readiness endpoint
test_readiness() {
    log_info "Testing readiness endpoint..."
    
    local response=$(http_request "GET" "${SERVICE_URL}/analytics/v1/status/readyz")
    local status=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status" = "200" ] && [ "$body" = "OK" ]; then
        log_success "Readiness check passed"
        return 0
    else
        log_error "Readiness check failed: status=$status, body=$body"
        return 1
    fi
}

# Test metrics endpoint
test_metrics() {
    log_info "Testing Prometheus metrics endpoint..."
    
    local response=$(http_request "GET" "${SERVICE_URL}/metrics")
    local status=$(echo "$response" | tail -n1)
    local body=$(echo "$response" | head -n-1)
    
    if [ "$status" = "200" ] && echo "$body" | grep -q "http_requests_total\|analytics_"; then
        log_success "Metrics endpoint accessible"
        return 0
    else
        log_error "Metrics endpoint failed: status=$status"
        return 1
    fi
}

# Test usage API
test_usage_api() {
    log_info "Testing usage API..."
    
    local start_date=$(date -u -v-7d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "7 days ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")
    local end_date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    local url="${SERVICE_URL}/analytics/v1/orgs/${ORG_ID}/usage?start=${start_date}&end=${end_date}&granularity=daily"
    local headers="-H 'X-Actor-Subject: ${ACTOR_SUBJECT}' -H 'X-Actor-Roles: ${ACTOR_ROLES}'"
    
    local response=$(http_request "GET" "$url" "" "$headers")
    local status=$(echo "$response" | tail -n1)
    
    if [ "$status" = "200" ] || [ "$status" = "404" ]; then
        # 404 is acceptable if no data exists yet
        log_success "Usage API accessible (status: $status)"
        return 0
    else
        log_error "Usage API failed: status=$status"
        return 1
    fi
}

# Test reliability API
test_reliability_api() {
    if [ "$SKIP_RELIABILITY" = true ]; then
        log_skip "Reliability API tests skipped"
        return 0
    fi
    
    log_info "Testing reliability API..."
    
    local start_date=$(date -u -v-7d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "7 days ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")
    local end_date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    local url="${SERVICE_URL}/analytics/v1/orgs/${ORG_ID}/reliability?start=${start_date}&end=${end_date}&granularity=daily"
    local headers="-H 'X-Actor-Subject: ${ACTOR_SUBJECT}' -H 'X-Actor-Roles: ${ACTOR_ROLES}'"
    
    local response=$(http_request "GET" "$url" "" "$headers")
    local status=$(echo "$response" | tail -n1)
    
    if [ "$status" = "200" ] || [ "$status" = "404" ]; then
        log_success "Reliability API accessible (status: $status)"
        return 0
    else
        log_error "Reliability API failed: status=$status"
        return 1
    fi
}

# Test export API
test_export_api() {
    if [ "$SKIP_EXPORTS" = true ]; then
        log_skip "Export API tests skipped"
        return 0
    fi
    
    log_info "Testing export API..."
    
    local start_date=$(date -u -v-7d +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "7 days ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")
    local end_date=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    # Test list exports
    local list_url="${SERVICE_URL}/analytics/v1/orgs/${ORG_ID}/exports"
    local headers="-H 'X-Actor-Subject: ${ACTOR_SUBJECT}' -H 'X-Actor-Roles: ${ACTOR_ROLES}'"
    
    local response=$(http_request "GET" "$list_url" "" "$headers")
    local status=$(echo "$response" | tail -n1)
    
    if [ "$status" = "200" ]; then
        log_success "Export list API accessible"
    else
        log_error "Export list API failed: status=$status"
        return 1
    fi
    
    # Test create export (optional - may require S3 config)
    log_verbose "Skipping export creation test (requires S3 configuration)"
    
    return 0
}

# Test RBAC (should fail without proper headers)
test_rbac_protection() {
    log_info "Testing RBAC protection..."
    
    local url="${SERVICE_URL}/analytics/v1/orgs/${ORG_ID}/usage"
    local response=$(http_request "GET" "$url")
    local status=$(echo "$response" | tail -n1)
    
    # Should return 401 or 403 without auth headers
    if [ "$status" = "401" ] || [ "$status" = "403" ]; then
        log_success "RBAC protection working (status: $status)"
        return 0
    else
        log_error "RBAC protection may not be working: status=$status (expected 401/403)"
        return 1
    fi
}

# Main test execution
main() {
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Analytics Service Smoke Tests${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo ""
    
    # Generate org ID if not provided
    if [ -z "$ORG_ID" ]; then
        ORG_ID=$(generate_uuid)
        log_info "Generated test org ID: $ORG_ID"
    fi
    
    log_info "Service URL: $SERVICE_URL"
    log_info "Org ID: $ORG_ID"
    log_info "Actor Subject: $ACTOR_SUBJECT"
    log_info "Actor Roles: $ACTOR_ROLES"
    echo ""
    
    # Wait for service
    if ! wait_for_service; then
        log_error "Service is not available. Exiting."
        exit 1
    fi
    
    echo ""
    log_info "Running smoke tests..."
    echo ""
    
    # Run tests
    test_health
    test_readiness
    test_metrics
    test_rbac_protection
    test_usage_api
    
    if [ "$SKIP_RELIABILITY" = false ]; then
        test_reliability_api
    fi
    
    if [ "$SKIP_EXPORTS" = false ]; then
        test_export_api
    fi
    
    # Summary
    echo ""
    echo -e "${BLUE}========================================${NC}"
    echo -e "${BLUE}Test Summary${NC}"
    echo -e "${BLUE}========================================${NC}"
    echo -e "${GREEN}Passed:${NC} $TESTS_PASSED"
    echo -e "${RED}Failed:${NC} $TESTS_FAILED"
    echo -e "${YELLOW}Skipped:${NC} $TESTS_SKIPPED"
    echo ""
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    fi
}

# Run main
main

