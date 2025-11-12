# Deploying E2E Tests to Development Environment

This guide explains how to deploy and run the end-to-end tests against the deployed user-org-service in the development Kubernetes cluster.

## Prerequisites

1. **Docker** - Must be running and configured
2. **kubectl** - Configured with access to the development cluster
3. **Container Registry Access** - Authenticated to push images (default: `ghcr.io/otherjamesbrown`)
4. **Service Deployed** - The user-org-service must be running in the development namespace

## Quick Start

```bash
# From the service root directory
make deploy-e2e-test
```

This will:
1. Build the e2e-test Docker image
2. Push it to the container registry
3. Create a Kubernetes Job in the development namespace
4. Stream the test logs
5. Report success/failure

## Manual Deployment

### Step 1: Build and Push Image

```bash
# Set your registry (default: ghcr.io/otherjamesbrown)
export REGISTRY=ghcr.io/otherjamesbrown
export IMAGE_TAG=latest

# Build the image
docker build -f Dockerfile.e2e-test \
  -t ${REGISTRY}/user-org-service-e2e-test:${IMAGE_TAG} .

# Push to registry
docker push ${REGISTRY}/user-org-service-e2e-test:${IMAGE_TAG}
```

### Step 2: Create Kubernetes Job

```bash
# Set variables
export NAMESPACE=development
export KUBECTL_CONTEXT=dev-platform
export IMAGE=${REGISTRY}/user-org-service-e2e-test:${IMAGE_TAG}
export TIMESTAMP=$(date +%s)
export JOB_NAME=e2e-test-${TIMESTAMP}

# Create job manifest
cat <<EOF | kubectl --context=${KUBECTL_CONTEXT} apply -f -
apiVersion: batch/v1
kind: Job
metadata:
  name: ${JOB_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: user-org-service
    component: e2e-test
spec:
  ttlSecondsAfterFinished: 3600
  backoffLimit: 2
  template:
    metadata:
      labels:
        app: user-org-service
        component: e2e-test
    spec:
      restartPolicy: Never
      containers:
      - name: e2e-test
        image: ${IMAGE}
        imagePullPolicy: Always
        env:
        - name: API_URL
          value: "http://user-org-service.${NAMESPACE}.svc.cluster.local:8081"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
EOF
```

### Step 3: Monitor Job

```bash
# Watch job status
kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} get job ${JOB_NAME} -w

# View logs
kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} logs -f job/${JOB_NAME}

# Check pod status
kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} get pods -l job-name=${JOB_NAME}
```

### Step 4: Clean Up

```bash
# Delete the job (auto-cleanup after 1 hour, but you can delete manually)
kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} delete job ${JOB_NAME}
```

## Using the Deployment Script

The `scripts/deploy-e2e-test.sh` script automates the entire process:

```bash
# Basic usage (uses defaults)
./scripts/deploy-e2e-test.sh

# Custom namespace
./scripts/deploy-e2e-test.sh --namespace staging

# Custom registry and tag
./scripts/deploy-e2e-test.sh --registry my-registry.io --tag v1.0.0

# Custom Kubernetes context
./scripts/deploy-e2e-test.sh --context my-cluster
```

### Environment Variables

You can also set these via environment variables:

```bash
export REGISTRY=ghcr.io/otherjamesbrown
export IMAGE_TAG=latest
export KUBECTL_CONTEXT=dev-platform
export NAMESPACE=development
export SERVICE_NAME=user-org-service

./scripts/deploy-e2e-test.sh
```

## Troubleshooting

### Image Pull Errors

If the job fails with image pull errors:

1. Verify you're logged into the registry:
   ```bash
   docker login ghcr.io
   ```

2. Check image exists:
   ```bash
   docker pull ${REGISTRY}/user-org-service-e2e-test:${IMAGE_TAG}
   ```

3. Verify Kubernetes has pull secrets configured for the registry

### Service Not Found

If tests fail with connection errors:

1. Verify service is running:
   ```bash
   kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} get svc user-org-service
   ```

2. Check service endpoints:
   ```bash
   kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} get endpoints user-org-service
   ```

3. Test connectivity from within cluster:
   ```bash
   kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} run test-curl --image=curlimages/curl --rm -it -- \
     curl http://user-org-service.${NAMESPACE}.svc.cluster.local:8081/healthz
   ```

### Job Stuck in Pending

1. Check pod events:
   ```bash
   kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} describe pod -l job-name=${JOB_NAME}
   ```

2. Check resource quotas:
   ```bash
   kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} describe quota
   ```

3. Check node resources:
   ```bash
   kubectl --context=${KUBECTL_CONTEXT} top nodes
   ```

## CI/CD Integration

### GitHub Actions Example

```yaml
- name: Deploy and Run E2E Tests
  run: |
    export REGISTRY=ghcr.io
    export IMAGE_TAG=${{ github.sha }}
    export KUBECTL_CONTEXT=dev-platform
    export NAMESPACE=development
    
    # Authenticate to registry
    echo "${{ secrets.GITHUB_TOKEN }}" | docker login ghcr.io -u ${{ github.actor }} --password-stdin
    
    # Deploy and run tests
    cd services/user-org-service
    ./scripts/deploy-e2e-test.sh --registry ${REGISTRY} --tag ${IMAGE_TAG}
```

### ArgoCD Integration

You can create an ArgoCD Application to manage the test job:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: user-org-service-e2e-test
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/otherjamesbrown/ai-aas
    path: services/user-org-service/configs/k8s
    targetRevision: main
  destination:
    server: https://kubernetes.default.svc
    namespace: development
  syncPolicy:
    automated:
      prune: true
      selfHeal: true
```

## Best Practices

1. **Tag Images** - Use commit SHA or version tags instead of `latest` for reproducibility
2. **Clean Up** - Jobs auto-delete after 1 hour, but manually clean up if needed
3. **Monitor Resources** - Watch for resource exhaustion if running multiple test jobs
4. **Parallel Execution** - Use unique job names (timestamp-based) to run multiple tests
5. **Log Retention** - Consider exporting logs to a logging system for long-term storage

