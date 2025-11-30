# User Model Access Control Runbook

**Service**: User Org Service, API Router Service
**Last Updated**: 2025-11-30
**Owner**: Platform Engineering Team

## Overview

This runbook provides procedures for managing user-level model access control, including troubleshooting access issues, managing grants, and performing migrations.

## Quick Reference

### Key Endpoints (Admin API)

- **Get User Model Access**: `GET /v1/orgs/{org_id}/users/{user_id}/model-access`
- **Set Access Mode**: `PUT /v1/orgs/{org_id}/users/{user_id}/model-access/mode`
- **List Grants**: `GET /v1/orgs/{org_id}/users/{user_id}/model-access/grants`
- **Create Grant**: `POST /v1/orgs/{org_id}/users/{user_id}/model-access/grants`
- **Delete Grant**: `DELETE /v1/orgs/{org_id}/users/{user_id}/model-access/grants/{model_name}`
- **Bulk Grant**: `POST /v1/orgs/{org_id}/users/{user_id}/model-access/grants/all-current`

### CLI Commands

```bash
# Show user's model access
ai-aas-cli user model-access show --org-id <org> --user-id <user>
ai-aas-cli user model-access show --org-id <org> --email <email>

# Set access mode
ai-aas-cli user model-access set-mode --org-id <org> --user-id <user> --mode restricted
ai-aas-cli user model-access set-mode --org-id <org> --user-id <user> --mode auto_grant

# Grant specific model
ai-aas-cli user model-access grant --org-id <org> --user-id <user> --model gpt-4
ai-aas-cli user model-access grant --org-id <org> --user-id <user> --model gpt-4 --expires-in 7d

# Revoke model access
ai-aas-cli user model-access revoke --org-id <org> --user-id <user> --model gpt-4

# List all grants
ai-aas-cli user model-access list --org-id <org> --user-id <user>

# Grant all current models
ai-aas-cli user model-access grant-all --org-id <org> --user-id <user>

# Migrate existing users
ai-aas-cli user model-access migrate --org-id <org> --mode restricted --models gpt-4,gpt-3.5-turbo
ai-aas-cli user model-access migrate --org-id <org> --mode restricted --models gpt-4 --dry-run
```

### Access Modes

| Mode | Description | Behavior |
|------|-------------|----------|
| `restricted` | Default mode | User can only access explicitly granted models |
| `auto_grant` | Legacy/admin mode | User can access any model available to the org |

### Feature Flag

The feature is controlled by `user_model_access_enabled`:
- **Development**: Enabled by default
- **Production**: Disabled by default (gradual rollout)

When disabled, the middleware skips access checks and all users have unrestricted access (legacy behavior).

## Common Tasks

### 1. Check Why a User Cannot Access a Model

**Symptom**: User receives 403 "Access denied: you do not have access to model X"

#### Investigation Steps

1. **Check the user's access mode**:
   ```bash
   ai-aas-cli user model-access show --org-id <org> --user-id <user>
   ```

2. **If mode is `restricted`, check grants**:
   ```bash
   ai-aas-cli user model-access list --org-id <org> --user-id <user>
   ```

3. **Verify the model name matches exactly**:
   - Model names are case-sensitive
   - Check for typos or aliases

4. **Check if grant has expired**:
   - Review `expiresAt` field in grant list
   - Expired grants do not provide access

5. **Check feature flag status**:
   ```bash
   kubectl get configmap api-router-config -n platform -o yaml | grep user_model_access
   ```

6. **Check auth cache (access changes may take up to 30s)**:
   - Wait 30 seconds and retry
   - Cache TTL is 30 seconds by default

#### Resolution

**To grant access to a specific model**:
```bash
ai-aas-cli user model-access grant --org-id <org> --user-id <user> --model <model-name>
```

**To grant access to all current models**:
```bash
ai-aas-cli user model-access grant-all --org-id <org> --user-id <user>
```

**To switch user to auto_grant mode**:
```bash
ai-aas-cli user model-access set-mode --org-id <org> --user-id <user> --mode auto_grant
```

### 2. Grant a User Access to a New Model

```bash
# Grant single model
ai-aas-cli user model-access grant --org-id <org> --user-id <user> --model <model-name>

# Grant with expiration (useful for trials)
ai-aas-cli user model-access grant --org-id <org> --user-id <user> --model <model-name> --expires-in 30d
```

### 3. Revoke Model Access

```bash
ai-aas-cli user model-access revoke --org-id <org> --user-id <user> --model <model-name>
```

**Note**: Revocation takes effect within 30 seconds due to auth caching.

### 4. Migrate Existing Users to Restricted Mode

When enabling model access control for an existing org, use the migration command to grant all current models to existing users:

```bash
# Preview migration (dry run)
ai-aas-cli user model-access migrate --org-id <org> --mode restricted --models gpt-4,gpt-3.5-turbo,llama-2 --dry-run

# Execute migration
ai-aas-cli user model-access migrate --org-id <org> --mode restricted --models gpt-4,gpt-3.5-turbo,llama-2
```

