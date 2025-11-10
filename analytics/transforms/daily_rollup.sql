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
SELECT date_trunc('day', occurred_at)::date AS bucket_start,
       organization_id,
       model_id,
       COUNT(*) AS request_count,
       SUM(tokens_consumed) AS tokens_total,
       SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
       SUM(cost_usd) AS cost_total,
       NOW() AS updated_at
FROM usage_events
WHERE occurred_at >= $1 AND occurred_at < $2
GROUP BY 1,2,3
ON CONFLICT (bucket_start, organization_id, model_id)
DO UPDATE SET request_count = EXCLUDED.request_count,
              tokens_total  = EXCLUDED.tokens_total,
              error_count   = EXCLUDED.error_count,
              cost_total    = EXCLUDED.cost_total,
              updated_at    = NOW();
