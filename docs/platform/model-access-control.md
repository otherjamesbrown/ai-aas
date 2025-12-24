# Model Access Control System

## Overview

The AI-AAS platform implements a comprehensive model access control system that governs which users can access which AI models within an organization. This system provides fine-grained control over model access through two complementary mechanisms:

1. **Routing Policies**: Define which models are available at the infrastructure level
2. **User Access Control**: Define which users within an organization can access specific models

This document explains how both systems work together to provide secure, flexible model access management.

## Architecture

### Components

The model access control system consists of three main components:

1. **Admin API Service** (`/home/dev/ai-aas/services/admin-api-service`)
   - Manages the model registry and routing policies
   - Auto-creates routing policies when models are registered
   - Provides REST APIs for model and policy management

2. **User-Org Service** (`/home/dev/ai-aas/services/user-org-service`)
   - Manages user access modes and model grants
   - Enforces user-level access control
   - Auto-assigns `auto_grant` mode to admin users

3. **AI-AAS CLI** (`/home/dev/ai-aas/services/ai-aas-cli`)
   - Provides command-line interface for managing both systems
   - Supports automated workflows and scripting

### Access Control Flow

```
1. Model Deployment → Model Registration → Routing Policy Auto-Created
                                          ↓
2. User Creation → Role Assignment → Access Mode Set (auto_grant for admins)
                                          ↓
3. API Request → Check Routing Policy → Check User Access → Route to Model
```

## Routing Policies

### What are Routing Policies?

Routing policies define how inference requests are routed to backend model deployments. They specify:

- **Model**: The model name (e.g., `qwen2-7b-instruct`, `gpt-4`)
- **Organization**: Which organization the policy applies to (or `*` for global)
- **Backends**: One or more backend deployments with traffic distribution weights
- **Failover**: Configuration for handling backend failures

### Auto-Creation on Model Registration

When a model is registered through the Admin API, a global routing policy is automatically created:

**File**: `/home/dev/ai-aas/services/admin-api-service/internal/service/registry.go`

```go
// Auto-create global routing policy for new models (default behavior)
// This ensures newly deployed models are immediately accessible to all orgs
func (s *ModelRegistryService) Register(ctx context.Context, reg *domain.ModelRegistration) (*domain.Model, bool, error) {
    // ... model registration logic ...

    // Auto-create routing policy
    if err := s.createDefaultRoutingPolicy(ctx, model); err != nil {
        // Log error but don't fail registration
        s.logger.Error("failed to auto-create routing policy", zap.Error(err))
    }

    return model, created, nil
}
```

**Default Policy Configuration**:
- `organization_id`: `*` (applies to all organizations)
- `model`: Model name from registration
- `backends`: Single backend with 100% traffic weight
- `backend_id`: Matches model name
- `failover_threshold`: 3 failed requests before failover
- `enabled`: true
- `metadata`: Includes auto-creation flag and model details

**Example Auto-Created Policy**:
```json
{
  "policy_id": "550e8400-e29b-41d4-a716-446655440000",
  "organization_id": "*",
  "model": "qwen2-7b-instruct",
  "backends": [
    {
      "backend_id": "qwen2-7b-instruct",
      "weight": 100
    }
  ],
  "failover_threshold": 3,
  "enabled": true,
  "metadata": {
    "auto_created": true,
    "created_reason": "auto-created on model registration"
  }
}
```

### Managing Routing Policies via CLI

The CLI provides commands for manually creating, listing, and deleting routing policies.

**File**: `/home/dev/ai-aas/services/ai-aas-cli/internal/admin/routing.go`

#### Create a Global Routing Policy

```bash
# Create global policy (applies to all organizations)
ai-aas-cli routing policy create \
  --global \
  --model qwen2-7b-instruct \
  --backends qwen2-7b-backend:100

# Create organization-specific policy
ai-aas-cli routing policy create \
  --org-id aa6f9015-132a-4694-8b10-7d4d4550faed \
  --model gpt-4 \
  --backends "backend-1:70,backend-2:30"
```

**Requirements**:
- Backend weights must sum to 100
- At least one backend must be specified
- Model name must be valid

#### List Routing Policies

```bash
# List all policies
ai-aas-cli routing policy list

# Filter by model
ai-aas-cli routing policy list --model gpt-4

# Filter by organization
ai-aas-cli routing policy list --org-id myorg

# JSON output
ai-aas-cli routing policy list --format json
```

#### Delete a Routing Policy

```bash
ai-aas-cli routing policy delete --policy-id <policy-uuid>
```

**Warning**: Deleting a routing policy will make the model inaccessible for routing. Ensure you have alternative policies or create a replacement before deletion.

## User Access Control

### Access Modes

Every user has an access mode that determines their default model access:

