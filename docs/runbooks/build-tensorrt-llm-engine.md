# Build TensorRT-LLM Engine - Runbook

**Last Updated**: 2026-01-06
**Owner**: AI Platform Engineering
**Related Spec**: [specs/029-triton-tensorrt-llm](../../specs/029-triton-tensorrt-llm/)

## Overview

This runbook documents the process of building TensorRT-LLM engines from HuggingFace checkpoints for deployment with NVIDIA Triton Inference Server on the ai-aas platform.

TensorRT-LLM provides optimized inference kernels for NVIDIA GPUs, delivering:
- Higher throughput via in-flight batching
- Lower latency through kernel fusion
- Better GPU memory utilization
- Faster time-to-first-token

**IMPORTANT**: TensorRT engines are GPU-architecture specific. You must rebuild engines for each GPU architecture (e.g., Ada sm_89, Blackwell sm_120).

## Prerequisites

### Hardware Requirements

- **GPU**: NVIDIA GPU with Compute Capability 8.0+
  - Ada Lovelace (RTX 4090, L40S): sm_89
  - Hopper (H100): sm_90
  - **Blackwell (RTX 6000 Blackwell)**: sm_120 - requires container 25.06+
  - Minimum 24GB VRAM for Llama 3.1 8B
  - Minimum 48GB VRAM for larger models (13B+)
- **CPU**: 8+ cores recommended
- **RAM**: 64GB+ recommended
- **Storage**: 100GB+ free space for checkpoints and engines

### Software Requirements

- **Operating System**: Ubuntu 22.04 or 24.04 LTS
- **CUDA**: 12.4 or later (13.0 for Blackwell)
- **cuDNN**: 8.9.x or later
- **Docker**: 24.0+ with NVIDIA Container Toolkit
- **Python**: 3.10+
- **TensorRT-LLM**: 0.20.0+ (via NGC container)

### Container Version by GPU Architecture

| GPU Architecture | Compute Capability | Minimum Container | Recommended |
|------------------|-------------------|-------------------|-------------|
| Ada Lovelace | sm_89 | 24.04 | 24.08 |
| Hopper | sm_90 | 24.04 | 24.08 |
| **Blackwell** | sm_120 | **25.06** | **25.06+** |

**Note**: Containers before 25.06 will show "Blackwell not supported" warnings and fail to build engines.

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
# For Ada/Hopper:
docker pull nvcr.io/nvidia/tritonserver:24.08-trtllm-python-py3

# For Blackwell (REQUIRED - older containers don't support sm_120):
docker pull nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3

# Create workspace directory
mkdir -p ~/tensorrt-llm-workspace
cd ~/tensorrt-llm-workspace

# Run container with GPU access (use appropriate tag for your GPU)
# For Blackwell:
docker run --gpus all --rm -it \
  -v $(pwd):/workspace \
  -v ~/.cache/huggingface:/root/.cache/huggingface \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3 \
  /bin/bash

# Inside container, verify TensorRT-LLM installation
python3 -c "import tensorrt_llm; print(tensorrt_llm.__version__)"
# Expected: 0.20.0 or later for 25.06 container
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

### API Version Note

**TRT-LLM 0.20.0+ (container 25.06+)** uses a new high-level Python API. The old `convert_checkpoint.py` + `trtllm-build` workflow is deprecated. This runbook documents the new API.

### Step 1: Set Up HuggingFace Authentication

```bash
# Set HuggingFace token (required for gated models like Llama)
export HF_TOKEN="hf_xxxxxxxxxxxxxxxxxxxxxxxxxx"

# Login to HuggingFace
huggingface-cli login --token $HF_TOKEN
```

### Step 2: Build and Save TensorRT Engine (New API)

Create a build script inside the container:

