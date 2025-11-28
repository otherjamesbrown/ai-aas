-- Migration: Create model_validations table
-- Feature: 020-model-management
-- Description: Records validation check results

-- +goose Up
CREATE TABLE IF NOT EXISTS model_validations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_id UUID NOT NULL REFERENCES model_registry(id) ON DELETE CASCADE,
    environment VARCHAR(50),                        -- Environment validated
    validation_type VARCHAR(100),                   -- Layer: registry, cache, deployment, endpoint, router
    check_name VARCHAR(100),                        -- Specific check name
    status VARCHAR(20),                             -- pass, warn, fail, skip
    message TEXT,                                   -- Status message
    remediation TEXT,                               -- Fix suggestion
    validated_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for queries
CREATE INDEX idx_model_validations_model_id ON model_validations(model_id);
CREATE INDEX idx_model_validations_environment ON model_validations(environment);
CREATE INDEX idx_model_validations_status ON model_validations(status);
CREATE INDEX idx_model_validations_validated_at ON model_validations(validated_at);
CREATE INDEX idx_model_validations_validation_type ON model_validations(validation_type);

-- Constraints
ALTER TABLE model_validations ADD CONSTRAINT chk_model_validations_status 
    CHECK (status IN ('pass', 'warn', 'fail', 'skip'));

ALTER TABLE model_validations ADD CONSTRAINT chk_model_validations_type 
    CHECK (validation_type IN ('registry', 'cache', 'deployment', 'endpoint', 'router'));

COMMENT ON TABLE model_validations IS 'Records validation check results for audit and debugging';
COMMENT ON COLUMN model_validations.validation_type IS 'Layer being validated: registry, cache, deployment, endpoint, router';
COMMENT ON COLUMN model_validations.remediation IS 'Suggested fix for failed checks';

-- +goose Down
ALTER TABLE model_validations DROP CONSTRAINT IF EXISTS chk_model_validations_type;
ALTER TABLE model_validations DROP CONSTRAINT IF EXISTS chk_model_validations_status;
DROP INDEX IF EXISTS idx_model_validations_validation_type;
DROP INDEX IF EXISTS idx_model_validations_validated_at;
DROP INDEX IF EXISTS idx_model_validations_status;
DROP INDEX IF EXISTS idx_model_validations_environment;
DROP INDEX IF EXISTS idx_model_validations_model_id;
DROP TABLE IF EXISTS model_validations;