| Mode | Description | Use Case |
|------|-------------|----------|
| `auto_grant` | User has access to **all models** in the organization | Admin users, trusted developers, unrestricted access |
| `restricted` | User only has access to **explicitly granted models** | Regular users, external users, compliance requirements |

**Default Behavior**:
- **Admin users**: Automatically set to `auto_grant` mode on creation
- **Non-admin users**: Default to `restricted` mode

### Auto-Grant for Admin Users

Admin users are automatically assigned `auto_grant` access mode during user creation:

**File**: `/home/dev/ai-aas/services/user-org-service/internal/httpapi/users/handlers.go`

```go
// Set auto_grant for admin users
if containsRole(req.Roles, "admin") {
    _, err := h.runtime.Postgres.SetUserAccessMode(ctx, orgID, createdUser.ID, "auto_grant")
    if err != nil {
        h.logger.Warn("failed to set auto_grant for admin user", zap.Error(err))
        // Don't fail user creation, just log warning
    }
}
```

This applies to both invite-based and direct user creation methods.

### Model Grants

For users in `restricted` mode, access is controlled through explicit model grants:

**File**: `/home/dev/ai-aas/services/user-org-service/internal/storage/postgres/model_access.go`

**Grant Properties**:
- `model_name`: The model the user can access
- `granted_at`: Timestamp of when access was granted
- `granted_by`: (Optional) User ID who granted access
- `expires_at`: (Optional) Expiration timestamp for temporary access

**Storage Schema**:
```sql
-- User access mode
CREATE TABLE user_model_access (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    user_id UUID NOT NULL,
    access_mode TEXT NOT NULL, -- 'auto_grant' or 'restricted'
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    UNIQUE (org_id, user_id)
);

-- Model grants (only used for restricted mode)
CREATE TABLE user_model_grants (
    id UUID PRIMARY KEY,
    org_id UUID NOT NULL,
    user_id UUID NOT NULL,
    model_name TEXT NOT NULL,
    granted_at TIMESTAMP NOT NULL,
    granted_by UUID,
    expires_at TIMESTAMP,
    UNIQUE (org_id, user_id, model_name)
);
```

### Access Check Logic

When a user makes an inference request, the system checks:

1. **Does a routing policy exist for the model?**
   - If no → Request denied (404 Model Not Found)

2. **What is the user's access mode?**
   - If `auto_grant` → Request allowed
   - If `restricted` → Continue to step 3

3. **Does the user have an explicit grant for this model?**
   - If yes and not expired → Request allowed
   - If no or expired → Request denied (403 Forbidden)

**Hot Path Query** (optimized for performance):
```sql
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
    END as has_access
```

## CLI Commands

### User Creation with Model Access

**File**: `/home/dev/ai-aas/services/ai-aas-cli/internal/admin/user.go`

The `ai-aas-cli user create` command supports the `--model-access` flag to explicitly set access mode:

```bash
# Create user with unrestricted access to all models
ai-aas-cli user create \
  --org-id acme \
  --email user@example.com \
  --direct \
  --model-access all

# Create user with restricted access (must grant models explicitly)
ai-aas-cli user create \
  --org-id acme \
  --email user@example.com \
  --direct \
  --model-access restricted

# Create admin user (automatically gets auto_grant mode)
ai-aas-cli user create \
  --org-id acme \
  --email admin@example.com \
  --roles admin \
  --direct
```

**Flag Values**:
- `all`: Sets user to `auto_grant` mode (access to all models)
- `restricted`: Sets user to `restricted` mode (explicit grants required)
- Not specified: Defaults based on roles (admins get `all`, others get `restricted`)

### Model Access Management Commands

**File**: `/home/dev/ai-aas/services/ai-aas-cli/internal/admin/user_model_access.go`

#### Show User's Model Access

```bash
# Show access mode and granted models
ai-aas-cli user model-access show \
  --org-id acme \
  --email user@example.com

# By user ID
ai-aas-cli user model-access show \
  --org-id acme \
  --user-id <uuid>

# JSON output
ai-aas-cli user model-access show \
  --org-id acme \
  --email user@example.com \
  --format json
```

**Output Example**:
```
User Model Access:
  User ID:     550e8400-e29b-41d4-a716-446655440000
  Org ID:      aa6f9015-132a-4694-8b10-7d4d4550faed
  Access Mode: restricted

  Granted Models (2):
  Model                Granted At              Expires At
  -------------------- ----------------------- ----------
  qwen2-7b-instruct    2024-12-10T08:00:00Z   -
  gpt-4                2024-12-10T09:00:00Z   2024-12-17T09:00:00Z
```

#### Set Access Mode

```bash
# Set user to unrestricted mode (all models)
ai-aas-cli user model-access set-mode \
  --org-id acme \
  --email user@example.com \
  --mode auto_grant

# Set user to restricted mode (explicit grants only)
ai-aas-cli user model-access set-mode \
  --org-id acme \
  --email user@example.com \
  --mode restricted
```

