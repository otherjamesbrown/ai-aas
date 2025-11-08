#!/usr/bin/env bash
set -euo pipefail

# Wrapper for running GitHub Actions workflows locally via `act`.

WORKFLOW="${WORKFLOW:-ci}"
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

if ! command -v act >/dev/null 2>&1; then
  echo "act executable not found. Install via https://github.com/nektos/act" >&2
  exit 1
fi

WORKFLOW_FILE="${WORKFLOW_FILE:-.github/workflows/${WORKFLOW}.yml}"
IMAGE="${ACT_IMAGE:-ghcr.io/catthehacker/ubuntu:act-latest}"
CONTAINER_ARCH="${ACT_ARCH:-linux/amd64}"
CACHE_DIR="${ACT_CACHE_DIR:-${HOME}/.cache/act}"
PULL_IMAGES="${ACT_PULL:-true}"
REUSE_WORKSPACE="${ACT_REUSE:-false}"

mkdir -p "${CACHE_DIR}"

ACT_ARGS=()
if [[ "${PULL_IMAGES}" == "true" ]]; then
  ACT_ARGS+=(--pull)
fi
if [[ "${REUSE_WORKSPACE}" == "true" ]]; then
  ACT_ARGS+=(--reuse)
fi
if [[ -n "${CACHE_DIR}" ]]; then
  ACT_ARGS+=(--bind "${CACHE_DIR}:/tmp/act-cache")
fi

echo "Running workflow '${WORKFLOW}' using act image ${IMAGE}"
echo "  workflow file : ${WORKFLOW_FILE}"
echo "  architecture  : ${CONTAINER_ARCH}"
echo "  cache dir     : ${CACHE_DIR}"

act workflow \
  -W "${WORKFLOW_FILE}" \
  --container-architecture "${CONTAINER_ARCH}" \
  --actor "${ACTOR:-dev@example.com}" \
  --image "${IMAGE}" \
  "${ACT_ARGS[@]}" \
  "$@"

