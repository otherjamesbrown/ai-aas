# Break-Glass Operations

## Overview

Break-glass operations are emergency procedures used when normal access methods are unavailable or when immediate administrative action is required.

## When to Use Break-Glass Procedures

- Normal authentication methods are unavailable
- Recovery from security incidents
- Emergency service restoration
- Credential recovery scenarios
- System-wide policy changes requiring immediate action

## Break-Glass Access Methods

### Direct Database Access

For emergency data access or recovery:

1. Obtain database credentials from secure vault
2. Connect directly to database
3. Perform necessary operations
4. Document all actions in audit log

**Warning**: Direct database access bypasses normal access controls. Use only when necessary and document thoroughly.

### Kubernetes Direct Access

For emergency service management:

1. Access Kubernetes cluster via `kubectl`
2. Use service account with elevated permissions
3. Perform necessary operations
4. Document actions

### Admin CLI

Use the Admin CLI for break-glass operations:

```bash
# Bootstrap first admin
./admin-cli bootstrap

# Rotate credentials
./admin-cli rotate-credentials --confirm

# Emergency user management
./admin-cli user create --break-glass
```

## Emergency Procedures

### Service Recovery

1. Identify affected service
2. Check service logs and metrics
3. Restart service if needed
4. Verify service health
5. Document incident

### Credential Recovery

1. Verify identity through secondary method
2. Rotate compromised credentials
3. Revoke old credentials immediately
4. Notify affected users
5. Review audit logs for unauthorized access

### Data Recovery

1. Identify data loss or corruption
2. Restore from latest backup
3. Verify data integrity
4. Document recovery process
5. Review backup procedures

## Security Considerations

- All break-glass actions must be logged
- Require multiple approvals for sensitive operations
- Time-bound access where possible
- Regular review of break-glass usage
- Post-incident analysis required

## Audit and Compliance

- All break-glass operations generate audit logs
- Review break-glass usage regularly
- Document justification for each use
- Report to security team

## Related Documentation

- [Admin CLI Documentation](../admin-cli.md)
- [Disaster Recovery](./disaster-recovery.md)
- [Credential Management](./credential-management.md)

