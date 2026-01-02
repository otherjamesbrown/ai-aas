#!/usr/bin/env bash
#
# run-uc-tests.sh - Run use case tests with coverage reporting
#
# A unified test runner for UC tests that:
#   - Runs tests against specified environment (dev/staging)
#   - Supports filtering by UC prefix, UC ID, or AC ID
#   - Optionally includes GitOps integration tests
#   - Reports coverage after test runs
#
# Usage:
#   ./scripts/run-uc-tests.sh                        # Run all UC tests (dev)
#   ./scripts/run-uc-tests.sh --env staging          # Run against staging
#   ./scripts/run-uc-tests.sh --uc UC-ORG-001        # Run specific UC
#   ./scripts/run-uc-tests.sh --prefix MDL           # Run all UC-MDL-* tests
#   ./scripts/run-uc-tests.sh --gitops               # Include GitOps tests
#   ./scripts/run-uc-tests.sh --coverage-only        # Show coverage without running
#   ./scripts/run-uc-tests.sh --list                 # List available UCs
#
# Exit codes:
#   0 - All tests passed
#   1 - Test failures
#   2 - Configuration error

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
BOLD='\033[1m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
TESTS_DIR="$REPO_ROOT/tests/usecases"
SECRETS_DIR="$REPO_ROOT/secrets"

# Default configuration
ENV="development"
FILTER=""
PREFIX=""
INCLUDE_GITOPS=false
COVERAGE_ONLY=false
LIST_ONLY=false
VERBOSE=false
TIMEOUT="10m"

# Environment endpoints
declare -A ENDPOINTS=(
    [development]="https://user-org.dev.otherjamesbrown.com"
    [staging]="https://user-org.staging.otherjamesbrown.com"
)

# Print usage
usage() {
    cat << EOF
Usage: $(basename "$0") [OPTIONS]

Run use case tests with coverage reporting.

Options:
    -e, --env ENV         Environment: development (default), staging
    -u, --uc UC_ID        Run specific UC (e.g., UC-ORG-001)
    -p, --prefix PREFIX   Run UCs with prefix (e.g., MDL for UC-MDL-*)
    -g, --gitops          Include GitOps integration tests (UC-MLC-010, UC-MLC-011)
    -t, --timeout TIME    Test timeout (default: 10m, 30m for gitops)
    -c, --coverage-only   Show coverage report without running tests
    -l, --list            List available UCs and exit
    -v, --verbose         Verbose output
    -h, --help            Show this help message

Examples:
    $(basename "$0")                     # Run all UC tests against development
    $(basename "$0") -e staging          # Run all tests against staging
    $(basename "$0") -u UC-ORG-001       # Run specific UC test
    $(basename "$0") -p MDL              # Run all model tests (UC-MDL-*)
    $(basename "$0") -g                  # Include GitOps tests
    $(basename "$0") -p MLC -g           # Run all lifecycle tests including GitOps
    $(basename "$0") -c                  # Show coverage report only

Environment Variables:
    AI_AAS_API_KEY          API key (auto-loaded from secrets/env/.env)
    AI_AAS_ORG_ID           Organization ID (auto-loaded from secrets/env/.env)
    RUN_GITOPS_TESTS        Set to 1 to enable GitOps tests (set automatically with -g)
    AI_AAS_CONFIG_PATH      Path to ai-aas-config repo (default: ~/ai-aas-config)
    KUBECONFIG              Kubeconfig path (default: secrets/kubeconfigs/kubeconfig-development.yaml)
EOF
}

# Log functions
info() { echo -e "${CYAN}INFO:${NC} $*"; }
success() { echo -e "${GREEN}OK:${NC} $*"; }
warn() { echo -e "${YELLOW}WARN:${NC} $*"; }
error() { echo -e "${RED}ERROR:${NC} $*" >&2; }

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -e|--env)
            ENV="$2"
            shift 2
            ;;
        -u|--uc)
            FILTER="$2"
            shift 2
            ;;
        -p|--prefix)
            PREFIX="$2"
            shift 2
            ;;
        -g|--gitops)
            INCLUDE_GITOPS=true
            TIMEOUT="30m"
            shift
            ;;
        -t|--timeout)
            TIMEOUT="$2"
            shift 2
            ;;
        -c|--coverage-only)
            COVERAGE_ONLY=true
            shift
            ;;
        -l|--list)
            LIST_ONLY=true
            shift
            ;;
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            error "Unknown option: $1"
            usage
            exit 2
            ;;
    esac
