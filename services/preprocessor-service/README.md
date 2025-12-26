# Preprocessor Service

Universal inference pre-processing service that applies model-specific chat templates using HuggingFace AutoTokenizer.

## Overview

This service sits between the API Router and inference backends (Triton, vLLM) to apply model-specific formatting:

```
Client → api-router-service → [preprocessor-service] → api-router-service → Triton/vLLM
```

### Features

- **Chat Template Application**: Uses HuggingFace `apply_chat_template()` for 100% compatibility with model-specific Jinja2 templates
- **Multiple Output Modes**:
  - `vllm` / `triton_string`: Returns formatted prompt string
  - `triton_tensor`: Returns tokenized input IDs for direct tensor input
- **Tokenizer Caching**: LRU cache with configurable size and pre-loading
- **gRPC Interface**: Low-latency, streaming-ready protocol

### Supported Models (MVP)

| Model | Format | Example Output |
|-------|--------|----------------|
| `meta-llama/Llama-3.1-8B-Instruct` | Llama-3 | `<\|begin_of_text\|><\|start_header_id\|>user...` |
| `openai/gpt-oss-20b` | Harmony | `<\|start\|>user\n...` |

## Quick Start

### Local Development

```bash
# Install dependencies
pip install -r requirements.txt

# Generate proto code
python -m grpc_tools.protoc -I./proto --python_out=./src/preprocessor --grpc_python_out=./src/preprocessor ./proto/preprocessor.proto

# Run server
python -m preprocessor.main
```

### Docker Build

```bash
docker build -t preprocessor-service:latest .
docker run -p 50051:50051 preprocessor-service:latest
```

### Testing with grpcurl

```bash
# Health check
grpcurl -plaintext localhost:50051 preprocessor.PreprocessorService/HealthCheck

# Preprocess request
grpcurl -plaintext -d '{
  "model_id": "meta-llama/Llama-3.1-8B-Instruct",
  "engine_type": "vllm",
  "messages": [{"role": "user", "content": "Hello!"}]
}' localhost:50051 preprocessor.PreprocessorService/Preprocess
```

## Configuration

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `GRPC_PORT` | `50051` | gRPC server port |
| `GRPC_MAX_WORKERS` | `10` | Max worker threads |
| `LOG_LEVEL` | `INFO` | Log level (DEBUG, INFO, WARNING, ERROR) |
| `LOG_FORMAT` | `json` | Log format (json, console) |
| `HF_HOME` | `/app/hf_cache` | HuggingFace cache directory |
| `HF_TOKEN` | - | HuggingFace API token (for private models) |
| `MAX_CACHED_TOKENIZERS` | `50` | Max tokenizers in memory |
| `PRELOAD_MODELS` | `meta-llama/Llama-3.1-8B-Instruct` | Models to preload (comma-separated) |

## API Reference

### PreprocessorService

#### Preprocess

Applies model-specific chat template to messages.

**Request:**
```protobuf
message PreprocessRequest {
  string model_id = 1;           // HuggingFace model ID
  string engine_type = 2;        // "vllm", "triton_string", "triton_tensor"
  repeated Message messages = 3;
  int32 max_tokens = 10;
}
```

**Response:**
```protobuf
message PreprocessResponse {
  string formatted_prompt = 1;   // For string modes
  repeated int32 input_ids = 2;  // For tensor mode
  int32 prompt_token_count = 5;
}
```

#### HealthCheck

Returns service health and list of loaded tokenizers.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    preprocessor-service                      │
├─────────────────────────────────────────────────────────────┤
│  main.py              │  gRPC server entry point            │
│  service.py           │  PreprocessorServicer implementation│
│  template_engine.py   │  HuggingFace template application   │
│  tokenizer_cache.py   │  LRU cache for tokenizers           │
│  config.py            │  Environment configuration          │
└─────────────────────────────────────────────────────────────┘
```

## Development

### Run Tests

```bash
pytest tests/ -v
```

### Lint

```bash
ruff check src/
mypy src/
```

## Deployment

Deploy via Helm chart and ArgoCD:

```bash
# ArgoCD will auto-sync from gitops/clusters/development/apps/preprocessor-service.yaml
```

See `deployments/helm/preprocessor-service/` for Helm chart configuration.
