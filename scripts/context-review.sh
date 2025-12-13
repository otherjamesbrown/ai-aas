#!/bin/bash
# Context Review Helper Script
# Maps changed files to agent domains and lists context files to check

set -e

BASE_BRANCH="${1:-develop}"

echo "=== Context Review ==="
echo "Comparing against: $BASE_BRANCH"
echo ""

# Get changed files
CHANGED_FILES=$(git diff --name-only "${BASE_BRANCH}...HEAD" 2>/dev/null || git diff --name-only HEAD~10)

if [ -z "$CHANGED_FILES" ]; then
    echo "No changed files found"
    exit 0
fi

echo "Changed files:"
echo "$CHANGED_FILES" | head -20
TOTAL=$(echo "$CHANGED_FILES" | wc -l)
if [ "$TOTAL" -gt 20 ]; then
    echo "... and $((TOTAL - 20)) more"
fi
echo ""

# Map files to agents
declare -A AGENT_FILES
AGENT_FILES=(
    [cli-developer]=0
    [go-services-developer]=0
    [operator-developer]=0
    [infra-ops-manager]=0
    [web-portal-developer]=0
)

while IFS= read -r file; do
    case "$file" in
        services/ai-aas-cli/*)
            ((AGENT_FILES[cli-developer]++)) || true
            ;;
        services/*-service/*|shared/*)
            ((AGENT_FILES[go-services-developer]++)) || true
            ;;
        operators/*)
            ((AGENT_FILES[operator-developer]++)) || true
            ;;
        infra/*|gitops/*|.github/*|services/*/deployments/helm/*)
            ((AGENT_FILES[infra-ops-manager]++)) || true
            ;;
        web-portal/*|web/*)
            ((AGENT_FILES[web-portal-developer]++)) || true
            ;;
        context/*)
            echo "Note: Context file changed: $file"
            ;;
    esac
done <<< "$CHANGED_FILES"

echo "=== Agent Domains Affected ==="
echo ""
printf "%-25s %s\n" "Agent" "Files Changed"
printf "%-25s %s\n" "-----" "-------------"

for agent in cli-developer go-services-developer operator-developer infra-ops-manager web-portal-developer; do
    count=${AGENT_FILES[$agent]}
    if [ "$count" -gt 0 ]; then
        printf "%-25s %d\n" "$agent" "$count"
    fi
done

echo ""
echo "=== Context Files to Review ==="
echo ""

for agent in cli-developer go-services-developer operator-developer infra-ops-manager web-portal-developer; do
    count=${AGENT_FILES[$agent]}
    if [ "$count" -gt 0 ]; then
        context_file="context/${agent}/agents.md"
        if [ -f "$context_file" ]; then
            echo "✓ $context_file (${count} files changed)"
        else
            echo "✗ $context_file (MISSING - ${count} files changed)"
        fi
    fi
done

echo ""
echo "=== Quick Checks ==="
echo ""
echo "Run context-maintainer agent for full review:"
echo "  - Check if api_endpoints need updates (go-services-developer)"
echo "  - Check if aimodel_crd_spec needs updates (operator-developer)"
echo "  - Check if any closed beads were caused by missing context"
echo ""
echo "Recent closed beads:"
bd list --status closed 2>/dev/null | head -5 || echo "(bd command not available)"
