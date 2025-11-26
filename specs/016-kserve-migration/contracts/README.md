# KServe Migration Contracts

This directory contains reference YAML examples for the KServe migration.

## Files

- `inference-service-vllm.yaml` - Complete InferenceService example for vLLM models
- `cluster-storage-container-hf.yaml` - ClusterStorageContainer for Hugging Face models
- `cluster-storage-container-s3.yaml` - ClusterStorageContainer for S3 storage
- `serving-runtime-vllm.yaml` - Custom ServingRuntime for vLLM (optional)
- `api-router-backend-config.yaml` - API Router backend configuration for KServe

## Usage

These are reference examples. To deploy:

1. Copy and modify for your specific model
2. Update resource requests/limits based on model size
3. Configure autoscaling parameters for your workload
4. Deploy via GitOps (recommended) or `kubectl apply`

## Testing

To test an InferenceService:

```bash
# Apply the manifest
kubectl apply -f inference-service-vllm.yaml

# Wait for ready status
kubectl wait --for=condition=Ready inferenceservice/llama-2-7b -n development --timeout=15m

# Port-forward to the predictor
kubectl port-forward -n development svc/llama-2-7b-predictor 8080:80

# Send test request (KServe V2 protocol)
curl -X POST http://localhost:8080/v2/models/llama-2-7b/infer \
  -H "Content-Type: application/json" \
  -d @test-request.json
```

## References

- [KServe API Reference](https://kserve.github.io/website/latest/reference/api/)
- [vLLM Configuration](https://docs.vllm.ai/)
- [Knative Autoscaling](https://knative.dev/docs/serving/autoscaling/)
