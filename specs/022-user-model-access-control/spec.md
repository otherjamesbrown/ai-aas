# Spec 022: User-Level Model Access Control

## Status: Draft

## Overview

This specification defines user-level model access control within organizations. Currently, model access is controlled at the organization level via routing policies. This enhancement adds fine-grained per-user model permissions managed by org admins.

## Problem Statement

**Current State:**
- Routing policies control which models an organization can access
- All users within an org have identical model access
- No mechanism for org admins to restrict specific users to specific models

**Use Cases:**
1. As an org admin, I want to restrict certain users to only access specific models
2. As an org admin, I want to enable a user to have access to all currently available models
3. As an org admin, I want to enable a user to have access to all current AND future models (auto-grant)

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Granularity | Per-model grants | Maximum flexibility for org admins |
| Enforcement | API Router | Secure, validates on every request |
| "All current" handling | Snapshot at grant time | Explicit, auditable, no ambiguity |

---

## Data Model

### New Table: `user_model_access`

```sql
CREATE TABLE user_model_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    -- Access mode determines how permissions work for this user
    -- 'restricted': User can ONLY access explicitly granted models
    -- 'auto_grant': User automatically gets access to all new models
    access_mode VARCHAR(20) NOT NULL DEFAULT 'restricted',

    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),

    UNIQUE(org_id, user_id)
);

CREATE INDEX idx_user_model_access_org_user ON user_model_access(org_id, user_id);
```

### New Table: `user_model_grants`

```sql
CREATE TABLE user_model_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    model_name VARCHAR(255) NOT NULL,  -- References model in registry

    -- Grant metadata
    granted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    granted_by UUID REFERENCES users(id),  -- Admin who granted access

    -- Optional: expiration
    expires_at TIMESTAMP WITH TIME ZONE,

    UNIQUE(org_id, user_id, model_name)
);

CREATE INDEX idx_user_model_grants_lookup ON user_model_grants(org_id, user_id, model_name);
CREATE INDEX idx_user_model_grants_user ON user_model_grants(user_id);
```

---

## Access Modes

### Mode: `restricted` (Default)
- User can **only** access models explicitly granted via `user_model_grants`
- New models added to org do NOT automatically become available
- Most secure, requires explicit grants

### Mode: `auto_grant`
- User automatically gets access to all models (current and future)
- When org gets access to a new model, user immediately can use it
- Useful for admins, power users, or orgs with simple access needs

---

## Access Check Logic

```
┌─────────────────────────────────────────────────────────────────┐
│                    API Request with Model                        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 1. Authenticate API Key → Get org_id, user_id                   │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 2. Check Org Routing Policy                                      │
│    - Does org have access to this model?                        │
│    - If NO → Return 403 "Model not available for organization"  │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ 3. Get User Access Mode                                          │
│    - Query user_model_access for (org_id, user_id)              │
│    - If no record exists → Create with mode='restricted'        │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
            ┌─────────────────┴─────────────────┐
            │                                   │
            ▼                                   ▼
┌───────────────────────┐           ┌───────────────────────┐
│ Mode = 'auto_grant'   │           │ Mode = 'restricted'   │
│ → ALLOW               │           │ → Check explicit grant│
└───────────────────────┘           └───────────────────────┘
                                              │
                                              ▼
                              ┌─────────────────────────────────┐
                              │ 4. Query user_model_grants      │
                              │    WHERE org_id, user_id, model │
                              │    AND (expires_at IS NULL      │
                              │         OR expires_at > NOW())  │
                              └─────────────────────────────────┘
                                              │
                              ┌───────────────┴───────────────┐
                              │                               │
                              ▼                               ▼
                    ┌─────────────────┐             ┌─────────────────┐
                    │ Grant EXISTS    │             │ Grant NOT FOUND │
                    │ → ALLOW         │             │ → DENY (403)    │
                    └─────────────────┘             └─────────────────┘
```

---

## API Endpoints

### User-Org Service Extensions

#### Get User Model Access
```
GET /api/v1/orgs/{org_id}/users/{user_id}/model-access
Authorization: Bearer <admin-api-key>

Response:
{
  "user_id": "uuid",
  "org_id": "uuid",
  "access_mode": "restricted",
  "granted_models": [
    {
      "model_name": "mistral-7b",
      "granted_at": "2024-11-30T10:00:00Z",
      "granted_by": "admin-user-id",
      "expires_at": null
    }
  ],
  "available_models": [
    "mistral-7b",
    "llama-3-8b",
    "gpt-4-turbo"
  ]
}
```

#### Set User Access Mode
```
PUT /api/v1/orgs/{org_id}/users/{user_id}/model-access/mode
Authorization: Bearer <admin-api-key>
Content-Type: application/json

{
  "access_mode": "auto_grant"
}
```

#### Grant Model Access
```
POST /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants
Authorization: Bearer <admin-api-key>
Content-Type: application/json

{
  "model_name": "mistral-7b",
  "expires_at": null  // Optional: ISO timestamp
}

Response:
{
  "grant_id": "uuid",
  "model_name": "mistral-7b",
  "granted_at": "2024-11-30T10:00:00Z",
  "granted_by": "admin-user-id"
}
```

#### Grant All Current Models (Snapshot)
```
POST /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants/all-current
Authorization: Bearer <admin-api-key>

Response:
{
  "granted_count": 5,
  "models": ["mistral-7b", "llama-3-8b", "gpt-4-turbo", "claude-3", "gemini-pro"],
  "granted_at": "2024-11-30T10:00:00Z"
}
```

