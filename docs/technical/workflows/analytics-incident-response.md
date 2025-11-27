# Analytics Service Incident Response Runbook

**Service**: Analytics Service  
**Last Updated**: 2025-01-27  
**Owner**: Platform Engineering Team

## Overview

This runbook provides step-by-step procedures for responding to incidents affecting the Analytics Service, including error rate spikes, latency degradation, freshness lag, and data quality issues.

## Quick Reference

### Key Endpoints

- **Reliability API**: `GET /analytics/v1/orgs/{orgId}/reliability`
- **Usage API**: `GET /analytics/v1/orgs/{orgId}/usage`
- **Export API**: 
  - `POST /analytics/v1/orgs/{orgId}/exports` - Create export job
  - `GET /analytics/v1/orgs/{orgId}/exports` - List export jobs
  - `GET /analytics/v1/orgs/{orgId}/exports/{jobId}` - Get job status
  - `GET /analytics/v1/orgs/{orgId}/exports/{jobId}/download` - Download CSV
- **Health Check**: `GET /analytics/v1/status/healthz`
- **Readiness Check**: `GET /analytics/v1/status/readyz`
- **Metrics**: `GET /metrics`

**Note**: All analytics API endpoints require RBAC headers:
- `X-Actor-Subject`: User/actor identifier
- `X-Actor-Roles`: Comma-separated list of roles (e.g., `analytics:usage:read,admin`)

### Alert Thresholds

- **Error Rate**: > 5% for 5 minutes (critical)
- **Latency P95**: > 2s for 10 minutes (warning)
- **Latency P99**: > 5s for 5 minutes (critical)
- **Freshness Lag**: > 10 minutes (critical)
- **Error Rate Spike**: 3x increase in 15 minutes (critical)
- **Latency Spike**: 2x increase in 15 minutes (warning)

## Common Incidents

### 1. High Error Rate Alert

**Alert**: `AnalyticsServiceHighErrorRate`  
**Severity**: Critical  
**Threshold**: Error rate > 5% for 5 minutes

#### Investigation Steps

1. **Check Reliability API**:
   ```bash
   curl "https://api.ai-aas.local/analytics/v1/orgs/{orgId}/reliability?start={start}&end={end}&granularity=hour" \
     -H "X-Actor-Subject: operator" \
     -H "X-Actor-Roles: analytics:reliability:read,admin"
   ```

2. **Identify Affected Models**:
   - Review the reliability series response
   - Check which models show elevated error rates
   - Look for patterns in error codes

3. **Export Incident Data**:
   ```bash
   # Use incident exporter to generate CSV
   # Time range should cover the incident window
   ```

4. **Check Upstream Services**:
   - Verify API Router service health
   - Check RabbitMQ queue depth
   - Review ingestion batch status

5. **Review Recent Changes**:
   - Check deployment history
   - Review configuration changes
   - Check for database migrations

#### Resolution Steps

1. **If upstream issue**:
   - Escalate to API Router team
   - Monitor ingestion pipeline

2. **If data quality issue**:
   - Check for duplicate events
   - Verify event schema compliance
   - Review ingestion batch dedupe conflicts

3. **If service issue**:
   - Check service logs: `kubectl logs -f deployment/analytics-service`
   - Verify database connectivity
   - Check Redis connectivity
   - Review rollup worker status

### 2. High Latency Alert

**Alert**: `AnalyticsServiceHighLatencyP95` or `AnalyticsServiceHighLatencyP99`  
**Severity**: Warning/Critical  
**Threshold**: P95 > 2s or P99 > 5s

#### Investigation Steps

1. **Check Reliability API**:
   ```bash
   curl "https://api.ai-aas.local/analytics/v1/orgs/{orgId}/reliability?start={start}&end={end}&granularity=hour&percentile=p95" \
     -H "X-Actor-Subject: operator" \
     -H "X-Actor-Roles: analytics:reliability:read,admin"
   ```

2. **Identify Affected Models**:
   - Review latency percentiles by model
   - Check if latency is consistent or sporadic
   - Look for correlation with error rates

