# 🚀 Step-by-Step: Get User-Org-Service Running in Dev with ArgoCD

**Welcome!** This is your complete step-by-step guide. Follow each step in order.

## Quick Start Options

**Option A: Interactive Script (Recommended for First Time)**
```bash
cd services/user-org-service
./scripts/setup-argocd-step-by-step.sh
```
This script will guide you through everything interactively.

**Option B: Manual Steps**
Follow the detailed guide in `docs/ARGOCD-SETUP-GUIDE.md`

---

## Step-by-Step Checklist

### ✅ Step 0: Prerequisites Check (5 minutes)

```bash
# 1. Check kubectl access
kubectl config get-contexts
# Note which context is for your dev cluster

# 2. Test cluster access
kubectl --context=YOUR_DEV_CONTEXT get nodes
# Replace YOUR_DEV_CONTEXT with your actual context name

# 3. Check if you have ArgoCD CLI
argocd version --client
# If not installed: brew install argocd (macOS) or see guide
```

**✅ Checkpoint**: Can you see your cluster nodes? If yes, continue. If no, configure kubectl first.

---

### ✅ Step 1: Verify/Install ArgoCD (10 minutes)

```bash
# Set your context (replace with your actual context)
export KUBE_CONTEXT="dev-platform"  # or lke531921-ctx, or whatever yours is

# Check if ArgoCD exists
kubectl --context=$KUBE_CONTEXT get namespace argocd
```

**If ArgoCD exists:**
```bash
# Get admin password
kubectl --context=$KUBE_CONTEXT -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
echo ""
# Save this password!
```

**If ArgoCD doesn't exist:**
```bash
# Install ArgoCD
kubectl --context=$KUBE_CONTEXT create namespace argocd
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argocd argo/argo-cd --namespace argocd --kube-context=$KUBE_CONTEXT

# Wait for it to be ready (2-3 minutes)
kubectl --context=$KUBE_CONTEXT -n argocd wait --for=condition=available --timeout=5m deployment/argocd-server

# Get admin password
kubectl --context=$KUBE_CONTEXT -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d
echo ""
```

**✅ Checkpoint**: Do you have ArgoCD installed and the admin password? Continue to Step 2.

---

### ✅ Step 2: Login to ArgoCD (2 minutes)

```bash
# Option 1: If ArgoCD has a load balancer
export ARGOCD_SERVER=$(kubectl --context=$KUBE_CONTEXT -n argocd get svc argocd-server \
  -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')

# Option 2: Use port-forward (in a separate terminal, keep it running)
kubectl --context=$KUBE_CONTEXT port-forward svc/argocd-server -n argocd 8080:443
# Then use: ARGOCD_SERVER="localhost:8080"
```

```bash
# Login (use the password from Step 1)
argocd login $ARGOCD_SERVER --username admin --insecure --grpc-web
# Enter password when prompted
```

**✅ Checkpoint**: Can you run `argocd app list` successfully? If yes, continue.

---

### ✅ Step 3: Register Git Repository (3 minutes)

```bash
# For public repo (simpler)
argocd repo add https://github.com/otherjamesbrown/ai-aas --name ai-aas

# For private repo (you'll need a GitHub token)
# Create token at: https://github.com/settings/tokens (needs 'repo' scope)
read -p "Enter GitHub username: " GITHUB_USERNAME
read -p "Enter GitHub token: " -s GITHUB_TOKEN
echo ""

argocd repo add https://github.com/otherjamesbrown/ai-aas \
  --name ai-aas \
  --username $GITHUB_USERNAME \
  --password $GITHUB_TOKEN
```

**✅ Checkpoint**: Run `argocd repo list` - do you see your repo? Continue.

---

### ✅ Step 4: Create Required Secrets (5 minutes)

The service needs database and OAuth secrets. Let's create them:

```bash
# Create namespace
kubectl --context=$KUBE_CONTEXT create namespace user-org-service || true

# 1. Database secret
read -p "Enter your database URL (postgres://user:pass@host:5432/dbname): " DATABASE_URL
kubectl --context=$KUBE_CONTEXT create secret generic user-org-service-db-secret \
  --namespace=user-org-service \
  --from-literal=database-url="$DATABASE_URL" \
  --dry-run=client -o yaml | kubectl --context=$KUBE_CONTEXT apply -f -

# 2. OAuth secrets (generate if you don't have them)
OAUTH_HMAC_SECRET=$(openssl rand -hex 32)
OAUTH_CLIENT_SECRET=$(openssl rand -hex 16)

echo "Generated OAuth HMAC Secret: $OAUTH_HMAC_SECRET"
echo "Generated OAuth Client Secret: $OAUTH_CLIENT_SECRET"
echo "(Save these!)"

kubectl --context=$KUBE_CONTEXT create secret generic user-org-service-secrets \
  --namespace=user-org-service \
  --from-literal=oauth-hmac-secret="$OAUTH_HMAC_SECRET" \
  --from-literal=oauth-client-secret="$OAUTH_CLIENT_SECRET" \
  --dry-run=client -o yaml | kubectl --context=$KUBE_CONTEXT apply -f -
```

**✅ Checkpoint**: Run `kubectl --context=$KUBE_CONTEXT -n user-org-service get secrets` - do you see both secrets? Continue.

---

### ✅ Step 5: Create ArgoCD Application (2 minutes)

The application manifest is already created at `gitops/clusters/development/apps/user-org-service.yaml`.

