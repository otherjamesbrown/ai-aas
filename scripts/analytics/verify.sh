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

MIGRATION_ENV_FILE="$ENV_FILE" DB_URL="$DSN" "$ROOT_DIR/scripts/analytics/run-hourly.sh" --period hourly
MIGRATION_ENV_FILE="$ENV_FILE" DB_URL="$DSN" "$ROOT_DIR/scripts/analytics/run-hourly.sh" --period daily
( cd "$ROOT_DIR/analytics/tests" && GOWORK=off DB_URL="$DSN" go test ./... )
