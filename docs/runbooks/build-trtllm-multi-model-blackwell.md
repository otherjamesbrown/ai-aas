# Build TensorRT-LLM Multi-Model Engines for Blackwell

**Created**: 2026-01-07
**Related Bead**: aas-uyvr4
**Target GPU**: RTX PRO 6000 Blackwell Server Edition (sm_120, 96GB VRAM)

## Overview

This runbook documents building and deploying multiple TensorRT-LLM engines on a single Blackwell GPU using Triton's multi-model serving capability. This enables hosting 3x 7-8B parameter models on one 96GB GPU.

**Target Configuration:**
- 3 models on 1 GPU (Llama 3.1 8B, Mistral 7B, Qwen2 7B)
- ~30GB VRAM per model (16GB engine + KV cache)
- Dynamic KV cache allocation via `kv_cache_free_gpu_mem_fraction`

## Prerequisites

### Hardware Requirements
- RTX PRO 6000 Blackwell Server Edition (96GB VRAM)
- 64GB+ system RAM
- 200GB+ storage for engines

### Software Requirements
- Container: `nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3` or newer
- HuggingFace token with access to gated models

### Access
```bash
# SSH to Blackwell build server
ssh root@172.236.157.4

# Verify GPU
nvidia-smi --query-gpu=name,compute_cap,memory.total --format=csv
# Expected: NVIDIA RTX PRO 6000 Blackwell Server Edition, 12.0, 97887 MiB
```

## Model Selection

For a 96GB GPU with 3 models, each model gets ~30GB VRAM budget:

| Model | HuggingFace ID | Engine Size | KV Cache Fraction |
|-------|----------------|-------------|-------------------|
| Llama 3.1 8B Instruct | meta-llama/Llama-3.1-8B-Instruct | ~16GB | 0.28 |
| Mistral 7B Instruct v0.3 | mistralai/Mistral-7B-Instruct-v0.3 | ~14GB | 0.28 |
| Qwen2 7B Instruct | Qwen/Qwen2-7B-Instruct | ~14GB | 0.28 |

**Total KV Cache**: 0.84 (leaving 0.16 or ~15GB for system overhead and engine loading)

## Step-by-Step Build Process

### Step 1: Set Up Build Environment

```bash
# Create workspace
mkdir -p /root/trtllm-multi-model
cd /root/trtllm-multi-model

# Set HuggingFace token
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxx"

# Pull TensorRT-LLM container for Blackwell
docker pull nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3
```

### Step 2: Create Multi-Model Build Script

