-- Add deployment metadata to model_registry_entries table
-- Migration: 20250127120000_add_deployment_metadata.up.sql

BEGIN;

-- Add deployment endpoint URL
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_endpoint TEXT;

-- Add deployment status
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_status TEXT 
CHECK (deployment_status IN ('pending', 'deploying', 'ready', 'degraded', 'failed', 'disabled'));

-- Add deployment environment
ALTER TABLE model_registry_entries 
ADD COLUMN IF NOT EXISTS deployment_environment TEXT 
CHECK (deployment_environment IN ('development', 'staging', 'production'));

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
ALTER TABLE model_registry_entries
ADD CONSTRAINT model_registry_entries_unique_deployment
UNIQUE (model_name, deployment_environment);

COMMIT;

