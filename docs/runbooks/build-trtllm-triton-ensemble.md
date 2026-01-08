# Build TensorRT-LLM Engines with Triton Ensemble Structure

**Created**: 2026-01-07
**Related Bead**: aas-nslza
**Target**: Multi-model deployment on RTX PRO 6000 Blackwell

## Overview

This runbook documents building TensorRT-LLM engines with the **full Triton ensemble structure** required by the TensorRT-LLM backend in Triton Inference Server.

**Why Ensemble Structure?**

The Triton TensorRT-LLM backend (in containers 25.06+) requires:
- **preprocessing**: Tokenizes input text → token IDs
- **tensorrt_llm**: Runs inference on token IDs
- **postprocessing**: Decodes output tokens → text
- **ensemble**: Orchestrates the three models

The simplified `LLM().save()` API outputs only the engine + tokenizer, which is incompatible with the Triton backend.

## Directory Structure

```
model_repository/
├── ensemble/
│   ├── config.pbtxt
│   └── 1/
│       └── .placeholder
├── preprocessing/
│   ├── config.pbtxt
│   └── 1/
│       └── model.py
├── tensorrt_llm/
│   ├── config.pbtxt
│   └── 1/
│       ├── rank0.engine
│       ├── config.json
│       ├── tokenizer.json
│       └── tokenizer_config.json
└── postprocessing/
    ├── config.pbtxt
    └── 1/
        └── model.py
```

## Prerequisites

### SSH Access
```bash
ssh root@172.236.157.4  # Blackwell build server
```

### Credentials
- HuggingFace token: `$HF_TOKEN` (from secrets/env/.env)
- S3 credentials (from secrets/env/.env)

### Container
```bash
docker pull nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3
```

## Step 1: Build TRT-LLM Engine

The engine build process uses the TensorRT-LLM Python API.

```bash
mkdir -p /root/trtllm-ensemble-build
cd /root/trtllm-ensemble-build

# Set HuggingFace token
export HF_TOKEN="hf_xxxxxxxxxx"

# Create build script
cat > build_engine.py << 'SCRIPT'
#!/usr/bin/env python3
"""
Build TensorRT-LLM engine for Triton deployment.
"""
import os
from tensorrt_llm import LLM, BuildConfig

MODEL_ID = "meta-llama/Llama-3.1-8B-Instruct"
OUTPUT_DIR = "/workspace/tensorrt_llm/1"

def main():
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    print("=" * 60)
    print(f"Building TensorRT-LLM Engine")
    print(f"Model: {MODEL_ID}")
    print(f"Output: {OUTPUT_DIR}")
    print("=" * 60)

    # Build configuration for multi-model deployment
    # Reduced batch size and KV cache for sharing GPU with other models
    build_config = BuildConfig()
    build_config.max_batch_size = 32
    build_config.max_input_len = 4096
    build_config.max_seq_len = 6144
    build_config.max_num_tokens = 4096

    print(f"\nConfiguration:")
    print(f"  Max Batch Size: {build_config.max_batch_size}")
    print(f"  Max Input Length: {build_config.max_input_len}")
    print(f"  Max Sequence Length: {build_config.max_seq_len}")

    print("\nBuilding engine...")
    llm = LLM(
        model=MODEL_ID,
        build_config=build_config,
    )

    print(f"\nSaving engine to {OUTPUT_DIR}...")
    llm.save(OUTPUT_DIR)

    print("\nBuild complete!")

if __name__ == "__main__":
    main()
SCRIPT

chmod +x build_engine.py

# Run build inside container
docker run --gpus all --rm \
  -v /root/trtllm-ensemble-build:/workspace \
  -v /root/.cache/huggingface:/root/.cache/huggingface \
  -e HF_TOKEN=$HF_TOKEN \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3 \
  python3 /workspace/build_engine.py 2>&1 | tee build.log
```

## Step 2: Create Triton Model Repository

### Create directory structure

