# AI Coding Assistant Debugging Workflow

This guide provides step-by-step instructions for AI coding assistants to debug issues in the AI-AAS platform using the centralized logging infrastructure.

## Quick Reference

| Component | URL | Notes |
|-----------|-----|-------|
| Grafana | `https://grafana.172.232.58.222.nip.io` | Log visualization dashboard |
| Loki | `https://loki.172.232.58.222.nip.io` | Direct log API access |
| ArgoCD | `https://argocd.172.232.58.222.nip.io` | Deployment status |

## Prerequisites

Before debugging, ensure you have:

1. **Kubeconfig access**: `secrets/kubeconfigs/kubeconfig-development.yaml`
2. **kubectl configured**: `export KUBECONFIG=/path/to/kubeconfig-development.yaml`
3. **Access to log endpoints** (verified via curl)

## Debugging Workflow

### Step 1: Identify the Issue

When a test or deployment fails, gather initial context:

```bash
# Check pod status for the failing service
kubectl get pods -n development -l app=<service-name>

# Get recent events
kubectl get events -n development --sort-by='.lastTimestamp' | tail -20
```

### Step 2: Access Logs via Grafana

The preferred method for log analysis is through Grafana's Explore feature:

1. Navigate to `https://grafana.172.232.58.222.nip.io`
2. Go to **Explore** (compass icon)
3. Select **Loki** as the data source
4. Use LogQL queries (see examples below)

### Step 3: LogQL Query Examples

#### Find Errors for a Specific Service

```logql
{namespace="development", app="api-router-service"} |= "error" | json
```

#### Find Logs with Specific Request ID

```logql
{namespace="development"} | json | request_id="abc123"
```

#### Find Logs with Specific Trace ID

```logql
{namespace="development"} | json | trace_id="4bf92f3577b34da6a3ce929d0e0e4736"
```

#### View HTTP 5xx Errors

```logql
{namespace="development"} | json | status >= 500
```

#### Search by Time Range (last 5 minutes)

```logql
{namespace="development", app="user-org-service"} | json
```
Then set the time range in the Grafana UI.

### Step 4: Direct Loki API Access

For programmatic access (useful in scripts):

```bash
# Query Loki directly
curl -k "https://loki.172.232.58.222.nip.io/loki/api/v1/query_range" \
  --data-urlencode 'query={namespace="development"}' \
  --data-urlencode 'limit=100' \
  --data-urlencode 'start='$(date -u -v-5M +%s)000000000 \
  --data-urlencode 'end='$(date -u +%s)000000000
```

### Step 5: Correlate with Traces

If OpenTelemetry tracing is enabled:

1. Find the `trace_id` in the logs
2. Search for all logs with that trace ID:
   ```logql
   {namespace="development"} | json | trace_id="<trace_id>"
   ```

## Common Debugging Scenarios

### Scenario 1: API Request Failure

1. Get the request ID from the error response or client logs
2. Query Loki for the request:
   ```logql
   {namespace="development"} | json | request_id="<request_id>"
   ```
3. Check the status code and error message
4. Look for upstream service errors in the same time window

### Scenario 2: Pod Crash Loop

1. Check pod status and restart count:
   ```bash
   kubectl get pods -n development -l app=<service> -o wide
   ```
2. Get the crash logs:
   ```bash
   kubectl logs -n development <pod-name> --previous
   ```
3. Check Loki for logs before the crash:
   ```logql
   {namespace="development", pod=~"<pod-name>.*"} | json
   ```

### Scenario 3: Database Connection Issues

1. Search for database errors:
   ```logql
   {namespace="development"} |= "database" |= "error" | json
   ```
2. Check connection pool status in metrics
3. Verify database pod health:
   ```bash
   kubectl get pods -n development -l app=postgresql
   ```

### Scenario 4: Authentication Failures

1. Search for auth-related logs:
   ```logql
   {namespace="development", app="user-org-service"} | json | level="warn" OR level="error"
   ```
