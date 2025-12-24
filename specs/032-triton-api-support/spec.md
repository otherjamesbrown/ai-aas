# Spec: Triton API Support

**Spec Number:** 032
**Created:** 2025-12-18
**Status:** Ready for Implementation
**Epic Bead:** ai-aas-spec032

## Summary

Add OpenAI API compatibility for Triton Inference Server backends by implementing a protocol translation layer in the API Router. This enables TensorRT-LLM models to be accessed via the standard `/v1/chat/completions` endpoint.

## Clarifications

The following decisions were made during idea refinement:

| Topic | Decision | Rationale |
|-------|----------|-----------|
| **Approach** | Translation layer in API Router | Centralized control, no extra services, enables smart load balancing |
| **Backend Type Config** | Explicit in routing policy | Validate on sync, return clear errors if misconfigured |
| **Token Counting** | Tiktoken/tokenizer library | Accurate counts needed for billing/quotas |
| **Streaming** | Phase 2 (not MVP) | MVP is non-streaming; add SSE streaming later |
| **Error Mapping** | Triton → OpenAI codes | Preserve Triton details in response for debugging |
| **Tokenizer Config** | Configurable per model | Different models need different encodings (cl100k_base, llama3, etc.) |

## User Scenarios

### US-1: Developer uses OpenAI SDK with TRT-LLM model

**Actor:** Application Developer
**Precondition:** TRT-LLM model deployed via Triton, routing policy configured with `backend_type: triton`

**Flow:**
1. Developer configures OpenAI SDK with platform API endpoint and key
2. Developer calls `client.chat.completions.create(model="meta-llama/Llama-3.1-8B-Instruct", messages=[...])`
3. API Router receives request, detects Triton backend type from routing policy
4. API Router translates OpenAI format to Triton V2 protocol
5. Triton processes request and returns tensor response
6. API Router translates response back to OpenAI format with accurate token counts
7. Developer receives standard OpenAI-format response

**Postcondition:** Developer can use TRT-LLM models with standard tooling (LangChain, LlamaIndex, OpenAI SDK)

### US-2: Platform operator configures Triton backend

**Actor:** Platform Operator
**Precondition:** Triton deployment running in cluster

**Flow:**
1. Operator adds routing policy with `backend_type: triton` and `tokenizer: llama3`
2. API Router validates configuration on sync (checks backend reachability, tokenizer exists)
3. If validation fails, operator receives clear error message
4. If validation succeeds, model becomes available via OpenAI-compatible API

**Postcondition:** Triton backend is accessible via standard API

### US-3: Request fails with informative error

**Actor:** Application Developer
**Precondition:** Triton backend is overloaded or unavailable

**Flow:**
1. Developer sends chat completion request
2. Triton returns error (e.g., `RESOURCE_EXHAUSTED`)
3. API Router maps error to OpenAI format (`rate_limit_exceeded`)
4. Response includes Triton details for debugging
5. Developer can diagnose issue from error response

**Postcondition:** Errors are actionable and follow OpenAI conventions

## Functional Requirements

### FR-1: Routing Policy Schema

The routing policy MUST support:
- `backend_type`: enum `"openai"` (default) | `"triton"`
- `tokenizer`: string specifying tiktoken encoding (e.g., `cl100k_base`, `llama3`, `o200k_base`)

### FR-2: Protocol Translation

The API Router MUST:
- Detect backend type from routing policy
- Translate OpenAI `ChatCompletionRequest` to Triton V2 `InferRequest`
- Translate Triton V2 `InferResponse` to OpenAI `ChatCompletionResponse`
- Support the `/v2/models/{model}/infer` Triton endpoint

### FR-3: Token Counting

The API Router MUST:
- Use tiktoken for accurate prompt and completion token counts
- Support configurable tokenizer per model via routing policy
- Include accurate `usage` object in response (`prompt_tokens`, `completion_tokens`, `total_tokens`)

### FR-4: Error Mapping

The API Router MUST map Triton errors to OpenAI format:

| Triton Status | HTTP | OpenAI Error Type |
|---------------|------|-------------------|
| `UNAVAILABLE` | 503 | `service_unavailable` |
| `INVALID_ARG` | 400 | `invalid_request_error` |
| `NOT_FOUND` | 404 | `model_not_found` |
| `RESOURCE_EXHAUSTED` | 429 | `rate_limit_exceeded` |
| `DEADLINE_EXCEEDED` | 504 | `timeout` |
| `INTERNAL` | 500 | `internal_error` |

Triton-specific details MUST be preserved in `triton_details` field for debugging.

### FR-5: Configuration Validation

On routing policy sync, the API Router MUST:
- Validate backend is reachable (health check)
- Validate tokenizer encoding exists
- Return clear error if misconfigured

## Non-Functional Requirements

### NFR-1: Latency

Translation overhead MUST be < 5ms for request/response transformation.

### NFR-2: Compatibility

MUST support standard OpenAI SDK clients without modification.

### NFR-3: Observability

All translated requests MUST:
- Include trace_id correlation
- Log backend_type in structured logs
- Emit metrics for translation success/failure

## Out of Scope

- Streaming inference (Phase 2)
- Supporting other non-OpenAI protocols (gRPC, custom REST APIs)
- Direct Triton endpoint exposure to users
- Triton model management (loading/unloading models)
- Istio service mesh integration

