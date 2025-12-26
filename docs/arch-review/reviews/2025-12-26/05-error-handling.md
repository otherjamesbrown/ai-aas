# Theme 5: Error Handling

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-6ctb

## Summary

Error handling shows inconsistent patterns across services with three incompatible error schemas. The shared/go/errors package is excellent but underutilized. Only 40-50% of services properly wrap errors with context.

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| shared/go/errors | 5/5 | Excellent schema with options pattern |
| api-router-service | 4/5 | Solid error mapping, typed errors |
| ai-aas-cli | 4/5 | Structured CLI errors with recovery |
| shared/go/logging | 4/5 | ErrorLogger with trace context |
| ai-model-operator | 3/5 | Basic wrapping |
| admin-api-service | 2/5 | Ad-hoc httputil, no typed errors |
| analytics-service | 2/5 | Simplistic handling |
| user-org-service | 2/5 | Simple sentinel errors |

**Average Score:** 3.1/5

## Criteria Checklist

- [ ] Consistent error types defined - **FAIL** (3 different schemas)
- [ ] Error wrapping with context - **PARTIAL** (40-50%)
- [ ] Proper error propagation - **PARTIAL**
- [ ] Client-facing error format standardized - **FAIL** (incompatible)
- [ ] No swallowed errors - **FAIL** (9 instances of `_ = err`)
- [ ] Retryable vs non-retryable distinction - **FAIL** (only api-router)

## Three Incompatible Error Schemas

| Service | Schema |
|---------|--------|
| shared/go/errors | `{error, code, detail, request_id, trace_id, actor, timestamp}` |
| api-router | `{error, code, trace_id}` + rate limit extensions |
| admin-api | `{error: {type, title, detail, status, errors[]}}` |

## Critical Issues

### 1. Error Swallowing (9 instances)

Pattern: `_ = err` ignoring write failures
- `api-router/internal/api/errors.go:238`
- Multiple in `admin_proxy.go`

### 2. Missing Error Wrapping

Many services return errors without `fmt.Errorf("%w", err)` losing context.

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P1 | Unify error response format to shared schema | all services | High | TBD |
| P1 | Eliminate `_ = err` pattern | api-router, admin-api | Low | TBD |
| P2 | Add IsRetryable() to all services | all | Medium | TBD |
| P2 | Wrap database errors consistently | all | Medium | TBD |

## Files Examined

- `shared/go/errors/errors.go` (excellent)
- `services/api-router-service/internal/api/errors.go` (good)
- `services/admin-api-service/internal/httputil/response.go` (ad-hoc)
