#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

show_help() {
  cat <<USAGE
Usage: $(basename "$0") [--env <name>] [--auto-approve] [--apply-arg <value>]...

Runs make infra-apply with optional extra arguments.
USAGE
}

ENVIRONMENT=""
APPLY_ARGS=()
AUTO_APPROVE=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)
      ENVIRONMENT="$2"
      shift 2
      ;;
    --auto-approve)
      AUTO_APPROVE=true
      shift
      ;;
    --apply-arg)
      APPLY_ARGS+=("$2")
      shift 2
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

ENVIRONMENT="$(resolve_environment "$ENVIRONMENT")"
if [[ "$AUTO_APPROVE" == true ]]; then
  APPLY_ARGS+=("-auto-approve")
fi

log "Applying terraform changes for ENV=$ENVIRONMENT"
run_make infra-apply ENV="$ENVIRONMENT" APPLY_ARGS="${APPLY_ARGS[*]:-}" 
