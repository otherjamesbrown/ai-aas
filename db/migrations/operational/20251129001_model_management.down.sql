-- +goose Down
-- Rollback Model Management Tables

-- Drop triggers first
DROP TRIGGER IF EXISTS update_platform_credentials_updated_at ON platform_credentials;
DROP TRIGGER IF EXISTS update_model_deployments_updated_at ON model_deployments;
DROP TRIGGER IF EXISTS update_model_aliases_updated_at ON model_aliases;
DROP TRIGGER IF EXISTS update_model_registry_updated_at ON model_registry;

-- Drop function (only if no other triggers use it)
-- DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_model_aliases_model_id;
DROP INDEX IF EXISTS idx_model_validations_model_id;
DROP INDEX IF EXISTS idx_model_state_history_deployment_id;
DROP INDEX IF EXISTS idx_model_deployments_status;
DROP INDEX IF EXISTS idx_model_deployments_environment;
DROP INDEX IF EXISTS idx_model_deployments_model_id;
DROP INDEX IF EXISTS idx_model_cache_status;
DROP INDEX IF EXISTS idx_model_cache_model_id;

-- Drop tables in reverse order of creation (respecting FK constraints)
DROP TABLE IF EXISTS platform_credentials;
DROP TABLE IF EXISTS model_validations;
DROP TABLE IF EXISTS model_state_history;
DROP TABLE IF EXISTS model_deployments;
DROP TABLE IF EXISTS model_cache;
DROP TABLE IF EXISTS model_aliases;
DROP TABLE IF EXISTS model_registry;
