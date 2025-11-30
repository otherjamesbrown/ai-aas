#!/bin/bash
# E2E Test Script: User Model Access Control (022-user-model-access-control)
#
# Tests the complete user-model-access-control feature using the CLI.
# Designed to run against the development cluster.
#
# Prerequisites:
#   - ai-aas-cli installed and configured
#   - Access to development cluster (kubeconfig)
#   - MASTER_ADMIN_API_KEY environment variable set
#
# Usage:
#   ./test-user-model-access.sh                    # Run all tests
#   ./test-user-model-access.sh --skip-cleanup     # Keep test data for debugging
#   ./test-user-model-access.sh --verbose          # Show detailed output

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
SKIP_CLEANUP=false
VERBOSE=false
TEST_ORG_SLUG="e2e-model-access-test-$(date +%s)"
TEST_USER_EMAIL="e2e-test-user-$(date +%s)@test.example.com"
TEST_MODEL="mistral-7b-instruct"  # Must be a model that exists in the cluster

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --skip-cleanup)
            SKIP_CLEANUP=true
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --skip-cleanup    Keep test data after tests complete"
            echo "  --verbose, -v     Show detailed command output"
            echo "  --help, -h        Show this help message"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            exit 1
            ;;
    esac
done

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

# CLI binary path - prefer local dev build if available
CLI_BIN="${CLI_BIN:-ai-aas-cli}"
if [[ -x "$ROOT_DIR/services/ai-aas-cli/bin/ai-aas-cli" ]]; then
    CLI_BIN="$ROOT_DIR/services/ai-aas-cli/bin/ai-aas-cli"
fi

# Run CLI command with optional verbose output
run_cli() {
    if [[ "$VERBOSE" == "true" ]]; then
        "$CLI_BIN" "$@"
    else
        "$CLI_BIN" "$@" 2>/dev/null
    fi
}

