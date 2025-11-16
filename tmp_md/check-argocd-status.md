# Check Argo CD Deployment Status

## Prerequisites Setup

### 1. Install kubectl (if not installed)
```bash
# Linux
sudo snap install kubectl
# OR download from: https://kubernetes.io/docs/tasks/tools/install-kubectl-linux/

# macOS
brew install kubectl
```

### 2. Install ArgoCD CLI
```bash
# macOS
brew install argocd

# Linux - Download binary
curl -sSL -o /usr/local/bin/argocd https://github.com/argoproj/argo-cd/releases/latest/download/argocd-linux-amd64
chmod +x /usr/local/bin/argocd
```

### 3. Set up kubeconfig
Based on your memory, kubeconfigs should be at:
- Development: `~/kubeconfigs/kubeconfig-development.yaml`
- Production: `~/kubeconfigs/kubeconfig-production.yaml`

```bash
# Set kubeconfig for development
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context lke531921-ctx

# OR for production
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml
kubectl config use-context lke531922-ctx
```

## Check Argo CD Status

### Option 1: Using ArgoCD CLI (Recommended)

#### Step 1: Port-forward ArgoCD Server
```bash
# In a separate terminal, keep this running
kubectl -n argocd port-forward svc/argocd-server 8080:80
```

#### Step 2: Login to ArgoCD
```bash
# Get admin password (first time only)
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo ""

# Login
argocd login localhost:8080 --username admin --insecure --grpc-web
# Enter the password when prompted
```

#### Step 3: List All Applications
```bash
# List all ArgoCD applications
argocd app list

# Expected applications (development):
# - user-org-service-development
# - network-development
# - observability-development
# - secrets-development
# - web-portal-development (if deployed)
```

#### Step 4: Check Specific Application Status
```bash
# Check user-org-service status
argocd app get user-org-service-development

# Check infrastructure applications
argocd app get network-development
argocd app get observability-development
argocd app get secrets-development

# Watch sync status in real-time
argocd app get user-org-service-development --watch
```

#### Step 5: Check Sync Status Summary
```bash
# Get sync status for all apps
argocd app list --output wide

# Filter by status
argocd app list | grep -E "Synced|OutOfSync|Unknown|Degraded"
```

### Option 2: Using kubectl (Direct)

#### Check ArgoCD Applications via kubectl
```bash
# List all ArgoCD applications
kubectl -n argocd get applications

# Get detailed status of specific application
kubectl -n argocd get application user-org-service-development -o yaml

# Check application status
kubectl -n argocd describe application user-org-service-development

# Check ArgoCD server pods
kubectl -n argocd get pods

# Check ArgoCD server logs
kubectl -n argocd logs -l app.kubernetes.io/name=argocd-server --tail=50
```

### Option 3: Access ArgoCD Web UI

#### Port-forward and open browser
```bash
# Port-forward ArgoCD UI
kubectl -n argocd port-forward svc/argocd-server 8080:80

# Open in browser
# URL: http://localhost:8080
# Username: admin
# Password: (from kubectl command above)
```

## Quick Status Check Commands

```bash
# Quick status check script
#!/bin/bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context lke531921-ctx

echo "=== ArgoCD Applications ==="
kubectl -n argocd get applications

echo ""
echo "=== Application Status Details ==="
for app in $(kubectl -n argocd get applications -o name); do
  echo "--- $app ---"
  kubectl -n argocd get $app -o jsonpath='{.status.sync.status}/{.status.health.status}' && echo ""
done

echo ""
echo "=== ArgoCD Server Status ==="
kubectl -n argocd get pods -l app.kubernetes.io/name=argocd-server
```

## Expected Application Names

Based on the GitOps configuration:

### Development Environment:
- `user-org-service-development`
- `network-development`
- `observability-development`
- `secrets-development`
- `web-portal-development` (if deployed)

### Production Environment:
- `user-org-service-production`
- `network-production`
- `observability-production`
- `secrets-production`

## Troubleshooting

### If ArgoCD is not installed:
```bash
# Install ArgoCD
kubectl create namespace argocd
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argocd argo/argo-cd --namespace argocd

# Wait for installation
kubectl -n argocd wait --for=condition=available --timeout=5m deployment/argocd-server
```

### If applications are OutOfSync:
```bash
# Sync manually
argocd app sync user-org-service-development

# Force sync (if needed)
argocd app sync user-org-service-development --force
```

### Check application events:
```bash
kubectl -n argocd get events --field-selector involvedObject.name=user-org-service-development
```

