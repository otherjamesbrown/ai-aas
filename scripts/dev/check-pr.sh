#!/usr/bin/env bash
# Check PR for review issues and failing tests
# Usage: ./scripts/dev/check-pr.sh [PR_NUMBER]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

PR_NUMBER="${1:-}"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 PR Review Checker${NC}"
echo ""

# Check prerequisites
if ! command -v gh >/dev/null 2>&1; then
  echo -e "${RED}❌ GitHub CLI (gh) is required.${NC}"
  echo "   Install: https://cli.github.com/"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo -e "${RED}❌ jq is required for parsing JSON output.${NC}"
  echo "   Install: https://stedolan.github.io/jq/download/"
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo -e "${RED}❌ GitHub CLI not authenticated${NC}"
  echo "   Run: gh auth login"
  exit 1
fi

# If no PR number provided, list open PRs
if [ -z "$PR_NUMBER" ]; then
  echo -e "${YELLOW}No PR number provided. Listing open PRs...${NC}"
  echo ""
  gh pr list --state open --limit 10
  echo ""
  echo "Usage: $0 <PR_NUMBER>"
  echo "Example: $0 14"
  exit 0
fi

echo -e "${GREEN}Checking PR #${PR_NUMBER}...${NC}"
echo ""

# Get PR details
PR_INFO=$(gh pr view $PR_NUMBER --json number,title,state,headRefName,baseRefName,author,url 2>/dev/null || echo "")
if [ -z "$PR_INFO" ]; then
  echo -e "${RED}❌ PR #${PR_NUMBER} not found or not accessible${NC}"
  exit 1
fi

PR_TITLE=$(echo "$PR_INFO" | jq -r '.title')
PR_BRANCH=$(echo "$PR_INFO" | jq -r '.headRefName')
PR_STATE=$(echo "$PR_INFO" | jq -r '.state')
PR_URL=$(echo "$PR_INFO" | jq -r '.url')

if [ "$PR_STATE" != "OPEN" ]; then
  echo -e "${YELLOW}⚠️  PR #${PR_NUMBER} is ${PR_STATE}, not OPEN${NC}"
  exit 0
fi

echo -e "${GREEN}PR: #${PR_NUMBER} - ${PR_TITLE}${NC}"
echo "   Branch: ${PR_BRANCH}"
echo "   URL: ${PR_URL}"
echo ""

# Check current branch
CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "$PR_BRANCH" ]; then
  echo -e "${YELLOW}⚠️  Current branch (${CURRENT_BRANCH}) doesn't match PR branch (${PR_BRANCH})${NC}"
  echo "   Switch to PR branch? (y/n)"
  read -r SWITCH
  if [[ "$SWITCH" =~ ^[Yy]$ ]]; then
    git checkout "$PR_BRANCH"
    echo -e "${GREEN}✓ Switched to ${PR_BRANCH}${NC}"
  else
    echo -e "${YELLOW}Continuing with current branch...${NC}"
  fi
  echo ""
fi

# Step 1: Check review comments
echo -e "${BLUE}[1] Checking review comments...${NC}"
REVIEW_COMMENTS=$(gh pr view $PR_NUMBER --comments 2>/dev/null | grep -i "gemini-code-assist" || echo "")

if [ -n "$REVIEW_COMMENTS" ]; then
  echo -e "${YELLOW}Found Gemini Code Assist review comments:${NC}"
  echo "$REVIEW_COMMENTS" | head -20
  echo ""
  echo -e "${YELLOW}View all comments:${NC}"
  echo "   gh pr view $PR_NUMBER --comments"
else
  echo -e "${GREEN}✓ No Gemini Code Assist comments found${NC}"
fi
echo ""

# Step 2: Check test/CI status
echo -e "${BLUE}[2] Checking test/CI status...${NC}"
CHECK_OUTPUT=$(gh pr checks $PR_NUMBER 2>&1 || echo "")

if [ -n "$CHECK_OUTPUT" ]; then
  echo "$CHECK_OUTPUT"
  
  # Count failed checks more robustly (check for "fail" status, not just the word "fail")
  FAILED_COUNT=$(echo "$CHECK_OUTPUT" | awk '$2 ~ /fail|failure/ {count++} END {print count+0}')
  if [ "${FAILED_COUNT:-0}" -gt 0 ]; then
    echo ""
    echo -e "${RED}⚠️  Found ${FAILED_COUNT} failing check(s)${NC}"
    echo ""
    echo "To view failed test logs:"
    echo "   gh run list --workflow ci.yml --limit 5"
    echo "   gh run view <RUN_ID> --log-failed"
  else
    echo -e "${GREEN}✓ All checks passing${NC}"
  fi
else
  echo -e "${YELLOW}Could not retrieve check status${NC}"
fi
echo ""

# Step 3: Summary
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo -e "${BLUE}Summary${NC}"
echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""
echo "PR: #${PR_NUMBER} - ${PR_TITLE}"
echo "URL: ${PR_URL}"
echo ""

if [ -n "$REVIEW_COMMENTS" ]; then
  echo -e "${YELLOW}📝 Review Issues Found${NC}"
  echo "   Address issues raised by Gemini Code Assist"
  echo "   View: gh pr view $PR_NUMBER --comments"
  echo ""
fi

if [ "$FAILED_COUNT" -gt 0 ]; then
  echo -e "${RED}🧪 Failing Tests Found${NC}"
  echo "   Fix failing tests before merging"
  echo "   Check: gh pr checks $PR_NUMBER"
  echo ""
fi

echo "Next steps:"
echo "1. Review comments: gh pr view $PR_NUMBER --comments"
echo "2. Check tests: gh pr checks $PR_NUMBER"
echo "3. Address issues and comment on PR"
echo "4. Run tests locally: make test SERVICE=<service>"
echo "5. Commit fixes: git commit -m 'fix: Address PR review feedback'"
echo "6. Push: git push origin ${PR_BRANCH}"
echo ""

