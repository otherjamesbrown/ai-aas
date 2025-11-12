-- Initial analytics schema: usage_events, ingestion_batches, freshness_status
BEGIN;

-- Enable TimescaleDB extension
CREATE EXTENSION IF NOT EXISTS timescaledb;

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
    PRIMARY KEY (event_id, org_id)
);

-- Convert to TimescaleDB hypertable partitioned by occurred_at
SELECT create_hypertable('analytics.usage_events', 'occurred_at',
    chunk_time_interval => INTERVAL '1 day',
    if_not_exists => TRUE
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

COMMIT;

