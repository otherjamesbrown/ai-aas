-- Rollback external_name from model_registry table

BEGIN;

DROP INDEX IF EXISTS idx_model_registry_external_name;
ALTER TABLE model_registry DROP COLUMN IF EXISTS external_name;

COMMIT;
