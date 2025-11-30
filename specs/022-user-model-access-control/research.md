# Research: User-Level Model Access Control

**Feature**: 022-user-model-access-control  
**Date**: 2025-11-30

## Open Questions Resolution

### Q1: Should there be a "deny list" in addition to grants?

**Decision**: No, not in initial implementation.

**Rationale**: 
- Grant-based (allowlist) approach is simpler and sufficient for stated use cases
- Deny lists add complexity: what happens if user has both grant AND deny for same model?
- If needed later, can add `deny_models` field to `user_model_access` table

**Alternatives Considered**:
- Deny list with grant precedence: Too complex, confusing for admins
- Deny list with deny precedence: Creates management burden for most use cases

---

### Q2: Should grants be copyable between users?

**Decision**: Yes, via CLI command only (not API endpoint).

**Rationale**:
- Common admin workflow: "Give new user same access as existing user"
- CLI can read grants from user A and create them for user B (simple orchestration)
- No need for dedicated API endpoint—CLI can call existing grant endpoints in loop

**Implementation**: 
```bash
ai-aas-cli user model-access copy --org-id acme --from-user u_123 --to-user u_456
```

**Alternatives Considered**:
- Dedicated copy API endpoint: Over-engineering for a convenience feature
- No copy capability: Forces manual re-entry, error-prone

---

### Q3: Should there be model groups/bundles for easier management?

**Decision**: No, defer to future RBAC roles implementation.

**Rationale**:
- Model groups add a new abstraction layer (more tables, more complexity)
- Per-model grants are explicit and auditable
- Future RBAC roles (spec TBD) will provide grouping via role-based permissions
- "Grant all current" command provides bulk grant without new abstraction

**Alternatives Considered**:
- Model groups table with many-to-many: Adds 2 tables, complicates access check logic
- Tags on models for filtering: Doesn't actually simplify grant management

---

### Q4: How should this integrate with future RBAC/roles system?

**Decision**: Design for forward compatibility; implement as parallel system.

**Rationale**:
- User model access is orthogonal to RBAC roles (roles = what you can do; model access = what you can access)
- When RBAC ships, can add `ModelAccessManager` role that permits managing grants
- Can add model access to role definitions later without migration

**Integration Points**:
1. Org admin check uses existing org membership + admin flag
2. Future: Add `model_access:manage` scope to role permissions
3. Future: Roles could auto-grant model bundles (role → model grants)

---

## Technical Research

### Access Check Caching Strategy

**Decision**: Include model access info in auth response; cache in API Router.

**Rationale**:
- Avoid extra round-trip to user-org-service on every request
- 30-second cache TTL balances performance vs. revocation latency
- Cache key: `user_model_access:{org_id}:{user_id}`

**Implementation**:
1. User-org-service `/v1/auth/validate-api-key` response extended with:
   ```json
   {
     "modelAccessMode": "restricted",
     "grantedModels": ["mistral-7b", "llama-3-8b"]
   }
   ```
2. API Router caches full auth context (including model access) for 30s
3. Grant changes trigger cache invalidation via Redis pub/sub or TTL expiry

**Performance Analysis**:
- Redis GET: ~0.5ms
- Cache miss + user-org-service call: ~10-20ms
- Acceptable for 30s cache with rare grant changes

---

### Default Behavior for New Users

**Decision**: New users default to `restricted` mode with no grants.

**Rationale**:
- Secure by default: new user cannot access any models until admin explicitly grants
- Org admin must either:
  a. Grant specific models, OR
  b. Set `auto_grant` mode, OR
  c. Run migration to grant all current models

**Migration for Existing Orgs**:
- Feature flag `USER_MODEL_ACCESS_ENABLED=false` initially
- When enabled:
  - Existing users with no `user_model_access` record → `restricted` mode
  - Org admin runs `ai-aas-cli user model-access migrate-existing --org-id acme --mode auto_grant` to preserve current behavior

---

### Cache Invalidation Strategy

**Decision**: TTL-based expiry (30 seconds), no active invalidation.

**Rationale**:
- Simplest approach: grants take up to 30s to take effect / be revoked
- Active invalidation (Redis pub/sub) adds complexity for minimal gain
- Revocations are rare; 30s delay is acceptable

**Alternatives Considered**:
- Redis pub/sub invalidation: Adds messaging complexity
- Shorter TTL (5s): More cache misses, higher latency
- No cache: Too slow for inference path

---

### API Router Integration Point

**Decision**: Add `ModelAccessMiddleware` after `AuthContextMiddleware`, before routing.

**Rationale**:
- Auth context already has `org_id`, `user_id`—model access check needs these
- Must happen before request reaches backend to return proper 403
- Middleware pattern consistent with existing rate limit / budget checks

**Middleware Order**:
1. BodyBufferMiddleware
2. AuthContextMiddleware (sets auth context)
3. **ModelAccessMiddleware (NEW)** - checks user model access
4. RateLimitMiddleware
5. BudgetMiddleware

---

## Dependencies Confirmed

| Dependency | Version | Notes |
|------------|---------|-------|
| user-org-service | Existing | Extend with model access endpoints |
| api-router-service | Existing | Add ModelAccessMiddleware |
| ai-aas-cli | Existing | Add `user model-access` commands |
| PostgreSQL | 15+ | Existing, add 2 new tables |
| Redis | 7+ | Existing, used for auth caching |

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Cache staleness causes access after revocation | Low | Medium | 30s max staleness is acceptable; document behavior |
| Performance regression on inference path | Low | High | Cache ensures <1ms overhead in hot path |
| Complex admin UX for grant management | Medium | Medium | CLI has `grant-all-current` and `copy` commands |
| Breaking change for existing users | Medium | High | Feature flag; migration tooling; opt-in rollout |

