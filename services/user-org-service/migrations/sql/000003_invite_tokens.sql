-- +goose Up
-- Migration: Add invite_tokens table for secure token storage

CREATE TABLE IF NOT EXISTS invite_tokens (
  invite_token_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  token_hash TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_by_user_id UUID,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(org_id, token_hash)
);

CREATE INDEX IF NOT EXISTS invite_tokens_user_idx ON invite_tokens(org_id, user_id);
CREATE INDEX IF NOT EXISTS invite_tokens_expires_idx ON invite_tokens(expires_at) WHERE used_at IS NULL;

-- Enable RLS
ALTER TABLE invite_tokens ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS invite_tokens_tenant_isolation ON invite_tokens;
CREATE POLICY invite_tokens_tenant_isolation ON invite_tokens
USING (org_id::text = current_setting('app.org_id', true))
WITH CHECK (org_id::text = current_setting('app.org_id', true));

-- +goose Down
DROP POLICY IF EXISTS invite_tokens_tenant_isolation ON invite_tokens;
ALTER TABLE invite_tokens DISABLE ROW LEVEL SECURITY;
DROP INDEX IF EXISTS invite_tokens_expires_idx;
DROP INDEX IF EXISTS invite_tokens_user_idx;
DROP TABLE IF EXISTS invite_tokens;

