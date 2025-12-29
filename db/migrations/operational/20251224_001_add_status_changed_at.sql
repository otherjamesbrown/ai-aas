-- Migration: Add status_changed_at to model_deployments
-- Feature: ai-aas-rwnx
-- Description: Track when deployment status last changed to show phase duration

-- +goose Up
ALTER TABLE model_deployments
ADD COLUMN IF NOT EXISTS status_changed_at TIMESTAMP;

-- Backfill status_changed_at with created_at for existing rows
-- This gives a reasonable default for existing deployments
UPDATE model_deployments
SET status_changed_at = created_at
WHERE status_changed_at IS NULL;

COMMENT ON COLUMN model_deployments.status_changed_at IS 'Timestamp when status last changed (for phase duration tracking)';

-- +goose Down
ALTER TABLE model_deployments
DROP COLUMN IF EXISTS status_changed_at;
