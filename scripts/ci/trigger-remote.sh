#!/usr/bin/env bash
set -euo pipefail

# Triggers GitHub Actions workflow_dispatch for remote CI execution.

SERVICE="${SERVICE:-all}"
DEFAULT_REF="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
if [[ -z "${DEFAULT_REF}" || "${DEFAULT_REF}" == "HEAD" ]]; then
  DEFAULT_REF="$(git rev-parse HEAD 2>/dev/null || true)"
fi
REF="${REF:-${DEFAULT_REF}}"
NOTES="${NOTES:-remote execution}"
WORKFLOW="${CI_REMOTE_WORKFLOW:-ci-remote.yml}"

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) is required." >&2
  exit 1
fi

STATUS="$(gh auth status 2>&1 || true)"
if printf '%s\n' "${STATUS}" | grep -q "You are not logged into any GitHub hosts"; then
  echo "GitHub CLI not authenticated; run 'gh auth login' with workflow scope." >&2
  exit 1
fi

echo "Dispatching workflow ${WORKFLOW} for ref ${REF} (service=${SERVICE})"

RUN_OUTPUT=$(mktemp)
if ! gh workflow run "${WORKFLOW}" \
  --ref "${REF}" \
  --raw-field service="${SERVICE}" \
  --raw-field notes="${NOTES}" > "${RUN_OUTPUT}"; then
  cat "${RUN_OUTPUT}" >&2
  rm -f "${RUN_OUTPUT}"
  exit 1
fi

RUN_URL=$(grep -Eo "https://github.com/[^\s]+" "${RUN_OUTPUT}" | head -n 1 || true)
rm -f "${RUN_OUTPUT}"

sleep 3

RUN_ID=$(gh run list --workflow "${WORKFLOW}" --limit 1 --json databaseId --jq '.[0].databaseId' 2>/dev/null || true)

if [[ -z "${RUN_URL}" || -z "${RUN_ID}" ]]; then
  RUN_URL=$(gh run view "${RUN_ID}" --json url --jq '.url' 2>/dev/null || true)
fi

if [[ -n "${RUN_URL}" ]]; then
  echo "Workflow queued: ${RUN_URL}"
fi

if [[ -z "${RUN_ID}" ]]; then
  echo "Unable to determine workflow run ID. Check gh run list manually." >&2
  exit 1
fi

if [[ "${CI_REMOTE_WAIT:-true}" != "true" ]]; then
  exit 0
fi

echo "Waiting for workflow completion..."
gh run watch "${RUN_ID}"
RESULT=$(gh run view "${RUN_ID}" --json conclusion,url --jq '.conclusion + " " + .url')
echo "Workflow result: ${RESULT}"

if [[ "${RESULT}" != success* ]]; then
  exit 2
fi

