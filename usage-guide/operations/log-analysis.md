# Log Analysis

## Overview

This guide covers analyzing logs to troubleshoot issues, investigate incidents, and understand system behavior.

## Log Types

### Application Logs

- Service-specific logs
- Request/response logs
- Business logic logs
- Error logs

### System Logs

- Operating system logs
- Container logs
- Infrastructure logs
- Security logs

### Audit Logs

- Access logs
- Authentication logs
- Authorization logs
- Administrative action logs

## Log Analysis Tools

### Centralized Logging

- Access logs via logging platform
- Search across all services
- Filter by time, service, level
- Export logs for analysis

### Command-Line Tools

```bash
# View logs
kubectl logs <pod-name>

# Follow logs
kubectl logs -f <pod-name>

# Search logs
kubectl logs <pod-name> | grep "error"
```

## Common Log Analysis Tasks

### Error Investigation

1. Search for error patterns
2. Filter by time range
3. Correlate with other events
4. Identify root cause

### Performance Analysis

1. Search for slow requests
2. Analyze request patterns
3. Identify bottlenecks
4. Optimize performance

### Security Investigation

1. Review authentication logs
2. Check for unauthorized access
3. Analyze access patterns
4. Investigate security events

## Log Search Patterns

### Common Searches

- **Errors**: `level=error`
- **Warnings**: `level=warning`
- **Specific user**: `user_id=123`
- **Time range**: `timestamp:[2025-01-15 TO 2025-01-16]`
- **Service**: `service=api-router-service`

### Advanced Queries

- **Correlation ID**: Track request across services
- **Error patterns**: Identify recurring errors
- **Performance**: Find slow requests
- **Security**: Detect suspicious activity

## Log Retention

### Retention Policies

- **Application logs**: 30 days
- **Audit logs**: 1 year
- **Security logs**: 2 years
- **Archive logs**: 7 years

### Log Export

- Export logs for analysis
- Archive logs for compliance
- Backup critical logs
- Regular cleanup

## Best Practices

### Logging Standards

- Use structured logging
- Include correlation IDs
- Log at appropriate levels
- Avoid logging sensitive data

### Analysis Workflow

1. Start with high-level overview
2. Narrow down to specific time/service
3. Correlate with other events
4. Document findings

## Related Documentation

- [Service Health Checks](./service-health-checks.md)
- [Incident Response](./incident-response.md)
- [Troubleshooting Guide](../troubleshooting.md)

