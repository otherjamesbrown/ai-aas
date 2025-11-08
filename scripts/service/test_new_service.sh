#!/usr/bin/env bash
set -euo pipefail

# Basic smoke test for service generator.

TEMP_DIR=$(mktemp -d)
cleanup() { rm -rf "${TEMP_DIR}"; }
trap cleanup EXIT

cp -R . "${TEMP_DIR}/repo"
cd "${TEMP_DIR}/repo"

SERVICE_NAME="smoke-service"
scripts/service/new.sh "${SERVICE_NAME}"

if [[ ! -f "services/${SERVICE_NAME}/Makefile" ]]; then
  echo "Makefile not created for ${SERVICE_NAME}" >&2
  exit 1
fi

if ! make -C "services/${SERVICE_NAME}" build >/dev/null 2>&1; then
  echo "make build failed for ${SERVICE_NAME}" >&2
  exit 1
fi

echo "Service generator smoke test passed."

