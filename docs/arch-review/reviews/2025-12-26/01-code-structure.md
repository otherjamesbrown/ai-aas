# Theme 1: Code Structure & Reuse

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-ooqp

## Summary

The platform has established good foundations with shared libraries and consistent layered architecture patterns. However, significant code duplication exists across services (particularly API key validation and HTTP response handling), package organization is inconsistent, and several large functions need decomposition.

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| admin-api-service | 3/5 | Good structure but dual service dirs, large functions |
| api-router-service | 4/5 | Best patterns, good adapters, centralized errors |
| analytics-service | 3.5/5 | Clean separation, missing standard response patterns |
| user-org-service | 3.5/5 | Comprehensive auth but huge middleware files |
| ai-model-operator | 3/5 | Isolated implementation, no shared clients |
| ai-aas-cli | 2.5/5 | Most inconsistent, 15+ internal packages |
| internal (shared) | 4/5 | Good foundations but missing HTTP/API key utilities |

**Average Score:** 3.4/5

## Criteria Checklist

- [x] Consistent package organization across services - **PARTIAL** (varies by service)
- [ ] Shared code in `internal/` properly extracted - **GAPS** (API key, HTTP responses missing)
- [ ] No duplicated code between services - **FAIL** (250+ lines duplicated)
- [ ] Functions < 50 lines (guideline) - **PARTIAL** (several 78-180 line functions)
- [x] Clear separation of concerns - **GOOD** (API → Service → Repository pattern)
- [x] Consistent naming conventions - **PARTIAL** (error types inconsistent)

## Detailed Findings

### admin-api-service

**Score: 3/5**

**Strengths:**
- Clear separation between handlers → services → repositories (3-layer pattern)
- Dedicated `httputil/response.go` for consistent JSON/error responses
- Well-organized handler structure (models, recipes, pods, organizations, policies)
- Repository pattern with interface-based abstraction

**Gaps:**
- Has both `service/` AND `services/` directories (confusing naming)
- `services/models/service.go` is 422 lines with `AddModel()` at 78 lines
- `httputil` only in this service, not in shared/
- No shared error codes

**Files Examined:**
- `services/admin-api-service/internal/services/models/service.go:78` - AddModel too large
- `services/admin-api-service/internal/handlers/models/handler.go:34`
- `services/admin-api-service/internal/repository/models.go:301`

---

### api-router-service

**Score: 4/5**

**Strengths:**
- Centralized error handling in `internal/api/errors.go` (326 lines, 22 error codes)
- Good adapter pattern for backend communication (Triton, KServe)
- Well-organized auth package with multiple authenticators
- Extensive use of context-based middleware

**Gaps:**
- Error structure incompatible with admin-api (`ErrorResponse{Error string, Code string}` vs `ErrorResponse{Error ErrorDetail}`)
- `internal/api/public/handler.go` is 435 lines
- Auth middleware not shared with other services

**Files Examined:**
- `services/api-router-service/internal/api/errors.go:326`
- `services/api-router-service/internal/api/public/handler.go:435`
- `services/api-router-service/internal/auth/authenticator.go:121-136`

---

### analytics-service

**Score: 3.5/5**

**Strengths:**
- Clean separation: storage (postgres), ingestion (Kafka), aggregation
- Good use of shared middleware from `shared-go`
- Proper RBAC integration

**Gaps:**
- Custom RBAC wrapper in `internal/middleware/rbac.go` with path normalization that could be shared
- Uses `http.Error()` directly instead of shared patterns
- No consistent response wrapper

**Files Examined:**
- `services/analytics-service/internal/middleware/rbac.go:66-127`

---

### user-org-service

**Score: 3.5/5**

**Strengths:**
- Comprehensive auth middleware handling OAuth2 and API keys
- Well-structured handler organization
- Good use of sqlc for type-safe database access

