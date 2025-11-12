# Making API Requests

## Overview

This guide covers making API requests to the AIaaS platform for AI inference.

## API Endpoints

### Base URL

- **Production**: `https://api.example.com`
- **Staging**: `https://staging-api.example.com`
- **Development**: `https://dev-api.example.com`

### API Versioning

- Current version: `v1`
- Include version in URL: `/v1/inference`
- Version changes communicated in advance

## Making Requests

### Basic Request

```bash
curl -X POST https://api.example.com/v1/inference \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "prompt": "Hello, world!",
    "max_tokens": 100
  }'
```

### Request Headers

- **Authorization**: Bearer token with API key
- **Content-Type**: application/json
- **User-Agent**: Your application identifier
- **X-Request-ID**: Optional request ID for tracking

## Request Parameters

### Inference Request

- **model**: AI model to use (required)
- **prompt**: Input prompt (required)
- **max_tokens**: Maximum tokens in response
- **temperature**: Sampling temperature
- **top_p**: Nucleus sampling parameter

### Example Request

```json
{
  "model": "gpt-4",
  "prompt": "Explain quantum computing",
  "max_tokens": 500,
  "temperature": 0.7,
  "top_p": 0.9
}
```

## Handling Responses

### Success Response

```json
{
  "id": "req_123",
  "model": "gpt-4",
  "choices": [
    {
      "text": "Response text here",
      "index": 0
    }
  ],
  "usage": {
    "prompt_tokens": 10,
    "completion_tokens": 50,
    "total_tokens": 60
  }
}
```

### Error Response

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Invalid model specified",
    "type": "invalid_request_error"
  }
}
```

## Best Practices

### Request Optimization

- Batch requests when possible
- Use appropriate model for task
- Set reasonable token limits
- Cache responses when appropriate

### Error Handling

- Handle errors gracefully
- Retry on transient errors
- Log errors for debugging
- Monitor API usage

## Related Documentation

- [API Authentication](./api-authentication.md)
- [Error Handling](./error-handling.md)
- [API Reference](./api-reference.md)

