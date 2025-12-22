# Idea: Triton API Support

**Spec Number:** 032
**Created:** 2025-12-18
**Status:** Draft

## Problem

The AI-AAS platform currently only supports backends that serve OpenAI-compatible APIs (vLLM, TGI). TensorRT-LLM deployments use Triton Inference Server which serves a different protocol (`/v2/models/{model}/infer` with tensor-based request/response format). This means:

- The `llama-3.1-8b-instruct` model deployed with TensorRT-LLM cannot be accessed via the standard `/v1/chat/completions` endpoint
- Users cannot use standard tools (LangChain, LlamaIndex, OpenAI SDK) to interact with TRT-LLM models
- We lose the performance benefits of TensorRT-LLM for users who expect OpenAI API compatibility

## Discussion

### Industry Standard
- OpenAI API format is the de facto standard for LLM interactions
- All major inference servers (vLLM, TGI, Ollama, llama.cpp) have implemented OpenAI compatibility
- Tools like LangChain, LlamaIndex, and the OpenAI SDK all expect OpenAI-format endpoints
- Triton's native protocol is primarily used by ML infrastructure teams, not application developers

### Options Explored

**Option A: Translation Layer in API Router (Selected)**
- Add protocol translation directly in the API router
- Leverage existing `internal/adapter/kserve/` infrastructure as template
- ~500-700 lines of new code
- Pros: Centralized control, no extra services, enables smart load balancing
- Cons: Maintenance burden for protocol changes

**Option B: OpenAI-Compatible Sidecar** (Rejected)
- Deploy translation service alongside Triton pods
- Rejected: Extra network hop, more operational overhead

**Option C: KServe OpenAI Protocol** (Rejected)
- Use KServe's experimental OpenAI support
- Rejected: Too immature for production

### Key Decisions

| Topic | Decision | Rationale |
|-------|----------|-----------|
| **Backend Type Config** | Explicit in routing policy | Validate on sync, return clear errors if misconfigured |
| **Token Counting** | Tiktoken/tokenizer library | Accurate counts needed for billing/quotas |
| **Streaming** | Phase 2 (not MVP) | MVP is non-streaming; add later if needed |
| **Error Mapping** | Triton → OpenAI codes | Preserve Triton details for debugging |
| **Istio/Service Mesh** | Not required | No mTLS requirements; handle retries in app |
| **Tokenizer Config** | Configurable per model | Different models need different encodings (cl100k_base, llama3, etc.) |

### Architecture Context

**GPU Workloads Use RawDeployment Mode**: KServe supports two deployment modes:
- **RawDeployment**: Standard K8s Deployment + ClusterIP Service (used for GPU workloads)
- **Serverless**: Knative-based with Istio routing (NOT used for GPU workloads)

GPU workloads (vLLM, Triton, TensorRT-LLM) MUST use RawDeployment because:
- Knative rejects `nodeSelector` (needed for GPU node scheduling)
- Knative has single-port restriction (Triton needs multiple ports)
- Scale-to-zero is counterproductive (5-10 min model load times)
- Istio/Knative routing adds complexity and failure modes

**Current Architecture** (simplified):
```
┌─────────────────────────────────────────────────────────────────┐
│                         Kubernetes                               │
│                                                                  │
│   API Router (Deployment)                                        │
│        │                                                         │
│        ├──── HTTP ────► vLLM (RawDeployment + ClusterIP)        │
│        │                  └─ OpenAI-compatible                   │
│        │                                                         │
│        └──── HTTP ────► Triton (RawDeployment + ClusterIP)      │
│                           └─ Needs translation layer             │
└─────────────────────────────────────────────────────────────────┘
```

Direct service-to-service HTTP via ClusterIP services (no Istio mesh for inference traffic).

**GPU Scheduling Architecture**: The platform uses a **pool-per-instance-type** pattern (not pool-per-model):

1. **GPU nodes are auto-tainted** via DaemonSet (`nvidia.com/gpu=true:NoSchedule`)
   - Prevents non-GPU workloads from landing on expensive GPU nodes

2. **Models declare requirements** in `ModelRecipe` CRD:
   ```yaml
   spec:
     scheduling:
       tolerations:
         - key: nvidia.com/gpu
           operator: Exists
           effect: NoSchedule
       nodeSelector:
         nvidia.com/gpu.present: "true"
         # Can also target specific GPU types:
         # gpu-type: nvidia-a100
   ```

3. **K8s scheduler places pods** on nodes matching those constraints

This pattern provides:
- **Cost efficiency** - Models share nodes; no idle dedicated capacity
- **Flexibility** - New models just need proper selectors, no infra changes
- **Scaling** - Add nodes to a pool, all compatible models benefit

The `AIModel` and `ModelRecipe` CRDs support `nodeSelector`, `tolerations`, and full `affinity` rules for complex scheduling needs.

