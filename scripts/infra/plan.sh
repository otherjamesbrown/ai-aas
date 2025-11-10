#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

show_help() {
  cat <<USAGE
Usage: $(basename "$0") [--env <name>] [--plan-arg <value>]...

Runs \
  make infra-plan ENV=<env> PLAN_ARGS="<aggregated args>"

Options:
  --env <name>       Environment to target (default: \
$(resolve_environment ""))
  --plan-arg <arg>   Additional argument passed to \
                     terraform plan (can be used multiple times)
  --help             Display this help message
USAGE
}

ENVIRONMENT=""
PLAN_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)
      ENVIRONMENT="$2"
      shift 2
      ;;
    --plan-arg)
      PLAN_ARGS+=("$2")
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
log "Running terraform plan for ENV=$ENVIRONMENT"

run_make infra-plan ENV="$ENVIRONMENT" PLAN_ARGS="${PLAN_ARGS[*]:-}" 
