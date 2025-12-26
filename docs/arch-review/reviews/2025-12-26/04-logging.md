# Theme 4: Logging & Observability

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-ylrm

## Summary

The platform has an excellent shared logging library foundation (4.7/5) with standardized Zap integration and OpenTelemetry support. However, adoption is inconsistent across services, particularly in admin-api (uses raw zap) and user-org-service (no request logging middleware).

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| admin-api-service | 3.8/5 | Raw zap.NewProduction(), missing trace context |
| api-router-service | 4.5/5 | Full trace propagation, excellent |
| analytics-service | 4/5 | Good OTEL, missing Prometheus metrics |
| user-org-service | 3.5/5 | No request logging middleware, no metrics |
| ai-model-operator | 4.5/5 | Controller-runtime integration |
| ai-aas-cli | 3/5 | Custom logger (appropriate for CLI) |
| shared/go/logging | 4.7/5 | Excellent foundation |

**Average Score:** 4.0/5

## Criteria Checklist

- [x] Structured JSON logging - **GOOD** (via shared library)
- [x] Consistent log levels - **EXCELLENT** (5/5)
- [ ] Trace ID propagation - **PARTIAL** (api-router only)
- [ ] Request ID in all logs - **PARTIAL** (api-router only)
- [x] No sensitive data in logs - **GOOD** (redaction patterns)
- [ ] Prometheus metrics exposed - **PARTIAL** (60% coverage)
- [x] Health/ready endpoints - **EXCELLENT** (all services)

## Critical Gaps

### 1. Admin-API Uses Raw Zap

**File:** `services/admin-api-service/cmd/admin-api/main.go:22`
```go
logger, err := zap.NewProduction()  // ❌ Missing shared library
```
**Impact:** No service name, environment, sampling, or trace context.

### 2. User-Org-Service Missing Request Logger

No request logging middleware configured - cannot correlate requests.

### 3. Analytics Missing Prometheus Metrics

Zero metrics defined despite background workers (rollup, ingestion, export).

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P0 | Migrate admin-api to shared logging library | admin-api | Low | TBD |
| P0 | Add request logger middleware to user-org | user-org | Low | TBD |
| P1 | Add worker metrics to analytics | analytics | Medium | TBD |
| P1 | Fix audit log levels (denial=WARN) | api-router | Low | TBD |

## Files Examined

- `services/admin-api-service/cmd/admin-api/main.go:22` (raw zap)
- `services/api-router-service/internal/telemetry/telemetry.go:55-87` (shared library)
- `shared/go/logging/logger.go:22-126` (excellent)
- `shared/go/middleware/request_logger.go:49-210` (unused by some services)