done

# Validate environment
if [[ ! -v "ENDPOINTS[$ENV]" ]]; then
    error "Invalid environment: $ENV (valid: development, staging)"
    exit 2
fi

# List UCs if requested
if $LIST_ONLY; then
    echo -e "${BOLD}Available Use Cases${NC}"
    echo "==================="
    echo ""

    if command -v yq &> /dev/null && [[ -d "$REPO_ROOT/usecases" ]]; then
        for file in "$REPO_ROOT/usecases"/*.yaml; do
            [[ -f "$file" ]] || continue
            feature=$(basename "$file" .yaml)
            echo -e "${CYAN}$feature:${NC}"

            yq eval '.usecases[] | "  " + .id + ": " + .title' "$file" 2>/dev/null || true
            echo ""
        done
    else
        warn "yq not available or no usecases/ directory - listing test files instead"
        find "$TESTS_DIR" -name "*_test.go" -exec basename {} \; | sort -u
    fi
    exit 0
fi

# Show coverage only if requested
if $COVERAGE_ONLY; then
    if [[ -x "$SCRIPT_DIR/uc-coverage.sh" ]]; then
        exec "$SCRIPT_DIR/uc-coverage.sh" $([[ -n "$FILTER" ]] && echo "$FILTER")
    else
        error "Coverage script not found: $SCRIPT_DIR/uc-coverage.sh"
        exit 2
    fi
fi

# Build test filter pattern
build_test_pattern() {
    local pattern=""

    if [[ -n "$FILTER" ]]; then
        # Specific UC or AC: UC-ORG-001 or UC-ORG-001/AC-01
        # Convert dashes to underscores for Go test names
        pattern=$(echo "$FILTER" | sed 's/-/_/g')
        # Handle slashes (AC subtest)
        pattern=$(echo "$pattern" | sed 's/\//\//g')
        # Remove UC_ prefix if present (user might pass UC-ORG-001 or ORG-001)
        pattern="${pattern#UC_}"
        echo "TestUC_${pattern}"
    elif [[ -n "$PREFIX" ]]; then
        # Prefix: MDL -> TestUC_MDL_
        echo "TestUC_${PREFIX}_"
    else
        # All UC tests
        echo "TestUC_"
    fi
}

# Check prerequisites
check_prerequisites() {
    local has_errors=false

    info "Checking prerequisites..."

    # Check go
    if ! command -v go &> /dev/null; then
        error "go is not installed"
        has_errors=true
    else
        $VERBOSE && success "go $(go version | cut -d' ' -f3)"
    fi

    # Check test directory
    if [[ ! -d "$TESTS_DIR" ]]; then
        error "Test directory not found: $TESTS_DIR"
        has_errors=true
    fi

    # Check secrets file exists
    if [[ ! -f "$SECRETS_DIR/env/.env" ]]; then
        warn "Secrets file not found: $SECRETS_DIR/env/.env"
        warn "Tests will fail without API credentials"
    else
        $VERBOSE && success "Secrets file found"
    fi

    # GitOps-specific checks
    if $INCLUDE_GITOPS; then
        info "Checking GitOps prerequisites..."

        # Check ai-aas-config repo
        local config_path="${AI_AAS_CONFIG_PATH:-$HOME/ai-aas-config}"
        if [[ ! -d "$config_path" ]]; then
            warn "ai-aas-config not found at $config_path"
            warn "GitOps tests will be skipped"
        else
            $VERBOSE && success "ai-aas-config found at $config_path"
        fi

        # Check kubeconfig
        local kubeconfig="${KUBECONFIG:-$SECRETS_DIR/kubeconfigs/kubeconfig-development.yaml}"
        if [[ ! -f "$kubeconfig" ]]; then
            warn "Kubeconfig not found at $kubeconfig"
            warn "GitOps tests will be skipped"
        else
            $VERBOSE && success "Kubeconfig found at $kubeconfig"
        fi
    fi

    if $has_errors; then
        error "Prerequisites check failed"
        exit 2
    fi

    success "Prerequisites OK"
}

# Run tests
run_tests() {
    local pattern
    pattern=$(build_test_pattern)
    local endpoint="${ENDPOINTS[$ENV]}"

    echo ""
    echo -e "${BOLD}Running Use Case Tests${NC}"
    echo "======================="
    echo ""
    echo -e "Environment:  ${CYAN}$ENV${NC}"
    echo -e "Endpoint:     ${CYAN}$endpoint${NC}"
    echo -e "Pattern:      ${CYAN}$pattern${NC}"
    echo -e "Timeout:      ${CYAN}$TIMEOUT${NC}"
    echo -e "GitOps:       ${CYAN}$($INCLUDE_GITOPS && echo "enabled" || echo "disabled")${NC}"
    echo ""

    # Build environment variables
    local env_vars=(
        "AI_AAS_API_ENDPOINT=$endpoint"
    )

    if $INCLUDE_GITOPS; then
        env_vars+=("RUN_GITOPS_TESTS=1")

        # Set config path if not already set
        if [[ -z "${AI_AAS_CONFIG_PATH:-}" ]]; then
            env_vars+=("AI_AAS_CONFIG_PATH=$HOME/ai-aas-config")
        fi

        # Set kubeconfig if not already set
        if [[ -z "${KUBECONFIG:-}" ]]; then
            env_vars+=("KUBECONFIG=$SECRETS_DIR/kubeconfigs/kubeconfig-development.yaml")
        fi
    fi

    # Build go test command
    local test_cmd="go test"
    $VERBOSE && test_cmd="$test_cmd -v"
    test_cmd="$test_cmd -timeout $TIMEOUT"
    test_cmd="$test_cmd -run '$pattern'"
    test_cmd="$test_cmd ./..."

    # Run tests
    info "Running: $test_cmd"
    echo ""

    cd "$TESTS_DIR"

    local exit_code=0
    if env "${env_vars[@]}" bash -c "$test_cmd"; then
        echo ""
        success "All tests passed!"
    else
        exit_code=$?
        echo ""
        error "Some tests failed (exit code: $exit_code)"
    fi

    return $exit_code
}

# Show coverage summary
show_coverage_summary() {
    echo ""
    echo -e "${BOLD}Coverage Summary${NC}"
    echo "================"

    if [[ -x "$SCRIPT_DIR/uc-coverage.sh" ]]; then
        # Run coverage in JSON mode and parse summary
        local coverage_json
        coverage_json=$("$SCRIPT_DIR/uc-coverage.sh" --json 2>/dev/null || echo '{"usecases":[]}')

        local total_ucs
        local total_acs
        local covered_acs

        total_ucs=$(echo "$coverage_json" | grep -o '"id":' | wc -l)
        total_acs=$(echo "$coverage_json" | grep -o '"total_acs":' | wc -l)
        covered_acs=$(echo "$coverage_json" | grep -oP '"covered":\s*\K\d+' | awk '{s+=$1} END {print s+0}')

        if command -v jq &> /dev/null; then
            total_acs=$(echo "$coverage_json" | jq '[.usecases[].total_acs] | add // 0')
            covered_acs=$(echo "$coverage_json" | jq '[.usecases[].covered] | add // 0')
        fi

        echo -e "Use Cases: ${CYAN}$total_ucs${NC}"
        echo -e "Total ACs: ${CYAN}$total_acs${NC}"
        echo -e "Covered:   ${CYAN}$covered_acs${NC}"

        if [[ $total_acs -gt 0 ]]; then
            local pct=$((covered_acs * 100 / total_acs))
            if [[ $pct -eq 100 ]]; then
                echo -e "Coverage:  ${GREEN}$pct%${NC}"
            elif [[ $pct -ge 80 ]]; then
                echo -e "Coverage:  ${YELLOW}$pct%${NC}"
            else
                echo -e "Coverage:  ${RED}$pct%${NC}"
            fi
        fi

        echo ""
        echo -e "${GRAY}Run './scripts/uc-coverage.sh' for detailed report${NC}"
    else
        warn "Coverage script not available"
    fi
}

# Main
main() {
    check_prerequisites

    local test_exit_code=0
    run_tests || test_exit_code=$?

    show_coverage_summary

    exit $test_exit_code
}

main
