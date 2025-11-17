# Data Model: Model Inference Deployment

**Branch**: `010-vllm-deployment`  
**Date**: 2025-01-27  
**Spec**: [spec.md](./spec.md)

## Entities

### ModelRegistryEntry (Extended)

- **description**: Existing entity extended with deployment metadata for vLLM inference engines.
- **base table**: `model_registry_entries` (from `003-database-schemas`)
- **new fields**:
  - `deployment_endpoint`: TEXT nullable - Kubernetes service endpoint URL (e.g., `llama-7b-production.system.svc.cluster.local:8000`)
  - `deployment_status`: TEXT nullable - Current deployment state, CHECK constraint: `('pending', 'deploying', 'ready', 'degraded', 'failed', 'disabled')`
  - `deployment_environment`: TEXT nullable - Environment name (`development`, `staging`, `production`)
  - `deployment_namespace`: TEXT nullable - Kubernetes namespace where model is deployed
  - `last_health_check_at`: TIMESTAMPTZ nullable - Timestamp of last successful health check
- **existing fields** (from base schema):
  - `model_id`: UUID (primary key)
  - `organization_id`: UUID nullable (null => globally available)
  - `model_name`: TEXT
  - `revision`: INTEGER
  - `deployment_target`: TEXT CHECK (`'managed'`, `'self_hosted'`)
  - `cost_per_1k_tokens`: NUMERIC(10,4)
  - `metadata`: JSONB
  - `created_at`: TIMESTAMPTZ
  - `updated_at`: TIMESTAMPTZ
- **relationships**:
  - Optional ownership by **Organization** (via `organization_id`)
  - Referenced by **UsageEvent** (`model_id`) for analytics
  - Referenced by API Router Service for routing decisions
- **rules**:
  - Unique constraint on `(organization_id, model_name, revision)` (existing)
  - `deployment_status` must be set when `deployment_endpoint` is set
  - `deployment_environment` must match one of: `development`, `staging`, `production`
  - `deployment_namespace` must be valid Kubernetes namespace name
  - When `deployment_status = 'disabled'`, API Router should not route to this model
  - `last_health_check_at` updated by health check service or management API

### ModelDeployment (Logical Entity)

- **description**: Represents a deployed vLLM instance in Kubernetes. Not a database table, but a logical entity tracked via Helm releases and Kubernetes resources.
- **fields**:
  - `release_name`: TEXT - Helm release name (e.g., `llama-7b-production`)
  - `model_name`: TEXT - Logical model name (matches `model_registry_entries.model_name`)
  - `revision`: INTEGER - Model revision number
  - `environment`: TEXT - Deployment environment (`development`, `staging`, `production`)
  - `namespace`: TEXT - Kubernetes namespace
  - `service_endpoint`: TEXT - Kubernetes service DNS name
  - `pod_name`: TEXT - Current pod name (may change on restart)
  - `status`: TEXT - Deployment status (`pending`, `deploying`, `ready`, `degraded`, `failed`)
  - `helm_revision`: INTEGER - Helm release revision number
  - `deployed_at`: TIMESTAMPTZ - When deployment was created
  - `last_updated_at`: TIMESTAMPTZ - When deployment was last updated
- **relationships**:
  - Maps to **ModelRegistryEntry** via `(model_name, revision, environment)`
  - Managed by Helm (release history)
  - Tracked in Kubernetes (Deployment, Service, ConfigMap resources)
- **rules**:
  - One deployment per `(model_name, revision, environment)` combination
  - Status transitions: `pending` → `deploying` → `ready` | `failed`
  - Rollback updates `helm_revision` but preserves `model_name` and `revision`
  - Service endpoint format: `{model-name}-{environment}.{namespace}.svc.cluster.local:8000`

### HealthCheck (Logical Entity)

- **description**: Represents health check results for a deployed model. Not a database table, but tracked via `ModelRegistryEntry.last_health_check_at` and Kubernetes probe status.
- **fields**:
  - `model_id`: UUID - Reference to ModelRegistryEntry
  - `endpoint`: TEXT - Health check endpoint URL
  - `status`: TEXT - Health status (`healthy`, `degraded`, `unhealthy`)
  - `checked_at`: TIMESTAMPTZ - When health check was performed
  - `response_time_ms`: INTEGER nullable - Health check response time
  - `error_message`: TEXT nullable - Error message if check failed
- **relationships**:
  - References **ModelRegistryEntry** via `model_id`
  - Used by API Router for routing decisions
- **rules**:
  - Health checks performed every 30 seconds (configurable)
  - `status = 'unhealthy'` triggers alert and may disable routing
  - `last_health_check_at` updated on successful check
  - Failed checks don't update `last_health_check_at` (preserves last known good time)

## State Transitions

### ModelRegistryEntry.deployment_status

