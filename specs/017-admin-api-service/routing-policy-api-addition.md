# Routing Policy API Specification
## Addition to Admin API Service Spec (017)

This document provides the complete routing policy management API that should be added to the Admin API Service specification.

---

## New Functional Requirements (FR-013 through FR-020)

- **FR-013**: Provide routing policy CRUD endpoints for managing model routing configuration independently from deployments (POST, GET, PATCH, DELETE /v1/routing/policies).

- **FR-014**: Provide automatic routing policy creation when registering models via `auto_create_policy` flag in model registration endpoint.

- **FR-015**: Provide policy validation ensuring referenced backends exist in model registry, weights sum to 100, and organization context matches authentication.

- **FR-016**: Provide policy versioning with automatic version increment on updates to enable rollback and audit trail.

- **FR-017**: Provide policy sync endpoint (GET /v1/routing/policies/sync) for api-router-service instances to poll for updates with If-Modified-Since support.

- **FR-018**: Provide policy activation/deactivation without deletion to enable quick traffic cutover during incidents.

- **FR-019**: Provide policy export/import endpoints for backup and migration scenarios (GET/POST /v1/routing/policies/export).

- **FR-020**: Provide policy validation endpoint (POST /v1/routing/policies/validate) to test policy configurations before applying them.

---

## Routing Policy API Endpoints

### 1. Create Routing Policy

**Endpoint**: `POST /v1/routing/policies`

**Description**: Creates a new routing policy for a model. Policies define how traffic is distributed across backend deployments.

**Authentication**: Required (admin API key)

**Request Body**:
```json
{
  "policy_id": "string (optional, auto-generated if omitted)",
  "organization_id": "string (UUID or '*' for global)",
  "model": "string (required, model name)",
  "backends": [
    {
      "backend_id": "string (required, must exist in model registry)",
      "weight": "integer (required, 1-100)"
    }
  ],
  "fallback_backends": [
    {
      "backend_id": "string (optional fallback)",
      "weight": "integer"
    }
  ],
  "failover_threshold": "integer (optional, default 3)",
  "enabled": "boolean (optional, default true)",
  "metadata": {
    "description": "string (optional)",
    "owner": "string (optional)",
    "tags": ["string"]
  }
}
```

**Validation Rules**:
- `model`: Required, 1-255 characters, alphanumeric + hyphens
- `organization_id`: Must be valid UUID or "*" for global policies
- `backends`: Required, at least 1 backend, max 10 backends
- `backends[].backend_id`: Must exist in model registry for same environment
- `backends[].weight`: Integer 1-100, sum of all weights must equal 100
- `failover_threshold`: Integer 1-10, defaults to 3
- `policy_id`: If provided, must be unique; format: `{org_id}-{model}` or custom

**Success Response** (201 Created):
```json
{
  "policy_id": "uuid-or-custom-id",
  "organization_id": "uuid-or-*",
  "model": "gpt-oss-20b",
  "backends": [
    {
      "backend_id": "gpt-oss-20b",
      "weight": 100
    }
  ],
  "fallback_backends": [],
  "failover_threshold": 3,
  "enabled": true,
  "version": 1,
  "metadata": {},
  "created_at": "2025-11-26T12:00:00Z",
  "updated_at": "2025-11-26T12:00:00Z",
  "created_by": "api-key-id-last4"
}
```

**Error Responses**:

**400 Bad Request** - Validation failure:
```json
{
  "type": "https://docs.ai-aas.local/errors/validation-error",
  "title": "Validation Failed",
  "status": 400,
  "detail": "One or more validation errors occurred",
  "instance": "/v1/routing/policies",
  "errors": [
    {
      "field": "backends[0].backend_id",
      "message": "Backend 'nonexistent-backend' does not exist in model registry"
    },
    {
      "field": "backends",
      "message": "Total weight must equal 100 (currently 80)"
    }
  ]
}
```

**409 Conflict** - Policy already exists:
```json
{
  "type": "https://docs.ai-aas.local/errors/conflict",
  "title": "Policy Already Exists",
  "status": 409,
  "detail": "A routing policy for model 'gpt-oss-20b' and organization '*' already exists",
  "instance": "/v1/routing/policies",
  "existing_policy_id": "uuid",
  "suggestion": "Use PATCH to update existing policy or DELETE then POST to replace"
}
```

**403 Forbidden** - Insufficient permissions:
```json
{
  "type": "https://docs.ai-aas.local/errors/forbidden",
  "title": "Insufficient Permissions",
  "status": 403,
  "detail": "API key does not have permission to create global policies",
  "instance": "/v1/routing/policies"
}
```