3. **Check Database Performance**:
   ```sql
   -- Check for slow queries
   SELECT pid, now() - pg_stat_activity.query_start AS duration, query
   FROM pg_stat_activity
   WHERE (now() - pg_stat_activity.query_start) > interval '5 seconds'
   AND state = 'active';
   ```

4. **Check Rollup Worker**:
   - Verify rollup worker is running
   - Check rollup execution time
   - Review rollup worker logs

#### Resolution Steps

1. **If database issue**:
   - Check database connection pool
   - Review query performance
   - Consider increasing connection pool size

2. **If rollup worker issue**:
   - Restart rollup worker if stalled
   - Check rollup interval configuration
   - Verify rollup queries are optimized

3. **If service resource issue**:
   - Check CPU/memory usage
   - Consider horizontal scaling
   - Review resource limits

### 3. Freshness Lag Alert

**Alert**: `AnalyticsServiceHighFreshnessLag`  
**Severity**: Critical  
**Threshold**: Lag > 10 minutes

#### Investigation Steps

1. **Check Freshness Status**:
   ```sql
   SELECT org_id, model_id, last_event_at, last_rollup_at, lag_seconds, status
   FROM analytics.freshness_status
   WHERE lag_seconds > 600
   ORDER BY lag_seconds DESC;
   ```

2. **Check Rollup Worker**:
   - Verify rollup worker is running
   - Check last successful rollup time
   - Review rollup worker logs

3. **Check Ingestion Pipeline**:
   - Verify RabbitMQ consumer is running
   - Check ingestion batch completion times
   - Review ingestion processor logs

#### Resolution Steps

1. **If rollup worker stalled**:
   - Restart rollup worker
   - Manually trigger rollup if needed
   - Check for database locks

2. **If ingestion pipeline issue**:
   - Restart ingestion consumer
   - Check RabbitMQ connectivity
   - Verify batch processing is working

3. **If database issue**:
   - Check database connectivity
   - Review database performance
   - Verify TimescaleDB hypertable status

### 4. Error Rate Spike Alert

**Alert**: `AnalyticsServiceErrorRateSpike`  
**Severity**: Critical  
**Threshold**: 3x increase in 15 minutes

#### Investigation Steps

1. **Immediate Actions**:
   - Export incident data immediately
   - Check reliability API for attribution
   - Review recent deployments

2. **Export Incident Data**:
   ```go
   // Use incident exporter with time range covering spike
   exporter := exports.NewIncidentExporter(store, logger)
   csv, err := exporter.Export(ctx, exports.ExportRequest{
       OrgID:   orgID,
       Start:   spikeStartTime,
       End:     spikeEndTime,
       MaxRows: 10000,
   })
   ```

3. **Check Upstream**:
   - Verify API Router service status
   - Check for upstream service incidents
   - Review error codes in exported data

#### Resolution Steps

1. **If upstream incident**:
   - Coordinate with API Router team
   - Monitor upstream service recovery
   - Verify data quality after recovery

2. **If data quality issue**:
   - Review exported CSV for patterns
   - Check for schema violations
   - Verify event deduplication is working

### 5. Latency Spike Alert

**Alert**: `AnalyticsServiceLatencySpike`  
**Severity**: Warning  
**Threshold**: 2x increase in 15 minutes

#### Investigation Steps

1. **Check Reliability API**:
   - Review latency percentiles by model
   - Identify which models are affected
   - Check for correlation with traffic patterns

2. **Check Service Resources**:
   - Review CPU/memory metrics
   - Check database connection pool
   - Verify rollup worker performance

#### Resolution Steps

1. **If resource constraint**:
   - Scale service horizontally
   - Increase resource limits
   - Optimize database queries

2. **If database issue**:
   - Check for slow queries
   - Review connection pool usage
   - Consider read replicas for queries

## Export Job Management

### Creating Export Jobs

Export jobs are created via the API and processed asynchronously by the export worker:

