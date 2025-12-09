-- Migration: Create model_deployments table
-- Feature: 020-model-management
-- Description: Tracks deployed model instances per environment

-- +goose Up
CREATE TABLE IF NOT EXISTS model_deployments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    cache_id UUID REFERENCES model_cache(id),       -- Cache version being used
    environment VARCHAR(50) NOT NULL,               -- development, staging, production
    namespace VARCHAR(100) NOT NULL,                -- Kubernetes namespace
    inferenceservice_name VARCHAR(255),             -- KServe resource name
    endpoint VARCHAR(512),                          -- Service endpoint URL
    enabled BOOLEAN DEFAULT true,                   -- Whether deployment is active
    status VARCHAR(50) DEFAULT 'pending',           -- Deployment status
    replicas_desired INT DEFAULT 1,                 -- Target replica count
    replicas_ready INT DEFAULT 0,                   -- Current ready replicas
    gpu_count INT DEFAULT 1,                        -- GPUs per replica
    memory_gb INT,                                  -- Memory per replica
    last_health_check_at TIMESTAMP,                 -- Last health check time
    last_health_status VARCHAR(50),                 -- Last health result
    last_enabled_at TIMESTAMP,                      -- When last enabled
    last_enabled_by VARCHAR(255),                   -- Who enabled
    last_disabled_at TIMESTAMP,                     -- When last disabled
    last_disabled_by VARCHAR(255),                  -- Who disabled
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(model_id, environment)
);

-- Indexes for common queries
CREATE INDEX idx_model_deployments_model_id ON model_deployments(model_id);
CREATE INDEX idx_model_deployments_environment ON model_deployments(environment);
CREATE INDEX idx_model_deployments_status ON model_deployments(status);
CREATE INDEX idx_model_deployments_enabled ON model_deployments(enabled);

-- Constraints (idempotent)
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'chk_model_deployments_environment'
        AND table_name = 'model_deployments'
    ) THEN
        ALTER TABLE model_deployments ADD CONSTRAINT chk_model_deployments_environment
            CHECK (environment IN ('development', 'staging', 'production'));
    END IF;

    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'chk_model_deployments_status'
        AND table_name = 'model_deployments'
    ) THEN
        ALTER TABLE model_deployments ADD CONSTRAINT chk_model_deployments_status
            CHECK (status IN ('pending', 'deploying', 'ready', 'failed', 'disabled', 'terminated'));
    END IF;
END $$;
-- +goose StatementEnd

-- Trigger to update updated_at
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_model_deployments_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trigger_model_deployments_updated_at
    BEFORE UPDATE ON model_deployments
    FOR EACH ROW
    EXECUTE FUNCTION update_model_deployments_updated_at();

COMMENT ON TABLE model_deployments IS 'Tracks deployed model instances per environment';
COMMENT ON COLUMN model_deployments.enabled IS 'False = disabled (config kept but not deployed)';
COMMENT ON COLUMN model_deployments.status IS 'Deployment status: pending, deploying, ready, failed, disabled, terminated';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_model_deployments_updated_at ON model_deployments;
DROP FUNCTION IF EXISTS update_model_deployments_updated_at();
ALTER TABLE model_deployments DROP CONSTRAINT IF EXISTS chk_model_deployments_status;
ALTER TABLE model_deployments DROP CONSTRAINT IF EXISTS chk_model_deployments_environment;
DROP INDEX IF EXISTS idx_model_deployments_enabled;
DROP INDEX IF EXISTS idx_model_deployments_status;
DROP INDEX IF EXISTS idx_model_deployments_environment;
DROP INDEX IF EXISTS idx_model_deployments_model_id;
DROP TABLE IF EXISTS model_deployments;

