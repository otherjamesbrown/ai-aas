#!/bin/bash
#
# Test script for direct vLLM inference endpoint
#
# This script bypasses the API router and tests the vLLM backend directly
# to validate that the GPU inference is working correctly.
#
# Usage:
#   ./test-vllm-direct.sh [VLLM_URL] [MODEL_NAME]
#
# Examples:
#   # Test local vLLM
#   ./test-vllm-direct.sh http://localhost:8000
#
#   # Test cluster vLLM (port-forward first: kubectl port-forward svc/vllm-service 8000:8000)
#   ./test-vllm-direct.sh http://localhost:8000 meta-llama/Llama-2-7b-chat-hf
#
#   # Test cluster vLLM directly (if accessible)
#   ./test-vllm-direct.sh http://vllm-service.system.svc.cluster.local:8000
#

set -e

# Configuration
VLLM_URL="${1:-http://localhost:8000}"
MODEL_NAME="${2:-gpt-4o}"  # Default, override with actual model deployed
ENDPOINT="${VLLM_URL}/v1/chat/completions"

# ANSI color codes
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo "=================================================="
echo "vLLM Direct Inference Test"
echo "=================================================="
echo "Endpoint: ${ENDPOINT}"
echo "Model: ${MODEL_NAME}"
echo ""

# First, check if vLLM is reachable
echo -e "${BLUE}Checking vLLM health...${NC}"
HEALTH_RESPONSE=$(curl -s "${VLLM_URL}/health" 2>&1 || echo "FAILED")
if [ "$HEALTH_RESPONSE" = "FAILED" ] || [ -z "$HEALTH_RESPONSE" ]; then
  echo -e "${RED}✗ vLLM is not reachable at ${VLLM_URL}${NC}"
  echo ""
  echo "Troubleshooting steps:"
  echo "1. Check if vLLM is running in the cluster:"
  echo "   kubectl get pods -n system | grep vllm"
  echo ""
  echo "2. Port-forward the vLLM service:"
  echo "   kubectl port-forward -n system svc/vllm-service 8000:8000"
  echo ""
  echo "3. Or run vLLM locally for testing"
  exit 1
fi
echo -e "${GREEN}✓ vLLM is healthy${NC}"
echo ""

# List available models
echo -e "${BLUE}Fetching available models...${NC}"
MODELS_RESPONSE=$(curl -s "${VLLM_URL}/v1/models" | jq '.' 2>/dev/null || echo "{}")
echo "$MODELS_RESPONSE"
echo ""

# Test request payload - asking about the capital of France
REQUEST_PAYLOAD=$(cat <<EOF
{
  "model": "${MODEL_NAME}",
  "messages": [
    {
      "role": "user",
      "content": "In one word, can you provide me the capital of France?"
    }
  ],
  "max_tokens": 10,
  "temperature": 0.1
}
EOF
)

echo -e "${BLUE}Sending inference request...${NC}"
echo "Question: In one word, can you provide me the capital of France?"
echo ""

# Make the request and capture timing
START_TIME=$(date +%s%N)
RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${ENDPOINT}" \
  -H "Content-Type: application/json" \
  -d "${REQUEST_PAYLOAD}")
END_TIME=$(date +%s%N)

# Calculate latency in milliseconds
LATENCY_MS=$(( (END_TIME - START_TIME) / 1000000 ))

# Extract status code and body
HTTP_CODE=$(echo "$RESPONSE" | tail -n1)
BODY=$(echo "$RESPONSE" | sed '$d')

echo "HTTP Status: ${HTTP_CODE}"
echo "Latency: ${LATENCY_MS}ms"
echo ""

if [ "$HTTP_CODE" != "200" ]; then
  echo -e "${RED}✗ Test FAILED${NC}"
  echo "Response body:"
  echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
  exit 1
fi

# Parse and validate response
echo -e "${YELLOW}Full Response:${NC}"
echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
echo ""

# Extract the assistant's message
ANSWER=$(echo "$BODY" | jq -r '.choices[0].message.content' 2>/dev/null)

if [ -z "$ANSWER" ] || [ "$ANSWER" = "null" ]; then
  echo -e "${RED}✗ Test FAILED: No answer in response${NC}"
  exit 1
fi

echo -e "${YELLOW}Question:${NC} In one word, can you provide me the capital of France?"
echo -e "${YELLOW}Answer from vLLM:${NC} ${ANSWER}"
echo ""

# Check if answer contains "Paris" (case-insensitive)
if echo "$ANSWER" | grep -qi "Paris"; then
  echo -e "${GREEN}✓ Test PASSED: Answer contains 'Paris'${NC}"
  echo -e "${GREEN}✓ vLLM GPU inference is working correctly!${NC}"

  # Extract token usage
  PROMPT_TOKENS=$(echo "$BODY" | jq -r '.usage.prompt_tokens' 2>/dev/null)
  COMPLETION_TOKENS=$(echo "$BODY" | jq -r '.usage.completion_tokens' 2>/dev/null)
  TOTAL_TOKENS=$(echo "$BODY" | jq -r '.usage.total_tokens' 2>/dev/null)

  echo ""
  echo "Performance metrics:"
  echo "  Latency: ${LATENCY_MS}ms"
  echo "  Prompt tokens: ${PROMPT_TOKENS}"
  echo "  Completion tokens: ${COMPLETION_TOKENS}"
  echo "  Total tokens: ${TOTAL_TOKENS}"
  echo "  Tokens/second: $(echo "scale=2; $COMPLETION_TOKENS * 1000 / $LATENCY_MS" | bc 2>/dev/null || echo "N/A")"

  exit 0
else
  echo -e "${YELLOW}⚠ Test WARNING: Answer doesn't contain 'Paris'${NC}"
  echo "Expected: Paris"
  echo "Got: ${ANSWER}"
  echo ""
  echo "This might be due to:"
  echo "  - Model needs different prompting"
  echo "  - Model configuration (temperature, max_tokens)"
  echo "  - Model variant behavior"
  exit 1
fi
