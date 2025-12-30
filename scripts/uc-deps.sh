#!/usr/bin/env bash
#
# uc-deps.sh - Show use case dependency graph
#
# Usage:
#   ./scripts/uc-deps.sh                  # Show all dependencies
#   ./scripts/uc-deps.sh UC-BM-002        # Show deps for specific UC
#   ./scripts/uc-deps.sh --reverse        # Show what depends on each UC
#   ./scripts/uc-deps.sh --roots          # Show UCs with no dependencies
#   ./scripts/uc-deps.sh --leaves         # Show UCs nothing depends on
#
# Exit codes:
#   0 - Success

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
GRAY='\033[0;90m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
USECASES_DIR="$REPO_ROOT/usecases"

# Parse arguments
mode="deps"
filter_uc=""

while [[ $# -gt 0 ]]; do
    case $1 in
        --reverse)
            mode="reverse"
            shift
            ;;
        --roots)
            mode="roots"
            shift
            ;;
        --leaves)
            mode="leaves"
            shift
            ;;
        UC-*)
            filter_uc="$1"
            shift
            ;;
        -h|--help)
            echo "Usage: $0 [OPTIONS] [UC-XXX-NNN]"
            echo ""
            echo "Options:"
            echo "  --reverse   Show what depends on each UC"
            echo "  --roots     Show UCs with no dependencies (entry points)"
            echo "  --leaves    Show UCs nothing depends on (endpoints)"
            echo ""
            echo "Examples:"
            echo "  $0                  # Show all dependency trees"
            echo "  $0 UC-BM-002        # Show deps for specific UC"
            echo "  $0 --roots          # Find starting points"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            echo "Use --help for usage"
            exit 1
            ;;
    esac
done

# Check if yq is available
if ! command -v yq &> /dev/null; then
    echo -e "${RED}Error: yq is required but not installed.${NC}"
    exit 1
fi

# Build dependency graph
declare -A uc_titles
declare -A uc_deps
declare -A reverse_deps

