#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
COMPONENT="${MIGRATION_COMPONENT:-operational}"

if [[ ! -f "$ROOT_DIR/db/docs/dictionary.md" ]]; then
  echo "[ERROR] db/docs/dictionary.md missing" >&2
  exit 1
fi

if [[ ! -s "$ROOT_DIR/db/docs/dictionary.md" ]]; then
  echo "[ERROR] db/docs/dictionary.md is empty" >&2
  exit 1
fi

if [[ ! -f "$ROOT_DIR/db/docs/erd/schema.puml" ]]; then
  echo "[ERROR] db/docs/erd/schema.puml missing" >&2
  exit 1
fi

# Always fetch latest status to ensure migrations are accessible.
"$ROOT_DIR/scripts/db/apply.sh" --status --component "$COMPONENT" --env "$ENV_FILE"

if command -v psql >/dev/null 2>&1; then
  echo "[INFO] Capturing schema snapshot via psql (diff comparison not yet automated)"
  TMP_FILE="$(mktemp)"
  trap 'rm -f "$TMP_FILE"' EXIT
  PSQL_URL=$(awk -F= '/^DB_URL=/{print $2}' "$ENV_FILE")
  if [[ -n "$PSQL_URL" ]]; then
    psql "$PSQL_URL" -X -v ON_ERROR_STOP=1 <<SQL >"$TMP_FILE"
SELECT table_name, column_name, data_type
FROM information_schema.columns
WHERE table_schema = 'public'
  AND table_name IN ('organizations','users','api_keys','model_registry_entries','usage_events','audit_log_entries')
ORDER BY table_name, ordinal_position;
SQL
    echo "[INFO] Captured schema snapshot at $TMP_FILE"
  else
    echo "[WARN] DB_URL not set in $ENV_FILE; skipping live schema introspection" >&2
  fi
else
  echo "[WARN] psql not available; quick validation limited to documentation presence" >&2
fi
