# KServe Infrastructure

This directory contains KServe infrastructure configuration for the AI-AAS platform.

## Contents

- `inferenceservice-config.yaml` - KServe controller configuration
- `cluster-storage-container-hf.yaml` - Hugging Face model storage backend
- `cluster-storage-container-s3.yaml` - S3-compatible storage backend
- `secret-templates.yaml` - Templates for storage credentials
- `gpu-node-setup.md` - GPU node configuration guide
- `test-inferenceservice.yaml` - Test InferenceService for validation

## Deployment

These resources are deployed via ArgoCD using the `kserve-config-development` application.

```bash
# Apply manually (not recommended, use GitOps instead)
kubectl apply -f inferenceservice-config.yaml
kubectl apply -f cluster-storage-container-hf.yaml
kubectl apply -f cluster-storage-container-s3.yaml
```

## Creating Secrets

Before deploying InferenceServices, create the required secrets:

```bash
# Hugging Face token (for private models)
kubectl create secret generic huggingface-secret \
  --from-literal=token=$HF_TOKEN \
  -n development

# S3 credentials (if using private S3 storage)
kubectl create secret generic s3-credentials \
  --from-literal=access_key=$AWS_ACCESS_KEY_ID \
  --from-literal=secret_key=$AWS_SECRET_ACCESS_KEY \
  -n development
```

## Testing the Infrastructure

Deploy the test InferenceService:

```bash
# Apply the test InferenceService
kubectl apply -f test-inferenceservice.yaml

# Wait for ready status (may take 5-10 minutes for model download)
kubectl wait --for=condition=Ready inferenceservice/test-distilgpt2 -n development --timeout=15m

# Check status
kubectl get inferenceservice test-distilgpt2 -n development

# Check pods
kubectl get pods -n development -l serving.kserve.io/inferenceservice=test-distilgpt2

# Port-forward to test inference
kubectl port-forward -n development svc/test-distilgpt2-predictor 8080:80

# Send test request
curl -X POST http://localhost:8080/v2/models/test-distilgpt2/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-123",
    "inputs": [{
      "name": "input",
      "shape": [1],
      "datatype": "BYTES",
      "data": ["Hello world"]
    }]
  }'
```

## Troubleshooting

### InferenceService not becoming Ready

```bash
# Check InferenceService status
kubectl describe inferenceservice test-distilgpt2 -n development

# Check controller logs
kubectl logs -n kserve -l control-plane=kserve-controller-manager

# Check predictor pod logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=test-distilgpt2 -c kserve-container
```

### Storage initializer failures

```bash
# Check storage initializer logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=test-distilgpt2 -c storage-initializer
```

## References

- [KServe Documentation](https://kserve.github.io/website/)
- [Knative Serving](https://knative.dev/docs/serving/)
- [Istio Documentation](https://istio.io/latest/docs/)
