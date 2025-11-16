#!/usr/bin/env bash
set -euo pipefail

# Deploy Changes to Environments via GitOps (ArgoCD)
# Usage: ./scripts/dev/deploy.sh [development|production]
#
# This script follows the GitOps workflow:
# 1. Commits changes to current branch
# 2. Creates PR if not on main (for code review)
# 3. After PR merge, ArgoCD auto-syncs from main
# 4. Verifies deployment

ENVIRONMENT="${1:-development}"
BRANCH=$(git branch --show-current)
REPO=$(git config --get remote.origin.url | sed -E 's|.*github.com[:/]([^/]+/[^/]+)(\.git)?$|\1|')

echo "🚀 Deploying to ${ENVIRONMENT} environment"
echo "   Branch: ${BRANCH}"
echo "   Repo: ${REPO}"
echo ""

# Check prerequisites
if ! command -v gh >/dev/null 2>&1; then
  echo "❌ GitHub CLI (gh) is required."
  echo "   Install: https://cli.github.com/"
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "❌ GitHub CLI not authenticated"
  echo "   Run: gh auth login"
  exit 1
fi

# Step 1: Review changes
echo "📋 Step 1: Reviewing changes..."
CHANGES=$(git status --porcelain)
if [ -z "$CHANGES" ]; then
  echo "✅ No changes to commit"
  echo ""
else
  echo "Changes to be committed:"
  git status --short
  echo ""
fi

# Generate defaults (needed for both commit and PR)
DEFAULT_COMMIT_MSG="feat: Deploy changes to ${ENVIRONMENT}

- Auto-deployed via deploy script
- Timestamp: $(date -Iseconds)"

DEFAULT_PR_TITLE=$(git log -1 --pretty=%s 2>/dev/null || echo "feat: Deploy changes to ${ENVIRONMENT}")
DEFAULT_PR_BODY="Deploying changes to ${ENVIRONMENT} environment

- Auto-created via deploy script
- After merge, ArgoCD will auto-sync from main branch"

# Step 2: Commit (if there are changes)
if [ -n "$CHANGES" ]; then
  echo "💾 Step 2: Committing changes..."
  git add .
  
  # Show defaults and ask for confirmation
  echo ""
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Default Commit Message:"
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "${DEFAULT_COMMIT_MSG}"
  echo ""
  
  if [ "${BRANCH}" != "main" ]; then
    echo "Default PR Title:"
    echo "${DEFAULT_PR_TITLE}"
    echo ""
    echo "Default PR Description:"
    echo "${DEFAULT_PR_BODY}"
    echo ""
  fi
  
  echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
  echo "Use these defaults? (y/n, or press Enter for yes):"
  read -r USE_DEFAULTS
  
  USE_DEFAULTS=${USE_DEFAULTS:-y}
  
  if [[ "${USE_DEFAULTS}" =~ ^[Yy]$ ]]; then
    COMMIT_MSG="$DEFAULT_COMMIT_MSG"
    PR_TITLE="$DEFAULT_PR_TITLE"
    PR_BODY="$DEFAULT_PR_BODY"
    echo "✅ Using defaults"
  else
    # Prompt for custom values
    echo ""
    echo "Enter commit message:"
    read -r COMMIT_MSG
    if [ -z "$COMMIT_MSG" ]; then
      COMMIT_MSG="$DEFAULT_COMMIT_MSG"
    fi
    
    if [ "${BRANCH}" != "main" ]; then
      echo ""
      echo "Enter PR title (or press Enter for default):"
      read -r PR_TITLE
      if [ -z "$PR_TITLE" ]; then
        PR_TITLE="$DEFAULT_PR_TITLE"
      fi
      
      echo ""
      echo "Enter PR description (or press Enter for default):"
      read -r PR_BODY
      if [ -z "$PR_BODY" ]; then
        PR_BODY="$DEFAULT_PR_BODY"
      fi
    fi
  fi
  
  git commit -m "$COMMIT_MSG"
  echo "✅ Committed"
  echo ""
fi

# Step 3: Push
echo "📤 Step 3: Pushing to git..."
if ! git push origin "${BRANCH}" 2>/dev/null; then
  echo "⚠️  Push failed. Setting upstream..."
  git push -u origin "${BRANCH}"
fi
echo "✅ Pushed to origin/${BRANCH}"
echo ""

# Step 4: Handle branch vs main
if [ "${BRANCH}" = "main" ]; then
  echo "✅ On main branch - ArgoCD will auto-sync"
  echo ""
  
  # Trigger CI (automatic on push to main, but we can verify)
  echo "🔄 Step 4: CI will trigger automatically on push to main"
  echo ""
  
  # Step 5: Wait for ArgoCD sync
  echo "⏳ Step 5: Waiting for ArgoCD to detect changes..."
  echo "   ArgoCD watches main branch and auto-syncs within 30-60 seconds"
  echo ""
  
  if [ "${ENVIRONMENT}" = "development" ]; then
    export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
    
    echo "Waiting for ArgoCD sync (checking every 10 seconds, max 2 minutes)..."
    for i in {1..12}; do
      sleep 10
      LATEST_COMMIT=$(git rev-parse HEAD)
      SYNC_REV=$(kubectl get application web-portal-development -n argocd -o jsonpath='{.status.sync.revision}' 2>/dev/null || echo "")
      
      if [ "$SYNC_REV" = "$LATEST_COMMIT" ]; then
        echo "✅ ArgoCD synced to latest commit"
        break
      fi
      
      if [ $i -eq 12 ]; then
        echo "⚠️  ArgoCD hasn't synced yet. Check manually:"
        echo "   kubectl get application -n argocd"
        echo "   Or visit: https://argocd.dev.ai-aas.local"
      else
        echo "   Waiting... ($((i * 10))s)"
      fi
    done
  fi
