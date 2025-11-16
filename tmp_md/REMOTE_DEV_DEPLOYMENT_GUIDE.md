# Remote Development Cluster Deployment & Testing Guide

## Overview

This guide walks through deploying the web portal to the remote development Kubernetes cluster and running Playwright E2E tests against the deployed instance. This approach bypasses local Playwright/Fosite compatibility issues by testing against a real deployment.

## Prerequisites

1. **Kubernetes Access**: Access to the remote development cluster
   ```bash
   # Verify kubeconfig is configured
   export KUBECONFIG=~/kubeconfigs/kubeconfig-development.yaml
   kubectl config get-contexts
   kubectl config use-context <your-dev-context>
   ```

2. **ArgoCD Access**: ArgoCD should be installed and configured
   ```bash
   # Check if ArgoCD is accessible
   kubectl -n argocd get pods
   
   # Port-forward ArgoCD UI (optional)
   kubectl -n argocd port-forward svc/argocd-server 8080:443
   ```

3. **Container Registry Access**: GitHub Container Registry (ghcr.io) access
   ```bash
   # Login to GitHub Container Registry
   echo $GITHUB_TOKEN | docker login ghcr.io -u <your-username> --password-stdin
   ```

4. **Required Services**: Ensure backend services are deployed:
   - `user-org-service` in `development` namespace
   - `api-router-service` in `development` namespace
   - Database and Redis available

## Step 1: Build and Push Docker Image

### Option A: Use GitHub Actions (Recommended)

The web portal CI/CD workflow automatically builds and pushes images on push to `main`:

```bash
# Push your changes to trigger build
git push origin main

# Or trigger manually via workflow_dispatch
gh workflow run web-portal.yml
```

The workflow will:
- Build the Docker image
- Tag it with: `dev`, `latest`, `main-<sha>`, `<short-sha>`
- Push to `ghcr.io/otherjamesbrown/web-portal`

### Option B: Build and Push Manually

```bash
# Build the image
cd /home/dev/ai-aas
docker build -f web/portal/Dockerfile \
  -t ghcr.io/otherjamesbrown/web-portal:dev \
  -t ghcr.io/otherjamesbrown/web-portal:latest \
  .

# Login to registry
echo $GITHUB_TOKEN | docker login ghcr.io -u <your-username> --password-stdin

# Push the image
docker push ghcr.io/otherjamesbrown/web-portal:dev
docker push ghcr.io/otherjamesbrown/web-portal:latest
```

## Step 2: Deploy via ArgoCD

### Check Current Deployment Status

```bash
# List ArgoCD applications
argocd app list

# Check web portal application status
argocd app get web-portal-development

# View application details
kubectl -n argocd get application web-portal-development -o yaml
```

### Deploy/Update Application

The ArgoCD application manifest is already configured at `gitops/clusters/development/apps/web-portal.yaml`. ArgoCD will automatically sync if `syncPolicy.automated` is enabled.

**Manual Sync** (if needed):
```bash
# Sync the application
argocd app sync web-portal-development

# Watch sync status
argocd app get web-portal-development --watch

# Or watch via kubectl
kubectl -n argocd get application web-portal-development -w
```

**Verify Deployment**:
```bash
# Check pods
kubectl -n development get pods -l app=web-portal

# Check service
kubectl -n development get svc web-portal

# Check ingress
kubectl -n development get ingress web-portal

# View logs
kubectl -n development logs -l app=web-portal --tail=100 -f
```

### Update Image Tag (if needed)

If you need to force a specific image tag:

```bash
# Edit the ArgoCD application
kubectl -n argocd edit application web-portal-development

# Update the image tag in the values section:
#   image:
#     tag: dev  # or specific tag like main-abc1234
```

Then sync:
```bash
argocd app sync web-portal-development
```

## Step 3: Verify Deployment

### Check Ingress and DNS

The web portal should be accessible via:
- **HTTPS**: `https://portal.ai-aas.dev`
- **Internal**: `http://web-portal.development.svc.cluster.local`

```bash
# Check ingress configuration
kubectl -n development get ingress web-portal -o yaml

# Test internal access (from within cluster)
kubectl -n development run curl-test --image=curlimages/curl --rm -it -- \
  curl -v http://web-portal.development.svc.cluster.local/healthz

# Test external access
curl -v https://portal.ai-aas.dev/healthz
```

### Verify Environment Variables

The deployment should have these environment variables configured:
- `VITE_API_BASE_URL`: API router service URL
- `VITE_OAUTH_CLIENT_ID`: OAuth client ID
- `VITE_OAUTH_ISSUER_URL`: OAuth issuer URL
- `VITE_OAUTH_REDIRECT_URI`: OAuth redirect URI

```bash
# Check environment variables in pod
kubectl -n development get pod -l app=web-portal -o jsonpath='{.items[0].spec.containers[0].env}' | jq
```

## Step 4: Run Playwright Tests Against Remote Deployment

### Configure Test Environment

Create a test configuration file or set environment variables:

```bash
# Set base URL for Playwright tests
export PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev

# Set API router URL (for tests that call APIs directly)
export API_ROUTER_URL=https://api-router-service.development.svc.cluster.local:8080
# OR if you have external ingress:
# export API_ROUTER_URL=https://api.ai-aas.dev

# Skip web server (we're using remote deployment)
export SKIP_WEBSERVER=true

# Run in headless mode (or set to false to see browser)
export PLAYWRIGHT_HEADLESS=true
```

### Run E2E Tests