---

### 2. List Routing Policies

**Endpoint**: `GET /v1/routing/policies`

**Description**: Lists routing policies with optional filtering and pagination.

**Authentication**: Required (admin API key)

**Query Parameters**:
- `model` (string, optional): Filter by model name
- `organization_id` (string, optional): Filter by organization ID or "*"
- `enabled` (boolean, optional): Filter by enabled status
- `backend_id` (string, optional): Filter policies using specific backend
- `limit` (integer, optional): Page size (default 100, max 500)
- `offset` (integer, optional): Pagination offset (default 0)
- `sort` (string, optional): Sort field (created_at, updated_at, model)
- `order` (string, optional): Sort order (asc, desc, default desc)

**Success Response** (200 OK):
```json
{
  "policies": [
    {
      "policy_id": "uuid",
      "organization_id": "*",
      "model": "gpt-oss-20b",
      "backends": [
        {
          "backend_id": "gpt-oss-20b",
          "weight": 80,
          "health_status": "healthy"
        },
        {
          "backend_id": "gpt-oss-20b-v2",
          "weight": 20,
          "health_status": "healthy"
        }
      ],
      "fallback_backends": [],
      "failover_threshold": 3,
      "enabled": true,
      "version": 3,
      "metadata": {
        "description": "Canary deployment 80/20 split"
      },
      "last_synced_at": "2025-11-26T12:05:30Z",
      "sync_status": "synced",
      "created_at": "2025-11-26T10:00:00Z",
      "updated_at": "2025-11-26T12:00:00Z"
    }
  ],
  "pagination": {
    "total": 15,
    "limit": 100,
    "offset": 0,
    "next_offset": null
  }
}
```

**Notes**:
- `health_status` is enriched from api-router-service health monitoring
- `last_synced_at` shows when api-router instances last fetched this policy
- `sync_status` indicates if policy is propagated: "synced", "pending", "error"

---

### 3. Get Routing Policy

**Endpoint**: `GET /v1/routing/policies/{policy_id}`

**Description**: Retrieves a specific routing policy with full details including sync status across api-router instances.

**Authentication**: Required (admin API key)

**Success Response** (200 OK):
```json
{
  "policy_id": "uuid",
  "organization_id": "*",
  "model": "gpt-oss-20b",
  "backends": [
    {
      "backend_id": "gpt-oss-20b",
      "weight": 100,
      "health_status": "healthy",
      "endpoint": "gpt-oss-20b-predictor-00001-private.development.svc.cluster.local:80",
      "last_health_check": "2025-11-26T12:10:00Z"
    }
  ],
  "fallback_backends": [],
  "failover_threshold": 3,
  "enabled": true,
  "version": 1,
  "metadata": {},
  "sync_info": {
    "instances_synced": 3,
    "instances_total": 3,
    "last_sync_times": [
      {
        "instance_id": "api-router-abc123",
        "synced_at": "2025-11-26T12:05:30Z",
        "version": 1
      }
    ]
  },
  "usage_stats": {
    "requests_last_hour": 1523,
    "errors_last_hour": 2,
    "avg_latency_ms": 145.3
  },
  "created_at": "2025-11-26T10:00:00Z",
  "updated_at": "2025-11-26T10:00:00Z",
  "created_by": "api-key-1234",
  "updated_by": "api-key-1234"
}
```

**Error Response** (404 Not Found):
```json
{
  "type": "https://docs.ai-aas.local/errors/not-found",
  "title": "Policy Not Found",
  "status": 404,
  "detail": "Routing policy with ID 'invalid-uuid' does not exist",
  "instance": "/v1/routing/policies/invalid-uuid"
}
```

---

### 4. Update Routing Policy

**Endpoint**: `PATCH /v1/routing/policies/{policy_id}`

**Description**: Updates an existing routing policy. Only provided fields are updated. Version is automatically incremented.

**Authentication**: Required (admin API key)

**Request Body** (partial updates allowed):
```json
{
  "backends": [
    {
      "backend_id": "gpt-oss-20b",
      "weight": 70
    },
    {
      "backend_id": "gpt-oss-20b-v2",
      "weight": 30
    }
  ],
  "metadata": {
    "description": "Increased canary to 30%"
  }
}
```

**Success Response** (200 OK):
```json
{
  "policy_id": "uuid",
  "organization_id": "*",
  "model": "gpt-oss-20b",
  "backends": [
    {
      "backend_id": "gpt-oss-20b",
      "weight": 70
    },
    {
      "backend_id": "gpt-oss-20b-v2",
      "weight": 30
    }
  ],
  "version": 4,
  "updated_at": "2025-11-26T12:15:00Z",
  "updated_by": "api-key-5678"
}
```

