# Error Handling

## Overview

This guide covers handling API errors, retries, and error recovery.

## Error Types

### Client Errors (4xx)

- **400 Bad Request**: Invalid request parameters
- **401 Unauthorized**: Invalid or missing API key
- **403 Forbidden**: Insufficient permissions
- **404 Not Found**: Resource not found
- **429 Too Many Requests**: Rate limit exceeded

### Server Errors (5xx)

- **500 Internal Server Error**: Server error
- **502 Bad Gateway**: Gateway error
- **503 Service Unavailable**: Service unavailable
- **504 Gateway Timeout**: Request timeout

## Error Response Format

### Standard Error Format

```json
{
  "error": {
    "code": "invalid_request",
    "message": "Invalid model specified",
    "type": "invalid_request_error",
    "param": "model",
    "request_id": "req_123"
  }
}
```

## Error Handling Strategies

### Retry Logic

- Retry on transient errors (5xx)
- Exponential backoff
- Maximum retry attempts
- Idempotent requests

### Example Retry Logic

```python
import time
from requests.exceptions import HTTPError

def make_request_with_retry(url, headers, data, max_retries=3):
    for attempt in range(max_retries):
        try:
            response = requests.post(url, headers=headers, json=data)
            response.raise_for_status()
            return response.json()
        except HTTPError as e:
            if e.response.status_code >= 500 and attempt < max_retries - 1:
                wait_time = 2 ** attempt
                time.sleep(wait_time)
                continue
            raise
```

## Handling Specific Errors

### Rate Limit Errors

```python
if response.status_code == 429:
    retry_after = int(response.headers.get('Retry-After', 60))
    time.sleep(retry_after)
    # Retry request
```

### Authentication Errors

```python
if response.status_code == 401:
    # API key invalid or expired
    # Refresh API key or notify user
    pass
```

### Validation Errors

```python
if response.status_code == 400:
    error_data = response.json()
    # Handle validation errors
    print(f"Validation error: {error_data['error']['message']}")
```

## Best Practices

### Error Handling

- Handle all error cases
- Log errors for debugging
- Provide user-friendly messages
- Implement retry logic

### Monitoring

- Monitor error rates
- Track error types
- Alert on high error rates
- Analyze error patterns

## Related Documentation

- [Making API Requests](./making-api-requests.md)
- [Rate Limiting](./rate-limiting.md)
- [Best Practices](./best-practices.md)

