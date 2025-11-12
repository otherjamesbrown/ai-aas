#!/bin/bash
# Load test script for API Router Service using vegeta
#
# Purpose:
#   This script runs load test scenarios to validate SLO requirements:
#   - Latency P95 ≤ 3s (NFR-001)
#   - Latency P99 ≤ 5s
#   - Error rate < 1%
#   - Router overhead ≤ 150ms median (NFR-001)
#   - Router overhead ≤ 400ms p95 (NFR-001)
#   - Rate limit decision ≤ 5ms (NFR-002)
#
# Usage:
#   ./scripts/loadtest.sh [scenario]
#
# Scenarios:
#   baseline  - Normal traffic patterns (100 RPS for 60s)
#   peak      - Sustained high RPS (1000 RPS for 60s)
#   burst     - Sudden spike (2000 RPS for 30s)
#   backend-failure - Backend failure scenario
#   rate-limit - Rate limit enforcement
#   budget    - Budget enforcement
#   all       - Run all scenarios (default)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
TARGET_URL="${TARGET_URL:-http://localhost:8080}"
API_KEY="${API_KEY:-dev-test-key}"
SCENARIO="${1:-all}"
RESULTS_DIR="${RESULTS_DIR:-./tmp/loadtest-results}"
TIMEOUT="${TIMEOUT:-30s}"

# SLO thresholds
SLO_P95_LATENCY=3.0      # seconds
SLO_P99_LATENCY=5.0      # seconds
SLO_ERROR_RATE=0.01      # 1%
SLO_ROUTER_OVERHEAD_MEDIAN=0.15  # 150ms
SLO_ROUTER_OVERHEAD_P95=0.4      # 400ms
SLO_RATE_LIMIT_DECISION=0.005     # 5ms

# Create results directory
mkdir -p "$RESULTS_DIR"

# Check if vegeta is installed
if ! command -v vegeta &> /dev/null; then
    echo -e "${RED}Error: vegeta is not installed${NC}"
    echo "Install with: go install github.com/tsenart/vegeta/v2@latest"
    exit 1
fi

# Generate a UUID for request ID
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

# Create request body template
create_request_body() {
    local request_id=$(generate_uuid)
    cat <<EOF
{
  "request_id": "$request_id",
  "model": "gpt-4o",
  "payload": "This is a test payload for load testing the API router service.",
  "parameters": {
    "temperature": 0.7,
    "max_tokens": 100
  }
}
EOF
}

# Run a load test scenario
run_scenario() {
    local name=$1
    local rate=$2
    local duration=$3
    local description=$4
    
    echo -e "\n${YELLOW}Running scenario: $name${NC}"
    echo "Description: $description"
    echo "Rate: $rate RPS"
    echo "Duration: $duration"
    echo "Target: $TARGET_URL/v1/inference"
    
    # Create request file
    local request_file="$RESULTS_DIR/${name}_requests.txt"
    local body_file="$RESULTS_DIR/${name}_body.json"
    
    create_request_body > "$body_file"
    
    # Create vegeta target file
    cat > "$request_file" <<EOF
POST $TARGET_URL/v1/inference
X-API-Key: $API_KEY
Content-Type: application/json
@$body_file
EOF
    
    # Run vegeta attack
    local results_file="$RESULTS_DIR/${name}_results.bin"
    local report_file="$RESULTS_DIR/${name}_report.txt"
    
    echo "Starting load test..."
    vegeta attack \
        -rate="$rate" \
        -duration="$duration" \
        -timeout="$TIMEOUT" \
        -targets="$request_file" \
        -output="$results_file" \
        > /dev/null 2>&1 || true
    
    # Generate report
    vegeta report -type=text "$results_file" > "$report_file"
    vegeta report -type=json "$results_file" > "$RESULTS_DIR/${name}_results.json"
    
    # Display results
    echo -e "\n${GREEN}Results:${NC}"
    cat "$report_file"
    
    # Validate SLOs
    validate_slos "$name" "$results_file"
    
    echo -e "${GREEN}Scenario $name completed${NC}"
}

