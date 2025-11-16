#!/usr/bin/env bash
# Seed test users and organizations for development and testing
#
# This script creates:
# - System Admin user
# - Acme Ltd organization with admin and manager users
# - JoeBlogs Ltd organization with admin and manager users
#
# Usage:
#   ./scripts/seed-test-users.sh
#
# Environment variables:
#   DATABASE_URL: Postgres connection string (defaults to local-dev)
#   FORCE: Set to "true" to force re-seeding existing users/orgs

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Default database URL
DATABASE_URL="${DATABASE_URL:-postgres://postgres:postgres@localhost:5433/ai_aas?sslmode=disable}"
FORCE="${FORCE:-false}"

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Seeding test users and organizations..."
echo "Database: ${DATABASE_URL}"
echo ""

cd "$PROJECT_ROOT/services/user-org-service"

# Export DATABASE_URL for the seed command
export DATABASE_URL

# Build seed-test-users if it doesn't exist
if [ ! -f "./bin/seed-test-users" ]; then
  echo -e "${YELLOW}Building seed-test-users...${NC}"
  export PATH="$HOME/.local/go/bin:$PATH"
  export GOROOT="$HOME/.local/go"
  export GOTOOLCHAIN=go1.24.10
  go build -trimpath -o bin/seed-test-users ./cmd/seed-test-users || {
    echo "Failed to build seed-test-users. Make sure Go is installed."
    exit 1
  }
fi

# Run the seed-test-users command
FORCE_FLAG=""
if [ "$FORCE" = "true" ]; then
  FORCE_FLAG="-force"
fi

./bin/seed-test-users $FORCE_FLAG

echo -e "\n${GREEN}✓ Seeding completed!${NC}"
echo ""
echo "Test users are ready. See seeded-users.md for credentials."

