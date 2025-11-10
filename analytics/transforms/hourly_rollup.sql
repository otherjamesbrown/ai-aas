WITH bounds AS (
    SELECT
        '{{START_WINDOW}}'::timestamptz AS start_window,
        '{{END_WINDOW}}'::timestamptz AS end_window
)
INSERT INTO analytics_hourly_rollups (
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
    date_trunc('hour', occurred_at) AS bucket_start,
    organization_id,
    model_id,
    COUNT(*) AS request_count,
    SUM(tokens_consumed) AS tokens_total,
    SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
    SUM(cost_usd) AS cost_total,
    NOW() AS updated_at
FROM usage_events, bounds
WHERE occurred_at >= bounds.start_window AND occurred_at < bounds.end_window
GROUP BY 1, 2, 3
ON CONFLICT (bucket_start, organization_id, model_id)
DO UPDATE SET
    request_count = EXCLUDED.request_count,
    tokens_total  = EXCLUDED.tokens_total,
    error_count   = EXCLUDED.error_count,
    cost_total    = EXCLUDED.cost_total,
    updated_at    = NOW();
