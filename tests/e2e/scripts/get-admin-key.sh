#!/bin/bash
# Script to help get or create an admin API key for testing

set -euo pipefail

echo "=== Admin API Key for E2E Tests ==="
echo ""
echo "The ADMIN_API_KEY is used for test setup and cleanup operations."
echo ""
echo "Options:"
echo ""
echo "1. Use Dev Stub Key (Quick - for development only):"
echo "   export ADMIN_API_KEY=dev-00000000-0000-0000-0000-000000000001-test"
echo ""
echo "2. Create via API (if you have an existing admin key):"
echo "   curl -X POST https://172.232.58.222/v1/api-keys \\"
echo "     -H 'Host: api.dev.ai-aas.local' \\"
echo "     -H 'Authorization: Bearer <existing-admin-key>' \\"
echo "     -H 'Content-Type: application/json' \\"
echo "     -d '{\"name\":\"e2e-test-admin\",\"scopes\":[\"admin\"]}'"
echo ""
echo "3. Check if seed data was created:"
echo "   The seed script creates an API key, but you need the actual secret."
echo "   Check if the database was seeded and if you have access to the secret."
echo ""
echo "4. Use Service Account (if available):"
echo "   Some environments may have service accounts with admin privileges."
echo ""
echo "For development testing, option 1 (dev stub key) is recommended."
echo ""

