# Sample Metrics Report

The CI pipeline generates JSON telemetry files for each run. Example payload:

```json
{
  "run_id": "ci-2025-11-08-001",
  "service": "user-org-service",
  "command": "make check",
  "status": "success",
  "started_at": "2025-11-08T15:04:05Z",
  "finished_at": "2025-11-08T15:06:07Z",
  "duration_seconds": 122.3,
  "commit_sha": "abcdef1234567890",
  "actor": "dev@example.com",
  "environment": "github-actions",
  "collector_version": "1.0.0"
}
```

To analyze metrics:

```bash
aws s3 ls s3://ai-aas-build-metrics/metrics/2025/11/08/
aws s3 cp s3://ai-aas-build-metrics/metrics/2025/11/08/ci-2025-11-08-001.json -
```

Use these reports to track build/test timing trends, detect regressions, and provide evidence for NFR compliance.

---

# Analytics Export Reports

The analytics service provides finance-friendly CSV exports of usage and cost data for reconciliation and reporting purposes.

## Overview

Analytics exports enable finance stakeholders to self-serve month-to-date cost exports with audit trails and retention controls. Exports are generated from pre-aggregated rollup tables, ensuring fast generation and accurate reconciliation.

## Requesting Exports

### Via API

Exports are requested via the Analytics Export API:

```bash
# Create an export job
curl -X POST https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "timeRange": {
      "start": "2024-01-01T00:00:00Z",
      "end": "2024-01-31T23:59:59Z"
    },
    "granularity": "daily"
  }'
```

**Response** (202 Accepted):
```json
{
  "jobId": "550e8400-e29b-41d4-a716-446655440000",
  "orgId": "123e4567-e89b-12d3-a456-426614174000",
  "status": "pending",
  "granularity": "daily",
  "timeRange": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-31T23:59:59Z"
  },
  "createdAt": "2024-01-15T10:00:00Z"
}
```

### Check Job Status

```bash
# Get job status
curl https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports/{jobId} \
  -H "Authorization: Bearer {token}"
```

**Response** (200 OK):
```json
{
  "jobId": "550e8400-e29b-41d4-a716-446655440000",
  "orgId": "123e4567-e89b-12d3-a456-426614174000",
  "status": "succeeded",
  "granularity": "daily",
  "timeRange": {
    "start": "2024-01-01T00:00:00Z",
    "end": "2024-01-31T23:59:59Z"
  },
  "createdAt": "2024-01-15T10:00:00Z",
  "completedAt": "2024-01-15T10:02:30Z",
  "outputUri": "https://us-east-1.linodeobjects.com/analytics-exports/...",
  "checksum": "sha256:abc123...",
  "rowCount": 31,
  "initiatedBy": "user-uuid"
}
```

### Download Export

```bash
# Get download URL (302 redirect)
curl -L https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports/{jobId}/download \
  -H "Authorization: Bearer {token}"
```

The API returns a 302 redirect to a signed Linode Object Storage URL valid for 24 hours.

### List Export Jobs

```bash
# List all exports for an organization
curl https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports \
  -H "Authorization: Bearer {token}"

# Filter by status
curl "https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports?status=succeeded" \
  -H "Authorization: Bearer {token}"
```

## Export Formats

### Granularity Options

- **hourly**: Hour-by-hour breakdown (useful for detailed analysis)
- **daily**: Day-by-day breakdown (default, recommended for monthly reports)
- **monthly**: Month-by-month aggregation (useful for quarterly/annual reports)

### CSV Schema

All exports use the following CSV schema:

| Column | Type | Description |
|--------|------|-------------|
| `bucket_start` | ISO 8601 timestamp | Start of the time bucket |
| `organization_id` | UUID | Organization identifier |
| `model_id` | UUID (nullable) | Model identifier (empty if aggregated across all models) |
| `request_count` | Integer | Number of requests in this bucket |
| `tokens_total` | Integer | Total tokens (input + output) |
| `error_count` | Integer | Number of error responses |
| `cost_total` | Decimal (4 decimal places) | Total cost estimate in dollars |

