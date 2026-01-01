# Token Rate-Limit Policies

Token rate-limiting provides fair usage controls for AI inference by limiting the number of tokens a user can consume within rolling time windows.

## Overview

Token rate-limit policies replace the legacy dollar-based budget system with a more accurate, token-based approach that:
- Measures actual model usage (input + output tokens)
- Supports multiple time windows (1 hour, 24 hours, 7 days)
- Uses inheritance with user-level overrides
- Integrates with OpenAI-compatible 429 responses

## Concepts

### Policies

A **token rate-limit policy** defines maximum token consumption per time window. Policies can:
- Set limits for one or more windows (1h, 24h, 7d)
- Be assigned as an organization default
- Be overridden at the user level

### Built-in Policy

Every organization has access to the built-in **"No Token Rate-Limit"** policy (ID: `no-limit`). This policy has no limits and is the default for new organizations.

### Policy Inheritance

Token policies follow a two-level inheritance model:

```
Organization Default Policy
          ↓
    User Override (optional)
          ↓
      Effective Policy
```

1. **Organization default**: Applies to all users in the org
2. **User override**: Optional per-user policy that takes precedence

To check a user's effective policy:
```bash
ai-aas-org user get-token-policy --user alice@example.com
```

### Rolling Windows

Token usage is tracked in rolling windows, not fixed calendar periods:
- **1h window**: Usage in the past 60 minutes
- **24h window**: Usage in the past 24 hours
- **7d window**: Usage in the past 7 days

Windows reset continuously as old usage "rolls off" the window.

## CLI Commands

The `ai-aas-org` CLI provides full token policy management.

### Managing Policies

```bash
# List all policies (includes built-in)
ai-aas-org token-policy list

# Create a new policy
ai-aas-org token-policy create my-policy --1h=10000 --24h=100000 --7d=500000

# Show policy details
ai-aas-org token-policy show my-policy

# Update policy limits
ai-aas-org token-policy update my-policy --1h=20000

# Delete a policy (must not be in use)
ai-aas-org token-policy delete my-policy
```

### Organization Default

```bash
# Get the current org default policy
ai-aas-org token-policy get-default

# Set org default to a custom policy
ai-aas-org token-policy set-default --policy my-policy

# Reset to built-in "no limit" policy
ai-aas-org token-policy set-default --policy no-limit
```

### User Overrides

```bash
# Get a user's effective policy
ai-aas-org user get-token-policy --user alice@example.com

# Set a user-specific override
ai-aas-org user set-token-policy --user alice@example.com --policy restrictive-policy

# Clear override (user inherits org default)
ai-aas-org user set-token-policy --user alice@example.com --policy inherit

# View a user's current usage
ai-aas-org user usage --user alice@example.com
```

### Output Formats

All commands support `--output` flag for different formats:

```bash
# Table format (default)
ai-aas-org token-policy list

# JSON format
ai-aas-org token-policy list --output json

# YAML format
ai-aas-org token-policy list --output yaml
```

## API Endpoints

### Policy Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/orgs/{orgId}/token-policies` | Create policy |
| GET | `/v1/orgs/{orgId}/token-policies` | List policies |
| GET | `/v1/orgs/{orgId}/token-policies/{id}` | Get policy |
| PUT | `/v1/orgs/{orgId}/token-policies/{id}` | Update policy |
| DELETE | `/v1/orgs/{orgId}/token-policies/{id}` | Delete policy |
| GET | `/v1/orgs/{orgId}/token-policy` | Get org default |
| PUT | `/v1/orgs/{orgId}/token-policy` | Set org default |
| GET | `/v1/orgs/{orgId}/users/{userId}/token-policy` | Get user's effective policy |
| PUT | `/v1/orgs/{orgId}/users/{userId}/token-policy` | Set user override |
| DELETE | `/v1/orgs/{orgId}/users/{userId}/token-policy` | Clear user override |

