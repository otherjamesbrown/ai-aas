#!/bin/bash
# cleanup_test_orgs.sh - Purge stale test organizations from previous interrupted test runs
#
# Usage:
#   ./cleanup_test_orgs.sh                    # Dry run (list only)
#   ./cleanup_test_orgs.sh --delete           # Actually delete
#
# Environment variables required:
#   AI_AAS_API_ENDPOINT - User-org service endpoint (e.g., https://user-org.dev.otherjamesbrown.com)
#   AI_AAS_API_KEY      - Admin API key
#
# This script is useful when:
# - Test suite was interrupted (Ctrl+C, timeout, panic)
# - Tests failed before cleanup could run
# - Database has stale orgs causing slug conflicts

set -euo pipefail

# Check required environment variables
if [[ -z "${AI_AAS_API_ENDPOINT:-}" ]]; then
    echo "Error: AI_AAS_API_ENDPOINT not set" >&2
    exit 1
fi

if [[ -z "${AI_AAS_API_KEY:-}" ]]; then
    echo "Error: AI_AAS_API_KEY not set" >&2
    exit 1
fi

# Parse arguments
DELETE=false
if [[ "${1:-}" == "--delete" ]]; then
    DELETE=true
fi

# List all organizations
echo "Fetching organizations from ${AI_AAS_API_ENDPOINT}..."
RESPONSE=$(curl -s -H "Authorization: Bearer ${AI_AAS_API_KEY}" \
    -H "X-API-Key: ${AI_AAS_API_KEY}" \
    "${AI_AAS_API_ENDPOINT}/v1/orgs")

# Extract test organizations (those starting with "test-org-" or "test-sa-")
# Using jq to parse JSON response (array of organizations)
if ! command -v jq &> /dev/null; then
    echo "Error: jq not found. Install with: apt-get install jq" >&2
    exit 1
fi

# Parse organization list
TEST_ORGS=$(echo "$RESPONSE" | jq -r '.[] | select(.slug | startswith("test-org-") or startswith("test-sa-")) | .orgId + " " + .slug')

if [[ -z "$TEST_ORGS" ]]; then
    echo "No test organizations found."
    exit 0
fi

# Count organizations
ORG_COUNT=$(echo "$TEST_ORGS" | wc -l)
echo "Found ${ORG_COUNT} test organizations:"
echo "$TEST_ORGS" | while read -r org_id slug; do
    echo "  - ${slug} (${org_id})"
done
echo ""

if [[ "$DELETE" == "false" ]]; then
    echo "Dry run complete. Run with --delete to actually delete these organizations."
    exit 0
fi

# Delete organizations
echo "Deleting ${ORG_COUNT} test organizations..."
DELETED=0
FAILED=0

echo "$TEST_ORGS" | while read -r org_id slug; do
    echo -n "Deleting ${slug} (${org_id})... "

    # Attempt to delete
    STATUS=$(curl -s -o /dev/null -w "%{http_code}" \
        -X DELETE \
        -H "Authorization: Bearer ${AI_AAS_API_KEY}" \
        -H "X-API-Key: ${AI_AAS_API_KEY}" \
        "${AI_AAS_API_ENDPOINT}/v1/orgs/${org_id}")

    if [[ "$STATUS" == "200" || "$STATUS" == "204" ]]; then
        echo "✓ deleted"
        ((DELETED++)) || true
    else
        echo "✗ failed (HTTP ${STATUS})"
        ((FAILED++)) || true
    fi
done

echo ""
echo "Cleanup complete: ${DELETED} deleted, ${FAILED} failed"
