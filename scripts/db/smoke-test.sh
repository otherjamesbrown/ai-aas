#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
COMPONENT="${MIGRATION_COMPONENT:-operational}"
TARGET_VERSION=""

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--component <operational|analytics>] [--version <YYYYMMDDHHMM_slug>] [--env <env-file>]

Runs apply → status → optional rollback → reapply smoke validation.
If --version is provided, rollback/reapply will target the supplied version.
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

APPLY_CMD=("$ROOT_DIR/scripts/db/apply.sh" --component "$COMPONENT" --env "$ENV_FILE" --yes)
if [[ -n "$TARGET_VERSION" ]]; then
  APPLY_CMD+=( --version "$TARGET_VERSION" )
fi

ROLLBACK_CMD=("$ROOT_DIR/scripts/db/rollback.sh" --component "$COMPONENT" --env "$ENV_FILE" --yes)
if [[ -n "$TARGET_VERSION" ]]; then
  ROLLBACK_CMD+=( --version "$TARGET_VERSION" )
fi

STATUS_CMD=("$ROOT_DIR/scripts/db/apply.sh" --status --component "$COMPONENT" --env "$ENV_FILE")

set -x
"${APPLY_CMD[@]}"
"${STATUS_CMD[@]}"

if [[ -n "$TARGET_VERSION" ]]; then
  "${ROLLBACK_CMD[@]}"
  "${APPLY_CMD[@]}"
else
  echo "[INFO] No --version supplied; skipping rollback/reapply step" >&2
fi
set +x