else
  # Not on main - create PR
  echo "📝 Step 4: Creating Pull Request for code review..."
  echo ""
  
  # Check if PR already exists
  EXISTING_PR=$(gh pr list --head "${BRANCH}" --json number --jq '.[0].number' 2>/dev/null || echo "")
  
  if [ -n "$EXISTING_PR" ] && [ "$EXISTING_PR" != "null" ]; then
    echo "✅ PR already exists: #${EXISTING_PR}"
    PR_URL=$(gh pr view "${EXISTING_PR}" --json url --jq '.url')
    echo "   ${PR_URL}"
    echo ""
    echo "📋 Next steps:"
    echo "   1. Review and merge the PR: ${PR_URL}"
    echo "   2. After merge, ArgoCD will auto-sync from main"
    echo "   3. Monitor ArgoCD: https://argocd.dev.ai-aas.local"
  else
    # Create PR (using values from commit step or defaults)
    echo "Creating PR..."
    PR_ARGS=(
      "--title" "${PR_TITLE:-${DEFAULT_PR_TITLE}}"
      "--base" "main"
      "--head" "${BRANCH}"
      "--body" "${PR_BODY:-${DEFAULT_PR_BODY}}"
    )
    
    if PR_URL=$(gh pr create "${PR_ARGS[@]}" 2>&1); then
      echo "✅ PR created: ${PR_URL}"
      echo ""
      echo "📋 Next steps:"
      echo "   1. Review the PR: ${PR_URL}"
      echo "   2. Merge the PR (after review/approval)"
      echo "   3. After merge, ArgoCD will auto-sync from main"
      echo "   4. Monitor ArgoCD: https://argocd.dev.ai-aas.local"
    else
      echo "❌ Failed to create PR"
      echo "   Output: ${PR_URL}"
      exit 1
    fi
  fi
  
  # Trigger CI for the branch
  echo ""
  echo "🔄 Step 5: Triggering CI pipeline for branch..."
  if make ci-remote SERVICE=all NOTES="Deploy to ${ENVIRONMENT} via PR" 2>&1 | grep -q "Workflow queued"; then
    echo "✅ CI pipeline triggered"
  else
    echo "⚠️  CI trigger may have failed, check manually"
  fi
fi

# Step 6: Verify deployment (if on main or after sync)
if [ "${BRANCH}" = "main" ] || [ "${ENVIRONMENT}" = "development" ]; then
  echo ""
  echo "✅ Step 6: Verifying deployment..."
  export KUBECONFIG=~/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml
  
  if kubectl cluster-info >/dev/null 2>&1; then
    echo ""
    echo "📊 Ingress Resources:"
    kubectl get ingress --all-namespaces 2>/dev/null | head -10 || echo "⚠️  Could not list ingress"
    
    echo ""
    echo "📊 Pod Status:"
    kubectl get pods -n development 2>/dev/null | head -10 || echo "⚠️  Could not list pods"
  else
    echo "⚠️  Could not connect to cluster"
    echo "   Check kubeconfig: ~/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml"
  fi
fi

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ Deployment workflow initiated!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "${BRANCH}" != "main" ]; then
  echo "📋 Current Status:"
  echo "   - Changes committed and pushed to ${BRANCH}"
  echo "   - PR created/updated for code review"
  echo "   - CI pipeline triggered"
  echo ""
  echo "⏭️  Next Steps:"
  echo "   1. Review PR: $(gh pr list --head "${BRANCH}" --json url --jq '.[0].url' 2>/dev/null || echo 'Check GitHub')"
  echo "   2. Merge PR after approval"
  echo "   3. ArgoCD will auto-sync from main (development) or manual sync (production)"
  echo "   4. Monitor: https://argocd.dev.ai-aas.local"
else
  echo "📋 Current Status:"
  echo "   - Changes committed and pushed to main"
  echo "   - ArgoCD will auto-sync (development) or manual sync (production)"
  echo ""
  echo "📊 Monitor:"
  echo "   - CI Status: gh run list --workflow ci.yml"
  echo "   - ArgoCD: https://argocd.dev.ai-aas.local"
  echo "   - Services: kubectl get pods --all-namespaces"
fi

echo ""
echo "📚 Documentation:"
echo "   - GitOps Workflow: docs/runbooks/argocd-deployment-workflow.md"
echo "   - Deployment Guide: docs/runbooks/deploy-to-environments.md"
