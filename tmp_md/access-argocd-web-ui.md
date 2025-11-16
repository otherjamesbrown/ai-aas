# Access Argo CD Web UI for Remote Dev Cluster

## Quick Access Steps

### 1. Set Up Kubeconfig
```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context lke531921-ctx

# Verify access
kubectl get nodes
```

### 2. Get Argo CD Admin Password
```bash
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo ""
```

**Save this password** - you'll need it to log in!

### 3. Port-Forward Argo CD Server
```bash
# Run this command and keep the terminal open
kubectl -n argocd port-forward svc/argocd-server 8080:80
```

**Keep this terminal running** - closing it will disconnect the port-forward.

### 4. Access Web UI

Open your browser and go to:
- **URL**: `http://localhost:8080`
- **Username**: `admin`
- **Password**: (the password from Step 2)

## What You'll See

Once logged in, you should see:

### Applications Tab
- `user-org-service-development`
- `network-development`
- `observability-development`
- `secrets-development`
- `web-portal-development` (if deployed)

### Application Status Indicators
- **Synced** ✅ - Application is in sync with Git
- **OutOfSync** ⚠️ - Changes detected, needs sync
- **Unknown** ❓ - Status not yet determined
- **Degraded** ❌ - Application has issues

### Health Status
- **Healthy** ✅ - All resources healthy
- **Degraded** ⚠️ - Some resources unhealthy
- **Missing** ❌ - Resources missing
- **Progressing** 🔄 - Sync in progress

## Quick Actions in Web UI

### View Application Details
1. Click on any application name
2. See:
   - Resource tree (all Kubernetes resources)
   - Sync status and history
   - Health status
   - Events and logs

### Sync Application
1. Click on application
2. Click **"Sync"** button (top right)
3. Select resources to sync (or sync all)
4. Click **"Synchronize"**

### View Resource Details
1. Click on application
2. Click on any resource in the resource tree
3. See:
   - YAML manifest
   - Events
   - Logs (for pods)

## Troubleshooting

### Port-Forward Not Working
```bash
# Check if port 8080 is already in use
lsof -i :8080

# Use a different port
kubectl -n argocd port-forward svc/argocd-server 8081:80
# Then access: http://localhost:8081
```

### Can't Connect to Cluster
```bash
# Verify kubeconfig
kubectl config get-contexts

# Test cluster access
kubectl get nodes

# Check if Argo CD namespace exists
kubectl get namespace argocd
```

### Argo CD Not Installed
If Argo CD is not installed, you'll need to install it first:
```bash
# Install Argo CD
kubectl create namespace argocd
helm repo add argo https://argoproj.github.io/argo-helm
helm repo update
helm install argocd argo/argo-cd --namespace argocd \
  --values gitops/templates/argocd-values.yaml

# Wait for installation
kubectl -n argocd wait --for=condition=available --timeout=5m deployment/argocd-server
```

### Forgot Password
```bash
# Get password again
kubectl -n argocd get secret argocd-initial-admin-secret \
  -o jsonpath="{.data.password}" | base64 -d && echo ""
```

## Alternative: Use Argo CD CLI

If you prefer CLI over web UI:

```bash
# After port-forwarding (Step 3), login via CLI
argocd login localhost:8080 --username admin --insecure --grpc-web

# List applications
argocd app list

# Get application details
argocd app get user-org-service-development

# Sync application
argocd app sync user-org-service-development
```

## Expected Applications

Based on your GitOps configuration, you should see these applications in the dev cluster:

| Application Name | Description | Namespace |
|-----------------|-------------|-----------|
| `user-org-service-development` | User and organization management service | `user-org-service` |
| `network-development` | Network policies | `network` |
| `observability-development` | Monitoring stack (Loki, Grafana, Prometheus) | `observability` |
| `secrets-development` | Secrets management | `secrets` |
| `web-portal-development` | Web portal frontend | `development` |

## Tips

- **Keep port-forward running**: The web UI requires the port-forward to stay active
- **Bookmark the URL**: `http://localhost:8080` for quick access
- **Check sync status regularly**: Applications auto-sync, but you can manually sync if needed
- **Use filters**: Filter applications by status (Synced, OutOfSync, etc.) in the web UI
- **View logs**: Click on pods in the resource tree to view logs directly in the UI

