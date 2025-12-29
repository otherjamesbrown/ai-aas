#!/bin/bash
# Verify database migration status for user-org-service
# Usage: ./verify-migration-status.sh [development|staging|production]

set -e

ENVIRONMENT="${1:-development}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Color output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=== User Org Service Migration Status Verification ==="
echo "Environment: $ENVIRONMENT"
echo ""

# Get database URL from Kubernetes secret
echo "Fetching database connection from Kubernetes secret..."
DB_URL=$(kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  get secret -n user-org-service user-org-service-db-secret \
  -o jsonpath='{.data.database-url}' | base64 -d)

if [ -z "$DB_URL" ]; then
  echo -e "${RED}ERROR: Could not retrieve database URL${NC}"
  exit 1
fi

echo -e "${GREEN}✓ Database connection retrieved${NC}"
echo ""

# Run migration status check
echo "Checking goose migration status..."
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  run psql-migration-status-$(date +%s) --rm -i --restart=Never --image=postgres:15 -- \
  psql "$DB_URL" -c "SELECT id, version_id, is_applied, tstamp FROM goose_db_version ORDER BY id DESC LIMIT 10;"

echo ""

# Check specific columns for migration 000005
echo "Verifying migration 000005 (api_key_updates) columns..."
echo ""

echo "1. Checking key_id column..."
KEY_ID_EXISTS=$(kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  run psql-check-keyid-$(date +%s) --rm -i --restart=Never --image=postgres:15 -- \
  psql "$DB_URL" -t -c "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'api_keys' AND column_name = 'key_id';" 2>&1 | grep -E "^[[:space:]]*[0-9]+[[:space:]]*$" | xargs)

if [ "$KEY_ID_EXISTS" = "1" ]; then
  echo -e "${GREEN}✓ key_id column exists${NC}"
else
  echo -e "${RED}✗ key_id column missing!${NC}"
  exit 1
fi

echo ""
echo "2. Checking notes column..."
NOTES_EXISTS=$(kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  run psql-check-notes-$(date +%s) --rm -i --restart=Never --image=postgres:15 -- \
  psql "$DB_URL" -t -c "SELECT COUNT(*) FROM information_schema.columns WHERE table_name = 'api_keys' AND column_name = 'notes';" 2>&1 | grep -E "^[[:space:]]*[0-9]+[[:space:]]*$" | xargs)

if [ "$NOTES_EXISTS" = "1" ]; then
  echo -e "${GREEN}✓ notes column exists${NC}"
else
  echo -e "${RED}✗ notes column missing!${NC}"
  exit 1
fi

echo ""
echo "3. Checking existing API keys have key_id populated..."
kubectl --kubeconfig=/home/dev/kubeconfigs/kubeconfig-${ENVIRONMENT}.yaml \
  run psql-check-data-$(date +%s) --rm -i --restart=Never --image=postgres:15 -- \
  psql "$DB_URL" -c "SELECT COUNT(*) as total_keys, COUNT(key_id) as keys_with_key_id FROM api_keys;"

echo ""
echo -e "${GREEN}=== Verification Complete ===${NC}"