## Inference Engine Comparison

This section documents the three inference engines supported by the platform and their model capabilities.

### Engine Overview

| Engine | Strengths | API Style | Best For |
|--------|-----------|-----------|----------|
| **vLLM** | Fast LLM inference, PagedAttention, OpenAI-compatible | REST (OpenAI) | Chat/completion LLMs, some VLMs |
| **TensorRT-LLM** | Maximum throughput, NVIDIA-optimized, lowest latency | gRPC (KServe V2) | Production LLMs at scale |
| **TGI** | HuggingFace native, easy setup, broad model support | REST (OpenAI-like) | Rapid prototyping, diverse models |

### Model Type Support Matrix

| Model Type | vLLM | TensorRT-LLM | TGI | Example Models |
|------------|------|--------------|-----|----------------|
| **Chat/Instruct LLM** | Yes | Yes | Yes | Llama-3.1-8B-Instruct, Mistral-7B-Instruct, Qwen2.5-7B-Instruct |
| **Code LLM** | Yes | Yes | Yes | CodeLlama-34B, DeepSeek-Coder-V2, StarCoder2-15B |
| **Vision-Language (VLM)** | Yes | Limited | Yes | LLaVA-1.6, Qwen2-VL-7B, Pixtral-12B, InternVL2 |
| **Speech-to-Text (ASR)** | No | Yes | No | Whisper-large-v3, Distil-Whisper, Canary-1B |
| **Text-to-Speech (TTS)** | No | Yes | No | XTTS-v2, Bark, Parler-TTS |
| **Image Generation** | No | Yes | No | SDXL, FLUX.1, Stable Diffusion 3 |
| **Embeddings** | Yes | Yes | Yes | BGE-large, E5-Mistral-7B, GTE-Qwen2 |
| **Reranking** | No | Yes | Yes | BGE-Reranker-v2, Cohere-Rerank |

### Model Deployment Roadmap

#### Phase 1: TGI Addition (Text Models)

| Model | Engine | Type | Size | Use Case |
|-------|--------|------|------|----------|
| `mistralai/Mistral-7B-Instruct-v0.3` | TGI | Chat LLM | 7B | General chat, compare with vLLM |
| `Qwen/Qwen2.5-7B-Instruct` | TGI | Chat LLM | 7B | Multilingual, long context |
| `bigcode/starcoder2-15b` | TGI | Code LLM | 15B | Code generation/completion |

#### Phase 2: Vision-Language Models

| Model | Engine | Type | Size | Use Case |
|-------|--------|------|------|----------|
| `llava-hf/llava-v1.6-mistral-7b-hf` | vLLM | VLM | 7B | Image understanding |
| `Qwen/Qwen2-VL-7B-Instruct` | vLLM/TGI | VLM | 7B | Document OCR, charts |
| `mistral-community/pixtral-12b` | TGI | VLM | 12B | Multi-image reasoning |

#### Phase 3: Audio Models (Triton/TensorRT-LLM)

| Model | Engine | Type | Size | Use Case |
|-------|--------|------|------|----------|
| `openai/whisper-large-v3` | Triton | ASR | 1.5B | Speech transcription |
| `distil-whisper/distil-large-v3` | Triton | ASR | 756M | Fast transcription |
| `nvidia/canary-1b` | Triton | ASR | 1B | Multi-language ASR |

#### Phase 4: Embeddings & RAG

| Model | Engine | Type | Size | Use Case |
|-------|--------|------|------|----------|
| `BAAI/bge-large-en-v1.5` | vLLM/TGI | Embedding | 335M | Document search |
| `Alibaba-NLP/gte-Qwen2-7B-instruct` | vLLM | Embedding | 7B | High-quality embeddings |
| `BAAI/bge-reranker-v2-m3` | TGI | Reranker | 568M | Search result reranking |

## Technical Notes

- Existing TRT-LLM deployment: `llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80`
- Triton serves on port 8080 internally, mapped to port 80 on service
- Health endpoint: `/v2/health/ready`
- Related specs: 029-triton-tensorrt-llm
- No KNative or Istio mesh - direct service-to-service HTTP

## Validation

### Development Cluster

- [ ] API Router starts successfully with Triton backend configured
- [ ] `curl /v1/chat/completions` with Triton model returns valid OpenAI response
- [ ] Token counts in response are accurate (verified against tiktoken)
- [ ] Invalid tokenizer config returns clear validation error
- [ ] Triton errors are mapped to correct OpenAI error codes
- [ ] `triton_details` field present in error responses
- [ ] Logs show backend_type and trace_id correlation
- [ ] Translation latency < 5ms (check metrics)

### Staging

- [ ] End-to-end test with OpenAI Python SDK succeeds
- [ ] LangChain integration test passes
- [ ] Load test shows no regression in P99 latency
- [ ] Error rate under load matches vLLM backends

### Production

- [ ] Canary deployment successful
- [ ] No increase in error rates after rollout
- [ ] Customer-facing documentation updated

## Success Criteria

1. **Compatibility**: OpenAI SDK, LangChain, and LlamaIndex can use TRT-LLM models without code changes
2. **Accuracy**: Token counts match tiktoken calculations within 1%
3. **Reliability**: Error mapping provides actionable debugging information
4. **Performance**: Translation adds < 5ms overhead
5. **Operations**: Clear validation errors prevent misconfiguration
