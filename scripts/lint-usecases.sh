#!/usr/bin/env bash
#
# lint-usecases.sh - Validate use case YAML files
#
# Checks:
# - ID format matches UC-{PREFIX}-{NNN} pattern
# - Required fields present (title, description, acceptance_criteria, in_scope, out_of_scope, must_not)
# - AC IDs follow AC-{NN} pattern
# - Dependencies reference valid UCs
# - No duplicate IDs within feature
# - Status is valid (draft, active, deprecated)
# - Interface is valid (cli, web, api)
#
# Usage:
#   ./scripts/lint-usecases.sh                    # Lint all UC files
#   ./scripts/lint-usecases.sh usecases/auth.yaml # Lint specific file
#
# Exit codes:
#   0 - All checks passed
#   1 - Validation errors found

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
USECASES_DIR="$REPO_ROOT/usecases"

errors=0
warnings=0
files_checked=0

# Check if yq is available
if ! command -v yq &> /dev/null; then
    echo -e "${RED}Error: yq is required but not installed.${NC}"
    echo "Install with: brew install yq (macOS) or snap install yq (Linux)"
    exit 1
fi

# Collect all UC IDs for cross-reference checking
declare -A all_uc_ids

lint_file() {
    local file="$1"
    local file_errors=0
    local basename=$(basename "$file")

    echo "Checking $basename..."

    # Check if file is valid YAML
    if ! yq eval '.' "$file" > /dev/null 2>&1; then
        echo -e "  ${RED}✗ Invalid YAML syntax${NC}"
        ((errors++))
        return
    fi

    # Check feature-level required fields
    local feature=$(yq eval '.feature // ""' "$file")
    if [[ -z "$feature" ]]; then
        echo -e "  ${RED}✗ Missing required field: feature${NC}"
        ((file_errors++))
    fi

    local description=$(yq eval '.description // ""' "$file")
    if [[ -z "$description" ]]; then
        echo -e "  ${RED}✗ Missing required field: description${NC}"
        ((file_errors++))
    fi

    # Get number of use cases
    local uc_count=$(yq eval '.usecases | length' "$file")

    if [[ "$uc_count" == "0" ]] || [[ "$uc_count" == "null" ]]; then
        echo -e "  ${YELLOW}⚠ No use cases defined${NC}"
        ((warnings++))
        return
    fi

    # Check each use case
    for i in $(seq 0 $((uc_count - 1))); do
        local uc_id=$(yq eval ".usecases[$i].id // \"\"" "$file")
        local uc_title=$(yq eval ".usecases[$i].title // \"\"" "$file")

        # Check ID format (UC-XXX-NNN)
        if [[ -z "$uc_id" ]]; then
            echo -e "  ${RED}✗ Use case $i: Missing id${NC}"
            ((file_errors++))
        elif ! [[ "$uc_id" =~ ^UC-[A-Z]{2,5}-[0-9]{3}$ ]]; then
            echo -e "  ${RED}✗ $uc_id: Invalid ID format (expected UC-XXX-NNN)${NC}"
            ((file_errors++))
        else
            # Check for duplicate IDs
            if [[ -n "${all_uc_ids[$uc_id]:-}" ]]; then
                echo -e "  ${RED}✗ $uc_id: Duplicate ID (also in ${all_uc_ids[$uc_id]})${NC}"
                ((file_errors++))
            else
                all_uc_ids[$uc_id]="$basename"
            fi
        fi

        # Check required fields
        if [[ -z "$uc_title" ]]; then
            echo -e "  ${RED}✗ $uc_id: Missing title${NC}"
            ((file_errors++))
        fi

        local uc_desc=$(yq eval ".usecases[$i].description // \"\"" "$file")
        if [[ -z "$uc_desc" ]]; then
            echo -e "  ${RED}✗ $uc_id: Missing description${NC}"
            ((file_errors++))
        fi

        # Check interface is valid
        local interface=$(yq eval ".usecases[$i].interface // \"cli\"" "$file")
        if ! [[ "$interface" =~ ^(cli|web|api)$ ]]; then
            echo -e "  ${RED}✗ $uc_id: Invalid interface '$interface' (expected cli, web, or api)${NC}"
            ((file_errors++))
        fi

        # Check status is valid
        local status=$(yq eval ".usecases[$i].status // \"draft\"" "$file")
        if ! [[ "$status" =~ ^(draft|active|deprecated)$ ]]; then
            echo -e "  ${RED}✗ $uc_id: Invalid status '$status' (expected draft, active, or deprecated)${NC}"
            ((file_errors++))
        fi

        # Check actor
        local actor=$(yq eval ".usecases[$i].actor // \"\"" "$file")
        if [[ -z "$actor" ]]; then
            echo -e "  ${RED}✗ $uc_id: Missing actor${NC}"
            ((file_errors++))
        fi

        # Check acceptance criteria
        local ac_count=$(yq eval ".usecases[$i].acceptance_criteria | length" "$file")
        if [[ "$ac_count" == "0" ]] || [[ "$ac_count" == "null" ]]; then
            echo -e "  ${RED}✗ $uc_id: No acceptance criteria defined${NC}"
            ((file_errors++))
        elif [[ "$ac_count" -lt 2 ]]; then
            echo -e "  ${YELLOW}⚠ $uc_id: Only $ac_count acceptance criterion (recommend at least 2)${NC}"
            ((warnings++))
        else
            # Check each AC
            for j in $(seq 0 $((ac_count - 1))); do
                local ac_id=$(yq eval ".usecases[$i].acceptance_criteria[$j].id // \"\"" "$file")

                # Check AC ID format (AC-NN)
                if ! [[ "$ac_id" =~ ^AC-[0-9]{2}$ ]]; then
                    echo -e "  ${RED}✗ $uc_id/$ac_id: Invalid AC ID format (expected AC-NN)${NC}"
                    ((file_errors++))
                fi

                # Check required AC fields
                local criterion=$(yq eval ".usecases[$i].acceptance_criteria[$j].criterion // \"\"" "$file")
                local given=$(yq eval ".usecases[$i].acceptance_criteria[$j].given // \"\"" "$file")
                local when=$(yq eval ".usecases[$i].acceptance_criteria[$j].when // \"\"" "$file")
                local then_count=$(yq eval ".usecases[$i].acceptance_criteria[$j].then | length" "$file")

                if [[ -z "$criterion" ]]; then
                    echo -e "  ${RED}✗ $uc_id/$ac_id: Missing criterion${NC}"
                    ((file_errors++))
                fi
                if [[ -z "$given" ]]; then
                    echo -e "  ${RED}✗ $uc_id/$ac_id: Missing given${NC}"
                    ((file_errors++))
                fi
                if [[ -z "$when" ]]; then
                    echo -e "  ${RED}✗ $uc_id/$ac_id: Missing when${NC}"
                    ((file_errors++))
                fi
                if [[ "$then_count" == "0" ]] || [[ "$then_count" == "null" ]]; then
                    echo -e "  ${RED}✗ $uc_id/$ac_id: Missing then outcomes${NC}"
                    ((file_errors++))
                fi
            done
        fi

        # Check scope sections
        local in_scope_count=$(yq eval ".usecases[$i].in_scope | length" "$file")
        if [[ "$in_scope_count" == "0" ]] || [[ "$in_scope_count" == "null" ]]; then
            echo -e "  ${RED}✗ $uc_id: Missing in_scope section${NC}"
            ((file_errors++))
        fi

        local out_of_scope_count=$(yq eval ".usecases[$i].out_of_scope | length" "$file")
        if [[ "$out_of_scope_count" == "0" ]] || [[ "$out_of_scope_count" == "null" ]]; then
            echo -e "  ${RED}✗ $uc_id: Missing out_of_scope section${NC}"
            ((file_errors++))
        fi

        local must_not_count=$(yq eval ".usecases[$i].must_not | length" "$file")
        if [[ "$must_not_count" == "0" ]] || [[ "$must_not_count" == "null" ]]; then
            echo -e "  ${RED}✗ $uc_id: Missing must_not section${NC}"
            ((file_errors++))
        fi

        # Check dependencies reference valid UCs (will be validated in second pass)
        local deps_count=$(yq eval ".usecases[$i].depends_on | length" "$file")
        if [[ "$deps_count" != "0" ]] && [[ "$deps_count" != "null" ]]; then
            for k in $(seq 0 $((deps_count - 1))); do
                local dep=$(yq eval ".usecases[$i].depends_on[$k]" "$file")
                # Store for later validation
                echo "$uc_id depends on $dep" >> /tmp/uc_deps_$$.txt 2>/dev/null || true
            done
        fi
    done

    if [[ $file_errors -eq 0 ]]; then
        echo -e "  ${GREEN}✓ Valid ($uc_count use cases)${NC}"
    fi

    ((errors += file_errors))
    ((files_checked++))
}

