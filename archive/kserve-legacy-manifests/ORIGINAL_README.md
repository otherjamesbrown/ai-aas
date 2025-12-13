# KServe Model Deployments

This directory contains InferenceService manifests for deploying models via KServe.

## Deployed Models

- `llama-2-7b.yaml` - Llama 2 7B model (pilot deployment)

## Adding a New Model

1. Copy the template from `../templates/inference-service-vllm-template.yaml`
2. Replace all placeholders with actual values:
   - `{{MODEL_NAME}}` - Unique model identifier
   - `{{STORAGE_URI}}` - Model location (hf:// or s3://)
   - `{{MIN_REPLICAS}}` / `{{MAX_REPLICAS}}` - Scaling bounds
   - `{{CPU_REQUEST}}` / `{{MEMORY_REQUEST}}` / `{{GPU_REQUEST}}` - Resources
   - `{{VLLM_VERSION}}` - vLLM container version
3. Save to this directory with a descriptive name
4. Deploy via GitOps or manually:
   ```bash
   kubectl apply -f <model-name>.yaml
   ```

## Resource Guidelines

### 7B Models (Llama-2-7b, Mistral-7b)
```yaml
resources:
  requests:
    cpu: "4"
    memory: "16Gi"
    nvidia.com/gpu: 1
  limits:
    cpu: "8"
    memory: "32Gi"
    nvidia.com/gpu: 1
```

### 13B Models
```yaml
resources:
  requests:
    cpu: "8"
    memory: "32Gi"
    nvidia.com/gpu: 1
  limits:
    cpu: "16"
    memory: "64Gi"
    nvidia.com/gpu: 1
```

### 70B+ Models
```yaml
resources:
  requests:
    cpu: "16"
    memory: "128Gi"
    nvidia.com/gpu: 4
  limits:
    cpu: "32"
    memory: "256Gi"
    nvidia.com/gpu: 4
```

## Monitoring Deployments

```bash
# Check InferenceService status
kubectl get inferenceservice -n development

# Watch for ready status
kubectl get inferenceservice llama-2-7b -n development -w

# Check predictor pods
kubectl get pods -n development -l serving.kserve.io/inferenceservice=llama-2-7b

# View logs
kubectl logs -n development -l serving.kserve.io/inferenceservice=llama-2-7b -c kserve-container

# Describe InferenceService for troubleshooting
kubectl describe inferenceservice llama-2-7b -n development
```

## Testing Inference

```bash
# Port-forward to predictor service
kubectl port-forward -n development svc/llama-2-7b-predictor 8080:80

# Send test request (KServe V2 protocol)
curl -X POST http://localhost:8080/v2/models/llama-2-7b/infer \
  -H "Content-Type: application/json" \
  -d '{
    "id": "test-123",
    "inputs": [{
      "name": "prompt",
      "shape": [1],
      "datatype": "BYTES",
      "data": ["Tell me a joke"]
    }],
    "parameters": {
      "max_tokens": 50,
      "temperature": 0.7
    }
  }'
```

## Scaling Configuration

### Development (Variable Traffic)
- `minReplicas: 0` - Scale to zero when idle
- `maxReplicas: 3` - Limited scale-up
- `scaleTarget: 5` - 5 concurrent requests per pod

### Production (High Availability)
- `minReplicas: 2` - Always keep 2 replicas warm
- `maxReplicas: 10` - Scale up to 10 replicas
- `scaleTarget: 5` - 5 concurrent requests per pod
- `scaleDownDelay: 10m` - Wait 10 minutes before scaling down
