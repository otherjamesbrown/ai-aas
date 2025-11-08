#!/usr/bin/env bash
set -euo pipefail

# Uploads metric JSON artifacts to S3-compatible storage.

usage() {
  cat <<'EOF'
Usage: scripts/metrics/upload.sh [--dry-run] [--bucket <name>] [--prefix <path>] <file>
Defaults read from:
  METRICS_BUCKET (required unless --bucket provided)
  METRICS_ENDPOINT (optional custom endpoint)
EOF
}

DRY_RUN=false
BUCKET="${METRICS_BUCKET:-}"
PREFIX="${METRICS_PREFIX:-metrics}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --bucket)
      BUCKET="$2"
      shift 2
      ;;
    --prefix)
      PREFIX="$2"
      shift 2
      ;;
    -*)
      usage
      exit 1
      ;;
    *)
      FILE="$1"
      shift
      ;;
  esac
done

if [[ -z "${FILE:-}" ]]; then
  echo "missing metric file argument" >&2
  usage
  exit 1
fi

if [[ -z "${BUCKET}" ]]; then
  echo "METRICS_BUCKET not set and --bucket not provided" >&2
  exit 1
fi

if [[ ! -f "${FILE}" ]]; then
  echo "metric file ${FILE} does not exist" >&2
  exit 1
fi

TIMESTAMP=$(date -Iseconds)
RUN_ID=$(basename "${FILE}" .json)
DEST_PATH="${PREFIX}/$(date +%Y/%m/%d)/${RUN_ID}.json"

echo "Uploading ${FILE} to ${BUCKET}/${DEST_PATH}"

if [[ "${DRY_RUN}" == "true" ]]; then
  echo "Dry run enabled; skipping upload."
  exit 0
fi

if command -v aws >/dev/null 2>&1; then
  aws s3 cp "${FILE}" "s3://${BUCKET}/${DEST_PATH}"
elif command -v mc >/dev/null 2>&1; then
  if [[ -z "${METRICS_ENDPOINT:-}" ]]; then
    echo "METRICS_ENDPOINT must be set when using MinIO client" >&2
    exit 1
  fi
  mc alias set metrics "${METRICS_ENDPOINT}" "${METRICS_ACCESS_KEY:-}" "${METRICS_SECRET_KEY:-}" --api S3v4 >/dev/null
  mc cp "${FILE}" "metrics/${BUCKET}/${DEST_PATH}"
else
  echo "Neither aws CLI nor mc (MinIO) available for upload" >&2
  exit 1
fi

echo "Upload complete at ${TIMESTAMP}"

