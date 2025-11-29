-- Migration: Create model_state_history table
-- Feature: 020-model-management
-- Description: Audit trail for enable/disable operations

-- +goose Up
CREATE TABLE IF NOT EXISTS model_state_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    deployment_id UUID NOT NULL REFERENCES model_deployments(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL,                    -- enabled, disabled, swapped_out, swapped_in
    performed_by VARCHAR(255),                      -- User who performed action
    reason TEXT,                                    -- Optional reason for change
    scheduled_at TIMESTAMP,                         -- If scheduled for future
    executed_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for audit queries
CREATE INDEX idx_model_state_history_deployment_id ON model_state_history(deployment_id);
CREATE INDEX idx_model_state_history_executed_at ON model_state_history(executed_at);
CREATE INDEX idx_model_state_history_action ON model_state_history(action);
CREATE INDEX idx_model_state_history_performed_by ON model_state_history(performed_by);

-- Constraint for valid actions
ALTER TABLE model_state_history ADD CONSTRAINT chk_model_state_history_action 
    CHECK (action IN ('enabled', 'disabled', 'swapped_out', 'swapped_in'));

COMMENT ON TABLE model_state_history IS 'Audit trail for enable/disable operations';
COMMENT ON COLUMN model_state_history.action IS 'Action type: enabled, disabled, swapped_out, swapped_in';
COMMENT ON COLUMN model_state_history.scheduled_at IS 'If set, indicates a scheduled operation';

-- +goose Down
ALTER TABLE model_state_history DROP CONSTRAINT IF EXISTS chk_model_state_history_action;
DROP INDEX IF EXISTS idx_model_state_history_performed_by;
DROP INDEX IF EXISTS idx_model_state_history_action;
DROP INDEX IF EXISTS idx_model_state_history_executed_at;
DROP INDEX IF EXISTS idx_model_state_history_deployment_id;
DROP TABLE IF EXISTS model_state_history;

