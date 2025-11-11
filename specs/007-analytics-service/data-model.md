# Data Model: Analytics Service

**Date**: 2025-11-11  
**Source**: Derived from `specs/007-analytics-service/spec.md` and Phase 0 research.

---

## Entity Overview

| Entity | Purpose | Key Relationships |
|--------|---------|-------------------|
| `usage_events` | Raw ingestion of inference usage/error events | References `orgs`, `models`; linked to `ingestion_batches` |
| `ingestion_batches` | Tracks consumer offsets, dedupe status, and replay windows | One-to-many with `usage_events` |
| `hourly_rollups` | Materialized aggregates per org/model/hour | Derived from `usage_events` via continuous aggregates |
| `daily_rollups` | Coarser aggregates per org/model/day | Derived from `hourly_rollups` with retention policy |
| `freshness_status` | Stores latest ingestion and aggregation timestamps per org/model | Updated by aggregation worker; queried by UI |
| `export_jobs` | Records CSV export jobs, statuses, delivery URIs, and audit metadata | References `orgs`; links to generated artifacts |
| `orgs` | Tenant metadata (from user-org service) mirrored for analytics scopes | Read-only foreign key reference |
| `models` | Model metadata (from model registry) mirrored for analytics scopes | Read-only foreign key reference |

---

## Table Specifications

### `usage_events`
- **Primary Key**: (`event_id`, `org_id`)
- **Columns**:
  - `event_id` (UUID) — dedupe key from producer
  - `org_id` (UUID) — tenant scope
  - `actor_id` (UUID, nullable) — future per-user drilldowns
  - `model_id` (UUID)
  - `occurred_at` (TIMESTAMPTZ) — event timestamp
  - `received_at` (TIMESTAMPTZ DEFAULT now()) — ingestion timestamp
  - `input_tokens`, `output_tokens` (BIGINT)
  - `latency_ms` (INTEGER)
  - `status` (TEXT ENUM: `success`, `error`, `timeout`, `throttled`)
  - `error_code` (TEXT, nullable)
  - `cost_estimate_cents` (NUMERIC(18,4))
  - `metadata` (JSONB) — request traits (client id, region, tags)
  - `batch_id` (UUID) — FK → `ingestion_batches.batch_id`
- **Indexes**:
  - Hypertable partitioned by `occurred_at`, clustered by `org_id`, `model_id`
  - Index on (`org_id`, `model_id`, `occurred_at DESC`)
  - Partial index on (`status`) WHERE `status` != 'success' for error queries

### `ingestion_batches`
- **Primary Key**: `batch_id` (UUID)
- **Columns**:
  - `batch_id`
  - `stream_offset` (BIGINT) — RabbitMQ offset
  - `org_scope` (UUID ARRAY) — orgs included in batch
  - `started_at`, `completed_at` (TIMESTAMPTZ)
  - `late_arrival` (BOOLEAN) — signals replay refresh
  - `dedupe_conflicts` (INTEGER) — count of duplicates removed
  - `retry_count` (INTEGER)
- **Indexes**:
  - Btree on `completed_at`
  - Btree on (`late_arrival`, `completed_at`)

### `hourly_rollups`
- **Primary Key**: (`org_id`, `model_id`, `bucket_start`)
- **Columns**:
  - `bucket_start` (TIMESTAMPTZ) — truncated to hour
  - `org_id`, `model_id`
  - `actor_id` (UUID, nullable) — maintained when available
  - `invocations` (BIGINT)
  - `errors` (BIGINT)
  - `latency_p50_ms`, `latency_p95_ms`, `latency_p99_ms` (INTEGER)
  - `cost_estimate_cents` (NUMERIC(18,4))
  - `input_tokens`, `output_tokens` (BIGINT)
  - `fresh_as_of` (TIMESTAMPTZ) — last refresh timestamp
- **Indexes**:
  - Btree on (`org_id`, `bucket_start DESC`)
  - Covering index on (`model_id`, `bucket_start DESC`)
- **Policies**:
  - Continuous aggregate refresh every 5 minutes with 1-hour lookback
  - Retain exact data for 90 days, compress after 30 days

