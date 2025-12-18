# TensorRT-LLM End-to-End Testing Guide

**Last Updated**: 2025-12-15
**Owner**: AI Platform Engineering
**Related Spec**: [specs/029-triton-tensorrt-llm](../../specs/029-triton-tensorrt-llm/)
**Related Runbook**: [build-tensorrt-llm-engine.md](../runbooks/build-tensorrt-llm-engine.md)

## Overview

This guide documents the end-to-end testing procedure for TensorRT-LLM/Triton model deployments on the ai-aas platform. It covers all steps from deploying the ClusterServingRuntime to verifying inference functionality.

## Prerequisites

### Platform Requirements

- **Kubernetes Cluster**: Development or staging environment with GPU nodes
- **KServe**: v0.11+ installed with Knative Serving
- **GPU Nodes**: NVIDIA GPU with Compute Capability 8.0+ (A100, H100, L40S, RTX 4090)
- **ArgoCD**: For GitOps-based deployment
- **kubectl**: Configured with access to target cluster

### Model Requirements

- **TensorRT-LLM Engine**: Built and uploaded to S3 (see [build-tensorrt-llm-engine.md](../runbooks/build-tensorrt-llm-engine.md))
- **Model Repository**: Complete Triton model repository in S3 with ensemble, preprocessing, postprocessing, and tensorrt_llm models
- **ModelRecipe**: Created in `infra/model-recipes/` (e.g., `llama-3.1-8b-instruct-trtllm.yaml`)

### Access Requirements

- **kubectl**: Access to development/staging cluster
- **ArgoCD**: Admin access for syncing applications
- **API Access**: Valid API key for testing inference
- **Monitoring**: Access to Grafana for observability

## Test Procedure

### Step 1: Deploy ClusterServingRuntime

The TensorRT-LLM ClusterServingRuntime defines how Triton containers are deployed for tensorrt-llm models.

#### 1.1 Verify ClusterServingRuntime Manifest

```bash
# Check the ClusterServingRuntime file exists
cat /home/dev/worktrees/029-triton/infra/k8s/kserve/base/cluster-serving-runtime-tensorrt-llm.yaml
```

**Expected content**:
- `name: tensorrt-llm`
- `supportedModelFormats: name: tensorrt-llm`
- Container image: `nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3`
- Health probes configured for `/v2/health/live` and `/v2/health/ready`

#### 1.2 Apply via GitOps (Recommended)

```bash
# Verify file is committed to git
git status

# If changes not committed, commit them
git add infra/k8s/kserve/base/cluster-serving-runtime-tensorrt-llm.yaml
git commit -m "feat(kserve): add TensorRT-LLM ClusterServingRuntime"
git push origin develop

# Sync via ArgoCD (development environment)
argocd app sync kserve-base-development

# Verify sync status
argocd app get kserve-base-development
```

#### 1.3 Alternative: Apply Directly (Testing Only)

```bash
# For quick testing, apply directly (not recommended for production)
kubectl apply -f /home/dev/worktrees/029-triton/infra/k8s/kserve/base/cluster-serving-runtime-tensorrt-llm.yaml

# Verify ClusterServingRuntime was created
kubectl get clusterservingruntimes tensorrt-llm -o yaml
```

**Expected output**:
```yaml
apiVersion: serving.kserve.io/v1alpha1
kind: ClusterServingRuntime
metadata:
  name: tensorrt-llm
spec:
  supportedModelFormats:
    - name: tensorrt-llm
      version: "1"
  containers:
    - name: kserve-container
      image: nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3
      ...
```

### Step 2: Deploy PodMonitor (Optional)

The PodMonitor enables Prometheus scraping of Triton metrics.

```bash
# Apply PodMonitor via GitOps
git add infra/k8s/kserve/monitoring/podmonitor-tensorrt-llm.yaml
git commit -m "feat(monitoring): add PodMonitor for TensorRT-LLM"
git push origin develop

# Sync ArgoCD application
argocd app sync monitoring-development

# Verify PodMonitor
kubectl get podmonitor tensorrt-llm-metrics -n system -o yaml
```

### Step 3: Deploy ModelRecipe

ModelRecipes define baseline configurations for specific models.

