BEGIN;

CREATE TABLE IF NOT EXISTS analytics_hourly_rollups (
    bucket_start TIMESTAMPTZ NOT NULL,
    organization_id UUID NOT NULL,
    model_id UUID,
    request_count BIGINT NOT NULL DEFAULT 0,
    tokens_total BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    cost_total NUMERIC(14,4) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_start, organization_id, model_id)
);

CREATE TABLE IF NOT EXISTS analytics_daily_rollups (
    bucket_start DATE NOT NULL,
    organization_id UUID NOT NULL,
    model_id UUID,
    request_count BIGINT NOT NULL DEFAULT 0,
    tokens_total BIGINT NOT NULL DEFAULT 0,
    error_count BIGINT NOT NULL DEFAULT 0,
    cost_total NUMERIC(14,4) NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_start, organization_id, model_id)
);

CREATE INDEX IF NOT EXISTS idx_hourly_rollups_org ON analytics_hourly_rollups (organization_id, bucket_start DESC);
CREATE INDEX IF NOT EXISTS idx_daily_rollups_org ON analytics_daily_rollups (organization_id, bucket_start DESC);

COMMIT;
