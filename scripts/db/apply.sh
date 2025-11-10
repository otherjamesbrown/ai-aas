#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/.."
ENV_FILE="${MIGRATION_ENV_FILE:-$ROOT_DIR/migrate.env}"
COMPONENT="${MIGRATION_COMPONENT:-operational}"
TARGET_VERSION=""
STATUS_ONLY=0
DRY_RUN=${MIGRATION_DRY_RUN:-0}
AUTO_APPROVE=${MIGRATION_ASSUME_YES:-0}
APPROVED_BY="${MIGRATION_APPROVED_BY:-}"

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--component <operational|analytics>] [--version <YYYYMMDDHHMM_slug>] [--env <env-file>] [--status] [--dry-run] [--yes]

Options:
  --component    Target component (defaults to MIGRATION_COMPONENT or operational)
  --version      Optional target version to migrate up to
  --env          Path to environment file (defaults to migrate.env, falls back to configs/migrate.example.env)
  --status       Only report migration status without applying new changes
  --dry-run      Execute migrations in dry-run mode (no apply/commit)
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
    --status)
      STATUS_ONLY=1
      shift
      ;;
    --dry-run)
      DRY_RUN=1
      shift
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

CMD=(go run . -component "$COMPONENT")

if [[ $STATUS_ONLY -eq 1 ]]; then
  CMD+=( -status )
else
  CMD+=( -direction up )
  if [[ -n "$TARGET_VERSION" ]]; then
    CMD+=( -version "$TARGET_VERSION" )
  fi
  if [[ $DRY_RUN -eq 1 ]]; then
    CMD+=( -dry-run )
  fi

  REQUIRE_CONFIRMATION=${MIGRATION_REQUIRE_CONFIRMATION:-1}
  REQUIRE_APPROVAL=${MIGRATION_REQUIRE_APPROVAL:-1}
  if [[ $REQUIRE_APPROVAL -ne 0 && $APPROVED_BY == "" && $DRY_RUN -eq 0 ]]; then
    echo "[ERROR] MIGRATION_APPROVED_BY must be set (dual approval required)" >&2
    exit 1
  fi

  if [[ $DRY_RUN -eq 0 && $REQUIRE_CONFIRMATION -ne 0 && $AUTO_APPROVE -ne 1 ]]; then
    read -r -p "Apply migrations to component '$COMPONENT'? (y/N) " response
    case "$response" in
      [yY][eE][sS]|[yY]) ;;
      *) echo "Aborted."; exit 1 ;;
    esac
  fi
fi

( cd "$ROOT_DIR/db/tools/migrate" && GOWORK=off MIGRATION_APPROVED_BY="$APPROVED_BY" "${CMD[@]}" )