#### 3.1 Verify ModelRecipe

```bash
# Check the recipe file
cat /home/dev/worktrees/029-triton/infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml
```

**Key fields to verify**:
- `runtime: tensorrt-llm`
- `modelID: meta-llama/Llama-3.1-8B-Instruct`
- `resources.gpu.count: 1`
- `resources.gpu.vendor: nvidia`
- `healthCheck` section with appropriate probe timeouts

#### 3.2 Apply ModelRecipe

```bash
# Apply via kubectl (recipes are cluster-scoped)
kubectl apply -f /home/dev/worktrees/029-triton/infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml

# Verify recipe was created
kubectl get modelrecipe llama-3.1-8b-instruct-trtllm -n ai-model-system -o yaml

# Validate recipe using CLI
ai-aas-cli recipe validate llama-3.1-8b-instruct-trtllm
```

**Expected output**:
```
✓ Recipe llama-3.1-8b-instruct-trtllm is valid
Runtime: tensorrt-llm
GPU Requirements: 1x nvidia GPU
```

### Step 4: Create AIModel

The AIModel custom resource triggers the ai-model-operator to deploy the model.

#### 4.1 Create AIModel Manifest

Create a test AIModel manifest:

```yaml
# test-aimodel-tensorrt-llm.yaml
apiVersion: ai.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3.1-8b-instruct-trtllm-test
  namespace: ai-model-system
spec:
  recipeRef:
    name: llama-3.1-8b-instruct-trtllm
    namespace: ai-model-system
  modelName: llama-3.1-8b-instruct-trtllm-test
  modelID: meta-llama/Llama-3.1-8B-Instruct
  s3Bucket: ai-aas-models
  s3Key: triton/llama-3.1-8b-instruct-trtllm
  enabled: true
  environment: development
```

#### 4.2 Apply AIModel

```bash
# Apply the AIModel
kubectl apply -f test-aimodel-tensorrt-llm.yaml

# Watch the AIModel status
kubectl get aimodel llama-3.1-8b-instruct-trtllm-test -n ai-model-system -w

# Check detailed status
kubectl describe aimodel llama-3.1-8b-instruct-trtllm-test -n ai-model-system
```

**Expected phases**:
1. `Pending` - Initial state
2. `Deploying` - InferenceService created, waiting for pods
3. `Ready` - Model loaded and ready to serve

**Note**: The `Downloading` phase is skipped for TensorRT-LLM models since the engine is pre-built and stored in S3.

### Step 5: Verify InferenceService Creation

The ai-model-operator creates a KServe InferenceService based on the AIModel spec.

#### 5.1 Check InferenceService

```bash
# List InferenceServices
kubectl get inferenceservices -n system

# Get specific InferenceService
kubectl get inferenceservice llama-3.1-8b-instruct-trtllm-test -n system -o yaml

# Check InferenceService status
kubectl get inferenceservice llama-3.1-8b-instruct-trtllm-test -n system -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
```

**Expected status**:
- Ready condition: `True`
- URL populated: `http://llama-3.1-8b-instruct-trtllm-test.system.svc.cluster.local`

#### 5.2 Verify Runtime Configuration

```bash
# Check that runtime is tensorrt-llm
kubectl get inferenceservice llama-3.1-8b-instruct-trtllm-test -n system -o jsonpath='{.spec.predictor.model.runtime}'

# Expected output: tensorrt-llm
```

### Step 6: Verify Health Probes

Health probes ensure the Triton server is responsive before marking the pod as ready.

#### 6.1 Check Pod Health

```bash
# Get pod for the InferenceService
kubectl get pods -n system -l serving.kserve.io/inferenceservice=llama-3.1-8b-instruct-trtllm-test

# Check pod conditions
kubectl describe pod <pod-name> -n system | grep -A 10 "Conditions:"
```

**Expected conditions**:
- `Initialized: True`
- `Ready: True`
- `ContainersReady: True`
- `PodScheduled: True`

#### 6.2 Verify Probe Configuration

