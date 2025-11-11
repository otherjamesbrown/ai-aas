package perf

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestHourlyRollupDuration(t *testing.T) {
	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set; skipping rollup duration test")
	}

	startWindow := time.Now().Add(-1 * time.Hour).Truncate(time.Hour)
	endWindow := startWindow.Add(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	start := time.Now()
	if _, err := conn.Exec(ctx, `
INSERT INTO analytics_hourly_rollups (bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at)
SELECT date_trunc('hour', occurred_at) AS bucket_start,
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
`, startWindow, endWindow); err != nil {
		t.Fatalf("rollup exec: %v", err)
	}
	duration := time.Since(start)
	if duration > 5*time.Minute {
		t.Fatalf("rollup exceeded duration threshold: %s", duration)
	}
}
