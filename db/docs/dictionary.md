# Data Dictionary: Database Schemas & Migrations

_This document is generated and maintained alongside migrations. Populate entity and field details during Phase 3._

## Entities Overview

| Entity | Description | Primary Keys | Notes |
|--------|-------------|--------------|-------|
| Organization | Tenant context and billing metadata | `organization_id` | Multi-tenant isolation, quotas |
| User | Individual account within an organization | `user_id` | Role-based access controls |
| APIKey | Credential bound to organization | `api_key_id` | Secrets stored hashed |
| ModelRegistryEntry | Catalog of available model revisions | `model_id` | Supports org-specific overrides |
| UsageEvent | Fact table for per-request metrics | `event_id` | Partitioned by `occurred_at` |
| AuditLogEntry | Immutable change log | `audit_id` | Records schema + privileged actions |

## Field Definitions

### Organization
- `organization_id` (UUID) — Primary key; defaults to `gen_random_uuid()`.
- `slug` (text) — Immutable, unique per organization.
- `display_name` (text) — Human-readable name.
- `plan_tier` (enum) — `starter|growth|enterprise`.
- `budget_limit_tokens` (bigint) — Consumption guardrail (>= 0).
- `status` (enum) — `active|suspended|closed`; controls tenant access.
- `created_at`, `updated_at`, `soft_deleted_at` (timestamptz) — Lifecycle tracking (soft delete retains historical data).

### User
- `user_id` (UUID) — Primary key.
- `organization_id` (UUID) — FK → Organization.
- `email` (text) — Lowercased, stored hashed/encrypted; original value never persisted in plaintext.
- `email_hash` (text) — Deterministic hash used for uniqueness constraint.
- `role` (enum) — `owner|admin|developer|billing`.
- `status` (enum) — `active|invited|disabled`.
- `last_login_at` (timestamptz) — Nullable.
- `created_at`, `updated_at`, `soft_deleted_at` (timestamptz) — Lifecycle tracking.

### APIKey
- `api_key_id` (UUID) — Primary key.
- `organization_id` (UUID) — FK → Organization.
- `name` (text) — Human label.
- `hashed_secret` (bytea) — Bcrypt/Argon2 hashed secret (never plaintext).
- `scopes` (text[]) — Access scopes.
- `status` (enum) — `active|rotating|revoked`.
- `last_used_at` (timestamptz) — Nullable.
- `created_at`, `updated_at` (timestamptz) — Lifecycle tracking.

### ModelRegistryEntry
- `model_id` (UUID) — Primary key.
- `organization_id` (UUID) — Nullable FK → Organization (null = global).
- `model_name` (text) — Logical model name.
- `revision` (int) — Sequential revision number.
- `deployment_target` (enum) — `managed|self_hosted`.
- `cost_per_1k_tokens` (numeric) — Billing reference.
- `metadata` (jsonb) — Flexible metadata block.
- `created_at`, `updated_at` (timestamptz) — Lifecycle tracking.

### UsageEvent
- `event_id` (UUID) — Primary key.
- `occurred_at` (timestamptz) — Partition key.
- `organization_id` (UUID) — FK → Organization.
- `api_key_id` (UUID) — FK → APIKey.
- `model_id` (UUID) — FK → ModelRegistryEntry.
- `tokens_consumed` (bigint) — Usage metric.
- `latency_ms` (int) — Request latency.
- `status` (enum) — `success|rate_limited|error`.
- `error_code` (text) — Nullable.
- `region` (text) — Region identifier.
- `cost_usd` (numeric) — Billed amount.
- `created_at` (timestamptz) — Ingestion timestamp.

### AuditLogEntry
- `audit_id` (UUID) — Primary key.
- `actor_type` (enum) — `user|service|migration`.
- `actor_id` (UUID/text) — Actor reference.
- `action` (text) — Change description.
- `target` (text) — Table or entity affected.
- `metadata` (jsonb) — Row counts, checksums, etc.
- `occurred_at` (timestamptz) — Timestamp.

## Indexes & Constraints

- `users_email_unique` enforces one hashed email per organization.
- `api_keys_unique_name_per_org` prevents duplicate API key names per organization.
- `model_registry_entries_unique_revision` ensures revision uniqueness per model + tenant (or globally).
- `usage_events` BRIN indexes accelerate time-ranged queries by organization and model.
- ENUM values are enforced via `CHECK` constraints to protect data quality.

## Open Questions

- Define final enum values for `deployment_target` and `status` once infrastructure preferences are confirmed.
- Confirm retention periods align with compliance review outcomes.
