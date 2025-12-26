# Inference Routing Architecture

This document describes the inference routing architecture in the AI-AAS platform, covering how requests are routed from the OpenAI-compatible API to backend inference engines.

## Overview

The API Router Service acts as the gateway for all inference requests, translating OpenAI-compatible API calls to the appropriate backend inference engine protocol.

```
Client Request (OpenAI API)
         │
         ▼
   ┌─────────────┐
   │ API Router  │
   │   Service   │
   └─────────────┘
         │
    ┌────┴────┐
    │Routing  │
    │ Policy  │
    └────┬────┘
         │
    ┌────┴─────────────────────────┐
    │                              │
    ▼                              ▼
┌─────────┐                ┌────────────────┐
│  vLLM   │                │ TRT-LLM/Triton │
│ (HTTP)  │                │   (gRPC)       │
│ :8000   │                │   :9000        │
└─────────┘                └────────────────┘
```

## Inference Engines

### vLLM (OpenAI Backend Type)

vLLM provides a native OpenAI-compatible HTTP API, making it the simplest backend to integrate.

| Property | Value |
|----------|-------|
| Backend Type | `openai` |
| Protocol | HTTP |
| Port | 8000 (standard) or 80 (via predictor service) |
| API Format | OpenAI-compatible `/v1/chat/completions` |
| Model Example | `unsloth/gpt-oss-20b` |

**Routing Flow:**
1. Request arrives at API Router with model name
2. Router looks up routing policy
3. Request is forwarded directly to vLLM (passthrough)
4. vLLM response is returned as-is

### TRT-LLM with Triton (triton-grpc Backend Type)

TensorRT-LLM models running on Triton Inference Server require gRPC due to the "decoupled transaction policy" used for streaming inference.

| Property | Value |
|----------|-------|
| Backend Type | `triton-grpc` |
| Protocol | gRPC (bidirectional streaming) |
| Port | 9000 |
| API Format | Triton V2 gRPC with protocol translation |
| Model Name | Always `ensemble` (standard TRT-LLM pipeline) |
| Model Example | `meta-llama/Llama-3.1-8B-Instruct` |

**Why gRPC is Required:**

TRT-LLM models use Triton's "decoupled transaction policy" which:
- Allows the model to send multiple response chunks for a single request
- Enables true streaming at the engine level
- Is incompatible with HTTP endpoints (HTTP returns error: "HTTP end point doesn't support models with decoupled transaction policy")

**Routing Flow:**
1. Request arrives at API Router with model name
2. Router looks up routing policy (must have `backend_type: triton-grpc`)
3. Router translates OpenAI request to Triton gRPC format
4. gRPC streaming connection established to Triton
5. All response chunks collected (for non-streaming) or streamed (for streaming)
6. Response translated back to OpenAI format

## Routing Policy Configuration

### Policy Fields

| Field | Description | Required |
|-------|-------------|----------|
| `model` | Internal model path (e.g., `meta-llama/Llama-3.1-8B-Instruct`) | Yes |
| `external_name` | Name exposed in OpenAI API (optional alias) | No |
| `backends` | List of backend IDs with weights | Yes |
| `backend_type` | `openai`, `triton`, or `triton-grpc` | Yes for Triton |
| `tokenizer` | Tokenizer encoding for token counting | Yes for Triton |

### Backend Types

| Backend Type | Protocol | Use Case |
|--------------|----------|----------|
| `openai` | HTTP | vLLM, OpenAI-compatible backends |
| `triton` | HTTP | Standard Triton HTTP inference (not for TRT-LLM) |
| `triton-grpc` | gRPC | TRT-LLM on Triton (required for decoupled models) |

### Example Policies

**vLLM Backend:**
```json
{
  "model": "unsloth/gpt-oss-20b",
  "backends": [{"backend_id": "unsloth/gpt-oss-20b", "weight": 100}],
  "backend_type": "openai",
  "enabled": true
}
```

**TRT-LLM Backend:**
```json
{
  "model": "meta-llama/Llama-3.1-8B-Instruct",
  "backends": [{"backend_id": "meta-llama/Llama-3.1-8B-Instruct", "weight": 100}],
  "backend_type": "triton-grpc",
  "tokenizer": "llama3",
  "enabled": true
}
```

## Configuration Layers

### 1. Routing Policies (Preferred)

Stored in the Admin API database and synced to API Router periodically.

**Sync Mechanism:**
- API Router calls Admin API `/v1/routing/policies/sync` every 30s
- Policies are cached locally in BoltDB
- Changes take effect on next sync cycle