**Error Responses**:

**409 Conflict** - Concurrent modification:
```json
{
  "type": "https://docs.ai-aas.local/errors/conflict",
  "title": "Concurrent Modification",
  "status": 409,
  "detail": "Policy was modified by another request. Current version is 5, expected 4",
  "instance": "/v1/routing/policies/uuid",
  "current_version": 5,
  "suggestion": "Fetch latest policy and retry update"
}
```

---

### 5. Delete Routing Policy

**Endpoint**: `DELETE /v1/routing/policies/{policy_id}`

**Description**: Deletes a routing policy. Traffic will fail for this model until a new policy is created or default routing is configured.

**Authentication**: Required (admin API key)

**Query Parameters**:
- `force` (boolean, optional): If true, deletes even if traffic is currently active (default false)

**Success Response** (204 No Content)

**Error Response** (409 Conflict):
```json
{
  "type": "https://docs.ai-aas.local/errors/conflict",
  "title": "Policy In Use",
  "status": 409,
  "detail": "Cannot delete policy: 1523 requests in last hour. Use ?force=true to override",
  "instance": "/v1/routing/policies/uuid",
  "usage_stats": {
    "requests_last_hour": 1523
  }
}
```

---

### 6. Validate Routing Policy

**Endpoint**: `POST /v1/routing/policies/validate`

**Description**: Validates a policy configuration without creating it. Useful for testing configurations before applying.

**Authentication**: Required (admin API key)

**Request Body**: Same as create policy

**Success Response** (200 OK):
```json
{
  "valid": true,
  "warnings": [
    "Backend 'gpt-oss-20b-v2' has not received traffic in 24 hours - may be cold"
  ],
  "estimated_impact": {
    "affected_organizations": ["*"],
    "estimated_requests_per_hour": 1500
  }
}
```

**Validation Failure Response** (200 OK with valid: false):
```json
{
  "valid": false,
  "errors": [
    {
      "field": "backends[1].backend_id",
      "message": "Backend 'nonexistent' does not exist"
    }
  ],
  "warnings": []
}
```

---

### 7. Policy Sync Endpoint (for api-router-service)

**Endpoint**: `GET /v1/routing/policies/sync`

**Description**: Optimized endpoint for api-router-service to poll for policy updates. Returns only changed policies since last sync.

**Authentication**: Required (service-to-service API key)

**Headers**:
- `If-Modified-Since` (optional): RFC 7231 date, returns 304 if no changes
- `X-Router-Instance-ID` (required): Unique ID of api-router instance

**Query Parameters**:
- `since_version` (integer, optional): Return policies changed since this version
- `environment` (string, optional): Filter by deployment environment

**Success Response** (200 OK):
```json
{
  "policies": [
    {
      "policy_id": "uuid",
      "organization_id": "*",
      "model": "gpt-oss-20b",
      "backends": [...],
      "version": 5,
      "deleted": false
    },
    {
      "policy_id": "uuid2",
      "model": "deleted-model",
      "deleted": true,
      "version": 3
    }
  ],
  "sync_metadata": {
    "server_time": "2025-11-26T12:20:00Z",
    "max_version": 5,
    "next_sync_recommended_at": "2025-11-26T12:21:00Z"
  }
}
```

**Response** (304 Not Modified) - No changes since If-Modified-Since

---

### 8. Activate/Deactivate Policy

**Endpoint**: `POST /v1/routing/policies/{policy_id}/activate`
**Endpoint**: `POST /v1/routing/policies/{policy_id}/deactivate`

**Description**: Quick enable/disable of policies without deletion. Useful for incident response.

**Authentication**: Required (admin API key)

**Success Response** (200 OK):
```json
{
  "policy_id": "uuid",
  "enabled": false,
  "updated_at": "2025-11-26T12:25:00Z",
  "message": "Policy deactivated. Traffic will fail until policy is reactivated or deleted."
}
```

---

## Integration with Model Registry

### Enhanced Model Registration

Update `POST /v1/registry/models` to support automatic policy creation:

**Request Body Addition**:
```json
{
  "model_name": "gpt-oss-20b",
  "deployment_endpoint": "...",
  "deployment_environment": "development",
  "auto_create_policy": true,
  "policy_config": {
    "organization_id": "*",
    "weight": 100,
    "failover_threshold": 3,
    "metadata": {
      "description": "Auto-created during model registration"
    }
  }
}
```

