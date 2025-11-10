#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

show_help() {
  cat <<USAGE
Usage: $(basename "$0") --plan-file <path> [--env <name>] [--auto-approve]

Applies a previously generated rollback plan (terraform plan -out=<file>). This
script delegates to make infra-apply with the supplied plan file.
USAGE
}

PLAN_FILE=""
ENVIRONMENT=""
AUTO_APPROVE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --plan-file)
      PLAN_FILE="$2"
      shift 2
      ;;
    --env)
      ENVIRONMENT="$2"
      shift 2
      ;;
    --auto-approve)
      AUTO_APPROVE=true
      shift
      ;;
    --help|-h)
      show_help
      exit 0
      ;;
    *)
      warn "Unknown option: $1"
      show_help
      exit 1
      ;;
  esac
done

if [[ -z "$PLAN_FILE" ]]; then
  warn "--plan-file is required"
  show_help
  exit 1
fi

if [[ ! -f "$PLAN_FILE" ]]; then
  warn "Plan file not found: $PLAN_FILE"
  exit 1
fi

ENVIRONMENT="$(resolve_environment "$ENVIRONMENT")"
log "Applying rollback plan $PLAN_FILE for ENV=$ENVIRONMENT"

args=("-input=false" "$PLAN_FILE")
if [[ "$AUTO_APPROVE" == true ]]; then
  args+=("-auto-approve")
fi

run_make infra-apply ENV="$ENVIRONMENT" APPLY_ARGS="${args[*]}"
