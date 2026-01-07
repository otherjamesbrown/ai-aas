# Rebuild TensorRT-LLM Engine for RTX PRO 6000 Blackwell

**Created**: 2026-01-06
**Related Bead**: aas-mgmeu
**Target GPU**: RTX PRO 6000 Blackwell Server Edition (sm_120, 96GB VRAM)

## Overview

This runbook documents building and deploying TensorRT-LLM engines on Blackwell GPUs.

**Current State:**
- Engine: BF16, no quantization
- Performance: Fast build (~30 seconds)
- `/v1/completions`: Working
- `/v1/chat/completions`: Broken (see Known Issues)

## Known Issues

### Chat Completions Bug

**CRITICAL**: The `/v1/chat/completions` endpoint is broken in trtllm-serve across multiple versions:

| Version | Container | Error |
|---------|-----------|-------|
| TRT-LLM 0.20.0 | 25.06 | "No chat template found for the given tokenizer" |
| TRT-LLM 0.21.0 | 25.08 | "'NoneType' object has no attribute 'model_type'" |

**Root Cause**: Async/await bug in `chat_utils.py` (GitHub #5648)

**Working Endpoint**: `/v1/completions` works correctly in both versions.

**Workarounds**:
1. Use `/v1/completions` only - configure api-router to convert chat to completions
2. Deploy via Triton ensemble structure (like Ada) instead of trtllm-serve

## Prerequisites

### SSH Access
```bash
# SSH to Blackwell build server
ssh root@172.236.157.4
```

### Verify GPU
```bash
nvidia-smi --query-gpu=name,compute_cap,memory.total --format=csv
# Expected: NVIDIA RTX PRO 6000 Blackwell Server Edition, 12.0, 97887 MiB
```

### Required Credentials
- HuggingFace token with Llama access: `$HF_TOKEN` (find in `secrets/env/.env`)
- S3/Linode credentials for upload

## Container Version Compatibility

| Container | TRT-LLM | Blackwell (sm_120) | Chat Completions | Status |
|-----------|---------|-------------------|------------------|--------|
| 25.06 | 0.20.0 | Yes | Broken | Use for /v1/completions |
| 25.08 | 0.21.0 | Yes | Broken | Same issue, different error |

## Step-by-Step Build Process

### Step 1: Set Up Build Environment

```bash
# Create workspace
mkdir -p /root/trtllm-build
cd /root/trtllm-build

# Set HuggingFace token
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxx"

# Pull TensorRT-LLM container for Blackwell
docker pull nvcr.io/nvidia/tritonserver:25.08-trtllm-python-py3
```

### Step 2: Create Build Script

```bash
cat > /root/trtllm-build/build_engine.py << 'SCRIPT'
#!/usr/bin/env python3
"""
Build TensorRT-LLM engine for Llama 3.1 8B Instruct on Blackwell.
"""
import os
from tensorrt_llm import LLM, BuildConfig

def main():
    output_dir = "/workspace/engine_output"
    os.makedirs(output_dir, exist_ok=True)

    print("=" * 60)
    print("Building TensorRT-LLM Engine for Llama 3.1 8B Instruct")
    print("Target: RTX PRO 6000 Blackwell (sm_120)")
    print("=" * 60)

    # Build Configuration - optimized for Blackwell (96GB VRAM)
    # Single-model deployment: maximize batch size for full KV cache utilization
    build_config = BuildConfig()
    build_config.max_batch_size = 128  # 2x Ada - Blackwell has 4x VRAM headroom
    build_config.max_input_len = 8192
    build_config.max_seq_len = 12288   # Allow longer outputs
    build_config.max_num_tokens = 8192

    print("\nConfiguration:")
    print(f"  Max Batch Size: {build_config.max_batch_size}")
    print(f"  Max Input Length: {build_config.max_input_len}")
    print(f"  Max Sequence Length: {build_config.max_seq_len}")
    print()

    print("Building engine (this takes ~30 seconds on Blackwell)...")

    llm = LLM(
        model="meta-llama/Llama-3.1-8B-Instruct",
        build_config=build_config,
    )

    print(f"\nSaving engine to {output_dir}...")
    llm.save(output_dir)

    print("\n" + "=" * 60)
    print("Build Complete!")
    print(f"Engine saved to: {output_dir}")
    print("=" * 60)

    # List output files
    print("\nOutput files:")
    for f in os.listdir(output_dir):
        path = os.path.join(output_dir, f)
        size = os.path.getsize(path) / (1024**3)
        print(f"  {f}: {size:.2f} GB" if size > 0.01 else f"  {f}")

if __name__ == "__main__":
    main()
SCRIPT

chmod +x /root/trtllm-build/build_engine.py
```

### Step 3: Run the Build

```bash
docker run --gpus all --rm \
  -v /root/trtllm-build:/workspace \
  -v /root/.cache/huggingface:/root/.cache/huggingface \
  -e HF_TOKEN=$HF_TOKEN \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.08-trtllm-python-py3 \
  python3 /workspace/build_engine.py 2>&1 | tee build.log

# Build time: ~30 seconds on Blackwell (vs ~5-15 minutes on Ada)
```

### Step 4: Test Locally

```bash
# Start server
docker run --gpus all --rm -d --name trtllm-test \
  -v /root/trtllm-build:/workspace \
  -p 8000:8000 \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.08-trtllm-python-py3 \
  trtllm-serve /workspace/engine_output --host 0.0.0.0 --port 8000 --max_batch_size 64

# Wait for startup (~60 seconds)
sleep 60

# Test completions endpoint (this works)
curl -X POST http://localhost:8000/v1/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "engine_output",
    "prompt": "What is the capital of France?",
    "max_tokens": 50
  }'

# Test chat completions (broken - will return error)
curl -X POST http://localhost:8000/v1/chat/completions \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "engine_output",
    "messages": [{"role": "user", "content": "Hello"}],
    "max_tokens": 50
  }'

# Clean up
docker rm -f trtllm-test
```

### Step 5: Upload to S3

```bash
# Install mc (minio client)
which mc || curl -L https://dl.min.io/client/mc/release/linux-amd64/mc -o /usr/local/bin/mc && chmod +x /usr/local/bin/mc

# Configure (get credentials from secrets/env/.env)
mc alias set linode https://us-ord-1.linodeobjects.com ACCESS_KEY SECRET_KEY

# Upload engine files
mc cp -r /root/trtllm-build/engine_output/ linode/ai-aas/models/blackwell/llama-3.1-8b-instruct/trtllm-v1/

# Verify upload
mc ls linode/ai-aas/models/blackwell/llama-3.1-8b-instruct/trtllm-v1/
```

### Step 6: Deploy to Kubernetes

The deployment uses KServe with the Blackwell ClusterServingRuntime:

```yaml
# AIModel CR (ai-aas-config/environments/development/models/llama-3-1-8b-instruct-trtllm-blackwell.yaml)
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3-1-8b-instruct-trtllm-blackwell
spec:
  s3Key: models/blackwell/llama-3.1-8b-instruct/trtllm-v1
  runtime: tensorrt-llm-blackwell
  # ...
```

## Troubleshooting

### Container Fails to Start

Check max_batch_size parameter:
```bash
# Error: max_batch_size [2048] is greater than build_config.max_batch_size [64]
# Fix: Add --max_batch_size matching the engine build config
trtllm-serve /path/to/engine --max_batch_size 64
```

### Chat Completions Returns Error

This is a known bug. Use `/v1/completions` instead:
```bash
# Instead of chat/completions, use completions with proper prompt formatting
curl -X POST http://localhost:8000/v1/completions \
  -d '{"prompt": "<|begin_of_text|><|start_header_id|>user<|end_header_id|>\n\nHello<|eot_id|><|start_header_id|>assistant<|end_header_id|>\n\n"}'
```

### Engine Not Loading

Check TRT-LLM version compatibility:
```bash
# Engine version must match runtime version
docker run --rm --gpus all <container> python3 -c 'import tensorrt_llm; print(tensorrt_llm.__version__)'
```

## Performance Expectations

| Metric | Blackwell | Ada (for comparison) |
|--------|-----------|---------------------|
| Build Time | ~30 seconds | ~5-15 minutes |
| Engine Size | ~16 GB (BF16) | ~8.5 GB (FP8) |
| VRAM Available | 96 GB | 24 GB |
| Max Batch Size | 128 | 64 |
| KV Cache Budget | ~75 GB | ~12 GB |
| Concurrent Sequences | High (hundreds) | Medium (dozens) |

## References

- [TRT-LLM GitHub #5648](https://github.com/NVIDIA/TensorRT-LLM/issues/5648) - Chat completions bug
- [Bead aas-mgmeu](./.beads/) - Tracking issue for Blackwell chat completions
- [Ada FP8 Runbook](./rebuild-trtllm-fp8-ada.md) - Similar process for Ada GPUs
