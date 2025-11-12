# Step-by-Step Guide: ArgoCD Setup for User-Org-Service

This guide walks you through setting up automated deployment of the user-org-service to the development environment using ArgoCD, and configuring the e2e-test to run automatically.

## Prerequisites Checklist

Before we start, let's verify you have everything:

- [ ] Access to the development Kubernetes cluster
- [ ] `kubectl` configured with the dev cluster context
- [ ] ArgoCD installed in the dev cluster (or we'll install it)
- [ ] GitHub repository access (for GitOps)
- [ ] Container registry access (ghcr.io or your registry)

Let's check these one by one:

### Step 0: Verify Prerequisites

```bash
# 1. Check kubectl access to dev cluster
kubectl --context=dev-platform get nodes
# OR if your context has a different name:
kubectl config get-contexts
# Note the context name for dev cluster

# 2. Check if ArgoCD is installed
kubectl --context=dev-platform get namespace argocd
# If it exists, continue. If not, we'll install it in Step 1.

# 3. Check if you can access the cluster
kubectl --context=dev-platform cluster-info
```

**Expected output**: You should see node information and cluster info. If you get errors, you'll need to configure kubectl first.

---

## Step 1: Install/Verify ArgoCD

If ArgoCD isn't installed, we'll install it. If it is, we'll verify it's working.

### 1.1 Check ArgoCD Status

```bash
# Set your context (replace with your actual context name)
export KUBE_CONTEXT="dev-platform"  # or lke531921-ctx, or whatever your context is

# Check if ArgoCD namespace exists
kubectl --context=$KUBE_CONTEXT get namespace argocd
```

**If namespace exists**: Skip to Step 1.3  
**If namespace doesn't exist**: Continue to Step 1.2

### 1.2 Install ArgoCD (if needed)

```bash
# Create namespace
kubectl --context=$KUBE_CONTEXT create namespace argocd

# Install ArgoCD using Helm
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update

# Install ArgoCD
helm install argocd argo/argo-cd \
  --namespace argocd \
  --kube-context=$KUBE_CONTEXT \
  --create-namespace \
  --values gitops/templates/argocd-values.yaml  # If this file exists, otherwise omit
```

### 1.3 Get ArgoCD Admin Password

```bash
# Wait for ArgoCD to be ready (this may take 2-3 minutes)
kubectl --context=$KUBE_CONTEXT -n argocd wait --for=condition=available --timeout=5m deployment/argocd-server

# Get the admin password
kubectl --context=$KUBE_CONTEXT -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
echo ""  # New line after password
```

**Save this password** - you'll need it to log into ArgoCD UI.

### 1.4 Port-Forward to ArgoCD UI (for verification)

```bash
# In a separate terminal, keep this running:
kubectl --context=$KUBE_CONTEXT port-forward svc/argocd-server -n argocd 8080:443
```

Then open `https://localhost:8080` in your browser:
- Username: `admin`
- Password: (the password from Step 1.3)

**You can close the port-forward after verifying it works.**

---

## Step 2: Register This Repository with ArgoCD

ArgoCD needs to know about your Git repository to sync from it.

### 2.1 Install ArgoCD CLI (if not already installed)

```bash
# macOS
brew install argocd

# Linux
curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
chmod +x /usr/local/bin/argocd

# Verify installation
argocd version --client
```

### 2.2 Login to ArgoCD

```bash
# Get ArgoCD server address
export ARGOCD_SERVER=$(kubectl --context=$KUBE_CONTEXT -n argocd get svc argocd-server -o jsonpath='{.status.loadBalancer.ingress[0].hostname}')
# If no load balancer, use port-forward (in another terminal):
# kubectl --context=$KUBE_CONTEXT port-forward svc/argocd-server -n argocd 8080:443

# Login (use the password from Step 1.3)
argocd login localhost:8080 --insecure  # If using port-forward
# OR
argocd login $ARGOCD_SERVER --insecure  # If using load balancer
```

### 2.3 Register the Repository

```bash
# Get your GitHub token (you'll need a personal access token with repo access)
# Create one at: https://github.com/settings/tokens
# Permissions needed: repo (read access)

export GITHUB_TOKEN="your_github_token_here"
export REPO_URL="https://github.com/otherjamesbrown/ai-aas"

# Add the repository
argocd repo add $REPO_URL \
  --type git \
  --username $GITHUB_USERNAME \
  --password $GITHUB_TOKEN \
  --name ai-aas \
  --insecure-skip-server-verification
```

**Alternative**: If your repo is public, you can add it without credentials:
```bash
argocd repo add $REPO_URL --name ai-aas
```

---

## Step 3: Create ArgoCD Application for User-Org-Service

Now we'll create an ArgoCD Application that will automatically deploy the service.

### 3.1 Create the Application Manifest

We'll create an ArgoCD Application that points to the Helm chart:

```bash
cd /Users/jabrown/Documents/GitHub/otherjamesbrown/ai-aas
```

Create the application file:

```bash
cat > gitops/clusters/development/apps/user-org-service.yaml <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: user-org-service-development
  namespace: argocd
  labels:
    environment: development
    app: user-org-service
spec:
  project: platform-development
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    targetRevision: main  # or your branch name
    path: services/user-org-service/configs/helm
    helm:
      valueFiles:
        - values.yaml
      # Override values for development
      values: |
        image:
          repository: ghcr.io/otherjamesbrown/user-org-service
          tag: latest
        env:
          - name: ENVIRONMENT
            value: development
          - name: LOG_LEVEL
            value: info
        postgres:
          dsnSecretName: user-org-service-db-secret
        redis:
          address: redis.development.svc.cluster.local:6379
  destination:
    server: https://kubernetes.default.svc
    namespace: user-org-service
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
      allowEmpty: false
    syncOptions:
      - CreateNamespace=true
EOF
```

### 3.2 Apply the Application

```bash
# Apply the application manifest
kubectl --context=$KUBE_CONTEXT apply -f gitops/clusters/development/apps/user-org-service.yaml

# Check if it was created
kubectl --context=$KUBE_CONTEXT -n argocd get application user-org-service-development
```

### 3.3 Sync the Application

```bash
# Trigger a sync (ArgoCD will deploy the service)
argocd app sync user-org-service-development

# Watch the sync status
argocd app get user-org-service-development

# Or watch in real-time
watch -n 2 'argocd app get user-org-service-development'
```

**Expected**: ArgoCD will create the namespace and deploy the service. This may take a few minutes.

---

## Step 4: Create Database Secret

The service needs database credentials. We'll create a secret.

### 4.1 Create Database Secret

```bash
# Set your database connection string
export DATABASE_URL="postgres://user:password@host:5432/dbname?sslmode=disable"

# Create the secret in the user-org-service namespace
kubectl --context=$KUBE_CONTEXT create namespace user-org-service || true

kubectl --context=$KUBE_CONTEXT create secret generic user-org-service-db-secret \
  --namespace=user-org-service \
  --from-literal=database-url="$DATABASE_URL" \
  --dry-run=client -o yaml | kubectl --context=$KUBE_CONTEXT apply -f -
```

**Note**: Replace the DATABASE_URL with your actual database connection string.

### 4.2 Update the Deployment to Use the Secret

The Helm chart needs to reference this secret. Let's check if the values.yaml already has this configured, or we may need to update the application.

---

## Step 5: Verify Service Deployment

Let's check that everything is running:

### 5.1 Check Pods

```bash
# Check if pods are running
kubectl --context=$KUBE_CONTEXT -n user-org-service get pods

# Check pod logs if there are issues
kubectl --context=$KUBE_CONTEXT -n user-org-service logs -l app=user-org-service --tail=50
```

### 5.2 Check Service

```bash
# Check if service is created
kubectl --context=$KUBE_CONTEXT -n user-org-service get svc

# Test connectivity from within cluster
kubectl --context=$KUBE_CONTEXT -n user-org-service run test-curl \
  --image=curlimages/curl --rm -it -- \
  curl http://user-org-service.user-org-service.svc.cluster.local:8081/healthz
```

**Expected**: You should see a 200 OK response.

---

## Step 6: Configure GitHub Secrets for CI/CD

Now we'll set up the GitHub secrets so the CI/CD pipeline can deploy the e2e-test.

### 6.1 Get Your Kubeconfig

```bash
# Export your kubeconfig as base64
kubectl --context=$KUBE_CONTEXT config view --flatten | base64 | tr -d '\n'
```

**Copy this output** - you'll need it for the GitHub secret.

### 6.2 Get Your Kubernetes Context Name

```bash
# Get the current context name
kubectl config current-context
```

**Copy this** - you'll need it too.

### 6.3 Add GitHub Secrets

1. Go to your GitHub repository: `https://github.com/otherjamesbrown/ai-aas`
2. Click **Settings** → **Secrets and variables** → **Actions**
3. Click **New repository secret**
4. Add these secrets:

   **Secret 1:**
   - Name: `DEV_KUBECONFIG_B64`
   - Value: (paste the base64 kubeconfig from Step 6.1)

   **Secret 2:**
   - Name: `DEV_KUBE_CONTEXT`
   - Value: (paste the context name from Step 6.2, e.g., `lke531921-ctx` or `dev-platform`)

---

## Step 7: Test the E2E Test Deployment

Now let's test that the e2e-test can be deployed and run.

### 7.1 Test Locally First

```bash
cd services/user-org-service

# Build the test image
make docker-build-e2e-test

# Tag and push (replace with your registry)
docker tag user-org-service-e2e-test:latest \
  ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest

docker push ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest
```

### 7.2 Deploy Test Job Manually

```bash
# Use the deployment script
make deploy-e2e-test

# OR manually create the job
./scripts/deploy-e2e-test.sh
```

**Expected**: The test job should run and you should see test results in the logs.

---

## Step 8: Verify CI/CD Integration

Now let's test that the GitHub Actions workflow works.

### 8.1 Trigger the Workflow

You can either:
- Push a commit to `main` branch (will trigger automatically)
- Or manually trigger: Go to **Actions** tab → **user-org-service** workflow → **Run workflow**

### 8.2 Monitor the Workflow

1. Go to: `https://github.com/otherjamesbrown/ai-aas/actions`
2. Click on the running workflow
3. Watch the `deploy-e2e-test` job
4. Check the logs to see if it:
   - Builds the image ✅
   - Pushes to registry ✅
   - Creates the Kubernetes Job ✅
   - Runs the tests ✅

---

## Step 9: Set Up Automated Service Deployment (Optional)

If you want ArgoCD to automatically deploy new versions when you push code:

### 9.1 Update Application to Watch Main Branch

The application we created in Step 3 already watches `main` branch. But we need to ensure:
1. Your Helm chart builds are automated (Docker images pushed on commit)
2. ArgoCD syncs automatically (already configured with `syncPolicy.automated`)

### 9.2 Add Image Update Automation (Optional)

You can use ArgoCD Image Updater or update the Helm values manually. For now, the manual process is:
1. Build and push new Docker image with a tag
2. Update `gitops/clusters/development/apps/user-org-service.yaml` with new image tag
3. Commit and push - ArgoCD will automatically sync

---

## Troubleshooting

### ArgoCD Application Shows "Unknown" or "Degraded"

```bash
# Check application details
argocd app get user-org-service-development

# Check application events
kubectl --context=$KUBE_CONTEXT -n argocd get events --field-selector involvedObject.name=user-org-service-development

# Try manual sync
argocd app sync user-org-service-development --force
```

### Service Pods Not Starting

```bash
# Check pod status
kubectl --context=$KUBE_CONTEXT -n user-org-service describe pod <pod-name>

# Check events
kubectl --context=$KUBE_CONTEXT -n user-org-service get events

# Common issues:
# - Missing secrets (database, redis, etc.)
# - Image pull errors (check registry credentials)
# - Resource limits too low
```

### E2E Test Job Fails

```bash
# Check job status
kubectl --context=$KUBE_CONTEXT -n user-org-service get job -l component=e2e-test

# Check pod logs
kubectl --context=$KUBE_CONTEXT -n user-org-service logs -l job-name=e2e-test-<timestamp>

# Common issues:
# - Service not reachable (check service name/namespace)
# - Network policies blocking traffic
# - Service not ready (check service health)
```

---

## Next Steps

Once everything is working:

1. **Monitor ArgoCD**: Set up alerts for sync failures
2. **Add More Environments**: Repeat for staging/production
3. **Automate Image Builds**: Set up CI to build and push images on commit
4. **Add Monitoring**: Set up Prometheus/Grafana dashboards
5. **Document Runbooks**: Create runbooks for common operations

---

## Quick Reference Commands

```bash
# Check ArgoCD apps
argocd app list

# Sync an app
argocd app sync user-org-service-development

# Get app status
argocd app get user-org-service-development

# Check service pods
kubectl --context=$KUBE_CONTEXT -n user-org-service get pods

# View service logs
kubectl --context=$KUBE_CONTEXT -n user-org-service logs -l app=user-org-service -f

# Deploy e2e-test
cd services/user-org-service && make deploy-e2e-test

# Check e2e-test jobs
kubectl --context=$KUBE_CONTEXT -n user-org-service get jobs -l component=e2e-test
```

---

Ready to start? Let's begin with **Step 0** - verifying your prerequisites!

