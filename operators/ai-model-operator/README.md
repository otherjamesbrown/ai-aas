# AI Model Operator

## Introduction

The AI Model Operator is a Kubernetes operator designed to manage the lifecycle of AI models within a Kubernetes cluster. It enables GitOps-driven deployment and management of models by introducing a custom resource definition (CRD) called `AIModel`.

This operator automates the process of fetching model artifacts from HuggingFace (or other sources), caching them in S3-compatible object storage, and deploying them using vLLM inference servers as Kubernetes Deployments and Services.

## Features

*   **GitOps-driven Model Management:** Define your AI models declaratively using `AIModel` Custom Resources.
*   **Automated Model Artifact Caching:** Downloads model weights from HuggingFace and uploads them to S3-compatible storage via a Kubernetes Job.
*   **vLLM Inference Deployment:** Automatically creates and manages Kubernetes Deployments and Services for vLLM inference servers based on the `AIModel` specification.
*   **Status Tracking:** Provides real-time status updates on the model's phase (e.g., `Downloading`, `Ready`, `Failed`, `Disabled`) and its inference endpoint.
*   **Lifecycle Management:** Handles creation, updates, enabling/disabling, and deletion of model resources.
*   **Prometheus Metrics:** Exposes key operational metrics for observability.

## Prerequisites

*   A Kubernetes cluster (v1.28+)
*   `kubectl` installed and configured to connect to your cluster
*   `kustomize` (v5.0.0+) for applying manifests
*   `controller-gen` for generating CRDs and boilerplate code
*   Docker for building the operator and downloader images

## Installation

### 1. Build the Operator and Downloader Images

Ensure you have built the `model-downloader` image as per the instructions in `hack/model-downloader/Dockerfile`.

```bash
# From the project root directory
docker build -t ai-model-operator/model-downloader:latest -f operators/ai-model-operator/hack/model-downloader/Dockerfile .
# Build the operator image (replace with your desired image name and tag)
# TODO: Add specific build command for the operator image once Dockerfile is finalized
# Example:
# docker build -t your-repo/ai-model-operator:latest -f operators/ai-model-operator/Dockerfile .
```

### 2. Deploy CRDs

```bash
kubectl apply -k operators/ai-model-operator/config/crd
```

### 3. Deploy the Operator

First, modify `operators/ai-model-operator/config/manager/kustomization.yaml` to set the image name for the operator to your built image. Then deploy:

```bash
kustomize build operators/ai-model-operator/config/default | kubectl apply -f -
```

### 4. Create Necessary Secrets

The operator requires Kubernetes Secrets for S3 credentials and an optional HuggingFace token.

**S3 Credentials Secret (`ai-aas-s3-credentials`):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-aas-s3-credentials
  namespace: default # Adjust namespace if needed
type: Opaque
stringData:
  endpoint: "<your-s3-endpoint>" # e.g., https://s3.us-west-1.amazonaws.com
  bucket: "<your-s3-bucket-name>"
  accessKeyID: "<your-aws-access-key-id>"
  secretAccessKey: "<your-aws-secret-access-key>"
```

**HuggingFace Token Secret (Optional, `ai-aas-huggingface-token`):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: ai-aas-huggingface-token
  namespace: default # Adjust namespace if needed
type: Opaque
stringData:
  token: "<your-huggingface-read-token>"
```

Apply these secrets to your cluster:

```bash
kubectl apply -f s3-secret.yaml
kubectl apply -f hf-token-secret.yaml
```

## Usage

Create an `AIModel` Custom Resource to deploy your AI model:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: my-first-model
  namespace: default # Adjust namespace if needed
spec:
  modelID: "meta-llama/Llama-2-7b-hf" # The HuggingFace model ID
  revision: "main" # Optional, defaults to "main"
  image: "vllm/vllm-openai:latest" # The vLLM inference server image
  replicas: 1 # Optional, number of inference server replicas
  enabled: true # Set to false to scale down/delete deployment
```

Apply your `AIModel` resource:

```bash
kubectl apply -f my-first-model.yaml
```

### Monitoring Model Status

You can check the status of your `AIModel`:

```bash
kubectl get aimodel my-first-model -o yaml
```

The `status` field will show the `phase` (e.g., `Downloading`, `Ready`, `Failed`, `Disabled`) and `inferenceEndpoint`.

### Disabling/Enabling a Model

To disable a model (which will delete its associated Deployment and Service):

```bash
kubectl patch aimodel my-first-model --type=merge -p '{"spec":{"enabled":false}}'
```

To re-enable it:

```bash
kubectl patch aimodel my-first-model --type=merge -p '{"spec":{"enabled":true}}'
```

## Metrics

The AI Model Operator exposes Prometheus metrics on port `8080` at the `/metrics` endpoint. Key metrics include:

*   `aimodel_reconcile_total{result="success|error|skipped"}`: Total number of reconciliations by result.
*   `aimodel_active_count`: Number of active (enabled) `AIModel` resources.
*   `aimodel_status_phase{name="<model-name>",namespace="<namespace>",phase="<phase>"}`: Current phase of each `AIModel`.

## Development

Refer to the Go modules in `operators/ai-model-operator` for source code. Unit tests are located in `controllers/aimodel_controller_test.go`.

### Running Tests

```bash
go test ./...
```

### Generating Code and Manifests

```bash
go run sigs.k8s.io/controller-tools/cmd/controller-gen object:headerFile="./hack/boilerplate.go.txt" paths="./..."
go run sigs.k8s.io/controller-tools/cmd/controller-gen crd paths="./..." output:crd:dir=./config/crd/bases
```