```bash
cat > /root/trtllm-multi-model/build_all_engines.py << 'SCRIPT'
#!/usr/bin/env python3
"""
Build TensorRT-LLM engines for multi-model deployment on Blackwell.
Builds 3 models sequentially to avoid OOM during compilation.
"""
import os
import sys
from tensorrt_llm import LLM, BuildConfig

# Model configurations - adjust batch size based on KV cache budget
MODELS = [
    {
        "name": "llama-3.1-8b-instruct",
        "model_id": "meta-llama/Llama-3.1-8B-Instruct",
        "max_batch_size": 32,  # Reduced for multi-model deployment
        "max_input_len": 4096,
        "max_seq_len": 6144,
    },
    {
        "name": "mistral-7b-instruct",
        "model_id": "mistralai/Mistral-7B-Instruct-v0.3",
        "max_batch_size": 32,
        "max_input_len": 4096,
        "max_seq_len": 6144,
    },
    {
        "name": "qwen2-7b-instruct",
        "model_id": "Qwen/Qwen2-7B-Instruct",
        "max_batch_size": 32,
        "max_input_len": 4096,
        "max_seq_len": 6144,
    },
]

def build_model(model_config, output_base_dir):
    """Build a single TensorRT-LLM engine."""
    name = model_config["name"]
    output_dir = os.path.join(output_base_dir, name)
    os.makedirs(output_dir, exist_ok=True)

    print("=" * 60)
    print(f"Building: {name}")
    print(f"Model ID: {model_config['model_id']}")
    print(f"Output: {output_dir}")
    print("=" * 60)

    build_config = BuildConfig()
    build_config.max_batch_size = model_config["max_batch_size"]
    build_config.max_input_len = model_config["max_input_len"]
    build_config.max_seq_len = model_config["max_seq_len"]
    build_config.max_num_tokens = model_config["max_input_len"]

    print(f"\nBuild Configuration:")
    print(f"  Max Batch Size: {build_config.max_batch_size}")
    print(f"  Max Input Length: {build_config.max_input_len}")
    print(f"  Max Sequence Length: {build_config.max_seq_len}")

    print("\nBuilding engine (this takes ~30 seconds per model on Blackwell)...")

    llm = LLM(
        model=model_config["model_id"],
        build_config=build_config,
    )

    print(f"\nSaving engine to {output_dir}...")
    llm.save(output_dir)

    # List output files
    print(f"\nOutput files for {name}:")
    total_size = 0
    for f in sorted(os.listdir(output_dir)):
        path = os.path.join(output_dir, f)
        if os.path.isfile(path):
            size = os.path.getsize(path) / (1024**3)
            total_size += size
            print(f"  {f}: {size:.2f} GB" if size > 0.01 else f"  {f}")
    print(f"  Total: {total_size:.2f} GB")

    # Force cleanup to free GPU memory before next model
    del llm
    import gc
    gc.collect()

    return output_dir


def main():
    output_base = "/workspace/engines"
    os.makedirs(output_base, exist_ok=True)

    print("=" * 60)
    print("Multi-Model TensorRT-LLM Engine Build")
    print(f"Target: RTX PRO 6000 Blackwell (sm_120, 96GB)")
    print(f"Models: {len(MODELS)}")
    print("=" * 60)

    built_engines = []
    for i, model_config in enumerate(MODELS, 1):
        print(f"\n[{i}/{len(MODELS)}] Building {model_config['name']}...\n")
        try:
            engine_dir = build_model(model_config, output_base)
            built_engines.append((model_config["name"], engine_dir))
            print(f"\n[{i}/{len(MODELS)}] SUCCESS: {model_config['name']}")
        except Exception as e:
            print(f"\n[{i}/{len(MODELS)}] FAILED: {model_config['name']}: {e}")
            sys.exit(1)

    print("\n" + "=" * 60)
    print("All Engines Built Successfully!")
    print("=" * 60)
    for name, path in built_engines:
        print(f"  {name}: {path}")

    print("\nNext steps:")
    print("  1. Create Triton model repository structure")
    print("  2. Generate config.pbtxt files with KV cache settings")
    print("  3. Upload to S3")


if __name__ == "__main__":
    main()
SCRIPT

chmod +x /root/trtllm-multi-model/build_all_engines.py
```

### Step 3: Run the Multi-Model Build

```bash
docker run --gpus all --rm \
  -v /root/trtllm-multi-model:/workspace \
  -v /root/.cache/huggingface:/root/.cache/huggingface \
  -e HF_TOKEN=$HF_TOKEN \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3 \
  python3 /workspace/build_all_engines.py 2>&1 | tee build_all.log

# Build time: ~90-120 seconds total (3 models x ~30 seconds each)
```

### Step 4: Create Triton Multi-Model Repository

After building engines, create the Triton model repository structure:

```bash
cat > /root/trtllm-multi-model/create_model_repo.sh << 'SCRIPT'
#!/bin/bash
set -e

ENGINES_DIR="/root/trtllm-multi-model/engines"
REPO_DIR="/root/trtllm-multi-model/model_repository"

# Model configurations (name, kv_cache_fraction)
declare -A MODELS=(
    ["llama-3.1-8b-instruct"]="0.28"
    ["mistral-7b-instruct"]="0.28"
    ["qwen2-7b-instruct"]="0.28"
)

echo "Creating Triton multi-model repository..."
mkdir -p "$REPO_DIR"

for model_name in "${!MODELS[@]}"; do
    kv_fraction="${MODELS[$model_name]}"
    engine_dir="$ENGINES_DIR/$model_name"
    model_dir="$REPO_DIR/$model_name"

    if [ ! -d "$engine_dir" ]; then
        echo "ERROR: Engine directory not found: $engine_dir"
        exit 1
    fi

    echo "Setting up $model_name (kv_cache_fraction=$kv_fraction)..."

    # Create model directory structure
    mkdir -p "$model_dir/1"

    # Copy engine and tokenizer files
    cp "$engine_dir/rank0.engine" "$model_dir/1/" 2>/dev/null || \
    cp "$engine_dir/"*.engine "$model_dir/1/"

    # Copy tokenizer files
    for f in tokenizer.json tokenizer_config.json special_tokens_map.json; do
        if [ -f "$engine_dir/$f" ]; then
            cp "$engine_dir/$f" "$model_dir/1/"
        fi
    done

    # Copy config.json
    [ -f "$engine_dir/config.json" ] && cp "$engine_dir/config.json" "$model_dir/1/"

    # Generate config.pbtxt with KV cache settings
    cat > "$model_dir/config.pbtxt" << EOF
name: "$model_name"
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
  value: {
    string_value: "inflight_fused_batching"
  }
}

parameters: {
  key: "batch_scheduler_policy"
  value: {
    string_value: "max_utilization"
  }
}

# CRITICAL: KV cache allocation for multi-model deployment
# Total across all models should be < 0.95 to leave room for engine loading
parameters: {
  key: "kv_cache_free_gpu_mem_fraction"
  value: {
    string_value: "$kv_fraction"
  }
}

parameters: {
  key: "max_beam_width"
  value: {
    string_value: "1"
  }
}

parameters: {
  key: "enable_chunked_context"
  value: {
    string_value: "true"
  }
}
EOF

    echo "  Created $model_dir/config.pbtxt"
done

echo ""
echo "Model repository created at: $REPO_DIR"
echo ""
echo "Directory structure:"
find "$REPO_DIR" -type f -name "*.pbtxt" -o -name "*.engine" | head -20

echo ""
echo "To test locally:"
echo "  tritonserver --model-repository=$REPO_DIR \\"
echo "    --model-control-mode=explicit \\"
echo "    --load-model=llama-3.1-8b-instruct \\"
echo "    --load-model=mistral-7b-instruct \\"
echo "    --load-model=qwen2-7b-instruct"
SCRIPT

chmod +x /root/trtllm-multi-model/create_model_repo.sh
./create_model_repo.sh
```

### Step 5: Test Multi-Model Locally

```bash
# Start Triton with explicit model loading
docker run --gpus all --rm -d --name triton-multi-test \
  -v /root/trtllm-multi-model/model_repository:/models \
  -p 8000:8000 -p 8001:8001 -p 8002:8002 \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3 \
  tritonserver --model-repository=/models \
    --model-control-mode=explicit \
    --load-model=llama-3.1-8b-instruct \
    --load-model=mistral-7b-instruct \
    --load-model=qwen2-7b-instruct

# Wait for models to load (~2-3 minutes for 3 models)
sleep 180

# Check model status
curl http://localhost:8000/v2/models

# Test each model
for model in llama-3.1-8b-instruct mistral-7b-instruct qwen2-7b-instruct; do
  echo "Testing $model..."
  curl -X POST http://localhost:8000/v2/models/$model/infer \
    -H 'Content-Type: application/json' \
    -d '{
      "inputs": [
        {"name": "text_input", "shape": [1, 1], "datatype": "BYTES", "data": ["Hello, how are you?"]},
        {"name": "max_tokens", "shape": [1, 1], "datatype": "INT32", "data": [50]}
      ]
    }' | jq .
done

# Check GPU memory usage
nvidia-smi

# Cleanup
docker rm -f triton-multi-test
```

### Step 6: Upload to S3

```bash
# Install rclone if needed
which rclone || curl https://rclone.org/install.sh | bash

# Configure rclone (credentials from secrets/env/.env)
cat > ~/.config/rclone/rclone.conf << 'EOF'
[linode]
type = s3
provider = Ceph
access_key_id = YOUR_ACCESS_KEY
secret_access_key = YOUR_SECRET_KEY
endpoint = https://us-ord-1.linodeobjects.com
acl = private
EOF

# Upload entire model repository
rclone copy /root/trtllm-multi-model/model_repository \
  linode:ai-aas/models/blackwell/multi-model-3x8b/ \
  --progress

# Verify upload
rclone ls linode:ai-aas/models/blackwell/multi-model-3x8b/
```

## KV Cache Allocation Strategy

### Memory Budget (96GB VRAM)