```bash
cd /root/trtllm-ensemble-build

# Create directories
mkdir -p model_repository/{ensemble/1,preprocessing/1,postprocessing/1}

# Move tensorrt_llm engine (already created by build script)
# Note: tensorrt_llm/1 already exists from build_engine.py
```

### Create preprocessing/1/model.py

```bash
cat > model_repository/preprocessing/1/model.py << 'PYTHON'
import json
import numpy as np
import triton_python_backend_utils as pb_utils
from transformers import AutoTokenizer

class TritonPythonModel:
    def initialize(self, args):
        model_config = json.loads(args["model_config"])
        tokenizer_dir = model_config["parameters"]["tokenizer_dir"]["string_value"]
        self.tokenizer = AutoTokenizer.from_pretrained(tokenizer_dir, trust_remote_code=True)

    def execute(self, requests):
        responses = []
        for request in requests:
            query = pb_utils.get_input_tensor_by_name(request, "QUERY").as_numpy()
            request_output_len = pb_utils.get_input_tensor_by_name(request, "REQUEST_OUTPUT_LEN").as_numpy()

            # Decode query and tokenize
            text = query[0][0].decode("utf-8")
            input_ids = self.tokenizer.encode(text, add_special_tokens=True)

            input_id = np.array([input_ids], dtype=np.int32)
            input_len = np.array([[len(input_ids)]], dtype=np.int32)

            out_input_id = pb_utils.Tensor("INPUT_ID", input_id)
            out_input_len = pb_utils.Tensor("REQUEST_INPUT_LEN", input_len)
            out_output_len = pb_utils.Tensor("REQUEST_OUTPUT_LEN", request_output_len)

            responses.append(pb_utils.InferenceResponse([out_input_id, out_input_len, out_output_len]))

        return responses

    def finalize(self):
        pass
PYTHON
```

### Create postprocessing/1/model.py

```bash
cat > model_repository/postprocessing/1/model.py << 'PYTHON'
import json
import numpy as np
import triton_python_backend_utils as pb_utils
from transformers import AutoTokenizer

class TritonPythonModel:
    def initialize(self, args):
        model_config = json.loads(args["model_config"])
        tokenizer_dir = model_config["parameters"]["tokenizer_dir"]["string_value"]
        self.tokenizer = AutoTokenizer.from_pretrained(tokenizer_dir, trust_remote_code=True)

    def execute(self, requests):
        responses = []
        for request in requests:
            tokens_batch = pb_utils.get_input_tensor_by_name(request, "TOKENS_BATCH").as_numpy()
            sequence_length = pb_utils.get_input_tensor_by_name(request, "SEQUENCE_LENGTH").as_numpy()

            # Flatten sequence_length if needed
            if sequence_length.ndim == 0:
                sequence_length = np.array([sequence_length.item()])
            sequence_length = sequence_length.flatten()

            # Handle different token batch shapes
            if tokens_batch.ndim == 3:
                tokens_batch = tokens_batch[:, 0, :]  # Take first beam

            outputs = []
            for i in range(len(tokens_batch)):
                tokens = tokens_batch[i]
                seq_len = int(sequence_length[i]) if i < len(sequence_length) else int(sequence_length[0])
                output_ids = [int(t) for t in tokens[:seq_len]]
                output_text = self.tokenizer.decode(output_ids, skip_special_tokens=True)
                outputs.append(output_text)

            output = np.array(outputs, dtype=object)
            out_tensor = pb_utils.Tensor("OUTPUT", output)

            responses.append(pb_utils.InferenceResponse([out_tensor]))

        return responses

    def finalize(self):
        pass
PYTHON
```

### Create ensemble placeholder

```bash
touch model_repository/ensemble/1/.placeholder
```

## Step 3: Create Config Files

### preprocessing/config.pbtxt