```bash
cat > /workspace/build_engine.py << 'EOF'
from tensorrt_llm import LLM, SamplingParams, BuildConfig
import os

def main():
    os.makedirs("/workspace/engine_output", exist_ok=True)

    print("Building TensorRT-LLM engine...", flush=True)
    print("This will download the model and compile for the current GPU architecture.", flush=True)

    # Configure build parameters
    bc = BuildConfig()
    bc.max_batch_size = 64
    bc.max_input_len = 8192
    bc.max_seq_len = 10240  # max_input_len + max_output_len

    # Build engine - uses HuggingFace model ID directly
    # The LLM class handles checkpoint download, conversion, and engine building
    llm = LLM(model="meta-llama/Llama-3.1-8B-Instruct", build_config=bc)

    # Save the compiled engine to disk
    print("Saving engine to /workspace/engine_output...", flush=True)
    llm.save("/workspace/engine_output")

    print("Done! Engine saved to /workspace/engine_output", flush=True)

if __name__ == "__main__":
    main()
EOF
```

Run the build:

```bash
cd /workspace
python3 build_engine.py

# Build time: ~30 seconds on RTX 6000 Blackwell (sm_120)
# Build time: ~10-15 minutes on A100 (sm_80)

# Verify engine build
ls -lh /workspace/engine_output/
# Should contain:
# - rank0.engine (~16GB for Llama 3.1 8B)
# - config.json
# - tokenizer.json
# - tokenizer_config.json
# - special_tokens_map.json
```

### Step 3: Test the Saved Engine (Optional)

Verify the engine loads and runs correctly:

```bash
cat > /workspace/test_engine.py << 'EOF'
from tensorrt_llm import LLM, SamplingParams

def main():
    print("Loading saved engine...", flush=True)
    llm = LLM(model="/workspace/engine_output")

    print("Running inference test...", flush=True)
    prompts = ["What is the capital of France?"]
    sampling_params = SamplingParams(temperature=0.7, max_tokens=100)

    outputs = llm.generate(prompts, sampling_params)
    for output in outputs:
        print(f"Prompt: {output.prompt}")
        print(f"Generated: {output.outputs[0].text}")

if __name__ == "__main__":
    main()
EOF

python3 test_engine.py
```

### Legacy Build Method (TRT-LLM < 0.16.0)

<details>
<summary>Click to expand legacy build instructions for older containers (24.04-24.08)</summary>

For containers using TRT-LLM < 0.16.0, use the old checkpoint conversion workflow:

```bash
# Step 1: Download checkpoint
huggingface-cli download meta-llama/Llama-3.1-8B-Instruct \
  --local-dir ./hf_checkpoint \
  --local-dir-use-symlinks False

# Step 2: Convert checkpoint
python3 /opt/tritonserver/backends/tensorrtllm/examples/llama/convert_checkpoint.py \
  --model_dir ./hf_checkpoint \
  --output_dir ./trtllm_checkpoint \
  --dtype float16 \
  --tp_size 1 \
  --pp_size 1

# Step 3: Build engine
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
```

</details>

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

### Step 6: Upload to Object Storage

#### Using rclone (Recommended for Linode Object Storage)

**Why rclone?** AWS CLI has checksum compatibility issues with Linode Object Storage. rclone handles this correctly and supports large file uploads reliably.

```bash
# Install rclone
curl https://rclone.org/install.sh | bash

# Configure rclone for Linode Object Storage
mkdir -p ~/.config/rclone
cat > ~/.config/rclone/rclone.conf << 'EOF'
[linode]
type = s3
provider = Ceph
access_key_id = YOUR_ACCESS_KEY
secret_access_key = YOUR_SECRET_KEY
endpoint = https://fr-par-1.linodeobjects.com
acl = private
EOF

# Upload engine files
# For new high-level API output (engine_output directory):
rclone copy /workspace/engine_output \
  linode:ai-aas/models/llama-3.1-8b-instruct-trtllm-blackwell/ \
  --progress

# For traditional Triton ensemble structure:
rclone copy /workspace/model_repository \
  linode:ai-aas/models/llama-3.1-8b-instruct-trtllm/ \
  --progress \
  --exclude "*.pyc" \
  --exclude "__pycache__/**"

# Verify upload
rclone ls linode:ai-aas/models/llama-3.1-8b-instruct-trtllm-blackwell/
```

#### Using AWS CLI (for AWS S3 or compatible services)

**Note**: AWS CLI may have issues with Linode Object Storage due to checksum headers. Use rclone instead for Linode.