load_dependencies() {
    if [[ ! -d "$USECASES_DIR" ]]; then
        echo -e "${YELLOW}No usecases/ directory found${NC}"
        exit 0
    fi

    for file in "$USECASES_DIR"/*.yaml; do
        [[ -f "$file" ]] || continue

        local uc_count=$(yq eval '.usecases | length' "$file")
        [[ "$uc_count" == "0" ]] || [[ "$uc_count" == "null" ]] && continue

        for i in $(seq 0 $((uc_count - 1))); do
            local uc_id=$(yq eval ".usecases[$i].id" "$file")
            local uc_title=$(yq eval ".usecases[$i].title" "$file")
            local uc_status=$(yq eval ".usecases[$i].status // \"active\"" "$file")

            # Skip deprecated
            [[ "$uc_status" == "deprecated" ]] && continue

            uc_titles[$uc_id]="$uc_title"
            uc_deps[$uc_id]=""

            local deps_count=$(yq eval ".usecases[$i].depends_on | length" "$file")
            if [[ "$deps_count" != "0" ]] && [[ "$deps_count" != "null" ]]; then
                for j in $(seq 0 $((deps_count - 1))); do
                    local dep=$(yq eval ".usecases[$i].depends_on[$j]" "$file")
                    uc_deps[$uc_id]+="$dep "

                    # Build reverse deps
                    reverse_deps[$dep]+="$uc_id "
                done
            fi
        done
    done
}

# Print tree for a single UC
print_tree() {
    local uc_id="$1"
    local indent="$2"
    local visited="$3"

    # Check for cycles
    if [[ "$visited" == *"|$uc_id|"* ]]; then
        echo -e "${indent}${RED}↺ $uc_id (cycle detected)${NC}"
        return
    fi

    local title="${uc_titles[$uc_id]:-Unknown}"
    local deps="${uc_deps[$uc_id]:-}"

    if [[ -z "$deps" ]]; then
        echo -e "${indent}${GREEN}◉${NC} ${CYAN}$uc_id${NC}: $title"
    else
        echo -e "${indent}${YELLOW}◉${NC} ${CYAN}$uc_id${NC}: $title"
        for dep in $deps; do
            print_tree "$dep" "${indent}  └── " "${visited}|$uc_id|"
        done
    fi
}

# Print reverse tree (what depends on this)
print_reverse_tree() {
    local uc_id="$1"
    local indent="$2"
    local visited="$3"

    # Check for cycles
    if [[ "$visited" == *"|$uc_id|"* ]]; then
        echo -e "${indent}${RED}↺ $uc_id (cycle detected)${NC}"
        return
    fi

    local title="${uc_titles[$uc_id]:-Unknown}"
    local dependents="${reverse_deps[$uc_id]:-}"

    if [[ -z "$dependents" ]]; then
        echo -e "${indent}${GREEN}◉${NC} ${CYAN}$uc_id${NC}: $title ${GRAY}(leaf)${NC}"
    else
        echo -e "${indent}${YELLOW}◉${NC} ${CYAN}$uc_id${NC}: $title"
        for dep in $dependents; do
            echo -e "${indent}  ↑ "
            print_reverse_tree "$dep" "${indent}  " "${visited}|$uc_id|"
        done
    fi
}

# Main
load_dependencies

case "$mode" in
    deps)
        echo "Use Case Dependencies"
        echo "====================="
        echo ""

        if [[ -n "$filter_uc" ]]; then
            if [[ -n "${uc_titles[$filter_uc]:-}" ]]; then
                print_tree "$filter_uc" "" "|"
            else
                echo -e "${RED}UC not found: $filter_uc${NC}"
                exit 1
            fi
        else
            # Show all UCs that have dependencies
            for uc_id in "${!uc_titles[@]}"; do
                deps="${uc_deps[$uc_id]:-}"
                if [[ -n "$deps" ]]; then
                    print_tree "$uc_id" "" "|"
                    echo ""
                fi
            done

            # Show UCs with no dependencies
            echo -e "${GRAY}Root UCs (no dependencies):${NC}"
            for uc_id in "${!uc_titles[@]}"; do
                deps="${uc_deps[$uc_id]:-}"
                if [[ -z "$deps" ]]; then
                    echo -e "  ${GREEN}◉${NC} ${CYAN}$uc_id${NC}: ${uc_titles[$uc_id]}"
                fi
            done
        fi
        ;;

    reverse)
        echo "Reverse Dependencies (what depends on each UC)"
        echo "=============================================="
        echo ""

        if [[ -n "$filter_uc" ]]; then
            if [[ -n "${uc_titles[$filter_uc]:-}" ]]; then
                print_reverse_tree "$filter_uc" "" "|"
            else
                echo -e "${RED}UC not found: $filter_uc${NC}"
                exit 1
            fi
        else
            for uc_id in "${!uc_titles[@]}"; do
                dependents="${reverse_deps[$uc_id]:-}"
                if [[ -n "$dependents" ]]; then
                    echo -e "${CYAN}$uc_id${NC}: ${uc_titles[$uc_id]}"
                    echo "  Depended on by:"
                    for dep in $dependents; do
                        echo -e "    ← ${YELLOW}$dep${NC}: ${uc_titles[$dep]:-Unknown}"
                    done
                    echo ""
                fi
            done
        fi
        ;;

    roots)
        echo "Root UCs (no dependencies - entry points)"
        echo "========================================="
        echo ""

        for uc_id in "${!uc_titles[@]}"; do
            deps="${uc_deps[$uc_id]:-}"
            if [[ -z "$deps" ]]; then
                echo -e "${GREEN}◉${NC} ${CYAN}$uc_id${NC}: ${uc_titles[$uc_id]}"
            fi
        done
        ;;

    leaves)
        echo "Leaf UCs (nothing depends on them - endpoints)"
        echo "=============================================="
        echo ""

        for uc_id in "${!uc_titles[@]}"; do
            dependents="${reverse_deps[$uc_id]:-}"
            if [[ -z "$dependents" ]]; then
                echo -e "${GREEN}◉${NC} ${CYAN}$uc_id${NC}: ${uc_titles[$uc_id]}"
            fi
        done
        ;;
esac

# Summary
echo ""
echo "=============================================="
echo "Total UCs: ${#uc_titles[@]}"