**Response Addition**:
```json
{
  "model_id": "uuid",
  "model_name": "gpt-oss-20b",
  "routing_policy_created": true,
  "routing_policy_id": "*-gpt-oss-20b",
  "routing_policy_version": 1
}
```

**Behavior**:
- If `auto_create_policy: true` and policy doesn't exist → Create new policy
- If `auto_create_policy: true` and policy exists → Update existing policy (upsert)
- If `auto_create_policy: false` or omitted → No policy changes
- Transaction: Both model registration and policy creation succeed or both fail

---

## Database Schema

### routing_policies Table

```sql
CREATE TABLE routing_policies (
    policy_id VARCHAR(255) PRIMARY KEY,
    organization_id UUID NOT NULL,
    model VARCHAR(255) NOT NULL,
    backends JSONB NOT NULL,
    fallback_backends JSONB DEFAULT '[]'::jsonb,
    failover_threshold INTEGER NOT NULL DEFAULT 3,
    enabled BOOLEAN NOT NULL DEFAULT true,
    version INTEGER NOT NULL DEFAULT 1,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    deleted_at TIMESTAMP NULL,

    CONSTRAINT fk_organization FOREIGN KEY (organization_id)
        REFERENCES organizations(organization_id) ON DELETE CASCADE,
    CONSTRAINT unique_org_model UNIQUE (organization_id, model)
        WHERE deleted_at IS NULL,
    CONSTRAINT check_backends_not_empty CHECK (jsonb_array_length(backends) > 0),
    CONSTRAINT check_failover_threshold CHECK (failover_threshold BETWEEN 1 AND 10)
);

CREATE INDEX idx_routing_policies_model ON routing_policies(model)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_routing_policies_org ON routing_policies(organization_id)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_routing_policies_enabled ON routing_policies(enabled)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_routing_policies_updated_at ON routing_policies(updated_at DESC);

-- Support for global policies (organization_id = '*')
-- Requires special handling in application code
```

### policy_sync_log Table

Tracks when api-router instances sync policies for monitoring:

```sql
CREATE TABLE policy_sync_log (
    id BIGSERIAL PRIMARY KEY,
    router_instance_id VARCHAR(255) NOT NULL,
    policy_id VARCHAR(255) NOT NULL,
    policy_version INTEGER NOT NULL,
    synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sync_duration_ms INTEGER,
    environment VARCHAR(50),

    INDEX idx_sync_log_instance (router_instance_id, synced_at DESC),
    INDEX idx_sync_log_policy (policy_id, synced_at DESC)
);

-- Cleanup old logs (retention 7 days)
CREATE OR REPLACE FUNCTION cleanup_old_sync_logs() RETURNS void AS $$
BEGIN
    DELETE FROM policy_sync_log WHERE synced_at < NOW() - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;
```

---

## Policy Sync Architecture

### Option 1: HTTP Polling (Recommended for MVP)

**api-router-service implementation**:
```go
// Poll every 30 seconds
ticker := time.NewTicker(30 * time.Second)
for range ticker.C {
    policies, err := adminClient.SyncPolicies(ctx, lastSyncVersion)
    if err != nil {
        logger.Error("policy sync failed", zap.Error(err))
        continue
    }

    for _, policy := range policies {
        if policy.Deleted {
            cache.DeletePolicy(ctx, policy.PolicyID)
        } else {
            cache.StorePolicy(ctx, &policy)
        }
    }

    lastSyncVersion = policies.MaxVersion
}
```

**Advantages**:
- Simple implementation
- No additional infrastructure
- Works with firewall restrictions
- Easy to debug

**Disadvantages**:
- 30-60 second delay for policy updates
- Unnecessary polling when no changes

---

### Option 2: Server-Sent Events (SSE) (Recommended for Production)

**Admin API Service**:
```go
// GET /v1/routing/policies/stream
func handlePolicyStream(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")

    // Subscribe to policy changes
    ch := policyNotifier.Subscribe(r.Context())

    for {
        select {
        case policy := <-ch:
            data, _ := json.Marshal(policy)
            fmt.Fprintf(w, "data: %s\n\n", data)
            w.(http.Flusher).Flush()
        case <-r.Context().Done():
            return
        }
    }
}
```

**Advantages**:
- Real-time updates (<1 second)
- Efficient (no polling overhead)
- Standard HTTP (no WebSocket complexity)

---

## Security Considerations

### 1. API Key Scopes

Define policy-specific scopes:
- `policies:read` - List and get policies
- `policies:write` - Create and update policies
- `policies:delete` - Delete policies
- `policies:admin` - All policy operations including force delete

