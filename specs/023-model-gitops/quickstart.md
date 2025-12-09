# Quickstart: Deploying a Model with GitOps

This guide walks through the process of deploying a new AI model using the GitOps workflow.

## Prerequisites

- `kubectl` configured to access the target Kubernetes cluster.
- An account in the Git repository used for GitOps.

## Steps

### 1. Create a HuggingFace Token Secret (if needed)

If you are deploying a private model from HuggingFace, you must first create a Kubernetes secret containing your access token.

```bash
kubectl create secret generic huggingface-token --from-literal=token=<YOUR_HF_TOKEN>
```

### 2. Define the `AIModel` Resource

Create a YAML file for your model. For example, to deploy `Mistral-7B-Instruct`, create a file named `mistral-7b.yaml`:

```yaml
apiVersion: models.ai-aas.com/v1alpha1
kind: AIModel
metadata:
  name: mistral-7b-instruct
spec:
  source: "mistralai/Mistral-7B-Instruct-v0.1"
  # hfTokenSecretName: "huggingface-token" # Uncomment for private models
  serving:
    engine: "vllm"
    minReplicas: 1
  hardware:
    nodeSelector:
      "accelerator": "nvidia-l4"
```

### 3. Commit to the GitOps Repository

Add this file to the appropriate directory in your GitOps repository. For the development environment, this would be:

```
gitops/clusters/development/apps/ai-models/mistral-7b.yaml
```

Commit the file and open a Pull Request.

### 4. Wait for ArgoCD to Sync

Once the Pull Request is merged, ArgoCD will automatically detect the change and apply the `AIModel` resource to the cluster.

### 5. Monitor the Deployment

You can monitor the status of the deployment by checking the `AIModel` resource:

```bash
# Watch the status
kubectl get aimodel mistral-7b-instruct -w

# Get detailed information
kubectl describe aimodel mistral-7b-instruct
```

The `phase` field in the status will show the progress (`Downloading`, `Ready`, `Failed`). Once the phase is `Ready`, the `inferenceEndpoint` field will be populated with the URL you can use to send requests to the model.