```bash
cat > model_repository/preprocessing/config.pbtxt << 'CONFIG'
name: "preprocessing"
backend: "python"
max_batch_size: 32

input [
  {
    name: "QUERY"
    data_type: TYPE_STRING
    dims: [1]
  },
  {
    name: "REQUEST_OUTPUT_LEN"
    data_type: TYPE_INT32
    dims: [1]
  }
]

output [
  {
    name: "INPUT_ID"
    data_type: TYPE_INT32
    dims: [-1]
  },
  {
    name: "REQUEST_INPUT_LEN"
    data_type: TYPE_INT32
    dims: [1]
  },
  {
    name: "REQUEST_OUTPUT_LEN"
    data_type: TYPE_INT32
    dims: [1]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_CPU
  }
]

parameters: {
  key: "tokenizer_dir"
  value: {
    string_value: "/mnt/models/tensorrt_llm/1"
  }
}

parameters: {
  key: "tokenizer_type"
  value: {
    string_value: "auto"
  }
}
CONFIG
```

### tensorrt_llm/config.pbtxt

```bash
cat > model_repository/tensorrt_llm/config.pbtxt << 'CONFIG'
name: "tensorrt_llm"
backend: "tensorrtllm"
max_batch_size: 32

model_transaction_policy {
  decoupled: False
}

# NOTE: decoupled: False is required for HTTP endpoint compatibility.
# Set to True only if using gRPC streaming exclusively.

input [
  {
    name: "input_ids"
    data_type: TYPE_INT32
    dims: [-1]
  },
  {
    name: "input_lengths"
    data_type: TYPE_INT32
    dims: [1]
    reshape: { shape: [] }
  },
  {
    name: "request_output_len"
    data_type: TYPE_INT32
    dims: [1]
  },
  {
    name: "end_id"
    data_type: TYPE_INT32
    dims: [1]
    reshape: { shape: [] }
    optional: true
  },
  {
    name: "pad_id"
    data_type: TYPE_INT32
    dims: [1]
    reshape: { shape: [] }
    optional: true
  },
  {
    name: "beam_width"
    data_type: TYPE_INT32
    dims: [1]
    reshape: { shape: [] }
    optional: true
  },
  {
    name: "temperature"
    data_type: TYPE_FP32
    dims: [1]
    reshape: { shape: [] }
    optional: true
  },
  {
    name: "runtime_top_k"
    data_type: TYPE_INT32
    dims: [1]
    reshape: { shape: [] }
    optional: true
  },
  {
    name: "runtime_top_p"
    data_type: TYPE_FP32
    dims: [1]
    reshape: { shape: [] }
    optional: true
  },
  {
    name: "streaming"
    data_type: TYPE_BOOL
    dims: [1]
    reshape: { shape: [] }
    optional: true
  }
]

output [
  {
    name: "output_ids"
    data_type: TYPE_INT32
    dims: [-1, -1]
  },
  {
    name: "sequence_length"
    data_type: TYPE_INT32
    dims: [-1]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_GPU
    gpus: [0]
  }
]

parameters: {
  key: "max_beam_width"
  value: {
    string_value: "1"
  }
}

parameters: {
  key: "batching_type"
  value: {
    string_value: "inflight_fused_batching"
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
    string_value: "/mnt/models/tensorrt_llm/1"
  }
}

# For multi-model deployment, reduce KV cache fraction
# Single model: 0.9, Three models: 0.28 each
parameters: {
  key: "kv_cache_free_gpu_mem_fraction"
  value: {
    string_value: "0.28"
  }
}

parameters: {
  key: "gpu_device_ids"
  value: {
    string_value: "0"
  }
}

parameters: {
  key: "use_custom_all_reduce"
  value: {
    string_value: "0"
  }
}

# Required for TRT-LLM 0.20.0+ (container 25.06+)
parameters: {
  key: "tokenizer_dir"
  value: {
    string_value: "/mnt/models/tensorrt_llm/1"
  }
}

parameters: {
  key: "xgrammar_tokenizer_info_path"
  value: {
    string_value: ""
  }
}

parameters: {
  key: "guided_decoding_backend"
  value: {
    string_value: "xgrammar"
  }
}

parameters: {
  key: "xgrammar_data_dir"
  value: {
    string_value: ""
  }
}

parameters: {
  key: "executor_worker_path"
  value: {
    string_value: ""
  }
}
CONFIG
```

