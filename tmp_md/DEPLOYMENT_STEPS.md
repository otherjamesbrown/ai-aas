# Deployment Steps - Ingress & TLS Configuration

## Summary of Changes

This commit includes:
- ✅ Self-signed certificates for local development
- ✅ Ingress configuration for all services (API Router, Web Portal, ArgoCD)
- ✅ Updated Helm values with TLS and new URLs
- ✅ GitOps application configurations updated
- ✅ Documentation for TLS/SSL setup

## Step 1: Commit Changes

```bash
# Review what will be committed
git status

# Commit all changes
git add docs/ infra/ services/ web/ gitops/ scripts/infra/
git commit -m "feat: Configure ingress with TLS for dev/prod environments

- Add self-signed certificates for local development
- Configure ingress for API Router, Web Portal, ArgoCD
- Update Helm values with TLS and new .ai-aas.local URLs
- Update GitOps applications for development
- Add TLS/SSL setup documentation
- Add scripts for certificate generation and hosts file management

Related: T-S013-P01-001"
```

## Step 2: Push to Git

```bash
# Push current branch
git push origin chore/cleanup-uncommitted-changes

# Or if merging to main:
git checkout main
git merge chore/cleanup-uncommitted-changes
git push origin main
```

## Step 3: ArgoCD Sync (Development)

ArgoCD should automatically sync for development (auto-sync enabled), but you can manually trigger:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Check ArgoCD apps
argocd app list

# Sync web portal
argocd app sync web-portal-development

# Sync API router (if app exists)
argocd app sync api-router-service-development

# Sync ArgoCD itself (requires updating ArgoCD deployment)
# This might need to be done via Helm upgrade
```

## Step 4: Verify Deployment

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Check ingress resources
kubectl get ingress --all-namespaces

# Check services are running
kubectl get pods -n development
kubectl get pods -n default

# Test endpoints (after hosts file is updated)
curl -k https://api.dev.ai-aas.local/healthz
curl -k https://portal.dev.ai-aas.local
```

## Step 5: Production Deployment

Production requires manual ArgoCD sync:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml

# Sync applications manually
argocd app sync web-portal-production  # If exists
argocd app sync api-router-service-production  # If exists

# Or update ArgoCD values
helm upgrade argocd argo/argo-cd \
  --namespace argocd \
  -f ./gitops/templates/argocd-values.yaml
```

## Important Notes

1. **TLS Secret**: Make sure `ai-aas-tls` secret exists in `ingress-nginx` namespace
   ```bash
   kubectl get secret ai-aas-tls -n ingress-nginx
   ```

2. **Hosts File**: Ensure hosts file is updated on machines accessing services
   ```bash
   sudo ./scripts/infra/update-hosts-file.sh --ingress-ip <INGRESS_IP>
   ```

3. **CA Certificate**: Trust the CA certificate on all machines
   ```bash
   sudo cp infra/secrets/certs/ai-aas-ca.crt /usr/local/share/ca-certificates/
   sudo update-ca-certificates
   ```

4. **Ingress IP**: Get the ingress IP for hosts file:
   ```bash
   kubectl get svc -n ingress-nginx ingress-nginx-controller \
     -o jsonpath='{.status.loadBalancer.ingress[0].ip}'
   ```

## Troubleshooting

### ArgoCD Not Syncing
```bash
# Check app status
argocd app get web-portal-development

# Force sync
argocd app sync web-portal-development --force

# Check for errors
kubectl get events -n argocd --field-selector involvedObject.name=web-portal-development
```

### Ingress Not Working
```bash
# Check ingress controller
kubectl get pods -n ingress-nginx

# Check ingress resources
kubectl describe ingress -n development
kubectl describe ingress -n default

# Check TLS secret
kubectl describe secret ai-aas-tls -n ingress-nginx
```

### Certificate Issues
```bash
# Verify certificate
openssl x509 -in infra/secrets/certs/tls.crt -text -noout | grep -A 2 "Subject Alternative Name"

# Check certificate in secret
kubectl get secret ai-aas-tls -n ingress-nginx -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

