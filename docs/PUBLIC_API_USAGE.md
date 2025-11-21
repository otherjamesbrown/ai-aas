# Public API Usage Guide

This guide shows how to use the AI-AAS public API for OpenAI-compatible inference.

## Overview

The AI-AAS platform exposes a public HTTPS API that implements OpenAI-compatible endpoints. All requests are authenticated and routed through the API Router service to backend inference engines (vLLM).

## Endpoint

**Base URL**: `https://api.dev.ai-aas.local`
**Protocol**: HTTPS (TLS 1.2+)

**Note**: For development environments where DNS is not configured, you can use the ingress IP address `172.232.58.222` with the `Host` header set to `api.dev.ai-aas.local`.

## Authentication

All API requests require authentication via API key:

**Header**: `X-API-Key: <your-api-key>`

### Development API Key

For development/testing purposes, API keys can be obtained through the User-Org Service API or by querying the development database directly. Contact your platform administrator or refer to the platform setup documentation for instructions on creating test API keys in your development environment.

## Available Endpoints

### 1. Chat Completions (Recommended)

OpenAI-compatible chat completions endpoint.

**Endpoint**: `POST /v1/chat/completions`

**Example Request**:
```bash
curl -X POST https://api.dev.ai-aas.local/v1/chat/completions \
  -H "X-API-Key: <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [
      {"role": "user", "content": "Explain quantum computing in 3 sentences"}
    ],
    "max_tokens": 100,
    "temperature": 0.7
  }'
```

**Example Response**:
```json
{
  "id": "chatcmpl-abc123",
  "object": "chat.completion",
  "created": 1732147200,
  "model": "gpt-oss-20b",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "Quantum computing uses quantum bits (qubits) that can exist in superposition..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 15,
    "completion_tokens": 85,
    "total_tokens": 100
  }
}
```

### 2. Text Completions (Legacy)

OpenAI-compatible text completions endpoint.

**Endpoint**: `POST /v1/completions`

**Example Request**:
```bash
curl -k -X POST https://api.dev.ai-aas.local/v1/completions \
  -H "X-API-Key: <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "prompt": "Once upon a time",
    "max_tokens": 50
  }'
```

**Example Response**:
```json
{
  "id": "cmpl-xyz789",
  "object": "text_completion",
  "created": 1732147200,
  "model": "gpt-oss-20b",
  "choices": [
    {
      "text": " in a distant galaxy, there lived a curious robot...",
      "index": 0,
      "finish_reason": "length"
    }
  ],
  "usage": {
    "prompt_tokens": 4,
    "completion_tokens": 50,
    "total_tokens": 54
  }
}
```

## Request Parameters

### Chat Completions Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `model` | string | Yes | Model name (e.g., `gpt-oss-20b`) |
| `messages` | array | Yes | Array of message objects with `role` and `content` |
| `max_tokens` | integer | No | Maximum tokens to generate (default: model max) |
| `temperature` | float | No | Sampling temperature 0.0-2.0 (default: 1.0) |
| `top_p` | float | No | Nucleus sampling parameter (default: 1.0) |
| `n` | integer | No | Number of completions to generate (default: 1) |
| `stream` | boolean | No | Enable streaming responses (default: false) |
| `stop` | string/array | No | Stop sequences |

### Message Format

Messages must include `role` and `content`:

```json
{
  "role": "system|user|assistant",
  "content": "message text"
}
```

**Supported roles**:
- `system`: System instructions/context
- `user`: User messages
- `assistant`: Assistant responses (for conversation history)

## Available Models

| Model Name | Backend | Description |
|------------|---------|-------------|
| `gpt-oss-20b` | vLLM | Unsloth GPT-OSS 20B parameter model |

To list available models:
```bash
curl -k -X GET https://api.dev.ai-aas.local/v1/models \
  -H "X-API-Key: <your-api-key>"
```

## Error Responses

### 401 Unauthorized
```json
{
  "error": {
    "message": "Invalid or missing API key",
    "type": "authentication_error",
    "code": "invalid_api_key"
  }
}
```

### 400 Bad Request
```json
{
  "error": {
    "message": "Invalid request: missing required field 'messages'",
    "type": "invalid_request_error",
    "code": "invalid_request"
  }
}
```

### 429 Rate Limited
```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

### 503 Service Unavailable
```json
{
  "error": {
    "message": "No healthy backends available",
    "type": "service_error",
    "code": "service_unavailable"
  }
}
```

## Client Libraries

The API is OpenAI-compatible, so you can use official OpenAI client libraries:

### Python (openai)

```python
from openai import OpenAI

client = OpenAI(
    api_key="<your-api-key>",
    base_url="https://api.dev.ai-aas.local/v1"
    # For development with self-signed certs, add:
    # http_client=httpx.Client(verify=False)
)

response = client.chat.completions.create(
    model="gpt-oss-20b",
    messages=[
        {"role": "user", "content": "Hello, how are you?"}
    ],
    max_tokens=50
)

