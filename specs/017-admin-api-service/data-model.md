# Data Model: Admin API Service

**Feature**: 017-admin-api-service
**Date**: 2025-11-26

## Entities

### 1. Model (model_registry table - existing)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| model_id | UUID | PK, auto-generated | Unique identifier |
| model_name | VARCHAR(255) | NOT NULL | Model name (e.g., gpt-oss-20b) |
| deployment_endpoint | VARCHAR(512) | NOT NULL | Kubernetes service endpoint |
| deployment_environment | VARCHAR(50) | NOT NULL | development/staging/production |
| deployment_namespace | VARCHAR(255) | NOT NULL | Kubernetes namespace |
| deployment_status | VARCHAR(50) | NOT NULL | ready/degraded/offline/pending |
| deployment_target | VARCHAR(50) | DEFAULT 'managed' | managed/external |
| revision | INTEGER | DEFAULT 1 | Deployment revision |
| cost_per_1k_tokens | DECIMAL(10,6) | NULL | Cost tracking |
| metadata | JSONB | DEFAULT '{}' | Additional metadata |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Last update timestamp |

**Unique Constraint**: (model_name, deployment_environment)

### 2. Organization (organizations table - existing)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| organization_id | UUID | PK, auto-generated | Unique identifier |
| slug | VARCHAR(100) | UNIQUE, NOT NULL | URL-friendly identifier |
| display_name | VARCHAR(255) | NOT NULL | Human-readable name |
| plan_tier | VARCHAR(50) | NOT NULL | free/starter/enterprise |
| status | VARCHAR(50) | DEFAULT 'active' | active/suspended/deleted |
| budget_limit_tokens | BIGINT | NULL | Token budget limit |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Last update timestamp |

### 3. RoutingPolicy (routing_policies table - new)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| policy_id | VARCHAR(255) | PK | Custom or auto-generated ID |
| organization_id | UUID | NOT NULL, FK | Organization or '*' for global |
| model | VARCHAR(255) | NOT NULL | Target model name |
| backends | JSONB | NOT NULL | Array of backend configs |
| fallback_backends | JSONB | DEFAULT '[]' | Fallback backend configs |
| failover_threshold | INTEGER | DEFAULT 3, CHECK 1-10 | Failures before failover |
| enabled | BOOLEAN | DEFAULT true | Policy active status |
| version | INTEGER | DEFAULT 1 | Auto-incremented version |
| metadata | JSONB | DEFAULT '{}' | Description, tags, etc. |
| created_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Creation timestamp |
| updated_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Last update timestamp |
| created_by | VARCHAR(255) | NULL | API key ID |
| updated_by | VARCHAR(255) | NULL | API key ID |
| deleted_at | TIMESTAMP | NULL | Soft delete timestamp |

**Unique Constraint**: (organization_id, model) WHERE deleted_at IS NULL

**backends JSONB structure**:
```json
[
  {
    "backend_id": "string",
    "weight": 100
  }
]
```

### 4. AuditLog (audit_logs table - new or existing)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | BIGSERIAL | PK | Auto-increment ID |
| timestamp | TIMESTAMP | NOT NULL, DEFAULT NOW() | Event timestamp |
| actor | VARCHAR(255) | NOT NULL | API key ID or subject |
| action | VARCHAR(100) | NOT NULL | Operation performed |
| resource_type | VARCHAR(50) | NOT NULL | model/organization/policy |
| resource_id | VARCHAR(255) | NOT NULL | Target resource identifier |
| outcome | VARCHAR(20) | NOT NULL | success/failure |
| changes | JSONB | NULL | Before/after values |
| error_detail | TEXT | NULL | Error message if failed |
| client_ip | INET | NULL | Client IP address |
| user_agent | VARCHAR(512) | NULL | Client user agent |
| request_id | UUID | NULL | Correlation ID |

**Indexes**: (actor, timestamp DESC), (resource_type, resource_id, timestamp DESC)

