# Rebuild TensorRT-LLM Engine with FP8 for RTX 4000 Ada

**Created**: 2026-01-06
**Related Bead**: aas-bhkzt
**Target GPU**: RTX 4000 Ada Generation (sm_89, 20GB VRAM)

## Overview

This runbook documents rebuilding the Llama 3.1 8B Instruct TensorRT-LLM engine with FP8 quantization to improve performance on RTX 4000 Ada GPUs.

**Current State:**
- Engine: FP16, no quantization
- Performance: ~72 tokens/sec
- Expected after FP8: ~130+ tokens/sec (matching or exceeding vLLM)

**Why FP8?**
- RTX 4000 Ada (Ada Lovelace, sm_89) has native FP8 tensor cores
- FP8 provides ~2x speedup for matrix multiplications vs FP16
- FP8 KV cache reduces memory usage, allowing more concurrent requests

## Prerequisites

### SSH Access
```bash
# SSH to RTX 4000 Ada build server
ssh user@<rtx4000-ada-server>
```

### Verify GPU
```bash
nvidia-smi --query-gpu=name,compute_cap,memory.total --format=csv
# Expected: NVIDIA RTX 4000 Ada Generation, 8.9, 20480 MiB
```

### Required Credentials
- HuggingFace token with Llama access: `$HF_TOKEN`
- S3/Linode credentials for upload (already configured in rclone)

## Step-by-Step Build Process

### Step 1: Set Up Build Environment

```bash
# Create workspace
mkdir -p ~/trtllm-fp8-build
cd ~/trtllm-fp8-build

# Set HuggingFace token
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxx"

# Pull TensorRT-LLM container for Ada (24.10 - required for FP8)
# NOTE: 24.08 has transformers too old for Llama 3.1, 24.12 is broken
docker pull nvcr.io/nvidia/tritonserver:24.10-trtllm-python-py3

# Run container with GPU access
docker run --gpus all --rm -it \
  -v $(pwd):/workspace \
  -v ~/.cache/huggingface:/root/.cache/huggingface \
  -e HF_TOKEN=$HF_TOKEN \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:24.10-trtllm-python-py3 \
  /bin/bash
```

### Step 2: Verify Environment (Inside Container)

```bash
# Verify TensorRT-LLM version
python3 -c "import tensorrt_llm; print(f'TRT-LLM: {tensorrt_llm.__version__}')"

# Verify GPU access
nvidia-smi

# Login to HuggingFace
huggingface-cli login --token $HF_TOKEN
```

### Step 3: Create FP8 Build Script

```bash
cat > /workspace/build_fp8_engine.py << 'EOF'
#!/usr/bin/env python3
"""
Build TensorRT-LLM engine with FP8 quantization for RTX 4000 Ada.

This script:
1. Downloads Llama 3.1 8B Instruct from HuggingFace
2. Calibrates FP8 quantization scales
3. Builds optimized TensorRT engine
4. Saves engine to /workspace/engine_output

NOTE: Requires setuptools (pip install setuptools) in the container
"""

import os
from tensorrt_llm.hlapi import LLM, BuildConfig, QuantConfig
from tensorrt_llm.quantization import QuantAlgo

def main():
    output_dir = "/workspace/engine_output"
    os.makedirs(output_dir, exist_ok=True)

    print("=" * 60)
    print("Building FP8 TensorRT-LLM Engine for Llama 3.1 8B Instruct")
    print("Target: RTX 4000 Ada (sm_89)")
    print("=" * 60)

    # FP8 Quantization Configuration
    # Uses tensor-wise FP8 for weights and KV cache
    quant_config = QuantConfig(
        quant_algo=QuantAlgo.FP8,
        kv_cache_quant_algo=QuantAlgo.FP8,  # FP8 KV cache for memory efficiency
    )

    # Build Configuration - optimized for RTX 4000 Ada (20GB VRAM)
    build_config = BuildConfig(
        max_batch_size=32,        # Reduced from 64 for 20GB VRAM
        max_input_len=4096,       # Optimized for typical workloads
        max_seq_len=8192,         # max_input + max_output
        max_num_tokens=4096,      # Tokens per batch
    )

    print("\nConfiguration:")
    print(f"  Quantization: FP8 (weights + KV cache)")
    print(f"  Max Batch Size: {build_config.max_batch_size}")
    print(f"  Max Input Length: {build_config.max_input_len}")
    print(f"  Max Sequence Length: {build_config.max_seq_len}")
    print()

    print("Building engine (this may take 5-15 minutes)...")
    print("  - Downloading model from HuggingFace")
    print("  - Calibrating FP8 scales")
    print("  - Compiling TensorRT engine")
    print()

    # Build the engine
    llm = LLM(
        model="meta-llama/Llama-3.1-8B-Instruct",
        build_config=build_config,
        quant_config=quant_config,
    )

    # Save the compiled engine
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
        size = os.path.getsize(path) / (1024**3)  # GB
        print(f"  {f}: {size:.2f} GB" if size > 0.01 else f"  {f}")

if __name__ == "__main__":
    main()
EOF

chmod +x /workspace/build_fp8_engine.py
```

### Step 4: Run the Build

```bash
cd /workspace

# Install setuptools (required for modelopt)
pip install setuptools

# Run the build
python3 build_fp8_engine.py 2>&1 | tee build.log

# Build time: ~5-15 minutes on RTX 4000 Ada
# Monitor GPU usage: watch -n 1 nvidia-smi
```

