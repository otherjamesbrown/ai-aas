#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/common.sh"

ENVIRONMENT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env)
      ENVIRONMENT="$2"
      shift 2
      ;;
    --help|-h)
      cat <<USAGE
Usage: $(basename "$0") [--env <name>]

Runs terraform validate, tflint, and tfsec for the specified environment.
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
ENV_DIR="$REPO_ROOT/infra/terraform/environments/$ENVIRONMENT"
TFLINT_CONFIG="$REPO_ROOT/infra/terraform/.tflint.hcl"
TFSEC_CONFIG="$REPO_ROOT/infra/terraform/.tfsec.yaml"

if [[ ! -d "$ENV_DIR" ]]; then
  warn "Environment directory not found: $ENV_DIR"
  exit 1
fi

log "Running terraform validate for ENV=$ENVIRONMENT"
run_make infra-validate ENV="$ENVIRONMENT"

if command -v tflint >/dev/null 2>&1; then
  log "Running tflint"
  (cd "$ENV_DIR" && tflint --config "$TFLINT_CONFIG" --recursive)
else
  warn "tflint not found; skipping lint step"
fi

if command -v tfsec >/dev/null 2>&1; then
  log "Running tfsec"
  (cd "$REPO_ROOT/infra/terraform" && tfsec --config-file "$TFSEC_CONFIG" "environments/$ENVIRONMENT")
else
  warn "tfsec not found; skipping security scan"
fi