The migration command:
1. Iterates through all active users in the org
2. Sets each user's access mode to `restricted`
3. Grants all specified models to each user
4. Reports progress and any errors

### 5. Emergency: Disable Model Access Control

If model access control is causing widespread issues:

1. **Disable feature flag** (immediate effect):
   ```yaml
   # configs/environments/production.yaml
   features:
     user_model_access_enabled: false
   ```

2. **Commit and push**:
   ```bash
   git add configs/environments/production.yaml
   git commit -m "Disable user model access control"
   git push origin main
   ```

3. **Sync ArgoCD** (or wait for auto-sync):
   ```bash
   argocd app sync api-router-service
   ```

All users will immediately have unrestricted access to all org models.

## Troubleshooting

### Access Denied Despite Grant Existing

**Possible Causes**:

1. **Grant expired**: Check `expiresAt` field
2. **Cache delay**: Wait 30 seconds for cache to refresh
3. **Wrong model name**: Model names are case-sensitive
4. **Feature flag off**: When disabled, grants are ignored (all access allowed)
5. **Service principal**: Service accounts default to `auto_grant`

**Debug Query**:
```sql
-- Check user's access mode
SELECT access_mode, version, updated_at
FROM user_model_access
WHERE org_id = '<org-id>' AND user_id = '<user-id>';

-- Check user's grants
SELECT model_name, granted_at, expires_at
FROM user_model_grants
WHERE org_id = '<org-id>'
  AND user_id = '<user-id>'
  AND (expires_at IS NULL OR expires_at > NOW());
```

### User Has Access When They Shouldn't

**Possible Causes**:

1. **Mode is auto_grant**: Check user's access mode
2. **Feature flag disabled**: Check if middleware is enabled
3. **Service account**: Service principals bypass restrictions

**Investigation**:
```bash
ai-aas-cli user model-access show --org-id <org> --user-id <user>
```

If mode is `auto_grant`, change to `restricted`:
```bash
ai-aas-cli user model-access set-mode --org-id <org> --user-id <user> --mode restricted
```

### Migration Failed Midway

If the migration command fails partway through:

1. **Check which users were processed**: Review command output
2. **Run migration again**: The migration is idempotent - already-granted models are skipped
3. **Check for specific user errors**: Some users may have issues (e.g., deleted, suspended)

### Performance Issues

**Symptoms**: Slow inference requests, high latency on CheckModelAccess

**Investigation**:

1. **Check index usage**:
   ```sql
   EXPLAIN ANALYZE
   SELECT EXISTS (
     SELECT 1 FROM user_model_grants
     WHERE org_id = '<org-id>' AND user_id = '<user-id>' AND model_name = '<model>'
       AND (expires_at IS NULL OR expires_at > NOW())
   );
   ```

2. **Check cache hit rate**: Review metrics for auth cache hits/misses

3. **Check table size**:
   ```sql
   SELECT reltuples AS estimate FROM pg_class WHERE relname = 'user_model_grants';
   ```

**Resolution**:

- Ensure indexes exist (created by migration)
- Increase auth cache TTL if hit rate is low
- Consider read replicas for high-traffic deployments

## Database Schema

### Tables

**user_model_access**:
- `id` (UUID): Primary key
- `org_id` (UUID): Organization ID (FK to orgs)
- `user_id` (UUID): User ID (FK to users)
- `access_mode` (VARCHAR): 'restricted' or 'auto_grant'
- `version` (BIGINT): Optimistic locking version
- `created_at`, `updated_at` (TIMESTAMPTZ)

**user_model_grants**:
- `id` (UUID): Primary key
- `org_id` (UUID): Organization ID
- `user_id` (UUID): User ID
- `model_name` (VARCHAR): Model name (e.g., 'gpt-4')
- `granted_at` (TIMESTAMPTZ): When grant was created
- `granted_by` (UUID): Who created the grant (nullable)
- `expires_at` (TIMESTAMPTZ): Expiration time (nullable)

### Indexes

- `idx_user_model_access_org_user`: (org_id, user_id) - Unique
- `idx_user_model_grants_org_user_model`: (org_id, user_id, model_name) - Unique
- `idx_user_model_grants_user_model`: (user_id, model_name) - Hot path query

## Monitoring

### Key Metrics

| Metric | Description |
|--------|-------------|
| `model_access_denials_total` | Counter of access denial events |
| `auth_validate_duration_seconds` | Latency of auth validation (includes model access check) |

### Alerts to Consider

1. **High denial rate**: Many users being denied access (possible misconfiguration)
2. **Auth latency spike**: Model access check adding latency
3. **Migration failures**: Batch grant operations failing

## Related Documentation

- [User Model Access Control Spec](../../specs/022-user-model-access-control/spec.md)
- [Data Model](../../specs/022-user-model-access-control/data-model.md)
- [API Contracts](../../specs/022-user-model-access-control/contracts/openapi.yaml)
- [Environment Access](../platform/environment-access.md)
