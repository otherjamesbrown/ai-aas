-- Migration: 000008_token_policies
-- Description: Add token rate-limit policies for controlling API usage
-- Spec: 035-token-rate-limits

-- +goose Up
-- +goose StatementBegin

-- Token rate-limit policies (org-scoped, with one system-wide built-in)
CREATE TABLE token_rate_limit_policies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID REFERENCES orgs(org_id) ON DELETE CASCADE,  -- NULL for system built-in
    name TEXT NOT NULL,
    description TEXT,
    limit_1h BIGINT,      -- 1-hour rolling window limit (NULL = unlimited)
    limit_24h BIGINT,     -- 24-hour rolling window limit (NULL = unlimited)
    limit_7d BIGINT,      -- 7-day rolling window limit (NULL = unlimited)
    is_builtin BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT token_policies_name_unique UNIQUE NULLS NOT DISTINCT (org_id, name)
);

-- System-wide "No Token Rate-Limit" policy (well-known UUID)
-- This policy has no limits and is available to all orgs
INSERT INTO token_rate_limit_policies (id, org_id, name, description, is_builtin)
VALUES (
    '00000000-0000-0000-0000-000000000001',
    NULL,
    'No Token Rate-Limit',
    'System default: No token rate limiting applied',
    TRUE
);

-- Add default token policy to orgs (defaults to "No Token Rate-Limit")
ALTER TABLE orgs
    ADD COLUMN default_token_policy_id UUID
    REFERENCES token_rate_limit_policies(id)
    DEFAULT '00000000-0000-0000-0000-000000000001';

-- Add per-user token policy override (NULL = inherit from org)
ALTER TABLE users
    ADD COLUMN token_policy_override_id UUID
    REFERENCES token_rate_limit_policies(id);

-- Rolling window usage tracking
-- Each row tracks tokens consumed within a specific time window
CREATE TABLE token_usage_windows (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    window_type TEXT NOT NULL CHECK (window_type IN ('1h', '24h', '7d')),
    window_start TIMESTAMPTZ NOT NULL,
    tokens_used BIGINT NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT token_usage_user_window_unique UNIQUE (user_id, window_type, window_start)
);

-- Index for efficient usage queries by user and window type
CREATE INDEX idx_token_usage_user_window ON token_usage_windows(user_id, window_type, window_start DESC);

-- Index for cleanup of expired windows
CREATE INDEX idx_token_usage_window_start ON token_usage_windows(window_start);

-- Index for policy lookups by org
CREATE INDEX idx_token_policies_org ON token_rate_limit_policies(org_id) WHERE org_id IS NOT NULL;

-- Trigger for updated_at on policies
CREATE TRIGGER trg_token_policies_updated_at
    BEFORE UPDATE ON token_rate_limit_policies
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- Trigger for updated_at on usage windows
CREATE TRIGGER trg_token_usage_updated_at
    BEFORE UPDATE ON token_usage_windows
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TRIGGER IF EXISTS trg_token_usage_updated_at ON token_usage_windows;
DROP TRIGGER IF EXISTS trg_token_policies_updated_at ON token_rate_limit_policies;

DROP INDEX IF EXISTS idx_token_policies_org;
DROP INDEX IF EXISTS idx_token_usage_window_start;
DROP INDEX IF EXISTS idx_token_usage_user_window;

DROP TABLE IF EXISTS token_usage_windows;

ALTER TABLE users DROP COLUMN IF EXISTS token_policy_override_id;
ALTER TABLE orgs DROP COLUMN IF EXISTS default_token_policy_id;

DROP TABLE IF EXISTS token_rate_limit_policies;

-- +goose StatementEnd
