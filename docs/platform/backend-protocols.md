# Backend Protocols and Protocol Translation

**Last Updated**: 2026-01-03
**Status**: Active

## Overview

The API Router Service supports multiple backend protocols to communicate with inference servers. Different model serving frameworks (vLLM, TensorRT-LLM, Triton) use different API protocols, and the router handles translation between the OpenAI-compatible API format that clients use and the native protocol of each backend.

## Backend Types

The `backend_type` field in routing policies determines which protocol the router uses to communicate with the backend:

| backend_type | Protocol | Use Case |
|--------------|----------|----------|
| `openai` (default) | OpenAI-compatible REST | vLLM, OpenAI API, most LLM servers |
| `triton` | Triton V2 HTTP | Standard Triton Inference Server |
| `triton-grpc` | Triton V2 gRPC | TensorRT-LLM, TRT-LLM backends |

### OpenAI Protocol (default)

Used for vLLM and other OpenAI-compatible backends:
- Endpoint: `/v1/chat/completions` or `/v1/completions`
- Request/Response: Standard OpenAI format

### Triton V2 Protocol

Used for NVIDIA Triton Inference Server backends:
- HTTP Endpoint: `/v2/models/{model_name}/infer`
- gRPC Endpoint: Port 8001 (default)
- Request/Response: KServe V2 inference protocol

### TensorRT-LLM (TRT-LLM)

TensorRT-LLM models **require** `backend_type: triton-grpc` because:
1. They use the "decoupled transaction policy" which requires gRPC streaming
2. They do not expose `/v1/chat/completions` endpoints
3. All requests (streaming and non-streaming) must use gRPC

## Routing Policy Configuration

### Example: vLLM Backend (OpenAI Protocol)

```bash
curl -X POST https://admin-api.dev.otherjamesbrown.com/v1/routing/policies \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "organization_id": "*",
    "model": "gpt-oss-20b",
    "backends": [{"backend_id": "unsloth/gpt-oss-20b", "weight": 100}],
    "failover_threshold": 3,
    "backend_type": "openai"
  }'
```

### Example: TRT-LLM Backend (Triton gRPC Protocol)

```bash
curl -X POST https://admin-api.dev.otherjamesbrown.com/v1/routing/policies \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "organization_id": "*",
    "model": "llama-3.1-8b-instruct",
    "backends": [{"backend_id": "meta-llama/Llama-3.1-8B-Instruct", "weight": 100}],
    "failover_threshold": 3,
    "backend_type": "triton-grpc",
    "tokenizer": "cl100k_base"
  }'
```

### Updating an Existing Policy

To fix a misconfigured policy:

```bash
curl -X PATCH https://admin-api.dev.otherjamesbrown.com/v1/routing/policies/{policy_id} \
  -H "Authorization: Bearer $ADMIN_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "backend_type": "triton-grpc",
    "tokenizer": "cl100k_base"
  }'
```

## How to Identify Backend Type

### By Service Name

The backend URI often indicates the protocol:
- Contains `trtllm`, `tensorrt`, or `-trt-` → Use `triton-grpc`
- Contains `vllm` or no special suffix → Use `openai`
- Contains `triton` (without trtllm) → Use `triton` (HTTP) or `triton-grpc`

### By Health Endpoint

- vLLM backends: `/health` returns 200
- Triton backends: `/v2/health/ready` returns 200
- TRT-LLM backends: `/v2/health/ready` returns 200, `/v1/*` returns 404

## Error Detection

The API Router automatically detects misconfigured backends and provides actionable error messages:

### Runtime Detection

If a request to a backend returns 404 and the backend URL contains TRT-LLM patterns, the router returns:

```json
{
  "error": "backend returned 404: this appears to be a KServe v2 / TRT-LLM backend. Update the routing policy to use backend_type: 'triton-grpc' and set a tokenizer.",
  "trace_id": "abc123..."
}
```

### Startup Validation

At startup, the router validates all routing policies against backend URIs and logs warnings for potential misconfigurations:

```
WARN: MISCONFIGURATION: routing policy has backend_type 'openai' but backend URL suggests KServe v2 / TRT-LLM
  policy_id: "218c32d3-7412-4e9f-b48c-a566433d36ba"
  model: "llama-3.1-8b-instruct"
  backend_uri: "http://llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80"
  current_backend_type: "openai"
  suggested_backend_type: "triton-grpc"
  hint: "Update routing policy via Admin API: PATCH /v1/routing/policies/{id} with backend_type='triton-grpc'"
```

## Tokenizer Configuration

When using `triton` or `triton-grpc` backend types, you should also set a `tokenizer` for accurate token counting:

| Model Family | Tokenizer |
|-------------|-----------|
| GPT-4, GPT-3.5 | `cl100k_base` |
| Llama 2/3 | `cl100k_base` (approximation) |
| Custom | Use tiktoken encoding name |

## Protocol Translation Details

### OpenAI to Triton V2

The router translates:
1. OpenAI `messages` array → Triton `text_input` tensor
2. OpenAI `max_tokens` → Triton `max_tokens` parameter
3. OpenAI `temperature` → Triton `temperature` parameter
4. Response: Triton `text_output` tensor → OpenAI `choices[0].message.content`

### Streaming

- OpenAI protocol: Server-Sent Events (SSE)
- Triton gRPC: Bidirectional streaming
- The router handles translation between these formats

## Troubleshooting

### Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| 404 errors from backend | Wrong `backend_type` for TRT-LLM | Set `backend_type: triton-grpc` |
| "decoupled mode" errors | TRT-LLM requires gRPC | Set `backend_type: triton-grpc` |
| No response from backend | Backend health check failing | Check `/v2/health/ready` endpoint |
| Token counting incorrect | Missing tokenizer | Set `tokenizer` field in policy |

### Diagnostic Commands

```bash
# Check current routing policies
curl -H "Authorization: Bearer $ADMIN_API_KEY" \
  https://admin-api.dev.otherjamesbrown.com/v1/routing/policies | jq '.policies[] | {model, backend_type}'

# Check backend endpoints
curl -H "Authorization: Bearer $ADMIN_API_KEY" \
  https://api.dev.otherjamesbrown.com/v1/models

# Test backend health (vLLM)
curl http://<backend-service>:8000/health

# Test backend health (Triton/TRT-LLM)
curl http://<backend-service>:8000/v2/health/ready
```

## Related Documentation

- [API Router and KServe Integration](./api-router-kserve-integration.md)
- [Model Registration Workflow](./model-registration-workflow.md)
- [KServe Deployment Troubleshooting](./troubleshooting/kserve-deployment-troubleshooting.md)
