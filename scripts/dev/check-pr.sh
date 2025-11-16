#!/usr/bin/env bash
# Check PR for review issues and failing tests
# Usage: ./scripts/dev/check-pr.sh [PR_NUMBER]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PR_NUMBER="${1:-}"

# --- Colors ---
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

# --- Helper Functions ---
function check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo -e "${RED}❌ Prerequisite missing: '$1' is not installed.${NC}"
    echo "   Please install it to continue."
    exit 1
  fi
}

# --- Main Script ---
echo -e "${BLUE}🔍 PR Review Checker${NC}\n"

# Check prerequisites
check_command "gh"
check_command "jq"

if ! gh auth status >/dev/null 2>&1; then
  echo -e "${RED}❌ GitHub CLI not authenticated.${NC}"
  echo "   Run 'gh auth login' to authenticate."
  exit 1
fi

# If no PR number provided, list open PRs
if [ -z "$PR_NUMBER" ]; then
  echo -e "${YELLOW}No PR number provided. Listing open PRs...${NC}\n"
  gh pr list --state open --limit 10
  echo -e "\nUsage: $0 <PR_NUMBER>\nExample: $0 14"
  exit 0
fi

echo -e "${GREEN}Checking PR #${PR_NUMBER}...${NC}\n"

# Get PR details
PR_INFO=$(gh pr view "$PR_NUMBER" --json number,title,state,headRefName,url,reviews,latestReviews,comments,checks 2>/dev/null || echo "")
if [ -z "$PR_INFO" ]; then
  echo -e "${RED}❌ PR #${PR_NUMBER} not found or not accessible.${NC}"
  exit 1
fi

PR_TITLE=$(echo "$PR_INFO" | jq -r '.title')
PR_BRANCH=$(echo "$PR_INFO" | jq -r '.headRefName')
PR_STATE=$(echo "$PR_INFO" | jq -r '.state')
PR_URL=$(echo "$PR_INFO" | jq -r '.url')

if [ "$PR_STATE" != "OPEN" ]; then
  echo -e "${YELLOW}⚠️  PR #${PR_NUMBER} is ${PR_STATE}, not OPEN.${NC}"
  exit 0
fi

echo -e "${GREEN}PR: #${PR_NUMBER} - ${PR_TITLE}${NC}"
echo "   Branch: ${PR_BRANCH}"
echo -e "   URL: ${PR_URL}\n"

# --- Check review comments ---
echo -e "${BLUE}[1] Checking for review feedback...${NC}"
# Check for any reviews that are not just comments and are requesting changes
CHANGES_REQUESTED=$(echo "$PR_INFO" | jq -r '.latestReviews[] | select(.state == "CHANGES_REQUESTED") | .author.login' | wc -l)

if [ "$CHANGES_REQUESTED" -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Changes have been requested on this PR.${NC}"
    gh pr view "$PR_NUMBER" --comments
else
    echo -e "${GREEN}✓ No outstanding change requests.${NC}"
fi
echo ""

# --- Check test/CI status ---
echo -e "${BLUE}[2] Checking test/CI status...${NC}"
CHECKS=$(echo "$PR_INFO" | jq -r '.checks')

if [ -z "$CHECKS" ] || [ "$CHECKS" == "[]" ] || [ "$CHECKS" == "null" ]; then
    echo -e "${YELLOW}No CI checks found for this PR.${NC}"
else
    gh pr checks "$PR_NUMBER"
    FAILING_CHECKS=$(echo "$CHECKS" | jq -r '.[] | select(.conclusion == "FAILURE" or .conclusion == "TIMED_OUT") | .name' | wc -l)

    if [ "$FAILING_CHECKS" -gt 0 ]; then
        echo -e "\n${RED}❌ Found ${FAILING_CHECKS} failing check(s).${NC}"
    else
        echo -e "\n${GREEN}✓ All checks are passing.${NC}"
    fi
fi
echo ""

# --- Summary ---
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Summary for PR #${PR_NUMBER}${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n"

OVERALL_STATUS="${GREEN}✓ READY TO MERGE${NC}"

if [ "$CHANGES_REQUESTED" -gt 0 ]; then
    echo -e "${YELLOW}📝 Review: Changes requested.${NC}"
    OVERALL_STATUS="${RED}❌ NOT READY${NC}"
else
    echo -e "${GREEN}✓ Review: Approved or no pending reviews.${NC}"
fi

if [ -n "$CHECKS" ] && [ "$FAILING_CHECKS" -gt 0 ]; then
    echo -e "${RED}🧪 CI/CD: ${FAILING_CHECKS} check(s) failing.${NC}"
    OVERALL_STATUS="${RED}❌ NOT READY${NC}"
else
    echo -e "${GREEN}✓ CI/CD: All checks passing.${NC}"
fi

echo -e "\n${BLUE}Overall Status: ${OVERALL_STATUS}\n"