2. Look for `auth_failure_reason` in structured logs
3. Verify JWT token validity and expiration

### Scenario 5: Performance Issues

1. Filter for slow requests:
   ```logql
   {namespace="development"} | json | duration > 1s
   ```
2. Check for high latency patterns
3. Review resource usage in Grafana dashboards

## Log Field Reference

The structured logs contain these common fields:

| Field | Description | Example |
|-------|-------------|---------|
| `timestamp` | ISO 8601 timestamp | `2024-01-15T10:30:45Z` |
| `level` | Log level | `info`, `warn`, `error` |
| `msg` | Log message | `request_completed` |
| `service` | Service name | `api-router-service` |
| `trace_id` | OpenTelemetry trace ID | `4bf92f3577b34da6...` |
| `request_id` | Request correlation ID | `abc123` |
| `method` | HTTP method | `POST` |
| `path` | Request path | `/v1/inference` |
| `status` | HTTP status code | `200`, `500` |
| `duration` | Request duration | `150ms` |
| `error` | Error message | `connection refused` |

## Automated Debug Script

Use this script to quickly gather debug information:

```bash
#!/bin/bash
# debug-service.sh - Quick service debugging

SERVICE=${1:-api-router-service}
NAMESPACE=${2:-development}
KUBECONFIG=${3:-secrets/kubeconfigs/kubeconfig-development.yaml}

echo "=== Pod Status ==="
kubectl --kubeconfig=$KUBECONFIG get pods -n $NAMESPACE -l app=$SERVICE

echo ""
echo "=== Recent Events ==="
kubectl --kubeconfig=$KUBECONFIG get events -n $NAMESPACE \
  --field-selector involvedObject.name=$(kubectl --kubeconfig=$KUBECONFIG get pods -n $NAMESPACE -l app=$SERVICE -o jsonpath='{.items[0].metadata.name}') \
  --sort-by='.lastTimestamp' 2>/dev/null || echo "No events found"

echo ""
echo "=== Recent Logs ==="
kubectl --kubeconfig=$KUBECONFIG logs -n $NAMESPACE -l app=$SERVICE --tail=50
```

## Troubleshooting Tips

1. **No logs appearing in Grafana?**
   - Verify Promtail is running: `kubectl get pods -n system -l app=promtail`
   - Check pod has `logging.enabled: "true"` annotation
   - Verify Loki is healthy: `kubectl get pods -n system -l app=loki`

2. **Logs are not in JSON format?**
   - Service may not be using structured logging
   - Check service configuration for LOG_FORMAT setting

3. **Can't find specific request?**
   - Expand the time range
   - Check if the service was restarted (logs are retained for 14 days)
   - Verify request ID is correct

4. **Grafana dashboard not loading?**
   - Check ingress status: `kubectl get ingress -n system`
   - Verify TLS certificate is valid
   - Check Grafana pod logs

## Integration with CI/CD

For automated testing and debugging:

```yaml
# In your CI workflow
- name: Check for errors after test
  run: |
    # Wait for logs to be ingested
    sleep 30

    # Query for errors in the test time window
    ERROR_COUNT=$(curl -s -k "https://loki.172.232.58.222.nip.io/loki/api/v1/query" \
      --data-urlencode 'query=count_over_time({namespace="development"} |= "error" [5m])' \
      | jq '.data.result[0].value[1] // "0"')

    if [ "$ERROR_COUNT" -gt "0" ]; then
      echo "Found $ERROR_COUNT errors in logs"
      # Fetch and display the errors
      curl -s -k "https://loki.172.232.58.222.nip.io/loki/api/v1/query_range" \
        --data-urlencode 'query={namespace="development"} |= "error"' \
        --data-urlencode 'limit=10'
    fi
```

## Related Documentation

- [Environment Access Guide](../platform/environment-access.md) - Credentials and endpoints
- [Observability Guide](../platform/observability-guide.md) - Full observability setup
- [Infrastructure Troubleshooting](./infrastructure-troubleshooting.md) - General infrastructure issues