### 5. PolicySyncLog (policy_sync_log table - new)

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | BIGSERIAL | PK | Auto-increment ID |
| router_instance_id | VARCHAR(255) | NOT NULL | api-router pod ID |
| policy_id | VARCHAR(255) | NOT NULL | Synced policy ID |
| policy_version | INTEGER | NOT NULL | Version synced |
| synced_at | TIMESTAMP | NOT NULL, DEFAULT NOW() | Sync timestamp |
| sync_duration_ms | INTEGER | NULL | Sync duration |
| environment | VARCHAR(50) | NULL | Environment |

**Indexes**: (router_instance_id, synced_at DESC), (policy_id, synced_at DESC)
**Retention**: 7 days

## Relationships

```
Organization (1) ----< (N) RoutingPolicy
    organization_id       organization_id (FK)

Model (1) ----< (N) RoutingPolicy.backends[].backend_id
    model_name            backend_id (logical reference)

AuditLog references all entities via resource_type + resource_id
```

## State Transitions

### Model Status
```
pending -> ready -> degraded -> offline
                 -> ready (recovery)
```

### Organization Status
```
active -> suspended -> active (reactivation)
       -> deleted (soft delete)
```

### RoutingPolicy
```
enabled: true -> false (deactivate)
              -> true (activate)
deleted_at: NULL -> timestamp (soft delete)
```

## Validation Rules

### Model
- model_name: 1-255 chars, alphanumeric + hyphens
- deployment_environment: enum (development, staging, production)
- deployment_status: enum (ready, degraded, offline, pending)
- deployment_endpoint: valid hostname:port format

### Organization
- slug: 1-100 chars, lowercase alphanumeric + hyphens, must start with letter
- plan_tier: enum (free, starter, enterprise)
- status: enum (active, suspended, deleted)

### RoutingPolicy
- model: 1-255 chars, must exist in model_registry
- backends: 1-10 items, weights sum to 100
- failover_threshold: 1-10
- organization_id: valid UUID or '*' for global

## Migration Scripts

```sql
-- 001_create_routing_policies.up.sql
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
    
    CONSTRAINT check_backends_not_empty CHECK (jsonb_array_length(backends) > 0),
    CONSTRAINT check_failover_threshold CHECK (failover_threshold BETWEEN 1 AND 10)
);

CREATE INDEX idx_routing_policies_model ON routing_policies(model) WHERE deleted_at IS NULL;
CREATE INDEX idx_routing_policies_org ON routing_policies(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_routing_policies_enabled ON routing_policies(enabled) WHERE deleted_at IS NULL;
CREATE INDEX idx_routing_policies_updated_at ON routing_policies(updated_at DESC);
CREATE UNIQUE INDEX idx_routing_policies_unique_org_model ON routing_policies(organization_id, model) WHERE deleted_at IS NULL;

-- 002_create_audit_logs.up.sql
CREATE TABLE IF NOT EXISTS audit_logs (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL DEFAULT NOW(),
    actor VARCHAR(255) NOT NULL,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255) NOT NULL,
    outcome VARCHAR(20) NOT NULL,
    changes JSONB,
    error_detail TEXT,
    client_ip INET,
    user_agent VARCHAR(512),
    request_id UUID
);

CREATE INDEX idx_audit_logs_actor ON audit_logs(actor, timestamp DESC);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id, timestamp DESC);

-- 003_create_policy_sync_log.up.sql
CREATE TABLE policy_sync_log (
    id BIGSERIAL PRIMARY KEY,
    router_instance_id VARCHAR(255) NOT NULL,
    policy_id VARCHAR(255) NOT NULL,
    policy_version INTEGER NOT NULL,
    synced_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sync_duration_ms INTEGER,
    environment VARCHAR(50)
);

CREATE INDEX idx_sync_log_instance ON policy_sync_log(router_instance_id, synced_at DESC);
CREATE INDEX idx_sync_log_policy ON policy_sync_log(policy_id, synced_at DESC);
```

