# Data Model: Database Schemas & Migrations

**Branch**: `003-database-schemas-clarifications`  
**Date**: 2025-11-10  
**Spec**: `/specs/003-database-schemas/spec.md`

## Entities

### Organization
- **description**: Tenant-level record driving isolation, billing, and governance.
- **fields**:
  - `organization_id`: UUID (primary key)
  - `slug`: string (unique, kebab-case)
  - `display_name`: string
  - `status`: enum (`active`, `suspended`, `closed`)
  - `plan_tier`: enum (`starter`, `growth`, `enterprise`)
  - `budget_limit_tokens`: bigint
  - `created_at` / `updated_at`: timestamptz
  - `soft_deleted_at`: timestamptz nullable
- **relationships**:
  - One-to-many with **User**, **APIKey**, **UsageEvent**, **ModelRegistryEntry**
  - Referenced by **AnalyticsRollup** (aggregated by organization)
- **rules**:
  - `slug` immutable post-creation.
  - Soft delete retains record but disallows new API keys; cleanup jobs must purge PII within 30 days.

### User
- **description**: Individual identity within an organization with role-based access.
- **fields**:
  - `user_id`: UUID (primary key)
  - `organization_id`: UUID (FK → Organization)
  - `email`: string (unique per org, lowercased, encrypted at rest)
  - `role`: enum (`owner`, `admin`, `developer`, `billing`)
  - `last_login_at`: timestamptz nullable
  - `status`: enum (`active`, `invited`, `disabled`)
  - `created_at` / `updated_at`: timestamptz
  - `soft_deleted_at`: timestamptz nullable
- **relationships**:
  - Belongs to **Organization**
  - Activities referenced in **AuditLogEntry**
- **rules**:
  - `email` stored hashed+encrypted; unique constraint uses deterministic hash.
  - Disable cascades on delete; rely on soft delete to preserve audit trail.

### APIKey
- **description**: Credential granting API access scoped to organization.
- **fields**:
  - `api_key_id`: UUID (primary key)
  - `organization_id`: UUID (FK → Organization)
  - `name`: string
  - `hashed_secret`: bytea (bcrypt/argon2 hashed)
  - `scopes`: string array
  - `status`: enum (`active`, `rotating`, `revoked`)
  - `last_used_at`: timestamptz nullable
  - `created_at` / `updated_at`: timestamptz
- **relationships**:
  - Belongs to **Organization**
  - Referenced by **UsageEvent** (`api_key_id`)
- **rules**:
  - Enforce unique `(organization_id, name)` combination.
  - Rotation workflow must create new record before revoking old key (dual-write guidance).

### ModelRegistryEntry
- **description**: Defines AI model versions available to organizations.
- **fields**:
  - `model_id`: UUID (primary key)
  - `organization_id`: UUID nullable (null => globally available)
  - `model_name`: string
  - `revision`: integer
  - `deployment_target`: enum (`managed`, `self_hosted`)
  - `cost_per_1k_tokens`: numeric(10,4)
  - `metadata`: jsonb
  - `created_at` / `updated_at`: timestamptz
- **relationships**:
  - Optional ownership by **Organization**
  - Referenced by **UsageEvent** (`model_id`)
- **rules**:
  - Unique constraint on `(model_name, revision, coalesce(organization_id, '00000000-0000-0000-0000-000000000000'))`.
  - Retain history; no destructive updates.

### UsageEvent
- **description**: Fact table capturing per-request metrics for analytics.
- **fields**:
  - `event_id`: UUID (primary key)
  - `occurred_at`: timestamptz (partition key)
  - `organization_id`: UUID (FK → Organization)
  - `api_key_id`: UUID (FK → APIKey)
  - `model_id`: UUID (FK → ModelRegistryEntry)
  - `tokens_consumed`: bigint
  - `latency_ms`: integer
  - `status`: enum (`success`, `rate_limited`, `error`)
  - `error_code`: string nullable
  - `region`: string
  - `cost_usd`: numeric(12,4)
- **relationships**:
  - Source for **AnalyticsRollup**
  - Referenced by **AuditLogEntry** for compliance events
- **rules**:
  - Partition by hourly buckets on `occurred_at`.
  - Enforce check constraint `tokens_consumed >= 0`.

