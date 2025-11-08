#!/usr/bin/env bash
set -euo pipefail

# measures execution time of `make help` and verifies it completes within 1 second.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

if ! command -v make >/dev/null 2>&1; then
  echo "make command not found; install GNU Make 4.x+" >&2
  exit 1
fi

TMP_LOG="$(mktemp)"
cleanup() { rm -f "${TMP_LOG}"; }
trap cleanup EXIT

START_NS=$(date +%s%N)
if ! make help | tee "${TMP_LOG}"; then
  echo "make help failed; investigate output above" >&2
  exit 1
fi
END_NS=$(date +%s%N)

elapsed_ms=$(( (END_NS - START_NS) / 1000000 ))
echo "make help elapsed: ${elapsed_ms} ms"

MAX_MS=${HELP_MAX_MS:-1000}
if (( elapsed_ms > MAX_MS )); then
  echo "FAIL: make help exceeded ${MAX_MS} ms threshold" >&2
  exit 1
fi

echo "PASS: make help meets ${MAX_MS} ms target"

