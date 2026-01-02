# Task Breakdown: Token Rate-Limit Policies

**Spec:** 035-token-rate-limits
**Date:** 2026-01-01
**Parent Bead:** aas-2zku

## Task Summary

| Phase | Tasks | Priority | Status |
|-------|-------|----------|--------|
| Phase 1: Database & Core API | 8 | P1 | Pending |
| Phase 2: API Router Integration | 5 | P1 | Pending |
| Phase 3: CLI Updates | 4 | P2 | Pending |
| Phase 4: E2E Tests & Cleanup | 4 | P2 | Pending |
| **Total** | **21** | | |

---

## Phase 1: Database & Core API (user-org-service)

### T-S035-P01-001: Create token policies database migration
**Bead:** aas-eyje6
**Priority:** P1 | **Complexity:** Medium | **Depends on:** None

Create migration `000008_token_policies.sql` with:
- `token_rate_limit_policies` table (id, org_id, name, limits, is_builtin)
- Built-in "No Token Rate-Limit" policy insert
- `organizations.default_token_policy_id` column
- `users.token_policy_override_id` column
- `token_usage_windows` table for rolling window tracking
- Indexes for usage queries

**Files:**
- `services/user-org-service/migrations/sql/000008_token_policies.sql`

**Acceptance Criteria:**
- [ ] Migration applies cleanly
- [ ] Migration rolls back cleanly
- [ ] Built-in policy has well-known UUID

---

### T-S035-P01-002: Implement token policy repository
**Bead:** aas-cgann
**Priority:** P1 | **Complexity:** Medium | **Depends on:** T-S035-P01-001

