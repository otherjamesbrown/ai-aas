# Tasks: Triton API Support

**Spec**: [spec.md](./spec.md)
**Plan**: [plan.md](./plan.md)
**Epic Bead**: ai-aas-spec032
**Implementation Bead**: ai-aas-spec032.2

---

## Phase 1: Schema Extension (Low Risk)

### Task 1.1: Add BackendType and Tokenizer to Domain Types
**Bead**: ai-aas-x0uw
**Priority**: P1
**Complexity**: Low
**Depends On**: None

**Files**:
- `services/admin-api-service/internal/domain/policy.go`

**Changes**:
```go
type RoutingPolicy struct {
    // ... existing fields ...
    BackendType string `json:"backend_type,omitempty"` // "openai" (default) | "triton"
    Tokenizer   string `json:"tokenizer,omitempty"`    // tiktoken encoding name
}
```

**Acceptance Criteria**:
- [ ] BackendType field added with json tag
- [ ] Tokenizer field added with json tag
- [ ] Existing tests pass (fields are optional)

---

### Task 1.2: Create Database Migration
**Bead**: ai-aas-m0r5
**Priority**: P1
**Complexity**: Low
**Depends On**: 1.1

**Files**:
- `services/admin-api-service/migrations/NNNN_add_backend_type.sql`

**Changes**:
```sql
-- +goose Up
ALTER TABLE routing_policies ADD COLUMN backend_type VARCHAR(32) DEFAULT 'openai';
ALTER TABLE routing_policies ADD COLUMN tokenizer VARCHAR(64);

-- +goose Down
ALTER TABLE routing_policies DROP COLUMN backend_type;
ALTER TABLE routing_policies DROP COLUMN tokenizer;
```

**Acceptance Criteria**:
- [ ] Migration file created with goose format
- [ ] Up migration adds columns with defaults
- [ ] Down migration removes columns
- [ ] Migration runs successfully

---

### Task 1.3: Update Repository SQL Queries
**Bead**: ai-aas-kxuv
**Priority**: P1
**Complexity**: Medium
**Depends On**: 1.2

**Files**:
- `services/admin-api-service/internal/repository/policies.go`

**Changes**:
- Add backend_type, tokenizer to SELECT column list
- Add backend_type, tokenizer to INSERT statements
- Add backend_type, tokenizer to UPDATE statements
- Add fields to Scan() calls

**Acceptance Criteria**:
- [ ] All CRUD operations include new fields
- [ ] Repository tests pass
- [ ] Null handling works correctly

---

### Task 1.4: Update API Router Config Structs
**Bead**: ai-aas-3ogy
**Priority**: P1
**Complexity**: Low
**Depends On**: 1.1

**Files**:
- `services/api-router-service/internal/config/loader.go`
- `services/api-router-service/internal/config/cache.go`

**Changes**:
```go
type RoutingPolicy struct {
    // ... existing fields ...
    BackendType string `json:"backend_type,omitempty"`
    Tokenizer   string `json:"tokenizer,omitempty"`
}
```

**Acceptance Criteria**:
- [ ] RoutingPolicy struct updated in loader.go
- [ ] Cache handles new fields
- [ ] Existing config tests pass

---

### Task 1.5: Add API Validation for backend_type
**Bead**: ai-aas-tpwf
**Priority**: P1
**Complexity**: Low
**Depends On**: 1.1

**Files**:
- `services/admin-api-service/internal/api/handlers/policies.go`

**Changes**:
```go
// In CreatePolicy/UpdatePolicy handlers, add validation:
func validateRoutingPolicy(policy *domain.RoutingPolicy) error {
    // Validate backend_type
    if policy.BackendType != "" && policy.BackendType != "openai" && policy.BackendType != "triton" {
        return fmt.Errorf("invalid backend_type: must be 'openai' or 'triton'")
    }

    // Validate tokenizer format (if provided)
    if policy.Tokenizer != "" {
        validTokenizers := []string{"cl100k_base", "llama3", "o200k_base"}
        if !slices.Contains(validTokenizers, policy.Tokenizer) {
            return fmt.Errorf("invalid tokenizer: must be one of %v", validTokenizers)
        }
    }

    // Require tokenizer when backend_type is triton
    if policy.BackendType == "triton" && policy.Tokenizer == "" {
        return fmt.Errorf("tokenizer is required when backend_type is 'triton'")
    }
    return nil
}
```

