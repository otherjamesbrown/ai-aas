#!/bin/bash
# promote-images.sh - Update all platform service images to a specific SHA tag
#
# Usage:
#   ./scripts/promote-images.sh <sha-tag> [environment]
#
# Examples:
#   ./scripts/promote-images.sh sha-923f2ed           # Update all environments
#   ./scripts/promote-images.sh sha-923f2ed staging   # Update staging only
#
# This script updates Helm values files for all platform services to use
# a consistent SHA-based image tag across environments.

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Function to print usage
usage() {
    echo "Usage: $0 <sha-tag> [environment]"
    echo ""
    echo "Arguments:"
    echo "  sha-tag      The SHA-based tag to promote (e.g., sha-923f2ed)"
    echo "  environment  Optional: specific environment (development, staging, production)"
    echo "               If not specified, updates all environments"
    echo ""
    echo "Examples:"
    echo "  $0 sha-923f2ed              # Update all environments"
    echo "  $0 sha-923f2ed staging      # Update staging only"
    echo "  $0 sha-923f2ed development  # Update development only"
    exit 1
}

# Validate arguments
if [[ $# -lt 1 ]]; then
    echo -e "${RED}Error: Missing required argument${NC}"
    usage
fi

SHA_TAG="$1"
TARGET_ENV="${2:-all}"

# Validate SHA tag format
if [[ ! "$SHA_TAG" =~ ^sha-[a-f0-9]{7}$ ]]; then
    echo -e "${RED}Error: Invalid SHA tag format${NC}"
    echo "Expected format: sha-xxxxxxx (7 hex characters)"
    echo "Example: sha-923f2ed"
    exit 1
fi

# Validate environment if specified
if [[ "$TARGET_ENV" != "all" ]] && [[ "$TARGET_ENV" != "development" ]] && [[ "$TARGET_ENV" != "staging" ]] && [[ "$TARGET_ENV" != "production" ]]; then
    echo -e "${RED}Error: Invalid environment: $TARGET_ENV${NC}"
    echo "Valid environments: development, staging, production, all"
    exit 1
fi

echo -e "${GREEN}Promoting platform images to $SHA_TAG${NC}"
if [[ "$TARGET_ENV" != "all" ]]; then
    echo -e "${YELLOW}Target environment: $TARGET_ENV${NC}"
fi
echo ""

# Service configurations
# Format: service_path:values_file_pattern
declare -a SERVICES=(
    "services/admin-api-service/deployments/helm/admin-api-service:values"
    "services/analytics-service/deployments/helm/analytics-service:values"
    "services/api-router-service/deployments/helm/api-router-service:values"
    "services/user-org-service/configs/helm:values"
    "operators/ai-model-operator/deployments/helm/ai-model-operator:values"
)

# Environments to update
declare -a ENVIRONMENTS=()
if [[ "$TARGET_ENV" == "all" ]]; then
    ENVIRONMENTS=("development" "staging" "production")
else
    ENVIRONMENTS=("$TARGET_ENV")
fi

# Function to update image tag in a values file
update_values_file() {
    local file="$1"
    local sha_tag="$2"

    if [[ ! -f "$file" ]]; then
        echo -e "${YELLOW}  ⚠ File not found: $file${NC}"
        return 1
    fi

    # Check if file has an image.tag field
    if ! grep -q "^\s*tag:" "$file"; then
        echo -e "${YELLOW}  ⚠ No image.tag found in: $file${NC}"
        return 1
    fi

    # Update the tag (preserve indentation and comment structure)
    sed -i.bak "s|^\(\s*tag:\s*\).*|\1\"$sha_tag\"  # Standardized SHA-based tag|" "$file"
    rm -f "${file}.bak"

    echo -e "${GREEN}  ✓ Updated: $(basename "$file")${NC}"
    return 0
}

# Track statistics
UPDATED_COUNT=0
SKIPPED_COUNT=0
FAILED_COUNT=0

# Update each service
for service_config in "${SERVICES[@]}"; do
    IFS=':' read -r service_path values_pattern <<< "$service_config"
    service_name="$(basename "$(dirname "$service_path")")"

    echo -e "${YELLOW}Processing $service_name...${NC}"

    for env in "${ENVIRONMENTS[@]}"; do
        values_file="$REPO_ROOT/$service_path/${values_pattern}-${env}.yaml"

        if update_values_file "$values_file" "$SHA_TAG"; then
            ((UPDATED_COUNT++))
        else
            ((SKIPPED_COUNT++))
        fi
    done
    echo ""
done

# Summary
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "${GREEN}Promotion Summary${NC}"
echo -e "${GREEN}════════════════════════════════════════${NC}"
echo -e "SHA Tag: ${YELLOW}$SHA_TAG${NC}"
echo -e "Updated: ${GREEN}$UPDATED_COUNT${NC} files"
echo -e "Skipped: ${YELLOW}$SKIPPED_COUNT${NC} files"
echo -e "Failed:  ${RED}$FAILED_COUNT${NC} files"
echo ""

if [[ $UPDATED_COUNT -gt 0 ]]; then
    echo -e "${YELLOW}Next steps:${NC}"
    echo "1. Review changes: git diff"
    echo "2. Commit changes: git add . && git commit -m 'chore: promote images to $SHA_TAG'"
    echo "3. Push to repository: git push origin \$(git branch --show-current)"
    echo "4. ArgoCD will sync automatically (or manually sync for production)"
fi
