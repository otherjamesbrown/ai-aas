#!/usr/bin/env bash
set -euo pipefail

# Deploy Changes to Environments
# Usage: ./scripts/dev/deploy.sh [development|production]

ENVIRONMENT="${1:-development}"
BRANCH=$(git branch --show-current)

echo "🚀 Deploying to ${ENVIRONMENT} environment"
echo "   Branch: ${BRANCH}"
echo ""

# Step 1: Review changes
echo "📋 Step 1: Reviewing changes..."
git status --short
echo ""

# Step 2: Commit (if there are changes)
if [ -n "$(git status --porcelain)" ]; then
  echo "💾 Step 2: Committing changes..."
  git add .
  read -p "Enter commit message (or press Enter for default): " COMMIT_MSG
  if [ -z "$COMMIT_MSG" ]; then
    COMMIT_MSG="feat: Deploy changes to ${ENVIRONMENT}

- Auto-deployed via deploy script
- Timestamp: $(date -Iseconds)"
  fi
  git commit -m "$COMMIT_MSG"
else
  echo "✅ No changes to commit"
fi

# Step 3: Push
echo "📤 Step 3: Pushing to git..."
git push origin "${BRANCH}"

# Step 4: Trigger CI (if not on main)
if [ "${BRANCH}" != "main" ]; then
  echo "🔄 Step 4: Triggering CI pipeline..."
  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    make ci-remote SERVICE=all NOTES="Auto-deploy to ${ENVIRONMENT}"
  else
    echo "⚠️  GitHub CLI not authenticated, skipping CI trigger"
    echo "   Run: gh auth login"
  fi
fi

# Step 5: Sync ArgoCD
if [ "${ENVIRONMENT}" = "development" ]; then
  echo "🔄 Step 5: Syncing ArgoCD (development)..."
  export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
  
  if command -v argocd >/dev/null 2>&1; then
    # Check if already logged in
    if argocd account can-i get applications '*' >/dev/null 2>&1; then
      argocd app sync web-portal-development 2>/dev/null || true
      argocd app sync api-router-service-development 2>/dev/null || true
    else
      echo "⚠️  ArgoCD login required"
      echo "   Run: argocd login argocd.dev.ai-aas.local --grpc-web --insecure"
    fi
  else
    echo "⚠️  ArgoCD CLI not found, skipping sync"
  fi
elif [ "${ENVIRONMENT}" = "production" ]; then
  echo "🔄 Step 5: Production sync requires manual approval"
  export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml
  echo "   Run: argocd app sync web-portal-production"
fi

# Step 6: Verify
echo "✅ Step 6: Verifying deployment..."
export KUBECONFIG=~/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml
if kubectl cluster-info >/dev/null 2>&1; then
  kubectl get ingress --all-namespaces 2>/dev/null || echo "⚠️  Could not verify ingress"
else
  echo "⚠️  Could not connect to cluster"
fi

echo ""
echo "✅ Deployment initiated!"
echo ""
echo "📋 Next steps:"
echo "   1. Monitor CI: gh run list --workflow ci.yml"
echo "   2. Check ArgoCD: argocd app list"
echo "   3. Verify services: kubectl get pods --all-namespaces"

