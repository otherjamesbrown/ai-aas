-- Rollback: Remove model_type from model_registry

ALTER TABLE model_registry
DROP COLUMN IF EXISTS model_type;
