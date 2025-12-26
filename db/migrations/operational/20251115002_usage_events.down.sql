-- Drop usage event fact table
BEGIN;

DROP INDEX IF EXISTS idx_usage_events_status;
DROP INDEX IF EXISTS idx_usage_events_model_time;
DROP INDEX IF EXISTS idx_usage_events_org_time;

DROP TABLE IF EXISTS usage_events;

COMMIT;
