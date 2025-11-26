-- Migration: Create routing_policies table for admin-api-service
-- Feature: 017-admin-api-service

CREATE TABLE IF NOT EXISTS routing_policies (
    policy_id VARCHAR(255) PRIMARY KEY,
    organization_id VARCHAR(255) NOT NULL,  -- VARCHAR to support '*' for global policies
    model VARCHAR(255) NOT NULL,
    backends JSONB NOT NULL,
    fallback_backends JSONB DEFAULT '[]'::jsonb,
    failover_threshold INTEGER NOT NULL DEFAULT 3,
    enabled BOOLEAN NOT NULL DEFAULT true,
    version INTEGER NOT NULL DEFAULT 1,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    created_by VARCHAR(255),
    updated_by VARCHAR(255),
    deleted_at TIMESTAMP NULL,
    
    CONSTRAINT check_backends_not_empty CHECK (jsonb_array_length(backends) > 0),
    CONSTRAINT check_failover_threshold CHECK (failover_threshold BETWEEN 1 AND 10)
);

CREATE INDEX IF NOT EXISTS idx_routing_policies_model ON routing_policies(model) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_routing_policies_org ON routing_policies(organization_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_routing_policies_enabled ON routing_policies(enabled) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_routing_policies_updated_at ON routing_policies(updated_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_routing_policies_unique_org_model ON routing_policies(organization_id, model) WHERE deleted_at IS NULL;