Implement CRUD operations for token rate-limit policies:
- CreatePolicy, GetPolicy, GetPolicyByName
- ListPolicies (including built-in)
- UpdatePolicy, DeletePolicy
- GetBuiltinPolicy
- SetOrgDefaultPolicy, GetOrgDefaultPolicy
- SetUserPolicyOverride, ClearUserPolicyOverride
- GetEffectivePolicy (user's active policy with inheritance)

**Files:**
- `services/user-org-service/internal/storage/postgres/token_policies.go`

**Acceptance Criteria:**
- [ ] All CRUD operations work
- [ ] Built-in policy returned in list
- [ ] Delete fails if policy in use (409)
- [ ] Effective policy respects override > org default

---

### T-S035-P01-003: Implement token usage repository
**Bead:** aas-yqtzf
**Priority:** P1 | **Complexity:** Medium | **Depends on:** T-S035-P01-001

Implement rolling window usage tracking:
- RecordUsage (add tokens to all applicable windows)
- GetUsage (current usage per window)
- GetUsageForPolicy (usage with limits and percentages)
- CleanupExpiredWindows (prune old records)

**Files:**
- `services/user-org-service/internal/storage/postgres/token_usage.go`

**Acceptance Criteria:**
- [ ] Rolling windows calculated correctly
- [ ] Usage aggregates across windows
- [ ] Cleanup removes expired windows

---

### T-S035-P01-004: Add token policy repository tests
**Bead:** aas-y75dn
**Priority:** P1 | **Complexity:** Low | **Depends on:** T-S035-P01-002

Unit tests for token policy repository:
- CRUD operations
- Built-in policy behavior
- Delete protection when in use
- Effective policy inheritance

**Files:**
- `services/user-org-service/internal/storage/postgres/token_policies_test.go`

**Acceptance Criteria:**
- [ ] >80% coverage on policy repo
- [ ] Edge cases tested (delete in use, duplicate name)

---

### T-S035-P01-005: Add token usage repository tests
**Bead:** aas-0zxsn
**Priority:** P1 | **Complexity:** Low | **Depends on:** T-S035-P01-003

Unit tests for token usage repository:
- Window calculations
- Usage aggregation
- Cleanup logic

**Files:**
- `services/user-org-service/internal/storage/postgres/token_usage_test.go`

**Acceptance Criteria:**
- [ ] >80% coverage on usage repo
- [ ] Window boundary edge cases tested

---

### T-S035-P01-006: Implement token policy HTTP handlers
**Bead:** aas-wa1j6
**Priority:** P1 | **Complexity:** Medium | **Depends on:** T-S035-P01-002

REST handlers for policy management:
- POST /v1/orgs/{orgId}/token-policies (create)
- GET /v1/orgs/{orgId}/token-policies (list)
- GET /v1/orgs/{orgId}/token-policies/{id} (get)
- PUT /v1/orgs/{orgId}/token-policies/{id} (update)
- DELETE /v1/orgs/{orgId}/token-policies/{id} (delete)
- GET/PUT /v1/orgs/{orgId}/token-policy (org default)
- GET/PUT/DELETE /v1/orgs/{orgId}/users/{userId}/token-policy (user override)

**Files:**
- `services/user-org-service/internal/httpapi/tokenpolicies/handlers.go`

**Acceptance Criteria:**
- [ ] All endpoints return correct status codes
- [ ] Proper authorization (org:admin for writes)
- [ ] Validation errors return 400

---

### T-S035-P01-007: Implement token usage HTTP handlers
**Bead:** aas-6xug1
**Priority:** P1 | **Complexity:** Low | **Depends on:** T-S035-P01-003

REST handlers for usage queries:
- GET /v1/usage/tokens (own usage)
- GET /v1/orgs/{orgId}/users/{userId}/usage/tokens (admin view)
- POST /internal/v1/usage/tokens (M2M: record usage)
- GET /internal/v1/users/{userId}/rate-limit (M2M: check limit)

**Files:**
- `services/user-org-service/internal/httpapi/tokenusage/handlers.go`

**Acceptance Criteria:**
- [ ] Users can only see own usage
- [ ] Admins can see any user in their org
- [ ] Internal endpoints require M2M auth

---

### T-S035-P01-008: Register token policy routes
**Bead:** aas-xisvc
**Priority:** P1 | **Complexity:** Low | **Depends on:** T-S035-P01-006, T-S035-P01-007

Mount new handlers in server.go:
- Import tokenpolicies and tokenusage packages
- Call RegisterRoutes for both
- Configure middleware (auth, scope checks)

**Files:**
- `services/user-org-service/internal/server/server.go`
- `services/user-org-service/cmd/main.go` (if needed)

**Acceptance Criteria:**
- [ ] All new routes accessible
- [ ] Health check still works
- [ ] No route conflicts

---

## Phase 2: API Router Integration

### T-S035-P02-009: Refactor BudgetClient to TokenLimiter
**Bead:** aas-zczxp
**Priority:** P1 | **Complexity:** Medium | **Depends on:** T-S035-P01-007

Rename and rewrite budget_client.go:
- Rename to token_limiter.go
- Change from monetary to token-based
- Call user-org-service internal endpoints
- Return RateLimitStatus with window details

**Files:**
- `services/api-router-service/internal/limiter/token_limiter.go` (was budget_client.go)
- `services/api-router-service/internal/limiter/types.go`

**Acceptance Criteria:**
- [ ] Calls user-org-service for limit check
- [ ] Returns correct window status
- [ ] Graceful degradation if unavailable

---

### T-S035-P02-010: Add rate limit cache
**Bead:** aas-f8e18
**Priority:** P1 | **Complexity:** Medium | **Depends on:** T-S035-P02-009

Implement in-memory cache for rate limits:
- Cache effective policy + current usage per user
- 60-second TTL
- Thread-safe (sync.RWMutex or sync.Map)
- Background refresh option

**Files:**
- `services/api-router-service/internal/limiter/cache.go`

**Acceptance Criteria:**
- [ ] Cache hits avoid user-org calls
- [ ] TTL expiration works correctly
- [ ] Thread-safe under concurrent access

---

### T-S035-P02-011: Update rate limit middleware
**Bead:** aas-nnqu3
**Priority:** P1 | **Complexity:** Medium | **Depends on:** T-S035-P02-009, T-S035-P02-010

Pre-request middleware:
- Get user ID from context
- Check cached rate limit
- Block with 429 if exceeded
- Allow if under limit or service unavailable

**Files:**
- `services/api-router-service/internal/handler/middleware.go`

**Acceptance Criteria:**
- [ ] Blocks when limit exceeded
- [ ] Allows when under limit
- [ ] Graceful degradation

---

### T-S035-P02-012: Add usage reporting
**Bead:** aas-61b3c
**Priority:** P1 | **Complexity:** Low | **Depends on:** T-S035-P02-009

Post-request usage reporting:
- Extract total_tokens from inference response
- Report to user-org-service asynchronously
- Handle reporting failures gracefully

**Files:**
- `services/api-router-service/internal/handler/inference.go`

**Acceptance Criteria:**
- [ ] Tokens recorded after each request
- [ ] Async (doesn't block response)
- [ ] Failures logged, not fatal

---

### T-S035-P02-013: Add OpenAI-compatible 429 response
**Bead:** aas-opjs5
**Priority:** P1 | **Complexity:** Low | **Depends on:** T-S035-P02-011

429 response formatting:
- OpenAI-compatible error body
- x-ratelimit-* headers
- Retry-After header
- Show which window triggered

**Files:**
- `services/api-router-service/internal/handler/errors.go`

**Acceptance Criteria:**
- [ ] Error body matches OpenAI format
- [ ] All required headers present
- [ ] Window details in message

---

## Phase 3: CLI Updates

### T-S035-P03-014: Add token-policy command to ai-aas-org
**Bead:** aas-stc2n
**Priority:** P2 | **Complexity:** Medium | **Depends on:** T-S035-P01-006

Full CRUD for token policies:
- token-policy create --name NAME [--1h] [--24h] [--7d]
- token-policy list
- token-policy show NAME
- token-policy update NAME [--1h] [--24h] [--7d]
- token-policy delete NAME
- token-policy set-default --policy NAME
- token-policy get-default

**Files:**
- `services/ai-aas-org/internal/cmd/token_policy.go`

**Acceptance Criteria:**
- [ ] All commands work
- [ ] Good help text
- [ ] Table/JSON output formats

---

### T-S035-P03-015: Add user token-policy subcommand
**Bead:** aas-1gxwe
**Priority:** P2 | **Complexity:** Low | **Depends on:** T-S035-P03-014

User-specific commands:
- user set-token-policy --user USER --policy POLICY
- user set-token-policy --user USER --policy inherit
- user usage --user USER

**Files:**
- `services/ai-aas-org/internal/cmd/user.go`

**Acceptance Criteria:**
- [ ] Override and inherit work
- [ ] Usage shows all windows

---

### T-S035-P03-016: Update userorg client library
**Bead:** aas-xrkl9
**Priority:** P2 | **Complexity:** Low | **Depends on:** T-S035-P01-006

Add types and methods to shared client:
- TokenRateLimitPolicy type
- TokenUsage type
- Policy CRUD methods
- Usage query methods

**Files:**
- `services/ai-aas-cli/internal/client/userorg/types.go`
- `services/ai-aas-cli/internal/client/userorg/client.go`

**Acceptance Criteria:**
- [ ] Types match API responses
- [ ] All endpoints callable

---

### T-S035-P03-017: Deprecate budget command
**Bead:** aas-h4k56
**Priority:** P2 | **Complexity:** Low | **Depends on:** T-S035-P03-014

Add deprecation alias:
- Keep `budget` as alias to `token-policy`
- Print deprecation warning
- Update help text

**Files:**
- `services/ai-aas-cli/internal/admin/budget.go`

**Acceptance Criteria:**
- [ ] Alias works
- [ ] Warning shown
- [ ] Points to new command

---

## Phase 4: E2E Tests & Cleanup

### T-S035-P04-018: Rewrite E2E test fixture
**Bead:** aas-lmq9c
**Priority:** P2 | **Complexity:** Medium | **Depends on:** T-S035-P01-006

New fixture for token rate-limits:
- TokenPolicyFixture (create, get, delete)
- TokenUsageFixture (query usage)
- Cleanup on test teardown

**Files:**
- `tests/e2e/fixtures/token_policies.go`

**Acceptance Criteria:**
- [ ] All CRUD operations
- [ ] Cleanup registered

---

### T-S035-P04-019: Rewrite E2E tests
**Bead:** aas-iggqx
**Priority:** P2 | **Complexity:** Medium | **Depends on:** T-S035-P04-018, T-S035-P02-013

New test suite:
- Policy CRUD
- Org default
- User override
- Usage query
- Rate limit enforcement (429)
- Overage behavior

**Files:**
- `tests/e2e/suites/token_rate_limit_test.go`

**Acceptance Criteria:**
- [ ] All use cases covered
- [ ] Tests pass in CI

---

### T-S035-P04-020: Remove deprecated budget schemas
**Bead:** aas-l4gti
**Priority:** P2 | **Complexity:** Low | **Depends on:** T-S035-P04-019

Cleanup spec 005:
- Remove BudgetStatus schema
- Remove BudgetOverride schema
- Remove /v1/budgets endpoints
- Update any references

**Files:**
- `specs/005-user-org-service/contracts/user-org-service.openapi.yaml`

**Acceptance Criteria:**
- [ ] No budget references remain
- [ ] OpenAPI validates

---

### T-S035-P04-021: Update documentation
**Bead:** aas-p316z
**Priority:** P2 | **Complexity:** Low | **Depends on:** T-S035-P04-019

User-facing docs:
- Token rate-limit user guide
- CLI command reference
- API reference updates

**Files:**
- `docs/guides/token-rate-limits.md`
- `docs/reference/cli.md`

**Acceptance Criteria:**
- [ ] Guide covers all features
- [ ] Examples work

---

## Dependency Graph

```
T-S035-P01-001 (migration)
    │
    ├─► T-S035-P01-002 (policy repo)
    │       │
    │       ├─► T-S035-P01-004 (policy tests)
    │       │
    │       └─► T-S035-P01-006 (policy handlers)
    │               │
    │               ├─► T-S035-P01-008 (register routes)
    │               │
    │               ├─► T-S035-P03-014 (CLI token-policy)
    │               │       │
    │               │       ├─► T-S035-P03-015 (CLI user token-policy)
    │               │       │
    │               │       └─► T-S035-P03-017 (deprecate budget)
    │               │
    │               ├─► T-S035-P03-016 (userorg client)
    │               │
    │               └─► T-S035-P04-018 (E2E fixture)
    │
    └─► T-S035-P01-003 (usage repo)
            │
            ├─► T-S035-P01-005 (usage tests)
            │
            └─► T-S035-P01-007 (usage handlers)
                    │
                    ├─► T-S035-P01-008 (register routes)
                    │
                    └─► T-S035-P02-009 (TokenLimiter)
                            │
                            ├─► T-S035-P02-010 (cache)
                            │       │
                            │       └─► T-S035-P02-011 (middleware)
                            │
                            ├─► T-S035-P02-012 (usage reporting)
                            │
                            └─► T-S035-P02-013 (429 response)
                                    │
                                    └─► T-S035-P04-019 (E2E tests)
                                            │
                                            ├─► T-S035-P04-020 (cleanup schemas)
                                            │
                                            └─► T-S035-P04-021 (docs)
```