### 2. Organization Isolation

- API keys scoped to organizations can only manage policies for their organization
- Global policies (`organization_id: "*"`) require `policies:admin` scope
- Validation ensures authenticated org matches `organization_id` in request

### 3. Audit Logging

All policy operations create audit log entries:
```json
{
  "timestamp": "2025-11-26T12:00:00Z",
  "actor": "api-key-1234",
  "action": "routing_policy.update",
  "resource_type": "routing_policy",
  "resource_id": "uuid",
  "changes": {
    "backends": {
      "from": [{"backend_id": "gpt-oss-20b", "weight": 100}],
      "to": [
        {"backend_id": "gpt-oss-20b", "weight": 80},
        {"backend_id": "gpt-oss-20b-v2", "weight": 20}
      ]
    },
    "version": {"from": 3, "to": 4}
  },
  "client_ip": "10.0.1.5",
  "user_agent": "admin-cli/1.0.0"
}
```

### 4. Rate Limiting

- Policy mutations (POST, PATCH, DELETE): 10 req/min per API key
- Policy reads (GET): 100 req/min per API key
- Sync endpoint (GET /sync): 120 req/hour per router instance

### 5. Backend Validation

Before creating/updating policy:
```go
// Validate all backends exist
for _, backend := range policy.Backends {
    exists, err := modelRegistry.BackendExists(
        ctx,
        backend.BackendID,
        policy.Environment,
    )
    if !exists {
        return fmt.Errorf("backend %s not found", backend.BackendID)
    }
}
```

---

## Error Codes Reference

| Code | HTTP Status | Description |
|------|-------------|-------------|
| `POLICY_NOT_FOUND` | 404 | Policy with given ID doesn't exist |
| `POLICY_EXISTS` | 409 | Policy already exists for org+model |
| `INVALID_BACKEND` | 400 | Referenced backend doesn't exist |
| `INVALID_WEIGHTS` | 400 | Weights don't sum to 100 |
| `POLICY_IN_USE` | 409 | Cannot delete policy with active traffic |
| `CONCURRENT_MODIFICATION` | 409 | Policy was modified by another request |
| `INSUFFICIENT_PERMISSIONS` | 403 | API key lacks required scope |
| `VALIDATION_ERROR` | 400 | One or more fields invalid |

---

## Testing Strategy

### Unit Tests
- Policy validation logic (weights, backend existence)
- Version increment logic
- Soft delete behavior

### Integration Tests
- Create policy → Verify in database
- Update policy → Version increments, audit log created
- Delete policy → Soft deleted, not returned in list
- Sync endpoint → Returns only changed policies

### End-to-End Tests
1. Register model with `auto_create_policy: true`
2. Verify policy created
3. Verify api-router-service syncs policy within 60s
4. Send traffic to model
5. Verify traffic routed correctly
6. Update policy weights
7. Verify traffic distribution changes

---

## Metrics

Expose Prometheus metrics:

```
# Policy operations
policy_operations_total{operation="create|update|delete|get|list",status="success|error"}
policy_operation_duration_seconds{operation="create|update|delete|get|list"}

# Sync operations
policy_sync_requests_total{instance_id="...",status="success|error"}
policy_sync_duration_seconds
policies_synced_per_request

# Policy state
active_policies_total{environment="..."}
policy_backend_count{policy_id="..."}
policies_pending_sync_total
```

---

## Migration Path

### Phase 1: Deploy Admin API Service
- Deploy admin-api-service with routing policy endpoints
- Existing bootstrap code in api-router-service remains unchanged

### Phase 2: Enable Policy Sync
- Configure api-router-service to poll admin-api-service for policies
- Bootstrap policies are used if admin-api-service unreachable (fallback)

### Phase 3: Migrate Existing Policies
- Script to convert hardcoded bootstrap policies to database entries
- Create policies via API for all existing models

### Phase 4: Remove Bootstrap Code
- Update api-router-service to require policies from admin-api-service
- Remove hardcoded bootstrap policies (except minimal fallback)

---

## Open Questions / Future Enhancements

1. **Policy Scheduling**: Should we support time-based policy changes? (e.g., route to cheaper backend during off-peak hours)

2. **A/B Testing**: Should we support user-based routing? (e.g., 10% of organization X to v2 backend)

3. **Geolocation Routing**: Should policies support region-based routing?

4. **Policy Templates**: Should we provide pre-built policy templates (canary, blue-green, etc.)?

5. **Cost Optimization**: Should policies consider backend cost when making routing decisions?

6. **Policy Inheritance**: Should org-specific policies inherit from global policies with override capability?
