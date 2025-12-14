#!/bin/bash
#
# Configure Routing Policies for Staging Environment
#
# This script creates routing policies for all models deployed in staging.
# It uses the Admin API to create policies that route requests to KServe InferenceServices.
#

set -e

# Configuration
ADMIN_API_ENDPOINT="http://admin-api.172.236.135.55.nip.io"
API_KEY="ai-aas__HYQk1SQgY4P_f2aMjYM39zL9NAxG63tcHn_Gx4If3M"  # STAGING_MASTER_ADMIN_API_KEY

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo "=================================="
echo "Staging Routing Policy Setup"
echo "=================================="
echo ""
echo "Admin API: $ADMIN_API_ENDPOINT"
echo "Target Namespace: staging"
echo ""

# Function to create a routing policy
create_policy() {
    local model_name=$1
    local backend_id=$2
    local backend_url=$3

    echo -e "${YELLOW}Creating policy for $model_name...${NC}"

    # Create policy JSON
    # For KServe, the backend URL should point to the predictor service
    # with the OpenAI-compatible /v1/completions or /v1/chat/completions path
    local policy_json=$(cat <<EOF
{
  "model": "$model_name",
  "organization_id": "*",
  "backends": [
    {
      "backend_id": "$backend_id",
      "weight": 100
    }
  ],
  "failover_threshold": 1,
  "enabled": true,
  "metadata": {
    "environment": "staging",
    "created_by": "configure-staging-routing-policies.sh",
    "backend_type": "kserve"
  }
}
EOF
)

    # Create the policy
    response=$(curl -s -w "\n%{http_code}" \
        -H "Authorization: Bearer $API_KEY" \
        -H "Content-Type: application/json" \
        -X POST \
        "$ADMIN_API_ENDPOINT/v1/routing/policies" \
        -d "$policy_json")

    http_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | sed '$d')

    if [ "$http_code" -eq 201 ]; then
        policy_id=$(echo "$body" | jq -r '.policy_id')
        echo -e "${GREEN}✓ Created policy: $policy_id${NC}"
        echo "  Model: $model_name"
        echo "  Backend: $backend_id"
        echo "  URL: $backend_url"
    elif [ "$http_code" -eq 409 ] || echo "$body" | grep -q "already exists"; then
        echo -e "${YELLOW}⚠ Policy already exists for $model_name${NC}"
    else
        echo -e "${RED}✗ Failed to create policy for $model_name${NC}"
        echo "  HTTP Status: $http_code"
        echo "  Response: $body"
        return 1
    fi

    echo ""
}

# Create policies for each model
# The backend URLs point to the KServe predictor services
# KServe routes through Istio, so we use the predictor service name

echo "Creating routing policies..."
echo ""

# mistral-7b-instruct-v03
create_policy \
    "mistral-7b-instruct-v03" \
    "staging-mistral-7b-instruct-v03" \
    "http://mistral-7b-instruct-v03-predictor.staging.svc.cluster.local/v1"

# openai-gpt-oss-20b
create_policy \
    "openai/gpt-oss-20b" \
    "staging-openai-gpt-oss-20b" \
    "http://openai-gpt-oss-20b-predictor.staging.svc.cluster.local/v1"

# unsloth-gpt-oss-20b
create_policy \
    "unsloth/gpt-oss-20b" \
    "staging-unsloth-gpt-oss-20b" \
    "http://unsloth-gpt-oss-20b-predictor.staging.svc.cluster.local/v1"

echo "=================================="
echo "Listing all policies..."
echo "=================================="
echo ""

curl -s -H "Authorization: Bearer $API_KEY" \
    "$ADMIN_API_ENDPOINT/v1/routing/policies" | jq '.policies[] | {policy_id, model, organization_id, enabled, backends}'

echo ""
echo -e "${GREEN}✓ Routing policy setup complete!${NC}"
echo ""
echo "Next steps:"
echo "  1. Verify policies are loaded in api-router-service"
echo "  2. Test inference requests through the API router"
echo "  3. Check api-router logs for policy lookup: kubectl logs -n staging -l app=api-router-service"
