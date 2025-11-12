# End-to-End Testing Guide

This document describes how to run end-to-end tests for the user-org-service, both locally and against deployed environments.

## Overview

The e2e-test binary exercises the complete user and organization lifecycle flows:
- Health checks
- Organization CRUD operations
- User invitation and management
- Authentication flows (when seeded data is available)

## Running Tests Locally

### Prerequisites

1. Service must be running locally:
   ```bash
   make run  # Starts admin-api on port 8081
   ```

2. Database must be migrated:
   ```bash
   make migrate
   ```

### Run Tests

```bash
# Run against localhost (default)
make e2e-test-local

# Or specify custom API URL
API_URL=http://localhost:8081 make e2e-test
```

## Running Tests Against Development Environment

### Option 1: Direct HTTP Access

If you have network access to the development environment:

```bash
# Set the development API URL
export DEV_API_URL=http://user-org-service.dev.platform.internal:8081

# Run tests
make e2e-test-dev
```

### Option 2: Kubernetes Job (Recommended)

Deploy the test as a Kubernetes Job that runs inside the cluster:

#### Build and Push Docker Image

```bash
# Build the test image
make docker-build-e2e-test

# Tag and push to your container registry
docker tag user-org-service-e2e-test:latest \
  your-registry/user-org-service-e2e-test:latest
docker push your-registry/user-org-service-e2e-test:latest
```

#### Deploy Test Job

```bash
# Update the image in the job manifest
sed -i 's|user-org-service-e2e-test:latest|your-registry/user-org-service-e2e-test:latest|g' \
  configs/k8s/e2e-test-job.yaml

# Create a unique job name
TIMESTAMP=$(date +%s)
sed "s/{{TIMESTAMP}}/$TIMESTAMP/g" configs/k8s/e2e-test-job.yaml | kubectl apply -f -

# Watch the job
kubectl get job e2e-test-$TIMESTAMP -n development -w

# View logs
kubectl logs -f job/e2e-test-$TIMESTAMP -n development

# Check results
kubectl get job e2e-test-$TIMESTAMP -n development
```

#### Clean Up

```bash
# Delete the job (auto-cleanup after 1 hour, but you can delete manually)
kubectl delete job e2e-test-$TIMESTAMP -n development
```

## CI/CD Integration

### GitHub Actions Example

Add to `.github/workflows/user-org-service.yml`:

```yaml
- name: E2E Tests
  if: github.ref == 'refs/heads/main' || github.event_name == 'pull_request'
  run: |
    export DEV_API_URL="http://user-org-service.dev.svc.cluster.local:8081"
    make e2e-test-dev
```

### Post-Deployment Testing

After deploying to development:

```bash
# In your deployment pipeline
kubectl create job e2e-test-post-deploy-$(date +%s) \
  --from=job/e2e-test-template \
  --env="API_URL=http://user-org-service.dev.svc.cluster.local:8081" \
  -n development
```

## Test Coverage

Current tests cover:

1. **Health Check** - Verifies service is reachable
2. **Organization Lifecycle** - Create, read, update operations
3. **User Invite Flow** - Invite creation and user retrieval
4. **User Management** - Status updates, profile updates, suspend/activate
5. **Authentication Flow** - Login, refresh, logout (requires seeded user)

## Extending Tests

To add new test cases:

1. Add a new test function in `cmd/e2e-test/main.go`:
   ```go
   func testNewFeature(tc *testContext, client *http.Client, apiURL string) error {
       // Test implementation
       return nil
   }
   ```

2. Register it in the test suite:
   ```go
   tests := []struct {
       name string
       fn   func(*testContext, *http.Client, string) error
   }{
       // ... existing tests
       {"TestNewFeature", testNewFeature},
   }
   ```

## Troubleshooting

### Tests Fail with Connection Refused

- Verify the service is running: `curl http://localhost:8081/healthz`
- Check API_URL environment variable is correct
- For Kubernetes: verify service name and namespace

### Tests Fail with 404 Not Found

- Verify database migrations are applied
- Check service logs for errors
- Ensure test data doesn't conflict with existing data

### Tests Timeout

- Increase timeout in test client (default 30s)
- Check network connectivity to service
- Verify service is not overloaded

## Manual Verification

After automated tests pass, manually verify:

1. **Organization Creation**:
   ```bash
   curl -X POST http://localhost:8081/v1/orgs \
     -H 'Content-Type: application/json' \
     -d '{"name":"Test Org","slug":"test-org"}'
   ```

2. **User Invite**:
   ```bash
   curl -X POST http://localhost:8081/v1/orgs/test-org/invites \
     -H 'Content-Type: application/json' \
     -d '{"email":"user@example.com"}'
   ```

3. **User Management**:
   ```bash
   curl -X PATCH http://localhost:8081/v1/orgs/test-org/users/{userId} \
     -H 'Content-Type: application/json' \
     -d '{"status":"active"}'
   ```

See `quickstart.md` for more detailed manual verification steps.

