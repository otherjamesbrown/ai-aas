-- +goose Up
-- User Model Access Control - Spec 022
-- Adds per-user model access control within organizations

-- User model access mode (per user per org)
CREATE TABLE user_model_access (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    access_mode VARCHAR(20) NOT NULL DEFAULT 'restricted'
        CHECK (access_mode IN ('restricted', 'auto_grant')),
    version BIGINT NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(org_id, user_id)
);

-- Primary lookup index for access mode checks
CREATE INDEX idx_user_model_access_org_user ON user_model_access(org_id, user_id);

-- Trigger for updated_at
CREATE TRIGGER trg_user_model_access_updated_at
    BEFORE UPDATE ON user_model_access
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- User model grants (explicit grants for restricted mode)
CREATE TABLE user_model_grants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    model_name VARCHAR(255) NOT NULL,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by UUID REFERENCES users(user_id) ON DELETE SET NULL,
    expires_at TIMESTAMPTZ,
    UNIQUE(org_id, user_id, model_name)
);

-- Primary lookup index for access check query (hot path)
CREATE INDEX idx_user_model_grants_lookup ON user_model_grants(org_id, user_id, model_name);

-- Index for listing grants for a user
CREATE INDEX idx_user_model_grants_user ON user_model_grants(user_id);

-- Index for cleanup of expired grants
CREATE INDEX idx_user_model_grants_expiry ON user_model_grants(expires_at)
    WHERE expires_at IS NOT NULL;

-- Enable row level security for tenant isolation
ALTER TABLE user_model_access ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_model_grants ENABLE ROW LEVEL SECURITY;

CREATE POLICY user_model_access_tenant_isolation ON user_model_access
    USING (org_id::text = current_setting('app.org_id', true))
    WITH CHECK (org_id::text = current_setting('app.org_id', true));

CREATE POLICY user_model_grants_tenant_isolation ON user_model_grants
    USING (org_id::text = current_setting('app.org_id', true))
    WITH CHECK (org_id::text = current_setting('app.org_id', true));

-- +goose Down
DROP POLICY IF EXISTS user_model_grants_tenant_isolation ON user_model_grants;
DROP POLICY IF EXISTS user_model_access_tenant_isolation ON user_model_access;

ALTER TABLE user_model_grants DISABLE ROW LEVEL SECURITY;
ALTER TABLE user_model_access DISABLE ROW LEVEL SECURITY;

DROP INDEX IF EXISTS idx_user_model_grants_expiry;
DROP INDEX IF EXISTS idx_user_model_grants_user;
DROP INDEX IF EXISTS idx_user_model_grants_lookup;
DROP TABLE IF EXISTS user_model_grants;

DROP TRIGGER IF EXISTS trg_user_model_access_updated_at ON user_model_access;
DROP INDEX IF EXISTS idx_user_model_access_org_user;
DROP TABLE IF EXISTS user_model_access;
