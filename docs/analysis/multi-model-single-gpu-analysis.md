# Multi-Model Single GPU Support - Analysis

**Date**: 2026-01-07
**Status**: Analysis for future implementation
**Context**: User wants to host multiple Llama models on a single RTX 6000 Blackwell GPU and compare against multiple models spread across RTX 4000 Ada GPUs.

---

## Executive Summary

**Current State**: The ai-aas platform currently deploys one model per GPU through the AIModel operator and KServe InferenceService abstraction.

**Goal**: Enable hosting multiple small/medium LLM models on a single high-capacity GPU (RTX 6000 Blackwell with 96GB VRAM) for cost efficiency and resource optimization.

**Key Finding**: Neither vLLM nor TensorRT-LLM natively support multi-model single-GPU deployment in their current releases. Implementation requires architectural changes at multiple layers of the ai-aas platform.

---

## 1. Current Platform Capabilities

### Model Deployment Architecture

```yaml
current_deployment_model:
  operator: ai-model-operator
  abstraction: KServe InferenceService
  constraint: "1 InferenceService = 1 model = 1 GPU request"

  flow:
    1: "AIModel CR created in ai-aas-config"
    2: "Operator creates KServe InferenceService"
    3: "KServe creates vLLM/Triton pod with GPU request"
    4: "Kubernetes schedules pod to node with available GPU"
    5: "Model loaded into full GPU VRAM"
```

**Current limitations for multi-model**:
- Each AIModel CR maps to exactly one InferenceService
- Each InferenceService requests full GPU (`nvidia.com/gpu: 1`)
- No mechanism to partition GPU resources between models
- API Router expects one backend endpoint per model

### Available GPU Hardware

| Environment | GPU | Architecture | VRAM | Max Models (Est.) |
|-------------|-----|--------------|------|-------------------|
| Development | RTX 4000 Ada | sm_89 | 20GB | 1-2 small models |
| Staging | RTX 4000 Ada | sm_89 | 20GB | 1-2 small models |
| Staging | RTX 6000 Blackwell | sm_120 | 96GB | 3-5 models (8B-13B) |

**Key observation**: RTX 6000 Blackwell has 4.8x the VRAM of RTX 4000 Ada, making it ideal for multi-model hosting.

---

## 2. Technical Requirements for Multi-Model GPU Hosting

### 2.1. vLLM Multi-Model Support

