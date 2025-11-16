#!/usr/bin/env bash
# Helper script for tailing logs from Docker Compose services
# Usage: ./scripts/dev/log-tail.sh [SERVICE=<name>] [--follow] [--since=<time>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
COMPOSE_DIR="${PROJECT_ROOT}/.dev/compose"

# Default values
SERVICE="${SERVICE:-}"
FOLLOW="${FOLLOW:-true}"
SINCE="${SINCE:-}"
TAIL="${TAIL:-100}"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-follow|--no-f)
      FOLLOW="false"
      shift
      ;;
    --follow|--f)
      FOLLOW="true"
      shift
      ;;
    --since=*)
      SINCE="${1#*=}"
      shift
      ;;
    --tail=*)
      TAIL="${1#*=}"
      shift
      ;;
    SERVICE=*)
      SERVICE="${1#*=}"
      shift
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

# Check Docker Compose
if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker command not found" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "Error: docker compose command not found" >&2
  exit 1
fi

# Build docker compose command
COMPOSE_CMD="docker compose -f ${COMPOSE_DIR}/compose.base.yaml -f ${COMPOSE_DIR}/compose.local.yaml"

# Build logs command
LOGS_CMD="${COMPOSE_CMD} logs"
if [[ "${FOLLOW}" == "true" ]]; then
  LOGS_CMD="${LOGS_CMD} --follow"
fi

if [[ -n "${SINCE}" ]]; then
  LOGS_CMD="${LOGS_CMD} --since=${SINCE}"
fi

LOGS_CMD="${LOGS_CMD} --tail=${TAIL}"

# Add service if specified
if [[ -n "${SERVICE}" ]]; then
  LOGS_CMD="${LOGS_CMD} ${SERVICE}"
fi

# Execute
exec ${LOGS_CMD}

