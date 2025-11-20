# Integration Tests - Hybrid Testing Strategy

This directory contains integration tests for the API Router Service using a **layered testing approach** to balance speed with confidence.

## Testing Philosophy

> **"Mocks give you speed, real backends give you confidence. You need both."**

We use a hybrid approach:
- **Mock tests**: Fast CI/CD feedback (milliseconds)
- **Real backend tests**: Catch actual integration issues (seconds)

## Test Types

### 1. Unit Tests with Mocks (Fast)

**File**: `openai_chat_test.go`
**Purpose**: Test API router logic, request validation, response formatting
**Speed**: ~20ms per test
**Runs**: Every PR, CI/CD pipeline

```bash
# Run mock tests only
go test -v ./test/integration -run TestOpenAIChatCompletions

# Run in short mode (CI/CD)
go test -short ./test/integration
```

**What they catch**:
- ✅ Request validation errors
- ✅ Response format issues
- ✅ Routing logic bugs
- ✅ Error handling

**What they DON'T catch**:
- ❌ Real model behavior
- ❌ Token counting errors
- ❌ Actual latency issues
- ❌ GPU OOM errors

### 2. E2E Tests with Real Backends (Confident)

**File**: `openai_chat_e2e_test.go`
**Purpose**: Test actual inference, catch real integration issues
**Speed**: ~2-10s per test
**Runs**: On-demand, smoke tests, nightly builds

```bash
# Set backend URL
export VLLM_BACKEND_URL=http://localhost:8000
export VLLM_MODEL_NAME=meta-llama/Llama-2-7b-chat-hf

# Run E2E tests
go test -v ./test/integration -run E2E
```

**What they catch**:
- ✅ Real model behavior issues
- ✅ Token counting accuracy
- ✅ Actual timeout problems
- ✅ GPU errors (OOM, crashes)
- ✅ Model-specific quirks
- ✅ Performance regressions

**What they require**:
- 🔧 vLLM backend running
- 🔧 GPU available
- 🔧 Model loaded
- 🔧 Network connectivity

## Running Tests

### Local Development (Fast Feedback)

```bash
# Run mock tests only (fastest)
go test -short -v ./test/integration

# Run specific mock test
go test -v ./test/integration -run TestOpenAIChatCompletions/^[^E2E]
```

### Before Committing (Quick Validation)

```bash
# Run all mock tests
go test -v ./test/integration -run "^TestOpenAI.*[^E2E]$"
```

### Before Deploying (Full Confidence)

```bash
# 1. Port-forward to vLLM in cluster
kubectl port-forward -n system svc/vllm-service 8000:8000 &

# 2. Run E2E tests
export VLLM_BACKEND_URL=http://localhost:8000
export VLLM_MODEL_NAME=your-model-name
go test -v ./test/integration -run E2E

# 3. Stop port-forward
pkill -f "port-forward.*vllm"
```

### CI/CD Pipeline

```yaml
# .github/workflows/test.yml
- name: Run Fast Tests
  run: go test -short -v ./test/integration

- name: Run E2E Tests (Nightly)
  if: github.event.schedule == '0 0 * * *'
  env:
    VLLM_BACKEND_URL: ${{ secrets.VLLM_URL }}
  run: go test -v ./test/integration -run E2E
```

## Critical Test Case: Capital of France

**Why this test matters**:

```go
Question: "In one word, can you provide me the capital of France?"
Expected: "Paris"
```

This simple test catches:
1. **Model quality**: Can it understand and follow instructions?
2. **Prompt engineering**: Does our prompt structure work?
3. **Response parsing**: Can we extract the answer?
4. **Consistency**: Does it give the same answer reliably?

If this test fails on a real backend:
- ❌ Model doesn't understand instructions
- ❌ Temperature too high (random answers)
- ❌ Max tokens too low (truncated answer)
- ❌ Model needs different prompting

## Test Results Interpretation

### Mock Test Pass ✅, E2E Test Pass ✅
**Status**: 🟢 All good! Safe to deploy.

### Mock Test Pass ✅, E2E Test Fail ❌
**Status**: 🔴 CRITICAL! Real integration issue detected.

**Common causes**:
- Model behavior different than mocked
- Token counting implementation wrong
- Timeout too short for real inference
- GPU errors not handled

**Action**: Fix the real integration issue before deploying!

### Mock Test Fail ❌
**Status**: 🔴 Logic error in API router.

**Action**: Fix the code, mocks caught a bug early!

## Performance Benchmarks

Expected latencies:

| Backend Type | Latency | Use Case |
|--------------|---------|----------|
| Mock | ~20ms | CI/CD, development |
| vLLM (7B) | 1-3s | Production |
| vLLM (13B) | 3-7s | Production |
| vLLM (70B) | 10-30s | Production |

If E2E tests show latency > 30s:
- ⚠️ GPU contention
- ⚠️ Model loading issues
- ⚠️ Network problems

## Troubleshooting

### E2E Tests Skipped

```bash
# Check if VLLM_BACKEND_URL is set
echo $VLLM_BACKEND_URL

# Check if vLLM is reachable
curl http://localhost:8000/health
```

### E2E Tests Fail with "Paris" Not Found

**Possible causes**:
1. Model doesn't understand English well
2. Need different prompt engineering
3. Temperature too high (try 0.1)
4. Max tokens too low (increase to 50)
5. Model needs fine-tuning

### E2E Tests Timeout

**Possible causes**:
1. GPU not available
2. Model not loaded
3. Backend overloaded
4. Network issues

**Fix**:
```bash
# Check GPU status
kubectl get nodes -l node-type=gpu
kubectl top nodes

# Check vLLM pod
kubectl get pods -n system | grep vllm
kubectl logs -n system <vllm-pod>
```

## Best Practices

1. **Run mocks locally**: Fast feedback during development
2. **Run E2E before merging**: Catch integration issues early
3. **Run E2E nightly**: Catch regressions
4. **Monitor latency**: Track performance over time
5. **Update mocks**: Keep them in sync with real behavior

## Adding New Tests

### Add Mock Test

```go
// openai_chat_test.go
func TestNewFeature(t *testing.T) {
    // Test with mockBackend
}
```

### Add E2E Test

```go
// openai_chat_e2e_test.go
func TestNewFeature_E2E(t *testing.T) {
    vllmURL := os.Getenv("VLLM_BACKEND_URL")
    if vllmURL == "" {
        t.Skip("Skipping E2E: VLLM_BACKEND_URL not set")
    }
    // Test with real backend
}
```

## Summary

| Aspect | Mock Tests | E2E Tests |
|--------|-----------|-----------|
| **Speed** | 20ms | 2-10s |
| **Confidence** | Medium | High |
| **Requirements** | None | vLLM + GPU |
| **CI/CD** | Always | Optional |
| **Cost** | Free | GPU time |
| **Failures** | Logic bugs | Real issues |

**Golden Rule**: Mock tests guard the gate. E2E tests guard production.