### postprocessing/config.pbtxt

```bash
cat > model_repository/postprocessing/config.pbtxt << 'CONFIG'
name: "postprocessing"
backend: "python"
max_batch_size: 32

input [
  {
    name: "TOKENS_BATCH"
    data_type: TYPE_INT32
    dims: [-1, -1]
  },
  {
    name: "SEQUENCE_LENGTH"
    data_type: TYPE_INT32
    dims: [-1]
  }
]

output [
  {
    name: "OUTPUT"
    data_type: TYPE_STRING
    dims: [-1]
  }
]

instance_group [
  {
    count: 1
    kind: KIND_CPU
  }
]

parameters: {
  key: "tokenizer_dir"
  value: {
    string_value: "/mnt/models/tensorrt_llm/1"
  }
}

parameters: {
  key: "tokenizer_type"
  value: {
    string_value: "auto"
  }
}
CONFIG
```

### ensemble/config.pbtxt

```bash
cat > model_repository/ensemble/config.pbtxt << 'CONFIG'
name: "ensemble"
platform: "ensemble"
max_batch_size: 32

input [
  {
    name: "text_input"
    data_type: TYPE_STRING
    dims: [1]
  },
  {
    name: "max_tokens"
    data_type: TYPE_INT32
    dims: [1]
  }
]

output [
  {
    name: "text_output"
    data_type: TYPE_STRING
    dims: [-1]
  }
]

ensemble_scheduling {
  step [
    {
      model_name: "preprocessing"
      model_version: -1
      input_map {
        key: "QUERY"
        value: "text_input"
      }
      input_map {
        key: "REQUEST_OUTPUT_LEN"
        value: "max_tokens"
      }
      output_map {
        key: "INPUT_ID"
        value: "_INPUT_ID"
      }
      output_map {
        key: "REQUEST_INPUT_LEN"
        value: "_REQUEST_INPUT_LEN"
      }
      output_map {
        key: "REQUEST_OUTPUT_LEN"
        value: "_REQUEST_OUTPUT_LEN"
      }
    },
    {
      model_name: "tensorrt_llm"
      model_version: -1
      input_map {
        key: "input_ids"
        value: "_INPUT_ID"
      }
      input_map {
        key: "input_lengths"
        value: "_REQUEST_INPUT_LEN"
      }
      input_map {
        key: "request_output_len"
        value: "_REQUEST_OUTPUT_LEN"
      }
      output_map {
        key: "output_ids"
        value: "_TOKENS_BATCH"
      }
      output_map {
        key: "sequence_length"
        value: "_SEQUENCE_LENGTH"
      }
    },
    {
      model_name: "postprocessing"
      model_version: -1
      input_map {
        key: "TOKENS_BATCH"
        value: "_TOKENS_BATCH"
      }
      input_map {
        key: "SEQUENCE_LENGTH"
        value: "_SEQUENCE_LENGTH"
      }
      output_map {
        key: "OUTPUT"
        value: "text_output"
      }
    }
  ]
}
CONFIG
```

## Step 4: Test Locally

```bash
# Start Triton server locally
docker run --gpus all --rm -d --name triton-test \
  -v /root/trtllm-ensemble-build/model_repository:/mnt/models \
  -p 8000:8000 -p 8001:8001 -p 8002:8002 \
  --ipc=host --ulimit memlock=-1 --ulimit stack=67108864 \
  nvcr.io/nvidia/tritonserver:25.06-trtllm-python-py3 \
  tritonserver --model-repository=/mnt/models

# Wait for startup (~2-3 minutes)
sleep 180

# Check model status
curl http://localhost:8000/v2/models

# Test inference via ensemble
curl -X POST http://localhost:8000/v2/models/ensemble/infer \
  -H 'Content-Type: application/json' \
  -d '{
    "inputs": [
      {"name": "text_input", "shape": [1, 1], "datatype": "BYTES", "data": ["What is the capital of France?"]},
      {"name": "max_tokens", "shape": [1, 1], "datatype": "INT32", "data": [50]}
    ]
  }'

# Cleanup
docker rm -f triton-test
```

