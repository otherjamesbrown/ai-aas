#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}" )/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
COMPONENT="${MIGRATION_COMPONENT:-operational}"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--component <operational|analytics>] [--env <env-file>]

Reports the latest migration status for the specified component.
USAGE
}

while (($# > 0)); do
  case "$1" in
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

if [[ ! -f "$ENV_FILE" ]]; then
  FALLBACK="$ROOT_DIR/configs/migrate.example.env"
  if [[ -f "$FALLBACK" ]]; then
    echo "[WARN] $ENV_FILE not found; falling back to $FALLBACK" >&2
    ENV_FILE="$FALLBACK"
  else
    echo "[ERROR] Environment file not found (expected $ENV_FILE or fallback $FALLBACK)" >&2
    exit 1
  fi
fi

set -a
source "$ENV_FILE"
set +a

( cd "$ROOT_DIR/db/tools/migrate" && GOWORK=off go run . --component "$COMPONENT" --status )