## Proposed Approach

Implement **Option A: Translation Layer in API Router**

### Routing Policy Schema Update

```yaml
# routing_policies table / routing policy config
model: meta-llama/Llama-3.1-8B-Instruct
backend_type: triton  # NEW FIELD: "openai" (default) | "triton"
tokenizer: llama3     # NEW FIELD: tiktoken encoding (cl100k_base, llama3, etc.)
backends:
  - backend_id: meta-llama/Llama-3.1-8B-Instruct
    weight: 100
```

**Tokenizer encodings:**
- `cl100k_base` - GPT-4, GPT-3.5-turbo (default for OpenAI-compatible)
- `llama3` - Llama 3.x models
- `o200k_base` - GPT-4o models
- Model-specific encodings loaded from HuggingFace tokenizers when needed

Validation on policy sync:
- Check backend is reachable
- Verify backend type matches actual protocol
- Validate tokenizer encoding exists
- Return clear error if misconfigured

### Files to Create/Modify

**New Files:**
1. `internal/adapter/triton/types.go` - Triton V2 protocol types
2. `internal/adapter/triton/translator.go` - OpenAI ↔ Triton translation
3. `internal/adapter/triton/tokenizer.go` - Tiktoken integration for accurate token counts

**Modified Files:**
4. `internal/api/public/openai.go` - Backend type detection, use translator
5. `internal/routing/backend_client.go` - Handle Triton response format
6. `internal/config/loader.go` - Backend type metadata in routing policy
7. `internal/config/cache.go` - Store backend_type with policy

### Translation Flow

```
Client Request (OpenAI format)
    │
    ▼
HandleOpenAIChatCompletions()
    │
    ▼
GetPolicy() → returns backend_type
    │
    ├─── backend_type == "openai" ───► Forward as-is to vLLM
    │
    └─── backend_type == "triton" ───► TritonTranslator
                                            │
                                            ▼
                                       TranslateOpenAIToTriton()
                                            │
                                            ▼
                                       POST /v2/models/{model}/infer
                                            │
                                            ▼
                                       TranslateTritonToOpenAI()
                                            │
                                            ▼
                                       TokenizeForUsage() (tiktoken)
                                            │
                                            ▼
                                       OpenAI Response to Client
```

### Error Mapping

| Triton Status | HTTP | OpenAI Error | Response |
|---------------|------|--------------|----------|
| `UNAVAILABLE` | 503 | `service_unavailable` | Backend temporarily unavailable |
| `INVALID_ARG` | 400 | `invalid_request_error` | Invalid parameter: {details} |
| `NOT_FOUND` | 404 | `model_not_found` | Model not found |
| `RESOURCE_EXHAUSTED` | 429 | `rate_limit_exceeded` | Backend overloaded |
| `DEADLINE_EXCEEDED` | 504 | `timeout` | Request timed out |
| `INTERNAL` | 500 | `internal_error` | Backend error |

Preserve Triton details:
```json
{
  "error": {
    "message": "Backend error: CUDA out of memory",
    "type": "internal_error",
    "code": "BACKEND_ERROR",
    "triton_details": "CUDA out of memory. Tried to allocate 2.00 GiB..."
  }
}
```

## Open Questions

- ~~Should backend type be auto-detected or explicitly configured?~~ **Explicit config**
- ~~Token counting strategy?~~ **Tiktoken for accuracy**
- ~~Streaming support?~~ **Phase 2, not MVP**
- ~~Error mapping approach?~~ **Map to OpenAI, preserve details**
- ~~Which tiktoken encoding to use for Llama models?~~ **Configurable per model in routing policy**

All open questions resolved.

## Out of Scope

- Streaming inference (Phase 2)
- Supporting other non-OpenAI protocols (gRPC, custom REST APIs)
- Direct Triton endpoint exposure to users
- Triton model management (loading/unloading models)
- Istio service mesh integration

## Notes

- Existing TRT-LLM deployment: `llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80`
- Triton serves on port 8080 internally, mapped to port 80 on service
- Health endpoint: `/v2/health/ready`
- Related specs: 029-triton-tensorrt-llm
- GPU workloads use RawDeployment mode (no Knative/Serverless) - simplifies architecture
- No mTLS/Istio mesh requirements for inference traffic

## Implementation Phases

**Phase 1 (MVP):**
- [ ] Add `backend_type` and `tokenizer` to routing policy schema
- [ ] Implement Triton translator (non-streaming)
- [ ] Integrate tiktoken with configurable encodings per model
- [ ] Error mapping (Triton → OpenAI codes)
- [ ] Validation on policy sync (backend type, tokenizer)
- [ ] Unit tests

**Phase 2 (Future):**
- [ ] Streaming support for Triton backends
- [ ] Additional backend types (TGI native, Ollama, etc.)
