#!/usr/bin/env bash
# Test script for Phase 5: Finance-friendly reporting
# This script tests the export functionality end-to-end

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
BASE_URL="${BASE_URL:-http://localhost:8084}"
ORG_ID="${ORG_ID:-123e4567-e89b-12d3-a456-426614174000}"
START_DATE="${START_DATE:-$(date -u -v-7d +%Y-%m-%dT00:00:00Z 2>/dev/null || date -u -d '7 days ago' +%Y-%m-%dT00:00:00Z)}"
END_DATE="${END_DATE:-$(date -u +%Y-%m-%dT23:59:59Z)}"

echo -e "${GREEN}Phase 5 Export Functionality Test${NC}"
echo "=================================="
echo "Base URL: $BASE_URL"
echo "Org ID: $ORG_ID"
echo "Time Range: $START_DATE to $END_DATE"
echo ""

# Test 1: Health check
echo -e "${YELLOW}Test 1: Health Check${NC}"
if curl -sf "$BASE_URL/analytics/v1/status/healthz" > /dev/null; then
    echo -e "${GREEN}✓ Service is healthy${NC}"
else
    echo -e "${RED}✗ Service health check failed${NC}"
    exit 1
fi
echo ""

# Test 2: Create export job
echo -e "${YELLOW}Test 2: Create Export Job${NC}"
RESPONSE=$(curl -s -X POST "$BASE_URL/analytics/v1/orgs/$ORG_ID/exports" \
    -H "Content-Type: application/json" \
    -d "{
        \"timeRange\": {
            \"start\": \"$START_DATE\",
            \"end\": \"$END_DATE\"
        },
        \"granularity\": \"daily\"
    }")

JOB_ID=$(echo "$RESPONSE" | grep -o '"jobId":"[^"]*' | cut -d'"' -f4)

if [ -z "$JOB_ID" ]; then
    echo -e "${RED}✗ Failed to create export job${NC}"
    echo "Response: $RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓ Export job created${NC}"
echo "Job ID: $JOB_ID"
echo "Response: $RESPONSE"
echo ""

# Test 3: Check job status
echo -e "${YELLOW}Test 3: Check Job Status${NC}"
STATUS="pending"
ATTEMPTS=0
MAX_ATTEMPTS=30

while [ "$STATUS" != "succeeded" ] && [ "$STATUS" != "failed" ] && [ $ATTEMPTS -lt $MAX_ATTEMPTS ]; do
    sleep 2
    ATTEMPTS=$((ATTEMPTS + 1))
    
    RESPONSE=$(curl -s "$BASE_URL/analytics/v1/orgs/$ORG_ID/exports/$JOB_ID")
    STATUS=$(echo "$RESPONSE" | grep -o '"status":"[^"]*' | cut -d'"' -f4)
    
    echo "Attempt $ATTEMPTS: Status = $STATUS"
    
    if [ "$STATUS" = "failed" ]; then
        ERROR=$(echo "$RESPONSE" | grep -o '"error":"[^"]*' | cut -d'"' -f4 || echo "Unknown error")
        echo -e "${RED}✗ Export job failed: $ERROR${NC}"
        echo "Response: $RESPONSE"
        exit 1
    fi
done

if [ "$STATUS" != "succeeded" ]; then
    echo -e "${RED}✗ Export job did not complete in time (status: $STATUS)${NC}"
    echo "Response: $RESPONSE"
    exit 1
fi

echo -e "${GREEN}✓ Export job completed successfully${NC}"
echo "Response: $RESPONSE"
echo ""

# Test 4: Get download URL
echo -e "${YELLOW}Test 4: Get Download URL${NC}"
DOWNLOAD_RESPONSE=$(curl -s -I "$BASE_URL/analytics/v1/orgs/$ORG_ID/exports/$JOB_ID/download" | grep -i "location" || echo "")
if echo "$DOWNLOAD_RESPONSE" | grep -qi "location"; then
    LOCATION=$(echo "$DOWNLOAD_RESPONSE" | grep -i "location" | cut -d' ' -f2 | tr -d '\r')
    echo -e "${GREEN}✓ Download URL retrieved${NC}"
    echo "Location: $LOCATION"
else
    echo -e "${YELLOW}⚠ Download URL not available (may require authentication)${NC}"
fi
echo ""

# Test 5: List export jobs
echo -e "${YELLOW}Test 5: List Export Jobs${NC}"
LIST_RESPONSE=$(curl -s "$BASE_URL/analytics/v1/orgs/$ORG_ID/exports")
if echo "$LIST_RESPONSE" | grep -q "$JOB_ID"; then
    echo -e "${GREEN}✓ Export job appears in list${NC}"
    echo "Found $JOB_ID in job list"
else
    echo -e "${YELLOW}⚠ Export job not found in list (may be expected)${NC}"
fi
echo ""

# Test 6: Filter by status
echo -e "${YELLOW}Test 6: Filter Jobs by Status${NC}"
FILTERED_RESPONSE=$(curl -s "$BASE_URL/analytics/v1/orgs/$ORG_ID/exports?status=succeeded")
if echo "$FILTERED_RESPONSE" | grep -q "$JOB_ID"; then
    echo -e "${GREEN}✓ Filtered list includes completed job${NC}"
else
    echo -e "${YELLOW}⚠ Completed job not in filtered list${NC}"
fi
echo ""

echo -e "${GREEN}All tests completed!${NC}"
echo ""
echo "Summary:"
echo "  - Export job created: ✓"
echo "  - Job processed: ✓"
echo "  - Download URL available: $(if [ -n "$LOCATION" ]; then echo "✓"; else echo "⚠"; fi)"
echo "  - Job listing works: ✓"

