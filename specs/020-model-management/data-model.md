# Data Model: Model Management

**Feature**: 020-model-management
**Date**: 2025-11-28

## Entity Relationship Diagram

```
┌─────────────────────┐       ┌─────────────────────┐
│   model_registry    │       │    model_aliases    │
├─────────────────────┤       ├─────────────────────┤
│ id (PK)             │◄──────│ model_id (FK)       │
│ name (UNIQUE)       │       │ alias_name (UNIQUE) │
│ hf_model_id         │       │ description         │
│ hf_revision         │       └─────────────────────┘
│ requires_auth       │
│ is_gated            │       ┌─────────────────────┐
│ license_type        │       │    model_cache      │
│ license_url         │       ├─────────────────────┤
│ license_accepted_at │◄──────│ model_id (FK)       │
│ license_accepted_by │       │ id (PK)             │
│ recommended_gpu_gb  │       │ version             │
│ recommended_cpu_gb  │       │ hf_revision         │
│ pinned_version      │       │ object_storage_path │
│ metadata (JSONB)    │       │ size_bytes          │
│ created_at          │       │ file_count          │
│ updated_at          │       │ checksum_sha256     │
└─────────────────────┘       │ status              │
         │                    │ cached_at           │
         │                    │ verified_at         │
         │                    └─────────────────────┘
         │                             │
         ▼                             ▼
┌─────────────────────────────────────────────────────┐
│                  model_deployments                   │
├─────────────────────────────────────────────────────┤
│ id (PK)                                              │
│ model_id (FK) ──────────────────────────────────────┤
│ cache_id (FK) ──────────────────────────────────────┤
│ environment (UNIQUE with model_id)                   │
│ namespace                                            │
│ inferenceservice_name                                │
│ endpoint                                             │
│ enabled                                              │
│ status                                               │
│ replicas_desired / replicas_ready                    │
│ gpu_count / memory_gb                                │
│ last_health_check_at / last_health_status           │
│ last_enabled_at / last_enabled_by                   │
│ last_disabled_at / last_disabled_by                 │
│ created_at / updated_at                              │
└─────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────┐
│              model_state_history                     │
├─────────────────────────────────────────────────────┤
│ id (PK)                                              │
│ deployment_id (FK)                                   │
│ action (enabled/disabled/swapped_out/swapped_in)    │
│ performed_by                                         │
│ reason                                               │
│ scheduled_at                                         │
│ executed_at                                          │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│              model_validations                       │
├─────────────────────────────────────────────────────┤
│ id (PK)                                              │
│ model_id (FK)                                        │
│ environment                                          │
│ validation_type (registry/cache/deployment/...)     │
│ check_name                                           │
│ status (pass/warn/fail/skip)                        │
│ message                                              │
│ remediation                                          │
│ validated_at                                         │
└─────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────┐
│             platform_credentials                     │
├─────────────────────────────────────────────────────┤
│ id (PK)                                              │
│ credential_type (UNIQUE: hf-token, s3-access, etc.) │
│ encrypted_value                                      │
│ metadata (JSONB)                                     │
│ created_at / updated_at                              │
└─────────────────────────────────────────────────────┘
```

## Entities

### 1. ModelRegistry

**Purpose**: Central registry of all known models with HuggingFace metadata.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| name | VARCHAR(255) | UNIQUE, NOT NULL | Internal model name (e.g., "llama-3-8b") |
| hf_model_id | VARCHAR(512) | NOT NULL | HuggingFace repo ID (e.g., "meta-llama/Llama-3-8B-Instruct") |
| hf_revision | VARCHAR(255) | DEFAULT 'main' | Branch/tag/commit to track |
| requires_auth | BOOLEAN | DEFAULT false | Whether model needs HF token |
| is_gated | BOOLEAN | DEFAULT false | Whether model requires license acceptance |
| license_type | VARCHAR(100) | NULL | License identifier (e.g., "llama3", "apache-2.0") |
| license_url | VARCHAR(512) | NULL | URL to accept license on HuggingFace |
| license_accepted_at | TIMESTAMP | NULL | When license was acknowledged |
| license_accepted_by | VARCHAR(255) | NULL | Who acknowledged (audit) |
| recommended_gpu_memory_gb | INT | NULL | Minimum GPU memory recommendation |
| recommended_cpu_memory_gb | INT | NULL | Minimum system memory recommendation |
| pinned_version | VARCHAR(255) | NULL | If set, skip update checks |
| metadata | JSONB | DEFAULT '{}' | Additional metadata |
| created_at | TIMESTAMP | DEFAULT NOW() | Record creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**State Transitions**: None (static configuration)

