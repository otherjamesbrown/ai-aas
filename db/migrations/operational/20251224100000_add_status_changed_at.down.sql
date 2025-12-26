DROP TRIGGER IF EXISTS trigger_deployment_status_changed_at ON model_deployments;
DROP FUNCTION IF EXISTS update_deployment_status_changed_at();
ALTER TABLE model_deployments
DROP COLUMN IF EXISTS status_changed_at;
