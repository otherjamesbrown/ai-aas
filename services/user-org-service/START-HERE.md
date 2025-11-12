# 🚀 Start Here: Deploy to Dev with ArgoCD

**Welcome!** This is your step-by-step guide. Follow each step in order.

## Two Ways to Get Started

### Option 1: Interactive Script (Easiest) ⭐
```bash
cd services/user-org-service
./scripts/setup-argocd-step-by-step.sh
```
This script will guide you through everything and pause for your input.

### Option 2: Manual Steps
Follow the checklist below, or see the detailed guide: `docs/STEP-BY-STEP-START-HERE.md`

---

## Quick Checklist

### Step 0: Prerequisites (2 min)
```bash
# Check kubectl access
kubectl config get-contexts
# Note your dev cluster context name

# Test access
kubectl --context=YOUR_CONTEXT get nodes
```

**✅ Ready?** Continue to Step 1.

---

### Step 1: ArgoCD Setup (10 min)

**Check if ArgoCD is installed:**
```bash
export KUBE_CONTEXT="your-dev-context"  # Replace with your actual context
kubectl --context=$KUBE_CONTEXT get namespace argocd
```

**If it exists:**
```bash
# Get admin password
kubectl --context=$KUBE_CONTEXT -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo ""
```

**If it doesn't exist:**
```bash
# Install ArgoCD
kubectl --context=$KUBE_CONTEXT create namespace argocd
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argocd argo/argo-cd --namespace argocd --kube-context=$KUBE_CONTEXT

# Wait 2-3 minutes, then get password (same command as above)
```

**✅ Checkpoint**: Do you have ArgoCD installed and the admin password? Continue.

---

### Step 2: Login to ArgoCD (2 min)

```bash
# Install ArgoCD CLI if needed
brew install argocd  # macOS
# OR see: https://argo-cd.readthedocs.io/en/stable/cli_installation/

# Port-forward to ArgoCD (in a separate terminal, keep it running)
kubectl --context=$KUBE_CONTEXT port-forward svc/argocd-server -n argocd 8080:443

# Login (use password from Step 1)
argocd login localhost:8080 --username admin --insecure --grpc-web
```

**✅ Checkpoint**: Can you run `argocd app list`? Continue.

---

### Step 3: Register Git Repo (2 min)

```bash
# For public repo
argocd repo add https://github.com/otherjamesbrown/ai-aas --name ai-aas

# For private repo (you'll need a GitHub token)
argocd repo add https://github.com/otherjamesbrown/ai-aas \
  --name ai-aas \
  --username YOUR_GITHUB_USERNAME \
  --password YOUR_GITHUB_TOKEN
```

**✅ Checkpoint**: Run `argocd repo list` - do you see your repo? Continue.

---

### Step 4: Create Secrets (5 min)

```bash
# Create namespace
kubectl --context=$KUBE_CONTEXT create namespace user-org-service || true

# 1. Database secret
read -p "Enter database URL: " DATABASE_URL
kubectl --context=$KUBE_CONTEXT create secret generic user-org-service-db-secret \
  --namespace=user-org-service \
  --from-literal=database-url="$DATABASE_URL"

# 2. OAuth secrets (auto-generate)
OAUTH_HMAC_SECRET=$(openssl rand -hex 32)
OAUTH_CLIENT_SECRET=$(openssl rand -hex 16)
echo "OAuth HMAC: $OAUTH_HMAC_SECRET"
echo "OAuth Client Secret: $OAUTH_CLIENT_SECRET"

kubectl --context=$KUBE_CONTEXT create secret generic user-org-service-secrets \
  --namespace=user-org-service \
  --from-literal=oauth-hmac-secret="$OAUTH_HMAC_SECRET" \
  --from-literal=oauth-client-secret="$OAUTH_CLIENT_SECRET"
```

**✅ Checkpoint**: Run `kubectl --context=$KUBE_CONTEXT -n user-org-service get secrets` - see both? Continue.

---

### Step 5: Create ArgoCD Application (1 min)

```bash
cd /Users/jabrown/Documents/GitHub/otherjamesbrown/ai-aas

# Apply the application (already created for you)
kubectl --context=$KUBE_CONTEXT apply -f gitops/clusters/development/apps/user-org-service.yaml

# Verify
kubectl --context=$KUBE_CONTEXT -n argocd get application user-org-service-development
```

**✅ Checkpoint**: Do you see the application? Continue.

---

### Step 6: Sync Application (5 min)

```bash
# Sync (this deploys the service)
argocd app sync user-org-service-development

# Watch status
argocd app get user-org-service-development -w
# Press Ctrl+C when it shows "Synced" and "Healthy"
```

**✅ Checkpoint**: Is the app synced and healthy? Continue.

---

### Step 7: Verify Service (2 min)

```bash
# Check pods
kubectl --context=$KUBE_CONTEXT -n user-org-service get pods

# Wait for ready
kubectl --context=$KUBE_CONTEXT -n user-org-service wait --for=condition=ready \
  --timeout=5m pod -l app=user-org-service

# Test health
kubectl --context=$KUBE_CONTEXT -n user-org-service run test-curl \
  --image=curlimages/curl --rm -i --restart=Never -- \
  curl http://user-org-service.user-org-service.svc.cluster.local:8081/healthz
```

**✅ Checkpoint**: Do you see `{"status":"ok"}` or similar? Continue.

---

### Step 8: GitHub Secrets for CI/CD (5 min)

```bash
# 1. Get kubeconfig as base64
kubectl --context=$KUBE_CONTEXT config view --flatten | base64 | tr -d '\n'
# Copy this entire output

# 2. Get context name
kubectl config current-context
# Copy this
```

**Add to GitHub:**
1. Go to: `https://github.com/otherjamesbrown/ai-aas/settings/secrets/actions`
2. Click **"New repository secret"**
3. Add:
   - Name: `DEV_KUBECONFIG_B64`, Value: (paste base64 from above)
   - Name: `DEV_KUBE_CONTEXT`, Value: (paste context name)

**✅ Checkpoint**: Are both secrets added? Continue.

---

### Step 9: Test E2E Test (5 min)

```bash
cd services/user-org-service

# Deploy and run tests
make deploy-e2e-test
```

**Expected**: Tests should run and pass.

**✅ Checkpoint**: Do tests pass? If yes, you're done! 🎉

---

### Step 10: Test CI/CD (Optional)

1. Push a commit to `main`, OR
2. Go to Actions → user-org-service → Run workflow

Watch the `deploy-e2e-test` job run automatically!

---

## Need Help?

- **Interactive script**: `./scripts/setup-argocd-step-by-step.sh`
- **Detailed guide**: `docs/STEP-BY-STEP-START-HERE.md`
- **Full documentation**: `docs/ARGOCD-SETUP-GUIDE.md`

## Quick Commands Reference

```bash
# View ArgoCD apps
argocd app list

# Sync app
argocd app sync user-org-service-development

# Check service
kubectl --context=$KUBE_CONTEXT -n user-org-service get pods,svc

# View logs
kubectl --context=$KUBE_CONTEXT -n user-org-service logs -l app=user-org-service -f

# Deploy e2e-test
cd services/user-org-service && make deploy-e2e-test
```

---

**Ready? Start with Step 0!** 🚀