**Validation Rules**:
- `name` must be lowercase alphanumeric with hyphens
- `hf_model_id` must match pattern `{org}/{repo}` or `{repo}`
- If `is_gated=true`, `license_accepted_at` must be set before pull

### 2. ModelAlias

**Purpose**: Alternative names for models enabling version abstraction.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| alias_name | VARCHAR(255) | UNIQUE, NOT NULL | Alias (e.g., "llama-latest") |
| model_id | UUID | FK → model_registry, ON DELETE RESTRICT | Target model |
| description | VARCHAR(512) | NULL | Purpose of alias |
| created_at | TIMESTAMP | DEFAULT NOW() | Record creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**Validation Rules**:
- `alias_name` must be lowercase alphanumeric with hyphens
- Cannot create alias with same name as existing model

### 3. ModelCache

**Purpose**: Tracks cached model versions in object storage.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| model_id | UUID | FK → model_registry, ON DELETE CASCADE | Parent model |
| version | VARCHAR(255) | NOT NULL | HF commit hash |
| hf_revision | VARCHAR(255) | NULL | Branch/tag that was pulled |
| object_storage_path | VARCHAR(512) | NOT NULL | S3 path (e.g., "s3://models/llama-3-8b/v1.0/") |
| size_bytes | BIGINT | NULL | Total size of cached files |
| file_count | INT | NULL | Number of files |
| checksum_sha256 | VARCHAR(64) | NULL | Manifest checksum |
| status | VARCHAR(50) | DEFAULT 'downloading' | Cache status |
| cached_at | TIMESTAMP | DEFAULT NOW() | When caching started |
| verified_at | TIMESTAMP | NULL | Last verification time |

**Unique Constraint**: (model_id, version)

**State Transitions**:
```
downloading → ready     (successful cache)
downloading → failed    (cache failed)
ready → deleted         (manual deletion)
failed → downloading    (retry)
```

**Validation Rules**:
- `object_storage_path` must be valid S3 URI
- `status` must be one of: downloading, ready, failed, deleted

### 4. ModelDeployment

**Purpose**: Tracks deployed model instances per environment.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| model_id | UUID | FK → model_registry, ON DELETE CASCADE | Model being deployed |
| cache_id | UUID | FK → model_cache, NULL | Cache version being used |
| environment | VARCHAR(50) | NOT NULL | Environment (development/staging/production) |
| namespace | VARCHAR(100) | NOT NULL | Kubernetes namespace |
| inferenceservice_name | VARCHAR(255) | NULL | KServe resource name |
| endpoint | VARCHAR(512) | NULL | Service endpoint URL |
| enabled | BOOLEAN | DEFAULT true | Whether deployment is active |
| status | VARCHAR(50) | DEFAULT 'pending' | Deployment status |
| replicas_desired | INT | DEFAULT 1 | Target replica count |
| replicas_ready | INT | DEFAULT 0 | Current ready replicas |
| gpu_count | INT | DEFAULT 1 | GPUs per replica |
| memory_gb | INT | NULL | Memory per replica |
| last_health_check_at | TIMESTAMP | NULL | Last health check time |
| last_health_status | VARCHAR(50) | NULL | Last health result |
| last_enabled_at | TIMESTAMP | NULL | When last enabled |
| last_enabled_by | VARCHAR(255) | NULL | Who enabled |
| last_disabled_at | TIMESTAMP | NULL | When last disabled |
| last_disabled_by | VARCHAR(255) | NULL | Who disabled |
| created_at | TIMESTAMP | DEFAULT NOW() | Record creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**Unique Constraint**: (model_id, environment)