print(response.choices[0].message.content)
```

### Node.js (openai)

```javascript
import OpenAI from 'openai';
// For development with self-signed certs
import https from 'https';

const client = new OpenAI({
  apiKey: '<your-api-key>',
  baseURL: 'https://api.dev.ai-aas.local/v1'
  // For development with self-signed certs, add:
  // httpAgent: new https.Agent({ rejectUnauthorized: false })
});

const response = await client.chat.completions.create({
  model: 'gpt-oss-20b',
  messages: [
    { role: 'user', content: 'Hello, how are you?' }
  ],
  max_tokens: 50
});

console.log(response.choices[0].message.content);
```

## Rate Limits

Default rate limits per organization:
- **Requests per second**: 100 RPS
- **Burst size**: 200 requests

Rate limits are enforced per API key and are configurable per organization.

## Best Practices

### 1. Always Use HTTPS
The platform enforces HTTPS for security. HTTP requests will be redirected to HTTPS.

### 2. Using IP Addresses
When DNS is not configured, you can use the ingress IP address with the `Host` header:
```bash
curl -X POST https://172.232.58.222/v1/chat/completions \
  -H "Host: api.dev.ai-aas.local" \
  -H "X-API-Key: <your-api-key>" \
  ...
```

### 3. Handle Rate Limits
Implement exponential backoff when receiving 429 responses:
```python
import time
from openai import RateLimitError

max_retries = 3
for attempt in range(max_retries):
    try:
        response = client.chat.completions.create(...)
        break
    except RateLimitError:
        if attempt < max_retries - 1:
            time.sleep(2 ** attempt)  # Exponential backoff
        else:
            raise
```

### 4. Set Appropriate Timeouts
The API Router has default timeouts:
- **Read timeout**: 300 seconds
- **Send timeout**: 300 seconds

Set client timeouts accordingly for long-running requests.

### 5. Monitor Usage
Check response headers for usage information:
```
X-RateLimit-Remaining: 95
X-RateLimit-Limit: 100
X-RateLimit-Reset: 1732147260
```

## Troubleshooting

### Certificate Errors
For development environments with self-signed certificates, use `-k` flag in curl:
```bash
curl -k https://api.dev.ai-aas.local/v1/chat/completions ...
```

Or disable certificate verification in client libraries (development only).

### Connection Refused
Verify the ingress endpoint is accessible:
```bash
kubectl get ingress -n development api-router-service-development-api-router-service
```

### Authentication Errors
Verify your API key is valid:
```bash
curl -k -X POST https://api.dev.ai-aas.local/v1/chat/completions \
  -H "X-API-Key: <your-api-key>" \
  -v
```

Check the response status code and error message.

### Model Not Available
List available models to verify the model name:
```bash
curl -k -X GET https://api.dev.ai-aas.local/v1/models \
  -H "X-API-Key: <your-api-key>"
```

## Architecture

```
┌─────────┐     HTTPS      ┌─────────────────┐     HTTP      ┌──────────┐
│ Client  │───────────────>│  API Router     │─────────────>│  vLLM    │
│         │  X-API-Key     │  (Auth + Route) │              │  Backend │
└─────────┘                └─────────────────┘              └──────────┘
                                   │
                                   │ Validate
                                   ▼
                           ┌────────────────┐
                           │  User-Org      │
                           │  Service       │
                           └────────────────┘
```

**Request Flow**:
1. Client sends HTTPS request with API key
2. NGINX ingress terminates TLS
3. API Router validates API key via User-Org Service
4. API Router routes request to backend (vLLM) based on model
5. vLLM generates response
6. API Router returns response to client

## Security

### API Key Management
- Never commit API keys to source control
- Rotate keys regularly in production
- Use environment variables to store keys
- Each organization should have unique keys

### Transport Security
- All production traffic uses TLS 1.2+
- Development environment uses NGINX default certificates
- Production should use valid certificates (Let's Encrypt, etc.)

### Network Isolation
- Backend services (vLLM) are not directly accessible
- All traffic must go through API Router
- API Router enforces authentication and authorization

## Production Considerations

For production deployments:

1. **DNS Configuration**: Use proper DNS instead of IP addresses
   ```
   Base URL: https://api.yourdomain.com
   ```

2. **Valid TLS Certificates**: Use Let's Encrypt or commercial certificates
   ```yaml
   tls:
     - secretName: api-tls
       hosts:
         - api.yourdomain.com
   ```

3. **Rate Limiting**: Configure per-organization limits
4. **Monitoring**: Enable Prometheus metrics and OpenTelemetry tracing
5. **Logging**: Enable structured logging with appropriate log levels
6. **Budget Enforcement**: Configure budget service for usage limits

## Support

For issues or questions:
- Check logs: `kubectl logs -n development -l app=api-router-service`
- Review ArgoCD sync status
- Verify backend health: `kubectl get pods -n system -l app=vllm`