### Step 5: Test the Engine (Optional)

```bash
cat > /workspace/test_engine.py << 'EOF'
from tensorrt_llm import LLM, SamplingParams

def main():
    print("Loading FP8 engine...")
    llm = LLM(model="/workspace/engine_output")

    print("Running inference test...")
    prompts = [
        "What is the capital of France?",
        "Explain quantum computing in simple terms.",
    ]
    sampling_params = SamplingParams(temperature=0.7, max_tokens=100)

    outputs = llm.generate(prompts, sampling_params)
    for output in outputs:
        print(f"\nPrompt: {output.prompt}")
        print(f"Response: {output.outputs[0].text}")

    print("\nFP8 engine test passed!")

if __name__ == "__main__":
    main()
EOF

python3 test_engine.py
```

### Step 6: Create Triton Model Repository

The new high-level API outputs a simplified structure. We need to create the full Triton ensemble structure.

```bash
# Create model repository structure
mkdir -p /workspace/model_repository/{ensemble/1,preprocessing/1,postprocessing/1,tensorrt_llm/1}

# Copy engine files
cp /workspace/engine_output/rank0.engine /workspace/model_repository/tensorrt_llm/1/
cp /workspace/engine_output/config.json /workspace/model_repository/tensorrt_llm/1/
cp /workspace/engine_output/tokenizer.json /workspace/model_repository/tensorrt_llm/1/
cp /workspace/engine_output/tokenizer_config.json /workspace/model_repository/tensorrt_llm/1/
cp /workspace/engine_output/special_tokens_map.json /workspace/model_repository/tensorrt_llm/1/

# Create placeholder for ensemble
touch /workspace/model_repository/ensemble/1/.placeholder
```

Create config files (copy from existing deployment or use templates from main runbook).

### Step 7: Upload to S3

```bash
# Exit container first, then from host:
exit

# Install rclone if not present
which rclone || (curl https://rclone.org/install.sh | sudo bash)

# Verify rclone config exists
cat ~/.config/rclone/rclone.conf | grep -A5 linode

# Backup existing engine (optional)
rclone copy linode:ai-aas/models/llama-3.1-8b-instruct-trtllm/ \
  linode:ai-aas/models/llama-3.1-8b-instruct-trtllm-backup-fp16/ \
  --progress

# Upload new FP8 engine
rclone sync ~/trtllm-fp8-build/model_repository/ \
  linode:ai-aas/models/llama-3.1-8b-instruct-trtllm/ \
  --progress \
  --exclude "*.pyc" \
  --exclude "__pycache__/**"

# Verify upload
rclone ls linode:ai-aas/models/llama-3.1-8b-instruct-trtllm/
```

### Step 8: Restart TRT-LLM Pod

```bash
# Delete pod to force re-download from S3
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-development.yaml \
  delete pod -n development -l serving.kserve.io/inferenceservice=llama-3-1-8b-instruct-trtllm

# Watch pod restart
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-development.yaml \
  get pods -n development -w | grep trtllm
```

### Step 9: Verify Performance

Wait for benchmarks to run (every 5 minutes) and check results:

```bash
# Check benchmark logs
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-development.yaml \
  logs -n development -l app=guidellm-runner --tail=20

# Expected: TRT-LLM tokens/sec should match or exceed vLLM (~130 tok/s)
```

## Rollback Procedure

If the FP8 engine causes issues:

```bash
# Restore FP16 backup
rclone sync linode:ai-aas/models/llama-3.1-8b-instruct-trtllm-backup-fp16/ \
  linode:ai-aas/models/llama-3.1-8b-instruct-trtllm/ \
  --progress

# Restart pod
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-development.yaml \
  delete pod -n development -l serving.kserve.io/inferenceservice=llama-3-1-8b-instruct-trtllm
```

## Troubleshooting

### Build Fails with CUDA OOM

Reduce batch size and sequence lengths:
```python
build_config = BuildConfig(
    max_batch_size=16,      # Reduce from 32
    max_input_len=2048,     # Reduce from 4096
    max_seq_len=4096,       # Reduce from 8192
)
```

### FP8 Not Supported Error

Verify GPU compute capability is >= 8.9:
```bash
nvidia-smi --query-gpu=compute_cap --format=csv
# Must be 8.9 or higher for FP8
```

### Calibration Takes Too Long

Reduce calibration batches:
```python
calib_config = CalibConfig(
    calib_batches=64,       # Reduce from 128
    calib_batch_size=1,
    calib_max_seq_length=256,  # Reduce from 512
)
```

### Pod CrashLoopBackOff After Upload

Check for S3 prefix conflicts (like the blackwell issue):
```bash
/tmp/mc ls linode/ai-aas/models/ | grep llama
# Ensure no overlapping prefixes
```

## References

- [TensorRT-LLM FP8 Quantization Guide](https://nvidia.github.io/TensorRT-LLM/performance/performance-tuning-guide/fp8-quantization.html)
- [TensorRT-LLM Quantization API](https://nvidia.github.io/TensorRT-LLM/python-api/tensorrt_llm.quantization.html)
- [Main TRT-LLM Build Runbook](./build-tensorrt-llm-engine.md)
- [Bead aas-bhkzt](../.beads/) - Tracking issue for this work
