-- +goose Up
-- Add unique index on event_id for ON CONFLICT deduplication
-- TimescaleDB requires unique indexes to include the partitioning column (occurred_at)
-- This allows ON CONFLICT (event_id, occurred_at) for deduplication
BEGIN;

CREATE UNIQUE INDEX IF NOT EXISTS idx_usage_events_event_id_unique
    ON analytics.usage_events (event_id, occurred_at);

COMMIT;

-- +goose Down
BEGIN;

DROP INDEX IF EXISTS analytics.idx_usage_events_event_id_unique;

COMMIT;
