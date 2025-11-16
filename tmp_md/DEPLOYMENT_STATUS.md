# Web Portal Deployment & Testing Status

## ✅ Completed Steps

1. **Docker Image Built**: Successfully built web portal Docker image
   - Image: `ghcr.io/otherjamesbrown/web-portal:dev`
   - Image: `ghcr.io/otherjamesbrown/web-portal:latest`
   - Status: ✅ Built locally

2. **Image Push**: Attempted to push to registry
   - Issue: GitHub token doesn't have required scopes for GHCR push
   - Solution Options:
     - Use GitHub Actions workflow (requires push to main or workflow_dispatch)
     - Create GitHub Personal Access Token with `write:packages` scope
     - Use existing `dev` image if already in registry

3. **Local Container Test**: Successfully ran container locally
   - Container ran on port 8080
   - Health check passed: ✅
   - Stopped container (conflicts with api-router-service port)

## 📋 Required for Full Testing

The Playwright E2E tests require these services:

1. **user-org-service** on `http://localhost:8081`
2. **mock-inference** on `http://localhost:8000`
3. **api-router-service** on `http://localhost:8080` (optional)
4. **web-portal** accessible via `PLAYWRIGHT_BASE_URL`

## 🚀 Next Steps - Choose One Path

### Option 1: Deploy to Remote Development Cluster (Recommended)

**Prerequisites:**
- Kubernetes cluster access configured
- ArgoCD installed and accessible
- Backend services already deployed in cluster

**Steps:**
```bash
# 1. Push image via GitHub Actions (or get token with write:packages scope)
#    Option A: Merge to main branch (triggers workflow automatically)
#    Option B: Trigger workflow manually: gh workflow run web-portal.yml

# 2. Deploy via ArgoCD
export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
kubectl config use-context <your-dev-context>
argocd app sync web-portal-development

# 3. Verify deployment
kubectl -n development get pods -l app=web-portal
curl https://portal.ai-aas.dev/healthz

# 4. Run tests
cd web/portal
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
API_ROUTER_URL=https://api.ai-aas.dev \
pnpm test:e2e
```

### Option 2: Run Full Stack Locally

**Steps:**
```bash
# 1. Start backend services
cd /home/dev/ai-aas
make up  # Start local dev stack (postgres, redis, etc.)

# Start user-org-service
cd services/user-org-service
make run  # Runs on port 8081

# Start api-router-service (if needed)
cd services/api-router-service
make run  # Runs on port 8080

# Start mock-inference (if needed)
# Check mock-inference service setup

# 2. Run web portal locally (dev mode)
cd web/portal
pnpm dev  # Runs on port 5173

# 3. Run tests
SKIP_WEBSERVER=true \
PLAYWRIGHT_BASE_URL=http://localhost:5173 \
API_ROUTER_URL=http://localhost:8080 \
pnpm test:e2e
```

### Option 3: Use Containerized Local Stack

**Steps:**
```bash
# 1. Run web portal container on different port
docker run -d --name web-portal-test -p 8082:80 \
  -e VITE_API_BASE_URL=http://localhost:8080/api \
  ghcr.io/otherjamesbrown/web-portal:dev

# 2. Ensure backend services are running (see Option 2)

# 3. Run tests
cd web/portal
PLAYWRIGHT_BASE_URL=http://localhost:8082 \
SKIP_WEBSERVER=true \
API_ROUTER_URL=http://localhost:8080 \
pnpm test:e2e
```

## 🔍 Current Status

- ✅ Docker image built successfully
- ⚠️ Image push blocked (need GitHub token with packages:write scope)
- ⚠️ Backend services not running locally
- ⚠️ Remote deployment not accessible (may need cluster access setup)

## 📝 Recommendations

1. **For immediate testing**: Use Option 2 (local dev stack) if you have backend services available
2. **For production-like testing**: Use Option 1 (remote cluster) - requires cluster access setup
3. **For CI/CD**: Push to main branch to trigger GitHub Actions workflow

## 🔗 Related Documentation

- `tmp_md/REMOTE_DEV_DEPLOYMENT_GUIDE.md` - Complete remote deployment guide
- `tmp_md/REMOTE_TESTING_QUICKSTART.md` - Quick reference
- `scripts/test-remote-deployment.sh` - Helper script for remote testing
- `tests/README.md` - Testing documentation

