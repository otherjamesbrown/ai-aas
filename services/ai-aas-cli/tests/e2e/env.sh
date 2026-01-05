#!/bin/bash
# Environment setup for AI-AAS CLI E2E tests
# Source this file before running tests: source tests/e2e/env.sh

set -e

# Project root (relative to this script)
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../../.." && pwd)"

# Check if secrets are available
SECRETS_FILE="${PROJECT_ROOT}/secrets/env/.env"
if [[ ! -f "$SECRETS_FILE" ]]; then
    echo "ERROR: Secrets file not found at $SECRETS_FILE"
    echo "Make sure git-crypt is unlocked: git-crypt unlock"
    exit 1
fi

# Load master admin API key from secrets (development environment)
export AI_AAS_API_KEY=$(grep '^DEVELOP_MASTER_ADMIN_API_KEY=' "$SECRETS_FILE" | cut -d'=' -f2)
export AI_AAS_MASTER_ORG_ID=$(grep '^MASTER_ADMIN_ORG_ID=' "$SECRETS_FILE" | cut -d'=' -f2)
export AI_AAS_MASTER_USER_ID=$(grep '^MASTER_ADMIN_USER_ID=' "$SECRETS_FILE" | cut -d'=' -f2)

# Development environment endpoints
# All services use dev.otherjamesbrown.com with Let's Encrypt TLS certificates
export AI_AAS_USER_ORG_ENDPOINT="https://user-org.dev.otherjamesbrown.com"
export AI_AAS_INFERENCE_ENDPOINT="https://api.dev.otherjamesbrown.com"
export AI_AAS_ANALYTICS_ENDPOINT="https://analytics.dev.otherjamesbrown.com"
export AI_AAS_ENVIRONMENT="development"

# TLS settings (using Let's Encrypt certificates - no need to skip verification)
export AI_AAS_TLS_INSECURE="false"

# Test configuration
export E2E_TEST_ORG_PREFIX="e2e-test-"
export E2E_TEST_USER_EMAIL_DOMAIN="test.ai-aas.dev"

# Validation
if [[ -z "$AI_AAS_API_KEY" ]]; then
    echo "ERROR: Failed to load DEVELOP_MASTER_ADMIN_API_KEY from secrets"
    exit 1
fi

echo "E2E test environment configured:"
echo "  API Key: ****${AI_AAS_API_KEY: -4}"
echo "  User-Org Endpoint: $AI_AAS_USER_ORG_ENDPOINT"
echo "  Inference Endpoint: $AI_AAS_INFERENCE_ENDPOINT"
echo "  Analytics Endpoint: $AI_AAS_ANALYTICS_ENDPOINT"
echo "  Environment: $AI_AAS_ENVIRONMENT"
echo ""
echo "Run tests with: go test -v -tags=e2e ./tests/e2e/..."