#### Grant Model Access

```bash
# Grant permanent access to a model
ai-aas-cli user model-access grant \
  --org-id acme \
  --email user@example.com \
  --model qwen2-7b-instruct

# Grant temporary access (expires in 7 days)
ai-aas-cli user model-access grant \
  --org-id acme \
  --email user@example.com \
  --model gpt-4 \
  --expires-in 168h  # 7 days
```

**Expiration Format**: Use Go duration format (e.g., `24h`, `168h`, `720h`)

#### Revoke Model Access

```bash
ai-aas-cli user model-access revoke \
  --org-id acme \
  --email user@example.com \
  --model qwen2-7b-instruct
```

#### List Model Grants

```bash
# List all grants for a user
ai-aas-cli user model-access list \
  --org-id acme \
  --email user@example.com
```

#### Grant Multiple Models

```bash
# Grant access to multiple models at once
ai-aas-cli user model-access grant-all \
  --org-id acme \
  --email user@example.com \
  --models qwen2-7b-instruct,gpt-4,gpt-3.5-turbo
```

#### Migrate Existing Users

```bash
# Migrate all users in org to unrestricted mode
ai-aas-cli user model-access migrate \
  --org-id acme \
  --mode auto_grant

# Migrate all users to restricted mode with specific models
ai-aas-cli user model-access migrate \
  --org-id acme \
  --mode restricted \
  --models qwen2-7b-instruct,gpt-4

# Preview migration without making changes
ai-aas-cli user model-access migrate \
  --org-id acme \
  --mode restricted \
  --models gpt-4 \
  --dry-run
```

## Common Workflows

### Workflow 1: Deploy a New Model

1. **Deploy model infrastructure** (via Helm/ArgoCD)
2. **Register model** (auto-creates routing policy):
   ```bash
   # The Admin API auto-creates a global routing policy
   curl -X POST https://api.dev.otherjamesbrown.com/v1/models/register \
     -H "Authorization: Bearer $API_KEY" \
     -H "Content-Type: application/json" \
     -d '{
       "model_name": "new-model-v2",
       "deployment_endpoint": "http://new-model-v2.system.svc.cluster.local:8000",
       "deployment_environment": "development",
       "deployment_namespace": "system"
     }'
   ```
3. **Verify routing policy**:
   ```bash
   ai-aas-cli routing policy list --model new-model-v2
   ```
4. **Model is now available** to all users with `auto_grant` mode
5. **Grant access to restricted users**:
   ```bash
   ai-aas-cli user model-access grant \
     --org-id acme \
     --email user@example.com \
     --model new-model-v2
   ```

### Workflow 2: Onboard a New User

1. **Create user**:
   ```bash
   ai-aas-cli user create \
     --org-id acme \
     --email newuser@example.com \
     --display-name "New User" \
     --direct \
     --model-access restricted
   ```

2. **Grant access to specific models**:
   ```bash
   ai-aas-cli user model-access grant \
     --org-id acme \
     --email newuser@example.com \
     --model qwen2-7b-instruct
   ```

3. **Verify access**:
   ```bash
   ai-aas-cli user model-access show \
     --org-id acme \
     --email newuser@example.com
   ```

### Workflow 3: Restrict an Existing Admin User

1. **Change access mode from auto_grant to restricted**:
   ```bash
   ai-aas-cli user model-access set-mode \
     --org-id acme \
     --email admin@example.com \
     --mode restricted
   ```

2. **Grant specific models they should have access to**:
   ```bash
   ai-aas-cli user model-access grant-all \
     --org-id acme \
     --email admin@example.com \
     --models qwen2-7b-instruct,gpt-4
   ```

### Workflow 4: Grant Temporary Model Access

1. **Grant access with expiration**:
   ```bash
   # Access expires in 24 hours
   ai-aas-cli user model-access grant \
     --org-id acme \
     --email contractor@example.com \
     --model gpt-4 \
     --expires-in 24h
   ```

2. **User can access model until expiration**
3. **Access automatically revokes after expiration** (checked on each request)

### Workflow 5: Migrate Organization to Restricted Access

1. **Audit current user access**:
   ```bash
   ai-aas-cli user list --org-id acme
   ```

2. **Preview migration**:
   ```bash
   ai-aas-cli user model-access migrate \
     --org-id acme \
     --mode restricted \
     --models qwen2-7b-instruct,gpt-4 \
     --dry-run
   ```

3. **Execute migration**:
   ```bash
   ai-aas-cli user model-access migrate \
     --org-id acme \
     --mode restricted \
     --models qwen2-7b-instruct,gpt-4
   ```

4. **Verify migration**:
   ```bash
   # Check a sample user
   ai-aas-cli user model-access show \
     --org-id acme \
     --email user@example.com
   ```

