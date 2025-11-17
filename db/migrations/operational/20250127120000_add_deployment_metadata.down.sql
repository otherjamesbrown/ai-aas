-- Rollback deployment metadata from model_registry_entries table
-- Migration: 20250127120000_add_deployment_metadata.down.sql

BEGIN;

DROP INDEX IF EXISTS idx_model_registry_environment;
DROP INDEX IF EXISTS idx_model_registry_deployment_status;

ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS last_health_check_at;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_namespace;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_environment;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_status;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_endpoint;

COMMIT;

