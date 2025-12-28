-- +goose Up
-- Fix rollup schema to allow NULL model_id and add actor_id column
BEGIN;

-- Drop existing primary keys that implicitly require model_id to be NOT NULL
ALTER TABLE analytics_hourly_rollups DROP CONSTRAINT IF EXISTS analytics_hourly_rollups_pkey;
ALTER TABLE analytics_daily_rollups DROP CONSTRAINT IF EXISTS analytics_daily_rollups_pkey;

-- Add actor_id column for per-API-key queries
ALTER TABLE analytics_hourly_rollups ADD COLUMN IF NOT EXISTS actor_id UUID;
ALTER TABLE analytics_daily_rollups ADD COLUMN IF NOT EXISTS actor_id UUID;

-- Create new primary keys that don't include nullable columns
-- Use COALESCE to handle NULL model_id and actor_id by converting to sentinel UUIDs
ALTER TABLE analytics_hourly_rollups
    ADD CONSTRAINT analytics_hourly_rollups_pkey
    PRIMARY KEY (
        bucket_start,
        organization_id,
        COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(actor_id, '00000000-0000-0000-0000-000000000000'::uuid)
    );

ALTER TABLE analytics_daily_rollups
    ADD CONSTRAINT analytics_daily_rollups_pkey
    PRIMARY KEY (
        bucket_start,
        organization_id,
        COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid),
        COALESCE(actor_id, '00000000-0000-0000-0000-000000000000'::uuid)
    );

-- Add indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_hourly_rollups_actor
    ON analytics_hourly_rollups (organization_id, actor_id, bucket_start DESC)
    WHERE actor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_daily_rollups_actor
    ON analytics_daily_rollups (organization_id, actor_id, bucket_start DESC)
    WHERE actor_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_hourly_rollups_model
    ON analytics_hourly_rollups (organization_id, model_id, bucket_start DESC)
    WHERE model_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_daily_rollups_model
    ON analytics_daily_rollups (organization_id, model_id, bucket_start DESC)
    WHERE model_id IS NOT NULL;

COMMIT;

-- +goose Down
BEGIN;

-- Drop added indexes
DROP INDEX IF EXISTS idx_daily_rollups_model;
DROP INDEX IF EXISTS idx_hourly_rollups_model;
DROP INDEX IF EXISTS idx_daily_rollups_actor;
DROP INDEX IF EXISTS idx_hourly_rollups_actor;

-- Restore original primary keys (will fail if NULL data exists)
ALTER TABLE analytics_hourly_rollups DROP CONSTRAINT IF EXISTS analytics_hourly_rollups_pkey;
ALTER TABLE analytics_daily_rollups DROP CONSTRAINT IF EXISTS analytics_daily_rollups_pkey;

ALTER TABLE analytics_hourly_rollups
    ADD PRIMARY KEY (bucket_start, organization_id, model_id);

ALTER TABLE analytics_daily_rollups
    ADD PRIMARY KEY (bucket_start, organization_id, model_id);

-- Remove actor_id column
ALTER TABLE analytics_hourly_rollups DROP COLUMN IF EXISTS actor_id;
ALTER TABLE analytics_daily_rollups DROP COLUMN IF EXISTS actor_id;

COMMIT;
