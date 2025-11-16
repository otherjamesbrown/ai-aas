# Commit and Deploy Guide

## ✅ All Files Staged

Ready to commit:
- Certificate files (public certificates committed, private keys ignored)
- Helm values updated for ingress and TLS
- GitOps application configurations
- Documentation (TLS/SSL setup, infrastructure updates)
- Scripts (certificate generation, hosts file, TLS secrets)

## Step 1: Commit

```bash
git commit -m "feat: Configure ingress with TLS for dev/prod environments

- Add self-signed certificates for local development (infra/secrets/certs/)
- Configure ingress for API Router, Web Portal, ArgoCD with TLS
- Update Helm values with TLS and new .ai-aas.local URLs
- Create GitOps application for API Router Service
- Update GitOps web-portal application with HTTPS URLs
- Enable ArgoCD ingress with TLS
- Add TLS/SSL setup documentation (docs/platform/tls-ssl-setup.md)
- Add scripts for certificate generation and management
- Update infrastructure documentation

Endpoints configured:
- api.dev.ai-aas.local / api.prod.ai-aas.local
- portal.dev.ai-aas.local / portal.prod.ai-aas.local
- argocd.dev.ai-aas.local / argocd.prod.ai-aas.local
- grafana.dev.ai-aas.local / grafana.prod.ai-aas.local (ready)

Related: T-S013-P01-001"
```

## Step 2: Push to Git

```bash
# Push current branch
git push origin chore/cleanup-uncommitted-changes

# Or merge to main first
git checkout main
git merge chore/cleanup-uncommitted-changes
git push origin main
```

## Step 3: ArgoCD Sync (Development)

ArgoCD should auto-sync, but you can manually trigger:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Login to ArgoCD (if needed)
argocd login argocd.dev.ai-aas.local --grpc-web --insecure

# Sync applications
argocd app sync web-portal-development
argocd app sync api-router-service-development

# Check status
argocd app list
```

## Step 4: Update ArgoCD Ingress

ArgoCD ingress needs to be updated via Helm:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Update ArgoCD with new ingress config
helm upgrade argocd argo/argo-cd \
  --namespace argocd \
  --repo https://argoproj.github.io/argo-helm \
  -f ./gitops/templates/argocd-values.yaml
```

## Step 5: Verify Deployment

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml

# Check ingress resources
kubectl get ingress --all-namespaces

# Check services
kubectl get pods -n development
kubectl get pods -n default

# Test endpoints (after hosts file updated)
curl -k https://api.dev.ai-aas.local/healthz
curl -k https://portal.dev.ai-aas.local

# Check ArgoCD
curl -k https://argocd.dev.ai-aas.local
```

## Step 6: Production Deployment

For production, manually sync ArgoCD:

```bash
export KUBECONFIG=~/kubeconfigs/kubeconfig-production.yaml

# Create TLS secret in production
./scripts/infra/create-tls-secrets.sh --namespace ingress-nginx

# Sync applications (when they exist)
argocd app sync web-portal-production
argocd app sync api-router-service-production

# Update ArgoCD ingress
helm upgrade argocd argo/argo-cd \
  --namespace argocd \
  --repo https://argoproj.github.io/argo-helm \
  -f ./gitops/templates/argocd-values.yaml
```

## Quick Reference

**Ingress IP**: `172.232.58.222` (development)

**TLS Secret**: `ai-aas-tls` in namespace `ingress-nginx`

**URLs**:
- Development: `*.dev.ai-aas.local`
- Production: `*.prod.ai-aas.local`

**Documentation**:
- `docs/platform/tls-ssl-setup.md` - TLS setup guide
- `infra/secrets/certs/README.md` - Certificate management
- `tmp_md/INGRESS_CONFIGURATION_SUMMARY.md` - Ingress config summary