## Security Considerations

### Principle of Least Privilege

- **Default to restricted**: New non-admin users should default to `restricted` mode
- **Explicit grants**: Only grant access to models that users need
- **Time-bounded access**: Use `--expires-in` for temporary access needs
- **Regular audits**: Periodically review user access with `user model-access show`

### Admin User Access

- **Auto-grant by default**: Admin users get `auto_grant` mode automatically
- **Can be restricted**: Admin role doesn't prevent setting to `restricted` mode
- **Separation of duties**: Consider restricting admin access in production environments

### Routing Policy Security

- **Global vs. Org-specific**: Global policies (`organization_id: "*"`) apply to all orgs
- **Policy conflicts**: Org-specific policies override global policies
- **Backend validation**: Backend IDs should match actual deployed services

### Compliance

The model access control system supports compliance requirements:

- **Audit trails**: All access changes are logged in audit events
- **Access reports**: Use CLI to generate access reports for compliance reviews
- **Temporary access**: Support for time-limited access grants
- **Access revocation**: Immediate revocation through CLI commands

## Troubleshooting

### User Cannot Access Model

**Symptom**: API returns 403 Forbidden when user tries to access a model

**Diagnosis**:
```bash
# 1. Check if routing policy exists
ai-aas-cli routing policy list --model <model-name>

# 2. Check user's access configuration
ai-aas-cli user model-access show --org-id <org> --email <email>
```

**Solutions**:
- If no routing policy: Model not registered or policy was deleted
- If user is `restricted` and no grant: Grant access with `user model-access grant`
- If grant expired: Grant new access or set to `auto_grant` mode

### Model Not Found (404)

**Symptom**: API returns 404 Not Found

**Diagnosis**:
```bash
# Check if routing policy exists
ai-aas-cli routing policy list --model <model-name>
```

**Solutions**:
- Model not registered: Register model through Admin API
- Routing policy deleted: Recreate policy with `routing policy create`
- Typo in model name: Verify exact model name spelling

### Admin User Has Restricted Access

**Symptom**: Admin user cannot access all models

**Diagnosis**:
```bash
ai-aas-cli user model-access show --org-id <org> --email <admin-email>
```

**Solution**:
```bash
# Set admin to auto_grant mode
ai-aas-cli user model-access set-mode \
  --org-id <org> \
  --email <admin-email> \
  --mode auto_grant
```

### Routing Policy Not Auto-Created

**Symptom**: Model registered but no routing policy exists

**Diagnosis**:
- Check Admin API logs for errors during policy creation
- Verify PolicyRepository is properly initialized

**Solution**:
```bash
# Manually create routing policy
ai-aas-cli routing policy create \
  --global \
  --model <model-name> \
  --backends <model-name>:100
```

## API Endpoints

### Admin API Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/models/register` | Register model (auto-creates routing policy) |
| GET | `/v1/models` | List registered models |
| POST | `/v1/routing/policies` | Create routing policy |
| GET | `/v1/routing/policies` | List routing policies |
| DELETE | `/v1/routing/policies/{id}` | Delete routing policy |

### User-Org Service

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/orgs/{orgId}/users/{userId}/model-access` | Get user's model access |
| PUT | `/v1/orgs/{orgId}/users/{userId}/model-access/mode` | Set access mode |
| GET | `/v1/orgs/{orgId}/users/{userId}/model-access/grants` | List model grants |
| POST | `/v1/orgs/{orgId}/users/{userId}/model-access/grants` | Grant model access |
| DELETE | `/v1/orgs/{orgId}/users/{userId}/model-access/grants/{modelName}` | Revoke model access |
| POST | `/v1/orgs/{orgId}/users/{userId}/model-access/grants/all-current` | Grant multiple models |

## Best Practices

1. **Use auto_grant sparingly**: Reserve for trusted users and admins
2. **Default to restricted**: Start new users with restricted access
3. **Document access decisions**: Use descriptive metadata in grants
4. **Regular access reviews**: Audit user access quarterly
5. **Automate with CLI**: Use scripts for bulk operations
6. **Test routing policies**: Verify policies before production deployment
7. **Monitor access patterns**: Track which models are most accessed
8. **Plan for model sunset**: Revoke access before decommissioning models

## Related Documentation

- Model Registry: `/home/dev/ai-aas/services/admin-api-service/README.md`
- User Management: `/home/dev/ai-aas/services/user-org-service/README.md`
- CLI Reference: `/home/dev/ai-aas/services/ai-aas-cli/README.md`
- API Router Service: `/home/dev/ai-aas/services/api-router-service/README.md`

## Change History

- **2024-12-10**: Initial documentation created
  - Documented routing policy auto-creation (ai-aas-iti)
  - Documented auto_grant for admin users (ai-aas-cl3)
  - Documented --model-access CLI flag (ai-aas-1bp)
