# Troubleshooting

Common errors and how to resolve them.

## Authentication Errors

### 401 Unauthorized - Invalid API Key

```json
{
  "error": {
    "message": "Invalid or missing API key",
    "type": "authentication_error",
    "code": "invalid_api_key"
  }
}
```

**Causes**:
- Missing `X-API-Key` header
- Invalid or revoked API key
- Typo in API key

**Solutions**:
1. Verify you're including the header:
   ```bash
   curl -H "X-API-Key: your-key-here" ...
   ```
2. Check your API key is correct (no extra spaces)
3. Verify the key hasn't been revoked:
   ```bash
   ai-aas-cli apikey list --org-id your-org
   ```
4. Create a new API key if needed

### 403 Forbidden - Access Denied

```json
{
  "error": {
    "message": "Access denied to model",
    "type": "authorization_error",
    "code": "access_denied"
  }
}
```

**Causes**:
- API key doesn't have access to the requested model
- Organization doesn't have model enabled

**Solutions**:
1. Check which models your organization has access to
2. Contact your admin to enable the model

## Rate Limit Errors

### 429 Too Many Requests

```json
{
  "error": {
    "message": "Rate limit exceeded",
    "type": "rate_limit_error",
    "code": "rate_limit_exceeded"
  }
}
```

**Solutions**:
1. Check rate limit headers in the response
2. Implement exponential backoff:
   ```python
   import time

   for attempt in range(3):
       try:
           response = make_request()
           break
       except RateLimitError:
           time.sleep(2 ** attempt)
   ```
3. Reduce request frequency
4. Contact admin for limit increase

## Request Errors

### 400 Bad Request - Invalid Model

```json
{
  "error": {
    "message": "Model 'invalid-model' not found",
    "type": "invalid_request_error",
    "code": "model_not_found"
  }
}
```

**Solutions**:
1. List available models:
   ```bash
   curl https://api.dev.otherjamesbrown.com/v1/models \
     -H "X-API-Key: $AI_AAS_API_KEY"
   ```
2. Use a valid model ID (e.g., `gpt-oss-20b`)

### 400 Bad Request - Missing Messages

```json
{
  "error": {
    "message": "Invalid request: missing required field 'messages'",
    "type": "invalid_request_error",
    "code": "invalid_request"
  }
}
```

**Solutions**:
1. Ensure your request includes `messages` array:
   ```json
   {
     "model": "gpt-oss-20b",
     "messages": [
       {"role": "user", "content": "Hello"}
     ]
   }
   ```
2. Check JSON syntax is valid

## Service Errors

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

**Causes**:
- Model backend is starting up
- Backend is experiencing issues
- Maintenance in progress

**Solutions**:
1. Wait a few minutes and retry
2. Check platform status (if available)
3. Contact support if issue persists

### 504 Gateway Timeout

```json
{
  "error": {
    "message": "Request timed out",
    "type": "service_error",
    "code": "timeout"
  }
}
```

**Causes**:
- Request took too long (long generation)
- Backend overloaded

**Solutions**:
1. Reduce `max_tokens` in your request
2. Use shorter prompts
3. Retry with smaller request

## Connection Errors

### SSL/TLS Certificate Errors

```
SSL: CERTIFICATE_VERIFY_FAILED
```

**For Development** (self-signed certs):
```bash
# curl: skip certificate verification (dev only!)
curl -k https://api.dev.otherjamesbrown.com/...

# Python
import httpx
client = OpenAI(
    api_key="...",
    base_url="...",
    http_client=httpx.Client(verify=False)
)
```

**Warning**: Never disable certificate verification in production.

### Connection Refused

```
Connection refused
```

**Solutions**:
1. Verify the endpoint URL is correct
2. Check your network connection
3. Verify the service is running

## CLI Errors

### Configuration Not Found

```
Error: configuration not found
```

**Solutions**:
```bash
# Run setup wizard
ai-aas-cli --init

# Or set individual values
ai-aas-cli --init-api-key your-key
ai-aas-cli --init-domain dev.otherjamesbrown.com
```

### API Connection Failed

```
Error: API connection failed
```

**Solutions**:
```bash
# Check configuration
ai-aas-cli --init-status

# Test connection
ai-aas-cli config test
```

## Getting Help

If you can't resolve an issue:

1. **Check the logs** (if you have access):
   ```bash
   kubectl logs -n development -l app=api-router-service --tail=100
   ```

2. **Gather information**:
   - Request ID (from response headers)
   - Timestamp of the error
   - Full error message
   - Request payload (sanitize any secrets)

3. **Contact support** with the above information

## Common Mistakes

| Mistake | Fix |
|---------|-----|
| Using `Authorization: Bearer` header | Use `X-API-Key` header instead |
| Wrong model name | Check `/v1/models` for valid names |
| Missing `Content-Type` header | Add `Content-Type: application/json` |
| Expired API key | Create a new key with `ai-aas-cli apikey create` |
| Wrong base URL | Use `https://api.dev.otherjamesbrown.com` |
