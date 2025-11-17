# ArgoCD Testing Guide for Beginners

**Last Updated**: 2025-11-16  
**Audience**: Beginners to ArgoCD

## Quick Answer

**ArgoCD does NOT run your application tests** (like unit tests, integration tests, etc.). However, ArgoCD **does** perform validation and health checks to ensure deployments work correctly.

## Understanding the Testing Flow

### Where Tests Run: BEFORE ArgoCD

```
┌─────────────────────────────────────────────────────────────┐
│ 1. DEVELOPER COMMITS CODE                                    │
│    └─> Push to feature branch                               │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 2. PULL REQUEST CREATED                                      │
│    └─> Code review happens here                             │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 3. CI PIPELINE RUNS (GitHub Actions)                        │
│    ✅ Unit tests                                             │
│    ✅ Integration tests                                      │
│    ✅ E2E tests (including critical user workflows)          │
│    ✅ Linting                                                │
│    ✅ Build verification                                     │
│    ✅ Security scans                                         │
│    └─> If tests FAIL, PR cannot be merged                   │
│    └─> Build jobs depend on test jobs passing               │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 4. PR MERGED TO MAIN                                         │
│    └─> Only happens if CI tests pass                        │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 5. ARGOCD DETECTS CHANGES                                    │
│    └─> Watches main branch                                   │
└─────────────────────────────────────────────────────────────┘
                        ↓
┌─────────────────────────────────────────────────────────────┐
│ 6. ARGOCD VALIDATES & DEPLOYS                                │
│    ✅ Validates Kubernetes manifests                         │
│    ✅ Checks resource health                                 │
│    ✅ Monitors pod startup                                   │
│    └─> Deploys to cluster                                   │
└─────────────────────────────────────────────────────────────┘
```

## What ArgoCD DOES Test/Validate

### 1. **Kubernetes Manifest Validation**

ArgoCD validates that your Kubernetes manifests are:
- ✅ Valid YAML/JSON
- ✅ Correct Kubernetes API schema
- ✅ Resource definitions are correct

**Example:**
```yaml
# ArgoCD will catch this error:
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
spec:
  containers:
    - name: my-container
      image: nginx
      # Missing required fields will be caught
```

### 2. **Health Checks**

ArgoCD monitors your deployed resources to ensure they're healthy:

**Pod Health:**
- ✅ Pods are running (not crashed)
- ✅ Pods pass liveness probes
- ✅ Pods pass readiness probes
- ✅ Pods are not in error state

**Service Health:**
- ✅ Services have endpoints
- ✅ Services are accessible

**Example Health Check:**
```yaml
# In your Helm chart, define health checks:
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
```

### 3. **Sync Status Monitoring**

ArgoCD continuously monitors if your cluster matches the desired state in Git:

- ✅ Resources match Git state
- ✅ No manual changes detected
- ✅ All resources synced correctly

**Check sync status:**
```bash
argocd app get web-portal-development
# Shows: Sync Status, Health Status
```

### 4. **Pre-Sync Hooks (Optional)**

You can configure ArgoCD to run validation jobs **before** deploying:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: my-app
spec:
  syncPolicy:
    syncOptions:
      - PreSync
    hooks:
      preSync:
        - name: run-tests
          kind: Job
          spec:
            template:
              spec:
                containers:
                  - name: test-runner
                    image: my-test-image
                    command: ["/bin/sh", "-c", "npm test"]
                restartPolicy: Never
```

**Note:** This is **not** commonly used. Tests should run in CI, not during deployment.

## What ArgoCD DOES NOT Do

### ❌ Run Unit Tests
- Unit tests run in CI (GitHub Actions), not ArgoCD
- ArgoCD doesn't execute your test code

### ❌ Run Integration Tests
- Integration tests run in CI, not ArgoCD
- ArgoCD doesn't know about your test suite

### ❌ Validate Business Logic
- ArgoCD only validates Kubernetes resources
- It doesn't test if your application code works correctly

### ❌ Run E2E Tests
- End-to-end tests should run in CI or separate test environments
- ArgoCD focuses on deployment validation

## Best Practice: Test Before Deploy

### The Right Way

```
1. Write code
   ↓
2. Write tests
   ↓
3. Run tests locally
   ↓
4. Push to feature branch
   ↓
5. CI runs tests automatically
   ↓
6. Fix any failing tests
   ↓
7. Merge PR (only if tests pass)
   ↓
8. ArgoCD deploys (validates deployment)
```

### Example CI Pipeline (GitHub Actions)

**Go Services** (`.github/workflows/ci.yml`):
```yaml
# .github/workflows/ci.yml
name: CI
on:
  pull_request:
    branches: [main]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Run unit tests
        run: make test SERVICE=user-org-service
      
      - name: Run integration tests
        run: make test-integration SERVICE=user-org-service
      
      - name: Run linting
        run: make lint SERVICE=user-org-service
      
      # Only if ALL tests pass, PR can be merged
      # Then ArgoCD will deploy
