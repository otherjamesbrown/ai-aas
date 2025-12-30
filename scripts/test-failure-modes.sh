#!/usr/bin/env bash
#
# Test Harness for Model Deployment Failure Modes
#
# Purpose:
#   Systematically test each failure mode configuration and verify the CLI
#   produces correct, human-readable output with appropriate phase and message.
#
# Usage:
#   ./scripts/test-failure-modes.sh [OPTIONS]
#
# Options:
#   --test NAME        Run a specific test (e.g., test-insufficient-cpu)
#   --verbose          Show full CLI output during test runs
#   --no-cleanup       Leave test configs applied for debugging
#   --timeout SECS     Max wait time per test (default: 120)
#   --help             Show this help message
#
# Examples:
#   ./scripts/test-failure-modes.sh                    # Run all tests
#   ./scripts/test-failure-modes.sh --test test-oom-kill  # Run single test
#   ./scripts/test-failure-modes.sh --verbose --no-cleanup # Debug mode

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CONFIG_DIR="$PROJECT_ROOT/tests/failure-modes/configs"
RESULTS_DIR="$PROJECT_ROOT/test-results/failure-modes/$(date +%Y%m%d-%H%M%S)"
NAMESPACE="development"
TIMEOUT=120
VERBOSE=false
NO_CLEANUP=false
SPECIFIC_TEST=""

# Test definitions: name|expected_phase|expected_message_pattern1|expected_message_pattern2|...
declare -a TESTS=(
  "test-insufficient-cpu|Scheduling|Insufficient cpu"
  "test-insufficient-memory|Scheduling|Insufficient memory"
  "test-missing-toleration|Scheduling|untolerated taint"
  "test-no-gpu-available|Scheduling|Insufficient nvidia.com/gpu"
  "test-needs-scaleup|Scheduling|waiting for"
  "test-at-max-scale|Scheduling|Insufficient"
  "test-invalid-s3-path|Failed|not found"
  "test-invalid-hf-model|Failed|not found"
  "test-oom-kill|Failed|OOMKilled"
  "test-crash-loop|Failed|CrashLoopBackOff"
  "test-invalid-recipe|Failed|recipe.*not found"
)

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0
SKIPPED=0

# Cleanup list
declare -a CLEANUP_MODELS=()

# Functions
usage() {
  cat <<EOF
Test Harness for Model Deployment Failure Modes

Usage: $0 [OPTIONS]

Options:
  --test NAME        Run a specific test (e.g., test-insufficient-cpu)
  --verbose          Show full CLI output during test runs
  --no-cleanup       Leave test configs applied for debugging
  --timeout SECS     Max wait time per test (default: 120)
  --help             Show this help message

Examples:
  $0                              # Run all tests
  $0 --test test-oom-kill         # Run single test
  $0 --verbose --no-cleanup       # Debug mode
EOF
  exit 0
}

log() {
  echo -e "${BLUE}[INFO]${NC} $*"
}

success() {
  echo -e "${GREEN}[PASS]${NC} $*"
}

error() {
  echo -e "${RED}[FAIL]${NC} $*"
}

warning() {
  echo -e "${YELLOW}[WARN]${NC} $*"
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --test)
      SPECIFIC_TEST="$2"
      shift 2
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    --no-cleanup)
      NO_CLEANUP=true
      shift
      ;;
    --timeout)
      TIMEOUT="$2"
      shift 2
      ;;
    --help)
      usage
      ;;
    *)
      error "Unknown option: $1"
      usage
      ;;
  esac
done

# Verify prerequisites
check_prerequisites() {
  log "Checking prerequisites..."

  if ! command -v kubectl &> /dev/null; then
    error "kubectl not found - please install kubectl"
    exit 1
  fi

  if ! command -v ai-aas-cli &> /dev/null; then
    error "ai-aas-cli not found - please install or build the CLI"
    exit 1
  fi

  # Check kubectl access
  if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
    error "Cannot access namespace: $NAMESPACE"
    error "Please configure kubectl with cluster access"
    exit 1
  fi

  # Check config directory
  if [[ ! -d "$CONFIG_DIR" ]]; then
    error "Config directory not found: $CONFIG_DIR"
    exit 1
  fi

  success "Prerequisites check passed"
}

# Create results directory
setup_results_dir() {
  mkdir -p "$RESULTS_DIR"
  log "Results directory: $RESULTS_DIR"
}

