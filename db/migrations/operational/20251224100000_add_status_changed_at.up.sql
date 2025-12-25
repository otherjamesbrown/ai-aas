-- Migration: Add status_changed_at to model_deployments
-- Feature: ai-aas-yk31
-- Description: Track when deployment status last changed for phase duration tracking and operator sync

-- +goose Up
ALTER TABLE model_deployments
ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMP WITH TIME ZONE DEFAULT NOW();

-- Backfill status_changed_at with created_at for existing rows
-- This gives a reasonable default for existing deployments
UPDATE model_deployments
SET status_changed_at = created_at
WHERE status_changed_at IS NULL;

-- Create trigger to automatically update status_changed_at when status changes
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION update_deployment_status_changed_at()
RETURNS TRIGGER AS $$
BEGIN
    -- Only update status_changed_at if status actually changed
    IF NEW.status IS DISTINCT FROM OLD.status THEN
        NEW.status_changed_at = NOW();
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trigger_deployment_status_changed_at ON model_deployments;
CREATE TRIGGER trigger_deployment_status_changed_at
    BEFORE UPDATE ON model_deployments
    FOR EACH ROW
    EXECUTE FUNCTION update_deployment_status_changed_at();

COMMENT ON COLUMN model_deployments.status_changed_at IS 'Timestamp when status last changed (for phase duration tracking)';

-- +goose Down
DROP TRIGGER IF EXISTS trigger_deployment_status_changed_at ON model_deployments;
DROP FUNCTION IF EXISTS update_deployment_status_changed_at();
ALTER TABLE model_deployments
DROP COLUMN IF EXISTS status_changed_at;
