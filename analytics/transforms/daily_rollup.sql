-- Daily rollup: aggregate usage_events into daily buckets
-- Uses DELETE + INSERT pattern to avoid ON CONFLICT expression limitations
BEGIN;

WITH bounds AS (
    SELECT
        '{{START_WINDOW}}'::timestamptz AS start_window,
        '{{END_WINDOW}}'::timestamptz AS end_window
)
-- Delete existing rollups for this time window (will be re-aggregated)
DELETE FROM analytics_daily_rollups
WHERE bucket_start >= (SELECT start_window::date FROM bounds)
  AND bucket_start < (SELECT end_window::date FROM bounds);

WITH bounds AS (
    SELECT
        '{{START_WINDOW}}'::timestamptz AS start_window,
        '{{END_WINDOW}}'::timestamptz AS end_window
)
INSERT INTO analytics_daily_rollups (
    bucket_start,
    organization_id,
    model_id,
    request_count,
    tokens_total,
    error_count,
    cost_total,
    updated_at
)
SELECT
    date_trunc('day', occurred_at)::date AS bucket_start,
    organization_id,
    model_id,
    COUNT(*) AS request_count,
    SUM(tokens_consumed) AS tokens_total,
    SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
    SUM(cost_usd) AS cost_total,
    NOW() AS updated_at
FROM usage_events, bounds
WHERE occurred_at >= bounds.start_window AND occurred_at < bounds.end_window
GROUP BY 1, 2, 3;

COMMIT;
