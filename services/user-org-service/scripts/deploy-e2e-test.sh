#!/usr/bin/env bash
# Deploy e2e-test to development environment and run it.
#
# Purpose:
#   This script builds the e2e-test Docker image, pushes it to the container
#   registry, and creates a Kubernetes Job to run the tests against the
#   deployed user-org-service in the development environment.
#
# Usage:
#   ./scripts/deploy-e2e-test.sh [--namespace NAMESPACE] [--registry REGISTRY] [--tag TAG]
#
# Environment Variables:
#   - REGISTRY: Container registry (default: ghcr.io/otherjamesbrown)
#   - IMAGE_TAG: Image tag (default: latest)
#   - KUBECTL_CONTEXT: Kubernetes context (default: dev-platform)
#   - NAMESPACE: Kubernetes namespace (default: development)
#   - SERVICE_NAME: Service name (default: user-org-service)
#
# Requirements:
#   - Docker must be running
#   - kubectl must be configured with access to dev cluster
#   - Container registry credentials configured (docker login)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default values
REGISTRY="${REGISTRY:-ghcr.io/otherjamesbrown}"
IMAGE_TAG="${IMAGE_TAG:-latest}"
KUBECTL_CONTEXT="${KUBECTL_CONTEXT:-dev-platform}"
# Default namespace matches kustomize base (user-org-service) or can be overridden
NAMESPACE="${NAMESPACE:-user-org-service}"
SERVICE_NAME="${SERVICE_NAME:-user-org-service}"
IMAGE_NAME="${REGISTRY}/${SERVICE_NAME}-e2e-test"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --namespace)
      NAMESPACE="$2"
      shift 2
      ;;
    --registry)
      REGISTRY="$2"
      shift 2
      ;;
    --tag)
      IMAGE_TAG="$2"
      shift 2
      ;;
    --context)
      KUBECTL_CONTEXT="$2"
      shift 2
      ;;
    --help)
      echo "Usage: $0 [--namespace NAMESPACE] [--registry REGISTRY] [--tag TAG] [--context CONTEXT]"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
TIMESTAMP=$(date +%s)
JOB_NAME="e2e-test-${TIMESTAMP}"

echo "=========================================="
echo "Deploying e2e-test to development"
echo "=========================================="
echo "Registry:      ${REGISTRY}"
echo "Image:         ${FULL_IMAGE}"
echo "Namespace:     ${NAMESPACE}"
echo "Context:       ${KUBECTL_CONTEXT}"
echo "Service:       ${SERVICE_NAME}"
echo "Job Name:      ${JOB_NAME}"
echo "=========================================="
echo ""

# Step 1: Build Docker image
echo "[1/4] Building Docker image..."
cd "$SERVICE_ROOT"
docker build -f Dockerfile.e2e-test -t "${FULL_IMAGE}" .

# Step 2: Push image to registry
echo ""
echo "[2/4] Pushing image to registry..."
docker push "${FULL_IMAGE}"

# Step 3: Create Kubernetes Job manifest
echo ""
echo "[3/4] Creating Kubernetes Job manifest..."
JOB_MANIFEST=$(cat <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: ${JOB_NAME}
  namespace: ${NAMESPACE}
  labels:
    app: ${SERVICE_NAME}
    component: e2e-test
spec:
  ttlSecondsAfterFinished: 3600
  backoffLimit: 2
  template:
    metadata:
      labels:
        app: ${SERVICE_NAME}
        component: e2e-test
    spec:
      restartPolicy: Never
      containers:
      - name: e2e-test
        image: ${FULL_IMAGE}
        imagePullPolicy: Always
        env:
        - name: API_URL
          value: "http://${SERVICE_NAME}.${NAMESPACE}.svc.cluster.local:8081"
        - name: TIMEOUT
          value: "300"
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "256Mi"
            cpu: "500m"
EOF
)

# Step 4: Apply Job and wait for completion
echo ""
echo "[4/4] Deploying Job to Kubernetes..."
echo "$JOB_MANIFEST" | kubectl --context="${KUBECTL_CONTEXT}" apply -f -

echo ""
echo "Job created: ${JOB_NAME}"
echo ""
echo "To watch the job:"
echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} get job ${JOB_NAME} -w"
echo ""
echo "To view logs:"
echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} logs -f job/${JOB_NAME}"
echo ""
echo "Waiting for job to start..."
sleep 3

# Wait for pod to be created
POD_NAME=""
for i in {1..30}; do
  POD_NAME=$(kubectl --context="${KUBECTL_CONTEXT}" -n "${NAMESPACE}" get pods -l job-name="${JOB_NAME}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")
  if [ -n "$POD_NAME" ]; then
    break
  fi
  sleep 1
done

if [ -z "$POD_NAME" ]; then
  echo "Warning: Pod not found. Check job status:"
  kubectl --context="${KUBECTL_CONTEXT}" -n "${NAMESPACE}" describe job "${JOB_NAME}"
  exit 1
fi

echo "Pod created: ${POD_NAME}"
echo ""
echo "Streaming logs (Ctrl+C to stop, job will continue running)..."
echo "=========================================="
kubectl --context="${KUBECTL_CONTEXT}" -n "${NAMESPACE}" logs -f "job/${JOB_NAME}" || true

echo ""
echo "=========================================="
echo "Checking job status..."
JOB_STATUS=$(kubectl --context="${KUBECTL_CONTEXT}" -n "${NAMESPACE}" get job "${JOB_NAME}" -o jsonpath='{.status.conditions[?(@.type=="Complete")].status}' 2>/dev/null || echo "Unknown")
JOB_FAILED=$(kubectl --context="${KUBECTL_CONTEXT}" -n "${NAMESPACE}" get job "${JOB_NAME}" -o jsonpath='{.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null || echo "Unknown")

if [ "$JOB_STATUS" = "True" ]; then
  echo "✅ Job completed successfully!"
  echo ""
  echo "To view full logs:"
  echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} logs job/${JOB_NAME}"
  echo ""
  echo "To clean up:"
  echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} delete job ${JOB_NAME}"
  exit 0
elif [ "$JOB_FAILED" = "True" ]; then
  echo "❌ Job failed!"
  echo ""
  echo "View logs:"
  echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} logs job/${JOB_NAME}"
  echo ""
  echo "View job details:"
  echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} describe job ${JOB_NAME}"
  exit 1
else
  echo "Job is still running or status unknown."
  echo ""
  echo "Check status:"
  echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} get job ${JOB_NAME}"
  echo ""
  echo "View logs:"
  echo "  kubectl --context=${KUBECTL_CONTEXT} -n ${NAMESPACE} logs -f job/${JOB_NAME}"
  exit 0
fi

