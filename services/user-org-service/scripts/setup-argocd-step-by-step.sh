#!/usr/bin/env bash
# Interactive step-by-step script to set up ArgoCD for user-org-service.
#
# This script guides you through each step and pauses for confirmation.
# Run it step by step, or let it guide you through the entire process.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
PROJECT_ROOT="$(cd "$SERVICE_ROOT/../.." && pwd)"

echo "=========================================="
echo "ArgoCD Setup for User-Org-Service"
echo "=========================================="
echo ""
echo "This script will guide you through:"
echo "  1. Verifying prerequisites"
echo "  2. Installing/verifying ArgoCD"
echo "  3. Registering the Git repository"
echo "  4. Creating the ArgoCD Application"
echo "  5. Setting up secrets"
echo "  6. Testing the deployment"
echo ""
read -p "Press Enter to continue or Ctrl+C to exit..."

# Step 0: Verify kubectl access
echo ""
echo "=========================================="
echo "STEP 0: Verify Kubernetes Access"
echo "=========================================="
echo ""
echo "Checking kubectl contexts..."
kubectl config get-contexts

echo ""
read -p "Enter your development cluster context name (or press Enter to use current): " KUBE_CONTEXT
if [ -z "$KUBE_CONTEXT" ]; then
  KUBE_CONTEXT=$(kubectl config current-context)
fi

echo "Using context: $KUBE_CONTEXT"
kubectl --context="$KUBE_CONTEXT" cluster-info || {
  echo "❌ Cannot access cluster. Please configure kubectl first."
  exit 1
}
echo "✅ Cluster access verified!"
echo ""

# Step 1: Check ArgoCD
echo "=========================================="
echo "STEP 1: Check ArgoCD Installation"
echo "=========================================="
echo ""

if kubectl --context="$KUBE_CONTEXT" get namespace argocd >/dev/null 2>&1; then
  echo "✅ ArgoCD namespace exists"
  
  if kubectl --context="$KUBE_CONTEXT" -n argocd get deployment argocd-server >/dev/null 2>&1; then
    echo "✅ ArgoCD is installed"
    ARGOCD_INSTALLED=true
  else
    echo "⚠️  ArgoCD namespace exists but server not found"
    ARGOCD_INSTALLED=false
  fi
else
  echo "❌ ArgoCD not installed"
  ARGOCD_INSTALLED=false
fi

if [ "$ARGOCD_INSTALLED" = "false" ]; then
  echo ""
  read -p "Install ArgoCD now? (y/n): " INSTALL_ARGOCD
  if [ "$INSTALL_ARGOCD" = "y" ]; then
    echo "Installing ArgoCD..."
    kubectl --context="$KUBE_CONTEXT" create namespace argocd || true
    
    helm repo add argo https://argoproj.github.io/argo-helm
    helm repo update
    
    VALUES_FILE="$PROJECT_ROOT/gitops/templates/argocd-values.yaml"
    if [ -f "$VALUES_FILE" ]; then
      helm install argocd argo/argo-cd \
        --namespace argocd \
        --kube-context="$KUBE_CONTEXT" \
        --values "$VALUES_FILE"
    else
      helm install argocd argo/argo-cd \
        --namespace argocd \
        --kube-context="$KUBE_CONTEXT"
    fi
    
    echo "Waiting for ArgoCD to be ready (this may take 2-3 minutes)..."
    kubectl --context="$KUBE_CONTEXT" -n argocd wait --for=condition=available --timeout=5m deployment/argocd-server || {
      echo "⚠️  ArgoCD installation may still be in progress. Continue anyway? (y/n)"
      read -p "> " CONTINUE
      [ "$CONTINUE" = "y" ] || exit 1
    }
    echo "✅ ArgoCD installed!"
  else
    echo "Skipping ArgoCD installation. Please install it manually and re-run this script."
    exit 1
  fi
fi