**Acceptance Criteria**:
- [ ] backend_type validated (empty, "openai", or "triton" only)
- [ ] tokenizer validated against known encodings
- [ ] tokenizer required when backend_type is "triton"
- [ ] Clear error messages returned to API clients
- [ ] Existing policy tests updated

---

## Phase 2: Triton Adapter (Medium Risk)

### Task 2.1: Create Triton V2 Protocol Types
**Bead**: ai-aas-dfje
**Priority**: P1
**Complexity**: Medium
**Depends On**: None

**Files**:
- `services/api-router-service/internal/adapter/triton/types.go`

**Changes**:
```go
// InferRequest represents a Triton V2 inference request
type InferRequest struct {
    ID         string                 `json:"id,omitempty"`
    Inputs     []InferInputTensor     `json:"inputs"`
    Outputs    []InferRequestedOutput `json:"outputs,omitempty"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// InferResponse represents a Triton V2 inference response
type InferResponse struct {
    ID         string              `json:"id"`
    ModelName  string              `json:"model_name"`
    Outputs    []InferOutputTensor `json:"outputs"`
    Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// Tensor types...
```

**Acceptance Criteria**:
- [ ] All Triton V2 types defined
- [ ] JSON tags match Triton protocol spec
- [ ] Types compile without errors

---

### Task 2.2: Implement OpenAI ↔ Triton Translator
**Bead**: ai-aas-1qew
**Priority**: P1
**Complexity**: High
**Depends On**: 2.1

**Files**:
- `services/api-router-service/internal/adapter/triton/translator.go`

**Changes**:
```go
type Translator struct {
    tokenizer *Tokenizer
}

func NewTranslator(tokenizerEncoding string) (*Translator, error)

func (t *Translator) TranslateOpenAIToTriton(req *OpenAIChatCompletionRequest) (*InferRequest, error)

func (t *Translator) TranslateTritonToOpenAI(resp *InferResponse, origReq *OpenAIChatCompletionRequest) (*OpenAIChatCompletionResponse, error)

func (t *Translator) formatPrompt(messages []ChatMessage) (string, error)
```

**Reference**: `internal/adapter/kserve/translator.go`

**Acceptance Criteria**:
- [ ] Request translation preserves all parameters
- [ ] Response translation extracts text correctly
- [ ] Token counts calculated via tokenizer
- [ ] Unit tests cover happy path and edge cases

---

### Task 2.3: Implement Tiktoken Integration
**Bead**: ai-aas-9uww
**Priority**: P1
**Complexity**: Medium
**Depends On**: None

**Files**:
- `services/api-router-service/internal/adapter/triton/tokenizer.go`
- `services/api-router-service/go.mod` (add dependency)

**Changes**:
```go
type Tokenizer struct {
    encoding tiktoken.Encoding
}

func NewTokenizer(encodingName string) (*Tokenizer, error)

func (t *Tokenizer) CountTokens(text string) (int, error)

// Supported encodings: cl100k_base, llama3, o200k_base
```

**Dependency**: `github.com/pkoukk/tiktoken-go`

**Acceptance Criteria**:
- [ ] tiktoken-go added to go.mod
- [ ] Tokenizer supports multiple encodings
- [ ] Token counts match expected values
- [ ] Error handling for unknown encodings

---

### Task 2.4: Implement Error Mapping
**Bead**: ai-aas-446l
**Priority**: P2
**Complexity**: Low
**Depends On**: 2.1

**Files**:
- `services/api-router-service/internal/adapter/triton/errors.go`

**Changes**:
```go
// MapTritonError maps Triton error status to OpenAI error format
func MapTritonError(tritonStatus string, details string) (*OpenAIError, int) {
    // UNAVAILABLE → 503, service_unavailable
    // INVALID_ARG → 400, invalid_request_error
    // NOT_FOUND → 404, model_not_found
    // RESOURCE_EXHAUSTED → 429, rate_limit_exceeded
    // DEADLINE_EXCEEDED → 504, timeout
    // INTERNAL → 500, internal_error
}

type OpenAIError struct {
    Message      string `json:"message"`
    Type         string `json:"type"`
    Code         string `json:"code"`
    TritonDetails string `json:"triton_details,omitempty"`
}
```

**Acceptance Criteria**:
- [ ] All Triton error codes mapped
- [ ] HTTP status codes correct
- [ ] triton_details preserved for debugging

---

### Task 2.5: Add Unit Tests for Adapter
**Bead**: ai-aas-re7n
**Priority**: P1
**Complexity**: Medium
**Depends On**: 2.2, 2.3, 2.4

**Files**:
- `services/api-router-service/internal/adapter/triton/translator_test.go`
- `services/api-router-service/internal/adapter/triton/tokenizer_test.go`
- `services/api-router-service/internal/adapter/triton/errors_test.go`

**Acceptance Criteria**:
- [ ] >80% coverage for translator
- [ ] >80% coverage for tokenizer
- [ ] Error mapping tests cover all cases
- [ ] `go test ./internal/adapter/triton/...` passes

---

## Phase 3: Integration (High Risk)

### Task 3.1: Initialize Translator in Handler
**Bead**: ai-aas-c3gr
**Priority**: P1
**Complexity**: Low
**Depends On**: 2.2

**Files**:
- `services/api-router-service/internal/api/public/handler.go`

**Changes**:
```go
type Handler struct {
    // ... existing fields ...
    tritonTranslators map[string]*triton.Translator // keyed by tokenizer encoding
    translatorMu      sync.RWMutex
}

func NewHandler(...) *Handler {
    return &Handler{
        // ... existing fields ...
        tritonTranslators: make(map[string]*triton.Translator),
    }
}

// getOrCreateTranslator returns a cached translator or creates one for the encoding
func (h *Handler) getOrCreateTranslator(encoding string) (*triton.Translator, error) {
    h.translatorMu.RLock()
    if t, ok := h.tritonTranslators[encoding]; ok {
        h.translatorMu.RUnlock()
        return t, nil
    }
    h.translatorMu.RUnlock()

    h.translatorMu.Lock()
    defer h.translatorMu.Unlock()
    // Double-check after acquiring write lock
    if t, ok := h.tritonTranslators[encoding]; ok {
        return t, nil
    }
    t, err := triton.NewTranslator(encoding)
    if err != nil {
        return nil, err
    }
    h.tritonTranslators[encoding] = t
    return t, nil
}
```

**Design Decision**: Use lazy initialization with caching per tokenizer encoding. This avoids loading all tokenizer encodings at startup (memory efficient) while ensuring each encoding is only initialized once (performance).

**Acceptance Criteria**:
- [ ] Handler has tritonTranslators map field
- [ ] getOrCreateTranslator implements lazy init with caching
- [ ] Thread-safe access with RWMutex
- [ ] Existing handler tests pass

---

### Task 3.2: Add Backend Type Branching
**Bead**: ai-aas-av4v
**Priority**: P1
**Complexity**: High
**Depends On**: 3.1, 1.4

**Files**:
- `services/api-router-service/internal/api/public/openai.go`

**Changes**:
```go
// In HandleOpenAIChatCompletions, after GetPolicy():
switch policy.BackendType {
case "triton":
    h.handleTritonChatCompletion(ctx, w, r, policy, openAIReq)
    return
default: // "openai" or empty
    // existing vLLM forwarding logic
}
```

**Risk**: HIGH - modifies production request path

**Acceptance Criteria**:
- [ ] Backend type switch added after GetPolicy()
- [ ] Empty/missing backend_type defaults to "openai"
- [ ] Existing vLLM flow unchanged
- [ ] Integration tests pass

---

### Task 3.3: Implement Triton Request Forwarding
**Bead**: ai-aas-fp9r
**Priority**: P1
**Complexity**: High
**Depends On**: 3.2

**Files**:
- `services/api-router-service/internal/api/public/openai.go`

**Changes**:
```go
func (h *Handler) handleTritonChatCompletion(
    ctx context.Context,
    w http.ResponseWriter,
    r *http.Request,
    policy *config.RoutingPolicy,
    req OpenAIChatCompletionRequest,
) {
    // 1. Build Triton endpoint: /v2/models/{model}/infer
    // 2. Translate request
    // 3. POST to Triton
    // 4. Parse response
    // 5. Translate back to OpenAI
    // 6. Handle errors with mapping
    // 7. Write response
}
```

**Acceptance Criteria**:
- [ ] Triton endpoint URL built correctly
- [ ] Request translated and sent
- [ ] Response parsed and translated
- [ ] Errors mapped to OpenAI format
- [ ] Token counts included in response

---

### Task 3.4: Add Observability
**Bead**: ai-aas-3kt8
**Priority**: P2
**Complexity**: Low
**Depends On**: 3.3

**Files**:
- `services/api-router-service/internal/api/public/openai.go`

**Changes**:
- Add `backend_type` to structured log fields
- Add `triton_translation_duration_ms` metric
- Preserve trace_id across translation

**Acceptance Criteria**:
- [ ] Logs include backend_type field
- [ ] Translation duration metric emitted
- [ ] trace_id preserved in Triton request headers

---

### Task 3.5: Add Integration Tests
**Bead**: ai-aas-pixb
**Priority**: P1
**Complexity**: Medium
**Depends On**: 3.3

**Files**:
- `services/api-router-service/test/integration/triton_backend_test.go`

**Changes**:
- Test with Triton mock server (httptest)
- Test successful translation round-trip
- Test error scenarios (backend unavailable, timeout)
- Test token counting accuracy

**Acceptance Criteria**:
- [ ] Mock Triton server responds correctly
- [ ] Success path test passes
- [ ] Error handling tests pass
- [ ] Token count verification test passes

---

### Task 3.6: Implement Configuration Validation on Sync
**Bead**: ai-aas-mept
**Priority**: P1
**Complexity**: Medium
**Depends On**: 2.3, 1.4

**Files**:
- `services/api-router-service/internal/config/loader.go`

**Changes**:
```go
// In SyncPolicies or after loading policies from Admin API:
func (l *Loader) validateTritonPolicy(policy *RoutingPolicy) error {
    if policy.BackendType != "triton" {
        return nil // No validation needed for non-triton backends
    }

    // 1. Validate tokenizer encoding exists
    if _, err := triton.NewTokenizer(policy.Tokenizer); err != nil {
        return fmt.Errorf("invalid tokenizer encoding %q: %w", policy.Tokenizer, err)
    }

    // 2. Validate backend is reachable (health check)
    healthURL := fmt.Sprintf("http://%s/v2/health/ready", policy.BackendEndpoint)
    resp, err := http.Get(healthURL)
    if err != nil {
        l.logger.Warn("triton backend unreachable",
            zap.String("policy", policy.Name),
            zap.String("endpoint", policy.BackendEndpoint),
            zap.Error(err))
        // Log warning but don't fail - backend may come up later
        return nil
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        l.logger.Warn("triton backend unhealthy",
            zap.String("policy", policy.Name),
            zap.Int("status", resp.StatusCode))
    }

    return nil
}
```

**Design Decision**: Tokenizer validation is strict (fail on invalid), but backend health is a warning only. This allows policies to be synced even if Triton is temporarily down.

**Acceptance Criteria**:
- [ ] Tokenizer encoding validated on sync (strict - fails if invalid)
- [ ] Backend health checked on sync (warning only - logs if unreachable)
- [ ] Clear log messages for configuration issues
- [ ] Validation runs on each policy sync
- [ ] Unit tests for validation logic

**Implements**: spec.md FR-5 (Configuration Validation)

---

## Phase 4: Validation (Low Risk)

### Task 4.1: Development Cluster Validation
**Bead**: ai-aas-7vuf
**Priority**: P1
**Complexity**: Low
**Depends On**: 3.5

**Steps**:
1. Deploy updated api-router-service to development
2. Create routing policy with `backend_type: triton`
3. Test with curl:
   ```bash
   curl -X POST https://api.dev.otherjamesbrown.com/v1/chat/completions \
     -H "Authorization: Bearer $API_KEY" \
     -d '{"model":"meta-llama/Llama-3.1-8B-Instruct","messages":[{"role":"user","content":"Hello"}]}'
   ```
4. Verify response has valid token counts

**Acceptance Criteria**:
- [ ] API Router starts with Triton config
- [ ] Request returns valid OpenAI response
- [ ] Token counts present and reasonable

---

### Task 4.2: SDK Validation
**Bead**: ai-aas-c7ug
**Priority**: P2
**Complexity**: Low
**Depends On**: 4.1

**Steps**:
1. Test with OpenAI Python SDK
2. Test with LangChain
3. Verify streaming returns appropriate error

**Acceptance Criteria**:
- [ ] OpenAI SDK works without modification
- [ ] LangChain integration works
- [ ] Streaming request returns clear error

---

### Task 4.3: Load Test
**Bead**: ai-aas-0let
**Priority**: P2
**Complexity**: Medium
**Depends On**: 4.1

**Steps**:
1. Run guidellm benchmark against Triton backend
2. Measure P99 latency
3. Compare with vLLM baseline
4. Verify translation overhead < 5ms

**Acceptance Criteria**:
- [ ] Load test completes successfully
- [ ] P99 latency within acceptable range
- [ ] Translation overhead measured and < 5ms

---

### Task 4.4: Update Documentation
**Bead**: ai-aas-brww
**Priority**: P3
**Complexity**: Low
**Depends On**: 4.1

**Files**:
- `docs/platform/model-access-control.md`
- `docs/runbooks/ai-debugging-workflow.md`

**Changes**:
- Add backend_type configuration examples
- Add Triton error debugging section
- Document supported tokenizer encodings

**Acceptance Criteria**:
- [ ] Configuration examples added
- [ ] Debugging guide updated
- [ ] Tokenizer encodings documented

---

## Summary

| Phase | Tasks | Priority P1 | Priority P2 | Priority P3 |
|-------|-------|-------------|-------------|-------------|
| Phase 1 | 5 | 5 | 0 | 0 |
| Phase 2 | 5 | 4 | 1 | 0 |
| Phase 3 | 6 | 5 | 1 | 0 |
| Phase 4 | 4 | 1 | 2 | 1 |
| **Total** | **20** | **15** | **4** | **1** |

## Task Dependencies Graph

```
Phase 1:
  1.1 ──► 1.2 ──► 1.3
    │
    ├───────────► 1.4
    │
    └───────────► 1.5 (API validation)

Phase 2:
  2.1 ──► 2.2 ───┐
    │           │
    └──► 2.4    ├──► 2.5
                │
  2.3 ──────────┘

Phase 3:
  2.2 ──► 3.1 ──► 3.2 ──► 3.3 ──► 3.4
  1.4 ────────────┘         │
                            └──► 3.5
  2.3 ─────────────────────────► 3.6 (config validation)
  1.4 ─────────────────────────►

Phase 4:
  3.5 ──► 4.1 ──► 4.2
            │
            ├──► 4.3
            │
            └──► 4.4
```
