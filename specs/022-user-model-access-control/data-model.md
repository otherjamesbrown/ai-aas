# Data Model: User-Level Model Access Control

**Feature**: 022-user-model-access-control  
**Date**: 2025-11-30

## Entity Overview

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                          Entity Relationship Diagram                          │
├──────────────────────────────────────────────────────────────────────────────┤
│                                                                               │
│  ┌─────────────┐       ┌────────────────────┐       ┌──────────────────┐     │
│  │    orgs     │──────▶│ user_model_access  │◀──────│      users       │     │
│  │  (existing) │  1:N  │   (org_id, user_id)│  N:1  │    (existing)    │     │
│  └─────────────┘       └────────────────────┘       └──────────────────┘     │
│         │                       │                            │               │
│         │                       │ 1:N                        │               │
│         │                       ▼                            │               │
│         │              ┌────────────────────┐                │               │
│         └─────────────▶│ user_model_grants  │◀───────────────┘               │
│                   1:N  │ (org, user, model) │  N:1                           │
│                        └────────────────────┘                                │
│                                                                               │
└──────────────────────────────────────────────────────────────────────────────┘
```

---

## New Tables

### Table: `user_model_access`

Controls the access mode for a user within an organization.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | UUID | PK, DEFAULT gen_random_uuid() | Unique identifier |
| `org_id` | UUID | NOT NULL, FK → orgs(org_id), ON DELETE CASCADE | Organization |
| `user_id` | UUID | NOT NULL, FK → users(user_id), ON DELETE CASCADE | User |
| `access_mode` | VARCHAR(20) | NOT NULL, DEFAULT 'restricted', CHECK IN ('restricted', 'auto_grant') | Access mode |
| `created_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Creation timestamp |
| `updated_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Constraints**:
- UNIQUE(org_id, user_id) — One access mode per user per org

**Indexes**:
- `idx_user_model_access_org_user` ON (org_id, user_id) — Primary lookup

---

### Table: `user_model_grants`

Records explicit model grants for users in `restricted` mode.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | UUID | PK, DEFAULT gen_random_uuid() | Unique identifier |
| `org_id` | UUID | NOT NULL, FK → orgs(org_id), ON DELETE CASCADE | Organization |
| `user_id` | UUID | NOT NULL, FK → users(user_id), ON DELETE CASCADE | User |
| `model_name` | VARCHAR(255) | NOT NULL | Model name (references model registry) |
| `granted_at` | TIMESTAMPTZ | NOT NULL, DEFAULT NOW() | When grant was created |
| `granted_by` | UUID | FK → users(user_id), ON DELETE SET NULL | Admin who granted |
| `expires_at` | TIMESTAMPTZ | NULL | Optional expiration timestamp |

**Constraints**:
- UNIQUE(org_id, user_id, model_name) — One grant per user per model per org

**Indexes**:
- `idx_user_model_grants_lookup` ON (org_id, user_id, model_name) — Access check query
- `idx_user_model_grants_user` ON (user_id) — List grants for user
- `idx_user_model_grants_expiry` ON (expires_at) WHERE expires_at IS NOT NULL — Cleanup expired grants

---

## Access Modes

### `restricted` (Default)

- User can **only** access models with explicit grants in `user_model_grants`
- New models added to org do NOT automatically become available
- Most secure option; requires explicit grants for each model

### `auto_grant`

- User automatically gets access to all models (current and future)
- When org gets access to a new model, user immediately can use it
- Useful for admins, power users, or orgs with simple access needs
- No entries in `user_model_grants` needed (mode check is sufficient)

---

## State Transitions

```
                              ┌───────────────────────┐
                              │    User Created in    │
                              │    Organization       │
                              └───────────┬───────────┘
                                          │
                                          ▼
                              ┌───────────────────────┐
                              │   No Access Record    │
                              │  (Implicit restricted │
                              │   with no grants)     │
                              └───────────┬───────────┘
                                          │
                    ┌─────────────────────┼─────────────────────┐
                    │                     │                     │
                    ▼                     ▼                     ▼
        ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
        │  Admin creates  │   │  Admin grants   │   │  Admin sets     │
        │  access record  │   │  specific model │   │  auto_grant     │
        │  (explicit      │   │  (creates grant │   │  mode           │
        │  restricted)    │   │  + access rec)  │   │                 │
        └────────┬────────┘   └────────┬────────┘   └────────┬────────┘
                 │                     │                     │
                 ▼                     ▼                     ▼
        ┌─────────────────┐   ┌─────────────────┐   ┌─────────────────┐
        │    restricted   │   │    restricted   │   │   auto_grant    │
        │   (no grants)   │◀──│  (with grants)  │   │  (all models)   │
        │   ACCESS: NONE  │   │  ACCESS: LISTED │   │  ACCESS: ALL    │
        └────────┬────────┘   └────────┬────────┘   └────────┬────────┘
                 │                     │                     │
                 │         ┌───────────┴───────────┐         │
                 │         │                       │         │
                 │         ▼                       ▼         │
                 │  ┌─────────────┐       ┌─────────────┐    │
                 │  │ Add grant   │       │ Revoke grant│    │
                 │  │ for model   │       │ for model   │    │
                 │  └─────────────┘       └─────────────┘    │
                 │                                           │
                 │           ┌─────────────────────┐         │
                 └──────────▶│  Mode change        │◀────────┘
                             │  (restricted ↔      │
                             │   auto_grant)       │
                             └─────────────────────┘
