# Implementation Plan: Token Rate-Limit Policies

**Spec:** 035-token-rate-limits
**Date:** 2026-01-01
**Status:** Draft

## Overview

This plan implements token-based rate limiting with org-scoped policies, per-user overrides, and OpenAI-compatible enforcement. The implementation follows the 4-phase migration strategy from `impact.md`.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           API Flow                                       │
└─────────────────────────────────────────────────────────────────────────┘

   Client Request
         │
         ▼
┌─────────────────┐     1. Check cached limits     ┌──────────────────────┐
│   API Router    │◄─────────────────────────────►│   In-Memory Cache    │
│                 │     (60s TTL)                  │   (policy + usage)   │
└────────┬────────┘                                └──────────────────────┘
         │                                                    ▲
         │ 2. If limit exceeded → 429                         │
         │ 3. If allowed → forward                            │
         ▼                                                    │
┌─────────────────┐                                          │
│   Inference     │                                          │
│   Backend       │                                          │
└────────┬────────┘                                          │
         │                                                    │
         │ 4. Response (with token counts)                   │
         ▼                                                    │
┌─────────────────┐     5. Report usage            ┌─────────┴──────────┐
│   API Router    │─────────────────────────────►│  User-Org Service  │
│   (post-req)    │                                │  (source of truth) │
└─────────────────┘                                └────────────────────┘
                                                             │
                                                             ▼
                                                   ┌────────────────────┐
                                                   │    PostgreSQL      │
                                                   │  (policies, usage) │
                                                   └────────────────────┘
```

---

## Component Breakdown

### 1. Database Layer (user-org-service)

#### Migration: `000008_token_policies.sql`

```sql
-- Token rate-limit policies (org-scoped)
CREATE TABLE token_rate_limit_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES organizations(org_id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT,
    limit_1h BIGINT,      -- null = unlimited
    limit_24h BIGINT,     -- null = unlimited
    limit_7d BIGINT,      -- null = unlimited
    is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(org_id, name)
);

-- System "No Token Rate-Limit" policy (org_id = NULL, is_builtin = TRUE)
INSERT INTO token_rate_limit_policies (id, org_id, name, is_builtin)
VALUES ('00000000-0000-0000-0000-000000000000', NULL, 'No Token Rate-Limit', TRUE);

-- Org default token policy (FK to policies)
ALTER TABLE organizations
    ADD COLUMN default_token_policy_id UUID
    REFERENCES token_rate_limit_policies(id);

-- Per-user token policy override
ALTER TABLE users
    ADD COLUMN token_policy_override_id UUID
    REFERENCES token_rate_limit_policies(id);

-- Rolling window usage tracking
CREATE TABLE token_usage_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    window_type TEXT NOT NULL CHECK (window_type IN ('1h', '24h', '7d')),
    window_start TIMESTAMPTZ NOT NULL,
    tokens_used BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(user_id, window_type, window_start)
);

