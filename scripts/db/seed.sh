#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.."
PACKAGE="local-dev"
COMPONENT="operational"
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--package <name>] [--component <operational|analytics>] [--env <env-file>]

Packages:
  local-dev (default)
Components:
  operational | analytics
USAGE
}

while (($# > 0)); do
  case "$1" in
    --package)
      [[ $# -lt 2 ]] && { echo "--package requires a value" >&2; exit 1; }
      PACKAGE="$2"
      shift 2
      ;;
    --component)
      [[ $# -lt 2 ]] && { echo "--component requires a value" >&2; exit 1; }
      COMPONENT="$2"
      shift 2
      ;;
    --env)
      [[ $# -lt 2 ]] && { echo "--env requires a value" >&2; exit 1; }
      ENV_FILE="$2"
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

if [[ "$PACKAGE" != "local-dev" ]]; then
  echo "[ERROR] Unsupported seed package: $PACKAGE" >&2
  exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
  FALLBACK="$ROOT_DIR/configs/migrate.example.env"
  if [[ -f "$FALLBACK" ]]; then
    echo "[WARN] $ENV_FILE not found; using example env at $FALLBACK" >&2
    ENV_FILE="$FALLBACK"
  else
    echo "[ERROR] Environment file not found (expected $ENV_FILE or fallback $FALLBACK)" >&2
    exit 1
  fi
fi

set -a
source "$ENV_FILE"
set +a

case "$COMPONENT" in
  operational)
    exec env GOWORK=off go run "$ROOT_DIR/db/seeds/operational"
    ;;
  analytics)
    if [[ -z "${ANALYTICS_URL:-}" ]]; then
      echo "[ERROR] ANALYTICS_URL must be set in environment to seed analytics" >&2
      exit 1
    fi
    if command -v psql >/dev/null 2>&1; then
      psql "$ANALYTICS_URL" -X -v ON_ERROR_STOP=1 -f "$ROOT_DIR/db/seeds/analytics/seed.sql"
    else
      ( cd "$ROOT_DIR/db/tools/sqlrunner" && GOWORK=off DB_URL="$ANALYTICS_URL" go run . --file "$ROOT_DIR/db/seeds/analytics/seed.sql" )
    fi
    ;;
  *)
    echo "[ERROR] Unsupported component: $COMPONENT" >&2
    exit 1
    ;;
 esac
