#!/usr/bin/env bash
# Verify that GitHub Actions CI workflow triggered after git push
#
# Usage:
#   scripts/ci/verify-workflow-triggered.sh [branch] [workflow-name]
#
# Arguments:
#   branch: Git branch to check (default: current branch)
#   workflow-name: Workflow name to check (default: "CI")
#
# Exit codes:
#   0: CI workflow triggered successfully
#   1: CI workflow not found or failed to trigger
#   2: Missing dependencies (gh CLI)

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BRANCH="${1:-$(git rev-parse --abbrev-ref HEAD)}"
WORKFLOW_NAME="${2:-CI}"
MAX_WAIT_SECONDS=30
POLL_INTERVAL=2

# Verify gh CLI is installed
if ! command -v gh &> /dev/null; then
    echo -e "${RED}ERROR: GitHub CLI (gh) is not installed${NC}"
    echo "Install from: https://cli.github.com/"
    exit 2
fi

# Get the latest commit hash (short form)
COMMIT_SHA_SHORT=$(git rev-parse --short HEAD)
COMMIT_SHA_FULL=$(git rev-parse HEAD)

echo "Verifying CI workflow triggered for:"
echo "  Branch: $BRANCH"
echo "  Commit: $COMMIT_SHA_SHORT"
echo "  Workflow: $WORKFLOW_NAME"
echo ""

# Function to check if workflow is triggered
check_workflow() {
    gh run list \
        --limit 10 \
        --branch "$BRANCH" \
        --workflow "$WORKFLOW_NAME" \
        --json headSha,status,conclusion,createdAt,displayTitle \
        --jq ".[] | select(.headSha == \"$COMMIT_SHA_FULL\")"
}

# Poll for workflow to appear
echo "Waiting for workflow to trigger (max ${MAX_WAIT_SECONDS}s)..."
ELAPSED=0
WORKFLOW_FOUND=false

while [ $ELAPSED -lt $MAX_WAIT_SECONDS ]; do
    WORKFLOW_DATA=$(check_workflow)

    if [ -n "$WORKFLOW_DATA" ]; then
        WORKFLOW_FOUND=true
        break
    fi

    echo -n "."
    sleep $POLL_INTERVAL
    ELAPSED=$((ELAPSED + POLL_INTERVAL))
done

echo "" # New line after dots

if [ "$WORKFLOW_FOUND" = false ]; then
    echo -e "${RED}FAILED: No CI workflow found for commit $COMMIT_SHA_SHORT${NC}"
    echo ""
    echo "Possible reasons:"
    echo "  1. Push didn't trigger CI (check workflow path filters)"
    echo "  2. GitHub Actions is experiencing delays"
    echo "  3. Workflow file has syntax errors"
    echo ""
    echo "Recent workflows on branch $BRANCH:"
    gh run list --limit 5 --branch "$BRANCH" --workflow "$WORKFLOW_NAME"
    echo ""
    echo "To manually trigger:"
    echo "  gh workflow run \"$WORKFLOW_NAME\" --ref $BRANCH"
    exit 1
fi

# Extract workflow details
STATUS=$(echo "$WORKFLOW_DATA" | jq -r '.status')
CONCLUSION=$(echo "$WORKFLOW_DATA" | jq -r '.conclusion')
DISPLAY_TITLE=$(echo "$WORKFLOW_DATA" | jq -r '.displayTitle')

echo -e "${GREEN}SUCCESS: CI workflow triggered${NC}"
echo ""
echo "Workflow Details:"
echo "  Title: $DISPLAY_TITLE"
echo "  Status: $STATUS"
if [ "$CONCLUSION" != "null" ]; then
    echo "  Conclusion: $CONCLUSION"
fi
echo ""

# Show workflow URL
WORKFLOW_URL=$(gh run list --limit 1 --branch "$BRANCH" --workflow "$WORKFLOW_NAME" --json url --jq '.[0].url')
echo "View workflow: $WORKFLOW_URL"

# If workflow is already completed, show result
if [ "$STATUS" = "completed" ]; then
    if [ "$CONCLUSION" = "success" ]; then
        echo -e "${GREEN}Workflow completed successfully${NC}"
        exit 0
    else
        echo -e "${RED}Workflow completed with status: $CONCLUSION${NC}"
        exit 1
    fi
fi

# Workflow is still running
echo -e "${YELLOW}Workflow is still $STATUS...${NC}"
echo "Monitor with: gh run watch"
exit 0
