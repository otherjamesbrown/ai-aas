# Quickstart: User-Level Model Access Control

**Feature**: 022-user-model-access-control  
**Time to implement**: ~3-4 days

## Overview

This feature adds per-user model access control within organizations. By default, users have `restricted` access mode with no model grants. Org admins can:

1. Grant specific models to users
2. Grant all current models (snapshot)
3. Set `auto_grant` mode for automatic access to all current and future models

## Prerequisites

- [ ] PostgreSQL with existing `orgs` and `users` tables
- [ ] user-org-service running
- [ ] api-router-service running  
- [ ] Redis for auth context caching
- [ ] ai-aas-cli installed

## Implementation Checklist

### Phase 1: Database & Repository (Day 1)

```bash
# 1. Create migration file
cat > db/migrations/operational/20251130001_user_model_access.up.sql << 'EOF'
CREATE TABLE IF NOT EXISTS user_model_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    access_mode VARCHAR(20) NOT NULL DEFAULT 'restricted' 
        CHECK (access_mode IN ('restricted', 'auto_grant')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_model_access_org_user 
    ON user_model_access(org_id, user_id);

CREATE TABLE IF NOT EXISTS user_model_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    model_name VARCHAR(255) NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by UUID REFERENCES users(user_id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ,
    UNIQUE(org_id, user_id, model_name)
);

CREATE INDEX IF NOT EXISTS idx_user_model_grants_lookup 
    ON user_model_grants(org_id, user_id, model_name);
CREATE INDEX IF NOT EXISTS idx_user_model_grants_user 
    ON user_model_grants(user_id);
EOF

# 2. Run migration
make db-migrate
```

### Phase 2: User-Org Service API (Day 1-2)

Create `services/user-org-service/internal/httpapi/modelaccess/handlers.go`:

```go
// Key endpoints to implement:
// GET    /api/v1/orgs/{org_id}/users/{user_id}/model-access
// PUT    /api/v1/orgs/{org_id}/users/{user_id}/model-access/mode
// GET    /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants
// POST   /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants
// POST   /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants/all-current
// DELETE /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants/{model_name}
```

### Phase 3: API Router Integration (Day 2-3)

1. **Extend auth response** in user-org-service `/v1/auth/validate-api-key`:

```go
type ValidateAPIKeyResponse struct {
    Valid           bool     `json:"valid"`
    APIKeyID        string   `json:"apiKeyId"`
    OrganizationID  string   `json:"organizationId"`
    PrincipalID     string   `json:"principalId"`
    PrincipalType   string   `json:"principalType"`
    Scopes          []string `json:"scopes"`
    // NEW FIELDS
    ModelAccessMode string   `json:"modelAccessMode"` // "restricted" or "auto_grant"
    GrantedModels   []string `json:"grantedModels"`   // Only if restricted mode
}
```

2. **Add ModelAccessMiddleware** in api-router-service:

```go
// services/api-router-service/internal/api/public/model_access.go
func ModelAccessMiddleware(logger *zap.Logger, tracer trace.Tracer) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            authCtx := r.Context().Value(AuthContextKey).(*auth.AuthenticatedContext)
            
            // Extract model from request
            model := extractModelFromRequest(r)
            if model == "" {
                next.ServeHTTP(w, r)
                return
            }
            
            // Check access
            if authCtx.ModelAccessMode == "auto_grant" {
                next.ServeHTTP(w, r)
                return
            }
            
            // Restricted mode: check grants
            hasAccess := slices.Contains(authCtx.GrantedModels, model)
            if !hasAccess {
                writeAccessDeniedError(w, r, model)
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

3. **Update middleware chain** in `cmd/router/main.go`:

```go
appRouter.Use(public.BodyBufferMiddleware(64 * 1024))
appRouter.Use(public.AuthContextMiddleware(authenticator, logger, tracer))
appRouter.Use(public.ModelAccessMiddleware(logger, tracer))  // NEW
appRouter.Use(public.RateLimitMiddleware(rateLimiter, auditLogger, logger, tracer))
appRouter.Use(public.BudgetMiddleware(budgetClient, auditLogger, logger, tracer))
```

### Phase 4: CLI Commands (Day 3-4)

Create `services/ai-aas-cli/cmd/user/model_access.go`:

```go
// Commands to implement:
// ai-aas-cli user model-access show --org-id <org> --user-id <user>
// ai-aas-cli user model-access set-mode --org-id <org> --user-id <user> --mode <mode>
// ai-aas-cli user model-access grant --org-id <org> --user-id <user> --model <model>
// ai-aas-cli user model-access grant-all-current --org-id <org> --user-id <user>
// ai-aas-cli user model-access revoke --org-id <org> --user-id <user> --model <model>
// ai-aas-cli user model-access list --org-id <org> --user-id <user>
```

## Verification Steps

```bash
# 1. Create test user with restricted access (default)
ai-aas-cli user model-access show --org-id acme --user-id test-user
# Expected: access_mode=restricted, granted_models=[]

# 2. Grant specific model
ai-aas-cli user model-access grant --org-id acme --user-id test-user --model mistral-7b
# Expected: Grant created

# 3. Test inference - should work
curl -X POST https://api.ai-aas.io/v1/chat/completions \
  -H "X-API-Key: $TEST_API_KEY" \
  -d '{"model": "mistral-7b", "messages": [{"role": "user", "content": "Hello"}]}'
# Expected: 200 OK

# 4. Test inference with non-granted model - should fail
curl -X POST https://api.ai-aas.io/v1/chat/completions \
  -H "X-API-Key: $TEST_API_KEY" \
  -d '{"model": "llama-3-8b", "messages": [{"role": "user", "content": "Hello"}]}'
# Expected: 403 Forbidden - "User does not have access to model 'llama-3-8b'"

# 5. Set auto_grant mode
ai-aas-cli user model-access set-mode --org-id acme --user-id test-user --mode auto_grant

# 6. Test inference with any model - should work
curl -X POST https://api.ai-aas.io/v1/chat/completions \
  -H "X-API-Key: $TEST_API_KEY" \
  -d '{"model": "llama-3-8b", "messages": [{"role": "user", "content": "Hello"}]}'
# Expected: 200 OK
```

## Feature Flag

Add to environment configuration:

```yaml
# configs/environments/development.yaml
features:
  user_model_access_enabled: true  # Set to false to skip user-level checks
```

When disabled, skip the ModelAccessMiddleware check (preserves current org-level-only behavior).

## Common Issues

| Issue | Solution |
|-------|----------|
| 403 after granting | Cache TTL is 30s; wait or restart api-router-service |
| User not found | Ensure user exists in org; use UUID or email |
| Grant conflict | User already has grant for model; ignore or revoke first |
| Migration fails | Check foreign key references; orgs/users tables must exist |

## Next Steps

After basic implementation:

1. Add audit logging for grant changes
2. Add metrics for access check hit/miss
3. Add expiration job for time-limited grants
4. Add migration tooling for existing orgs

