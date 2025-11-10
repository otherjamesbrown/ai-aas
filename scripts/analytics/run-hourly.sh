#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}" )/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
ADAPTER="${ANALYTICS_ADAPTER:-default}"
PERIOD="hourly"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--env <env-file>] [--adapter <name>] [--period <hourly|daily>]

Executes the analytics rollup transform using DSNs from the specified environment file.
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
    --period)
      [[ $# -lt 2 ]] && { echo "--period requires a value" >&2; exit 1; }
      PERIOD=$(echo "$2" | tr '[:upper:]' '[:lower:]')
      if [[ "$PERIOD" != "hourly" && "$PERIOD" != "daily" ]]; then
        echo "[ERROR] --period must be hourly or daily" >&2
        exit 1
      fi
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
if ! command -v "$python_bin" >/dev/null 2>&1; then
  if [[ -z "${ROLLUP_START:-}" || -z "${ROLLUP_END:-}" ]]; then
    echo "[ERROR] $python_bin not found; set ROLLUP_START and ROLLUP_END or install python3" >&2
    exit 1
  fi
fi

if [[ -z "${ROLLUP_START:-}" || -z "${ROLLUP_END:-}" ]]; then
  START_WINDOW=$("$python_bin" - <<PY
import datetime
period = "${PERIOD}"
now = datetime.datetime.utcnow()
if period == "daily":
    end = now.replace(hour=0, minute=0, second=0, microsecond=0)
    start = end - datetime.timedelta(days=1)
else:
    end = now.replace(minute=0, second=0, microsecond=0)
    start = end - datetime.timedelta(hours=1)
print(start.isoformat() + 'Z')
PY
)
  END_WINDOW=$("$python_bin" - <<PY
import datetime
period = "${PERIOD}"
now = datetime.datetime.utcnow()
if period == "daily":
    end = now.replace(hour=0, minute=0, second=0, microsecond=0)
else:
    end = now.replace(minute=0, second=0, microsecond=0)
print(end.isoformat() + 'Z')
PY
)
else
  START_WINDOW="$ROLLUP_START"
  END_WINDOW="$ROLLUP_END"
fi

SQL_FILE="$ROOT_DIR/analytics/transforms/${PERIOD}_rollup.sql"
if [[ ! -f "$SQL_FILE" ]]; then
  echo "[ERROR] SQL transform not found for period $PERIOD at $SQL_FILE" >&2
  exit 1
fi

( cd "$ROOT_DIR/db/tools/sqlrunner" && GOWORK=off DB_URL="$DB_DSN" go run . --file "$SQL_FILE" --param START_WINDOW="$START_WINDOW" --param END_WINDOW="$END_WINDOW" )