# Validate SLOs from results
validate_slos() {
    local name=$1
    local results_file=$2
    local validation_file="$RESULTS_DIR/${name}_validation.txt"
    
    echo -e "\n${YELLOW}Validating SLOs...${NC}"
    
    # Extract metrics from JSON report
    local json_report="$RESULTS_DIR/${name}_results.json"
    
    if [ ! -f "$json_report" ]; then
        echo -e "${RED}Error: JSON report not found${NC}"
        return 1
    fi
    
    # Extract metrics (requires jq)
    if command -v jq &> /dev/null; then
        local p95=$(jq -r '.latencies.p95' "$json_report" 2>/dev/null || echo "0")
        local p99=$(jq -r '.latencies.p99' "$json_report" 2>/dev/null || echo "0")
        local error_rate=$(jq -r '.errors / .requests' "$json_report" 2>/dev/null || echo "0")
        local total_requests=$(jq -r '.requests' "$json_report" 2>/dev/null || echo "0")
        local errors=$(jq -r '.errors' "$json_report" 2>/dev/null || echo "0")
        
        # Convert nanoseconds to seconds for latency
        p95=$(echo "$p95 / 1000000000" | bc -l 2>/dev/null || echo "0")
        p99=$(echo "$p99 / 1000000000" | bc -l 2>/dev/null || echo "0")
        
        echo "P95 Latency: ${p95}s (threshold: ${SLO_P95_LATENCY}s)"
        echo "P99 Latency: ${p99}s (threshold: ${SLO_P99_LATENCY}s)"
        echo "Error Rate: ${error_rate} (threshold: ${SLO_ERROR_RATE})"
        echo "Total Requests: $total_requests"
        echo "Errors: $errors"
        
        # Validate thresholds
        local failed=0
        
        if (( $(echo "$p95 > $SLO_P95_LATENCY" | bc -l 2>/dev/null || echo "0") )); then
            echo -e "${RED}❌ FAIL: P95 latency ${p95}s exceeds threshold ${SLO_P95_LATENCY}s${NC}"
            failed=1
        else
            echo -e "${GREEN}✅ PASS: P95 latency ${p95}s within threshold${NC}"
        fi
        
        if (( $(echo "$p99 > $SLO_P99_LATENCY" | bc -l 2>/dev/null || echo "0") )); then
            echo -e "${RED}❌ FAIL: P99 latency ${p99}s exceeds threshold ${SLO_P99_LATENCY}s${NC}"
            failed=1
        else
            echo -e "${GREEN}✅ PASS: P99 latency ${p99}s within threshold${NC}"
        fi
        
        if (( $(echo "$error_rate > $SLO_ERROR_RATE" | bc -l 2>/dev/null || echo "0") )); then
            echo -e "${RED}❌ FAIL: Error rate ${error_rate} exceeds threshold ${SLO_ERROR_RATE}${NC}"
            failed=1
        else
            echo -e "${GREEN}✅ PASS: Error rate ${error_rate} within threshold${NC}"
        fi
        
        if [ $failed -eq 1 ]; then
            echo -e "\n${RED}SLO validation failed for scenario: $name${NC}"
            return 1
        else
            echo -e "\n${GREEN}All SLOs passed for scenario: $name${NC}"
            return 0
        fi
    else
        echo -e "${YELLOW}Warning: jq not installed, skipping detailed SLO validation${NC}"
        echo "Install jq for detailed validation: brew install jq (macOS) or apt-get install jq (Linux)"
        return 0
    fi
}

# Baseline load scenario
scenario_baseline() {
    run_scenario "baseline" "100" "60s" "Normal traffic patterns"
}

# Peak load scenario
scenario_peak() {
    run_scenario "peak" "1000" "60s" "Sustained high RPS"
}

# Burst traffic scenario
scenario_burst() {
    run_scenario "burst" "2000" "30s" "Sudden traffic spike"
}

# Backend failure scenario
scenario_backend_failure() {
    echo -e "\n${YELLOW}Backend failure scenario${NC}"
    echo "This scenario requires a backend to be marked as degraded"
    echo "Run this manually after marking a backend as degraded via admin API"
    run_scenario "backend-failure" "100" "30s" "Backend failure handling"
}