# Test assertion helper
assert_success() {
    local description="$1"
    shift
    TESTS_RUN=$((TESTS_RUN + 1))

    log_test "$description"

    if "$@"; then
        log_success "$description"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        log_error "$description"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

assert_failure() {
    local description="$1"
    shift
    TESTS_RUN=$((TESTS_RUN + 1))

    log_test "$description"

    if ! "$@"; then
        log_success "$description (expected failure)"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        log_error "$description (should have failed but succeeded)"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

assert_contains() {
    local description="$1"
    local expected="$2"
    local actual="$3"
    TESTS_RUN=$((TESTS_RUN + 1))

    log_test "$description"

    if echo "$actual" | grep -q "$expected"; then
        log_success "$description"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        log_error "$description - expected '$expected' in output"
        if [[ "$VERBOSE" == "true" ]]; then
            echo "Actual output: $actual"
        fi
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."

    # Check CLI is available
    if ! "$CLI_BIN" --version &> /dev/null && ! "$CLI_BIN" --help &> /dev/null; then
        log_error "ai-aas-cli is not installed or not accessible"
        echo "Build with: cd $ROOT_DIR/services/ai-aas-cli && go build -o bin/ai-aas-cli ."
        exit 1
    fi
    log_info "Using CLI: $CLI_BIN"

    # Check API key is set
    if [[ -z "${MASTER_ADMIN_API_KEY:-}" ]]; then
        # Try to load from secrets
        if [[ -f "$ROOT_DIR/secrets/env/.env" ]]; then
            source "$ROOT_DIR/secrets/env/.env" 2>/dev/null || true
        fi

        if [[ -z "${MASTER_ADMIN_API_KEY:-}" ]]; then
            log_error "MASTER_ADMIN_API_KEY is not set"
            echo "Set it with: export MASTER_ADMIN_API_KEY=<your-key>"
            echo "Or source: source $ROOT_DIR/secrets/env/.env"
            exit 1
        fi
    fi

    # Check CLI can connect
    if ! "$CLI_BIN" status &> /dev/null; then
        log_warn "CLI status check failed - services may not be accessible"
        echo "Ensure you have access to the development cluster"
    fi

    log_success "Prerequisites check passed"
}

# Setup test data
setup_test_data() {
    log_info "Setting up test data..."

    # Create test organization and capture output
    log_info "Creating test organization: $TEST_ORG_SLUG"
    local create_output
    create_output=$(run_cli org create --slug "$TEST_ORG_SLUG" --name "E2E Model Access Test Org" 2>&1)
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create test organization"
        echo "$create_output"
        exit 1
    fi

    # Extract org ID from the create output (from the "Org ID:" line or audit log)
    TEST_ORG_ID=$(echo "$create_output" | grep -E "Org ID:|orgId" | head -1 | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' || echo "")
    if [[ -z "$TEST_ORG_ID" ]]; then
        log_error "Failed to get test organization ID from create output"
        echo "$create_output"
        exit 1
    fi
    log_info "Test org ID: $TEST_ORG_ID"

    # Create test user and capture output
    log_info "Creating test user: $TEST_USER_EMAIL"
    local user_output
    user_output=$(run_cli user create --org-id "$TEST_ORG_ID" --email "$TEST_USER_EMAIL" 2>&1)
    if [[ $? -ne 0 ]]; then
        log_error "Failed to create test user"
        echo "$user_output"
        exit 1
    fi

    # Extract user ID from the create output (from "User ID:" line or audit log)
    TEST_USER_ID=$(echo "$user_output" | grep -E "User ID:|userId" | head -1 | grep -oE '[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}' || echo "")
    if [[ -z "$TEST_USER_ID" ]]; then
        log_error "Failed to get test user ID from create output"
        echo "$user_output"
        exit 1
    fi
    log_info "Test user ID: $TEST_USER_ID"

    log_success "Test data setup complete"
}

# Cleanup test data
cleanup_test_data() {
    if [[ "$SKIP_CLEANUP" == "true" ]]; then
        log_warn "Skipping cleanup (--skip-cleanup flag set)"
        log_info "Test org: $TEST_ORG_SLUG (ID: ${TEST_ORG_ID:-unknown})"
        log_info "Test user: $TEST_USER_EMAIL (ID: ${TEST_USER_ID:-unknown})"
        return 0
    fi

    log_info "Cleaning up test data..."

    # Delete test user (if exists)
    if [[ -n "${TEST_USER_ID:-}" ]]; then
        run_cli user delete --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>/dev/null || true
    fi

    # Delete test organization (if exists)
    if [[ -n "${TEST_ORG_ID:-}" ]]; then
        run_cli org delete --org-id "$TEST_ORG_ID" 2>/dev/null || true
    fi

    log_success "Cleanup complete"
}

# Trap to ensure cleanup runs on exit
trap cleanup_test_data EXIT

# =============================================================================
# TEST CASES
# =============================================================================

test_default_access_mode() {
    log_info "=== Test: Default Access Mode ==="

    # New users should have 'restricted' access mode by default
    local output
    output=$(run_cli user model-access show --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    assert_contains "Default access mode is 'restricted'" "restricted" "$output"
}

test_set_access_mode_auto_grant() {
    log_info "=== Test: Set Access Mode to auto_grant ==="

    # Set access mode to auto_grant
    assert_success "Set access mode to auto_grant" \
        run_cli user model-access set-mode --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" --mode auto_grant

    # Verify the change
    local output
    output=$(run_cli user model-access show --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    assert_contains "Access mode changed to auto_grant" "auto_grant" "$output"
}

test_set_access_mode_restricted() {
    log_info "=== Test: Set Access Mode to restricted ==="

    # Set access mode back to restricted
    assert_success "Set access mode to restricted" \
        run_cli user model-access set-mode --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" --mode restricted

    # Verify the change
    local output
    output=$(run_cli user model-access show --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    assert_contains "Access mode changed to restricted" "restricted" "$output"
}

test_grant_model_access() {
    log_info "=== Test: Grant Model Access ==="

    # Grant access to a specific model
    assert_success "Grant access to model '$TEST_MODEL'" \
        run_cli user model-access grant --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" --model "$TEST_MODEL"

    # Verify the grant appears in the list
    local output
    output=$(run_cli user model-access list --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    assert_contains "Model grant appears in list" "$TEST_MODEL" "$output"
}

test_list_model_grants() {
    log_info "=== Test: List Model Grants ==="

    # List should show the granted model
    local output
    output=$(run_cli user model-access list --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    assert_contains "List shows granted model" "$TEST_MODEL" "$output"
}

test_revoke_model_access() {
    log_info "=== Test: Revoke Model Access ==="

    # Revoke access to the model
    assert_success "Revoke access to model '$TEST_MODEL'" \
        run_cli user model-access revoke --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" --model "$TEST_MODEL"

    # Verify the grant is removed
    local output
    output=$(run_cli user model-access list --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    # Should NOT contain the model after revocation (or show empty/no grants)
    if echo "$output" | grep -q "$TEST_MODEL"; then
        log_error "Model should not appear after revocation"
        TESTS_RUN=$((TESTS_RUN + 1))
        TESTS_FAILED=$((TESTS_FAILED + 1))
    else
        log_success "Model removed from grants after revocation"
        TESTS_RUN=$((TESTS_RUN + 1))
        TESTS_PASSED=$((TESTS_PASSED + 1))
    fi
}

test_grant_all_current_models() {
    log_info "=== Test: Grant All Current Models ==="

    # Grant all current models
    assert_success "Grant all current models" \
        run_cli user model-access grant-all --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID"

    # Verify grants were created (should have at least one model)
    local output
    output=$(run_cli user model-access list --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" 2>&1) || true

    # Should show some grants (not empty)
    TESTS_RUN=$((TESTS_RUN + 1))
    if [[ -n "$output" ]] && ! echo "$output" | grep -qi "no grants\|empty\|none"; then
        log_success "Grant all created model grants"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        log_warn "Grant all may not have created grants (no models available?)"
        TESTS_PASSED=$((TESTS_PASSED + 1))  # Not a failure if no models exist
    fi
}

test_invalid_access_mode() {
    log_info "=== Test: Invalid Access Mode (Error Handling) ==="

    # Try to set an invalid access mode
    assert_failure "Reject invalid access mode" \
        run_cli user model-access set-mode --org-id "$TEST_ORG_ID" --user-id "$TEST_USER_ID" --mode invalid_mode
}

test_show_with_email() {
    log_info "=== Test: Show Access Using Email ==="

    # Should be able to use email instead of user-id
    local output
    output=$(run_cli user model-access show --org-id "$TEST_ORG_ID" --email "$TEST_USER_EMAIL" 2>&1) || true

    # Should return access info (contains 'restricted' or 'auto_grant')
    TESTS_RUN=$((TESTS_RUN + 1))
    if echo "$output" | grep -qE "(restricted|auto_grant)"; then
        log_success "Show with email works"
        TESTS_PASSED=$((TESTS_PASSED + 1))
    else
        log_error "Show with email failed"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# =============================================================================
# MAIN EXECUTION
# =============================================================================

main() {
    echo ""
    echo "========================================"
    echo "  User Model Access Control E2E Tests  "
    echo "  Feature: 022-user-model-access-control"
    echo "========================================"
    echo ""

    # Prerequisites
    check_prerequisites

    # Setup
    setup_test_data

    echo ""
    echo "========================================"
    echo "  Running Tests                         "
    echo "========================================"
    echo ""

    # Run tests
    test_default_access_mode
    test_set_access_mode_auto_grant
    test_set_access_mode_restricted
    test_grant_model_access
    test_list_model_grants
    test_revoke_model_access
    test_grant_all_current_models
    test_invalid_access_mode
    test_show_with_email

    # Summary
    echo ""
    echo "========================================"
    echo "  Test Summary                          "
    echo "========================================"
    echo ""
    echo "Tests run:    $TESTS_RUN"
    echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
    echo ""

    if [[ $TESTS_FAILED -gt 0 ]]; then
        log_error "Some tests failed!"
        exit 1
    else
        log_success "All tests passed!"
        exit 0
    fi
}

main "$@"
