# Rate Limiting

## Overview

This guide covers understanding and handling API rate limits.

## Rate Limits

### Default Limits

- **Requests per minute**: 100
- **Burst limit**: 200 requests per minute
- **Tokens per minute**: Varies by plan
- **Concurrent requests**: 10

### Rate Limit Headers

- `X-RateLimit-Limit`: Total requests allowed
- `X-RateLimit-Remaining`: Requests remaining
- `X-RateLimit-Reset`: Time when limit resets
- `Retry-After`: Seconds to wait before retry

## Handling Rate Limits

### Checking Rate Limits

```python
response = requests.post(url, headers=headers, json=data)

limit = int(response.headers.get('X-RateLimit-Limit', 0))
remaining = int(response.headers.get('X-RateLimit-Remaining', 0))
reset_time = int(response.headers.get('X-RateLimit-Reset', 0))

print(f"Limit: {limit}, Remaining: {remaining}")
```

### Rate Limit Errors

When rate limit is exceeded:

- Status code: `429 Too Many Requests`
- Retry-After header indicates wait time
- Request should be retried after wait period

### Handling Rate Limit Errors

```python
if response.status_code == 429:
    retry_after = int(response.headers.get('Retry-After', 60))
    time.sleep(retry_after)
    # Retry request
```

## Best Practices

### Rate Limit Management

- Monitor rate limit headers
- Implement exponential backoff
- Batch requests when possible
- Cache responses to reduce requests

### Optimization

- Reduce request frequency
- Use appropriate models
- Optimize token usage
- Implement request queuing

## Increasing Limits

### Request Limit Increase

- Contact organization administrator
- Upgrade plan tier
- Request custom limits
- Provide usage justification

## Related Documentation

- [Error Handling](./error-handling.md)
- [Performance Optimization](./performance-optimization.md)
- [Best Practices](./best-practices.md)

