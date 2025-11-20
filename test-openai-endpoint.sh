#!/bin/bash
#
# Test script for OpenAI-compatible chat completions endpoint
#
# This script sends a test request to the OpenAI-compatible /v1/chat/completions endpoint
# and validates the response.
#
# Usage:
#   ./test-openai-endpoint.sh [API_ROUTER_URL] [API_KEY]
#
# Defaults:
#   API_ROUTER_URL: http://localhost:8080
#   API_KEY: dev-test-key
#

set -e

# Configuration
API_ROUTER_URL="${1:-http://localhost:8080}"
API_KEY="${2:-dev-test-key}"
ENDPOINT="${API_ROUTER_URL}/v1/chat/completions"

# ANSI color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=================================================="
echo "OpenAI Chat Completions Endpoint Test"
echo "=================================================="
echo "Endpoint: ${ENDPOINT}"
echo "API Key: ${API_KEY:0:10}..."
echo ""

# Test request payload
REQUEST_PAYLOAD=$(cat <<EOF
{
  "model": "gpt-4o",
  "messages": [
    {
      "role": "user",
      "content": "in one word, can you provide me the Capital of France"
    }
  ]
}
EOF
)

echo "Sending request..."
echo ""

# Make the request
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${ENDPOINT}" \
  -H "Content-Type: application/json" \
  -H "X-API-Key: ${API_KEY}" \
  -d "${REQUEST_PAYLOAD}")

# Extract status code and body
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

echo "HTTP Status: ${HTTP_CODE}"
echo ""

if [ "$HTTP_CODE" != "200" ]; then
  echo -e "${RED}✗ Test FAILED${NC}"
  echo "Response body:"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  exit 1
fi

# Parse and validate response
echo "Response:"
echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
echo ""

# Extract the assistant's message
ANSWER=$(echo "$BODY" | jq -r '.choices[0].message.content' 2>/dev/null)

if [ -z "$ANSWER" ] || [ "$ANSWER" = "null" ]; then
  echo -e "${RED}✗ Test FAILED: No answer in response${NC}"
  exit 1
fi

echo -e "${YELLOW}Question:${NC} in one word, can you provide me the Capital of France"
echo -e "${YELLOW}Answer:${NC} ${ANSWER}"
echo ""

# Check if answer contains "Paris"
if echo "$ANSWER" | grep -qi "Paris"; then
  echo -e "${GREEN}✓ Test PASSED: Answer contains 'Paris'${NC}"

  # Validate other fields
  ID=$(echo "$BODY" | jq -r '.id' 2>/dev/null)
  MODEL=$(echo "$BODY" | jq -r '.model' 2>/dev/null)
  PROMPT_TOKENS=$(echo "$BODY" | jq -r '.usage.prompt_tokens' 2>/dev/null)
  COMPLETION_TOKENS=$(echo "$BODY" | jq -r '.usage.completion_tokens' 2>/dev/null)

  echo ""
  echo "Additional validation:"
  echo "  ID: ${ID}"
  echo "  Model: ${MODEL}"
  echo "  Prompt tokens: ${PROMPT_TOKENS}"
  echo "  Completion tokens: ${COMPLETION_TOKENS}"

  exit 0
else
  echo -e "${RED}✗ Test FAILED: Expected answer to contain 'Paris', got: ${ANSWER}${NC}"
  exit 1
fi