# Main
echo "Use Case Linter"
echo "==============="
echo ""

if [[ $# -gt 0 ]]; then
    # Lint specific files
    for file in "$@"; do
        if [[ -f "$file" ]]; then
            lint_file "$file"
        else
            echo -e "${RED}File not found: $file${NC}"
            ((errors++))
        fi
    done
else
    # Lint all UC files
    if [[ ! -d "$USECASES_DIR" ]]; then
        echo -e "${YELLOW}No usecases/ directory found${NC}"
        exit 0
    fi

    for file in "$USECASES_DIR"/*.yaml; do
        [[ -f "$file" ]] || continue
        [[ "$(basename "$file")" == "SCHEMA.md" ]] && continue
        lint_file "$file"
    done
fi

# Validate dependency references
if [[ -f /tmp/uc_deps_$$.txt ]]; then
    echo ""
    echo "Validating dependencies..."
    while IFS= read -r line; do
        uc_id=$(echo "$line" | cut -d' ' -f1)
        dep=$(echo "$line" | cut -d' ' -f4)
        if [[ -z "${all_uc_ids[$dep]:-}" ]]; then
            echo -e "  ${RED}✗ $uc_id: Dependency '$dep' not found${NC}"
            ((errors++))
        fi
    done < /tmp/uc_deps_$$.txt
    rm -f /tmp/uc_deps_$$.txt
fi

# Summary
echo ""
echo "==============="
echo "Summary"
echo "==============="
echo "Files checked: $files_checked"
echo "Use cases: ${#all_uc_ids[@]}"
echo -e "Errors: ${RED}$errors${NC}"
echo -e "Warnings: ${YELLOW}$warnings${NC}"

if [[ $errors -gt 0 ]]; then
    echo ""
    echo -e "${RED}Linting failed with $errors error(s)${NC}"
    exit 1
else
    echo ""
    echo -e "${GREEN}All checks passed${NC}"
    exit 0
fi
