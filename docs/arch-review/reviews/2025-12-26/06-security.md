# Theme 6: Security Practices

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-nlp8

## Summary

Solid security foundation with good auth middleware and parameterized queries. Critical gaps in CLI credential storage (plaintext) and incomplete HMAC signature verification in api-router.

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| admin-api-service | 4.1/5 | Strong auth, good rate limiting |
| api-router-service | 3.9/5 | Good multi-layer auth, HMAC incomplete |
| analytics-service | 3.4/5 | RBAC can be disabled |
| user-org-service | 3.6/5 | Good API key validation |
| ai-model-operator | 3.1/5 | Basic, no input validation |
| ai-aas-cli | 2.3/5 | **CRITICAL: Plaintext secrets** |

**Average Score:** 3.4/5

## Criteria Checklist

- [x] Input validation on endpoints - **GOOD** (4.1/5 average)
- [x] SQL injection prevention - **GOOD** (parameterized queries)
- [ ] No secrets in code - **FAIL** (CLI stores in plaintext)
- [x] Auth middleware applied - **GOOD**
- [ ] RBAC properly scoped - **PARTIAL** (can be disabled)
- [x] Rate limiting configured - **GOOD** (except analytics)
- [x] TLS for external connections - **GOOD**

## Critical Issues

### 1. CLI Secrets in Plaintext (CRITICAL)

**File:** `services/ai-aas-cli/internal/config/config.go:25-27`
```go
APIKey string `mapstructure:"api_key"`
HuggingFaceToken string `mapstructure:"huggingface_token"`
```
Stored in `~/.ai-aas-cli.yaml` - readable by any user on shared systems.

### 2. HMAC Signature Not Implemented (HIGH)

**File:** `services/api-router-service/internal/auth/authenticator.go:300`
```go
secret := []byte("stub-secret") // Placeholder
```
HMAC verification accepts anything - webhooks not actually secured.

### 3. RBAC Can Be Disabled (MEDIUM)

**File:** `services/analytics-service/internal/middleware/rbac.go:92-96`
```go
if !cfg.EnableRBAC {
    cfg.Logger.Warn("RBAC middleware is disabled")
```
Configuration error could bypass all authorization.

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P0 | Move CLI secrets to OS credential manager | cli | Medium | TBD |
| P0 | Implement HMAC signature verification | api-router | Medium | TBD |
| P1 | Enforce RBAC in production | analytics | Low | TBD |
| P1 | Add rate limiting to user-org auth | user-org | Low | TBD |
| P2 | Reduce API key cache TTL | api-router | Low | TBD |

## Files Examined

- `services/ai-aas-cli/internal/config/config.go:25-27` (plaintext)
- `services/api-router-service/internal/auth/authenticator.go:297-312` (stub)
- `services/analytics-service/internal/middleware/rbac.go:91-127`
- `services/admin-api-service/internal/api/middleware/security.go:8-36`