### Usage Queries

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/usage/tokens` | Get own usage (authenticated user) |
| GET | `/v1/orgs/{orgId}/users/{userId}/usage/tokens` | Get user's usage (admin) |

### Internal Endpoints (M2M)

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/internal/v1/usage/tokens` | Record token usage |
| GET | `/internal/v1/users/{userId}/rate-limit` | Check rate limit status |

## Policy Response Format

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "org_id": "123e4567-e89b-12d3-a456-426614174000",
  "name": "my-policy",
  "description": "Standard user policy",
  "limit_1h": 10000,
  "limit_24h": 100000,
  "limit_7d": 500000,
  "is_builtin": false,
  "created_at": "2026-01-01T12:00:00Z",
  "updated_at": "2026-01-01T12:00:00Z"
}
```

## Usage Response Format

```json
{
  "user_id": "789e0123-e89b-12d3-a456-426614174000",
  "org_id": "123e4567-e89b-12d3-a456-426614174000",
  "policy_id": "550e8400-e29b-41d4-a716-446655440000",
  "policy_name": "my-policy",
  "windows": [
    {
      "window": "1h",
      "limit": 10000,
      "used": 2500,
      "remaining": 7500,
      "percentage": 25.0,
      "resets_at": "2026-01-01T13:00:00Z"
    },
    {
      "window": "24h",
      "limit": 100000,
      "used": 45000,
      "remaining": 55000,
      "percentage": 45.0,
      "resets_at": "2026-01-02T12:00:00Z"
    }
  ]
}
```

## Rate Limit Response (429)

When a user exceeds their token limit, the API router returns an OpenAI-compatible 429 response:

```json
{
  "error": {
    "type": "tokens",
    "code": "rate_limit_exceeded",
    "message": "Token rate limit exceeded for 1h window. Used 10000/10000 tokens. Retry after 45m.",
    "param": null
  }
}
```

Response headers include:
- `Retry-After`: Seconds until rate limit resets
- `X-RateLimit-Limit-Tokens`: Token limit for the blocking window
- `X-RateLimit-Remaining-Tokens`: Remaining tokens (0 when blocked)
- `X-RateLimit-Reset-Tokens`: Unix timestamp when limit resets

## Best Practices

### Policy Design

1. **Start permissive**: Use generous limits initially, then tune based on actual usage
2. **Use multiple windows**: Combine short (1h) and long (7d) windows for burst and sustained control
3. **Leave room for overhead**: Token counts include both input and output tokens

### Monitoring

1. Check usage regularly:
   ```bash
   ai-aas-org user usage --user alice@example.com
   ```

2. Monitor for users approaching limits (>80% in any window)

3. Set up alerts for repeated 429 responses in your observability stack

### Troubleshooting

**User reports 429 errors:**
```bash
# Check their effective policy
ai-aas-org user get-token-policy --user alice@example.com

# Check their current usage
ai-aas-org user usage --user alice@example.com

# If legitimate need, either:
# 1. Increase their policy limits
ai-aas-org token-policy update their-policy --1h=20000

# 2. Or assign them a different policy
ai-aas-org user set-token-policy --user alice@example.com --policy higher-limit
```

**Cannot delete a policy:**
```bash
# Policy is in use - find where
ai-aas-org token-policy show the-policy

# Reassign users/org to a different policy before deleting
```

## Migration from Budget System

The token rate-limit system replaces the legacy dollar-based budget system.

### Key Differences

| Aspect | Budget (Legacy) | Token Rate-Limit |
|--------|-----------------|------------------|
| Unit | USD dollars | Tokens (model-agnostic) |
| Windows | Monthly fixed | Rolling (1h, 24h, 7d) |
| Scope | Organization | Organization + User |
| Override | Request approval | Admin sets policy |

### Migration Steps

1. The `budget` CLI command shows a deprecation warning
2. Create equivalent token policies for your use cases
3. Assign policies to organizations/users
4. Monitor usage with the new commands
5. The legacy budget endpoints will be removed in a future release

## Related Documentation

- [Model Access Control](./model-access-control.md) - Access control for AI models
- [Routing Policies](./routing-policies.md) - Request routing configuration
- [Observability Guide](./observability-guide.md) - Monitoring and alerting
