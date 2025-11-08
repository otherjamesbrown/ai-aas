#!/usr/bin/env bash
set -euo pipefail

# Triggers GitHub Actions workflow_dispatch for remote CI execution.

SERVICE="${SERVICE:-all}"
REF="${REF:-$(git rev-parse HEAD)}"
NOTES="${NOTES:-remote execution}"
WORKFLOW="${CI_REMOTE_WORKFLOW:-ci-remote.yml}"

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI (gh) is required." >&2
  exit 1
fi

readarray -t STATUS < <(gh auth status 2>&1 || true)
if printf '%s\n' "${STATUS[@]}" | grep -q "You are not logged into any GitHub hosts"; then
  echo "GitHub CLI not authenticated; run 'gh auth login' with workflow scope." >&2
  exit 1
fi

echo "Dispatching workflow ${WORKFLOW} for ref ${REF} (service=${SERVICE})"

RUN_JSON=$(gh workflow run "${WORKFLOW}" \
  --ref "${REF}" \
  --field service="${SERVICE}" \
  --field notes="${NOTES}" \
  --json run)

RUN_ID=$(echo "${RUN_JSON}" | jq -r '.id' 2>/dev/null || true)
RUN_URL=$(echo "${RUN_JSON}" | jq -r '.url' 2>/dev/null || true)

echo "Workflow queued: ${RUN_URL:-unknown}"

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

