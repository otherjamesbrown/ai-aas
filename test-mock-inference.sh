#!/usr/bin/env bash
# Test script for mock OpenAI API on remote dev environment
# Usage: ./test-mock-inference.sh <WORKSPACE_HOST>

set -euo pipefail

WORKSPACE_HOST="${1:-}"
MOCK_PORT="${MOCK_INFERENCE_PORT:-8000}"

if [[ -z "${WORKSPACE_HOST}" ]]; then
  echo "Usage: $0 <WORKSPACE_HOST>"
  echo "Example: $0 192.0.2.1"
  exit 1
fi

MOCK_BASE_URL="http://${WORKSPACE_HOST}:${MOCK_PORT}"

echo "Testing Mock Inference Service at ${MOCK_BASE_URL}"
echo "=========================================="
echo ""

# Test 1: Health check
echo "1. Testing /health endpoint..."
HEALTH_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" "${MOCK_BASE_URL}/health" || echo "FAILED")
HTTP_CODE=$(echo "${HEALTH_RESPONSE}" | grep "HTTP_CODE" | cut -d: -f2)
HEALTH_BODY=$(echo "${HEALTH_RESPONSE}" | sed '/HTTP_CODE/d')

if [[ "${HTTP_CODE}" == "200" ]]; then
  echo "✅ Health check passed"
  echo "${HEALTH_BODY}" | jq . 2>/dev/null || echo "${HEALTH_BODY}"
else
  echo "❌ Health check failed (HTTP ${HTTP_CODE})"
  echo "${HEALTH_BODY}"
fi
echo ""

# Test 2: Readiness check
echo "2. Testing /ready endpoint..."
READY_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" "${MOCK_BASE_URL}/ready" || echo "FAILED")
HTTP_CODE=$(echo "${READY_RESPONSE}" | grep "HTTP_CODE" | cut -d: -f2)
READY_BODY=$(echo "${READY_RESPONSE}" | sed '/HTTP_CODE/d')

if [[ "${HTTP_CODE}" == "200" ]]; then
  echo "✅ Readiness check passed"
  echo "${READY_BODY}" | jq . 2>/dev/null || echo "${READY_BODY}"
else
  echo "❌ Readiness check failed (HTTP ${HTTP_CODE})"
  echo "${READY_BODY}"
fi
echo ""

# Test 3: /v1/completions endpoint
echo "3. Testing /v1/completions endpoint..."
COMPLETION_REQUEST='{
  "prompt": "Hello, world!",
  "max_tokens": 50,
  "model": "gpt-4o"
}'

COMPLETION_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
  -X POST "${MOCK_BASE_URL}/v1/completions" \
  -H "Content-Type: application/json" \
  -d "${COMPLETION_REQUEST}" || echo "FAILED")
HTTP_CODE=$(echo "${COMPLETION_RESPONSE}" | grep "HTTP_CODE" | cut -d: -f2)
COMPLETION_BODY=$(echo "${COMPLETION_RESPONSE}" | sed '/HTTP_CODE/d')

if [[ "${HTTP_CODE}" == "200" ]]; then
  echo "✅ Completions endpoint passed"
  echo "${COMPLETION_BODY}" | jq . 2>/dev/null || echo "${COMPLETION_BODY}"
else
  echo "❌ Completions endpoint failed (HTTP ${HTTP_CODE})"
  echo "${COMPLETION_BODY}"
fi
echo ""

# Test 4: /v1/chat/completions endpoint
echo "4. Testing /v1/chat/completions endpoint..."
CHAT_REQUEST='{
  "messages": [
    {"role": "user", "content": "Hello!"}
  ]
}'

CHAT_RESPONSE=$(curl -s -w "\nHTTP_CODE:%{http_code}" \
  -X POST "${MOCK_BASE_URL}/v1/chat/completions" \
  -H "Content-Type: application/json" \
  -d "${CHAT_REQUEST}" || echo "FAILED")
HTTP_CODE=$(echo "${CHAT_RESPONSE}" | grep "HTTP_CODE" | cut -d: -f2)
CHAT_BODY=$(echo "${CHAT_RESPONSE}" | sed '/HTTP_CODE/d')

if [[ "${HTTP_CODE}" == "200" ]]; then
  echo "✅ Chat completions endpoint passed"
  echo "${CHAT_BODY}" | jq . 2>/dev/null || echo "${CHAT_BODY}"
else
  echo "❌ Chat completions endpoint failed (HTTP ${HTTP_CODE})"
  echo "${CHAT_BODY}"
fi
echo ""

echo "=========================================="
echo "Testing complete!"

