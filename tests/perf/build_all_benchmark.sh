#!/usr/bin/env bash
set -euo pipefail

# Benchmarks `make build-all` execution time and records results.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${ROOT_DIR}"

TARGET=${BUILD_ALL_TARGET:-build-all}
REPORT_DIR="tests/perf/reports"
mkdir -p "${REPORT_DIR}"

OUTPUT_FILE="${REPORT_DIR}/$(date +%Y%m%dT%H%M%S)_${TARGET}.txt"

echo "Running ${TARGET} benchmark..."

START_NS=$(date +%s%N)
if ! make "${TARGET}"; then
  echo "make ${TARGET} failed; aborting benchmark" >&2
  exit 1
fi
END_NS=$(date +%s%N)

elapsed_sec=$(awk "BEGIN { printf \"%.2f\", (${END_NS} - ${START_NS}) / 1000000000 }")

{
  echo "target=${TARGET}"
  echo "timestamp=$(date -Iseconds)"
  echo "elapsed_seconds=${elapsed_sec}"
} | tee "${OUTPUT_FILE}"

MAX_SEC=${BUILD_ALL_MAX_SECONDS:-1800} # 30 minutes default
if (( $(awk "BEGIN {print (${elapsed_sec} > ${MAX_SEC})}") )); then
  echo "WARNING: ${TARGET} exceeded ${MAX_SEC}s budget" >&2
  exit 1
fi

echo "Benchmark complete. Report saved to ${OUTPUT_FILE}"