#### Revoke Model Access
```
DELETE /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants/{model_name}
Authorization: Bearer <admin-api-key>
```

#### List User's Granted Models
```
GET /api/v1/orgs/{org_id}/users/{user_id}/model-access/grants
Authorization: Bearer <admin-api-key>

Response:
{
  "grants": [
    {
      "model_name": "mistral-7b",
      "granted_at": "2024-11-30T10:00:00Z",
      "granted_by": "admin-user-id",
      "expires_at": null
    }
  ]
}
```

---

## CLI Commands

### New Commands under `ai-aas-cli user`

```bash
# View user's model access
ai-aas-cli user model-access show --org-id acme --user-id u_123
ai-aas-cli user model-access show --org-id acme --email user@example.com

# Set access mode
ai-aas-cli user model-access set-mode --org-id acme --user-id u_123 --mode restricted
ai-aas-cli user model-access set-mode --org-id acme --user-id u_123 --mode auto_grant

# Grant specific model
ai-aas-cli user model-access grant --org-id acme --user-id u_123 --model mistral-7b
ai-aas-cli user model-access grant --org-id acme --user-id u_123 --model llama-3-8b --expires-in 30d

# Grant all current models (snapshot)
ai-aas-cli user model-access grant-all-current --org-id acme --user-id u_123

# Revoke model access
ai-aas-cli user model-access revoke --org-id acme --user-id u_123 --model mistral-7b

# List grants
ai-aas-cli user model-access list --org-id acme --user-id u_123
```

### Example Workflow

```bash
# Use case 1: Restrict user to specific models
ai-aas-cli user model-access set-mode --org-id acme --email intern@acme.com --mode restricted
ai-aas-cli user model-access grant --org-id acme --email intern@acme.com --model mistral-7b

# Use case 2: Give user access to all current models
ai-aas-cli user model-access set-mode --org-id acme --email developer@acme.com --mode restricted
ai-aas-cli user model-access grant-all-current --org-id acme --email developer@acme.com

# Use case 3: Give user access to all current and future models
ai-aas-cli user model-access set-mode --org-id acme --email admin@acme.com --mode auto_grant
```

---

## API Router Integration

### Auth Context Extension

Extend the auth context returned from user-org-service to include model access info:

```go
type AuthContext struct {
    APIKeyID       string
    OrganizationID string
    UserID         string
    Scopes         []string

    // New fields for model access
    ModelAccessMode string   // "restricted" or "auto_grant"
    GrantedModels   []string // Only populated if mode="restricted"
}
```

### Request Flow Update

1. API Router authenticates request (existing)
2. Auth response now includes `ModelAccessMode` and `GrantedModels`
3. Before routing to backend, check user model access:
   - If `ModelAccessMode == "auto_grant"` → proceed
   - If `ModelAccessMode == "restricted"` → check if model in `GrantedModels`
4. Return 403 if access denied

### Caching Strategy

To avoid hitting the database on every request:

1. **Cache user model access in API Router** (30-second TTL)
2. **Include in auth response** - user-org-service returns grants with auth
3. **Invalidate on change** - When grants change, invalidate user's cache entry

---

## New Model Registration Hook

When a new model is registered in the platform:

1. Model added to `model_registry`
2. Routing policy created (org can access model)
3. **New:** For all users with `access_mode = 'auto_grant'`:
   - No action needed (they auto-have access)
4. For users with `access_mode = 'restricted'`:
   - No automatic grants (org admin must explicitly grant)

---

## Migration Path

### For Existing Users

1. All existing users default to `access_mode = 'restricted'`
2. Option A: Org admin manually sets up grants
3. Option B: Run migration to grant all existing users all current models:
   ```bash
   ai-aas-cli user model-access migrate-existing --org-id acme --grant-all-current
   ```

### Backward Compatibility

During rollout, add feature flag:
```
USER_MODEL_ACCESS_ENABLED=true|false
```

When `false`, skip user-level access check (current behavior).

---

## Implementation Phases

### Phase 1: Database & API
1. Create database migrations
2. Add user-org-service API endpoints
3. Add basic CLI commands

### Phase 2: Router Integration
1. Extend auth context with model access
2. Add access check to API Router
3. Add caching layer

### Phase 3: CLI & Admin Experience
1. Complete CLI commands with good UX
2. Add to Web Portal (if applicable)
3. Add audit logging for access changes

### Phase 4: Polish
1. Add feature flag for gradual rollout
2. Migration tooling for existing orgs
3. Documentation and runbooks

---

## Security Considerations

1. **Only org admins can manage grants** - Validate admin permission on all endpoints
2. **Audit all access changes** - Log who granted/revoked what
3. **Grant expiration** - Support time-limited access for contractors, trials
4. **Cache invalidation** - Ensure revocations take effect quickly (< 30 seconds)

---

## Open Questions

1. Should there be a "deny list" in addition to grants? (Block specific models for a user)
2. Should grants be copyable between users? (Clone user A's access to user B)
3. Should there be model groups/bundles for easier management?
4. How should this integrate with future RBAC/roles system?

---

## Files to Modify

| Component | Files |
|-----------|-------|
| Database | `db/migrations/xxx_user_model_access.sql` |
| User-Org Service | `services/user-org-service/internal/api/model_access.go` |
| User-Org Service | `services/user-org-service/internal/repository/model_access.go` |
| API Router | `services/api-router-service/internal/auth/authenticator.go` |
| API Router | `services/api-router-service/internal/middleware/model_access.go` |
| CLI | `services/ai-aas-cli/cmd/user/model_access.go` |
| CLI | `services/ai-aas-cli/internal/client/userorg/model_access.go` |
