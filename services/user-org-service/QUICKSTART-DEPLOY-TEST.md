# Quick Start: Deploy E2E Test to Development

This is a quick reference for deploying and running the e2e-test in the development environment.

## Prerequisites Check

```bash
# 1. Verify Docker is running
docker ps

# 2. Verify kubectl is configured
kubectl --context=dev-platform get nodes

# 3. Verify you can access the registry (if using GHCR)
docker login ghcr.io

# 4. Verify the service is deployed
kubectl --context=dev-platform -n user-org-service get svc user-org-service
```

## One-Command Deployment

```bash
cd services/user-org-service
make deploy-e2e-test
```

This will:
- Build the Docker image
- Push to `ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest`
- Create a Kubernetes Job in the `user-org-service` namespace
- Stream logs and report results

## Custom Configuration

```bash
# Use custom registry
REGISTRY=my-registry.io make deploy-e2e-test

# Use custom namespace (if service is deployed elsewhere)
NAMESPACE=development make deploy-e2e-test

# Use custom Kubernetes context
KUBECTL_CONTEXT=my-cluster make deploy-e2e-test

# Use commit SHA as tag
IMAGE_TAG=$(git rev-parse --short HEAD) make deploy-e2e-test
```

## Manual Steps (if script doesn't work)

### 1. Build and Push Image

```bash
cd services/user-org-service

# Build
docker build -f Dockerfile.e2e-test \
  -t ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest .

# Push
docker push ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest
```

### 2. Create Job

```bash
TIMESTAMP=$(date +%s)
cat <<EOF | kubectl --context=dev-platform apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: e2e-test-${TIMESTAMP}
  namespace: user-org-service
spec:
  ttlSecondsAfterFinished: 3600
  backoffLimit: 2
  template:
    spec:
      restartPolicy: Never
      containers:
      - name: e2e-test
        image: ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest
        imagePullPolicy: Always
        env:
        - name: API_URL
          value: "http://user-org-service.user-org-service.svc.cluster.local:8081"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
EOF
```

### 3. Watch Results

```bash
# Watch job
kubectl --context=dev-platform -n user-org-service get job e2e-test-${TIMESTAMP} -w

# View logs
kubectl --context=dev-platform -n user-org-service logs -f job/e2e-test-${TIMESTAMP}
```

## Troubleshooting

**Service not found?**
```bash
# Check service exists
kubectl --context=dev-platform -n user-org-service get svc

# Check pods are running
kubectl --context=dev-platform -n user-org-service get pods

# Test connectivity from within cluster
kubectl --context=dev-platform -n user-org-service run test-curl \
  --image=curlimages/curl --rm -it -- \
  curl http://user-org-service.user-org-service.svc.cluster.local:8081/healthz
```

**Image pull errors?**
```bash
# Verify image exists
docker pull ghcr.io/otherjamesbrown/user-org-service-e2e-test:latest

# Check registry credentials
kubectl --context=dev-platform -n user-org-service get secrets | grep regcred
```

**Job stuck?**
```bash
# Check pod events
kubectl --context=dev-platform -n user-org-service describe pod -l job-name=e2e-test-${TIMESTAMP}

# Check resource quotas
kubectl --context=dev-platform -n user-org-service describe quota
```

## Next Steps

After tests pass:
1. Review test output for any warnings
2. Check service logs for audit events
3. Verify test data was created correctly
4. Clean up test job (auto-cleanup after 1 hour)

For more details, see `docs/deploy-e2e-test.md`.

