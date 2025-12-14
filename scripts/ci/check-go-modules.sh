#!/usr/bin/env bash
#
# check-go-modules.sh - Verify Go module path consistency across the repository
#
# This script ensures that all Go modules follow the correct naming conventions
# and import patterns to prevent module path mismatches that can cause build failures.
#
# Expected patterns:
# - Services: github.com/otherjamesbrown/ai-aas/services/<service-name>
# - Shared: github.com/ai-aas/shared-go
# - Operators: github.com/ai-aas/<operator-name>
#
# Exit codes:
#   0 - All checks passed
#   1 - Module path violations found

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ERRORS=0

echo "Checking Go module path consistency..."
echo ""

# Expected patterns
EXPECTED_SERVICE_PREFIX="github.com/otherjamesbrown/ai-aas/services/"
EXPECTED_SHARED_MODULE="github.com/ai-aas/shared-go"
EXPECTED_OPERATOR_PREFIX="github.com/ai-aas/"

# Track all modules found
declare -a ALL_MODULES=()

# Function to check a go.mod file
check_gomod() {
    local gomod_path="$1"
    local dir="$(dirname "$gomod_path")"
    local rel_dir="${dir#$REPO_ROOT/}"

    # Extract module name from go.mod
    local module_name
    module_name=$(grep -E '^module ' "$gomod_path" | awk '{print $2}')

    if [ -z "$module_name" ]; then
        echo -e "${RED}ERROR${NC}: No module declaration found in $rel_dir/go.mod"
        ((ERRORS++))
        return
    fi

    ALL_MODULES+=("$module_name")

    # Determine expected module path based on location
    local expected_module=""
    local category=""

    if [[ "$rel_dir" == services/_template ]]; then
        # Skip template validation (it's intentionally generic)
        echo -e "${YELLOW}SKIP${NC}: Template service at $rel_dir"
        return
    elif [[ "$rel_dir" == services/* ]]; then
        category="service"
        local service_name="${rel_dir#services/}"
        expected_module="${EXPECTED_SERVICE_PREFIX}${service_name}"
    elif [[ "$rel_dir" == shared/go ]]; then
        category="shared"
        expected_module="$EXPECTED_SHARED_MODULE"
    elif [[ "$rel_dir" == operators/* ]]; then
        category="operator"
        local operator_name="${rel_dir#operators/}"
        expected_module="${EXPECTED_OPERATOR_PREFIX}${operator_name}"
    else
        echo -e "${YELLOW}WARN${NC}: Unknown module location: $rel_dir"
        return
    fi

    # Check if module name matches expected pattern
    if [ "$module_name" != "$expected_module" ]; then
        echo -e "${RED}ERROR${NC}: Module path mismatch in $rel_dir/go.mod"
        echo -e "  Expected: ${GREEN}$expected_module${NC}"
        echo -e "  Found:    ${RED}$module_name${NC}"
        ((ERRORS++))
    else
        echo -e "${GREEN}✓${NC} $rel_dir: $module_name"
    fi
}

# Function to check imports in Go source files
check_imports() {
    local service_dir="$1"
    local rel_dir="${service_dir#$REPO_ROOT/}"

    # Only check services (they import shared)
    if [[ "$rel_dir" != services/* ]] || [[ "$rel_dir" == services/_template ]]; then
        return
    fi

    # Find all .go files (excluding vendor and test files for import pattern check)
    local go_files
    go_files=$(find "$service_dir" -type f -name "*.go" -not -path "*/vendor/*" 2>/dev/null || true)

    if [ -z "$go_files" ]; then
        return
    fi

    # Check for incorrect shared module imports
    local bad_imports
    bad_imports=$(echo "$go_files" | xargs grep -l 'github.com/otherjamesbrown/ai-aas/shared/go' 2>/dev/null || true)

    if [ -n "$bad_imports" ]; then
        echo -e "${RED}ERROR${NC}: Incorrect shared module import in $rel_dir"
        echo -e "  Expected import: ${GREEN}github.com/ai-aas/shared-go${NC}"
        echo -e "  Found incorrect: ${RED}github.com/otherjamesbrown/ai-aas/shared/go${NC}"
        echo ""
        echo "  Files with incorrect imports:"
        echo "$bad_imports" | while read -r file; do
            echo -e "    - ${file#$REPO_ROOT/}"
        done
        echo ""
        ((ERRORS++))
    fi
}

# Function to check replace directives
check_replace_directives() {
    local gomod_path="$1"
    local dir="$(dirname "$gomod_path")"
    local rel_dir="${dir#$REPO_ROOT/}"

    # Only check services (they should have replace directives for shared)
    if [[ "$rel_dir" != services/* ]] || [[ "$rel_dir" == services/_template ]]; then
        return
    fi

    # Check if go.mod imports shared module
    if grep -q "github.com/ai-aas/shared-go" "$gomod_path" 2>/dev/null; then
        # Check if replace directive exists and is correct
        local replace_directive
        replace_directive=$(grep -E '^replace github.com/ai-aas/shared-go' "$gomod_path" || true)

        if [ -z "$replace_directive" ]; then
            echo -e "${RED}ERROR${NC}: Missing replace directive in $rel_dir/go.mod"
            echo -e "  Expected: ${GREEN}replace github.com/ai-aas/shared-go => ../../shared/go${NC}"
            ((ERRORS++))
        else
            # Verify the replace directive points to correct path
            if ! echo "$replace_directive" | grep -q "=> ../../shared/go"; then
                echo -e "${RED}ERROR${NC}: Incorrect replace directive in $rel_dir/go.mod"
                echo -e "  Expected: ${GREEN}replace github.com/ai-aas/shared-go => ../../shared/go${NC}"
                echo -e "  Found:    ${RED}$replace_directive${NC}"
                ((ERRORS++))
            fi
        fi
    fi

    # Check for old/incorrect replace directives
    local old_replace
    old_replace=$(grep -E '^replace github.com/otherjamesbrown/ai-aas/shared/go' "$gomod_path" || true)

    if [ -n "$old_replace" ]; then
        echo -e "${RED}ERROR${NC}: Obsolete replace directive in $rel_dir/go.mod"
        echo -e "  Found:    ${RED}$old_replace${NC}"
        echo -e "  Should be: ${GREEN}replace github.com/ai-aas/shared-go => ../../shared/go${NC}"
        ((ERRORS++))
    fi
}

# Find all go.mod files
echo "Scanning for go.mod files..."
echo ""

while IFS= read -r gomod_file; do
    check_gomod "$gomod_file"
done < <(find "$REPO_ROOT" -name "go.mod" -not -path "*/vendor/*" -not -path "*/node_modules/*")

echo ""
echo "Checking import statements..."
echo ""

# Check imports in all services
while IFS= read -r gomod_file; do
    service_dir="$(dirname "$gomod_file")"
    check_imports "$service_dir"
done < <(find "$REPO_ROOT/services" -name "go.mod" -not -path "*/vendor/*")

echo ""
echo "Checking replace directives..."
echo ""

# Check replace directives
while IFS= read -r gomod_file; do
    check_replace_directives "$gomod_file"
done < <(find "$REPO_ROOT" -name "go.mod" -not -path "*/vendor/*" -not -path "*/node_modules/*")

echo ""
echo "========================================="

if [ $ERRORS -eq 0 ]; then
    echo -e "${GREEN}✓ All Go module paths are consistent!${NC}"
    echo ""
    echo "Modules checked:"
    for module in "${ALL_MODULES[@]}"; do
        echo "  - $module"
    done
    exit 0
else
    echo -e "${RED}✗ Found $ERRORS module path error(s)${NC}"
    echo ""
    echo "Please fix the errors above to ensure build consistency."
    echo ""
    echo "Expected patterns:"
    echo "  Services:  $EXPECTED_SERVICE_PREFIX<service-name>"
    echo "  Shared:    $EXPECTED_SHARED_MODULE"
    echo "  Operators: $EXPECTED_OPERATOR_PREFIX<operator-name>"
    echo ""
    echo "For services importing shared module, use:"
    echo "  import \"github.com/ai-aas/shared-go/...\""
    echo "  replace github.com/ai-aas/shared-go => ../../shared/go"
    exit 1
fi
