#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
COMPONENT="${MIGRATION_COMPONENT:-operational}"
TARGET_VERSION=""
MEASURE_ROLLBACK=0

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--component <operational|analytics>] [--version <YYYYMMDDHHMM_slug>] [--env <env-file>] [--rollback]

Measures migration apply (and optional rollback) duration, failing if either exceeds 600 seconds.
USAGE
}

while (($# > 0)); do
  case "$1" in
    --component)
      [[ $# -lt 2 ]] && { echo "--component requires a value" >&2; exit 1; }
      COMPONENT="$2"
      shift 2
      ;;
    --version)
      [[ $# -lt 2 ]] && { echo "--version requires a value" >&2; exit 1; }
      TARGET_VERSION="$2"
      shift 2
      ;;
    --env)
      [[ $# -lt 2 ]] && { echo "--env requires a value" >&2; exit 1; }
      ENV_FILE="$2"
      shift 2
      ;;
    --rollback)
      MEASURE_ROLLBACK=1
      shift
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

apply_cmd=("$ROOT_DIR/scripts/db/apply.sh" --component "$COMPONENT" --env "$ENV_FILE" --yes)
if [[ -n "$TARGET_VERSION" ]]; then
  apply_cmd+=( --version "$TARGET_VERSION" )
fi

start=$(date +%s)
MIGRATION_REQUIRE_CONFIRMATION=0 "${apply_cmd[@]}"
end=$(date +%s)
apply_duration=$(( end - start ))

echo "apply_duration_seconds=$apply_duration"
if (( apply_duration > 600 )); then
  echo "[ERROR] Apply duration exceeded 600 seconds" >&2
  exit 1
fi

duration_ok=0

if (( MEASURE_ROLLBACK == 1 )); then
  if [[ -z "$TARGET_VERSION" ]]; then
    echo "[WARN] --rollback requires --version; skipping rollback measurement" >&2
  else
    rollback_cmd=("$ROOT_DIR/scripts/db/rollback.sh" --component "$COMPONENT" --env "$ENV_FILE" --version "$TARGET_VERSION" --yes)
    start=$(date +%s)
    MIGRATION_REQUIRE_CONFIRMATION=0 "${rollback_cmd[@]}"
    end=$(date +%s)
    rollback_duration=$(( end - start ))
    echo "rollback_duration_seconds=$rollback_duration"
    if (( rollback_duration > 600 )); then
      echo "[ERROR] Rollback duration exceeded 600 seconds" >&2
      exit 1
    fi
    duration_ok=1

    # Re-apply to restore state post measurement
    start=$(date +%s)
    MIGRATION_REQUIRE_CONFIRMATION=0 "${apply_cmd[@]}"
    end=$(date +%s)
    echo "reapply_duration_seconds=$(( end - start ))"
  fi
fi

if (( duration_ok == 0 )); then
  echo "[INFO] Rollback duration not measured (use --rollback --version <id> to include)"
fi