# Get ArgoCD admin password
echo ""
echo "Getting ArgoCD admin password..."
ARGOCD_PASSWORD=$(kubectl --context="$KUBE_CONTEXT" -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" 2>/dev/null | base64 -d || echo "")
if [ -n "$ARGOCD_PASSWORD" ]; then
  echo "✅ ArgoCD admin password retrieved"
  echo "   Username: admin"
  echo "   Password: $ARGOCD_PASSWORD"
  echo "   (Save this password!)"
else
  echo "⚠️  Could not retrieve password. You may need to reset it."
fi

# Step 2: Check ArgoCD CLI
echo ""
echo "=========================================="
echo "STEP 2: Check ArgoCD CLI"
echo "=========================================="
echo ""

if command -v argocd >/dev/null 2>&1; then
  echo "✅ ArgoCD CLI is installed"
  ARGOCD_CLI_INSTALLED=true
else
  echo "❌ ArgoCD CLI not found"
  ARGOCD_CLI_INSTALLED=false
  echo ""
  echo "Install ArgoCD CLI:"
  echo "  macOS: brew install argocd"
  echo "  Linux: curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64 && chmod +x /usr/local/bin/argocd"
  read -p "Press Enter after installing ArgoCD CLI, or Ctrl+C to exit..."
fi

# Step 3: Login to ArgoCD
echo ""
echo "=========================================="
echo "STEP 3: Login to ArgoCD"
echo "=========================================="
echo ""

# Try to get ArgoCD server address
ARGOCD_SERVER=$(kubectl --context="$KUBE_CONTEXT" -n argocd get svc argocd-server -o jsonpath='{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null || echo "")

if [ -z "$ARGOCD_SERVER" ]; then
  echo "No load balancer found. You'll need to use port-forward."
  echo ""
  echo "In another terminal, run:"
  echo "  kubectl --context=$KUBE_CONTEXT port-forward svc/argocd-server -n argocd 8080:443"
  echo ""
  read -p "Press Enter after starting port-forward..."
  ARGOCD_SERVER="localhost:8080"
fi

echo "Logging into ArgoCD at $ARGOCD_SERVER..."
if [ -n "$ARGOCD_PASSWORD" ]; then
  echo "$ARGOCD_PASSWORD" | argocd login "$ARGOCD_SERVER" --username admin --insecure --grpc-web || {
    echo "⚠️  Login failed. You may need to login manually:"
    echo "   argocd login $ARGOCD_SERVER --insecure --grpc-web"
    read -p "Press Enter after logging in manually..."
  }
else
  echo "Please login manually:"
  echo "  argocd login $ARGOCD_SERVER --insecure --grpc-web"
  read -p "Press Enter after logging in..."
fi

# Step 4: Register Repository
echo ""
echo "=========================================="
echo "STEP 4: Register Git Repository"
echo "=========================================="
echo ""

REPO_URL="https://github.com/otherjamesbrown/ai-aas"
echo "Repository URL: $REPO_URL"
echo ""

# Check if repo is already registered
if argocd repo list | grep -q "$REPO_URL"; then
  echo "✅ Repository already registered"
else
  echo "Repository not registered. Adding..."
  echo ""
  echo "For a public repository, you can add it without credentials."
  echo "For a private repository, you'll need a GitHub token."
  echo ""
  read -p "Is this a private repository? (y/n): " IS_PRIVATE
  
  if [ "$IS_PRIVATE" = "y" ]; then
    echo "You'll need a GitHub Personal Access Token with 'repo' scope."
    echo "Create one at: https://github.com/settings/tokens"
    read -p "Enter your GitHub token: " GITHUB_TOKEN
    read -p "Enter your GitHub username: " GITHUB_USERNAME
    
    argocd repo add "$REPO_URL" \
      --type git \
      --username "$GITHUB_USERNAME" \
      --password "$GITHUB_TOKEN" \
      --name ai-aas \
      --insecure-skip-server-verification || {
      echo "❌ Failed to add repository"
      exit 1
    }
  else
    argocd repo add "$REPO_URL" --name ai-aas || {
      echo "❌ Failed to add repository"
      exit 1
    }
  fi
  echo "✅ Repository registered!"
fi

# Step 5: Create Application
echo ""
echo "=========================================="
echo "STEP 5: Create ArgoCD Application"
echo "=========================================="
echo ""

APP_FILE="$PROJECT_ROOT/gitops/clusters/development/apps/user-org-service.yaml"
if [ -f "$APP_FILE" ]; then
  echo "✅ Application manifest found: $APP_FILE"
  echo ""
  echo "Applying application..."
  kubectl --context="$KUBE_CONTEXT" apply -f "$APP_FILE"
  echo "✅ Application created!"
else
  echo "❌ Application manifest not found: $APP_FILE"
  echo "Please create it first (see ARGOCD-SETUP-GUIDE.md)"
  exit 1
fi

# Step 6: Create Secrets
echo ""
echo "=========================================="
echo "STEP 6: Create Required Secrets"
echo "=========================================="
echo ""

echo "The service needs these secrets:"
echo "  1. user-org-service-db-secret (database-url)"
echo "  2. user-org-service-secrets (oauth-hmac-secret, oauth-client-secret)"
echo ""

# Create namespace
kubectl --context="$KUBE_CONTEXT" create namespace user-org-service || true

# Database secret
read -p "Enter database URL (postgres://user:pass@host:5432/dbname): " DATABASE_URL
if [ -n "$DATABASE_URL" ]; then
  kubectl --context="$KUBE_CONTEXT" create secret generic user-org-service-db-secret \
    --namespace=user-org-service \
    --from-literal=database-url="$DATABASE_URL" \
    --dry-run=client -o yaml | kubectl --context="$KUBE_CONTEXT" apply -f -
  echo "✅ Database secret created"
fi

# OAuth secrets
echo ""
read -p "Enter OAuth HMAC secret (min 32 bytes, or press Enter to generate): " OAUTH_HMAC_SECRET
if [ -z "$OAUTH_HMAC_SECRET" ]; then
  OAUTH_HMAC_SECRET=$(openssl rand -hex 32)
  echo "Generated HMAC secret: $OAUTH_HMAC_SECRET"
fi

read -p "Enter OAuth client secret (or press Enter to generate): " OAUTH_CLIENT_SECRET
if [ -z "$OAUTH_CLIENT_SECRET" ]; then
  OAUTH_CLIENT_SECRET=$(openssl rand -hex 16)
  echo "Generated client secret: $OAUTH_CLIENT_SECRET"
fi

kubectl --context="$KUBE_CONTEXT" create secret generic user-org-service-secrets \
  --namespace=user-org-service \
  --from-literal=oauth-hmac-secret="$OAUTH_HMAC_SECRET" \
  --from-literal=oauth-client-secret="$OAUTH_CLIENT_SECRET" \
  --dry-run=client -o yaml | kubectl --context="$KUBE_CONTEXT" apply -f -
echo "✅ OAuth secrets created"

# Step 7: Sync Application
echo ""
echo "=========================================="
echo "STEP 7: Sync Application"
echo "=========================================="
echo ""

echo "Syncing application..."
argocd app sync user-org-service-development

echo ""
echo "Watching sync status (Ctrl+C to stop)..."
argocd app get user-org-service-development -w || true

# Step 8: Verify Deployment
echo ""
echo "=========================================="
echo "STEP 8: Verify Deployment"
echo "=========================================="
echo ""

echo "Checking pods..."
kubectl --context="$KUBE_CONTEXT" -n user-org-service get pods

echo ""
echo "Checking service..."
kubectl --context="$KUBE_CONTEXT" -n user-org-service get svc

echo ""
echo "Testing health endpoint..."
sleep 5  # Give pods time to start
kubectl --context="$KUBE_CONTEXT" -n user-org-service run test-curl \
  --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://user-org-service.user-org-service.svc.cluster.local:8081/healthz || echo "⚠️  Health check failed (pods may still be starting)"

# Step 9: GitHub Secrets
echo ""
echo "=========================================="
echo "STEP 9: Configure GitHub Secrets"
echo "=========================================="
echo ""

echo "To enable CI/CD e2e-test deployment, add these GitHub secrets:"
echo ""
echo "1. DEV_KUBECONFIG_B64:"
echo "   Run: kubectl --context=$KUBE_CONTEXT config view --flatten | base64 | tr -d '\n'"
echo ""
echo "2. DEV_KUBE_CONTEXT:"
echo "   Value: $KUBE_CONTEXT"
echo ""
echo "Add them at: https://github.com/otherjamesbrown/ai-aas/settings/secrets/actions"
echo ""
read -p "Press Enter when done (or to skip)..."

# Summary
echo ""
echo "=========================================="
echo "✅ Setup Complete!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Monitor ArgoCD: argocd app get user-org-service-development"
echo "  2. Check service logs: kubectl --context=$KUBE_CONTEXT -n user-org-service logs -l app=user-org-service -f"
echo "  3. Test e2e-test: cd services/user-org-service && make deploy-e2e-test"
echo "  4. View ArgoCD UI: kubectl --context=$KUBE_CONTEXT port-forward svc/argocd-server -n argocd 8080:443"
echo ""
echo "For detailed documentation, see:"
echo "  services/user-org-service/docs/ARGOCD-SETUP-GUIDE.md"
echo ""

