#!/bin/bash
# Run AI-AAS CLI E2E tests against the development environment
#
# Usage:
#   ./tests/e2e/run_tests.sh           # Run all e2e tests
#   ./tests/e2e/run_tests.sh -v        # Run with verbose output
#   ./tests/e2e/run_tests.sh -run TestFullWorkflow  # Run specific test
#
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLI_DIR="$(cd "${SCRIPT_DIR}/../.." && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

echo "========================================"
echo "AI-AAS CLI End-to-End Tests"
echo "========================================"
echo ""

# Check for required tools
if ! command -v go &> /dev/null; then
    echo "ERROR: Go is not installed"
    exit 1
fi

# Load environment
echo "Loading test environment..."
if [[ -f "${SCRIPT_DIR}/env.sh" ]]; then
    source "${SCRIPT_DIR}/env.sh"
else
    echo "ERROR: env.sh not found at ${SCRIPT_DIR}/env.sh"
    exit 1
fi

# Build the CLI if needed
CLI_BINARY="${CLI_DIR}/ai-aas-cli"
if [[ ! -f "$CLI_BINARY" ]] || [[ "$1" == "--build" ]]; then
    echo ""
    echo "Building ai-aas-cli..."
    cd "$CLI_DIR"
    go build -o ai-aas-cli ./main.go
    echo "Built: $CLI_BINARY"
fi

export AI_AAS_CLI_PATH="$CLI_BINARY"

# Run tests
echo ""
echo "Running E2E tests..."
echo "----------------------------------------"

cd "$CLI_DIR"

# Pass through any arguments to go test
GO_TEST_ARGS="-v -tags=e2e ./tests/e2e/..."

# Add any additional flags passed to this script
for arg in "$@"; do
    case $arg in
        --build)
            # Already handled above
            ;;
        -v|--verbose)
            GO_TEST_ARGS="-v -tags=e2e ./tests/e2e/..."
            ;;
        -run=*|--run=*)
            GO_TEST_ARGS="$GO_TEST_ARGS ${arg#--}"
            ;;
        *)
            GO_TEST_ARGS="$GO_TEST_ARGS $arg"
            ;;
    esac
done

echo "go test $GO_TEST_ARGS"
echo ""

go test $GO_TEST_ARGS

echo ""
echo "========================================"
echo "E2E Tests Complete"
echo "========================================"
