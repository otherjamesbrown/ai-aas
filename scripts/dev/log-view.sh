#!/usr/bin/env bash
# Helper script for viewing logs via Loki/Grafana
# Usage: ./scripts/dev/log-view.sh [SERVICE=<name>] [--loki-url=<url>]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"

# Default values
SERVICE="${SERVICE:-}"
LOKI_URL="${LOKI_URL:-http://localhost:3100}"
GRAFANA_URL="${GRAFANA_URL:-http://localhost:3000}"

# Parse arguments
while [[ $# -gt 0 ]]; do
  case "$1" in
    --loki-url=*)
      LOKI_URL="${1#*=}"
      shift
      ;;
    --grafana-url=*)
      GRAFANA_URL="${1#*=}"
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

# Check if Loki is accessible
if ! curl -sf "${LOKI_URL}/ready" >/dev/null 2>&1; then
  echo "Error: Loki is not accessible at ${LOKI_URL}" >&2
  echo "Make sure Loki is running: docker compose -f .dev/compose/compose.base.yaml -f .dev/compose/compose.local.yaml ps loki" >&2
  exit 1
fi

# Build LogQL query
if [[ -n "${SERVICE}" ]]; then
  QUERY="{service=\"${SERVICE}\"}"
else
  QUERY="{environment=\"local-dev\"}"
fi

# Check if Grafana is available
if curl -sf "${GRAFANA_URL}/api/health" >/dev/null 2>&1; then
  echo "Opening Grafana Explore view..."
  EXPLORE_URL="${GRAFANA_URL}/explore?orgId=1&left=%5B%22now-1h%22,%22now%22,%22Loki%22,%7B%22expr%22:%22${QUERY}%22%7D%5D"
  echo "URL: ${EXPLORE_URL}"
  
  # Try to open in browser (works on macOS and Linux with xdg-open)
  if command -v xdg-open >/dev/null 2>&1; then
    xdg-open "${EXPLORE_URL}" 2>/dev/null || true
  elif command -v open >/dev/null 2>&1; then
    open "${EXPLORE_URL}" 2>/dev/null || true
  else
    echo "Please open the URL above in your browser"
  fi
else
  echo "Grafana is not available. Using Loki API directly."
  echo ""
  echo "Query: ${QUERY}"
  echo ""
  echo "To query logs via curl:"
  echo "  curl -G -s \"${LOKI_URL}/loki/api/v1/query_range\" \\"
  echo "    --data-urlencode \"query=${QUERY}\" \\"
  echo "    --data-urlencode \"start=$(date -u -d '1 hour ago' +%s)000000000\" \\"
  echo "    --data-urlencode \"end=$(date -u +%s)000000000\" | jq"
  echo ""
  echo "Or use logcli (if installed):"
  echo "  logcli query '${QUERY}' --addr=${LOKI_URL}"
fi

