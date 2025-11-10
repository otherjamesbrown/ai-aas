package tests

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

type rollupTestConfig struct {
	table            string
	intervalUnit     string
	defaultWindow    int
	windowEnvVar     string
	truncExpression  string
	resultIdentifier string
}

func (cfg rollupTestConfig) Run(t *testing.T) {
	t.Helper()

	dsn := os.Getenv("DB_URL")
	if dsn == "" {
		t.Skip("DB_URL not set; skipping")
	}

	window := cfg.defaultWindow
	if cfg.windowEnvVar != "" {
		if v := os.Getenv(cfg.windowEnvVar); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 {
				window = parsed
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	query := fmt.Sprintf(`
WITH usage AS (
  SELECT %s AS bucket_start,
         organization_id,
         model_id,
         COUNT(*) AS request_count,
         SUM(tokens_consumed) AS tokens_total,
         SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
         SUM(cost_usd) AS cost_total
  FROM usage_events
  WHERE occurred_at >= NOW() - INTERVAL '%d %s'
  GROUP BY 1,2,3
),
rollup AS (
  SELECT bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total
  FROM %s
  WHERE bucket_start >= NOW() - INTERVAL '%d %s'
)
SELECT usage.bucket_start,
       usage.organization_id,
       usage.model_id
FROM usage
LEFT JOIN rollup
  ON usage.bucket_start = rollup.bucket_start
 AND usage.organization_id = rollup.organization_id
 AND (usage.model_id IS NOT DISTINCT FROM rollup.model_id)
WHERE rollup.bucket_start IS NULL
   OR usage.request_count <> rollup.request_count
   OR usage.tokens_total <> rollup.tokens_total
   OR usage.error_count <> rollup.error_count
   OR usage.cost_total <> rollup.cost_total;`,
		cfg.truncExpression,
		window,
		cfg.intervalUnit,
		cfg.table,
		window,
		cfg.intervalUnit,
	)

	rows, err := conn.Query(ctx, query)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	if rows.Next() {
		t.Fatalf(cfg.resultIdentifier)
	}
}