CREATE INDEX idx_token_usage_user_window ON token_usage_windows(user_id, window_type, window_start);
```

#### Repository: `token_policies.go`

| Method | Description |
|--------|-------------|
| `CreatePolicy(ctx, orgID, name, limits)` | Create org-scoped policy |
| `GetPolicy(ctx, policyID)` | Get policy by ID |
| `GetPolicyByName(ctx, orgID, name)` | Get policy by org + name |
| `ListPolicies(ctx, orgID)` | List policies including built-in |
| `UpdatePolicy(ctx, policyID, updates)` | Update policy limits |
| `DeletePolicy(ctx, policyID)` | Delete (fails if in use) |
| `GetBuiltinPolicy(ctx)` | Get "No Token Rate-Limit" |
| `SetOrgDefaultPolicy(ctx, orgID, policyID)` | Set org default |
| `GetOrgDefaultPolicy(ctx, orgID)` | Get org's default policy |
| `SetUserPolicyOverride(ctx, userID, policyID)` | Set user override |
| `ClearUserPolicyOverride(ctx, userID)` | Remove user override |
| `GetEffectivePolicy(ctx, userID)` | Get user's effective policy |

#### Repository: `token_usage.go`

| Method | Description |
|--------|-------------|
| `RecordUsage(ctx, userID, tokens)` | Add tokens to all windows |
| `GetUsage(ctx, userID)` | Get current usage per window |
| `GetUsageForPolicy(ctx, userID, policy)` | Get usage with limits |
| `CleanupExpiredWindows(ctx)` | Prune old window records |

### 2. HTTP Handlers (user-org-service)

#### Package: `httpapi/tokenpolicies`

| Endpoint | Handler | Scope Required |
|----------|---------|----------------|
| `POST /v1/orgs/{orgId}/token-policies` | CreatePolicy | `org:admin` |
| `GET /v1/orgs/{orgId}/token-policies` | ListPolicies | `org:read` |
| `GET /v1/orgs/{orgId}/token-policies/{id}` | GetPolicy | `org:read` |
| `PUT /v1/orgs/{orgId}/token-policies/{id}` | UpdatePolicy | `org:admin` |
| `DELETE /v1/orgs/{orgId}/token-policies/{id}` | DeletePolicy | `org:admin` |
| `GET /v1/orgs/{orgId}/token-policy` | GetOrgDefault | `org:read` |
| `PUT /v1/orgs/{orgId}/token-policy` | SetOrgDefault | `org:admin` |
| `GET /v1/orgs/{orgId}/users/{userId}/token-policy` | GetUserPolicy | `org:read` |
| `PUT /v1/orgs/{orgId}/users/{userId}/token-policy` | SetUserPolicy | `org:admin` |
| `DELETE /v1/orgs/{orgId}/users/{userId}/token-policy` | ClearUserPolicy | `org:admin` |

#### Package: `httpapi/tokenusage`

| Endpoint | Handler | Scope Required |
|----------|---------|----------------|
| `GET /v1/usage/tokens` | GetOwnUsage | `user:read` (self) |
| `GET /v1/orgs/{orgId}/users/{userId}/usage/tokens` | GetUserUsage | `org:read` |

#### Internal Endpoint (M2M)

| Endpoint | Handler | Auth |
|----------|---------|------|
| `POST /internal/v1/usage/tokens` | RecordUsage | M2M token |
| `GET /internal/v1/users/{userId}/rate-limit` | CheckRateLimit | M2M token |

### 3. API Router Integration

#### Refactor: `limiter/token_limiter.go` (was budget_client.go)

```go
type TokenLimiter struct {
    userOrgEndpoint string
    cache           *RateLimitCache  // in-memory, 60s TTL
    logger          *zap.Logger
    client          *http.Client
}

type RateLimitStatus struct {
    Allowed        bool
    Windows        []WindowStatus
    BlockingWindow *WindowStatus  // which window is blocking (if any)
}

type WindowStatus struct {
    Window    string  // "1h", "24h", "7d"
    Limit     int64
    Used      int64
    Remaining int64
    ResetsAt  time.Time
}

