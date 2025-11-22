# E2E Admin API Key for Development Cluster

**Created**: 2025-11-22
**Environment**: Development Cluster (LKE 531921)

## Credentials

### Organization
- **Org ID**: `aa6f9015-132a-4694-8b10-7d4d4550faed`
- **Slug**: `e2e-admin-org`
- **Name**: E2E Test Admin Organization

### User
- **User ID**: `69d27b5d-00ba-4180-bbfa-2ce4ef5fa207`
- **Email**: `e2e-admin@ai-aas.dev`
- **Password**: `e2e-admin-password`
- **Display Name**: E2E Admin User

### API Key
- **API Key ID**: `9eae3441-1db9-4548-8476-4e87a0a59a72`
- **API Key Secret**: `1z_V2QVOJJt0d2f2aQI9PUwQfudAugmOs4issi96jv0`
- **Scopes**: `["*"]` (all scopes)
- **Status**: Active
- **Expires**: 2026-11-22

## Usage

### Quick Setup

The credentials are saved in `.env.e2e` at the project root:

```bash
# Load credentials into your environment
export $(grep -v '^#' .env.e2e | xargs)

# Or source the file
set -a; source .env.e2e; set +a

# Verify
echo $E2E_API_KEY
```

### For E2E Tests

Use the API key secret with the API Router Service:

```bash
# Example API call with the E2E admin API key
curl -X POST https://api.dev.ai-aas.local/v1/chat/completions \
  -H "X-API-Key: 1z_V2QVOJJt0d2f2aQI9PUwQfudAugmOs4issi96jv0" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

### For API Router Integration Tests

Set as environment variable:

```bash
export E2E_API_KEY="1z_V2QVOJJt0d2f2aQI9PUwQfudAugmOs4issi96jv0"
export E2E_ORG_ID="aa6f9015-132a-4694-8b10-7d4d4550faed"
```

### Validating the API Key

You can validate the API key using the user-org-service validation endpoint:

```bash
curl -X POST http://localhost:8081/v1/auth/validate-api-key \
  -H "Content-Type: application/json" \
  -d '{"apiKeySecret":"1z_V2QVOJJt0d2f2aQI9PUwQfudAugmOs4issi96jv0"}'
```

Expected response:
```json
{
  "valid": true,
  "apiKeyId": "9eae3441-1db9-4548-8476-4e87a0a59a72",
  "organizationId": "aa6f9015-132a-4694-8b10-7d4d4550faed",
  "principalId": "69d27b5d-00ba-4180-bbfa-2ce4ef5fa207",
  "principalType": "user",
  "scopes": ["*"],
  "status": "active",
  "expiresAt": "2026-11-22T13:48:31Z"
}
```

## Security Notes

⚠️ **IMPORTANT**: This API key has full admin access (`*` scope) and should only be used for:
- E2E testing in the development cluster
- Development and debugging purposes
- Never use this key in production or share it publicly

## Recreation Process

If you need to recreate this API key, see [SETUP_E2E_ADMIN_KEY.md](./SETUP_E2E_ADMIN_KEY.md) for detailed instructions.

## See Also

- [Development Cluster Configuration](../DEPLOYED_ENDPOINTS.md)
- [API Router Service Documentation](../services/README.md)
- [E2E Testing Guide](../../tests/README.md)
