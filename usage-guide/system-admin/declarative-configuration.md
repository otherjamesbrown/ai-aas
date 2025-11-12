# Declarative Configuration

## Overview

This guide covers using Git-as-source-of-truth for managing platform and organization configuration, including drift detection and reconciliation.

## Declarative Management Overview

Declarative configuration allows you to:
- Manage configuration via Git
- Enable reviewable changes
- Detect configuration drift
- Automatically reconcile changes

## Enabling Declarative Mode

### For Organizations

1. Enable declarative mode in organization settings
2. Configure Git repository URL
3. Set branch and path
4. Configure reconciliation schedule

### For System Configuration

1. Configure system-level declarative settings
2. Set up Git repository access
3. Configure reconciliation policies
4. Enable drift detection

## Configuration Structure

### Organization Configuration

```yaml
organization:
  name: "Example Org"
  slug: "example-org"
  plan_tier: "growth"
  budget_limit_tokens: 1000000
  members:
    - email: "admin@example.com"
      role: "admin"
  api_keys:
    - name: "production-key"
      scopes: ["inference:read", "inference:write"]
```

### System Configuration

```yaml
system:
  policies:
    mfa_required_roles: ["admin", "owner"]
    default_budget_limit: 100000
  features:
    declarative_mode: true
    drift_detection: true
```

## Reconciliation Process

### Automatic Reconciliation

- Runs on configured schedule
- Detects changes in Git repository
- Applies changes to system
- Reports reconciliation status

### Manual Reconciliation

Trigger manual reconciliation:
- Via Admin CLI
- Via API endpoint
- Via web portal

### Drift Detection

- Compares Git state with system state
- Reports differences
- Optionally auto-corrects drift
- Generates drift reports

## Conflict Resolution

### Handling Conflicts

When manual changes conflict with Git:

1. System detects conflict
2. Reports conflict details
3. Options:
   - Accept Git version (overwrite manual changes)
   - Keep manual changes (update Git)
   - Manual resolution required

### Conflict Prevention

- Require all changes via Git
- Lock manual changes when declarative enabled
- Regular reconciliation to catch drift early

## Best Practices

### Git Workflow

- Use feature branches for changes
- Require pull request reviews
- Test changes in staging first
- Tag releases for production

### Configuration Management

- Version control all configuration
- Document configuration changes
- Regular reconciliation checks
- Monitor drift reports

## Troubleshooting

Common issues:

- **Reconciliation failures**: Check Git access and permissions
- **Drift detection**: Review manual changes
- **Conflicts**: Resolve via conflict resolution process
- **Sync delays**: Check reconciliation schedule

## Related Documentation

- [User-Org Service Specification](../../specs/005-user-org-service/spec.md)
- [GitOps Documentation](../../gitops/README.md)
- [Admin CLI Documentation](../admin-cli.md)

