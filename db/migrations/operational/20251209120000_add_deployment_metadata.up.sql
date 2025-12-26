-- +goose Up
-- Add deployment metadata to model_registry_entries table
-- Migration: 20251209120000_add_deployment_metadata.sql

BEGIN;

-- Add deployment endpoint URL
ALTER TABLE model_registry_entries
ADD COLUMN IF NOT EXISTS deployment_endpoint TEXT;

-- Add deployment status column (without constraint first)
ALTER TABLE model_registry_entries
ADD COLUMN IF NOT EXISTS deployment_status TEXT;

-- Add deployment status constraint (idempotent)
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'chk_deployment_status'
        AND table_name = 'model_registry_entries'
    ) THEN
        ALTER TABLE model_registry_entries ADD CONSTRAINT chk_deployment_status
            CHECK (deployment_status IN ('pending', 'deploying', 'ready', 'degraded', 'failed', 'disabled'));
    END IF;
END $$;
-- +goose StatementEnd

-- Add deployment environment column (without constraint first)
ALTER TABLE model_registry_entries
ADD COLUMN IF NOT EXISTS deployment_environment TEXT;

-- Add deployment environment constraint (idempotent)
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'chk_deployment_environment'
        AND table_name = 'model_registry_entries'
    ) THEN
        ALTER TABLE model_registry_entries ADD CONSTRAINT chk_deployment_environment
            CHECK (deployment_environment IN ('development', 'staging', 'production'));
    END IF;
END $$;
-- +goose StatementEnd

-- Add deployment namespace
ALTER TABLE model_registry_entries
ADD COLUMN IF NOT EXISTS deployment_namespace TEXT;

-- Add last health check timestamp
ALTER TABLE model_registry_entries
ADD COLUMN IF NOT EXISTS last_health_check_at TIMESTAMPTZ;

-- Create index for API Router queries (filtering by ready status)
CREATE INDEX IF NOT EXISTS idx_model_registry_deployment_status
ON model_registry_entries(deployment_status, model_name)
WHERE deployment_status = 'ready';

-- Create index for environment queries
CREATE INDEX IF NOT EXISTS idx_model_registry_environment
ON model_registry_entries(deployment_environment, deployment_status);

-- Create unique constraint for model_name + environment (for upsert in registration)
-- This allows the same model to be deployed in different environments
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'model_registry_entries_unique_deployment'
        AND table_name = 'model_registry_entries'
    ) THEN
        ALTER TABLE model_registry_entries
        ADD CONSTRAINT model_registry_entries_unique_deployment
        UNIQUE (model_name, deployment_environment);
    END IF;
END $$;
-- +goose StatementEnd

COMMIT;
