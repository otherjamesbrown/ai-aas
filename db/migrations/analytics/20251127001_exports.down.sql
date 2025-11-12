-- Rollback export jobs table
BEGIN;

DROP INDEX IF EXISTS analytics.idx_export_jobs_status_pending_running;
DROP INDEX IF EXISTS analytics.idx_export_jobs_org_initiated;
DROP TABLE IF EXISTS analytics.export_jobs;

-- Drop ENUM types
DROP TYPE IF EXISTS analytics.export_job_granularity;
DROP TYPE IF EXISTS analytics.export_job_status;

COMMIT;

