#!/usr/bin/env bash
#
# Release Script - Create and push a production release tag
#
# Usage:
#   ./scripts/dev/release.sh <version> [notes]
#
# Examples:
#   ./scripts/dev/release.sh v1.0.0
#   ./scripts/dev/release.sh v1.1.0 "Add OpenAI endpoints"
#   ./scripts/dev/release.sh v1.1.1 "Hotfix: Fix authentication bug"
#
# This script:
# 1. Validates the version format (must start with 'v')
# 2. Checks that you're on the main branch
# 3. Ensures main is up to date
# 4. Creates an annotated Git tag
# 5. Pushes the tag to trigger production deployment
#
# After pushing the tag:
# - ArgoCD will detect the new tag
# - Manually sync production apps: argocd app sync <app-name> --revision <tag>

set -euo pipefail

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Usage function
usage() {
  echo "Usage: $0 <version> [release_notes]"
  echo ""
  echo "Examples:"
  echo "  $0 v1.0.0"
  echo "  $0 v1.1.0 'Add OpenAI endpoints'"
  echo "  $0 v1.1.1 'Hotfix: Fix authentication bug'"
  echo ""
  echo "Version must:"
  echo "  - Start with 'v'"
  echo "  - Follow semantic versioning (vMAJOR.MINOR.PATCH)"
  echo "  - Examples: v1.0.0, v1.2.3, v2.0.0"
  exit 1
}

# Check if version is provided
if [ $# -lt 1 ]; then
  echo -e "${RED}❌ Error: Version is required${NC}"
  usage
fi

VERSION="$1"
NOTES="${2:-Release $VERSION}"

# Validate version format
if [[ ! "$VERSION" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo -e "${RED}❌ Error: Invalid version format: $VERSION${NC}"
  echo ""
  echo "Version must follow semantic versioning: vMAJOR.MINOR.PATCH"
  echo "Examples: v1.0.0, v1.2.3, v2.0.0"
  exit 1
fi

echo -e "${BLUE}🚀 Creating Production Release: $VERSION${NC}"
echo ""

# Check if on main branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [ "$CURRENT_BRANCH" != "main" ]; then
  echo -e "${YELLOW}⚠️  Warning: You are on branch '$CURRENT_BRANCH', not 'main'${NC}"
  read -p "Continue anyway? (y/N): " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo -e "${RED}❌ Aborted${NC}"
    exit 1
  fi
fi

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
  echo -e "${RED}❌ Error: Tag $VERSION already exists${NC}"
  echo ""
  echo "Existing tags:"
  git tag -l "v*" | tail -5
  exit 1
fi

# Fetch latest from remote
echo -e "${BLUE}📥 Fetching latest from remote...${NC}"
git fetch origin

# Check if main is up to date
LOCAL=$(git rev-parse main)
REMOTE=$(git rev-parse origin/main)

if [ "$LOCAL" != "$REMOTE" ]; then
  echo -e "${YELLOW}⚠️  Warning: Your main branch is not up to date with origin/main${NC}"
  echo ""
  git log --oneline main..origin/main --no-decorate
  echo ""
  read -p "Pull latest changes? (y/N): " -n 1 -r
  echo
  if [[ $REPLY =~ ^[Yy]$ ]]; then
    git pull origin main
  else
    echo -e "${RED}❌ Aborted${NC}"
    exit 1
  fi
fi

# Show what will be in the release
echo ""
echo -e "${BLUE}📦 Release Details:${NC}"
echo "  Version: $VERSION"
echo "  Notes:   $NOTES"
echo "  Commit:  $(git rev-parse --short HEAD)"
echo ""

# Show recent commits since last tag (if any)
LAST_TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
if [ -n "$LAST_TAG" ]; then
  echo -e "${BLUE}📝 Changes since $LAST_TAG:${NC}"
  git log --oneline --no-decorate "$LAST_TAG"..HEAD | head -10
  echo ""
fi

# Confirm
read -p "Create and push tag $VERSION? (y/N): " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
  echo -e "${RED}❌ Aborted${NC}"
  exit 1
fi

# Create annotated tag
echo -e "${BLUE}🏷️  Creating annotated tag...${NC}"
git tag -a "$VERSION" -m "$NOTES"

# Push tag
echo -e "${BLUE}⬆️  Pushing tag to remote...${NC}"
git push origin "$VERSION"

echo ""
echo -e "${GREEN}✅ Release $VERSION created successfully!${NC}"
echo ""
echo -e "${BLUE}📋 Next Steps:${NC}"
echo ""
echo "1. Wait for CI/CD to build and push Docker images"
echo "   Check: gh run list --workflow ci.yml"
echo ""
echo "2. Manually sync production apps to the new tag:"
echo "   argocd app sync user-org-service-production --revision $VERSION"
echo ""
echo "3. Verify deployment:"
echo "   argocd app get user-org-service-production"
echo "   kubectl get pods -n user-org-service"
echo ""
echo "4. Monitor application health:"
echo "   https://argocd.dev.ai-aas.local"  # Update with your ArgoCD URL
echo ""
echo -e "${BLUE}📊 View release on GitHub:${NC}"
echo "   https://github.com/otherjamesbrown/ai-aas/releases/tag/$VERSION"
echo ""
echo -e "${GREEN}🎉 Happy deploying!${NC}"