```bash
# Create export job
curl -X POST "https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports" \
  -H "X-Actor-Subject: user-123" \
  -H "X-Actor-Roles: analytics:exports:create,admin" \
  -H "Content-Type: application/json" \
  -d '{
    "timeRange": {
      "start": "2025-11-01T00:00:00Z",
      "end": "2025-11-07T23:59:59Z"
    },
    "granularity": "daily"
  }'

# Check job status
curl "https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports/{jobId}" \
  -H "X-Actor-Subject: user-123" \
  -H "X-Actor-Roles: analytics:exports:read,admin"

# Download CSV (when status is succeeded)
curl "https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports/{jobId}/download" \
  -H "X-Actor-Subject: user-123" \
  -H "X-Actor-Roles: analytics:exports:download,admin" \
  -L -o export.csv
```

### Export Job Troubleshooting

**Job stuck in pending**:
- Check export worker logs: `kubectl logs -f deployment/analytics-service | grep export`
- Verify S3 credentials are configured
- Check worker concurrency: `EXPORT_WORKER_CONCURRENCY` (default: 2)
- Verify worker interval: `EXPORT_WORKER_INTERVAL` (default: 30s)

**Job failed**:
- Check job error message: `GET /analytics/v1/orgs/{orgId}/exports/{jobId}`
- Verify S3 bucket exists and credentials have permissions
- Check CSV generation: Review rollup table data for the time range
- Verify time range is valid (max 31 days)

**CSV download fails**:
- Check signed URL expiration (default: 24 hours)
- Verify S3 object exists in bucket
- Check S3 credentials and permissions

## Incident Export

### Using Incident Exporter

The incident exporter generates CSV files of usage events for a specified time range, useful for sharing data during incidents.

**Example Usage**:

```go
exporter := exports.NewIncidentExporter(store, logger)

csv, err := exporter.Export(ctx, exports.ExportRequest{
    OrgID:   orgID,
    ModelID: &modelID, // Optional: filter by model
    Start:   incidentStart,
    End:     incidentEnd,
    MaxRows: 10000, // Default limit
})
```

**CSV Format**:
- `event_id`: Unique event identifier
- `org_id`: Organization ID
- `occurred_at`: Event timestamp (RFC3339)
- `received_at`: Ingestion timestamp
- `model_id`: Model identifier
- `actor_id`: Actor identifier (if available)
- `input_tokens`: Input token count
- `output_tokens`: Output token count
- `latency_ms`: Request latency in milliseconds
- `status`: Event status (success, error, timeout, throttled)
- `error_code`: Error code (if status is error)
- `cost_estimate_cents`: Cost estimate
- `metadata`: JSON metadata

## Monitoring and Dashboards

### Grafana Dashboards

- **Usage Dashboard**: `/dashboards/analytics/usage.json`
- **Reliability Dashboard**: (To be created in Phase 4)

### Key Metrics to Monitor

1. **Error Rate**: `analytics_usage_error_rate`
2. **Latency P95**: `analytics_usage_latency_p95_ms`
3. **Latency P99**: `analytics_usage_latency_p99_ms`
4. **Freshness Lag**: `analytics_freshness_lag_seconds`
5. **Ingestion Rate**: `analytics_ingestion_events_total`
6. **Rollup Success**: `analytics_rollup_last_success_timestamp`

## Escalation

### On-Call Rotation

- **Primary**: Platform Engineering Team
- **Secondary**: Backend Engineering Team
- **Escalation**: Engineering Manager

### When to Escalate

- Service is completely down
- Data loss detected
- Freshness lag > 30 minutes
- Error rate > 20%
- Unable to export incident data

## Post-Incident

### Required Actions

1. **Export Incident Data**: Save CSV export for analysis
2. **Document Timeline**: Record incident start/end times
3. **Root Cause Analysis**: Identify root cause
4. **Update Runbook**: Add any new procedures discovered
5. **Create Incident Report**: Document findings and improvements

### Common Follow-ups

- Review alert thresholds
- Optimize slow queries
- Improve error handling
- Enhance monitoring coverage
- Update documentation

## Related Documentation

- [Analytics Service README](../../services/analytics-service/README.md)
- [Analytics Service Spec](../../specs/007-analytics-service/spec.md)
- [Infrastructure Overview](../platform/infrastructure-overview.md)
- [Observability Guide](../platform/observability-guide.md)

