#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
DEFAULT_ENVIRONMENT="development"

log() {
  printf '[infra] %s\n' "$*"
}

warn() {
  printf '[infra][warn] %s\n' "$*" >&2
}

run_make() {
  (cd "$REPO_ROOT" && make "$@")
}

resolve_environment() {
  local env_from_env_var="${ENV:-}" env_from_arg="$1"
  if [[ -n "$env_from_arg" ]]; then
    echo "$env_from_arg"
  elif [[ -n "$env_from_env_var" ]]; then
    echo "$env_from_env_var"
  else
    echo "$DEFAULT_ENVIRONMENT"
  fi
}
