# Creating API Keys

## Overview

This guide covers creating API keys for accessing the AIaaS platform APIs.

## API Key Overview

### What Are API Keys?

- Credentials for API access
- Scoped to your organization
- Used to authenticate API requests
- Can be rotated and revoked

### When to Create API Keys

- Integrating applications with the platform
- Automated workflows
- Service-to-service communication
- Development and testing

## Creating API Keys

### Via Web Portal

1. Navigate to API Keys section
2. Click "Create API Key"
3. Enter key name and description
4. Select API scopes
5. Set expiration (optional)
6. Create key
7. **Copy key immediately** (shown only once)

### Via API

```bash
curl -X POST https://api.example.com/v1/api-keys \
  -H "Authorization: Bearer <your-token>" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "production-key",
    "scopes": ["inference:read", "inference:write"],
    "expires_in_days": 90
  }'
```

## API Key Configuration

### Key Name

- Descriptive name for the key
- Helps identify key purpose
- Used in logs and audit trails
- Cannot be changed after creation

### API Scopes

Select scopes based on needed access:

- **inference:read**: Read inference capabilities
- **inference:write**: Make inference requests
- **analytics:read**: Access usage analytics
- **admin**: Administrative operations

### Expiration

- Set expiration date (optional)
- Keys expire automatically
- Receive notification before expiration
- Rotate before expiration

## API Key Security

### Best Practices

- **Store securely**: Never commit to version control
- **Use environment variables**: Store in secure configuration
- **Rotate regularly**: Change keys periodically
- **Limit scopes**: Use minimum required scopes
- **Monitor usage**: Review key usage regularly

### Display and Storage

- Full key shown only once at creation
- Subsequent views show partial fingerprint
- Store key securely
- Never share keys publicly

## Using API Keys

### Authentication

Include API key in requests:

```bash
curl -X POST https://api.example.com/v1/inference \
  -H "Authorization: Bearer <your-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-4", "prompt": "Hello"}'
```

### Rate Limits

- Rate limits apply per API key
- Monitor rate limit headers
- Handle rate limit errors gracefully
- Contact support for limit increases

## Related Documentation

- [Managing API Keys](./managing-api-keys.md)
- [API Key Security](./api-key-security.md)
- [Developer Guide](../developer/README.md)