```
[pending] → [deploying] → [ready] → [degraded] → [ready]
                              ↓
                          [failed]
                              ↓
                          [disabled] (manual)
```

**Transitions**:
- `pending` → `deploying`: Helm release created, pod scheduling
- `deploying` → `ready`: Pod running, readiness probe passing, model loaded
- `deploying` → `failed`: Pod failed to start, model load error, timeout
- `ready` → `degraded`: Health check failing but pod still running
- `degraded` → `ready`: Health check recovered
- `ready` → `disabled`: Manual disable (operator action)
- `disabled` → `ready`: Manual enable (operator action)
- `failed` → `deploying`: Retry deployment

## Validation Rules

### ModelRegistryEntry

1. **Endpoint Format Validation**:
   - Must match pattern: `{name}-{env}.{namespace}.svc.cluster.local:{port}`
   - Port must be 8000 (vLLM default)
   - Namespace must be valid Kubernetes namespace name

2. **Status Consistency**:
   - If `deployment_endpoint` is set, `deployment_status` must be set
   - If `deployment_status = 'ready'`, `deployment_endpoint` must be set
   - If `deployment_status = 'disabled'`, routing should be disabled regardless of endpoint

3. **Environment Validation**:
   - `deployment_environment` must be one of: `development`, `staging`, `production`
   - Must match Helm release environment label

4. **Namespace Validation**:
   - `deployment_namespace` must be valid Kubernetes namespace
   - Must exist in cluster before deployment

### ModelDeployment (Logical)

1. **Uniqueness**:
   - Only one active deployment per `(model_name, revision, environment)`
   - Previous deployments can exist in `failed` or rolled-back state

2. **Resource Constraints**:
   - GPU requests must not exceed node pool capacity
   - Memory limits must account for model size + KV cache

3. **Naming**:
   - Release name: `{model-name}-{environment}`
   - Service name: `{model-name}-{environment}`
   - Must be valid Kubernetes resource names (lowercase, alphanumeric, hyphens)

## Database Schema Changes

### Migration: Add Deployment Metadata to model_registry_entries

```sql
-- Migration: 20250127120000_add_deployment_metadata.up.sql

BEGIN;

-- Add deployment endpoint URL
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_endpoint TEXT;

-- Add deployment status
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_status TEXT 
CHECK (deployment_status IN ('pending', 'deploying', 'ready', 'degraded', 'failed', 'disabled'));

-- Add deployment environment
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_environment TEXT 
CHECK (deployment_environment IN ('development', 'staging', 'production'));

-- Add deployment namespace
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_namespace TEXT;

-- Add last health check timestamp
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS last_health_check_at TIMESTAMPTZ;

-- Create index for API Router queries
CREATE INDEX IF NOT EXISTS idx_model_registry_deployment_status 
ON model_registry_entries(deployment_status, model_name) 
WHERE deployment_status = 'ready';

-- Create index for environment queries
CREATE INDEX IF NOT EXISTS idx_model_registry_environment 
ON model_registry_entries(deployment_environment, deployment_status);

COMMIT;
```

### Down Migration

```sql
-- Migration: 20250127120000_add_deployment_metadata.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_model_registry_environment;
DROP INDEX IF EXISTS idx_model_registry_deployment_status;

ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS last_health_check_at;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_namespace;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_environment;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_status;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_endpoint;

COMMIT;
```

## API Router Integration

### Query Pattern

API Router queries model registry for routing decisions:

```sql
-- Get available models for routing
SELECT 
  model_id,
  model_name,
  revision,
  deployment_endpoint,
  deployment_status,
  cost_per_1k_tokens
FROM model_registry_entries
WHERE deployment_status = 'ready'
  AND model_name = $1  -- Requested model name
  AND (organization_id = $2 OR organization_id IS NULL)  -- Org-scoped or global
ORDER BY revision DESC  -- Latest revision first
LIMIT 1;
```

### Caching Strategy

- Cache model registry queries in Redis (TTL: 2 minutes)
- Invalidate cache on `deployment_status` changes
- Fallback to database query on cache miss

## Relationships Summary

```
Organization (1) ──< (N) ModelRegistryEntry
                         │
                         │ (references)
                         │
                         ▼
                    UsageEvent (analytics)
                         │
                         │ (routing)
                         ▼
                    API Router Service
                         │
                         │ (HTTP request)
                         ▼
                    vLLM Deployment (Kubernetes)
```

## Data Flow

1. **Deployment**: Operator deploys model → Helm creates release → Updates `ModelRegistryEntry` with endpoint and status
2. **Health Check**: Health check service queries vLLM endpoint → Updates `last_health_check_at` and `deployment_status`
3. **Routing**: Client request → API Router queries `ModelRegistryEntry` → Routes to `deployment_endpoint`
4. **Registration**: Operator enables/disables model → Updates `deployment_status` → API Router cache invalidated

