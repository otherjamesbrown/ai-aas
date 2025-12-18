#!/bin/bash
# map-files-to-agents.sh
# Maps changed files to their owning agent domains
# Used by context-reviewer agent and can be used in CI

set -e

# Get base branch (default to develop)
BASE_BRANCH="${1:-develop}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Agent domain mappings
# Format: "pattern:agent"
MAPPINGS=(
    "services/ai-aas-cli/:cli-developer"
    "services/admin-api-service/:go-services-developer"
    "services/api-router-service/:go-services-developer"
    "services/analytics-service/:go-services-developer"
    "services/user-org-service/:go-services-developer"
    "shared-go/:go-services-developer"
    "operators/ai-model-operator/:operator-developer"
    "operators/:operator-developer"
    "infra/:infra-ops-manager"
    "gitops/:infra-ops-manager"
    ".github/workflows/:infra-ops-manager"
    "services/*/deployments/helm/:infra-ops-manager"
    "web-portal/:web-portal-developer"
    "context/:context-meta"
    "docs/:docs-meta"
)

# Get changed files
get_changed_files() {
    if git rev-parse --verify "${BASE_BRANCH}" >/dev/null 2>&1; then
        git diff --name-only "${BASE_BRANCH}"...HEAD 2>/dev/null || \
        git diff --name-only HEAD~10..HEAD 2>/dev/null || \
        git diff --name-only HEAD
    else
        git diff --name-only HEAD~10..HEAD 2>/dev/null || \
        git diff --name-only HEAD
    fi
}

# Map a file to its agent
map_file_to_agent() {
    local file="$1"

    for mapping in "${MAPPINGS[@]}"; do
        local pattern="${mapping%%:*}"
        local agent="${mapping##*:}"

        # Handle glob patterns
        if [[ "$pattern" == *"*"* ]]; then
            # Convert glob to regex-like matching
            local regex_pattern="${pattern//\*/.*}"
            if [[ "$file" =~ ^${regex_pattern} ]]; then
                echo "$agent"
                return
            fi
        elif [[ "$file" == ${pattern}* ]]; then
            echo "$agent"
            return
        fi
    done

    echo "unknown"
}

# Main execution
main() {
    echo -e "${BLUE}=== File to Agent Mapping ===${NC}"
    echo -e "Base branch: ${BASE_BRANCH}"
    echo ""

    # Get changed files
    CHANGED_FILES=$(get_changed_files)

    if [ -z "$CHANGED_FILES" ]; then
        echo -e "${YELLOW}No changed files found${NC}"
        exit 0
    fi

    # Track affected agents
    declare -A AGENT_FILES
    declare -A AGENT_COUNT

    # Map each file
    while IFS= read -r file; do
        [ -z "$file" ] && continue

        agent=$(map_file_to_agent "$file")

        # Append file to agent's list
        if [ -n "${AGENT_FILES[$agent]}" ]; then
            AGENT_FILES[$agent]="${AGENT_FILES[$agent]}"$'\n'"  - $file"
        else
            AGENT_FILES[$agent]="  - $file"
        fi

        # Count files per agent
        ((AGENT_COUNT[$agent]++)) || AGENT_COUNT[$agent]=1

    done <<< "$CHANGED_FILES"

    # Output results by agent
    echo -e "${GREEN}Affected Agents:${NC}"
    echo ""

    for agent in "${!AGENT_FILES[@]}"; do
        count="${AGENT_COUNT[$agent]}"

        case "$agent" in
            "cli-developer")
                color="${GREEN}"
                ;;
            "go-services-developer")
                color="${BLUE}"
                ;;
            "operator-developer")
                color="${YELLOW}"
                ;;
            "infra-ops-manager")
                color="${RED}"
                ;;
            "web-portal-developer")
                color="${BLUE}"
                ;;
            *)
                color="${NC}"
                ;;
        esac

        echo -e "${color}${agent}${NC} (${count} files):"
        echo -e "${AGENT_FILES[$agent]}"
        echo ""
    done

    # Output context files to check
    echo -e "${BLUE}=== Context Files to Review ===${NC}"
    for agent in "${!AGENT_FILES[@]}"; do
        if [ "$agent" != "unknown" ] && [ "$agent" != "context-meta" ] && [ "$agent" != "docs-meta" ]; then
            context_file="context/${agent}/agents.md"
            if [ -f "$context_file" ]; then
                echo -e "  ${GREEN}[EXISTS]${NC} $context_file"
            else
                echo -e "  ${RED}[MISSING]${NC} $context_file"
            fi
        fi
    done

    echo ""
    echo -e "${BLUE}=== Summary ===${NC}"
    echo "Total files changed: $(echo "$CHANGED_FILES" | wc -l)"
    echo "Agents affected: ${#AGENT_FILES[@]}"

    # Output JSON for programmatic use
    if [ "$2" == "--json" ]; then
        echo ""
        echo -e "${BLUE}=== JSON Output ===${NC}"
        echo "{"
        first=true
        for agent in "${!AGENT_COUNT[@]}"; do
            if [ "$first" = true ]; then
                first=false
            else
                echo ","
            fi
            echo -n "  \"$agent\": ${AGENT_COUNT[$agent]}"
        done
        echo ""
        echo "}"
    fi
}

main "$@"
