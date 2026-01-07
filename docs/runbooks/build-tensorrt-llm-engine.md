# Build TensorRT-LLM Engine - Runbook

**Last Updated**: 2025-12-15  
**Owner**: AI Platform Engineering  
**Related Spec**: [specs/029-triton-tensorrt-llm](../../specs/029-triton-tensorrt-llm/)

## Overview

This runbook documents the process of building TensorRT-LLM engines from HuggingFace checkpoints for deployment with NVIDIA Triton Inference Server on the ai-aas platform.

TensorRT-LLM provides optimized inference kernels for NVIDIA GPUs, delivering:
- Higher throughput via in-flight batching
- Lower latency through kernel fusion
- Better GPU memory utilization
- Faster time-to-first-token

## Prerequisites

### Hardware Requirements

- **GPU**: NVIDIA GPU with Compute Capability 8.0+ (A100, H100, L40S, RTX 4090)
  - Minimum 24GB VRAM for Llama 3.1 8B
  - Minimum 48GB VRAM for larger models (13B+)
- **CPU**: 8+ cores recommended
- **RAM**: 64GB+ recommended
- **Storage**: 100GB+ free space for checkpoints and engines

### Software Requirements

- **Operating System**: Ubuntu 22.04 LTS (recommended)
- **CUDA**: 12.2 or later
- **cuDNN**: 8.9.x or later
- **Docker**: 24.0+ (optional, for containerized builds)
- **Python**: 3.10
- **TensorRT-LLM**: 0.8.0 (via NGC container or pip)

### Access Requirements

- **HuggingFace Account**: Required for gated models (Llama, etc.)
- **HuggingFace Token**: With access to target model repository
- **S3 Credentials**: For uploading model repository to ai-aas platform storage
- **AWS CLI**: Configured with ai-aas S3 credentials

### Verify GPU Access

```bash
# Check NVIDIA driver and CUDA version
nvidia-smi

# Expected output shows:
# - Driver Version: 535.x or later
# - CUDA Version: 12.2 or later
# - GPU name and memory

# Check compute capability
nvidia-smi --query-gpu=compute_cap --format=csv
# Should be >= 8.0 for TensorRT-LLM
```

## Build Environment Setup

### Option A: Using NGC Container (Recommended)

```bash
# Pull the official TensorRT-LLM container
docker pull nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3

# Create workspace directory
mkdir -p ~/tensorrt-llm-workspace
cd ~/tensorrt-llm-workspace

# Run container with GPU access
docker run --gpus all --rm -it \
  -v $(pwd):/workspace \
  -v ~/.cache/huggingface:/root/.cache/huggingface \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:24.04-trtllm-python-py3 \
  /bin/bash

# Inside container, verify TensorRT-LLM installation
python3 -c "import tensorrt_llm; print(tensorrt_llm.__version__)"
```

### Option B: Native Installation

```bash
# Install TensorRT-LLM via pip
pip install tensorrt_llm==0.8.0 --extra-index-url https://pypi.nvidia.com

# Install dependencies
pip install transformers==4.38.2 accelerate==0.27.2 sentencepiece==0.2.0

# Verify installation
python3 -c "import tensorrt_llm; print(tensorrt_llm.__version__)"
```

## Building Llama 3.1 8B Instruct Engine

This section uses **Llama 3.1 8B Instruct** as a complete example.

### Step 1: Download HuggingFace Checkpoint

```bash
# Set HuggingFace token
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxx"

# Create workspace
mkdir -p /workspace/llama-3.1-8b
cd /workspace/llama-3.1-8b

# Download model from HuggingFace
huggingface-cli login --token $HF_TOKEN
huggingface-cli download meta-llama/Llama-3.1-8B-Instruct \
  --local-dir ./hf_checkpoint \
  --local-dir-use-symlinks False

# Verify download
ls -lh ./hf_checkpoint/
# Should contain: config.json, tokenizer.json, model-*.safetensors, etc.
```

### Step 2: Convert Checkpoint to TensorRT-LLM Format

