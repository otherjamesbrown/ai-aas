-- Migration: 000007_audit_logs
-- Description: Create audit_logs table for audit event storage
-- Audit logs track all state-changing operations for compliance and security

-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS audit_logs (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id UUID NOT NULL REFERENCES orgs(org_id),
    actor_id UUID NOT NULL,                         -- User or service account ID
    actor_type TEXT NOT NULL,                       -- "user", "service_account", "system"
    actor_email TEXT NOT NULL DEFAULT '',
    target_id UUID,                                 -- Resource being acted upon
    target_type TEXT,                               -- "org", "user", "api_key", etc.
    action TEXT NOT NULL,                           -- "org.create", "user.invite", "user.suspend", etc.
    resource TEXT,                                  -- Resource path or identifier
    policy_id UUID,
    ip_address TEXT,
    user_agent TEXT,
    metadata JSONB,                                 -- Additional context
    hash TEXT NOT NULL,                             -- SHA256 of event payload (for tamper detection)
    signature TEXT NOT NULL DEFAULT '',             -- Reserved for Ed25519 signature
    delivered_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Index for listing by org (primary query pattern)
CREATE INDEX IF NOT EXISTS audit_logs_org_created_idx ON audit_logs(org_id, created_at DESC);

-- Index for filtering by action
CREATE INDEX IF NOT EXISTS audit_logs_org_action_idx ON audit_logs(org_id, action, created_at DESC);

-- Index for filtering by actor
CREATE INDEX IF NOT EXISTS audit_logs_org_actor_idx ON audit_logs(org_id, actor_id, created_at DESC);

-- Index for filtering by target type (resource type)
CREATE INDEX IF NOT EXISTS audit_logs_org_target_type_idx ON audit_logs(org_id, target_type, created_at DESC);

-- Composite index for common query pattern (org + filters)
CREATE INDEX IF NOT EXISTS audit_logs_org_filters_idx ON audit_logs(org_id, action, target_type, actor_id, created_at DESC);

-- Add constraint for valid actor_type values
ALTER TABLE audit_logs ADD CONSTRAINT audit_logs_actor_type_check
    CHECK (actor_type IN ('user', 'service_account', 'system', 'api_key'));
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS audit_logs;
-- +goose StatementEnd
