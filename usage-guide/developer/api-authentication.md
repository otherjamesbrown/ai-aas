# API Authentication

## Overview

This guide covers authenticating API requests to the AIaaS platform.

## Authentication Methods

### API Key Authentication

- Primary authentication method
- Bearer token in Authorization header
- Scoped to organization
- Can be rotated and revoked

### Authentication Header

Include API key in Authorization header:

```bash
Authorization: Bearer <your-api-key>
```

## Getting API Keys

### Requesting API Keys

1. Contact your organization administrator
2. Request API key with required scopes
3. Receive API key (shown only once)
4. Store securely

### API Key Scopes

- **inference:read**: Read inference capabilities
- **inference:write**: Make inference requests
- **analytics:read**: Access usage analytics
- **admin**: Administrative operations

## Using API Keys

### Basic Usage

```bash
curl -X POST https://api.example.com/v1/inference \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "prompt": "Hello"}'
```

### SDK Usage

```python
import requests

headers = {
    "Authorization": f"Bearer {api_key}",
    "Content-Type": "application/json"
}

response = requests.post(
    "https://api.example.com/v1/inference",
    headers=headers,
    json={"model": "gpt-4", "prompt": "Hello"}
)
```

## Security Best Practices

### Key Storage

- Never commit keys to version control
- Use environment variables
- Encrypt keys at rest
- Rotate keys regularly

### Key Usage

- Use HTTPS only
- Validate on server side
- Monitor key usage
- Revoke compromised keys

## Error Handling

### Authentication Errors

- **401 Unauthorized**: Invalid or missing API key
- **403 Forbidden**: Insufficient permissions
- **429 Too Many Requests**: Rate limit exceeded

### Handling Errors

```python
try:
    response = requests.post(url, headers=headers, json=data)
    response.raise_for_status()
except requests.HTTPError as e:
    if e.response.status_code == 401:
        # Invalid API key
        pass
    elif e.response.status_code == 403:
        # Insufficient permissions
        pass
```

## Related Documentation

- [Making API Requests](./making-api-requests.md)
- [Error Handling](./error-handling.md)
- [API Reference](./api-reference.md)