func (l *TokenLimiter) CheckLimit(ctx context.Context, userID string) (*RateLimitStatus, error)
func (l *TokenLimiter) RecordUsage(ctx context.Context, userID string, tokens int64) error
```

#### Update: Request flow in API Router

```go
// Pre-request middleware
func (h *Handler) rateLimitMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        userID := middleware.GetUserID(r.Context())

        status, err := h.limiter.CheckLimit(r.Context(), userID)
        if err != nil {
            // Graceful degradation: allow if service unavailable
            h.logger.Warn("rate limit check failed", zap.Error(err))
            next.ServeHTTP(w, r)
            return
        }

        if !status.Allowed {
            h.writeRateLimitError(w, status.BlockingWindow)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// Post-request: record actual usage
func (h *Handler) recordUsage(userID string, resp *InferenceResponse) {
    tokens := resp.Usage.TotalTokens
    go h.limiter.RecordUsage(context.Background(), userID, tokens)
}
```

#### OpenAI-Compatible 429 Response

```go
func (h *Handler) writeRateLimitError(w http.ResponseWriter, window *WindowStatus) {
    w.Header().Set("x-ratelimit-limit-tokens", strconv.FormatInt(window.Limit, 10))
    w.Header().Set("x-ratelimit-remaining-tokens", strconv.FormatInt(window.Remaining, 10))
    w.Header().Set("x-ratelimit-reset-tokens", window.ResetsAt.Format(time.RFC3339))
    w.Header().Set("Retry-After", strconv.Itoa(int(time.Until(window.ResetsAt).Seconds())))

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusTooManyRequests)

    json.NewEncoder(w).Encode(map[string]interface{}{
        "error": map[string]interface{}{
            "message":   fmt.Sprintf("Rate limit exceeded: %s token limit (%d) reached. Resets at %s",
                         window.Window, window.Limit, window.ResetsAt.Format(time.RFC3339)),
            "type":      "tokens",
            "code":      "rate_limit_exceeded",
            "window":    window.Window,
            "limit":     window.Limit,
            "used":      window.Used,
            "resets_at": window.ResetsAt,
        },
    })
}
```

### 4. CLI Commands

#### ai-aas-org (Org Admin CLI)

```
ai-aas-org token-policy create --name NAME [--1h LIMIT] [--24h LIMIT] [--7d LIMIT]
ai-aas-org token-policy list
ai-aas-org token-policy show POLICY_NAME
ai-aas-org token-policy update POLICY_NAME [--1h LIMIT] [--24h LIMIT] [--7d LIMIT]
ai-aas-org token-policy delete POLICY_NAME

ai-aas-org token-policy set-default --policy POLICY_NAME
ai-aas-org token-policy get-default

