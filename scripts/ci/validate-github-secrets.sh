#!/usr/bin/env bash
#
# validate-github-secrets.sh
#
# Validates which GitHub repository secrets are configured.
# Note: This script can only check if secrets exist, not their values.
#
# Usage:
#   ./scripts/ci/validate-github-secrets.sh
#
# Requirements:
#   - gh CLI installed and authenticated
#

set -euo pipefail

REPO="otherjamesbrown/ai-aas"

# Color codes
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "========================================="
echo "GitHub Secrets Validation"
echo "Repository: $REPO"
echo "========================================="
echo

# Fetch all existing secrets
echo "Fetching existing secrets..."
EXISTING_SECRETS=$(gh secret list --repo "$REPO" 2>&1)

if [ $? -ne 0 ]; then
    echo -e "${RED}ERROR: Failed to fetch secrets. Are you authenticated with gh CLI?${NC}"
    echo
    echo "Run: gh auth login"
    exit 1
fi

echo

# Define required secrets
declare -A CRITICAL_SECRETS=(
    ["DEVELOP_MASTER_ADMIN_API_KEY"]="E2E test authentication (development)"
    ["STAGING_MASTER_ADMIN_API_KEY"]="E2E test authentication (staging)"
    ["LINODE_OBJECT_STORAGE_ACCESS_KEY"]="Upload E2E test results"
    ["LINODE_OBJECT_STORAGE_SECRET_KEY"]="Upload E2E test results"
    ["DEV_KUBECONFIG_B64"]="Development cluster access"
    ["DEV_KUBE_CONTEXT"]="Development cluster context"
    ["PROD_KUBECONFIG_B64"]="Production cluster access"
    ["PROD_KUBE_CONTEXT"]="Production cluster context"
    ["GHCR_TOKEN"]="Container registry authentication"
    ["LINODE_TOKEN"]="Infrastructure provisioning"
)

declare -A OPTIONAL_SECRETS=(
    ["GRAFANA_API_KEY"]="Grafana annotations (optional)"
    ["GRAFANA_URL"]="Grafana instance URL (optional)"
    ["DEV_KUBECONFIG"]="Non-base64 kubeconfig (failure mode tests)"
    ["DEV_API_ENDPOINT"]="Development API endpoint (failure mode tests)"
    ["DEV_ADMIN_API_KEY"]="Development admin key (failure mode tests)"
    ["API_ROUTER_DEV_TEST_KEY"]="API router validation (development)"
    ["API_ROUTER_STAGING_TEST_KEY"]="API router validation (staging)"
    ["API_ROUTER_PROD_TEST_KEY"]="API router validation (production)"
)

check_secret() {
    local secret_name=$1
    if echo "$EXISTING_SECRETS" | grep -q "^$secret_name"; then
        return 0  # Secret exists
    else
        return 1  # Secret missing
    fi
}

# Check critical secrets
echo "========================================="
echo "CRITICAL SECRETS (block CI/CD)"
echo "========================================="
echo

CRITICAL_MISSING=0
for secret in "${!CRITICAL_SECRETS[@]}"; do
    if check_secret "$secret"; then
        echo -e "${GREEN}✓${NC} $secret - ${CRITICAL_SECRETS[$secret]}"
    else
        echo -e "${RED}✗${NC} $secret - ${CRITICAL_SECRETS[$secret]}"
        ((CRITICAL_MISSING++))
    fi
done

echo

# Check optional secrets
echo "========================================="
echo "OPTIONAL SECRETS (degrade functionality)"
echo "========================================="
echo

OPTIONAL_MISSING=0
for secret in "${!OPTIONAL_SECRETS[@]}"; do
    if check_secret "$secret"; then
        echo -e "${GREEN}✓${NC} $secret - ${OPTIONAL_SECRETS[$secret]}"
    else
        echo -e "${YELLOW}⚠${NC} $secret - ${OPTIONAL_SECRETS[$secret]}"
        ((OPTIONAL_MISSING++))
    fi
done

echo

# Summary
echo "========================================="
echo "SUMMARY"
echo "========================================="
echo

TOTAL_CRITICAL=${#CRITICAL_SECRETS[@]}
CRITICAL_OK=$((TOTAL_CRITICAL - CRITICAL_MISSING))

TOTAL_OPTIONAL=${#OPTIONAL_SECRETS[@]}
OPTIONAL_OK=$((TOTAL_OPTIONAL - OPTIONAL_MISSING))

echo "Critical: $CRITICAL_OK/$TOTAL_CRITICAL configured"
echo "Optional: $OPTIONAL_OK/$TOTAL_OPTIONAL configured"
echo

if [ $CRITICAL_MISSING -eq 0 ]; then
    echo -e "${GREEN}✓ All critical secrets are configured!${NC}"
    EXIT_CODE=0
else
    echo -e "${RED}✗ $CRITICAL_MISSING critical secret(s) missing!${NC}"
    echo
    echo "To fix:"
    echo "1. Go to: https://github.com/$REPO/settings/secrets/actions"
    echo "2. Click 'New repository secret'"
    echo "3. Add each missing secret (see docs/platform/github-secrets-setup.md)"
    EXIT_CODE=1
fi

if [ $OPTIONAL_MISSING -gt 0 ]; then
    echo -e "${YELLOW}⚠ $OPTIONAL_MISSING optional secret(s) missing${NC}"
    echo "   Some workflows may have degraded functionality"
fi

echo

# Show existing secrets that aren't documented
echo "========================================="
echo "UNDOCUMENTED SECRETS"
echo "========================================="
echo

UNDOCUMENTED=0
while IFS= read -r secret; do
    # Extract just the secret name (first column)
    secret_name=$(echo "$secret" | awk '{print $1}')

    # Skip empty lines
    [ -z "$secret_name" ] && continue

    # Check if documented
    if [ -z "${CRITICAL_SECRETS[$secret_name]:-}" ] && [ -z "${OPTIONAL_SECRETS[$secret_name]:-}" ]; then
        echo -e "${YELLOW}?${NC} $secret_name - Not documented in validation script"
        ((UNDOCUMENTED++))
    fi
done <<< "$EXISTING_SECRETS"

if [ $UNDOCUMENTED -eq 0 ]; then
    echo "None - all secrets are documented"
fi

echo

exit $EXIT_CODE
