# Theme 7: API Design

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-0mwi

## Summary

API design shows good REST conventions and versioning in admin-api, but inconsistent error response formats and missing pagination implementation across services. User-org-service uses raw `http.Error()` instead of structured JSON.

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| admin-api-service | 4.3/5 | Excellent OpenAPI, consistent REST |
| api-router-service | 3.5/5 | Good errors, mixed REST patterns |
| analytics-service | 3/5 | RFC 7807 errors, no pagination |
| user-org-service | 3/5 | Good REST, uses http.Error() |

**Average Score:** 3.5/5

## Criteria Checklist

- [x] Consistent REST conventions - **GOOD** (GET/POST/PUT/DELETE)
- [x] Proper HTTP status codes - **GOOD** (varies by service)
- [x] Versioned endpoints - **EXCELLENT** (/v1 everywhere)
- [ ] Consistent error response format - **FAIL** (3+ formats)
- [x] OpenAPI specs maintained - **EXCELLENT** (all services)
- [ ] Pagination implemented - **FAIL** (params accepted, not implemented)

## Error Response Format Inconsistency

| Service | Format |
|---------|--------|
| admin-api | `{error: {type, title, detail, status, errors[]}}` |
| api-router | `{error, code, trace_id, retry_after_seconds}` |
| analytics | `{title, status, timestamp, error}` |
| user-org | Plain text via `http.Error()` |

## Pagination Status

| Service | Parameters | Implementation |
|---------|------------|----------------|
| admin-api | `limit`/`offset` | Partial - no metadata |
| api-router | None | N/A |
| analytics | Query params | Not implemented |
| user-org | `limit`/`offset` | **Ignored** |

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P1 | Replace http.Error() with JSON responses | user-org | Medium | TBD |
| P1 | Implement standard pagination response | all | Medium | TBD |
| P2 | Unify error response schema | all | High | TBD |
| P2 | Standardize path parameter naming | admin-api | Low | TBD |

## Files Examined

- `specs/017-admin-api-service/contracts/openapi.yaml`
- `services/api-router-service/internal/api/errors.go`
- `services/user-org-service/internal/httpapi/apikeys/handlers.go` (http.Error)
