# Quick Start: Deploying Mistral-7B with AIModel CR

## Prerequisites

1. AI Model Operator is deployed in the cluster
2. KServe is installed and configured
3. GPU nodes are available and labeled
4. You have cluster admin access or appropriate RBAC

## Deploy Mistral-7B (Staging)

### Step 1: Apply the AIModel CR

```bash
kubectl apply -f mistral-7b-instruct-staging.yaml
```

Expected output:
```
aimodel.ai.ai-aas.io/mistral-7b-instruct created
```

### Step 2: Watch AIModel Status

```bash
kubectl get aimodel mistral-7b-instruct -n staging -w
```

You should see the Phase progress through:
1. `Pending` - AIModel created, operator starting work
2. `Deploying` - InferenceService being created
3. `Ready` - Model ready to serve requests

Press `Ctrl+C` to stop watching.

### Step 3: Verify InferenceService Created

```bash
kubectl get inferenceservice mistral-7b-instruct -n staging
```

Expected output:
```
NAME                   URL                                                   READY   PREV   LATEST   PREVROLLEDOUTREVISION   LATESTREADYREVISION   AGE
mistral-7b-instruct    http://mistral-7b-instruct.staging.svc.cluster.local  True    0      100                              mistral-7b-instruct-00001   2m
```

### Step 4: Check Pods

```bash
kubectl get pods -n staging -l serving.knative.dev/service=mistral-7b-instruct
```

The pod may take 5-6 minutes to become Ready (model loads into GPU memory).

### Step 5: Get Inference Endpoint

```bash
kubectl get aimodel mistral-7b-instruct -n staging -o jsonpath='{.status.inferenceEndpoint}' && echo
```

Example output:
```
http://mistral-7b-instruct.staging.svc.cluster.local
```

### Step 6: Test Inference

From inside the cluster (or via port-forward):

```bash
# Port-forward to the InferenceService
kubectl port-forward -n staging svc/mistral-7b-instruct 8000:80 &

# Test inference
curl -X POST http://localhost:8000/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "mistral-7b-instruct",
    "messages": [
      {"role": "system", "content": "You are a helpful assistant."},
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

Expected response (excerpt):
```json
{
  "id": "cmpl-...",
  "object": "chat.completion",
  "created": 1702400000,
  "model": "mistral-7b-instruct",
  "choices": [
    {
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ]
}
```

## Troubleshooting

### AIModel Stuck in Pending

```bash
kubectl describe aimodel mistral-7b-instruct -n staging
```

Check for events indicating why the operator isn't progressing.

### InferenceService Not Created

```bash
# Check operator logs
kubectl logs -n ai-model-operator-system deployment/ai-model-operator-controller-manager -f
```

Look for errors related to InferenceService creation.

### Pod Not Starting

```bash
# Check pod status
kubectl get pods -n staging -l serving.knative.dev/service=mistral-7b-instruct

# Describe pod
kubectl describe pod -n staging <pod-name>

# Check pod logs
kubectl logs -n staging <pod-name> -c kserve-container
```

Common issues:
- GPU not available (check `nvidia.com/gpu` resource)
- Node tolerations not matching (check GPU node taints)
- Image pull errors (check network/registry access)

### Model Loading Takes Too Long

The Mistral-7B model takes 5-6 minutes to load. Check:

```bash
# Check startup probe status
kubectl describe pod -n staging <pod-name>

# Check vLLM container logs
kubectl logs -n staging <pod-name> -c kserve-container
```

Look for logs indicating model loading progress:
```
INFO:     Loading model weights from mistralai/Mistral-7B-Instruct-v0.2
INFO:     Loading weights...
INFO:     Model loaded successfully
```

### Inference Request Fails

```bash
# Check if InferenceService is Ready
kubectl get inferenceservice mistral-7b-instruct -n staging

# Check Knative Revision
kubectl get revision -n staging -l serving.knative.dev/service=mistral-7b-instruct

# Check pod logs for errors
kubectl logs -n staging <pod-name> -c kserve-container --tail=50
```

## Cleanup

To delete the model deployment:

```bash
kubectl delete aimodel mistral-7b-instruct -n staging
```

This will delete:
- AIModel CR
- InferenceService (owned by AIModel)
- Knative Service (owned by InferenceService)
- Pods (owned by Knative Service)

## Next Steps

- Review [README.md](README.md) for more AIModel examples
- Read [MIGRATION-NOTES.md](MIGRATION-NOTES.md) for detailed migration guidance
- See [mistral-comparison.md](mistral-comparison.md) for configuration equivalence details