```bash
cd /Users/jabrown/Documents/GitHub/otherjamesbrown/ai-aas

# Apply the application
kubectl --context=$KUBE_CONTEXT apply -f gitops/clusters/development/apps/user-org-service.yaml

# Verify it was created
kubectl --context=$KUBE_CONTEXT -n argocd get application user-org-service-development
```

**✅ Checkpoint**: Do you see the application? Continue.

---

### ✅ Step 6: Sync the Application (5 minutes)

```bash
# Sync the application (this will deploy the service)
argocd app sync user-org-service-development

# Watch the sync status
argocd app get user-org-service-development

# Or watch in real-time
watch -n 2 'argocd app get user-org-service-development'
```

**What to expect:**
- ArgoCD will create the namespace (if needed)
- It will deploy the Helm chart
- Pods will start up
- This may take 2-3 minutes

**✅ Checkpoint**: Does the app show "Synced" and "Healthy"? Continue.

---

### ✅ Step 7: Verify Service is Running (3 minutes)

```bash
# Check pods
kubectl --context=$KUBE_CONTEXT -n user-org-service get pods

# Wait for pods to be ready (they should show "Running")
kubectl --context=$KUBE_CONTEXT -n user-org-service wait --for=condition=ready \
  --timeout=5m pod -l app=user-org-service

# Check service
kubectl --context=$KUBE_CONTEXT -n user-org-service get svc

# Test health endpoint
kubectl --context=$KUBE_CONTEXT -n user-org-service run test-curl \
  --image=curlimages/curl --rm -i --restart=Never -- \
  curl -s http://user-org-service.user-org-service.svc.cluster.local:8081/healthz
```

**Expected**: You should see `{"status":"ok"}` or similar.

**✅ Checkpoint**: Is the service responding? Continue.

---

### ✅ Step 8: Configure GitHub Secrets for CI/CD (5 minutes)

This enables the e2e-test to run automatically in GitHub Actions.

```bash
# 1. Get your kubeconfig as base64
kubectl --context=$KUBE_CONTEXT config view --flatten | base64 | tr -d '\n'
# Copy this entire output
```

```bash
# 2. Get your context name
kubectl config current-context
# Copy this
```

**Now add GitHub secrets:**

1. Go to: `https://github.com/otherjamesbrown/ai-aas/settings/secrets/actions`
2. Click **"New repository secret"**
3. Add two secrets:

   **Secret 1:**
   - Name: `DEV_KUBECONFIG_B64`
   - Value: (paste the base64 kubeconfig from above)

   **Secret 2:**
   - Name: `DEV_KUBE_CONTEXT`
   - Value: (paste your context name, e.g., `dev-platform` or `lke531921-ctx`)

**✅ Checkpoint**: Are both secrets added? Continue.

---

### ✅ Step 9: Test E2E Test Deployment (5 minutes)

Let's test that the e2e-test can be deployed and run:

```bash
cd services/user-org-service

# Build and deploy the test
make deploy-e2e-test
```

**What to expect:**
- Script builds Docker image
- Pushes to registry
- Creates Kubernetes Job
- Streams test logs
- Reports pass/fail

**✅ Checkpoint**: Do the tests pass? If yes, you're done! If no, check the troubleshooting section.

---

### ✅ Step 10: Test CI/CD Integration (5 minutes)

Now let's verify the GitHub Actions workflow works:

1. **Option A: Push a commit to main**
   ```bash
   git add .
   git commit -m "test: trigger e2e-test workflow"
   git push origin main
   ```

2. **Option B: Manual trigger**
   - Go to: `https://github.com/otherjamesbrown/ai-aas/actions`
   - Click **"user-org-service"** workflow
   - Click **"Run workflow"** → **"Run workflow"**

**Watch the workflow:**
- The `build` job should complete
- The `deploy-e2e-test` job should:
  - Build the image ✅
  - Push to registry ✅
  - Create Kubernetes Job ✅
  - Run tests ✅
  - Report results ✅

**✅ Checkpoint**: Does the workflow complete successfully? If yes, **you're all set!**

---

## Troubleshooting

### ArgoCD Application Not Syncing

```bash
# Check application status
argocd app get user-org-service-development

# Check for errors
kubectl --context=$KUBE_CONTEXT -n argocd get events --field-selector involvedObject.name=user-org-service-development

# Try manual sync
argocd app sync user-org-service-development --force
```

### Service Pods Not Starting

```bash
# Check pod status
kubectl --context=$KUBE_CONTEXT -n user-org-service describe pod <pod-name>

# Check logs
kubectl --context=$KUBE_CONTEXT -n user-org-service logs <pod-name>

# Common issues:
# - Missing secrets (check Step 4)
# - Database connection issues
# - Image pull errors
```

### E2E Test Fails

```bash
# Check if service is reachable
kubectl --context=$KUBE_CONTEXT -n user-org-service run test-curl \
  --image=curlimages/curl --rm -i --restart=Never -- \
  curl http://user-org-service.user-org-service.svc.cluster.local:8081/healthz

# Check test job logs
kubectl --context=$KUBE_CONTEXT -n user-org-service logs -l component=e2e-test --tail=100
```

---

## Quick Reference

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

## Need Help?

- **Detailed guide**: `docs/ARGOCD-SETUP-GUIDE.md`
- **Interactive script**: `./scripts/setup-argocd-step-by-step.sh`
- **E2E test docs**: `docs/e2e-testing.md`

**Ready to start? Begin with Step 0!** 🚀

