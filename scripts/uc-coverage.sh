#!/usr/bin/env bash
#
# uc-coverage.sh - Check test coverage for use case acceptance criteria
#
# Scans test files for UC/AC references and reports coverage.
#
# Usage:
#   ./scripts/uc-coverage.sh                  # Check all UCs
#   ./scripts/uc-coverage.sh UC-BM-001        # Check specific UC
#   ./scripts/uc-coverage.sh --json           # Output as JSON
#
# Exit codes:
#   0 - All ACs have tests (or --json mode)
#   1 - Some ACs missing tests

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
USECASES_DIR="$REPO_ROOT/usecases"

# Parse arguments
json_output=false
filter_uc=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --json)
            json_output=true
            shift
            ;;
        UC-*)
            filter_uc="$1"
            shift
            ;;
        *)
            echo "Unknown option: $1"
            echo "Usage: $0 [--json] [UC-XXX-NNN]"
            exit 1
            ;;
    esac
done

# Check if yq is available
if ! command -v yq &> /dev/null; then
    echo -e "${RED}Error: yq is required but not installed.${NC}"
    exit 1
fi

# Find all test files
find_test_files() {
    find "$REPO_ROOT/services" -name "*_test.go" -type f 2>/dev/null || true
    find "$REPO_ROOT/tests" -name "*_test.go" -type f 2>/dev/null || true
}

# Search for AC reference in test files
find_ac_test() {
    local uc_id="$1"
    local ac_id="$2"
    local pattern="${uc_id}.*${ac_id}|${uc_id}/${ac_id}|${uc_id}_${ac_id}"

    # Convert UC-BM-001 to various patterns: UC_BM_001, UC-BM-001
    local uc_underscore=$(echo "$uc_id" | tr '-' '_')
    pattern="${pattern}|${uc_underscore}.*${ac_id}|${uc_underscore}_${ac_id}"

    local result=""
    while IFS= read -r file; do
        if grep -lE "$pattern" "$file" 2>/dev/null; then
            result="$file"
            break
        fi
    done < <(find_test_files)

    echo "$result"
}

# Process a single UC file
process_uc_file() {
    local file="$1"
    local uc_count=$(yq eval '.usecases | length' "$file")

    [[ "$uc_count" == "0" ]] || [[ "$uc_count" == "null" ]] && return

    for i in $(seq 0 $((uc_count - 1))); do
        local uc_id=$(yq eval ".usecases[$i].id" "$file")
        local uc_title=$(yq eval ".usecases[$i].title" "$file")
        local uc_status=$(yq eval ".usecases[$i].status // \"active\"" "$file")

        # Skip if filtering and doesn't match
        [[ -n "$filter_uc" ]] && [[ "$uc_id" != "$filter_uc" ]] && continue

        # Skip deprecated UCs
        [[ "$uc_status" == "deprecated" ]] && continue

        local ac_count=$(yq eval ".usecases[$i].acceptance_criteria | length" "$file")
        local covered=0
        local uncovered=()
        local covered_list=()

        if [[ "$ac_count" != "0" ]] && [[ "$ac_count" != "null" ]]; then
            for j in $(seq 0 $((ac_count - 1))); do
                local ac_id=$(yq eval ".usecases[$i].acceptance_criteria[$j].id" "$file")
                local ac_criterion=$(yq eval ".usecases[$i].acceptance_criteria[$j].criterion" "$file")

                local test_file=$(find_ac_test "$uc_id" "$ac_id")

                if [[ -n "$test_file" ]]; then
                    ((covered++))
                    covered_list+=("$ac_id:$test_file")
                else
                    uncovered+=("$ac_id:$ac_criterion")
                fi
            done
        fi

        # Store results for output
        echo "$uc_id|$uc_title|$ac_count|$covered|${uncovered[*]:-}|${covered_list[*]:-}"
    done
}

# Main
declare -a results=()

if [[ ! -d "$USECASES_DIR" ]]; then
    if $json_output; then
        echo '{"error": "No usecases/ directory found", "usecases": []}'
    else
        echo -e "${YELLOW}No usecases/ directory found${NC}"
    fi
    exit 0
fi

# Process all UC files
for file in "$USECASES_DIR"/*.yaml; do
    [[ -f "$file" ]] || continue
    while IFS= read -r line; do
        [[ -n "$line" ]] && results+=("$line")
    done < <(process_uc_file "$file")
done

# Output results
if $json_output; then
    echo '{"usecases": ['
    first=true
    for result in "${results[@]}"; do
        IFS='|' read -r uc_id uc_title ac_count covered uncovered covered_list <<< "$result"

        $first || echo ','
        first=false

        echo -n "  {\"id\": \"$uc_id\", \"title\": \"$uc_title\", "
        echo -n "\"total_acs\": $ac_count, \"covered\": $covered, "
        echo -n "\"coverage_pct\": $(( ac_count > 0 ? (covered * 100 / ac_count) : 0 ))}"
    done
    echo ''
    echo ']}'
else
    echo "Use Case Test Coverage Report"
    echo "=============================="
    echo ""

    total_acs=0
    total_covered=0

    for result in "${results[@]}"; do
        IFS='|' read -r uc_id uc_title ac_count covered uncovered covered_list <<< "$result"

        ((total_acs += ac_count))
        ((total_covered += covered))

        local pct=0
        [[ $ac_count -gt 0 ]] && pct=$((covered * 100 / ac_count))

        if [[ $covered -eq $ac_count ]]; then
            echo -e "${GREEN}✓${NC} $uc_id: $uc_title"
            echo -e "  Coverage: ${GREEN}$covered/$ac_count ($pct%)${NC}"
        elif [[ $covered -eq 0 ]]; then
            echo -e "${RED}✗${NC} $uc_id: $uc_title"
            echo -e "  Coverage: ${RED}$covered/$ac_count ($pct%)${NC}"
        else
            echo -e "${YELLOW}◐${NC} $uc_id: $uc_title"
            echo -e "  Coverage: ${YELLOW}$covered/$ac_count ($pct%)${NC}"
        fi

        # Show covered ACs
        if [[ -n "$covered_list" ]]; then
            for item in $covered_list; do
                ac_id=$(echo "$item" | cut -d: -f1)
                test_file=$(echo "$item" | cut -d: -f2-)
                rel_path="${test_file#$REPO_ROOT/}"
                echo -e "    ${GREEN}✓${NC} $ac_id → $rel_path"
            done
        fi

        # Show uncovered ACs
        if [[ -n "$uncovered" ]]; then
            for item in $uncovered; do
                ac_id=$(echo "$item" | cut -d: -f1)
                criterion=$(echo "$item" | cut -d: -f2-)
                echo -e "    ${RED}✗${NC} $ac_id: $criterion ${RED}(NO TEST)${NC}"
            done
        fi

        echo ""
    done

    # Summary
    echo "=============================="
    echo "Summary"
    echo "=============================="
    echo "Use Cases: ${#results[@]}"
    echo "Acceptance Criteria: $total_acs"

    local total_pct=0
    [[ $total_acs -gt 0 ]] && total_pct=$((total_covered * 100 / total_acs))

    if [[ $total_covered -eq $total_acs ]]; then
        echo -e "Coverage: ${GREEN}$total_covered/$total_acs ($total_pct%)${NC}"
    else
        echo -e "Coverage: ${YELLOW}$total_covered/$total_acs ($total_pct%)${NC}"
    fi

    if [[ $total_covered -lt $total_acs ]]; then
        echo ""
        echo -e "${YELLOW}Some acceptance criteria are missing tests${NC}"
        exit 1
    fi
fi