## Step 5: Upload to S3

```bash
# Configure rclone
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

# Upload model repository
rclone copy /root/trtllm-ensemble-build/model_repository \
  linode:ai-aas/models/blackwell/llama-3-1-8b-instruct-triton/ \
  --progress

# Verify upload
rclone ls linode:ai-aas/models/blackwell/llama-3-1-8b-instruct-triton/
```

## Step 6: Deploy via AIModel CR

Create AIModel CR in `ai-aas-config/environments/staging/models/`:

```yaml
apiVersion: aimodel.ai-aas.io/v1alpha1
kind: AIModel
metadata:
  name: llama-3-1-8b-instruct-triton-blackwell
  namespace: staging
  labels:
    app: triton-inference
    runtime: triton-blackwell
spec:
  modelName: llama-3-1-8b-instruct-triton-blackwell
  modelID: meta-llama/Llama-3.1-8B-Instruct
  s3Bucket: ai-aas
  s3Key: models/blackwell/llama-3-1-8b-instruct-triton
  enabled: true
  runtime: triton
  runtimeName: kserve-triton-blackwell
  minReplicas: 1
  maxReplicas: 1
  resources:
    requests:
      cpu: "8"
      memory: "32Gi"
      nvidia.com/gpu: "1"
    limits:
      cpu: "16"
      memory: "48Gi"
      nvidia.com/gpu: "1"
  nodeSelector:
    node.kubernetes.io/instance-type: g3-gpu-rtxpro6000-blackwell-1
  tolerations:
    - key: nvidia.com/gpu
      operator: Exists
      effect: NoSchedule
```

## Multi-Model Deployment

For deploying multiple models on one GPU, create separate model repositories and adjust the KV cache fraction:

| Models | KV Cache Fraction Each | Total |
|--------|------------------------|-------|
| 1 model | 0.90 | 0.90 |
| 2 models | 0.42 | 0.84 |
| 3 models | 0.28 | 0.84 |

**Important**: Each model needs its own complete ensemble structure. The `gpt_model_path` in `tensorrt_llm/config.pbtxt` must point to the correct model directory.

## Troubleshooting

### "Cannot find parameter with name: tokenizer_dir"
- Ensure `preprocessing/config.pbtxt` has the `tokenizer_dir` parameter
- Verify tokenizer files exist in `tensorrt_llm/1/`

### "Cannot find parameter with name: xgrammar_tokenizer_info_path"
- This parameter is required in newer TRT-LLM versions (0.20.0+)
- The ensemble structure bypasses this by using Python preprocessing

### Models fail to load
```bash
# Check Triton logs
kubectl logs -n staging -l serving.kserve.io/inferenceservice=<name> --tail=100

# Common issues:
# - Missing files in tensorrt_llm/1/
# - Wrong paths in config.pbtxt
# - KV cache fraction too high for multi-model
```

### Out of memory during inference
- Reduce `kv_cache_free_gpu_mem_fraction`
- Reduce `max_batch_size`

## References

- [TensorRT-LLM Triton Backend](https://github.com/triton-inference-server/tensorrtllm_backend)
- [Triton Ensemble Scheduling](https://github.com/triton-inference-server/server/blob/main/docs/user_guide/architecture.md#ensemble-models)
- [KServe InferenceService](https://kserve.github.io/website/latest/modelserving/v1beta1/triton/)
- Working Ada model: `s3://ai-aas/models/ada/llama-3.1-8b-instruct/trtllm-v1-fp8/`
