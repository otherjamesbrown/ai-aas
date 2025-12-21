# Implementation Plan: Triton API Support

**Spec**: [spec.md](./spec.md)
**Impact Analysis**: [impact.md](./impact.md)
**Epic Bead**: ai-aas-spec032
**Created**: 2025-12-21

## Overview

This plan implements OpenAI API compatibility for Triton Inference Server backends via a protocol translation layer in the API Router. The implementation follows a 4-phase approach aligned with the impact analysis.

## Technical Architecture

```yaml
request_flow:
  1_receive: "Client → API Router (POST /v1/chat/completions)"
  2_auth: "API Router → user-org-service (validate API key)"
  3_policy: "API Router → GetPolicy() → returns backend_type"
  4_branch:
    openai: "Forward request as-is to vLLM backend"
    triton: "Translate → POST /v2/models/{model}/infer → Translate back"
  5_response: "API Router → Client (OpenAI format)"

triton_translation:
  request:
    input: "OpenAIChatCompletionRequest"
    output: "Triton V2 InferRequest"
    steps:
      - Format messages to prompt string
      - Build input tensor (BYTES type)
      - Map parameters (temperature, max_tokens, etc.)

  response:
    input: "Triton V2 InferResponse"
    output: "OpenAIChatCompletionResponse"
    steps:
      - Extract text from output tensor
      - Count tokens with tiktoken
      - Build OpenAI response structure
```

## Component Breakdown

### Component 1: Schema Extension (admin-api-service)

**Location**: `services/admin-api-service/`

| File | Change | Description |
|------|--------|-------------|
| `internal/domain/policy.go` | MODIFY | Add `BackendType`, `Tokenizer` fields to RoutingPolicy |
| `internal/repository/policies.go` | MODIFY | Update SQL queries for new columns |
| `migrations/NNNN_add_backend_type.sql` | ADD | Database migration |
| `internal/api/handlers/policies.go` | MODIFY | Add validation for backend_type values (Task 1.5) |

**Schema Changes**:
```go
type RoutingPolicy struct {
    // ... existing fields ...
    BackendType string `json:"backend_type,omitempty"` // "openai" (default) | "triton"
    Tokenizer   string `json:"tokenizer,omitempty"`    // tiktoken encoding name
}
```

### Component 2: Config Loader Extension (api-router-service)

**Location**: `services/api-router-service/internal/config/`

| File | Change | Description |
|------|--------|-------------|
| `loader.go` | MODIFY | Add BackendType, Tokenizer to RoutingPolicy struct |
| `cache.go` | MODIFY | Store/retrieve new fields |
| `loader_test.go` | MODIFY | Add test cases for new fields |

### Component 3: Triton Adapter Package (api-router-service)

**Location**: `services/api-router-service/internal/adapter/triton/`

| File | Change | Description |
|------|--------|-------------|
| `types.go` | ADD | Triton V2 protocol types (InferRequest, InferResponse, tensors) |
| `translator.go` | ADD | OpenAI ↔ Triton translation logic |
| `tokenizer.go` | ADD | Tiktoken integration for token counting |
| `errors.go` | ADD | Error mapping (Triton → OpenAI error codes) |
| `translator_test.go` | ADD | Unit tests for translation |
| `tokenizer_test.go` | ADD | Unit tests for token counting |

**Key Types** (from Triton V2 protocol):
```go
type InferRequest struct {
    ID         string                 `json:"id,omitempty"`
    Inputs     []InferInputTensor     `json:"inputs"`
    Outputs    []InferRequestedOutput `json:"outputs,omitempty"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}

