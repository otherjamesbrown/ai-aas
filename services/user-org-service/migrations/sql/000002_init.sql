-- +goose Up
-- Function set_updated_at() is created in 000001_setup_function.sql

CREATE TABLE orgs (
  org_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  slug TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  billing_owner_user_id UUID,
  budget_policy_id UUID,
  declarative_mode TEXT NOT NULL DEFAULT 'disabled',
  declarative_repo_url TEXT,
  declarative_branch TEXT,
  declarative_last_commit TEXT,
  mfa_required_roles JSONB NOT NULL DEFAULT '[]'::jsonb,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ
);

CREATE TABLE users (
  user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
  email TEXT NOT NULL,
  display_name TEXT NOT NULL,
  password_hash TEXT NOT NULL,
  status TEXT NOT NULL,
  mfa_enrolled BOOLEAN NOT NULL DEFAULT false,
  mfa_methods JSONB NOT NULL DEFAULT '[]'::jsonb,
  mfa_secret TEXT,
  last_login_at TIMESTAMPTZ,
  lockout_until TIMESTAMPTZ,
  recovery_tokens JSONB NOT NULL DEFAULT '[]'::jsonb,
  external_idp_id TEXT,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE(org_id, email)
);

CREATE TABLE service_accounts (
  service_account_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
  name TEXT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
  last_rotation_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE(org_id, name)
);

CREATE TABLE api_keys (
  api_key_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
  principal_type TEXT NOT NULL CHECK (principal_type IN ('user', 'service_account')),
  principal_id UUID NOT NULL,
  fingerprint TEXT NOT NULL,
  status TEXT NOT NULL,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  revoked_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ,
  last_used_at TIMESTAMPTZ,
  annotations JSONB NOT NULL DEFAULT '{}'::jsonb,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE(org_id, fingerprint)
);

CREATE TABLE sessions (
  session_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  org_id UUID NOT NULL REFERENCES orgs(org_id) ON DELETE CASCADE,
  user_id UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
  refresh_token_hash TEXT NOT NULL,
  ip_address TEXT,
  user_agent TEXT,
  mfa_verified_at TIMESTAMPTZ,
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ,
  version BIGINT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE(org_id, refresh_token_hash)
);

CREATE UNIQUE INDEX users_external_idp_idx ON users(org_id, external_idp_id) WHERE external_idp_id IS NOT NULL;
CREATE INDEX users_org_status_idx ON users(org_id, status);
CREATE INDEX api_keys_lookup_idx ON api_keys(org_id, principal_type, principal_id);
CREATE INDEX sessions_lookup_idx ON sessions(org_id, user_id);

CREATE TABLE oauth_sessions (
  signature TEXT PRIMARY KEY,
  token_type TEXT NOT NULL,
  request_id UUID NOT NULL,
  client_id TEXT NOT NULL,
  subject TEXT,
  org_id UUID,
  user_id UUID,
  scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  granted_scopes JSONB NOT NULL DEFAULT '[]'::jsonb,
  audience JSONB NOT NULL DEFAULT '[]'::jsonb,
  granted_audience JSONB NOT NULL DEFAULT '[]'::jsonb,
  form_data JSONB NOT NULL DEFAULT '{}'::jsonb,
  session_data JSONB NOT NULL,
  requested_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ,
  active BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX oauth_sessions_request_idx ON oauth_sessions(request_id);
CREATE INDEX oauth_sessions_type_idx ON oauth_sessions(token_type);

CREATE TRIGGER trg_orgs_updated_at BEFORE UPDATE ON orgs FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_users_updated_at BEFORE UPDATE ON users FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_service_accounts_updated_at BEFORE UPDATE ON service_accounts FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_api_keys_updated_at BEFORE UPDATE ON api_keys FOR EACH ROW EXECUTE FUNCTION set_updated_at();
CREATE TRIGGER trg_sessions_updated_at BEFORE UPDATE ON sessions FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE orgs ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE service_accounts ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY orgs_tenant_isolation ON orgs
USING (org_id::text = current_setting('app.org_id', true))
WITH CHECK (org_id::text = current_setting('app.org_id', true));

CREATE POLICY users_tenant_isolation ON users
USING (org_id::text = current_setting('app.org_id', true))
WITH CHECK (org_id::text = current_setting('app.org_id', true));

CREATE POLICY service_accounts_tenant_isolation ON service_accounts
USING (org_id::text = current_setting('app.org_id', true))
WITH CHECK (org_id::text = current_setting('app.org_id', true));

CREATE POLICY api_keys_tenant_isolation ON api_keys
USING (org_id::text = current_setting('app.org_id', true))
WITH CHECK (org_id::text = current_setting('app.org_id', true));

CREATE POLICY sessions_tenant_isolation ON sessions
USING (org_id::text = current_setting('app.org_id', true))
WITH CHECK (org_id::text = current_setting('app.org_id', true));

-- +goose Down
DROP POLICY IF EXISTS sessions_tenant_isolation ON sessions;
DROP POLICY IF EXISTS api_keys_tenant_isolation ON api_keys;
DROP POLICY IF EXISTS service_accounts_tenant_isolation ON service_accounts;
DROP POLICY IF EXISTS users_tenant_isolation ON users;
DROP POLICY IF EXISTS orgs_tenant_isolation ON orgs;

ALTER TABLE sessions DISABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys DISABLE ROW LEVEL SECURITY;
ALTER TABLE service_accounts DISABLE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;
ALTER TABLE orgs DISABLE ROW LEVEL SECURITY;

DROP TRIGGER IF EXISTS trg_sessions_updated_at ON sessions;
DROP TRIGGER IF EXISTS trg_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS trg_service_accounts_updated_at ON service_accounts;
DROP TRIGGER IF EXISTS trg_users_updated_at ON users;
DROP TRIGGER IF EXISTS trg_orgs_updated_at ON orgs;

-- Function set_updated_at() is dropped in 000001_setup_function.sql

DROP TABLE IF EXISTS oauth_sessions;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS service_accounts;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS orgs;

