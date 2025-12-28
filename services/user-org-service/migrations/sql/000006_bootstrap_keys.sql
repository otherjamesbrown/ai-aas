-- Migration: 000006_bootstrap_keys
-- Description: Create bootstrap_keys table for org admin onboarding
-- Bootstrap keys are one-time use tokens that allow org admins to initialize their CLI

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS bootstrap_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id TEXT NOT NULL,                           -- Short human-readable ID (e.g., "bsk_abc123")
    org_id UUID NOT NULL REFERENCES orgs(id),
    fingerprint TEXT NOT NULL,                      -- SHA-256 hash of the token
    status TEXT NOT NULL DEFAULT 'active',          -- active, revoked, redeemed, expired
    notes TEXT NOT NULL DEFAULT '',
    expires_at TIMESTAMPTZ NOT NULL,
    redeemed_at TIMESTAMPTZ,
    redeemed_by TEXT NOT NULL DEFAULT '',           -- User ID or email who redeemed
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Unique index on key_id for lookups
CREATE UNIQUE INDEX IF NOT EXISTS bootstrap_keys_key_id_idx ON bootstrap_keys(key_id);

-- Unique index on fingerprint for token validation
CREATE UNIQUE INDEX IF NOT EXISTS bootstrap_keys_fingerprint_idx ON bootstrap_keys(fingerprint);

-- Index for listing by org and status
CREATE INDEX IF NOT EXISTS bootstrap_keys_org_status_idx ON bootstrap_keys(org_id, status);

-- Add constraint for valid status values
ALTER TABLE bootstrap_keys ADD CONSTRAINT bootstrap_keys_status_check
    CHECK (status IN ('active', 'revoked', 'redeemed', 'expired'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS bootstrap_keys;
-- +goose StatementEnd
