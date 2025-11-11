#!/usr/bin/env bash
set -euo pipefail

SERVICE_URL="${SERVICE_URL:-http://localhost:8080}"
TIMEOUT="${TIMEOUT:-30}"

echo ">> Waiting for service at ${SERVICE_URL}"
for _ in $(seq 1 "${TIMEOUT}"); do
  if curl -fsS "${SERVICE_URL}/healthz" >/dev/null; then
    echo "Health check passed"
    break
  fi
  sleep 1
done

RESPONSE="$(curl -fsS "${SERVICE_URL}/info")"
echo "Info response: ${RESPONSE}"