```bash
# Convert HuggingFace checkpoint to TensorRT-LLM format
python3 /opt/tritonserver/backends/tensorrtllm/examples/llama/convert_checkpoint.py \
  --model_dir ./hf_checkpoint \
  --output_dir ./trtllm_checkpoint \
  --dtype float16 \
  --tp_size 1 \
  --pp_size 1

# Parameters explained:
# --dtype float16        : Use FP16 precision (faster, less memory)
# --tp_size 1            : Tensor parallelism size (1 GPU)
# --pp_size 1            : Pipeline parallelism size (no pipelining)

# For multi-GPU setups:
# --tp_size 2            : Use 2 GPUs with tensor parallelism
# --tp_size 4            : Use 4 GPUs with tensor parallelism

# Verify conversion
ls -lh ./trtllm_checkpoint/
# Should contain: config.json, rank0.safetensors
```

### Step 3: Build TensorRT Engine

```bash
# Build the optimized TensorRT engine
trtllm-build \
  --checkpoint_dir ./trtllm_checkpoint \
  --output_dir ./trtllm_engine \
  --gemm_plugin float16 \
  --gpt_attention_plugin float16 \
  --max_batch_size 64 \
  --max_input_len 8192 \
  --max_output_len 2048 \
  --max_beam_width 1 \
  --remove_input_padding enable \
  --context_fmha enable \
  --paged_kv_cache enable \
  --use_fused_mlp \
  --multiple_profiles enable

# Parameters explained:
# --max_batch_size 64         : Maximum concurrent requests
# --max_input_len 8192        : Maximum input tokens
# --max_output_len 2048       : Maximum output tokens
# --max_beam_width 1          : Greedy decoding (no beam search)
# --remove_input_padding      : Remove padding for efficiency
# --context_fmha              : Use Flash Attention for context phase
# --paged_kv_cache            : Enable paged KV cache (like vLLM's PagedAttention)
# --use_fused_mlp             : Fuse MLP operations for speed
# --multiple_profiles         : Support variable batch sizes

# Build time: ~10-15 minutes on A100
# Engine is GPU-architecture specific - must rebuild for different GPUs!

# Verify engine build
ls -lh ./trtllm_engine/
# Should contain: rank0.engine (large binary file, ~8-16GB)
```

### Step 4: Create Triton Model Repository Structure

The Triton model repository follows a specific directory structure with ensemble, preprocessing, postprocessing, and tensorrt_llm models.

```bash
# Create model repository structure
mkdir -p model_repository/{ensemble,preprocessing,postprocessing,tensorrt_llm}
cd model_repository

# Create version directories
mkdir -p preprocessing/1
mkdir -p postprocessing/1
mkdir -p tensorrt_llm/1

# Copy TensorRT engine to repository
cp ../trtllm_engine/rank0.engine tensorrt_llm/1/

# Copy tokenizer files
mkdir -p tensorrt_llm/1/tokenizer
cp ../hf_checkpoint/tokenizer.json tensorrt_llm/1/tokenizer/
cp ../hf_checkpoint/tokenizer_config.json tensorrt_llm/1/tokenizer/
cp ../hf_checkpoint/special_tokens_map.json tensorrt_llm/1/tokenizer/

# Verify structure
tree -L 3 .
# Should show:
# .
# ├── ensemble/
# │   └── config.pbtxt
# ├── preprocessing/
# │   ├── 1/
# │   │   └── model.py
# │   └── config.pbtxt
# ├── postprocessing/
# │   ├── 1/
# │   │   └── model.py
# │   └── config.pbtxt
# └── tensorrt_llm/
#     ├── 1/
#     │   ├── rank0.engine
#     │   └── tokenizer/
#     └── config.pbtxt
```

### Step 5: Create Triton Configuration Files

#### 5.1 Ensemble Model Configuration

Create `ensemble/config.pbtxt`:

```protobuf
name: "ensemble"
platform: "ensemble"
max_batch_size: 64

input [
  {
    name: "text_input"
    data_type: TYPE_STRING
    dims: [ -1 ]
  },
  {
    name: "max_tokens"
    data_type: TYPE_INT32
    dims: [ -1 ]
    optional: true
  },
  {
    name: "temperature"
    data_type: TYPE_FP32
    dims: [ -1 ]
    optional: true
  },
  {
    name: "top_p"
    data_type: TYPE_FP32
    dims: [ -1 ]
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

ensemble_scheduling {
  step [
    {
      model_name: "preprocessing"
      model_version: 1
      input_map {
        key: "QUERY"
        value: "text_input"
      }
      output_map {
        key: "INPUT_ID"
        value: "input_ids"
      }
      output_map {
        key: "REQUEST_INPUT_LEN"
        value: "input_lengths"
      }
    },
    {
      model_name: "tensorrt_llm"
      model_version: 1
      input_map {
        key: "input_ids"
        value: "input_ids"
      }
      input_map {
        key: "input_lengths"
        value: "input_lengths"
      }
      input_map {
        key: "request_output_len"
        value: "max_tokens"
      }
      input_map {
        key: "temperature"
        value: "temperature"
      }
      input_map {
        key: "runtime_top_p"
        value: "top_p"
      }
      output_map {
        key: "output_ids"
        value: "output_ids"
      }
    },
    {
      model_name: "postprocessing"
      model_version: 1
      input_map {
        key: "TOKENS_BATCH"
        value: "output_ids"
      }
      output_map {
        key: "OUTPUT"
        value: "text_output"
      }
    }
  ]
}
```

#### 5.2 Preprocessing Model Configuration

Create `preprocessing/config.pbtxt`:

```protobuf
name: "preprocessing"
backend: "python"
max_batch_size: 64

input [
  {
    name: "QUERY"
    data_type: TYPE_STRING
    dims: [ -1 ]
  }
]

output [
  {
    name: "INPUT_ID"
    data_type: TYPE_INT32
    dims: [ -1 ]
  },
  {
    name: "REQUEST_INPUT_LEN"
    data_type: TYPE_INT32
    dims: [ 1 ]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_CPU
  }
]

dynamic_batching {
  preferred_batch_size: [ 8, 16, 32, 64 ]
  max_queue_delay_microseconds: 100000
}
```

Create `preprocessing/1/model.py`:

```python
import json
import os
import numpy as np
import triton_python_backend_utils as pb_utils
from transformers import AutoTokenizer

class TritonPythonModel:
    def initialize(self, args):
        # The 'model_repository_path' is the path to the directory containing this model (e.g. /mnt/models/preprocessing).
        # We construct a relative path to the tokenizer directory.
        tokenizer_path = os.path.join(args['model_repository'], '..', 'tensorrt_llm', '1', 'tokenizer')
        self.tokenizer = AutoTokenizer.from_pretrained(
            tokenizer_path,
            trust_remote_code=True
        )
        self.tokenizer.pad_token = self.tokenizer.eos_token

    def execute(self, requests):
        responses = []
        for request in requests:
            query = pb_utils.get_input_tensor_by_name(request, "QUERY")
            query_text = query.as_numpy()[0].decode('utf-8')

            # Tokenize
            input_ids = self.tokenizer.encode(query_text, return_tensors="np")
            input_lengths = np.array([[len(input_ids[0])]], dtype=np.int32)

            # Create output tensors
            input_id_tensor = pb_utils.Tensor("INPUT_ID", input_ids)
            input_len_tensor = pb_utils.Tensor("REQUEST_INPUT_LEN", input_lengths)

            inference_response = pb_utils.InferenceResponse(
                output_tensors=[input_id_tensor, input_len_tensor]
            )
            responses.append(inference_response)

        return responses
```

#### 5.3 Postprocessing Model Configuration

Create `postprocessing/config.pbtxt`:

