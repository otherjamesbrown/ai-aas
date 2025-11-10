#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}" )/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
DSN=${DB_URL:-}

if [[ -z "$DSN" ]]; then
  if [[ ! -f "$ENV_FILE" ]]; then
    echo "[ERROR] migrate env not found: $ENV_FILE" >&2
    exit 1
  fi
  DSN=$(grep '^DB_URL=' "$ENV_FILE" | head -1 | cut -d= -f2-)
fi

if [[ -z "$DSN" ]]; then
  echo "[ERROR] DB_URL must be set" >&2
  exit 1
fi

TMP_SQL=$(mktemp)
trap 'rm -f "$TMP_SQL"' EXIT
cat > "$TMP_SQL" <<'SQL'
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
GROUP BY 1,2,3
ON CONFLICT (bucket_start, organization_id, model_id)
DO UPDATE SET request_count = EXCLUDED.request_count,
              tokens_total  = EXCLUDED.tokens_total,
              error_count   = EXCLUDED.error_count,
              cost_total    = EXCLUDED.cost_total,
              updated_at    = NOW();

INSERT INTO analytics_daily_rollups (bucket_start, organization_id, model_id, request_count, tokens_total, error_count, cost_total, updated_at)
SELECT date_trunc('day', occurred_at)::date AS bucket_start,
       organization_id,
       model_id,
       COUNT(*) AS request_count,
       SUM(tokens_consumed) AS tokens_total,
       SUM(CASE WHEN status = 'error' THEN 1 ELSE 0 END) AS error_count,
       SUM(cost_usd) AS cost_total,
       NOW() AS updated_at
FROM usage_events
GROUP BY 1,2,3
ON CONFLICT (bucket_start, organization_id, model_id)
DO UPDATE SET request_count = EXCLUDED.request_count,
              tokens_total  = EXCLUDED.tokens_total,
              error_count   = EXCLUDED.error_count,
              cost_total    = EXCLUDED.cost_total,
              updated_at    = NOW();
SQL

( cd "$ROOT_DIR/db/tools/sqlrunner" && GOWORK=off DB_URL="$DSN" go run . --file "$TMP_SQL" )
( cd "$ROOT_DIR/analytics/tests" && GOWORK=off DB_URL="$DSN" go test ./... )
