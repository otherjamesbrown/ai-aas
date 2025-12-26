-- Rollback deployment metadata from model_registry_entries table

BEGIN;

-- Drop unique constraint
ALTER TABLE model_registry_entries DROP CONSTRAINT IF EXISTS model_registry_entries_unique_deployment;

-- Drop check constraints
ALTER TABLE model_registry_entries DROP CONSTRAINT IF EXISTS chk_deployment_status;
ALTER TABLE model_registry_entries DROP CONSTRAINT IF EXISTS chk_deployment_environment;

DROP INDEX IF EXISTS idx_model_registry_environment;
DROP INDEX IF EXISTS idx_model_registry_deployment_status;

ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS last_health_check_at;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_namespace;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_environment;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_status;
ALTER TABLE model_registry_entries DROP COLUMN IF EXISTS deployment_endpoint;

COMMIT;
