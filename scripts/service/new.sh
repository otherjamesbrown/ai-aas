#!/usr/bin/env bash
set -euo pipefail

# Generates a new service skeleton using shared templates.

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <service-name>" >&2
  exit 1
fi

SERVICE_NAME="$1"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SERVICE_DIR="${ROOT_DIR}/services/${SERVICE_NAME}"

if [[ -d "${SERVICE_DIR}" ]]; then
  echo "Service ${SERVICE_NAME} already exists" >&2
  exit 1
fi

mkdir -p "${SERVICE_DIR}"

cp -R "${ROOT_DIR}/services/_template/." "${SERVICE_DIR}/"

if [[ -d "${SERVICE_DIR}/cmd/{{SERVICE_NAME}}" ]]; then
  mv "${SERVICE_DIR}/cmd/{{SERVICE_NAME}}" "${SERVICE_DIR}/cmd/${SERVICE_NAME}"
fi

find "${SERVICE_DIR}" -type f \
  \( -name "*.go" -o -name "Makefile" -o -name "*.md" -o -name "go.mod" \) \
  -print0 | xargs -0 sed -i.bak "s/{{SERVICE_NAME}}/${SERVICE_NAME}/g"
sed -i.bak "s/service-template/${SERVICE_NAME}/g" "${SERVICE_DIR}/go.mod"
find "${SERVICE_DIR}" -type f -name "*.bak" -delete

if command -v gofmt >/dev/null 2>&1; then
  GOFMT_FILES=$(find "${SERVICE_DIR}" -type f -name "*.go")
  if [[ -n "${GOFMT_FILES}" ]]; then
    gofmt -w ${GOFMT_FILES}
  fi
fi

echo "Adding ${SERVICE_NAME} to go.work"
go work use "./services/${SERVICE_NAME}" || true

echo "Service ${SERVICE_NAME} scaffolded."
echo "Next steps:"
echo "  - Update services/${SERVICE_NAME}/README.md"
echo "  - Implement service-specific code"

