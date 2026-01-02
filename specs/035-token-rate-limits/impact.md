# Impact Analysis: Token Rate-Limit Policies

**Spec:** 035-token-rate-limits
**Date:** 2026-01-01
**Status:** Draft

## Executive Summary

This feature replaces the planned monetary budget system with token-based rate limiting. Existing budget infrastructure is partially implemented (stubs, tests, client code) and needs to be refactored to the new token-based approach.

**Impact Level:** MEDIUM
- 11 files affected across 4 services
- No production data migration required (budget tables don't exist yet)
- E2E tests already skipped pending implementation

---

## Change Categories

### DEPRECATE (2 items)

| Item | Location | Risk | Notes |
|------|----------|------|-------|
| Budget OpenAPI endpoints | `specs/005-user-org-service/contracts/user-org-service.openapi.yaml` | LOW | Remove `/v1/budgets/{orgId}/status` and `/v1/budgets/{orgId}/overrides` |
| BudgetStatus/BudgetOverride schemas | Same file | LOW | Replace with TokenRateLimitPolicy schemas |

### MODIFY (6 items)

| Item | Location | Risk | Notes |
|------|----------|------|-------|
| BudgetClient | `services/api-router-service/internal/limiter/budget_client.go` | MEDIUM | Rename to TokenLimiterClient, change from monetary to token-based |
| BudgetClient tests | `services/api-router-service/internal/limiter/budget_client_test.go` | LOW | Update test cases for new API |
| CLI budget command | `services/ai-aas-cli/internal/admin/budget.go` | MEDIUM | Rename to token-policy, update for new API structure |
| CLI budget tests | `services/ai-aas-cli/internal/admin/budget_test.go` | LOW | Update tests |
| UserOrg client types | `services/ai-aas-cli/internal/client/userorg/types.go` | LOW | Add TokenRateLimitPolicy types |
| UserOrg client | `services/ai-aas-cli/internal/client/userorg/client.go` | LOW | Add token policy methods |

### REPLACE (2 items)

| Item | Location | Risk | Notes |
|------|----------|------|-------|
| Budget E2E fixture | `tests/e2e/fixtures/budgets.go` | LOW | Complete rewrite for token rate-limit API |
| Budget E2E tests | `tests/e2e/suites/budget_test.go` | LOW | Complete rewrite for token rate-limit tests |

### ADD (8 items)

| Item | Location | Risk | Notes |
|------|----------|------|-------|
| Token policy migration | `services/user-org-service/migrations/sql/000008_token_policies.sql` | MEDIUM | New tables: token_rate_limit_policies, user_token_policy_overrides, token_usage_windows |
| Token policy repository | `services/user-org-service/internal/storage/postgres/token_policies.go` | LOW | CRUD for policies |
| Token usage repository | `services/user-org-service/internal/storage/postgres/token_usage.go` | LOW | Usage tracking |
| Token policy handlers | `services/user-org-service/internal/httpapi/tokenpolicies/handlers.go` | LOW | API endpoints |
| Token usage handlers | `services/user-org-service/internal/httpapi/tokenusage/handlers.go` | LOW | Usage query endpoints |
| Router registration | `services/user-org-service/internal/server/server.go` | LOW | Register new routes |
| OpenAPI spec | `specs/035-token-rate-limits/contracts/token-rate-limits.openapi.yaml` | LOW | New API contract |
| CLI token-policy command | `services/ai-aas-org/internal/cmd/token_policy.go` | LOW | Org admin CLI commands |

---

## Risk Assessment

### HIGH Risk (0 items)
None - no production data or breaking API changes.

### MEDIUM Risk (3 items)

1. **BudgetClient → TokenLimiterClient refactor**
   - Used by API Router for rate limiting
   - Must maintain graceful degradation behavior
   - Rollback: Revert to stub that allows all requests

2. **CLI budget → token-policy rename**
   - Users may have scripts referencing `budget` subcommand
   - Mitigation: Add deprecation alias for 1 release cycle

3. **Database migration**
   - New tables, no data migration
   - Rollback: Standard migration rollback

### LOW Risk (8 items)
- Test files, type definitions, new code with no existing dependencies

---

## Service Impact Matrix

| Service | Changes | New Files | Risk |
|---------|---------|-----------|------|
| **user-org-service** | 1 (router) | 5 (migration, repos, handlers) | MEDIUM |
| **api-router-service** | 2 (client + tests) | 0 | MEDIUM |
| **ai-aas-cli** | 4 (budget.go, types, client) | 0 | LOW |
| **ai-aas-org** | 0 | 1 (token_policy.go) | LOW |
| **tests/e2e** | 2 (fixture + tests) | 0 | LOW |

---

## Migration Phases

### Phase 1: Database & Core API (user-org-service)
**Order:** 1 | **Rollback:** Drop tables, revert code

1. Add migration `000008_token_policies.sql`
2. Implement token policy repository
3. Implement token usage repository
4. Add token policy handlers
5. Add token usage handlers
6. Register routes in server.go
7. Unit tests for all new code

### Phase 2: API Router Integration
**Order:** 2 | **Rollback:** Revert to stub allowing all requests
**Depends on:** Phase 1

1. Refactor BudgetClient → TokenLimiterClient
2. Update API router to call user-org-service
3. Add caching layer (60s TTL)
4. Update usage reporting (post-request)
5. Add OpenAI-compatible 429 responses

### Phase 3: CLI Updates
**Order:** 3 | **Rollback:** Revert CLI changes
**Depends on:** Phase 1

1. Add token-policy commands to ai-aas-org
2. Update ai-aas-cli budget → token-policy (with alias)
3. Update client types and methods

### Phase 4: E2E Tests & Cleanup
**Order:** 4 | **Rollback:** Skip tests
**Depends on:** Phases 1-3

1. Rewrite budget_test.go → token_rate_limit_test.go
2. Rewrite budgets.go fixture → token_policies.go
3. Remove deprecated OpenAPI budget schemas from spec 005
4. Update documentation

---

## Dependencies

```
Phase 1 (user-org-service)
    │
    ├──► Phase 2 (api-router-service)
    │
    └──► Phase 3 (CLI)
             │
             └──► Phase 4 (E2E tests)
```

---

## Rollback Strategy

| Phase | Rollback Steps | Data Loss |
|-------|----------------|-----------|
| Phase 1 | Run down migration, revert code | None (new tables) |
| Phase 2 | Revert to stub client | None |
| Phase 3 | Revert CLI, keep alias | None |
| Phase 4 | Skip new tests | None |

**Full Rollback Time:** < 30 minutes (no data migration)

---

## Testing Strategy

1. **Unit tests:** Each new repository and handler
2. **Integration tests:** API Router ↔ User-Org service
3. **E2E tests:** Full flow from policy creation to enforcement
4. **Load tests:** Verify < 10ms latency for cached limit checks

---

## Open Items

1. [ ] Confirm migration number (000008) doesn't conflict
2. [ ] Decide on deprecation period for `ai-aas-cli budget` command
3. [ ] Verify Redis is available in API Router for caching (or use in-memory)
