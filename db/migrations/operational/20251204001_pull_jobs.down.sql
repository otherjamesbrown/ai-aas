-- +goose Down
DROP INDEX IF EXISTS idx_pull_jobs_active_unique;
ALTER TABLE pull_jobs DROP CONSTRAINT IF EXISTS chk_pull_jobs_status;
DROP INDEX IF EXISTS idx_pull_jobs_started_at;
DROP INDEX IF EXISTS idx_pull_jobs_status;
DROP INDEX IF EXISTS idx_pull_jobs_model_id;
DROP TABLE IF EXISTS pull_jobs;
