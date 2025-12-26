# Impact Analysis: Triton API Support

**Spec**: [spec.md](./spec.md)
**Analyzed**: 2025-12-21
**Type**: Feature Addition with Code Modification

## Summary

Adding Triton backend support requires modifying the API Router's request handling flow and extending the routing policy schema with `backend_type` and `tokenizer` fields. Impact is concentrated in `api-router-service` with ripple effects to `admin-api-service` for schema changes.

## Impact Signals

| Signal from Spec | Search Pattern | Findings |
|------------------|----------------|----------|
| "Add backend_type to routing policy" | `RoutingPolicy\|BackendWeight` | 28 files |
| "Detect backend type from routing policy" | `GetPolicy\|policy\.Backends` | 12 files |
| "Translate OpenAI to Triton V2" | `adapter\|kserve\|translator` | 4 files (template exists) |
| "Use tiktoken for token counts" | `estimateTokens\|tokenizer` | 1 file (kserve/translator.go) |

## Affected Components

```yaml
components:
  services/api-router-service/:
    files: 12
    risk: HIGH
    changes: [MODIFY, ADD]
    details:
      - internal/api/public/openai.go (MODIFY - add backend type branching)
      - internal/config/loader.go (MODIFY - schema update)
      - internal/config/cache.go (MODIFY - store backend_type)
      - internal/adapter/triton/ (ADD - new package)

  services/admin-api-service/:
    files: 4
    risk: MEDIUM
    changes: [MODIFY]
    details:
      - internal/domain/policy.go (MODIFY - add BackendType, Tokenizer fields)
      - internal/repository/policies.go (MODIFY - database schema)
      - internal/api/handlers/policies.go (MODIFY - validation)

  services/api-router-service/test/:
    files: 8
    risk: LOW
    changes: [ADD, UPDATE]
    details:
      - New unit tests for Triton adapter
      - Update integration tests for backend_type routing

  docs/:
    files: 2
    risk: LOW
    changes: [UPDATE]
```

## Detailed Findings

### ADD

| File | What | Risk | Notes |
|------|------|------|-------|
| `services/api-router-service/internal/adapter/triton/types.go` | Triton V2 protocol types | MEDIUM | New file, follows kserve pattern |
| `services/api-router-service/internal/adapter/triton/translator.go` | OpenAI ↔ Triton translation | HIGH | Core translation logic |
| `services/api-router-service/internal/adapter/triton/tokenizer.go` | Tiktoken integration | MEDIUM | Token counting for billing |
| `services/api-router-service/internal/adapter/triton/translator_test.go` | Unit tests | LOW | Required for coverage |

### MODIFY

| File | Lines | What | Risk | Notes |
|------|-------|------|------|-------|
| `services/api-router-service/internal/api/public/openai.go` | 130-165 | Add backend type detection and branching | HIGH | Production request path |
| `services/api-router-service/internal/config/loader.go` | 54-64 | Add BackendType, Tokenizer to RoutingPolicy struct | MEDIUM | Schema change |
| `services/api-router-service/internal/config/cache.go` | - | Store/retrieve backend_type | MEDIUM | Cache key strategy |
| `services/admin-api-service/internal/domain/policy.go` | 14-30 | Add BackendType, Tokenizer to RoutingPolicy | MEDIUM | API contract change |
| `services/admin-api-service/internal/repository/policies.go` | - | Database column additions | MEDIUM | Migration required |

### UPDATE (Tests)

| File | What |
|------|------|
| `services/api-router-service/internal/adapter/kserve/translator_test.go` | Reference for new Triton tests |
| `services/api-router-service/test/integration/openai_chat_test.go` | Add Triton backend test cases |
| `services/api-router-service/test/integration/openai_chat_e2e_test.go` | E2E test with Triton mock |
| `services/api-router-service/internal/config/loader_test.go` | Test backend_type handling |

### UPDATE (Documentation)

#### `/docs/` - Human Documentation

| File | What | Audience |
|------|------|----------|
| `docs/platform/model-access-control.md` | Add backend_type configuration examples | Operators |
| `docs/runbooks/ai-debugging-workflow.md` | Add Triton error debugging | On-call |

#### `/context/` - Agent Context

| File | What | Agent |
|------|------|-------|
| `context/go-services-developer/agents.md` | Add Triton adapter patterns | go-services-developer |

## Migration Order

```yaml
phase_1_prepare:
  description: "Add new types and schema, backward compatible"
  tasks:
    - Add BackendType and Tokenizer fields to domain/policy.go
    - Add database migration for new columns (nullable, defaults)
    - Add BackendType to api-router RoutingPolicy struct
    - Update config cache to handle new fields
  risk: LOW
  rollback: "Remove new fields, run down migration"

phase_2_implement:
  description: "Implement Triton adapter"
  tasks:
    - Create internal/adapter/triton/ package
    - Implement Triton V2 types (types.go)
    - Implement translator (translator.go)
    - Integrate tiktoken for token counting
    - Add unit tests
  risk: MEDIUM
  rollback: "Delete triton/ package, no production impact"

phase_3_integrate:
  description: "Wire up backend type routing"
  tasks:
    - Modify openai.go to detect backend_type
    - Add Triton request forwarding path
    - Add error mapping (Triton → OpenAI)
    - Add observability (logs, metrics)
    - Integration tests
  risk: HIGH
  rollback: "Revert openai.go changes, Triton backends fail gracefully"

phase_4_validate:
  description: "End-to-end validation"
  tasks:
    - Configure Triton backend in development
    - Validate with OpenAI SDK
    - Load test
    - Update documentation
  risk: LOW
  rollback: "Disable Triton routing policy"
```

## Dependencies

```yaml
external_dependencies:
  - tiktoken-go: "github.com/pkoukk/tiktoken-go" (new dependency)
  - Triton Inference Server: Existing deployment (llama-3-1-8b-instruct-trtllm)

internal_dependencies:
  - RoutingPolicy domain type (admin-api-service)
  - config.Loader (api-router-service)
  - routing.BackendEndpoint (api-router-service)

cross_service_impact:
  - admin-api-service: Schema change for backend_type
  - api-router-service: Core implementation
  - web-portal: May need UI update for backend_type field (future)
```

## Test Coverage

| Component | Current Coverage | Needs Update |
|-----------|------------------|--------------|
| `internal/adapter/kserve/` | ~80% | No (reference only) |
| `internal/adapter/triton/` | N/A | Yes - new package needs tests |
| `internal/api/public/openai.go` | ~70% | Yes - backend type branching |
| `internal/config/loader.go` | ~65% | Yes - new fields |
| Integration tests | Good | Yes - Triton backend scenarios |

## Rollback Plan

| Phase | Rollback Strategy |
|-------|-------------------|
| Phase 1 | Run down migration, remove new struct fields |
| Phase 2 | Delete `internal/adapter/triton/` directory |
| Phase 3 | Revert `openai.go` changes; Triton policies return "unsupported backend" error |
| Phase 4 | Disable Triton routing policy in admin API |

## Risk Assessment Summary

| Risk Level | Count | Items |
|------------|-------|-------|
| **HIGH** | 2 | `openai.go` request path, `translator.go` core logic |
| **MEDIUM** | 5 | Schema changes, config handling, tiktoken integration |
| **LOW** | 6 | Tests, documentation, new types |

## Open Questions

1. **Tiktoken library choice**: Use `pkoukk/tiktoken-go` or implement subset? (Recommend: use library)
2. **Database migration strategy**: Add columns in Phase 1 or defer? (Recommend: Phase 1 with nullable defaults)

## Next Step

`/jb-3-plan` - Plan will incorporate migration phases from this analysis.
