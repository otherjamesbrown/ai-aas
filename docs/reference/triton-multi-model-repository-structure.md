# Triton Multi-Model Repository Structure

**Created**: 2026-01-07
**Related Bead**: aas-uyvr4

## Overview

This document describes the model repository structure for hosting multiple TensorRT-LLM models on a single GPU using NVIDIA Triton Inference Server with explicit model control.

## Directory Structure

```
model_repository/
├── llama-3.1-8b-instruct/
│   ├── config.pbtxt              # Model configuration with KV cache settings
│   └── 1/                        # Version directory
│       ├── rank0.engine          # TensorRT engine file (~16GB)
│       ├── config.json           # Engine configuration from TRT-LLM
│       ├── tokenizer.json        # Tokenizer vocabulary
│       ├── tokenizer_config.json # Tokenizer settings
│       └── special_tokens_map.json
├── mistral-7b-instruct/
│   ├── config.pbtxt
│   └── 1/
│       ├── rank0.engine          # (~14GB)
│       ├── config.json
│       ├── tokenizer.json
│       ├── tokenizer_config.json
│       └── special_tokens_map.json
└── qwen2-7b-instruct/
    ├── config.pbtxt
    └── 1/
        ├── rank0.engine          # (~14GB)
        ├── config.json
        ├── tokenizer.json
        ├── tokenizer_config.json
        └── special_tokens_map.json
```

## Configuration Files

### config.pbtxt

Each model requires a `config.pbtxt` file that configures the TensorRT-LLM backend and KV cache allocation.

```protobuf
name: "llama-3.1-8b-instruct"
backend: "tensorrtllm"
max_batch_size: 32

# Decoupled mode for streaming support
model_transaction_policy {
  decoupled: True
}

input [
  {
    name: "text_input"
    data_type: TYPE_STRING
    dims: [ 1 ]
  },
  {
    name: "max_tokens"
    data_type: TYPE_INT32
    dims: [ 1 ]
    optional: true
  },
  {
    name: "temperature"
    data_type: TYPE_FP32
    dims: [ 1 ]
    optional: true
  },
  {
    name: "top_p"
    data_type: TYPE_FP32
    dims: [ 1 ]
    optional: true
  },
  {
    name: "stream"
    data_type: TYPE_BOOL
    dims: [ 1 ]
    optional: true
  }
]

output [
  {
    name: "text_output"
    data_type: TYPE_STRING
    dims: [ -1 ]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_GPU
    gpus: [ 0 ]
  }
]

# TensorRT-LLM specific parameters
parameters: {
  key: "gpt_model_type"
  value: { string_value: "inflight_fused_batching" }
}

parameters: {
  key: "batch_scheduler_policy"
  value: { string_value: "max_utilization" }
}

# CRITICAL: KV cache allocation for multi-model deployment
# This fraction applies to FREE GPU memory after engine loading
# For 3 models: ~0.28 each (total ~0.84, leaving 0.16 for overhead)
parameters: {
  key: "kv_cache_free_gpu_mem_fraction"
  value: { string_value: "0.28" }
}

parameters: {
  key: "max_beam_width"
  value: { string_value: "1" }
}

parameters: {
  key: "enable_chunked_context"
  value: { string_value: "true" }
}
```

### Key Configuration Parameters

| Parameter | Description | Multi-Model Value |
|-----------|-------------|-------------------|
| `max_batch_size` | Maximum concurrent requests | Reduced (32 vs 64) |
| `kv_cache_free_gpu_mem_fraction` | Fraction of free VRAM for KV cache | 0.28 per model for 3 models |
| `batch_scheduler_policy` | Batching strategy | `max_utilization` |
| `enable_chunked_context` | Enable chunked prefill | `true` (memory efficient) |
| `decoupled` | Enable streaming | `True` for gRPC streaming |

## KV Cache Allocation Strategy

### Memory Layout (96GB Blackwell GPU)

```
┌─────────────────────────────────────────────────────────────────────┐
│                          96 GB VRAM                                  │
├─────────────────────────────────────────────────────────────────────┤
│  Engine 1    │  Engine 2    │  Engine 3    │  KV Cache  │ Overhead  │
│  (Llama)     │  (Mistral)   │  (Qwen2)     │  (shared)  │           │
│  ~16GB       │  ~14GB       │  ~14GB       │  ~44GB     │  ~8GB     │
├──────────────┴──────────────┴──────────────┼────────────┴───────────┤
│              Static (~44GB)                 │   Dynamic (~52GB)      │
└─────────────────────────────────────────────────────────────────────┘
```

### KV Cache Fraction Formula