```bash
cd web/portal

# Run all E2E tests
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
API_ROUTER_URL=https://api.ai-aas.dev \
pnpm test:e2e

# Run specific test file
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
API_ROUTER_URL=https://api.ai-aas.dev \
pnpm playwright test tests/e2e/api-keys-inference.spec.ts

# Run with visible browser (for debugging)
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
PLAYWRIGHT_HEADLESS=false \
pnpm test:e2e

# Run in debug mode
PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
SKIP_WEBSERVER=true \
pnpm test:e2e:debug
```

### Test Configuration Notes

The Playwright config (`web/portal/playwright.config.ts`) uses:
- `baseURL`: From `PLAYWRIGHT_BASE_URL` env var (defaults to localhost)
- `ignoreHTTPSErrors: true`: Allows self-signed certificates
- `headless`: From `PLAYWRIGHT_HEADLESS` env var (defaults to true)

## Step 5: Troubleshooting

### Deployment Issues

**Pods not starting**:
```bash
# Check pod events
kubectl -n development describe pod -l app=web-portal

# Check logs
kubectl -n development logs -l app=web-portal --tail=100

# Check image pull errors
kubectl -n development get events --sort-by='.lastTimestamp' | grep web-portal
```

**Image pull errors**:
- Verify image exists: `docker pull ghcr.io/otherjamesbrown/web-portal:dev`
- Check image pull secrets: `kubectl -n development get secrets`
- Verify registry access from cluster

**Ingress not working**:
```bash
# Check ingress controller
kubectl -n ingress-nginx get pods

# Check ingress status
kubectl -n development describe ingress web-portal

# Check DNS resolution
nslookup portal.ai-aas.dev
```

### Test Issues

**Tests can't reach deployment**:
- Verify `PLAYWRIGHT_BASE_URL` is correct
- Check network connectivity: `curl https://portal.ai-aas.dev/healthz`
- Verify DNS resolution

**Authentication failures**:
- Check `VITE_OAUTH_ISSUER_URL` matches backend service
- Verify OAuth client ID/secret are configured
- Check backend service logs for auth errors

**CORS errors**:
- Verify backend CORS configuration allows the portal domain
- Check browser console for CORS error details
- Verify `VITE_API_BASE_URL` matches backend service

**SSL certificate errors**:
- Playwright config has `ignoreHTTPSErrors: true` by default
- For staging certs, this should work automatically
- If issues persist, check cert-manager status

### Common Commands

```bash
# View all resources for web portal
kubectl -n development get all -l app=web-portal

# Restart deployment
kubectl -n development rollout restart deployment/web-portal

# View ArgoCD sync status
argocd app get web-portal-development

# Force sync
argocd app sync web-portal-development --force

# View application logs
kubectl -n development logs -f deployment/web-portal

# Port-forward for local testing
kubectl -n development port-forward svc/web-portal 8080:80
# Then test: curl http://localhost:8080/healthz
```

## Step 6: Continuous Testing Workflow

### Automated Testing Script

Create a script to automate the testing workflow:

```bash
#!/bin/bash
# scripts/test-remote-deployment.sh

set -euo pipefail

# Configuration
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-https://portal.ai-aas.dev}"
API_ROUTER_URL="${API_ROUTER_URL:-https://api.ai-aas.dev}"
SKIP_WEBSERVER=true

echo "Testing remote deployment at $PLAYWRIGHT_BASE_URL"

# Verify deployment is accessible
echo "Checking deployment health..."
if ! curl -f -s "${PLAYWRIGHT_BASE_URL}/healthz" > /dev/null; then
  echo "ERROR: Deployment not accessible at $PLAYWRIGHT_BASE_URL"
  exit 1
fi

# Run tests
cd web/portal
PLAYWRIGHT_BASE_URL="$PLAYWRIGHT_BASE_URL" \
SKIP_WEBSERVER="$SKIP_WEBSERVER" \
API_ROUTER_URL="$API_ROUTER_URL" \
pnpm test:e2e

echo "Tests completed successfully!"
```

### Integration with CI/CD

You can integrate remote testing into your CI/CD pipeline:

1. **After deployment**: Add a test job that runs after ArgoCD sync
2. **Scheduled tests**: Run tests periodically against dev cluster
3. **Pre-production**: Run full test suite before promoting to staging

Example GitHub Actions workflow addition:
```yaml
test-remote:
  name: Test Remote Deployment
  runs-on: ubuntu-latest
  needs: [deploy]
  steps:
    - uses: actions/checkout@v4
    - uses: actions/setup-node@v4
      with:
        node-version: '20'
    - run: cd web/portal && pnpm install
    - run: |
        PLAYWRIGHT_BASE_URL=https://portal.ai-aas.dev \
        SKIP_WEBSERVER=true \
        pnpm test:e2e
```

## Benefits of Remote Testing

1. **Real Environment**: Tests run against actual Kubernetes deployment
2. **Bypasses Local Issues**: Avoids Playwright/Fosite compatibility problems
3. **Production-like**: Tests in environment closer to production
4. **Network Testing**: Validates ingress, DNS, and network policies
5. **Integration Testing**: Tests full stack integration

## Next Steps

1. **Monitor Test Results**: Track test success/failure rates
2. **Add More Tests**: Expand test coverage for critical flows
3. **Performance Testing**: Add load tests against remote deployment
4. **Automate Deployment**: Set up automated deployment on merge to main
5. **Staging Environment**: Set up staging cluster for pre-production testing

## Related Documentation

- [ArgoCD Setup Guide](../services/user-org-service/docs/ARGOCD-SETUP-GUIDE.md)
- [Web Portal Deployment](../gitops/clusters/development/apps/web-portal.yaml)
- [Playwright Configuration](../web/portal/playwright.config.ts)
- [Testing Guide](../tests/README.md)
- [API Key Creation Issue Summary](./API_KEY_CREATION_ISSUE_SUMMARY.md)