**Gaps:**
- `internal/httpapi/middleware/auth.go` is 442 lines doing too much (OAuth + API key + context)
- `tryAPIKeyAuth()` is 85 lines, duplicates logic from other services
- Uses `http.Error()` directly, not standardized

**Files Examined:**
- `services/user-org-service/internal/httpapi/middleware/auth.go:442`
- `services/user-org-service/internal/httpapi/middleware/auth.go:351-433`

---

### ai-model-operator

**Score: 3/5**

**Strengths:**
- Clean separation of concerns (kserve, recipe, adminapi packages)
- Good retry/backoff logic in Admin API client
- Well-structured recipe validation

**Gaps:**
- Reimplements Admin API client (70 lines) when shared library should exist
- Recipe validation not shared with admin-api-service
- Isolated from service patterns

**Files Examined:**
- `operators/ai-model-operator/internal/adminapi/client.go:70`
- `operators/ai-model-operator/internal/recipe/`

---

### ai-aas-cli

**Score: 2.5/5**

**Strengths:**
- Clean error type hierarchy in `internal/errors/errors.go`
- Good credential management
- Separate concern for output formatting

**Gaps:**
- 15+ internal packages with unclear relationships
- Custom error handling not shared
- K8s client not shared with operator
- Registry client duplicates admin-api logic

**Files Examined:**
- `cmd/ai-aas-cli/internal/errors/errors.go:109`
- `cmd/ai-aas-cli/internal/kubernetes/`
- `cmd/ai-aas-cli/internal/registry/`

---

### internal (shared libraries)

**Score: 4/5**

**Strengths:**
- `logging/` - Comprehensive with redaction, config, error helpers
- `observability/` - OTEL integration with middleware
- `auth/` - Policy-based authorization middleware
- `middleware/` - Request logging middleware
- `modelcache/` - Model caching with HuggingFace support

**Gaps:**
- NO HTTP response utilities (admin-api's httputil should be here)
- NO centralized error codes (each service defines own)
- NO API key validation client (reimplemented 3+ times)
- NO auth context extractors (each service defines own context keys)

**Files Examined:**
- `shared/go/auth/middleware.go:48-78`
- `shared/go/logging/`
- `shared/go/observability/`

## Critical Duplications Found

### 1. API Key Validation (3 implementations, 250+ lines total)

| Service | File | Lines |
|---------|------|-------|
| admin-api | `internal/api/middleware/auth.go:29-86` | 87 |
| api-router | `internal/auth/authenticator.go:140-236` | 96 |
| user-org | `internal/httpapi/middleware/auth.go:351-433` | 85 |

**Impact:** When API key validation logic changes, 3 places need updating. Current inconsistency in header handling ("Bearer" vs "X-API-Key").

### 2. HTTP Response Patterns (3 different patterns)

| Pattern | Service | Approach |
|---------|---------|----------|
| httputil | admin-api | `httputil.WriteJSON()`, `httputil.WriteError()` |
| api package | api-router | `api.WriteError()`, `api.WriteLimitError()` |
| stdlib | user-org, analytics | `http.Error()`, `json.NewEncoder()` |

### 3. Error Code Constants

- api-router: 22 error codes in `internal/api/errors.go:24-62`
- admin-api: None (only type names)
- Others: Minimal

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P1 | Extract API key validation to shared library | admin-api, api-router, user-org | Medium | aas-cssz |
| P1 | Consolidate HTTP response utilities to shared | all services | Medium | aas-wd41 |
| P2 | Standardize error codes in shared library | admin-api, api-router | Low | aas-r9ot |
| P2 | Decompose large middleware (442 lines) | user-org | Medium | aas-i18t |
| P3 | Extract Admin API client to shared | operator, cli | Low | - |
| P3 | Standardize package organization | admin-api | Low | - |

## Next Steps

1. Create remediation beads for P0 items
2. Continue to Theme 2: Configuration Management
3. After all themes, prioritize remediation backlog
