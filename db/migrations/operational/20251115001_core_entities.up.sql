-- +goose Up
-- Core entities for operational schema
BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS organizations (
    organization_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    plan_tier TEXT NOT NULL CHECK (plan_tier IN ('starter','growth','enterprise')),
    budget_limit_tokens BIGINT NOT NULL DEFAULT 0 CHECK (budget_limit_tokens >= 0),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','closed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    soft_deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS users (
    user_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(organization_id) ON DELETE RESTRICT,
    email TEXT NOT NULL,
    email_hash TEXT NOT NULL,
    role TEXT NOT NULL CHECK (role IN ('owner','admin','developer','billing')),
    status TEXT NOT NULL DEFAULT 'invited' CHECK (status IN ('active','invited','disabled')),
    last_login_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    soft_deleted_at TIMESTAMPTZ,
    CONSTRAINT users_email_unique UNIQUE (organization_id, email_hash)
);

CREATE TABLE IF NOT EXISTS api_keys (
    api_key_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(organization_id) ON DELETE RESTRICT,
    name TEXT NOT NULL,
    hashed_secret BYTEA NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT ARRAY[]::TEXT[],
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','rotating','revoked')),
    last_used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT api_keys_unique_name_per_org UNIQUE (organization_id, name)
);

CREATE TABLE IF NOT EXISTS model_registry_entries (
    model_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000'::UUID REFERENCES organizations(organization_id) ON DELETE SET DEFAULT,
    model_name TEXT NOT NULL,
    revision INT NOT NULL,
    deployment_target TEXT NOT NULL CHECK (deployment_target IN ('managed','self_hosted')),
    cost_per_1k_tokens NUMERIC(10,4) NOT NULL DEFAULT 0,
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT model_registry_entries_unique_revision UNIQUE (organization_id, model_name, revision)
);

CREATE TABLE IF NOT EXISTS audit_log_entries (
    audit_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_type TEXT NOT NULL CHECK (actor_type IN ('user','service','migration')),
    actor_id TEXT NOT NULL,
    action TEXT NOT NULL,
    target TEXT,
    metadata JSONB NOT NULL DEFAULT '{}'::JSONB,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Create indexes only if columns exist
-- This handles cases where tables exist with different schemas
-- +goose StatementBegin
DO $$
BEGIN
    -- Index on users.organization_id (only if column exists)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'users' AND column_name = 'organization_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_users_org ON users (organization_id);
    END IF;

    -- Index on api_keys.organization_id (only if column exists)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'api_keys' AND column_name = 'organization_id'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_api_keys_org ON api_keys (organization_id);
    END IF;

    -- Index on model_registry_entries.organization_id and model_name (only if columns exist)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'model_registry_entries' AND column_name = 'organization_id'
    ) AND EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'model_registry_entries' AND column_name = 'model_name'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_model_registry_org ON model_registry_entries (organization_id, model_name);
    END IF;

    -- Index on audit_log_entries.target (only if column exists)
    IF EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_name = 'audit_log_entries' AND column_name = 'target'
    ) THEN
        CREATE INDEX IF NOT EXISTS idx_audit_log_entries_target ON audit_log_entries (target);
    END IF;
END $$;
-- +goose StatementEnd

COMMIT;

-- +goose Down
-- Rollback core entities for operational schema
BEGIN;

DROP INDEX IF EXISTS idx_audit_log_entries_target;
DROP INDEX IF EXISTS idx_model_registry_org;
DROP INDEX IF EXISTS idx_api_keys_org;
DROP INDEX IF EXISTS idx_users_org;

DROP TABLE IF EXISTS audit_log_entries;
DROP TABLE IF EXISTS model_registry_entries;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;

COMMIT;
