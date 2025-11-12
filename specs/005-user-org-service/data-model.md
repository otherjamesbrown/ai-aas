# Data Model: User & Organization Service

**Branch**: `005-user-org-service-upgrade`  
**Date**: 2025-11-11  
**Spec**: `/specs/005-user-org-service/spec.md`

## Overview

The service maintains tenant-scoped identity, access, policy, and declarative state. PostgreSQL holds authoritative relational data, Redis caches session tokens and policy decisions, Kafka streams immutable audit events. All entities include `created_at`, `updated_at`, `version`, and soft-delete markers with retention policies as required.

## Entities

### Organization
- **Table**: `orgs`
- **Fields**: `org_id (uuid, pk)`, `name`, `slug`, `status (active|suspended|pending_delete)`, `billing_owner_user_id`, `budget_policy_id`, `declarative_mode (enabled|disabled|paused)`, `declarative_repo_url`, `declarative_branch`, `declarative_last_commit`, `mfa_required_roles` (jsonb), `metadata` (jsonb), `suspended_at`, `deleted_at`
- **Relationships**: One-to-many with `users`, `service_accounts`, `budget_policies`, `audit_events`; references `budget_policies.policy_id`
- **State Transitions**: `pending -> active -> suspended -> active` (via reinstatement) or `pending_delete -> deleted`
- **Validation**: Unique `slug`; cannot delete if outstanding invoices or active declarative reconciliation job; `declarative_mode=enabled` requires repo metadata

### User
- **Table**: `users`
- **Fields**: `user_id (uuid, pk)`, `org_id`, `email`, `display_name`, `mfa_enrolled` (bool), `mfa_methods` (jsonb), `status (invited|active|suspended|deleted)`, `last_login_at`, `lockout_until`, `recovery_tokens` (hashed), `external_idp_id`, `metadata` (jsonb)
- **Relationships**: Many-to-many with `roles` via `user_roles`; belongs to `orgs`; references `sessions`, `audit_events`
- **State Transitions**: `invited -> active -> suspended -> active` or `deleted`; lockouts temporary via `lockout_until`
- **Validation**: Email unique per org; must enroll MFA before elevated role assignment; suspension requires reason code

### Role
- **Table**: `roles`
- **Fields**: `role_id (uuid, pk)`, `org_id (nullable for global)`, `name`, `description`, `scopes` (jsonb), `system_managed` (bool)
- **Relationships**: Many-to-many with `users` (`user_roles`), `service_accounts` (`service_account_roles`)
- **State Transitions**: Custom roles mutable; system roles read-only
- **Validation**: Custom scope definitions validated against policy schema; cannot delete if assigned

### Service Account
- **Table**: `service_accounts`
- **Fields**: `service_account_id (uuid, pk)`, `org_id`, `name`, `description`, `status (active|disabled|deleted)`, `last_rotation_at`, `metadata` (jsonb)
- **Relationships**: Many-to-many with `roles`; one-to-many with `api_keys`; audit trail of rotations
- **State Transitions**: `active -> disabled -> active` or `deleted`
- **Validation**: Requires at least one role assignment; rotation frequency enforced via policy

### API Key
- **Table**: `api_keys`
- **Fields**: `api_key_id (uuid, pk)`, `org_id`, `principal_type (user|service_account)`, `principal_id`, `fingerprint`, `status (active|revoked|expired|pending_rotation)`, `issued_at`, `revoked_at`, `expires_at`, `last_used_at`, `scopes` (jsonb), `annotations` (jsonb)
- **Relationships**: Belongs to `users` or `service_accounts`; emits to audit and metrics
- **State Transitions**: `active -> revoked` or auto `expired`; `pending_rotation -> active` after success
- **Validation**: Fingerprints unique; secret material never stored; rotation requires success ack

### Session
- **Table**: `sessions`
- **Fields**: `session_id (uuid, pk)`, `org_id`, `user_id`, `refresh_token_hash`, `created_at`, `expires_at`, `ip_address`, `user_agent`, `revoked_at`, `mfa_verified_at`
- **Relationships**: Belongs to `users`; mirrored in Redis for fast revocation checks
- **State Transitions**: `active -> revoked` or `expired`
- **Validation**: Refresh tokens hashed with Argon2id; IP + user-agent captured for anomaly detection

### Budget Policy
- **Table**: `budget_policies`
- **Fields**: `policy_id (uuid, pk)`, `org_id`, `warn_threshold`, `block_threshold`, `currency`, `period (monthly|quarterly|annual)`, `override_required_roles` (array), `notifications` (jsonb), `grace_period_minutes`, `status (active|archived)`
- **Relationships**: Referenced by `orgs`; overrides stored in `budget_overrides`
- **State Transitions**: `draft -> active -> archived`
- **Validation**: Thresholds 0-100; block ≥ warn; overrides require dual-approval roles