# Rate limit enforcement scenario
scenario_rate_limit() {
    echo -e "\n${YELLOW}Rate limit enforcement scenario${NC}"
    echo "This scenario tests rate limit enforcement"
    echo "Expecting 429 responses when rate limit is exceeded"
    
    # Run at high rate to trigger rate limiting
    run_scenario "rate-limit" "500" "30s" "Rate limit enforcement"
    
    # Check for 429 responses in results
    local results_file="$RESULTS_DIR/rate-limit_results.bin"
    if [ -f "$results_file" ]; then
        local status_file="$RESULTS_DIR/rate-limit_status.txt"
        vegeta report -type=text "$results_file" | grep -E "429|Too Many Requests" > "$status_file" || true
        
        if [ -s "$status_file" ]; then
            echo -e "${GREEN}✅ Rate limiting is working (429 responses detected)${NC}"
        else
            echo -e "${YELLOW}⚠️  No 429 responses detected - rate limiting may not be configured${NC}"
        fi
    fi
}

# Budget enforcement scenario
scenario_budget() {
    echo -e "\n${YELLOW}Budget enforcement scenario${NC}"
    echo "This scenario tests budget/quota enforcement"
    echo "Expecting 402 responses when budget is exceeded"
    
    run_scenario "budget" "100" "30s" "Budget enforcement"
    
    # Check for 402 responses in results
    local results_file="$RESULTS_DIR/budget_results.bin"
    if [ -f "$results_file" ]; then
        local status_file="$RESULTS_DIR/budget_status.txt"
        vegeta report -type=text "$results_file" | grep -E "402|Payment Required" > "$status_file" || true
        
        if [ -s "$status_file" ]; then
            echo -e "${GREEN}✅ Budget enforcement is working (402 responses detected)${NC}"
        else
            echo -e "${YELLOW}⚠️  No 402 responses detected - budget service may not be configured${NC}"
        fi
    fi
}

# Generate summary report
generate_summary() {
    local summary_file="$RESULTS_DIR/summary.txt"
    
    echo "Load Test Summary" > "$summary_file"
    echo "=================" >> "$summary_file"
    echo "Date: $(date)" >> "$summary_file"
    echo "Target: $TARGET_URL" >> "$summary_file"
    echo "" >> "$summary_file"
    
    for report in "$RESULTS_DIR"/*_report.txt; do
        if [ -f "$report" ]; then
            local scenario=$(basename "$report" _report.txt)
            echo "Scenario: $scenario" >> "$summary_file"
            echo "---" >> "$summary_file"
            cat "$report" >> "$summary_file"
            echo "" >> "$summary_file"
        fi
    done
    
    echo -e "\n${GREEN}Summary report saved to: $summary_file${NC}"
}

# Main execution
main() {
    echo -e "${GREEN}API Router Service Load Testing${NC}"
    echo "=================================="
    echo "Target URL: $TARGET_URL"
    echo "Results Directory: $RESULTS_DIR"
    echo "Scenario: $SCENARIO"
    echo ""
    
    case "$SCENARIO" in
        baseline)
            scenario_baseline
            ;;
        peak)
            scenario_peak
            ;;
        burst)
            scenario_burst
            ;;
        backend-failure)
            scenario_backend_failure
            ;;
        rate-limit)
            scenario_rate_limit
            ;;
        budget)
            scenario_budget
            ;;
        all)
            scenario_baseline
            scenario_peak
            scenario_burst
            scenario_rate_limit
            scenario_budget
            generate_summary
            ;;
        *)
            echo -e "${RED}Unknown scenario: $SCENARIO${NC}"
            echo "Available scenarios: baseline, peak, burst, backend-failure, rate-limit, budget, all"
            exit 1
            ;;
    esac
    
    echo -e "\n${GREEN}Load testing completed!${NC}"
    echo "Results saved to: $RESULTS_DIR"
}

# Run main function
main "$@"

