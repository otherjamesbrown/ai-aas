-- +goose Up
-- Ensure analytics tables exist (recovery from goose version mismatch)
-- This migration re-creates tables from 20251112001_init.sql if they were skipped
-- NOTE: TimescaleDB removed for compatibility - hypertable can be added later
BEGIN;

-- Create schema if it doesn't exist
CREATE SCHEMA IF NOT EXISTS analytics;

-- usage_events: Raw inference usage events with deduplication
CREATE TABLE IF NOT EXISTS analytics.usage_events (
    event_id UUID NOT NULL,
    org_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    model_id UUID,
    actor_id UUID, -- Future per-user drilldowns
    input_tokens BIGINT NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
    output_tokens BIGINT NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
    latency_ms INTEGER NOT NULL CHECK (latency_ms >= 0),
    status TEXT NOT NULL CHECK (status IN ('success', 'error', 'timeout', 'throttled')),
    error_code TEXT,
    cost_estimate_cents NUMERIC(18,4) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    batch_id UUID,
    -- Composite primary key for deduplication
    PRIMARY KEY (event_id, occurred_at)
);

-- Indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_usage_events_org_model_time
    ON analytics.usage_events (org_id, model_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_events_status
    ON analytics.usage_events (status)
    WHERE status != 'success';
CREATE INDEX IF NOT EXISTS idx_usage_events_batch
    ON analytics.usage_events (batch_id);

-- ingestion_batches: Track consumer offsets and dedupe status
CREATE TABLE IF NOT EXISTS analytics.ingestion_batches (
    batch_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    stream_offset BIGINT NOT NULL,
    org_scope UUID[] NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    late_arrival BOOLEAN NOT NULL DEFAULT FALSE,
    dedupe_conflicts INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_ingestion_batches_completed
    ON analytics.ingestion_batches (completed_at);
CREATE INDEX IF NOT EXISTS idx_ingestion_batches_late
    ON analytics.ingestion_batches (late_arrival, completed_at);

-- freshness_status: Track latest ingestion and aggregation timestamps
CREATE TABLE IF NOT EXISTS analytics.freshness_status (
    org_id UUID NOT NULL,
    model_id UUID,
    last_event_at TIMESTAMPTZ,
    last_rollup_at TIMESTAMPTZ,
    lag_seconds INTEGER,
    status TEXT CHECK (status IN ('fresh', 'stale', 'delayed')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (org_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_freshness_status_status
    ON analytics.freshness_status (status, updated_at DESC);

-- Create ENUM types for export job status and granularity (if they don't exist)
DO $$ BEGIN
    CREATE TYPE analytics.export_job_status AS ENUM ('pending', 'running', 'succeeded', 'failed', 'expired');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE analytics.export_job_granularity AS ENUM ('hourly', 'daily', 'monthly');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- export_jobs: Track export job lifecycle for CSV generation and S3 delivery
CREATE TABLE IF NOT EXISTS analytics.export_jobs (
    job_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL,
    requested_by UUID NOT NULL,
    time_range_start TIMESTAMPTZ NOT NULL,
    time_range_end TIMESTAMPTZ NOT NULL,
    granularity analytics.export_job_granularity NOT NULL DEFAULT 'daily',
    status analytics.export_job_status NOT NULL DEFAULT 'pending',
    output_uri TEXT,
    checksum TEXT,
    row_count BIGINT,
    initiated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    error_message TEXT,
    CONSTRAINT valid_time_range CHECK (time_range_end > time_range_start)
);

-- Indexes for export job query patterns
CREATE INDEX IF NOT EXISTS idx_export_jobs_org_initiated
    ON analytics.export_jobs (org_id, initiated_at DESC);
CREATE INDEX IF NOT EXISTS idx_export_jobs_status_pending_running
    ON analytics.export_jobs (status)
    WHERE status IN ('pending', 'running');

COMMIT;

-- +goose Down
-- This is a recovery migration - down migration is a no-op
-- since we don't want to drop tables that might have data
BEGIN;
-- No-op: Tables were created with IF NOT EXISTS, so we don't drop them
COMMIT;
