# Role Management

## Overview

This guide covers managing user roles, permissions, and access control within your organization.

## Available Roles

### Owner

- Full organization control
- Can manage all settings
- Can delete organization
- Cannot be removed if last owner

### Admin

- Administrative access
- Can manage members and API keys
- Can configure budgets
- Cannot delete organization

### Manager

- Team management access
- Can view usage analytics
- Can manage team members
- Limited administrative access

### Analyst

- Read-only analytics access
- Can view usage reports
- Cannot modify settings
- View-only access to dashboards

### Developer

- API access
- Can use API keys
- Limited to API operations
- Cannot access web portal admin features

## Assigning Roles

### Assigning Roles to Members

1. Navigate to Members section
2. Select member
3. Click "Edit Role"
4. Select new role
5. Save changes

### Role Assignment Rules

- At least one Owner required
- Cannot remove last Owner
- Role changes require confirmation
- Audit log entry created

## Custom Roles

### Creating Custom Roles

- Define custom role permissions
- Assign specific scopes
- Configure role restrictions
- Apply to members

### Custom Role Permissions

- API scopes
- Feature access
- Data access levels
- Administrative permissions

## Role Permissions Matrix

| Permission | Owner | Admin | Manager | Analyst | Developer |
|------------|-------|-------|---------|---------|-----------|
| Manage Members | ✓ | ✓ | ✓ | ✗ | ✗ |
| Manage API Keys | ✓ | ✓ | ✗ | ✗ | ✗ |
| Configure Budgets | ✓ | ✓ | ✗ | ✗ | ✗ |
| View Analytics | ✓ | ✓ | ✓ | ✓ | ✗ |
| Delete Organization | ✓ | ✗ | ✗ | ✗ | ✗ |
| API Access | ✓ | ✓ | ✗ | ✗ | ✓ |

## Best Practices

### Role Management

- Use least privilege principle
- Regularly review role assignments
- Remove unnecessary permissions
- Document role purposes

### Security

- Limit Owner role assignments
- Require MFA for sensitive roles
- Monitor role changes
- Review access regularly

## Related Documentation

- [Inviting Members](./inviting-members.md)
- [Access Control](./access-control.md)
- [API Key Security](./api-key-security.md)