**State Transitions**:
```
pending → deploying     (deploy initiated)
deploying → ready       (pod healthy)
deploying → failed      (deploy failed)
ready → disabled        (model disabled)
disabled → deploying    (model enabled)
ready → terminated      (undeploy)
failed → deploying      (retry)
```

**Validation Rules**:
- `environment` must be one of: development, staging, production
- `status` must be one of: pending, deploying, ready, failed, disabled, terminated
- Cannot deploy without a ready cache_id

### 5. ModelStateHistory

**Purpose**: Audit trail for enable/disable operations.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| deployment_id | UUID | FK → model_deployments, ON DELETE CASCADE | Related deployment |
| action | VARCHAR(20) | NOT NULL | Action type |
| performed_by | VARCHAR(255) | NULL | User who performed action |
| reason | TEXT | NULL | Optional reason for change |
| scheduled_at | TIMESTAMP | NULL | If scheduled for future |
| executed_at | TIMESTAMP | DEFAULT NOW() | When action executed |

**Validation Rules**:
- `action` must be one of: enabled, disabled, swapped_out, swapped_in

### 6. ModelValidation

**Purpose**: Records validation check results.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| model_id | UUID | FK → model_registry, ON DELETE CASCADE | Model validated |
| environment | VARCHAR(50) | NULL | Environment validated |
| validation_type | VARCHAR(100) | NULL | Layer (registry/cache/deployment/endpoint/router) |
| check_name | VARCHAR(100) | NULL | Specific check name |
| status | VARCHAR(20) | NULL | Result status |
| message | TEXT | NULL | Status message |
| remediation | TEXT | NULL | Fix suggestion |
| validated_at | TIMESTAMP | DEFAULT NOW() | When validated |

**Validation Rules**:
- `status` must be one of: pass, warn, fail, skip

### 7. PlatformCredentials

**Purpose**: Encrypted storage for platform credentials.

| Field | Type | Constraints | Description |
|-------|------|-------------|-------------|
| id | UUID | PK, auto-gen | Unique identifier |
| credential_type | VARCHAR(100) | UNIQUE, NOT NULL | Type (hf-token, s3-access, etc.) |
| encrypted_value | TEXT | NOT NULL | AES-256 encrypted value |
| metadata | JSONB | DEFAULT '{}' | Additional metadata (e.g., endpoint URL) |
| created_at | TIMESTAMP | DEFAULT NOW() | Record creation time |
| updated_at | TIMESTAMP | DEFAULT NOW() | Last update time |

**Validation Rules**:
- `credential_type` must be one of: hf-token, s3-access, s3-secret, s3-endpoint, s3-bucket

## Indexes

```sql
-- Performance indexes
CREATE INDEX idx_model_cache_model_id ON model_cache(model_id);
CREATE INDEX idx_model_cache_status ON model_cache(status);
CREATE INDEX idx_model_deployments_model_id ON model_deployments(model_id);
CREATE INDEX idx_model_deployments_environment ON model_deployments(environment);
CREATE INDEX idx_model_deployments_status ON model_deployments(status);
CREATE INDEX idx_model_state_history_deployment_id ON model_state_history(deployment_id);
CREATE INDEX idx_model_validations_model_id ON model_validations(model_id);
CREATE INDEX idx_model_aliases_model_id ON model_aliases(model_id);
```

## Migration Order

1. `create_model_registry` - Base table, no dependencies
2. `create_model_aliases` - Depends on model_registry
3. `create_model_cache` - Depends on model_registry
4. `create_model_deployments` - Depends on model_registry, model_cache
5. `create_model_state_history` - Depends on model_deployments
6. `create_model_validations` - Depends on model_registry
7. `create_platform_credentials` - No dependencies