ai-aas-org user set-token-policy --user USER --policy POLICY_NAME
ai-aas-org user set-token-policy --user USER --policy inherit
ai-aas-org user usage --user USER
```

#### ai-aas-cli (Platform Admin CLI)

```
ai-aas-cli org token-policy --org ORG  # (same commands, org-scoped)
```

---

## Implementation Phases

### Phase 1: Database & Core API (user-org-service)
**Estimated Tasks:** 8 | **Risk:** MEDIUM

| # | Task | File | Notes |
|---|------|------|-------|
| 1.1 | Create migration | `migrations/sql/000008_token_policies.sql` | Tables + indexes |
| 1.2 | Implement policy repository | `storage/postgres/token_policies.go` | CRUD operations |
| 1.3 | Implement usage repository | `storage/postgres/token_usage.go` | Window tracking |
| 1.4 | Add policy repository tests | `storage/postgres/token_policies_test.go` | Unit tests |
| 1.5 | Add usage repository tests | `storage/postgres/token_usage_test.go` | Unit tests |
| 1.6 | Implement policy handlers | `httpapi/tokenpolicies/handlers.go` | REST endpoints |
| 1.7 | Implement usage handlers | `httpapi/tokenusage/handlers.go` | Usage query |
| 1.8 | Register routes | `server/server.go` | Mount new handlers |

### Phase 2: API Router Integration
**Estimated Tasks:** 5 | **Risk:** MEDIUM | **Depends on:** Phase 1

| # | Task | File | Notes |
|---|------|------|-------|
| 2.1 | Refactor BudgetClient → TokenLimiter | `limiter/token_limiter.go` | Rename + rewrite |
| 2.2 | Add in-memory cache | `limiter/cache.go` | 60s TTL |
| 2.3 | Update rate limit middleware | `handler/middleware.go` | Pre-request check |
| 2.4 | Add usage reporting | `handler/inference.go` | Post-request |
| 2.5 | Add OpenAI 429 response | `handler/errors.go` | Headers + body |

### Phase 3: CLI Updates
**Estimated Tasks:** 4 | **Risk:** LOW | **Depends on:** Phase 1

| # | Task | File | Notes |
|---|------|------|-------|
| 3.1 | Add token-policy command | `ai-aas-org/cmd/token_policy.go` | Full CRUD |
| 3.2 | Add user token-policy subcommand | `ai-aas-org/cmd/user.go` | Override mgmt |
| 3.3 | Update userorg client | `ai-aas-cli/client/userorg/` | Types + methods |
| 3.4 | Deprecate budget command | `ai-aas-cli/admin/budget.go` | Alias + warning |

### Phase 4: E2E Tests & Cleanup
**Estimated Tasks:** 4 | **Risk:** LOW | **Depends on:** Phases 1-3

| # | Task | File | Notes |
|---|------|------|-------|
| 4.1 | Rewrite E2E fixture | `tests/e2e/fixtures/token_policies.go` | New API |
| 4.2 | Rewrite E2E tests | `tests/e2e/suites/token_rate_limit_test.go` | Full coverage |
| 4.3 | Remove deprecated schemas | `specs/005-*/contracts/*.yaml` | Cleanup |
| 4.4 | Update documentation | `docs/` | User guide |

---

## Dependencies

```
┌─────────────────────────────────────────────────────────────┐
│                    Phase 1 (user-org-service)               │
│  Migration → Repositories → Handlers → Route Registration   │
└───────────────────────────┬─────────────────────────────────┘
                            │
            ┌───────────────┴───────────────┐
            │                               │
            ▼                               ▼
┌───────────────────────┐       ┌───────────────────────┐
│   Phase 2 (Router)    │       │    Phase 3 (CLI)      │
│  Limiter + Middleware │       │  Commands + Client    │
└───────────┬───────────┘       └───────────┬───────────┘
            │                               │
            └───────────────┬───────────────┘
                            │
                            ▼
            ┌───────────────────────────────┐
            │      Phase 4 (E2E Tests)      │
            │  Fixtures + Tests + Cleanup   │
            └───────────────────────────────┘
```

---

## Testing Strategy

| Level | Scope | Location |
|-------|-------|----------|
| **Unit** | Repository methods | `storage/postgres/*_test.go` |
| **Unit** | Handler logic | `httpapi/*/handlers_test.go` |
| **Unit** | Limiter logic | `limiter/*_test.go` |
| **Integration** | Router ↔ User-Org | `api-router-service/integration_test.go` |
| **E2E** | Full policy lifecycle | `tests/e2e/suites/token_rate_limit_test.go` |
| **E2E** | Enforcement | `tests/e2e/suites/token_rate_limit_test.go` |

### Key Test Scenarios

1. **Policy CRUD** - Create, read, update, delete policies
2. **Org Default** - Set default, verify inheritance
3. **User Override** - Override, verify precedence, clear
4. **Usage Query** - Verify % and absolute values
5. **Enforcement** - Verify 429 when limit exceeded
6. **Overage** - Allow current request, block next
7. **Cache Refresh** - Verify 60s TTL behavior
8. **Graceful Degradation** - Allow if user-org unavailable

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| API Router latency | In-memory cache with 60s TTL |
| User-Org unavailable | Graceful degradation (allow requests) |
| Database migration fails | Tested in dev first, standard rollback |
| CLI breaking change | Deprecation alias for `budget` command |

---

## Open Items (from impact.md)

1. [x] Confirm migration number (000008) - **Confirmed: 000007 is latest**
2. [ ] Decide on deprecation period for `ai-aas-cli budget` command
3. [ ] Verify caching strategy (in-memory vs Redis) - **Recommend: in-memory for simplicity**

---

## Estimated Effort

| Phase | Tasks | Complexity |
|-------|-------|------------|
| Phase 1 | 8 | Medium |
| Phase 2 | 5 | Medium |
| Phase 3 | 4 | Low |
| Phase 4 | 4 | Low |
| **Total** | **21** | |