```bash
# Check liveness probe
kubectl get pod <pod-name> -n system -o jsonpath='{.spec.containers[0].livenessProbe}' | jq

# Check readiness probe
kubectl get pod <pod-name> -n system -o jsonpath='{.spec.containers[0].readinessProbe}' | jq
```

**Expected probe configuration**:
```json
{
  "httpGet": {
    "path": "/v2/health/live",
    "port": 8000,
    "scheme": "HTTP"
  },
  "initialDelaySeconds": 60,
  "periodSeconds": 10,
  "timeoutSeconds": 5,
  "failureThreshold": 3
}
```

#### 6.3 Test Probes Directly

```bash
# Port-forward to pod
kubectl port-forward <pod-name> -n system 8000:8000

# In another terminal, test liveness probe
curl http://localhost:8000/v2/health/live
# Expected: 200 OK

# Test readiness probe
curl http://localhost:8000/v2/health/ready
# Expected: 200 OK

# Check model status
curl http://localhost:8000/v2/models/ensemble/ready
# Expected: 200 OK (model loaded and ready)
```

### Step 7: Test Inference

Verify the model can handle inference requests.

#### 7.1 Test via Triton HTTP API (Direct)

```bash
# Port-forward to the InferenceService pod
kubectl port-forward <pod-name> -n system 8000:8000

# Send inference request
curl -X POST http://localhost:8000/v2/models/ensemble/infer \
  -H "Content-Type: application/json" \
  -d '{
    "inputs": [
      {
        "name": "text_input",
        "shape": [1, 1],
        "datatype": "BYTES",
        "data": ["What is the capital of France?"]
      },
      {
        "name": "max_tokens",
        "shape": [1, 1],
        "datatype": "INT32",
        "data": [100]
      },
      {
        "name": "temperature",
        "shape": [1, 1],
        "datatype": "FP32",
        "data": [0.7]
      }
    ]
  }' | jq
```

**Expected response**:
```json
{
  "model_name": "ensemble",
  "model_version": "1",
  "outputs": [
    {
      "name": "text_output",
      "datatype": "BYTES",
      "shape": [1, 1],
      "data": ["The capital of France is Paris."]
    }
  ]
}
```

#### 7.2 Test via Platform API (End-to-End)

This requires the model to be registered in the Admin API and routed through api-router-service.

```bash
# Set API endpoint and key
export API_ENDPOINT="https://api.dev.otherjamesbrown.com"
export API_KEY="your-api-key-here"

# Send OpenAI-compatible request
curl -X POST ${API_ENDPOINT}/v1/chat/completions \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.1-8b-instruct-trtllm-test",
    "messages": [
      {"role": "user", "content": "What is the capital of France?"}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }' | jq
```

**Expected response**:
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "created": 1734307200,
  "model": "llama-3.1-8b-instruct-trtllm-test",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The capital of France is Paris."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 8,
    "total_tokens": 20
  }
}
```

### Step 8: Monitor and Validate

#### 8.1 Check Operator Logs

```bash
# Get ai-model-operator pod
kubectl get pods -n ai-model-system -l app=ai-model-operator

# Check logs for reconciliation
kubectl logs -n ai-model-system <operator-pod-name> --tail=100 | grep llama-3.1-8b-instruct-trtllm-test
```

**Look for**:
- `Successfully created InferenceService`
- `Updated AIModel status to Ready`
- No error messages

#### 8.2 Check Metrics (Grafana)

```bash
# Access Grafana
open https://grafana.dev.otherjamesbrown.com

# Navigate to:
# - "Service Logs" dashboard → Filter by service="system", model="llama-3.1-8b-instruct-trtllm-test"
# - "Request Tracing" dashboard → Check latency and error rates
```

**Metrics to verify**:
- Model loaded successfully (check Triton logs)
- No error spikes in Loki
- Request latency reasonable (<2s for first token)

#### 8.3 Check Prometheus Metrics (Optional)

If PodMonitor was deployed:

```bash
# Port-forward to Prometheus
kubectl port-forward -n monitoring svc/prometheus-operated 9090:9090