| Component | VRAM | Notes |
|-----------|------|-------|
| Engine 1 (Llama 3.1 8B) | ~16GB | Static weight loading |
| Engine 2 (Mistral 7B) | ~14GB | Static weight loading |
| Engine 3 (Qwen2 7B) | ~14GB | Static weight loading |
| **Total Engine Memory** | **~44GB** | |
| Available for KV Cache | ~52GB | |
| System Overhead | ~8GB | CUDA context, buffers |
| **Net KV Cache** | **~44GB** | Split across 3 models |

### KV Cache Fraction Calculation

The `kv_cache_free_gpu_mem_fraction` parameter controls what fraction of **remaining** GPU memory (after engine loading) is used for KV cache.

For 3 models with equal allocation:
- Total target: 0.84 (leaving 0.16 for overhead)
- Per model: 0.28

**Adjustment based on model usage:**
- High-traffic model (e.g., Llama): 0.35
- Medium-traffic models: 0.25 each
- Total: 0.35 + 0.25 + 0.25 = 0.85

## Deployment via AIModel CR

Create an AIModel CR with multi-model configuration:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: multi-model-3x8b-blackwell
  namespace: ai-model-system
spec:
  displayName: "Multi-Model 3x8B (Blackwell)"
  description: "3 x 7-8B LLM models on single RTX 6000 Blackwell"
  s3Bucket: ai-aas
  runtime: triton

  # Multi-model configuration
  multiModel:
    enabled: true
    models:
      - name: llama-3.1-8b-instruct
        s3Key: models/blackwell/multi-model-3x8b/llama-3.1-8b-instruct
        kvCacheFraction: "0.28"
        maxBatchSize: 32
      - name: mistral-7b-instruct
        s3Key: models/blackwell/multi-model-3x8b/mistral-7b-instruct
        kvCacheFraction: "0.28"
        maxBatchSize: 32
      - name: qwen2-7b-instruct
        s3Key: models/blackwell/multi-model-3x8b/qwen2-7b-instruct
        kvCacheFraction: "0.28"
        maxBatchSize: 32

  resources:
    gpu:
      vendor: nvidia
      count: 1
      minMemoryGB: 80
    cpu:
      requests: "16"
      limits: "32"
    memory:
      requests: "64Gi"
      limits: "96Gi"

  scheduling:
    nodeSelector:
      node.kubernetes.io/instance-type: g3-gpu-rtxpro6000-blackwell-1
```

## API Router Configuration

Update routing policy to route to specific models:

```yaml
# values-development.yaml
routing:
  policies:
    - model: "llama-3.1-8b-instruct"
      backends:
        - backendID: "multi-model-3x8b-blackwell"
          weight: 100
          tritonModelName: "llama-3.1-8b-instruct"
    - model: "mistral-7b-instruct"
      backends:
        - backendID: "multi-model-3x8b-blackwell"
          weight: 100
          tritonModelName: "mistral-7b-instruct"
    - model: "qwen2-7b-instruct"
      backends:
        - backendID: "multi-model-3x8b-blackwell"
          weight: 100
          tritonModelName: "qwen2-7b-instruct"
```

## Troubleshooting

### Models Fail to Load (OOM)

```bash
# Check GPU memory during loading
nvidia-smi -l 1

# Reduce KV cache fractions
# Total should be < 0.85 for 3 models
```

### KV Cache Exhaustion During Inference

```bash
# Monitor Triton metrics
curl http://localhost:8002/metrics | grep kv_cache

# Reduce max_batch_size in config.pbtxt
# Or reduce kv_cache_free_gpu_mem_fraction
```

### Slow Model Switching

The first request to a "cold" model may be slow as Triton loads the engine. This is expected behavior with `--model-control-mode=explicit`.

## Performance Expectations

| Metric | Single Model | Multi-Model (3x) |
|--------|--------------|------------------|
| Max Batch Size | 64 | 32 per model |
| KV Cache per Model | 80% of free | ~28% each |
| Concurrent Requests | High | Medium per model |
| Memory Utilization | ~20GB | ~80GB |

## References

- [Triton Model Control Mode](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/model_management.md)
- [TRT-LLM KV Cache Configuration](https://github.com/NVIDIA/TensorRT-LLM/blob/main/docs/source/kv_cache.md)
- [Bead aas-uyvr4](../../.beads/) - Tracking issue for this work
- [Bead aas-31zsd](../../.beads/) - Parent epic for multi-model deployment
