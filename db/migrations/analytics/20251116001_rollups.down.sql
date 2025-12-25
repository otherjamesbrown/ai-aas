BEGIN;

DROP INDEX IF EXISTS idx_daily_rollups_org;
DROP INDEX IF EXISTS idx_hourly_rollups_org;
DROP TABLE IF EXISTS analytics_daily_rollups;
DROP TABLE IF EXISTS analytics_hourly_rollups;

COMMIT;
