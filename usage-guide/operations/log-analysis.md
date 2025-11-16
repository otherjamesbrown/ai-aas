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

### Local Development

For local development and testing, logs are aggregated via Loki and accessible via Make targets:

```bash
# View logs via Loki/Grafana (opens Grafana Explore if available)
make logs-view

# View logs for specific service
make logs-view SERVICE=user-org-service

# Tail logs from Docker Compose (all services)
make logs-tail

# Tail logs for specific service
make logs-tail SERVICE=postgres

# Filter for error-level logs
make logs-error

# Filter for error-level logs from specific service
make logs-error SERVICE=user-org-service

# View logs for specific service from Loki
make logs-service SERVICE=user-org-service

# Enable verbose logging (requires service restart)
make logs-verbose
```

**Loki Access**:
- **API**: `http://localhost:3100`
- **Promtail**: `http://localhost:9080`
- **Grafana** (if available): `http://localhost:3000`

**LogQL Queries** (for direct Loki API access):
```bash
# All logs from local-dev environment
{environment="local-dev"}

# Logs from specific service
{service="user-org-service"}

# Error logs only
{environment="local-dev"} |= "error"

# Logs with specific request ID
{environment="local-dev"} | json | request_id="req-123"
```

### Remote Development / Production

- Access logs via Grafana Explore (Loki queries)
- Search across all services
- Filter by time, service, level
- Export logs for analysis

### Command-Line Tools

```bash
# View logs from Kubernetes
kubectl logs <pod-name>

# Follow logs
kubectl logs -f <pod-name>

# Search logs
kubectl logs <pod-name> | grep "error"

# View logs from Loki via logcli (if installed)
logcli query '{service="api-router-service"}' --addr=http://loki:3100
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

- **Use shared logging package**: All Go services MUST use `shared/go/logging` package
- **Structured logging**: JSON format with standardized fields (`service`, `environment`, `trace_id`, `request_id`, `user_id`, `org_id`)
- **Include correlation IDs**: Use `WithRequestID()`, `WithUserID()`, `WithOrgID()` helpers
- **Log at appropriate levels**: Use `LOG_LEVEL` environment variable (debug, info, warn, error)
- **Avoid logging sensitive data**: Use `logging.RedactString()` or `logging.RedactFields()` for sensitive values
- **OpenTelemetry integration**: Use `WithContext()` to automatically include trace context

### Analysis Workflow

1. Start with high-level overview
2. Narrow down to specific time/service
3. Correlate with other events
4. Document findings

## Related Documentation

- [Service Health Checks](./service-health-checks.md)
- [Incident Response](./incident-response.md)
- [Troubleshooting Guide](../troubleshooting.md)

