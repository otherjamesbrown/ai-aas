# Remote Testing Quick Start

## Quick Reference

### Deploy to Remote Development Cluster

```bash
# 1. Build and push image (or let GitHub Actions do it)
docker build -f web/portal/Dockerfile -t ghcr.io/otherjamesbrown/web-portal:dev .
docker push ghcr.io/otherjamesbrown/web-portal:dev

# 2. Deploy via ArgoCD
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context <your-dev-context>
argocd app sync web-portal-development

# 3. Verify deployment
kubectl -n development get pods -l app=web-portal
curl https://portal.ai-aas.dev/healthz
```

### Run Tests Against Remote Deployment

**Option 1: Use helper script (easiest)**
```bash
./scripts/test-remote-deployment.sh

# Run specific test file
./scripts/test-remote-deployment.sh tests/e2e/api-keys-inference.spec.ts
```

**Option 2: Manual command**
```bash
cd web/portal
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
API_ROUTER_URL=https://api.ai-aas.dev \
pnpm test:e2e
```

### Common Commands

```bash
# Check deployment status
kubectl -n development get all -l app=web-portal

# View logs
kubectl -n development logs -f deployment/web-portal

# Restart deployment
kubectl -n development rollout restart deployment/web-portal

# Check ArgoCD sync status
argocd app get web-portal-development

# Port-forward for local access
kubectl -n development port-forward svc/web-portal 8080:80
```

## Why Remote Testing?

- ✅ **Bypasses local Playwright/Fosite compatibility issues**
- ✅ **Tests against real Kubernetes deployment**
- ✅ **Validates ingress, DNS, and network configuration**
- ✅ **Production-like environment**

## Full Documentation

See `tmp_md/REMOTE_DEV_DEPLOYMENT_GUIDE.md` for complete instructions.