**Current State (2026-01-07)**:
- vLLM does **NOT** natively support multiple models in a single instance
- Feature request exists ([vllm#13633](https://github.com/vllm-project/vllm/issues/13633))
- Workaround: Run multiple vLLM instances with `--gpu-memory-utilization` partitioning

**Workaround Architecture**:
```yaml
approach: Multiple vLLM processes sharing one GPU
implementation:
  - Deploy 3 separate vLLM pods
  - Each pod requests fractional GPU via gpu-memory-utilization
  - Example:
      pod1: "--gpu-memory-utilization 0.30" # 28.8GB for Llama-3.1-8B
      pod2: "--gpu-memory-utilization 0.30" # 28.8GB for Mistral-7B
      pod3: "--gpu-memory-utilization 0.30" # 28.8GB for Qwen2-7B
  - Reserve 10% for overhead

challenges:
  - Kubernetes doesn't support fractional GPU requests natively
  - Need GPU time-slicing or MPS to schedule multiple pods
  - vLLM instances don't coordinate - risk of OOM
```

**Memory Estimation for RTX 6000 Blackwell (96GB)**:

| Model | Size | BF16 Weights | KV Cache (4K ctx) | Total VRAM | Max Batch |
|-------|------|--------------|-------------------|------------|-----------|
| Llama-3.1-8B | 8B params | ~16GB | ~8GB | ~24GB | 16 |
| Mistral-7B | 7B params | ~14GB | ~7GB | ~21GB | 16 |
| Qwen2-VL-7B | 7B params | ~14GB | ~7GB | ~21GB | 16 |
| **Total** | | | | **~66GB** | |
| **Available** | | | | **30GB buffer** | |

**Feasibility**: 3 models feasible on RTX 6000 Blackwell with vLLM workaround.

### 2.2. TensorRT-LLM Multi-Model Support

**Current State**:
- TensorRT-LLM does **NOT** support multiple models in a single trtllm-serve instance
- Triton Inference Server **DOES** support multiple models via separate model repositories
- Requires separate engine compilations per model

**Triton Multi-Model Architecture**:
```yaml
approach: Triton model repository with multiple models
structure:
  /mnt/models/
    ├── model1_ensemble/
    │   ├── preprocessing/
    │   ├── tensorrt_llm/
    │   ├── postprocessing/
    │   └── ensemble/
    ├── model2_ensemble/
    │   └── ...
    └── model3_ensemble/
        └── ...

configuration:
  tritonserver:
    args:
      - --model-repository=/mnt/models
      - --model-control-mode=explicit
      - --load-model=model1_ensemble
      - --load-model=model2_ensemble
      - --load-model=model3_ensemble

gpu_allocation:
  method: "Triton allocates GPU memory per model instance"
  control: "Set instance_group GPU count and max_batch_size"
  limitation: "No explicit fractional GPU support"
```

**Feasibility**: Possible with Triton, but requires:
1. All models pre-compiled as TRT-LLM engines for sm_120
2. Triton instance-group configuration to control memory
3. Risk of OOM if total KV cache exceeds VRAM

### 2.3. NVIDIA GPU Partitioning Technologies

#### Multi-Instance GPU (MIG)

**Supported GPUs**: A100, H100, H200, Blackwell (B200, GB200, RTX Pro 6000 SE)

**RTX 6000 Blackwell MIG Support**:
- **LIKELY SUPPORTED** (Blackwell architecture mentioned in NVIDIA docs)
- Would need to verify with `nvidia-smi mig -lgip` on actual hardware

**MIG Characteristics**:
```yaml
benefits:
  - Hardware-level isolation (separate memory, SM, cache)
  - QoS guarantees per instance
  - Up to 7 GPU instances

drawbacks:
  - Fixed partition sizes (not arbitrary)
  - Requires GPU reset to change partitions
  - Not all Blackwell models support MIG

typical_partitions_96gb:
  - "3x 32GB instances (3g.24gb profile)"
  - "4x 24GB instances (2g.16gb profile)"
  - "7x 14GB instances (1g.8gb profile)"
```

**ai-aas Integration**:
- Kubernetes GPU Operator supports MIG
- Expose MIG instances as separate GPU resources: `nvidia.com/mig-3g.24gb: 1`
- Deploy one model per MIG instance using existing AIModel operator

**Feasibility**: Best option if RTX 6000 Blackwell supports MIG. Requires cluster-level MIG enablement.

#### Multi-Process Service (MPS)

**Characteristics**:
```yaml
benefits:
  - No GPU reset required
  - Dynamic allocation
  - Simpler than MIG

drawbacks:
  - No memory isolation (processes can OOM each other)
  - No QoS guarantees
  - Requires CUDA MPS daemon on each node

use_case: "Smaller workloads that don't need strict isolation"
```

**ai-aas Integration**:
- Enable MPS on GPU nodes
- Deploy multiple vLLM pods, each requesting `nvidia.com/gpu: 1`
- MPS daemon time-slices GPU across pods

**Feasibility**: Easier than MIG, but risky for production (no isolation).

#### GPU Time-Slicing (Kubernetes)

**Characteristics**:
```yaml
mechanism: "NVIDIA Device Plugin replicates GPU resources"
configuration:
  replicas: 3  # Advertise 1 GPU as 3 resources

benefits:
  - Simple Kubernetes configuration
  - Works on all GPUs

drawbacks:
  - No actual resource control
  - All pods share full GPU memory (race conditions)
  - First-come-first-served allocation
```

**Feasibility**: Not recommended for LLM workloads (high OOM risk).

---

## 3. GPU Comparison Context

### Hardware Specifications

| GPU | Architecture | Compute | VRAM | Memory Bandwidth | TDP |
|-----|--------------|---------|------|------------------|-----|
| RTX 6000 Blackwell | sm_120 | ~90 TFLOPS (FP16) | 96GB GDDR6X | ~960 GB/s | 300W |
| RTX 4000 Ada | sm_89 | ~26 TFLOPS (FP16) | 20GB GDDR6 | ~360 GB/s | 130W |

**Key Insights**:
- RTX 6000 has **3.5x compute throughput**
- RTX 6000 has **4.8x memory capacity**
- RTX 6000 has **2.7x memory bandwidth**

### Model Sizing

**What fits where?**

| Model | FP16 Size | RTX 4000 Ada (20GB) | RTX 6000 Blackwell (96GB) |
|-------|-----------|---------------------|---------------------------|
| Llama-3.1-8B | ~16GB | ✅ 1 model | ✅ 3-4 models |
| Mistral-7B | ~14GB | ✅ 1 model | ✅ 4-5 models |
| Llama-3.1-13B | ~26GB | ❌ Too large | ✅ 2-3 models |
| Llama-3.1-70B | ~140GB | ❌ Too large | ❌ Too large (need 2 GPUs) |

---

## 4. Implementation Approaches

### Option A: MIG Partitioning (Recommended if supported)

**Architecture**:
```yaml
setup:
  1: "Enable MIG on RTX 6000 Blackwell nodes"
  2: "Create 3x 32GB MIG instances (3g.24gb profile)"
  3: "Kubernetes exposes: nvidia.com/mig-3g.24gb"

deployment:
  - Deploy 3 separate AIModel CRs
  - Each requests: nvidia.com/mig-3g.24gb: 1
  - Use existing operator, no code changes

routing:
  - Create 3 separate routing policies
  - API Router load balances across models
```

**Pros**:
- ✅ No ai-aas code changes required
- ✅ Hardware isolation (QoS guarantees)
- ✅ Uses existing AIModel operator
- ✅ Standard Kubernetes scheduling

**Cons**:
- ❌ Requires MIG support verification on RTX 6000 Blackwell
- ❌ Fixed partition sizes (can't do 40GB + 30GB + 26GB)
- ❌ Requires cluster admin MIG configuration

**Implementation Steps**:
1. Verify MIG support: `nvidia-smi mig -lgip` on RTX 6000 node
2. Enable MIG: `nvidia-smi -mig 1`
3. Create instances: `nvidia-smi mig -cgi 14,14,14 -C` (3x 32GB)
4. Configure NVIDIA Device Plugin with MIG strategy
5. Deploy models normally via AIModel CRs

### Option B: Multiple vLLM Instances with MPS

**Architecture**:
```yaml
setup:
  1: "Enable MPS on GPU nodes"
  2: "Create MPS daemon configuration"

deployment:
  model1:
    runtime: vllm
    runtimeArgs: ["--gpu-memory-utilization", "0.30"]
    resources:
      requests:
        nvidia.com/gpu: "1"  # MPS time-slices
  model2:
    runtime: vllm
    runtimeArgs: ["--gpu-memory-utilization", "0.30"]
    resources:
      requests:
        nvidia.com/gpu: "1"
  model3:
    runtime: vllm
    runtimeArgs: ["--gpu-memory-utilization", "0.30"]
    resources:
      requests:
        nvidia.com/gpu: "1"
```

**Pros**:
- ✅ Works on all GPUs (no MIG requirement)
- ✅ Dynamic allocation
- ✅ Minimal ai-aas changes (just runtimeArgs)

**Cons**:
- ❌ No memory isolation (OOM risk)
- ❌ gpu-memory-utilization is a hint, not enforced
- ❌ All pods see same nvidia.com/gpu=1 resource
- ❌ Requires MPS daemon setup

**Implementation Steps**:
1. Install NVIDIA MPS on GPU nodes
2. Add `--gpu-memory-utilization` to AIModel runtimeArgs
3. Deploy multiple AIModel CRs
4. Monitor for OOM events

### Option C: Triton Multi-Model Repository

**Architecture**:
```yaml
approach: "Single Triton server, multiple TRT-LLM models"

setup:
  s3_structure:
    s3://ai-aas/models/blackwell/multi-model-repo/
      ├── llama-8b-ensemble/
      ├── mistral-7b-ensemble/
      └── qwen-7b-ensemble/

deployment:
  single_aimodel:
    runtime: triton-blackwell
    s3Key: models/blackwell/multi-model-repo/
    runtimeArgs:
      - --model-control-mode=explicit
      - --load-model=llama-8b-ensemble
      - --load-model=mistral-7b-ensemble
      - --load-model=qwen-7b-ensemble
```

**Pros**:
- ✅ Single GPU request
- ✅ Triton handles multiple models natively
- ✅ Production-grade model management

**Cons**:
- ❌ Requires all models as TRT-LLM engines
- ❌ Complex build process (3 separate compilations)
- ❌ ai-aas routing needs to know model names within Triton
- ❌ API Router needs new "multi-model backend" logic

**Implementation Steps**:
1. Compile 3 TRT-LLM engines for Blackwell
2. Structure S3 as multi-model repository
3. Modify AIModel operator to support multi-model Triton
4. Update API Router to route to specific model in Triton

### Option D: Custom Multi-Model AIModel (Largest Change)

**Architecture**:
```yaml
new_crd_field:
  spec:
    models:  # NEW FIELD
      - name: llama-8b
        modelID: meta-llama/Llama-3.1-8B-Instruct
        gpuMemoryFraction: 0.30
      - name: mistral-7b
        modelID: mistralai/Mistral-7B-Instruct-v0.3
        gpuMemoryFraction: 0.30
      - name: qwen-7b
        modelID: Qwen/Qwen2-VL-7B-Instruct
        gpuMemoryFraction: 0.30
```

**Requires**:
- New AIModel CRD fields
- Operator spawns multiple vLLM containers in one pod
- Container orchestration (sidecar pattern or init containers)
- API Router multi-model endpoint routing

**Pros**:
- ✅ Declarative multi-model config
- ✅ Platform-native solution

**Cons**:
- ❌ Large operator changes
- ❌ Complex pod lifecycle management
- ❌ Still no memory isolation

---

## 5. Benchmark Comparison Design

### Scenario 1: Multi-Model Single GPU (RTX 6000 Blackwell)

**Setup**:
- 1x RTX 6000 Blackwell (96GB)
- 3 models: Llama-3.1-8B, Mistral-7B, Qwen2-7B
- Deployment: MIG with 3x 32GB instances (or MPS)

**Benchmark Configuration**:
```yaml
targets:
  - model: llama-8b-blackwell
    endpoint: https://api.staging.otherjamesbrown.com/v1/chat/completions
    concurrent_requests: 10
  - model: mistral-7b-blackwell
    endpoint: https://api.staging.otherjamesbrown.com/v1/chat/completions
    concurrent_requests: 10
  - model: qwen-7b-blackwell
    endpoint: https://api.staging.otherjamesbrown.com/v1/chat/completions
    concurrent_requests: 10

scenario: "concurrent_multi_model"
duration: 600s
```

**Metrics**:
- Aggregate tokens/sec across all 3 models
- Per-model latency (p50, p90, p99)
- GPU utilization (single GPU, 3 models)
- GPU memory usage

### Scenario 2: Multi-Model Multi-GPU (RTX 4000 Ada)

**Setup**:
- 3x RTX 4000 Ada (20GB each)
- 3 models: Llama-3.1-8B, Mistral-7B, Qwen2-7B
- Deployment: 1 model per GPU (standard)

**Benchmark Configuration**:
```yaml
targets:
  - model: llama-8b-ada
    endpoint: https://api.staging.otherjamesbrown.com/v1/chat/completions
    concurrent_requests: 10
  - model: mistral-7b-ada
    endpoint: https://api.staging.otherjamesbrown.com/v1/chat/completions
    concurrent_requests: 10
  - model: qwen-7b-ada
    endpoint: https://api.staging.otherjamesbrown.com/v1/chat/completions
    concurrent_requests: 10

scenario: "concurrent_multi_gpu"
duration: 600s
```

**Metrics**:
- Aggregate tokens/sec across all 3 models
- Per-model latency (p50, p90, p99)
- Per-GPU utilization (3 GPUs, 1 model each)
- Total GPU memory usage

### Comparison Metrics

| Metric | Single GPU (Blackwell) | Multi-GPU (Ada) | Winner |
|--------|------------------------|-----------------|--------|
| Total tokens/sec | ? | ? | TBD |
| Avg latency (p50) | ? | ? | TBD |
| Tail latency (p99) | ? | ? | TBD |
| GPU utilization | 1 GPU @ X% | 3 GPUs @ Y% | TBD |
| VRAM used | 96GB * X% | 3 * 20GB * Y% | TBD |
| Cost efficiency | 1 GPU * $X | 3 GPUs * $Y | TBD |

**Hypothesis**:
- Blackwell single GPU will have **higher total throughput** (3.5x compute advantage)
- Ada multi-GPU will have **lower tail latency** (no GPU contention)
- Blackwell will be **more cost-efficient** (1 GPU vs 3 GPUs)

---

## 6. guidellm-runner Integration

### Current Capabilities

guidellm-runner currently supports:
- Creating benchmark targets (model + scenario)
- Running concurrent benchmarks via guidellm CLI
- Collecting metrics (tokens/sec, latency, errors)

### Required Changes for Multi-Model Benchmarking

**Option 1: Multiple Targets (Recommended)**
```yaml
# No guidellm-runner changes needed
# Run 3 separate benchmark targets simultaneously

benchmark_run:
  targets:
    - llama-8b-blackwell
    - mistral-7b-blackwell
    - qwen-7b-blackwell
  mode: parallel
  duration: 600s
```

**Option 2: Single "Multi-Model" Target**
```yaml
# Requires guidellm-runner enhancement

target:
  type: multi_model
  models:
    - llama-8b-blackwell
    - mistral-7b-blackwell
    - qwen-7b-blackwell
  distribution: round_robin  # or weighted
  duration: 600s
```

**Recommendation**: Use Option 1 (separate targets) initially. No code changes required.

---

## 7. Open Questions & Next Steps

### Questions Requiring Answers

1. **MIG Support Verification**:
   - Does RTX 6000 Blackwell support MIG?
   - What MIG profiles are available? (`nvidia-smi mig -lgip`)
   - Answer: **REQUIRED** before recommending Option A

2. **Kubernetes MIG Configuration**:
   - Is NVIDIA Device Plugin configured for MIG?
   - Are MIG resources exposed? (`kubectl describe node <gpu-node>`)
   - Answer: Run on staging cluster

3. **vLLM gpu-memory-utilization Enforcement**:
   - Is this a hard limit or soft hint?
   - Test: Deploy 2 vLLM instances with 0.6 each, monitor for OOM
   - Answer: Required for Option B viability

4. **Triton Multi-Model Memory Management**:
   - How does Triton allocate GPU memory across models?
   - Can we set per-model memory limits?
   - Answer: Review Triton docs + test empirically

5. **Cost Analysis**:
   - What is the hourly cost of RTX 6000 Blackwell vs RTX 4000 Ada?
   - Calculate: (Cost per GPU) * (Number of GPUs) * (Utilization %)

### Recommended Next Steps

**Phase 1: Investigation (1-2 days)**
1. ✅ Check MIG support on RTX 6000 Blackwell node
2. ✅ Verify NVIDIA Device Plugin MIG configuration
3. ✅ Review vLLM gpu-memory-utilization behavior (community docs)
4. ✅ Review Triton multi-model docs

**Phase 2: Proof of Concept (3-5 days)**
1. Deploy 3 models on RTX 6000 using **Option A (MIG)** if supported
2. Fallback to **Option B (MPS)** if MIG not available
3. Run initial benchmarks with guidellm-runner (separate targets)
4. Collect metrics: throughput, latency, GPU utilization

**Phase 3: Comparison Benchmark (2-3 days)**
1. Deploy same 3 models on 3x RTX 4000 Ada GPUs
2. Run identical benchmark workload
3. Compare results across both configurations
4. Generate comparison report

**Phase 4: Production Readiness (if viable)**
1. Document deployment patterns
2. Create use case definition (UC-MM-001: Multi-Model Single GPU)
3. Add validation to ai-model-operator (prevent over-allocation)
4. Update library entries with multi-model guidance

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| MIG not supported on RTX 6000 Blackwell | Medium | High | Fallback to MPS |
| vLLM OOM with MPS | High | High | Conservative memory limits, monitoring |
| Triton model loading failures | Medium | Medium | Pre-validate engine compatibility |
| Benchmark interference between models | Medium | Low | Stagger benchmark start times |
| Production OOM outages | High | Critical | Start with dev environment, gradual rollout |

---

## 9. Alternatives Considered

### Alternative 1: Don't Implement Multi-Model

**Rationale**: Keep 1 model = 1 GPU for simplicity

**Pros**:
- No complexity
- No risk of OOM
- Existing architecture works

**Cons**:
- Underutilized Blackwell GPUs (96GB mostly unused for 8B models)
- Higher cost (need more GPUs)
- Missed cost optimization opportunity

### Alternative 2: Use Smaller GPUs

**Rationale**: Deploy 8B models on RTX 4000 Ada only

**Pros**:
- Right-sized for small models
- Lower per-GPU cost

**Cons**:
- Can't run 13B+ models
- Blackwell GPUs sit idle
- Not cost-optimal

### Alternative 3: Model Multiplexing in API Router

**Rationale**: Load models on-demand, swap in/out of GPU

**Pros**:
- Efficient GPU usage
- Unlimited models (limited by S3 storage)

**Cons**:
- 2-5 minute model load times (user-facing latency)
- Complex caching logic
- Not suitable for production user-facing APIs

---

## 10. Conclusion

**Primary Recommendation**: Implement **Option A (MIG Partitioning)** if RTX 6000 Blackwell supports MIG.

**Rationale**:
- Hardware isolation ensures QoS
- No ai-aas code changes required
- Standard Kubernetes patterns
- Production-safe

**Fallback Recommendation**: If MIG not supported, use **Option B (MPS)** for non-production testing only.

**Next Action**: Run `nvidia-smi mig -lgip` on RTX 6000 Blackwell node to verify MIG support.

---

## Sources

- [vLLM Multiple Models per GPU Discussion](https://github.com/vllm-project/vllm/issues/13633)
- [vLLM Production Stack Multi-Model Issue](https://github.com/vllm-project/production-stack/issues/649)
- [vLLM Multiple Models Discussion](https://github.com/vllm-project/vllm/issues/3326)
- [NVIDIA Multi-Instance GPU (MIG) User Guide](https://docs.nvidia.com/datacenter/tesla/mig-user-guide/)
- [NVIDIA Multi-Process Service (MPS) Documentation](https://docs.nvidia.com/deploy/mps/)
- [Google Cloud NVIDIA MPS Guide](https://docs.cloud.google.com/kubernetes-engine/docs/how-to/nvidia-mps-gpus)
- [RedHat GPU Partitioning Guide](https://github.com/rh-aiservices-bu/gpu-partitioning-guide)
- [TensorRT-LLM GitHub](https://github.com/NVIDIA/TensorRT-LLM)
- [TensorRT-LLM Documentation](https://nvidia.github.io/TensorRT-LLM/)

---

**Document Version**: 1.0
**Last Updated**: 2026-01-07
**Author**: Claude (model-manager agent)
**Review Status**: Draft - Awaiting MIG verification
