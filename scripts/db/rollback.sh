#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
COMPONENT="${MIGRATION_COMPONENT:-operational}"
TARGET_VERSION=""
AUTO_APPROVE=${MIGRATION_ASSUME_YES:-0}

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--component <operational|analytics>] --version <YYYYMMDDHHMM_slug> [--env <env-file>] [--yes]

Options:
  --component    Target component (defaults to MIGRATION_COMPONENT or operational)
  --version      Version to roll back (required)
  --env          Path to environment file (defaults to migrate.env, falls back to configs/migrate.example.env)
  --yes          Skip approval prompt
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
    --yes)
      AUTO_APPROVE=1
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

if [[ -z "$TARGET_VERSION" ]]; then
  echo "--version is required" >&2
  usage
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

CMD=(go run . -component "$COMPONENT" -direction down -version "$TARGET_VERSION")

REQUIRE_CONFIRMATION=${MIGRATION_REQUIRE_CONFIRMATION:-1}
if [[ $REQUIRE_CONFIRMATION -ne 0 && $AUTO_APPROVE -ne 1 ]]; then
  read -r -p "Rollback migrations for component '$COMPONENT' version '$TARGET_VERSION'? (y/N) " response
  case "$response" in
    [yY][eE][sS]|[yY]) ;;
    *) echo "Aborted."; exit 1 ;;
  esac
fi

( cd "$ROOT_DIR/db/tools/migrate" && GOWORK=off "${CMD[@]}" )