```protobuf
name: "postprocessing"
backend: "python"
max_batch_size: 64

input [
  {
    name: "TOKENS_BATCH"
    data_type: TYPE_INT32
    dims: [ -1, -1 ]
  }
]

output [
  {
    name: "OUTPUT"
    data_type: TYPE_STRING
    dims: [ -1 ]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_CPU
  }
]

dynamic_batching {
  preferred_batch_size: [ 8, 16, 32, 64 ]
  max_queue_delay_microseconds: 100000
}
```

Create `postprocessing/1/model.py`:

```python
import os
import numpy as np
import triton_python_backend_utils as pb_utils
from transformers import AutoTokenizer

class TritonPythonModel:
    def initialize(self, args):
        # The 'model_repository' is the path to the directory containing this model (e.g. /mnt/models/postprocessing).
        # We construct a relative path to the tokenizer directory.
        tokenizer_path = os.path.join(args['model_repository'], '..', 'tensorrt_llm', '1', 'tokenizer')
        self.tokenizer = AutoTokenizer.from_pretrained(
            tokenizer_path,
            trust_remote_code=True
        )

    def execute(self, requests):
        responses = []
        for request in requests:
            tokens = pb_utils.get_input_tensor_by_name(request, "TOKENS_BATCH")
            tokens_batch = tokens.as_numpy()

            # Decode tokens
            outputs = []
            for tokens in tokens_batch:
                output_text = self.tokenizer.decode(tokens, skip_special_tokens=True)
                outputs.append(output_text.encode('utf-8'))

            # Create output tensor
            output_tensor = pb_utils.Tensor("OUTPUT", np.array(outputs, dtype=object))

            inference_response = pb_utils.InferenceResponse(
                output_tensors=[output_tensor]
            )
            responses.append(inference_response)

        return responses
```

#### 5.4 TensorRT-LLM Model Configuration

Create `tensorrt_llm/config.pbtxt`:

```protobuf
name: "tensorrt_llm"
backend: "tensorrtllm"
max_batch_size: 64

model_transaction_policy {
  decoupled: False
}

input [
  {
    name: "input_ids"
    data_type: TYPE_INT32
    dims: [ -1 ]
  },
  {
    name: "input_lengths"
    data_type: TYPE_INT32
    dims: [ 1 ]
    reshape: { shape: [ ] }
  },
  {
    name: "request_output_len"
    data_type: TYPE_INT32
    dims: [ -1 ]
  },
  {
    name: "temperature"
    data_type: TYPE_FP32
    dims: [ -1 ]
    optional: true
  },
  {
    name: "runtime_top_p"
    data_type: TYPE_FP32
    dims: [ -1 ]
    optional: true
  }
]

output [
  {
    name: "output_ids"
    data_type: TYPE_INT32
    dims: [ -1, -1 ]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_GPU
    gpus: [ 0 ]
  }
]

parameters: {
  key: "max_beam_width"
  value: {
    string_value: "1"
  }
}

parameters: {
  key: "FORCE_CPU_ONLY_INPUT_TENSORS"
  value: {
    string_value: "no"
  }
}

parameters: {
  key: "gpt_model_type"
  value: {
    string_value: "inflight_fused_batching"
  }
}

parameters: {
  key: "gpt_model_path"
  value: {
    string_value: "/opt/tritonserver/backends/tensorrtllm/tensorrt_llm/1/rank0.engine"
  }
}

parameters: {
  key: "max_tokens_in_paged_kv_cache"
  value: {
    string_value: "524288"
  }
}

parameters: {
  key: "batch_scheduler_policy"
  value: {
    string_value: "max_utilization"
  }
}
```

### Step 6: Validate Model Repository (CRITICAL)

**CRITICAL**: Before uploading, validate that no hardcoded build-time paths exist in model files. Hardcoded paths cause pod crashes when deployed to different environments.

