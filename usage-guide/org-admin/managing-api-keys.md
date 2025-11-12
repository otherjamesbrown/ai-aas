# Managing API Keys

## Overview

This guide covers managing existing API keys, including viewing, rotating, and revoking keys.

## Viewing API Keys

### Key List

- View all organization API keys
- See key name, creation date, last used
- View key status (active, expired, revoked)
- Filter by status or name

### Key Details

- View key metadata
- See assigned scopes
- Check expiration date
- View usage statistics

### Key Fingerprint

- Partial key fingerprint displayed
- Used to identify keys
- Full key never shown after creation
- Regenerate to get new key

## Rotating API Keys

### When to Rotate

- Regular security practice
- Suspected compromise
- Key expiration approaching
- After security incident

### Rotation Process

1. Create new API key
2. Update applications to use new key
3. Verify functionality with new key
4. Revoke old API key
5. Monitor for old key usage

### Zero-Downtime Rotation

- Create new key before revoking old
- Update applications gradually
- Monitor both keys during transition
- Revoke old key after migration complete

## Revoking API Keys

### Revocation Process

1. Navigate to API Keys section
2. Select key to revoke
3. Click "Revoke"
4. Confirm revocation
5. Key immediately invalidated

### Revocation Effects

- Key immediately stops working
- All requests with key are rejected
- Audit log entry created
- Cannot be reactivated

## API Key Monitoring

### Usage Statistics

- View request counts
- See last used timestamp
- Monitor usage patterns
- Identify unused keys

### Usage Alerts

- Alert on unusual usage
- Monitor for abuse
- Track rate limit hits
- Review usage regularly

## Best Practices

### Key Management

- Regularly review key list
- Remove unused keys
- Rotate keys periodically
- Monitor key usage

### Security

- Revoke compromised keys immediately
- Use separate keys per environment
- Limit key scopes
- Document key purposes

## Related Documentation

- [Creating API Keys](./creating-api-keys.md)
- [API Key Security](./api-key-security.md)
- [Developer Guide](../developer/README.md)