# Cleanup function (runs on exit)
cleanup() {
  if [[ "$NO_CLEANUP" == "true" ]]; then
    warning "Skipping cleanup (--no-cleanup flag set)"
    log "Manual cleanup required for: ${CLEANUP_MODELS[*]}"
    return
  fi

  if [[ ${#CLEANUP_MODELS[@]} -eq 0 ]]; then
    return
  fi

  log "Cleaning up test models..."
  for model_name in "${CLEANUP_MODELS[@]}"; do
    log "Deleting AIModel: $model_name"
    kubectl delete aimodel "$model_name" -n "$NAMESPACE" --ignore-not-found=true &> /dev/null || true
  done
  success "Cleanup complete"
}

# Register cleanup handler
trap cleanup EXIT

# Apply test config by enabling it
apply_config() {
  local config_file="$1"
  local model_name

  model_name=$(basename "$config_file" .yaml)

  log "Applying config: $model_name"

  # Check if config exists
  if [[ ! -f "$config_file" ]]; then
    error "Config file not found: $config_file"
    return 1
  fi

  # Apply the config
  if ! kubectl apply -f "$config_file" -n "$NAMESPACE" &> /dev/null; then
    error "Failed to apply config: $config_file"
    return 1
  fi

  # Enable the model (patch enabled=true)
  if ! kubectl patch aimodel "$model_name" -n "$NAMESPACE" \
      --type merge -p '{"spec":{"enabled":true}}' &> /dev/null; then
    error "Failed to enable model: $model_name"
    return 1
  fi

  # Register for cleanup
  CLEANUP_MODELS+=("$model_name")

  success "Applied and enabled: $model_name"
  return 0
}

# Wait for model to reach expected phase
wait_for_phase() {
  local model_name="$1"
  local expected_phase="$2"
  local timeout="$3"
  local elapsed=0
  local interval=5

  log "Waiting for phase '$expected_phase' (timeout: ${timeout}s)"

  while [[ $elapsed -lt $timeout ]]; do
    # Get current phase using kubectl
    local current_phase
    current_phase=$(kubectl get aimodel "$model_name" -n "$NAMESPACE" \
      -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")

    if [[ "$current_phase" == "$expected_phase" ]] || \
       [[ "$expected_phase" == "Failed" && "$current_phase" == "Failed" ]] || \
       [[ "$expected_phase" == "Scheduling" && "$current_phase" == "Scheduling" ]]; then
      success "Reached phase: $current_phase"
      return 0
    fi

    if [[ "$VERBOSE" == "true" ]]; then
      log "Current phase: $current_phase (waiting for $expected_phase)"
    fi

    sleep "$interval"
    elapsed=$((elapsed + interval))
  done

  error "Timeout waiting for phase '$expected_phase' (current: $current_phase)"
  return 1
}

# Check status message contains expected patterns
check_status_message() {
  local model_name="$1"
  shift
  local expected_patterns=("$@")

  log "Checking status message for patterns..."

  # Get status message using CLI
  local cli_output
  cli_output=$(ai-aas-cli model status "$model_name" 2>&1 || true)

  # Save CLI output to results
  echo "$cli_output" > "$RESULTS_DIR/${model_name}_cli_output.txt"

  if [[ "$VERBOSE" == "true" ]]; then
    log "CLI output:"
    echo "$cli_output"
  fi

  # Check each pattern
  local all_found=true
  for pattern in "${expected_patterns[@]}"; do
    if echo "$cli_output" | grep -qiE "$pattern"; then
      success "Found pattern: $pattern"
    else
      error "Pattern not found: $pattern"
      all_found=false
    fi
  done

  if [[ "$all_found" == "true" ]]; then
    return 0
  else
    return 1
  fi
}

# Run a single test
run_test() {
  local test_def="$1"

  # Parse test definition
  IFS='|' read -r model_name expected_phase expected_patterns <<< "$test_def"

  # Convert pattern string to array
  IFS='|' read -ra pattern_array <<< "$expected_patterns"

  local config_file="$CONFIG_DIR/${model_name}.yaml"

  echo ""
  log "========================================"
  log "Test: $model_name"
  log "Expected Phase: $expected_phase"
  log "Expected Patterns: ${pattern_array[*]}"
  log "========================================"

  # Check if config exists
  if [[ ! -f "$config_file" ]]; then
    error "Config file not found: $config_file"
    SKIPPED=$((SKIPPED + 1))
    return
  fi

  # Apply config
  if ! apply_config "$config_file"; then
    error "Failed to apply config"
    FAILED=$((FAILED + 1))
    return
  fi

  # Wait for expected phase
  if ! wait_for_phase "$model_name" "$expected_phase" "$TIMEOUT"; then
    error "Failed to reach expected phase"
    FAILED=$((FAILED + 1))
    return
  fi

  # Check status message
  if ! check_status_message "$model_name" "${pattern_array[@]}"; then
    error "Status message validation failed"
    FAILED=$((FAILED + 1))
    return
  fi

  success "Test passed: $model_name"
  PASSED=$((PASSED + 1))
}

# Run all tests
run_all_tests() {
  for test_def in "${TESTS[@]}"; do
    IFS='|' read -r model_name _ <<< "$test_def"

    # If specific test requested, skip others
    if [[ -n "$SPECIFIC_TEST" && "$model_name" != "$SPECIFIC_TEST" ]]; then
      continue
    fi

    run_test "$test_def"
  done
}

# Generate summary report
generate_report() {
  local total=$((PASSED + FAILED + SKIPPED))

  echo ""
  log "========================================"
  log "Test Summary"
  log "========================================"
  log "Total:   $total"
  success "Passed:  $PASSED"
  error "Failed:  $FAILED"
  warning "Skipped: $SKIPPED"
  log "Results: $RESULTS_DIR"
  log "========================================"

  # Create summary file
  cat > "$RESULTS_DIR/summary.txt" <<EOF
Test Harness Results - $(date)

Total:   $total
Passed:  $PASSED
Failed:  $FAILED
Skipped: $SKIPPED

Results directory: $RESULTS_DIR
EOF

  if [[ $FAILED -gt 0 ]]; then
    error "Some tests failed - see results in $RESULTS_DIR"
    return 1
  else
    success "All tests passed!"
    return 0
  fi
}

# Main execution
main() {
  log "AI-AAS Model Deployment Failure Mode Test Harness"
  log "Started: $(date)"

  check_prerequisites
  setup_results_dir
  run_all_tests
  generate_report
}

# Run main
main
exit_code=$?
exit $exit_code
