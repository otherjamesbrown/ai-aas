#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}" )/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
ADAPTER="${ANALYTICS_ADAPTER:-default}"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--env <env-file>] [--adapter <name>]

Executes the hourly rollup transform using DSNs from the specified environment file.
USAGE
}

while (($# > 0)); do
  case "$1" in
    --env)
      [[ $# -lt 2 ]] && { echo "--env requires a value" >&2; exit 1; }
      ENV_FILE="$2"
      shift 2
      ;;
    --adapter)
      [[ $# -lt 2 ]] && { echo "--adapter requires a value" >&2; exit 1; }
      ADAPTER="$2"
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ ! -f "$ENV_FILE" ]]; then
  echo "[ERROR] Environment file not found: $ENV_FILE" >&2
  exit 1
fi

set -a
source "$ENV_FILE"
set +a

DB_DSN=${DB_URL:-}
if [[ -z "$DB_DSN" ]]; then
  echo "[ERROR] DB_URL must be set in $ENV_FILE" >&2
  exit 1
fi

python_bin=${PYTHON:-python3}

if [[ -z "${ROLLUP_START:-}" ]]; then
  START_WINDOW=$($python_bin - <<'PY'
import datetime
now = datetime.datetime.utcnow()
start = (now - datetime.timedelta(hours=1)).replace(minute=0, second=0, microsecond=0)
print(start.isoformat() + 'Z')
PY
)
else
  START_WINDOW="$ROLLUP_START"
fi

if [[ -z "${ROLLUP_END:-}" ]]; then
  END_WINDOW=$($python_bin - <<'PY'
import datetime
now = datetime.datetime.utcnow()
end = now.replace(minute=0, second=0, microsecond=0)
print(end.isoformat() + 'Z')
PY
)
else
  END_WINDOW="$ROLLUP_END"
fi

TMP_SQL=$(mktemp)
trap 'rm -f "$TMP_SQL"' EXIT
cat > "$TMP_SQL" <<SQL
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
WHERE occurred_at >= '$START_WINDOW'::timestamptz AND occurred_at < '$END_WINDOW'::timestamptz
GROUP BY 1,2,3
ON CONFLICT (bucket_start, organization_id, model_id)
DO UPDATE SET request_count = EXCLUDED.request_count,
              tokens_total  = EXCLUDED.tokens_total,
              error_count   = EXCLUDED.error_count,
              cost_total    = EXCLUDED.cost_total,
              updated_at    = NOW();
SQL

( cd "$ROOT_DIR/db/tools/sqlrunner" && GOWORK=off DB_URL="$DB_DSN" go run . --file "$TMP_SQL" )