```

---

## Migration SQL

```sql
-- Migration: 20251130001_user_model_access.up.sql

-- User model access mode (per user per org)
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

-- Trigger to update updated_at
CREATE TRIGGER set_updated_at_user_model_access
    BEFORE UPDATE ON user_model_access
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- User model grants (explicit grants for restricted mode)
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
CREATE INDEX IF NOT EXISTS idx_user_model_grants_expiry 
    ON user_model_grants(expires_at) WHERE expires_at IS NOT NULL;

-- Down migration: 20251130001_user_model_access.down.sql
-- DROP TRIGGER IF EXISTS set_updated_at_user_model_access ON user_model_access;
-- DROP TABLE IF EXISTS user_model_grants;
-- DROP TABLE IF EXISTS user_model_access;
```

---

## Validation Rules

### user_model_access

| Field | Validation |
|-------|------------|
| org_id | Must exist in orgs table |
| user_id | Must exist in users table; user must belong to org |
| access_mode | Must be 'restricted' or 'auto_grant' |

### user_model_grants

| Field | Validation |
|-------|------------|
| org_id | Must exist in orgs table |
| user_id | Must exist in users table; user must belong to org |
| model_name | Non-empty string; should match model in org's routing policy (soft validation) |
| granted_by | Must be admin of the org (validated at API layer, not DB) |
| expires_at | Must be in future if provided |

---

## Query Patterns

### Check User Model Access (Hot Path)

```sql
-- Called on every inference request (results cached in Redis)
WITH user_access AS (
    SELECT access_mode 
    FROM user_model_access 
    WHERE org_id = $1 AND user_id = $2
)
SELECT 
    COALESCE((SELECT access_mode FROM user_access), 'restricted') as access_mode,
    CASE 
        WHEN (SELECT access_mode FROM user_access) = 'auto_grant' THEN true
        ELSE EXISTS (
            SELECT 1 FROM user_model_grants 
            WHERE org_id = $1 AND user_id = $2 AND model_name = $3
            AND (expires_at IS NULL OR expires_at > NOW())
        )
    END as has_access;
```

### List User's Granted Models

```sql
SELECT model_name, granted_at, granted_by, expires_at
FROM user_model_grants
WHERE org_id = $1 AND user_id = $2
AND (expires_at IS NULL OR expires_at > NOW())
ORDER BY granted_at DESC;
```

### Grant All Current Models (Snapshot)

```sql
-- Get all models from org's routing policies and grant each
INSERT INTO user_model_grants (org_id, user_id, model_name, granted_by)
SELECT DISTINCT $1, $2, model, $3
FROM routing_policies 
WHERE org_id = $1 OR org_id = '*'
ON CONFLICT (org_id, user_id, model_name) DO NOTHING;
```

