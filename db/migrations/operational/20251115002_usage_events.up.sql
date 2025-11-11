-- Usage event fact table and supporting indexes
BEGIN;

CREATE TABLE IF NOT EXISTS usage_events (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    occurred_at TIMESTAMPTZ NOT NULL,
    organization_id UUID NOT NULL REFERENCES organizations(organization_id) ON DELETE RESTRICT,
    api_key_id UUID NOT NULL REFERENCES api_keys(api_key_id) ON DELETE RESTRICT,
    model_id UUID NOT NULL REFERENCES model_registry_entries(model_id) ON DELETE RESTRICT,
    tokens_consumed BIGINT NOT NULL CHECK (tokens_consumed >= 0),
    latency_ms INT NOT NULL CHECK (latency_ms >= 0),
    status TEXT NOT NULL CHECK (status IN ('success','rate_limited','error')),
    error_code TEXT,
    region TEXT NOT NULL,
    cost_usd NUMERIC(12,4) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_events_org_time ON usage_events USING BRIN (organization_id, occurred_at);
CREATE INDEX IF NOT EXISTS idx_usage_events_model_time ON usage_events (model_id, occurred_at DESC);
CREATE INDEX IF NOT EXISTS idx_usage_events_status ON usage_events (status, occurred_at);

COMMIT;
