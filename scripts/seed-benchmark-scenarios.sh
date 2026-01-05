#!/bin/bash
# Script to seed benchmark scenarios into the development database
# This script uses the Admin API's scenario sync endpoint (requires master API key)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Load master API key from secrets (development environment)
if [ -f "$REPO_ROOT/secrets/env/.env" ]; then
    MASTER_API_KEY=$(grep "^DEVELOP_MASTER_ADMIN_API_KEY=" "$REPO_ROOT/secrets/env/.env" | cut -d'=' -f2)
else
    echo "Error: secrets/env/.env not found"
    exit 1
fi

if [ -z "$MASTER_API_KEY" ]; then
    echo "Error: DEVELOP_MASTER_ADMIN_API_KEY not found in secrets/env/.env"
    exit 1
fi

# Admin API endpoint
ADMIN_API_URL="${ADMIN_API_URL:-https://admin-api.dev.otherjamesbrown.com}"

# Create scenarios JSON
SCENARIOS_JSON=$(cat <<'EOF'
{
  "scenarios": [
    {
      "name": "standard",
      "description": "Standard benchmark scenario with balanced load testing",
      "version": "1.0.0",
      "config": {
        "duration_seconds": 300,
        "concurrency": 10,
        "request_rate": 1,
        "max_tokens": 100,
        "temperature": 0.7
      }
    },
    {
      "name": "throughput",
      "description": "High-throughput benchmark scenario for stress testing",
      "version": "1.0.0",
      "config": {
        "duration_seconds": 600,
        "concurrency": 50,
        "request_rate": 10,
        "max_tokens": 200,
        "temperature": 0.8
      }
    }
  ],
  "delete_orphans": false
}
EOF
)

echo "Syncing benchmark scenarios to $ADMIN_API_URL..."

# Sync scenarios
RESPONSE=$(curl -s -X POST "$ADMIN_API_URL/v1/benchmarks/scenarios/sync" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $MASTER_API_KEY" \
  -d "$SCENARIOS_JSON")

echo "Response: $RESPONSE"

# Verify by listing scenarios
echo ""
echo "Verifying scenarios..."
curl -s -X GET "$ADMIN_API_URL/v1/benchmarks/scenarios" \
  -H "Authorization: Bearer $MASTER_API_KEY" | jq '.scenarios[] | {name: .name, description: .description}'

echo ""
echo "✓ Benchmark scenarios seeded successfully"
