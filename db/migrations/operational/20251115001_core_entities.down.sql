-- Rollback core entities for operational schema
BEGIN;

DROP INDEX IF EXISTS idx_audit_log_entries_target;
DROP INDEX IF EXISTS idx_model_registry_org;
DROP INDEX IF EXISTS idx_api_keys_org;
DROP INDEX IF EXISTS idx_users_org;

DROP TABLE IF EXISTS audit_log_entries;
DROP TABLE IF EXISTS model_registry_entries;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS organizations;

COMMIT;