```bash
cd /workspace/llama-3.1-8b/model_repository

# Check for hardcoded absolute paths in config files
echo "=== Checking for hardcoded paths ==="
grep -r "/workspace\|/home\|/tmp\|/opt" --include="*.pbtxt" --include="*.json" --include="*.py" .

# If any matches found, they MUST be fixed before upload!
# Common issues:
# - tokenizer_dir pointing to build machine path
# - model_path using absolute paths
# - Python scripts with hardcoded imports

# Validate tokenizer paths in config.pbtxt files
echo "=== Checking tokenizer_dir settings ==="
grep -r "tokenizer_dir\|tokenizer_path" --include="*.pbtxt" .

# The tokenizer_dir should be RELATIVE, e.g.:
#   string_value: "${model_repository}/tensorrt_llm/1/tokenizer"
# NOT absolute, e.g.:
#   string_value: "/workspace/llama-3.1-8b/tokenizer"  # WRONG!

# If hardcoded paths found, fix them:
# 1. Edit config.pbtxt files to use relative paths
# 2. Use ${model_repository} variable for Triton paths
# 3. Re-run this validation
```

**Expected output**: No matches (empty output) means validation passed.

Related bug: aas-p8n (hardcoded paths caused pod crashes)

### Step 7: Upload to S3

```bash
# Set S3 bucket and path
export S3_BUCKET="ai-aas-models"
export MODEL_PATH="triton/llama-3.1-8b-instruct-trtllm"

# Configure AWS CLI (if not already configured)
aws configure
# Enter:
# - AWS Access Key ID
# - AWS Secret Access Key
# - Default region (e.g., us-east-1)

# Upload model repository to S3
cd /workspace/llama-3.1-8b
aws s3 sync ./model_repository s3://${S3_BUCKET}/${MODEL_PATH}/ \
  --exclude "*.pyc" \
  --exclude "__pycache__/*"

# Verify upload
aws s3 ls s3://${S3_BUCKET}/${MODEL_PATH}/ --recursive

# Expected output:
# ensemble/config.pbtxt
# preprocessing/1/model.py
# preprocessing/config.pbtxt
# postprocessing/1/model.py
# postprocessing/config.pbtxt
# tensorrt_llm/1/rank0.engine
# tensorrt_llm/1/tokenizer/tokenizer.json
# tensorrt_llm/config.pbtxt
```

## Verification

### Local Testing with Triton

```bash
# Run Triton server locally (inside NGC container)
tritonserver --model-repository=/workspace/llama-3.1-8b/model_repository

# In another terminal, test inference
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
      }
    ]
  }'

# Expected: JSON response with "text_output" containing the model's answer
```

### Health Checks

```bash
# Check server health
curl http://localhost:8000/v2/health/live
# Expected: 200 OK

curl http://localhost:8000/v2/health/ready
# Expected: 200 OK (once models are loaded)

# Check model status
curl http://localhost:8000/v2/models/ensemble/ready
# Expected: 200 OK if ensemble is loaded

# List all models
curl http://localhost:8000/v2/models
# Should show: ensemble, preprocessing, postprocessing, tensorrt_llm
```

### Platform Deployment

Once uploaded to S3, deploy via ai-aas platform:

```bash
# Create ModelRecipe (see infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml)
kubectl apply -f infra/model-recipes/llm/llama/llama-3.1-8b-instruct-trtllm.yaml

# Deploy model
ai-aas-cli model deploy create llama-3.1-8b-instruct-trtllm \
  --environment development \
  --gpu-count 1

# Check deployment status
ai-aas-cli model deploy list

# Test inference via platform API
curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
  -H "Authorization: Bearer ${API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama-3.1-8b-instruct-trtllm",
    "messages": [{"role": "user", "content": "Hello!"}],
    "max_tokens": 100
  }'
```

## Troubleshooting

### Build Failures

**Error: "CUDA out of memory during build"**

```bash
# Reduce max batch size
trtllm-build --max_batch_size 32 ...  # Instead of 64

# Or reduce max input/output lengths
trtllm-build --max_input_len 4096 --max_output_len 1024 ...
```

**Error: "Unsupported GPU architecture"**

```bash
# Check GPU compute capability
nvidia-smi --query-gpu=compute_cap --format=csv

# TensorRT-LLM requires 8.0+ (Ampere or newer)
# Rebuild on the target GPU architecture
```