```
kv_cache_fraction = target_kv_cache_per_model / remaining_gpu_memory

For 3 models with equal allocation on 96GB GPU:
- Engines consume ~44GB
- Remaining: ~52GB
- Target per model: ~14GB KV cache
- Overhead: ~8GB
- Net for KV: ~44GB
- Per model fraction: 44GB / 52GB / 3 models ≈ 0.28
```

### Allocation Strategies

**Equal Allocation (Default):**
```
Model 1: kv_cache_free_gpu_mem_fraction = 0.28
Model 2: kv_cache_free_gpu_mem_fraction = 0.28
Model 3: kv_cache_free_gpu_mem_fraction = 0.28
Total: 0.84
```

**Traffic-Weighted Allocation:**
```
High-traffic model: kv_cache_free_gpu_mem_fraction = 0.40
Medium-traffic:     kv_cache_free_gpu_mem_fraction = 0.25
Low-traffic:        kv_cache_free_gpu_mem_fraction = 0.20
Total: 0.85
```

## Triton Server Launch

### Explicit Model Control Mode

For multi-model deployments, use `--model-control-mode=explicit` to control which models are loaded:

```bash
tritonserver \
  --model-repository=/models \
  --model-control-mode=explicit \
  --load-model=llama-3.1-8b-instruct \
  --load-model=mistral-7b-instruct \
  --load-model=qwen2-7b-instruct
```

### Model Control Modes

| Mode | Behavior | Use Case |
|------|----------|----------|
| `none` | All models loaded at startup | Single model |
| `poll` | Watch for new models | Dynamic deployment |
| `explicit` | Only load specified models | Multi-model, controlled loading |

### Loading/Unloading Models at Runtime

```bash
# Load a model
curl -X POST http://localhost:8000/v2/repository/models/llama-3.1-8b-instruct/load

# Unload a model
curl -X POST http://localhost:8000/v2/repository/models/llama-3.1-8b-instruct/unload

# Check model status
curl http://localhost:8000/v2/models/llama-3.1-8b-instruct
```

## S3 Storage Layout

For KServe/AIModel operator integration, the model repository is stored in S3:

```
s3://ai-aas/models/blackwell/multi-model-3x8b/
├── llama-3.1-8b-instruct/
│   ├── config.pbtxt
│   └── 1/
│       ├── rank0.engine
│       ├── config.json
│       └── tokenizer*.json
├── mistral-7b-instruct/
│   ├── config.pbtxt
│   └── 1/
│       └── ...
└── qwen2-7b-instruct/
    ├── config.pbtxt
    └── 1/
        └── ...
```

## API Router Integration

The API router routes requests to specific models within the multi-model Triton server using the `tritonModelName` field:

```yaml
# Routing policy configuration
routing:
  policies:
    - model: "llama-3.1-8b-instruct"
      backends:
        - backendID: "multi-model-3x8b-blackwell"
          weight: 100
          tritonModelName: "llama-3.1-8b-instruct"  # Routes to this model in Triton
```

The router constructs the Triton endpoint as:
- HTTP: `http://<backend>/v2/models/<tritonModelName>/infer`
- gRPC: Uses `tritonModelName` in the `ModelInferRequest.model_name` field

## Validation Checklist

Before deploying:

- [ ] Each model has `config.pbtxt` in the model root directory
- [ ] Engine files are in version subdirectory (e.g., `1/rank0.engine`)
- [ ] Tokenizer files are present in version directory
- [ ] KV cache fractions sum to < 0.90 (leave overhead)
- [ ] `max_batch_size` is consistent between engine build and config.pbtxt
- [ ] `decoupled: True` is set for streaming support

## Troubleshooting

### Model Fails to Load

Check Triton logs for:
```
E0107 10:30:00.123456 1 model_lifecycle.cc:123] failed to load 'llama-3.1-8b-instruct' version 1: ...
```

Common causes:
- Missing engine file
- Engine built for different GPU architecture
- KV cache fraction too high (OOM)

### KV Cache Exhausted

```
W0107 10:30:00.123456 1 scheduler.cc:123] Request rejected: no free KV cache blocks
```

Solutions:
- Reduce `max_batch_size`
- Reduce `kv_cache_free_gpu_mem_fraction`
- Reduce `max_seq_len` in engine build

### Slow First Request

Expected behavior with `--model-control-mode=explicit`. The first request to each model triggers KV cache allocation.

## References

- [Triton Model Repository](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/model_repository.md)
- [Triton Model Configuration](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/model_configuration.md)
- [TRT-LLM Backend](https://github.com/triton-inference-server/tensorrtllm_backend)
- [Build Runbook](../runbooks/build-trtllm-multi-model-blackwell.md)