# Query Triton metrics
# Open http://localhost:9090 and query:
# - nv_inference_request_success{model="ensemble"}
# - nv_inference_request_duration_us{model="ensemble"}
# - nv_gpu_utilization
```

### Step 9: Performance Validation

#### 9.1 Latency Test

```bash
# Measure time-to-first-token
time curl -X POST http://localhost:8000/v2/models/ensemble/infer \
  -H "Content-Type: application/json" \
  -d '{...}'

# Expected: < 2 seconds for first token on A100
```

#### 9.2 Throughput Test

Use a load testing tool like `wrk` or `hey`:

```bash
# Install hey
go install github.com/rakyll/hey@latest

# Run load test (100 requests, 10 concurrent)
hey -n 100 -c 10 -m POST \
  -H "Content-Type: application/json" \
  -D request.json \
  http://localhost:8000/v2/models/ensemble/infer
```

**Expected results for Llama 3.1 8B on A100**:
- Throughput: 2000-3000 tokens/sec
- Average latency: 50-100ms per token
- 95th percentile: <200ms per token

### Step 10: Cleanup (Optional)

After testing, clean up resources:

```bash
# Delete AIModel
kubectl delete aimodel llama-3.1-8b-instruct-trtllm-test -n ai-model-system

# Verify InferenceService is deleted (via owner reference)
kubectl get inferenceservice llama-3.1-8b-instruct-trtllm-test -n system
# Expected: NotFound

# Delete ModelRecipe (if no longer needed)
kubectl delete modelrecipe llama-3.1-8b-instruct-trtllm -n ai-model-system

# Delete ClusterServingRuntime (only if removing TensorRT-LLM support)
kubectl delete clusterservingruntime tensorrt-llm
```

## Troubleshooting

### Issue: InferenceService Never Becomes Ready

**Symptoms**:
- AIModel stuck in `Deploying` phase
- InferenceService Ready condition is `False`

**Debugging steps**:

```bash
# Check InferenceService status
kubectl describe inferenceservice llama-3.1-8b-instruct-trtllm-test -n system

# Check pod logs
kubectl logs -n system <pod-name>

# Common issues:
# 1. Missing TensorRT engine file in S3
# 2. Incorrect S3 path in AIModel spec
# 3. GPU not available on node
# 4. Triton model configuration errors
```

**Solutions**:
- Verify S3 path: `aws s3 ls s3://ai-aas-models/triton/llama-3.1-8b-instruct-trtllm/ --recursive`
- Check GPU availability: `kubectl describe node <gpu-node-name> | grep nvidia.com/gpu`
- Review Triton logs for model loading errors

### Issue: Health Probes Failing

**Symptoms**:
- Pod restarts frequently
- Pod shows `CrashLoopBackOff` or `NotReady`

**Debugging steps**:

```bash
# Check probe configuration
kubectl get pod <pod-name> -n system -o jsonpath='{.spec.containers[0].livenessProbe}'

# Check pod events
kubectl describe pod <pod-name> -n system | grep -A 5 "Events:"

# Test probes manually
kubectl port-forward <pod-name> -n system 8000:8000
curl http://localhost:8000/v2/health/live
curl http://localhost:8000/v2/health/ready
```

**Solutions**:
- Increase `startupProbeSeconds` in ModelRecipe if model is large (>10B parameters)
- Verify Triton server is actually listening on port 8000
- Check that model repository is complete (ensemble, preprocessing, postprocessing, tensorrt_llm)

### Issue: Inference Returns Errors

**Symptoms**:
- HTTP 500 errors from Triton
- Empty or malformed responses

**Debugging steps**:

```bash
# Check Triton logs
kubectl logs -n system <pod-name> --tail=100

# Test Triton directly
curl http://localhost:8000/v2/models/ensemble
curl http://localhost:8000/v2/models/tensorrt_llm

# Verify model configuration
kubectl exec -n system <pod-name> -- cat /mnt/models/tensorrt_llm/config.pbtxt
```

**Solutions**:
- Verify preprocessing/postprocessing Python models have correct tokenizer paths
- Check that TensorRT engine was built for the correct GPU architecture
- Ensure batch size in request doesn't exceed `max_batch_size` in config

### Issue: Low Performance

**Symptoms**:
- High latency (>500ms per token)
- Low throughput (<500 tokens/sec on A100)

**Debugging steps**:

```bash
# Check GPU utilization
kubectl exec -n system <pod-name> -- nvidia-smi

# Check Triton metrics
curl http://localhost:8002/metrics | grep nv_gpu_utilization
curl http://localhost:8002/metrics | grep batch
```

**Solutions**:
- Verify GPU is actually being used (check `nv_gpu_utilization` metric)
- Tune dynamic batching parameters in Triton config (`preferred_batch_size`, `max_queue_delay_microseconds`)
- Check that model was built with optimizations (`--paged_kv_cache enable`, `--use_fused_mlp`, etc.)
- Verify TensorRT engine was built on the same GPU architecture (rebuild if migrating between GPU types)

## Validation Checklist

Before marking E2E testing as complete, verify:

- [ ] ClusterServingRuntime deployed and visible via `kubectl get clusterservingruntimes`
- [ ] PodMonitor deployed (if using Prometheus)
- [ ] ModelRecipe created and validates via `ai-aas-cli recipe validate`
- [ ] AIModel created and reaches `Ready` phase
- [ ] InferenceService created with runtime=`tensorrt-llm`
- [ ] Health probes configured and passing (`/v2/health/live`, `/v2/health/ready`)
- [ ] Direct inference works (Triton HTTP API)
- [ ] End-to-end inference works (Platform API)
- [ ] No errors in operator logs
- [ ] No errors in Triton logs
- [ ] Metrics visible in Grafana (if monitoring enabled)
- [ ] Performance meets expectations (latency, throughput)

## References

- [Build TensorRT-LLM Engine Runbook](../runbooks/build-tensorrt-llm-engine.md)
- [Spec 029: TensorRT-LLM/Triton Support](../../specs/029-triton-tensorrt-llm/spec.md)
- [AI Model Operator Guide](../operators/ai-model-operator-guide.md)
- [KServe Documentation](https://kserve.github.io/website/)
- [Triton Inference Server Docs](https://github.com/triton-inference-server/server)
- [TensorRT-LLM Backend](https://github.com/triton-inference-server/tensorrtllm_backend)

## Appendix: Quick Test Script

```bash
#!/bin/bash
# quick-tensorrt-llm-test.sh
# Quick validation script for TensorRT-LLM deployment

set -e

MODEL_NAME="llama-3.1-8b-instruct-trtllm-test"
NAMESPACE="system"

echo "=== TensorRT-LLM E2E Test ==="
echo ""

echo "[1/5] Checking ClusterServingRuntime..."
kubectl get clusterservingruntime tensorrt-llm &>/dev/null && echo "✓ ClusterServingRuntime exists" || echo "✗ ClusterServingRuntime missing"

echo "[2/5] Checking ModelRecipe..."
kubectl get modelrecipe llama-3.1-8b-instruct-trtllm -n ai-model-system &>/dev/null && echo "✓ ModelRecipe exists" || echo "✗ ModelRecipe missing"

echo "[3/5] Checking AIModel status..."
STATUS=$(kubectl get aimodel $MODEL_NAME -n ai-model-system -o jsonpath='{.status.phase}' 2>/dev/null || echo "NotFound")
echo "  Status: $STATUS"
if [ "$STATUS" = "Ready" ]; then
  echo "✓ AIModel is Ready"
else
  echo "✗ AIModel is not Ready (current: $STATUS)"
fi

echo "[4/5] Checking InferenceService..."
kubectl get inferenceservice $MODEL_NAME -n $NAMESPACE &>/dev/null && echo "✓ InferenceService exists" || echo "✗ InferenceService missing"

echo "[5/5] Checking Pod health..."
POD=$(kubectl get pod -n $NAMESPACE -l serving.kserve.io/inferenceservice=$MODEL_NAME -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD" ]; then
  READY=$(kubectl get pod $POD -n $NAMESPACE -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')
  if [ "$READY" = "True" ]; then
    echo "✓ Pod is Ready"
  else
    echo "✗ Pod is not Ready"
  fi
else
  echo "✗ No pod found"
fi

echo ""
echo "=== Test Complete ==="
```

**Usage**:
```bash
chmod +x quick-tensorrt-llm-test.sh
./quick-tensorrt-llm-test.sh
```