### Budget Override
- **Table**: `budget_overrides`
- **Fields**: `override_id (uuid, pk)`, `org_id`, `policy_id`, `requested_by`, `approved_by`, `override_amount`, `status (pending|approved|rejected|expired)`, `expires_at`, `reason`, `attachments` (jsonb)
- **Relationships**: Links to `budget_policies`, `audit_events`
- **State Transitions**: `pending -> approved -> expired` or `pending -> rejected`
- **Validation**: Dual-approval enforced via constraint; cannot exceed policy-defined cap

### Declarative Config Revision
- **Table**: `declarative_revisions`
- **Fields**: `revision_id (uuid, pk)`, `org_id`, `commit_sha`, `source_repo`, `status (pending|synced|failed|drift_detected|paused)`, `applied_at`, `diff_snapshot` (jsonb), `error_details`
- **Relationships**: Linked to `orgs`; reconciliation jobs recorded in `reconciliation_runs`
- **State Transitions**: `pending -> synced`, `pending -> failed`, `synced -> drift_detected -> synced`
- **Validation**: Repository must match org's declarative settings; diff stored for audit

### Reconciliation Run
- **Table**: `reconciliation_runs`
- **Fields**: `run_id (uuid, pk)`, `org_id`, `revision_id`, `status (running|successful|failed|rollback)`, `started_at`, `completed_at`, `attempt`, `errors` (jsonb)
- **Relationships**: Links to `declarative_revisions`; metrics aggregated for dashboard
- **State Transitions**: `running -> successful` or `failed`; `failed` may trigger `rollback`
- **Validation**: Retries capped; failure triggers alert webhook

### Audit Event
- **Table**: `audit_events`
- **Fields**: `event_id (uuid, pk)`, `org_id`, `actor_id`, `actor_type (user|service_account|system)`, `target_id`, `target_type`, `action`, `resource`, `policy_id`, `ip_address`, `user_agent`, `metadata` (jsonb), `hash`, `signature`, `delivered_at`
- **Relationships**: Emitted to Kafka and stored in cold storage; references `orgs`, `users`, `service_accounts`
- **Validation**: Signature verified on write; hash unique; pipeline ensures write-ahead to Kafka before acknowledging request

### Policy Decision Cache
- **Redis Keys**: `policy:<org_id>:<subject_id>:<resource>:<action>`
- **Fields**: JSON containing `decision (allow|deny)`, `policy_id`, `reason`, `expires_at`
- **Behavior**: Cached for ≤5 minutes; invalidated on role/budget changes or explicit purge

## Relationships Summary

- One `org` → many `users`, `service_accounts`, `budget_policies`, `declarative_revisions`  
- `users` ↔ `roles` (many-to-many) via `user_roles`; `service_accounts` ↔ `roles` via `service_account_roles`  
- `api_keys` belong to either `users` or `service_accounts`; cascaded revocation on principal suspension  
- `budget_policies` feed enforcement decisions; overrides override policy temporarily  
- `declarative_revisions` produce `reconciliation_runs`; failures generate `audit_events` and alerts  
- Redis mirrors `sessions` and policy decisions for low-latency checks  
- Kafka topics (`billing.usage`, `identity.reconcile`, `audit.identity`) ensure asynchronous coordination

## Validation & Integrity Rules

- All state-mutating operations wrapped in transactions with optimistic concurrency (`version` column).  
- Soft deletes tracked via `deleted_at`; nightly jobs purge records past retention.  
- Foreign keys enforce tenant isolation (`org_id` must match across joined tables).  
- Row-level security (RLS) on Postgres ensures service accounts access only tenant data, supporting multi-tenant deployments.  
- Budget overrides require two distinct approvers; database constraint enforces uniqueness.  
- Declarative revisions locked to monotonic commit sequence per org to prevent replay attacks.

## Derived Views & Analytics

- Materialized view `authz_denials_summary` aggregates policy denials by resource/action for dashboards.  
- View `budget_consumption_status` joins usage feed snapshots with policy thresholds for real-time indicators.  
- View `mfa_enrollment_gap` identifies users with elevated roles lacking MFA to drive notifications.

## Open Modeling Considerations

- Confirm if cross-org service accounts (global automation) are needed; currently deferred per spec.  
- Define retention schedule for `audit_events` cold storage beyond 400-day requirement (likely offloaded to object store lifecycle).  
- Validate that Kafka topic partitioning strategy aligns with throughput (propose partition by `org_id`).