```bash
# Set S3 bucket and path
export S3_BUCKET="ai-aas-models"
export MODEL_PATH="triton/llama-3.1-8b-instruct-trtllm"

# Configure AWS CLI
aws configure
# Enter AWS Access Key ID, Secret Access Key, and region

# Upload model repository to S3
aws s3 sync ./model_repository s3://${S3_BUCKET}/${MODEL_PATH}/ \
  --exclude "*.pyc" \
  --exclude "__pycache__/*"

# Verify upload
aws s3 ls s3://${S3_BUCKET}/${MODEL_PATH}/ --recursive
```

#### Storage Paths

| GPU Architecture | Storage Path |
|------------------|--------------|
| Ada (sm_89) | `ai-aas/models/llama-3.1-8b-instruct-trtllm/` |
| Blackwell (sm_120) | `ai-aas/models/llama-3.1-8b-instruct-trtllm-blackwell/` |

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

#### On Ada (RTX 4090, L40S) - sm_89
- **GPU Memory**: ~16GB VRAM (FP16), ~22GB with KV cache overhead
- **Build Time**: 10-15 minutes
- **Startup Time**: 5-8 minutes to load engine
- **Throughput**: ~2000-3000 tokens/sec
- **Container**: 24.08-trtllm-python-py3

#### On Blackwell (RTX 6000 Blackwell) - sm_120
- **GPU Memory**: ~16GB VRAM of 96GB available
- **Build Time**: ~30 seconds (significantly faster than Ada/Hopper)
- **Engine Size**: ~16GB
- **Container**: 25.06-trtllm-python-py3 (REQUIRED)
- **Driver**: 580.x+ with CUDA 13.0
- **Storage Path**: `ai-aas/models/llama-3.1-8b-instruct-trtllm-blackwell/`

### Llama 3.1 70B Instruct

- **GPU Memory**: ~140GB VRAM (requires 2x A100 80GB or 4x A100 40GB)
- **Build Command** (new API):
  ```python
  bc = BuildConfig()
  bc.max_batch_size = 32
  # For tensor parallelism, set tp_size in LLM constructor
  llm = LLM(model="meta-llama/Llama-3.1-70B-Instruct",
            build_config=bc,
            tensor_parallel_size=2)
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

### Build Script Template (TRT-LLM 0.20.0+)

```python
from tensorrt_llm import LLM, SamplingParams, BuildConfig
import os

def main():
    os.makedirs("/workspace/engine_output", exist_ok=True)
    bc = BuildConfig()
    bc.max_batch_size = 64
    bc.max_input_len = 8192
    bc.max_seq_len = 10240

    llm = LLM(model="<MODEL_ID>", build_config=bc)
    llm.save("/workspace/engine_output")

if __name__ == "__main__":
    main()
```

### Legacy Build Command (TRT-LLM < 0.16.0)

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

### Engine Output Structure (New API)

```
engine_output/
├── config.json               # Engine configuration
├── rank0.engine              # TensorRT engine (~16GB)
├── tokenizer.json            # Tokenizer
├── tokenizer_config.json     # Tokenizer config
└── special_tokens_map.json   # Special tokens
```

### Triton Ensemble Structure (Legacy)

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

### Upload Commands

#### rclone (Recommended for Linode)

```bash
# Install
curl https://rclone.org/install.sh | bash

# Configure (~/.config/rclone/rclone.conf)
[linode]
type = s3
provider = Ceph
access_key_id = YOUR_KEY
secret_access_key = YOUR_SECRET
endpoint = https://fr-par-1.linodeobjects.com
acl = private

# Upload
rclone copy /workspace/engine_output linode:ai-aas/models/<model-name>/ --progress
```

#### AWS CLI (for AWS S3)

```bash
aws s3 sync ./model_repository s3://ai-aas-models/triton/<model-name>/ \
  --exclude "*.pyc" --exclude "__pycache__/*"
```

### Container Quick Reference

| GPU | Container Tag |
|-----|---------------|
| Ada (sm_89) | `nvcr.io/nvidia/tritonserver:24.08-trtllm-python-py3` |
| Hopper (sm_90) | `nvcr.io/nvidia/tritonserver:24.08-trtllm-python-py3` |
| **Blackwell (sm_120)** | `nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3` |