```

**Web Portal** (`.github/workflows/web-portal.yml`):
```yaml
# .github/workflows/web-portal.yml
jobs:
  lint:
    # Runs ESLint
  test:
    # Runs Vitest unit tests
  test-e2e:
    # Runs Playwright E2E tests (including sign-in)
  build:
    needs: [lint, test, test-e2e]  # Build only if all tests pass
    # Builds and pushes Docker image
```

**Critical**: The web portal build job depends on all test jobs (`lint`, `test`, `test-e2e`) passing. This prevents broken code from being deployed, even if it compiles successfully.

## ArgoCD Health Check Examples

### Check Application Health

```bash
# View application status
argocd app get web-portal-development

# Output shows:
# - Sync Status: Synced/OutOfSync
# - Health Status: Healthy/Degraded/Unhealthy
# - Resources: All resources and their health
```

### Monitor Deployment

```bash
# Watch deployment progress
watch -n 2 'argocd app get web-portal-development'

# Check specific resource health
argocd app get web-portal-development --show-resources
```

### Health Check Failures

If ArgoCD reports unhealthy:

```bash
# Check pod status
kubectl get pods -n development

# Check pod logs
kubectl logs -n development <pod-name>

# Check events
kubectl get events -n development --sort-by='.lastTimestamp'
```

## Adding Health Checks to Your Application

### For Go Services

```go
// Add health check endpoint
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
    // Check database connection
    if err := s.db.Ping(); err != nil {
        http.Error(w, "Database unhealthy", http.StatusServiceUnavailable)
        return
    }
    
    // Check Redis connection
    if err := s.redis.Ping(); err != nil {
        http.Error(w, "Redis unhealthy", http.StatusServiceUnavailable)
        return
    }
    
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "healthy",
    })
}
```

### In Kubernetes Deployment

```yaml
# Add to your Helm chart values
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  timeoutSeconds: 5
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  timeoutSeconds: 3
  failureThreshold: 3
```

## Common Scenarios

### Scenario 1: Tests Pass, But Deployment Fails

**Problem:** CI tests pass, but ArgoCD shows unhealthy pods.

**Solution:**
1. Check pod logs: `kubectl logs <pod-name>`
2. Check health endpoints: `curl http://<pod-ip>:8080/healthz`
3. Verify environment variables are set correctly
4. Check resource limits (CPU/memory)

### Scenario 2: Deployment Succeeds, But Service Doesn't Work

**Problem:** ArgoCD shows "Healthy", but service doesn't respond.

**Solution:**
1. Check service endpoints: `kubectl get endpoints`
2. Test service directly: `kubectl port-forward svc/my-service 8080:80`
3. Check ingress configuration
4. Verify DNS resolution

### Scenario 3: Want to Run Tests Before Deployment

**Problem:** You want ArgoCD to run tests before deploying.

**Solution:**
1. **Don't do this** - Tests should run in CI
2. Use pre-sync hooks only for deployment validation (not business logic tests)
3. Ensure CI pipeline blocks merges if tests fail

## Summary

| What | Where It Runs | Purpose |
|------|---------------|---------|
| Unit Tests | CI (GitHub Actions) | Test code logic |
| Integration Tests | CI (GitHub Actions) | Test component interactions |
| E2E Tests | CI or Test Environment | Test full system |
| Manifest Validation | ArgoCD | Validate Kubernetes resources |
| Health Checks | ArgoCD | Monitor deployment health |
| Sync Validation | ArgoCD | Ensure Git state matches cluster |

## Key Takeaways

1. ✅ **Tests run in CI** (GitHub Actions), not ArgoCD
2. ✅ **ArgoCD validates deployments**, not application logic
3. ✅ **Health checks** ensure pods/services are running correctly
4. ✅ **Sync monitoring** ensures cluster matches Git state
5. ✅ **Pre-sync hooks** can validate deployment, but shouldn't replace CI tests

## Next Steps

- Read: `docs/platform/ci-cd-pipeline.md` - Full CI/CD overview
- Read: `docs/runbooks/argocd-deployment-workflow.md` - ArgoCD workflow
- Practice: Check ArgoCD app status: `argocd app get <app-name>`
- Practice: Monitor health: `kubectl get pods -n <namespace>`

## Related Documentation

- `docs/platform/endpoints-and-urls.md` - Complete endpoint and URL configuration guide
- `docs/platform/ci-cd-pipeline.md` - CI/CD pipeline overview
- `docs/runbooks/argocd-deployment-workflow.md` - ArgoCD deployment guide
- `docs/runbooks/deploy-to-environments.md` - Complete deployment guide
- `.github/workflows/ci.yml` - CI pipeline definition