### `daily_rollups`
- Same columns as `hourly_rollups` except `bucket_start` truncated to day.
- Retain 13 months, compress after 90 days.
- Derived from `hourly_rollups` to reduce compute.

### `freshness_status`
- **Primary Key**: (`org_id`, `model_id`)
- **Columns**:
  - `org_id`, `model_id`
  - `last_event_at` (TIMESTAMPTZ)
  - `last_rollup_at` (TIMESTAMPTZ)
  - `lag_seconds` (INTEGER)
  - `status` (ENUM: `fresh`, `stale`, `delayed`)
  - `updated_at` (TIMESTAMPTZ DEFAULT now())
- **Indexes**:
  - Btree on (`status`, `updated_at DESC`)
- **Usage**: Cached in Redis with 60s TTL for UI dashboards; persisted here for audit trail.

### `export_jobs`
- **Primary Key**: `job_id` (UUID)
- **Columns**:
  - `job_id`
  - `org_id`
  - `requested_by` (UUID)
  - `time_range_start`, `time_range_end` (TIMESTAMPTZ)
  - `granularity` (ENUM: `hourly`, `daily`, `monthly`)
  - `status` (ENUM: `pending`, `running`, `succeeded`, `failed`, `expired`)
  - `output_uri` (TEXT) — signed S3 URL (nullable until ready)
  - `checksum` (TEXT) — SHA256 of CSV
  - `row_count` (BIGINT)
  - `initiated_at`, `completed_at` (TIMESTAMPTZ)
  - `error_message` (TEXT, nullable)
- **Indexes**:
  - Btree on (`org_id`, `initiated_at DESC`)
  - Partial index on (`status`) WHERE `status` IN ('pending', 'running')

---

## Relationships & Integrity

- `usage_events.org_id` and `usage_events.model_id` reference authoritative records via foreign keys to mirrored `orgs` and `models` tables kept in sync by the API Router service. On deletion events, rows are retained but marked with `metadata->>'org_status'`.
- `hourly_rollups` and `daily_rollups` are materialized views; refresh policies ensure `freshness_status` update transactions happen atomically via advisory locks.
- `export_jobs` lines link to S3 objects named `analytics/exports/{org_id}/{job_id}.csv`; cleanup job deletes S3 objects when status transitions to `expired`.
- Data retention:
  - Raw `usage_events` compressed after 7 days, dropped after 400 days (13 months + buffer).
  - Rollups keep 13 months of data; monthly archives stored as exports for finance continuity.
- Auditing:
  - Triggers on `usage_events` insert update `freshness_status.last_event_at`.
  - Triggers on rollup refresh update `freshness_status.last_rollup_at` and `lag_seconds`.

---

## Validation Rules

- Reject duplicate `(event_id, org_id)` pairs; duplicates increment `dedupe_conflicts` in the associated batch.
- Enforce positive `input_tokens`/`output_tokens`; allow zero when latency/error occurs before tokens produced.
- `time_range_start`/`end` validated in ingestion API to prevent >31-day exports.
- Signed S3 URLs expire after 24 hours; `status` transitions to `expired` automatically via scheduler.

---

## Access Patterns

- Primary queries: `hourly_rollups` filtered by `org_id`, `model_id`, `bucket_start BETWEEN <start> AND <end>`, aggregated to chart data (ordered descending).
- Reliability views: error rate derived from `errors / invocations`, latency percentiles projected from `latency_pXX_ms`.
- Freshness UI: `freshness_status` feed plus cached Redis entry.
- Finance export flow:
  1. Create `export_jobs` row (status `pending`).
  2. Worker reads job, executes SQL into temp table, writes CSV to S3, updates checksum + status.
  3. Client polls `GET /analytics/v1/orgs/{orgId}/exports/{jobId}` to retrieve `output_uri`.

---

## Future Considerations

- Optional `actor_id` indexes enable per-user drilldowns without backfill once upstream provides data.
- If real-time dashboards require <1 minute latency, consider streaming aggregates to Redis streams; documented as backlog item.
- Evaluate automatic anomaly detection for error spikes by adding `anomalies` table and hooking into observability stack.

