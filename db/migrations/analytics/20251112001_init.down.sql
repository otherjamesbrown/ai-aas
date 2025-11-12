-- Rollback initial analytics schema
BEGIN;

DROP INDEX IF EXISTS analytics.idx_freshness_status_status;
DROP TABLE IF EXISTS analytics.freshness_status;

DROP INDEX IF EXISTS analytics.idx_ingestion_batches_late;
DROP INDEX IF EXISTS analytics.idx_ingestion_batches_completed;
DROP TABLE IF EXISTS analytics.ingestion_batches;

DROP INDEX IF EXISTS analytics.idx_usage_events_batch;
DROP INDEX IF EXISTS analytics.idx_usage_events_status;
DROP INDEX IF EXISTS analytics.idx_usage_events_org_model_time;
-- Note: Hypertable drop handled automatically when table is dropped
DROP TABLE IF EXISTS analytics.usage_events;

-- Note: We don't drop the schema or extension as they may be used by other objects

COMMIT;

