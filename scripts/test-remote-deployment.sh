#!/bin/bash
# Test web portal against remote development deployment
# Usage: ./scripts/test-remote-deployment.sh [test-file]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
PORTAL_DIR="$ROOT_DIR/web/portal"

# Configuration
PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-https://portal.ai-aas.dev}"
API_ROUTER_URL="${API_ROUTER_URL:-https://api.ai-aas.dev}"
SKIP_WEBSERVER="${SKIP_WEBSERVER:-true}"
PLAYWRIGHT_HEADLESS="${PLAYWRIGHT_HEADLESS:-true}"

# Test file (optional)
TEST_FILE="${1:-}"

echo "🧪 Testing remote deployment"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Base URL:     $PLAYWRIGHT_BASE_URL"
echo "API Router:   $API_ROUTER_URL"
echo "Skip Server:  $SKIP_WEBSERVER"
echo "Headless:     $PLAYWRIGHT_HEADLESS"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Verify deployment is accessible
echo "📡 Checking deployment health..."
if ! curl -f -s --max-time 10 "${PLAYWRIGHT_BASE_URL}/healthz" > /dev/null 2>&1; then
  echo "❌ ERROR: Deployment not accessible at $PLAYWRIGHT_BASE_URL"
  echo ""
  echo "Troubleshooting:"
  echo "  1. Verify deployment is running: kubectl -n development get pods -l app=web-portal"
  echo "  2. Check ingress: kubectl -n development get ingress web-portal"
  echo "  3. Test manually: curl -v $PLAYWRIGHT_BASE_URL/healthz"
  exit 1
fi
echo "✅ Deployment is accessible"
echo ""

# Change to portal directory
cd "$PORTAL_DIR"

# Check if dependencies are installed
if [ ! -d "node_modules" ]; then
  echo "📦 Installing dependencies..."
  pnpm install
  echo ""
fi

# Run tests
echo "🚀 Running Playwright E2E tests..."
echo ""

if [ -n "$TEST_FILE" ]; then
  echo "Running specific test: $TEST_FILE"
  PLAYWRIGHT_BASE_URL="$PLAYWRIGHT_BASE_URL" \
  SKIP_WEBSERVER="$SKIP_WEBSERVER" \
  API_ROUTER_URL="$API_ROUTER_URL" \
  PLAYWRIGHT_HEADLESS="$PLAYWRIGHT_HEADLESS" \
  pnpm playwright test "$TEST_FILE"
else
  echo "Running all E2E tests"
  PLAYWRIGHT_BASE_URL="$PLAYWRIGHT_BASE_URL" \
  SKIP_WEBSERVER="$SKIP_WEBSERVER" \
  API_ROUTER_URL="$API_ROUTER_URL" \
  PLAYWRIGHT_HEADLESS="$PLAYWRIGHT_HEADLESS" \
  pnpm test:e2e
fi

echo ""
echo "✅ Tests completed!"