type InferResponse struct {
    ID         string             `json:"id"`
    ModelName  string             `json:"model_name"`
    Outputs    []InferOutputTensor `json:"outputs"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}
```

### Component 4: Request Handler Integration (api-router-service)

**Location**: `services/api-router-service/internal/api/public/`

| File | Change | Description |
|------|--------|-------------|
| `openai.go` | MODIFY | Add backend type detection and branching |
| `handler.go` | MODIFY | Initialize Triton translator |

**Integration Point** (openai.go ~line 130):
```go
// After GetPolicy()
switch policy.BackendType {
case "triton":
    return h.handleTritonChatCompletion(ctx, w, r, policy, openAIReq)
default: // "openai" or empty
    return h.forwardOpenAIRequest(ctx, backend, openAIReq, "chat")
}
```

### Component 5: Tests

| File | Change | Description |
|------|--------|-------------|
| `internal/adapter/triton/*_test.go` | ADD | Unit tests for adapter |
| `test/integration/triton_backend_test.go` | ADD | Integration tests |
| `test/integration/openai_chat_test.go` | MODIFY | Add Triton test cases |

## Dependencies

```yaml
external:
  new:
    - name: "github.com/pkoukk/tiktoken-go"
      version: "^0.1.6"
      purpose: "Accurate token counting for billing"

  existing:
    - "go.etcd.io/etcd/client/v3" (config sync)
    - "github.com/google/uuid" (request IDs)

internal:
  - config.Loader → RoutingPolicy with BackendType
  - routing.BackendEndpoint → backend connection info
  - auth.AuthenticatedContext → org/user context

infrastructure:
  - Triton deployment: llama-3-1-8b-instruct-trtllm-predictor.development.svc.cluster.local:80
  - Health endpoint: /v2/health/ready
```

## Implementation Order

### Phase 1: Schema Extension (Low Risk)

**Goal**: Add backend_type and tokenizer fields, backward compatible

```yaml
tasks:
  1.1_domain_types:
    files:
      - services/admin-api-service/internal/domain/policy.go
    changes:
      - Add BackendType string field (default empty = "openai")
      - Add Tokenizer string field
    tests: Existing tests should pass (new fields optional)

  1.2_database_migration:
    files:
      - services/admin-api-service/migrations/NNNN_add_backend_type.sql
    changes:
      - ALTER TABLE routing_policies ADD COLUMN backend_type VARCHAR(32) DEFAULT 'openai'
      - ALTER TABLE routing_policies ADD COLUMN tokenizer VARCHAR(64)
    rollback: DROP COLUMN statements

  1.3_repository_update:
    files:
      - services/admin-api-service/internal/repository/policies.go
    changes:
      - Update INSERT/UPDATE/SELECT queries
      - Add new fields to scan

  1.4_api_router_config:
    files:
      - services/api-router-service/internal/config/loader.go
      - services/api-router-service/internal/config/cache.go
    changes:
      - Add BackendType, Tokenizer to RoutingPolicy struct
      - Update JSON unmarshaling

  1.5_api_validation:
    files:
      - services/admin-api-service/internal/api/handlers/policies.go
    changes:
      - Validate backend_type enum (openai, triton)
      - Validate tokenizer encoding exists
      - Require tokenizer when backend_type is triton
    tests: Add validation tests
```

### Phase 2: Triton Adapter (Medium Risk)

**Goal**: Implement translation layer, not yet wired to request path

```yaml
tasks:
  2.1_triton_types:
    files:
      - services/api-router-service/internal/adapter/triton/types.go
    changes:
      - Define InferRequest, InferResponse
      - Define tensor types (InferInputTensor, InferOutputTensor)
      - Define error types

  2.2_translator:
    files:
      - services/api-router-service/internal/adapter/triton/translator.go
    changes:
      - TranslateOpenAIToTriton(req *OpenAIChatCompletionRequest) (*InferRequest, error)
      - TranslateTritonToOpenAI(resp *InferResponse, origReq) (*OpenAIChatCompletionResponse, error)
      - formatPrompt() - convert messages to single prompt
    reference: internal/adapter/kserve/translator.go (similar pattern)

  2.3_tokenizer:
    files:
      - services/api-router-service/internal/adapter/triton/tokenizer.go
    changes:
      - NewTokenizer(encoding string) (*Tokenizer, error)
      - CountTokens(text string) (int, error)
      - Supported encodings: cl100k_base, llama3, o200k_base
    dependency: github.com/pkoukk/tiktoken-go

  2.4_error_mapping:
    files:
      - services/api-router-service/internal/adapter/triton/errors.go
    changes:
      - MapTritonError(status string) (*OpenAIError, int)
      - Error code mapping table (UNAVAILABLE → 503, etc.)

  2.5_unit_tests:
    files:
      - services/api-router-service/internal/adapter/triton/translator_test.go
      - services/api-router-service/internal/adapter/triton/tokenizer_test.go
      - services/api-router-service/internal/adapter/triton/errors_test.go
    coverage_target: ">80%"
```

### Phase 3: Integration (High Risk)

**Goal**: Wire Triton adapter into request flow

```yaml
tasks:
  3.1_handler_init:
    files:
      - services/api-router-service/internal/api/public/handler.go
    changes:
      - Add tritonTranslator field to Handler
      - Initialize in NewHandler()

  3.2_request_branching:
    files:
      - services/api-router-service/internal/api/public/openai.go
    changes:
      - Add handleTritonRequest() method
      - Add backend type switch after GetPolicy()
      - Build Triton endpoint URL (/v2/models/{model}/infer)
    risk: HIGH - production request path

  3.3_triton_forwarding:
    files:
      - services/api-router-service/internal/api/public/openai.go
    changes:
      - forwardTritonRequest() - HTTP POST to Triton
      - Parse Triton response
      - Translate back to OpenAI format
      - Handle errors with mapping

  3.4_observability:
    files:
      - services/api-router-service/internal/api/public/openai.go
    changes:
      - Log backend_type in structured logs
      - Add triton_translation_duration_ms metric
      - Preserve trace_id across translation

  3.5_integration_tests:
    files:
      - services/api-router-service/test/integration/triton_backend_test.go
    changes:
      - Test with Triton mock server
      - Test error scenarios
      - Test token counting accuracy

  3.6_config_validation:
    files:
      - services/api-router-service/internal/config/loader.go
    changes:
      - Validate tokenizer encoding on policy sync
      - Health check Triton backend on sync (warning only)
      - Log clear messages for misconfigurations
    implements: FR-5 (Configuration Validation)
```

### Phase 4: Validation (Low Risk)

**Goal**: End-to-end validation and documentation

```yaml
tasks:
  4.1_development_validation:
    steps:
      - Deploy updated api-router-service
      - Configure routing policy with backend_type: triton
      - Test with curl
      - Verify token counts

  4.2_sdk_validation:
    steps:
      - Test with OpenAI Python SDK
      - Test with LangChain
      - Verify streaming not attempted (returns error)

  4.3_load_test:
    steps:
      - Run guidellm benchmark against Triton backend
      - Compare P99 latency with vLLM baseline
      - Verify translation overhead < 5ms

  4.4_documentation:
    files:
      - docs/platform/model-access-control.md
      - docs/runbooks/ai-debugging-workflow.md
    changes:
      - Add backend_type configuration examples
      - Add Triton error debugging section
```

## Risk Assessment

| Phase | Risk | Mitigation |
|-------|------|------------|
| Phase 1 | LOW | New fields are optional, defaults preserve existing behavior |
| Phase 2 | MEDIUM | Isolated package, no production impact until Phase 3 |
| Phase 3 | HIGH | Feature flag option; Triton backends fail gracefully if translation fails |
| Phase 4 | LOW | Validation only, no code changes |

## Rollback Plan

```yaml
phase_1_rollback:
  steps:
    - Run down migration: DROP COLUMN backend_type, tokenizer
    - Remove new fields from structs
    - Redeploy services

phase_2_rollback:
  steps:
    - Delete internal/adapter/triton/ directory
    - Remove tiktoken-go dependency
    - No production impact

phase_3_rollback:
  steps:
    - Revert openai.go changes
    - Triton-configured policies return "unsupported backend_type" error
    - OpenAI backends unaffected

phase_4_rollback:
  steps:
    - Disable Triton routing policies via admin API
    - No code changes needed
```

## Success Metrics

| Metric | Target | Measurement |
|--------|--------|-------------|
| Translation latency | < 5ms | Prometheus histogram |
| Token count accuracy | > 99% | Compare tiktoken vs response |
| Error rate | Same as vLLM | Grafana dashboard |
| Test coverage | > 80% | go test -cover |

## Estimated Task Count

| Phase | Tasks | Files |
|-------|-------|-------|
| Phase 1 | 5 | 7 |
| Phase 2 | 5 | 7 |
| Phase 3 | 6 | 5 |
| Phase 4 | 4 | 2 |
| **Total** | **20** | **21** |

## Next Step

Run `/jb-3-tasks` to create detailed task beads for implementation tracking.