**CLI Commands:**
```bash
# List policies
ai-aas-cli routing policy list

# Create/update policy (after CLI is extended for backend_type)
ai-aas-cli routing policy create \
  --model "meta-llama/Llama-3.1-8B-Instruct" \
  --backends "meta-llama/Llama-3.1-8B-Instruct:100" \
  --global
```

### 2. BACKEND_ENDPOINTS Environment Variable (Fallback)

Static endpoint configuration in Helm values. Used when routing policy doesn't specify an endpoint.

**Format:**
```yaml
backends:
  endpoints: "model-id:http://service.namespace.svc.cluster.local:port"
```

**Example:**
```yaml
backends:
  endpoints: "unsloth/gpt-oss-20b:http://unsloth-gpt-oss-20b-predictor.development.svc.cluster.local:80,meta-llama/Llama-3.1-8B-Instruct:http://llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80"
```

**Note:** For gRPC backends, the API Router extracts the host and uses port 9000.

### 3. NetworkPolicy Egress Rules

The API Router needs egress access to inference backends.

**Required Ports:**
| Port | Protocol | Purpose |
|------|----------|---------|
| 8000 | TCP | vLLM HTTP inference |
| 8080 | TCP | TRT-LLM HTTP (fallback) |
| 9000 | TCP | TRT-LLM gRPC inference |

**NetworkPolicy Configuration:**
```yaml
egress:
  - to:
    - namespaceSelector:
        matchLabels:
          name: development
    ports:
      - port: 8000   # vLLM
      - port: 8080   # TRT-LLM HTTP
      - port: 9000   # TRT-LLM gRPC
```

## Service Discovery

### Kubernetes Service Naming

Backend services follow the pattern: `{model-name}-predictor.{namespace}.svc.cluster.local`

**Examples:**
- vLLM: `unsloth-gpt-oss-20b-predictor.development.svc.cluster.local:80`
- TRT-LLM: `llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80` (HTTP) or `:9000` (gRPC)

### Port Mapping

Each inference service typically exposes:
| Port | Purpose |
|------|---------|
| 80 | HTTP inference (mapped to 8080 internally) |
| 9000 | gRPC inference |
| 8002 | Prometheus metrics |

## Streaming vs Non-Streaming

### Streaming Requests

When `stream: true` in the OpenAI request:

| Backend Type | Behavior |
|--------------|----------|
| `openai` | Passthrough HTTP SSE |
| `triton-grpc` | gRPC streaming → SSE translation |
| `triton` | Error (HTTP Triton doesn't support streaming) |

### Non-Streaming Requests

When `stream: false` (default):

| Backend Type | Behavior |
|--------------|----------|
| `openai` | Single HTTP request/response |
| `triton-grpc` | gRPC streaming, collect all chunks, return single response |
| `triton` | Single HTTP request/response |

## Token Counting

For Triton backends, the API Router performs token counting since Triton doesn't natively report token usage.

**Supported Tokenizers:**
- `llama3` - For Llama 3.x models
- `cl100k_base` - For GPT-4/3.5 compatible models
- `o200k_base` - For newer OpenAI models

**Configuration:**
The `tokenizer` field in the routing policy specifies which encoding to use.

## Troubleshooting

### Error: "HTTP end point doesn't support models with decoupled transaction policy"

**Cause:** Attempting to use HTTP with a TRT-LLM model that requires gRPC.

**Solution:** Update the routing policy to use `backend_type: triton-grpc`:
```bash
# Via Admin API
curl -X PATCH "https://admin-api.dev.otherjamesbrown.com/v1/routing/policies/{policy_id}" \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"backend_type": "triton-grpc", "tokenizer": "llama3"}'
```

### Error: "no routing policy configured"

**Cause:** No policy exists for the requested model.

**Solution:**
1. Check existing policies: `ai-aas-cli routing policy list`
2. Create a new policy via Admin API
3. Wait for sync (up to 30s)

### Error: "backend connection failed"

**Cause:** Network connectivity issue to inference backend.

**Solutions:**
1. Verify service exists: `kubectl get svc -n development | grep <model>`
2. Check NetworkPolicy allows egress to required ports
3. Verify DNS resolution from api-router pod

## Related Documents

- [Routing Policies](../routing-policies.md) - Policy configuration details
- [vLLM Registration Workflow](../vllm-registration-workflow.md) - vLLM model setup
- [Observability Architecture](./observability-architecture.md) - Monitoring and logging
