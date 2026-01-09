#!/bin/bash
#
# Quick screenshot of a Grafana dashboard
#
# Usage:
#   ./screenshot-dashboard.sh <dashboard-uid> <environment> [output-file]
#
# Examples:
#   ./screenshot-dashboard.sh benchmark-system-health staging
#   ./screenshot-dashboard.sh api-performance-v2 development /tmp/my-screenshot.png
#
# Environment variables:
#   GRAFANA_USER     - Grafana username (default: admin)
#   GRAFANA_PASSWORD - Grafana password (default: admin123)
#

set -e

DASHBOARD_UID="${1:?Usage: $0 <dashboard-uid> <environment> [output-file]}"
ENVIRONMENT="${2:?Usage: $0 <dashboard-uid> <environment> [output-file]}"
OUTPUT_FILE="${3:-/tmp/dashboard-screenshots/${DASHBOARD_UID}-$(date +%Y%m%d-%H%M%S).png}"

GRAFANA_URL="https://grafana.${ENVIRONMENT}.otherjamesbrown.com"
GRAFANA_USER="${GRAFANA_USER:-admin}"
GRAFANA_PASSWORD="${GRAFANA_PASSWORD:-admin123}"

# Create output directory
mkdir -p "$(dirname "$OUTPUT_FILE")"

echo "Capturing dashboard screenshot..."
echo "  Dashboard: $DASHBOARD_UID"
echo "  Environment: $ENVIRONMENT"
echo "  Output: $OUTPUT_FILE"

# Use Playwright CLI for quick screenshot
# Note: This requires playwright to be installed: npm install -D playwright
npx playwright screenshot \
  --browser chromium \
  --wait-for-timeout 5000 \
  --full-page \
  --ignore-https-errors \
  "${GRAFANA_URL}/d/${DASHBOARD_UID}?kiosk&auth_token=$(echo -n "${GRAFANA_USER}:${GRAFANA_PASSWORD}" | base64)" \
  "$OUTPUT_FILE" 2>/dev/null || {
    # Fallback: Use the TypeScript explorer for authenticated screenshots
    echo "Note: Quick screenshot failed (may need authentication)"
    echo "Using full explorer script instead..."

    SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    OUTPUT_DIR="$(dirname "$OUTPUT_FILE")"

    npx ts-node "${SCRIPT_DIR}/dashboard-explorer.ts" "$DASHBOARD_UID" "$ENVIRONMENT" "$OUTPUT_DIR"

    # Rename default.png to requested output file
    if [ -f "${OUTPUT_DIR}/default.png" ] && [ "$OUTPUT_FILE" != "${OUTPUT_DIR}/default.png" ]; then
      mv "${OUTPUT_DIR}/default.png" "$OUTPUT_FILE"
    fi
  }

echo ""
echo "✓ Screenshot saved: $OUTPUT_FILE"
echo ""
echo "To analyze this dashboard, use:"
echo "  Read $OUTPUT_FILE"
