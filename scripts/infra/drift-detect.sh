#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

ENVIRONMENT=""
DRIFT_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)
      ENVIRONMENT="$2"
      shift 2
      ;;
    --drift-arg)
      DRIFT_ARGS+=("$2")
      shift 2
      ;;
    --help|-h)
      cat <<USAGE
Usage: $(basename "$0") [--env <name>] [--drift-arg <value>]...

Runs terraform plan -refresh-only via make infra-drift.
USAGE
      exit 0
      ;;
    *)
      warn "Unknown option: $1"
      exit 1
      ;;
  esac
done

ENVIRONMENT="$(resolve_environment "$ENVIRONMENT")"
log "Detecting drift for ENV=$ENVIRONMENT"
run_make infra-drift ENV="$ENVIRONMENT" DRIFT_ARGS="${DRIFT_ARGS[*]:-}"