### AnalyticsRollup
- **description**: Materialized aggregates summarizing usage events.
- **fields**:
  - `bucket_start`: timestamptz (primary key component)
  - `bucket_granularity`: enum (`hour`, `day`)
  - `organization_id`: UUID
  - `model_id`: UUID nullable
  - `request_count`: bigint
  - `tokens_total`: bigint
  - `error_count`: bigint
  - `cost_total_usd`: numeric(14,4)
  - `last_reconciled_at`: timestamptz
- **relationships**:
  - Derived from **UsageEvent**
  - Surfaced to analytics dashboards / billing services
- **rules**:
  - Unique constraint on `(bucket_start, bucket_granularity, organization_id, coalesce(model_id, '000...'))`.
  - Rollup jobs must reconcile late-arriving events and update `last_reconciled_at`.

### AuditLogEntry
- **description**: Immutable record of privileged schema/data changes and migration runs.
- **fields**:
  - `audit_id`: UUID (primary key)
  - `actor_type`: enum (`user`, `service`, `migration`)
  - `actor_id`: UUID or string identifier
  - `action`: string (`migration_apply`, `schema_change`, `seed_run`, etc.)
  - `target`: string (table or entity name)
  - `metadata`: jsonb (row counts, checksums)
  - `occurred_at`: timestamptz
- **relationships**:
  - Links to **MigrationChangeSet** (`action` metadata includes change set version)
  - References **User** / **Service Account** for human/service actors
- **rules**:
  - Append-only; enforce trigger preventing update/delete.
  - Must include hash of migration script for tamper evidence.

### MigrationChangeSet
- **description**: Versioned definition of schema modifications with pre/post checks.
- **fields**:
  - `version`: string (`YYYYMMDDHHMM_slug`)
  - `direction`: enum (`up`, `down`)
  - `checksum`: string (sha256)
  - `applied_at`: timestamptz nullable
  - `applied_by`: string
  - `status`: enum (`pending`, `applied`, `rolled_back`, `failed`)
- **relationships**:
  - Associated with **AuditLogEntry** entries (application, rollback)
  - Stored in migrations table managed by `golang-migrate`
- **rules**:
  - Version numbers strictly increasing; no gap reuse.
  - Down script required for every up script (even if noop).

### SeedDatasetPackage
- **description**: Deterministic fixtures populating baseline tenants, users, API keys, budgets, and analytics samples.
- **fields**:
  - `package_name`: string (`local-dev`, `ci`, `staging`)
  - `applies_to`: enum (`operational`, `analytics`, `both`)
  - `hash`: string (content checksum)
  - `last_updated_at`: timestamptz
  - `idempotency_key`: string ensuring rerun safety
- **relationships**:
  - Invokes **MigrationChangeSet** prerequisites before seeding.
  - Generates **UsageEvent** samples consumed by **AnalyticsRollup** tests.
- **rules**:
  - Seeds must validate existing data by natural key before insert.
  - CI package resets relevant tables between runs via transactional truncation.

### DataClassificationPolicy
- **description**: Governance mapping for entities/fields to sensitivity levels and retention windows.
- **fields**:
  - `entity`: string
  - `field`: string
  - `classification`: enum (`restricted`, `confidential`, `internal`)
  - `encryption_required`: boolean
  - `retention_days`: integer
  - `purge_strategy`: enum (`anonymize`, `delete`, `archive`)
- **relationships**:
  - Enforced by schema lint tool and migration review checklist.
  - Referenced during migration apply/rollback to ensure compliance tasks executed.
- **rules**:
  - Policy stored in `configs/data-classification.yml`; changes require security review.
  - Enforcement scripts fail CI if migration touches classified field without policy entry.

## Relationships Overview
- **Organization** ← **User**, **APIKey**, **UsageEvent**, **ModelRegistryEntry** (FK relationships)
- **UsageEvent** → **AnalyticsRollup** (derived aggregates)
- **MigrationChangeSet** ↔ **AuditLogEntry** (bidirectional trace for apply/rollback)
- **SeedDatasetPackage** seeds **Organization**, **User**, **APIKey**, **UsageEvent**
- **DataClassificationPolicy** governs encryption/retention across all entities and is validated pre-deploy.

## Validation & Testing Notes
- Containerized Postgres instances execute apply → rollback → reapply for every change set before merge.
- Analytics rollups validated via reconciliation tests comparing aggregates to raw **UsageEvent** slices.
- Seed packages include smoke tests ensuring fixtures allow login and sample usage flow end-to-end.