### Runtime Failures

**Error: "Model failed to load in Triton"**

```bash
# Check Triton logs
kubectl logs -n system -l serving.kserve.io/inferenceservice=llama-3.1-8b-instruct-trtllm

# Common issues:
# 1. Missing tokenizer files in tensorrt_llm/1/tokenizer/
# 2. Incorrect paths in config.pbtxt
# 3. Engine built on different GPU architecture
```

**Error: "Inference timeout"**

```bash
# Increase startup probe timeout in ModelRecipe
healthCheck:
  startupProbeSeconds: 900  # Increase from 600 to 900

# Large models may take 10-15 minutes to load
```

### Performance Issues

**Low throughput compared to vLLM**

```bash
# Check batch size utilization
curl http://localhost:8002/metrics | grep batch

# Tune dynamic batching parameters in config.pbtxt:
dynamic_batching {
  preferred_batch_size: [ 16, 32, 64 ]  # Add more sizes
  max_queue_delay_microseconds: 50000   # Reduce delay
}
```

## Model-Specific Notes

### Llama 3.1 8B Instruct

- **GPU Memory**: ~16GB VRAM (FP16), ~22GB with KV cache overhead
- **Build Time**: 10-15 minutes on A100
- **Startup Time**: 5-8 minutes to load engine
- **Throughput**: ~2000-3000 tokens/sec on A100

### Llama 3.1 70B Instruct

- **GPU Memory**: ~140GB VRAM (requires 2x A100 80GB or 4x A100 40GB)
- **Build Command**:
  ```bash
  trtllm-build --tp_size 2 --max_batch_size 32 ...  # For 2 GPUs
  ```
- **Build Time**: 30-45 minutes
- **Startup Time**: 15-20 minutes

### Mistral 7B Instruct

- **GPU Memory**: ~14GB VRAM (FP16)
- **Build Time**: 8-12 minutes on A100
- Same process as Llama, use `mistralai/Mistral-7B-Instruct-v0.3` checkpoint

## References

- [TensorRT-LLM Documentation](https://github.com/NVIDIA/TensorRT-LLM)
- [Triton Inference Server Docs](https://github.com/triton-inference-server)
- [TensorRT-LLM Backend Guide](https://github.com/triton-inference-server/tensorrtllm_backend)
- [Spec 029: TensorRT-LLM/Triton Support](../../specs/029-triton-tensorrt-llm/spec.md)
- [AI Model Operator Guide](../operators/ai-model-operator-guide.md)

## Appendix: Quick Reference

### Build Command Template

```bash
trtllm-build \
  --checkpoint_dir ./trtllm_checkpoint \
  --output_dir ./trtllm_engine \
  --gemm_plugin float16 \
  --gpt_attention_plugin float16 \
  --max_batch_size <BATCH_SIZE> \
  --max_input_len <INPUT_LEN> \
  --max_output_len <OUTPUT_LEN> \
  --max_beam_width 1 \
  --remove_input_padding enable \
  --context_fmha enable \
  --paged_kv_cache enable \
  --use_fused_mlp \
  --multiple_profiles enable
```

### Directory Structure Template

```
model_repository/
├── ensemble/
│   └── config.pbtxt              # Ensemble orchestration
├── preprocessing/
│   ├── 1/
│   │   └── model.py              # Tokenization
│   └── config.pbtxt
├── postprocessing/
│   ├── 1/
│   │   └── model.py              # Detokenization
│   └── config.pbtxt
└── tensorrt_llm/
    ├── 1/
    │   ├── rank0.engine          # TensorRT engine (main artifact)
    │   └── tokenizer/            # Tokenizer files from HF
    │       ├── tokenizer.json
    │       ├── tokenizer_config.json
    │       └── special_tokens_map.json
    └── config.pbtxt
```

### S3 Upload Command

```bash
aws s3 sync ./model_repository s3://ai-aas-models/triton/<model-name>/ \
  --exclude "*.pyc" --exclude "__pycache__/*"
```
