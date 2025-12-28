-- +goose Up
-- Add unique index on event_id for ON CONFLICT deduplication
-- This allows ON CONFLICT (event_id) while preserving the composite primary key
-- (event_id, occurred_at) required for TimescaleDB hypertable partitioning
BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_events_event_id_unique
    ON analytics.usage_events (event_id);

COMMIT;

-- +goose Down
BEGIN;

DROP INDEX IF EXISTS analytics.idx_usage_events_event_id_unique;

COMMIT;