**Example CSV**:
```csv
bucket_start,organization_id,model_id,request_count,tokens_total,error_count,cost_total
2024-01-01T00:00:00Z,123e4567-e89b-12d3-a456-426614174000,550e8400-e29b-41d4-a716-446655440000,1000,100000,5,10.5000
2024-01-02T00:00:00Z,123e4567-e89b-12d3-a456-426614174000,550e8400-e29b-41d4-a716-446655440000,1100,110000,6,11.5500
```

## Reconciliation Procedures

Exports are generated from pre-aggregated rollup tables (`analytics_hourly_rollups`, `analytics_daily_rollups`). To verify accuracy:

1. **Download the CSV export**
2. **Sum the totals** from the CSV:
   - Sum `request_count` column
   - Sum `tokens_total` column
   - Sum `error_count` column
   - Sum `cost_total` column
3. **Compare with rollup queries** (via API or direct database access)
4. **Verify reconciliation** within 1% tolerance

The integration test suite (`tests/analytics/integration/export_reconciliation_test.go`) validates that CSV totals reconcile with rollup queries automatically.

## Retention Policy

### Signed URLs

- **Expiration**: 24 hours from generation
- **Regeneration**: Request a new export job to generate a new signed URL
- **Access**: Signed URLs provide temporary, secure access to exports

### Data Retention

- **Raw events**: Compressed after 7 days, dropped after 400 days (13 months + buffer)
- **Rollup tables**: 13 months of data retained
- **Export jobs**: Job metadata retained for audit trail; CSV files stored in Linode Object Storage
- **CSV files**: Retained for 90 days in Linode Object Storage, after which jobs can regenerate from rollup tables

### Monthly Export Workflows

For monthly reconciliation:

1. **Request export** on the 1st of each month for the previous month
2. **Use daily granularity** for detailed day-by-day breakdown
3. **Download CSV** within 24 hours (or request new export if expired)
4. **Reconcile totals** with finance systems
5. **Archive CSV** locally for audit purposes

**Example monthly export request**:
```bash
# Export January 2024 data (requested on Feb 1, 2024)
curl -X POST https://api.ai-aas.local/analytics/v1/orgs/{orgId}/exports \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "timeRange": {
      "start": "2024-01-01T00:00:00Z",
      "end": "2024-02-01T00:00:00Z"
    },
    "granularity": "daily"
  }'
```

## Limitations

- **Maximum time range**: 31 days per export job
- **Concurrent jobs**: Limited by worker concurrency (default: 2 concurrent jobs)
- **Processing time**: Typically completes within 1-5 minutes depending on data volume
- **Large exports**: For very large time ranges, consider splitting into multiple monthly exports

## Troubleshooting

### Export Job Stuck in "pending" Status

- **Check worker status**: Verify export worker is running
- **Check job queue**: Review pending jobs via API
- **Review logs**: Check analytics service logs for worker errors

### Export Job Failed

- **Check error message**: Review `error` field in job status response
- **Common causes**:
  - Invalid time range
  - Database connection issues
  - Linode Object Storage upload failures
- **Retry**: Create a new export job

### Signed URL Expired

- **Solution**: Request a new export job (exports can be regenerated from rollup tables)
- **Note**: Regenerated exports may have slightly different totals if rollup data was updated

### Reconciliation Mismatch

- **Verify time range**: Ensure export time range matches query time range exactly
- **Check granularity**: Hourly exports aggregate differently than daily exports
- **Tolerance**: Small differences (<1%) are expected due to rounding and timing
- **Contact support**: If mismatch exceeds 1%, contact platform engineering

## API Reference

Full OpenAPI specification: `specs/007-analytics-service/contracts/analytics-exports-openapi.yaml`

## Support

For issues or questions:
- **Documentation**: See `docs/runbooks/analytics-incident-response.md` for operational procedures
- **API Issues**: Check `services/analytics-service/PHASE_STATUS.md` for current status
- **Finance Questions**: Contact finance team or platform engineering

