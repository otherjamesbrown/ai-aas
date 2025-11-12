# Phase 5 Testing Guide

This guide helps you test the Phase 5 export functionality.

## Prerequisites

1. **Dependencies Running**:
   ```bash
   cd services/analytics-service
   make dev-up
   ```

2. **Migrations Applied**:
   ```bash
   # From project root
   make migrate-up SERVICE=analytics-service
   ```

3. **AWS SDK Dependencies** (if not already added):
   ```bash
   cd services/analytics-service
   go get github.com/aws/aws-sdk-go-v2/service/s3
   go get github.com/aws/aws-sdk-go-v2/config
   go get github.com/aws/aws-sdk-go-v2/credentials
   ```

4. **Linode Object Storage Configuration** (optional for basic testing):
   ```bash
   export S3_ENDPOINT=https://us-east-1.linodeobjects.com
   export S3_ACCESS_KEY=<your-access-key>
   export S3_SECRET_KEY=<your-secret-key>
   export S3_BUCKET=analytics-exports
   export S3_REGION=us-east-1
   ```

   **Note**: If S3 credentials are not provided, the export worker won't start, but API endpoints will still work (jobs will fail when processed).

## Test Options

### Option 1: Integration Tests (Recommended)

Run the automated integration tests:

```bash
cd tests/analytics/integration
go test -v -run TestExportJobCreation
go test -v -run TestCSVReconciliation
go test -v -run TestExportGranularities
```

These tests use testcontainers to spin up Postgres and Redis, seed test data, and validate the export pipeline.

### Option 2: Manual API Testing

1. **Start the service**:
   ```bash
   cd services/analytics-service
   export DATABASE_URL="postgres://postgres:postgres@localhost:5432/analytics_test?sslmode=disable"
   export REDIS_URL="redis://localhost:6379"
   make run
   ```

2. **Run the test script**:
   ```bash
   ./test-phase5.sh
   ```

   Or test manually with curl:

   ```bash
   # Create export job
   curl -X POST http://localhost:8084/analytics/v1/orgs/{orgId}/exports \
     -H "Content-Type: application/json" \
     -d '{
       "timeRange": {
         "start": "2024-01-01T00:00:00Z",
         "end": "2024-01-31T23:59:59Z"
       },
       "granularity": "daily"
     }'

   # Check job status
   curl http://localhost:8084/analytics/v1/orgs/{orgId}/exports/{jobId}

   # Get download URL (when succeeded)
   curl -L http://localhost:8084/analytics/v1/orgs/{orgId}/exports/{jobId}/download
   ```

### Option 3: Unit Tests

Run unit tests for individual components:

```bash
cd services/analytics-service
go test ./internal/exports/... -v
go test ./internal/api/... -v
```

## Test Scenarios

### Scenario 1: Basic Export Job Creation

**Goal**: Verify export jobs can be created via API

**Steps**:
1. Create export job with daily granularity
2. Verify job is created with status "pending"
3. Verify job appears in list endpoint

**Expected**: Job created successfully, status is "pending"

### Scenario 2: Export Job Processing

**Goal**: Verify export worker processes jobs and generates CSVs

**Steps**:
1. Ensure rollup data exists (seed test data or run rollup worker)
2. Create export job
3. Wait for job to complete (check status every 2 seconds)
4. Verify job status changes to "succeeded"
5. Verify output_uri, checksum, and row_count are populated

**Expected**: Job completes successfully, CSV uploaded to Linode Object Storage

**Note**: Requires S3 credentials and rollup data to be present.

### Scenario 3: CSV Reconciliation

**Goal**: Verify CSV totals reconcile with rollup queries

**Steps**:
1. Seed rollup data with known totals
2. Create export job
3. Wait for completion
4. Download CSV
5. Sum CSV columns
6. Query rollup tables directly
7. Compare totals (should be within 1%)

**Expected**: CSV totals match rollup query totals within 1% tolerance

### Scenario 4: Granularity Testing

**Goal**: Verify all granularities work correctly

**Steps**:
1. Test hourly granularity export
2. Test daily granularity export
3. Test monthly granularity export
4. Verify CSV format is correct for each

**Expected**: All granularities generate valid CSVs

### Scenario 5: Error Handling

**Goal**: Verify error handling works correctly

**Steps**:
1. Create export job with invalid time range (>31 days)
2. Verify 400 Bad Request response
3. Create export job with invalid granularity
4. Verify 400 Bad Request response
5. Request non-existent job
6. Verify 404 Not Found response

**Expected**: Appropriate error responses for invalid inputs

## Troubleshooting

### Export Job Stuck in "pending"

**Possible causes**:
- Export worker not running (check logs for "export worker not started")
- S3 credentials not configured
- No rollup data available

**Solutions**:
- Check service logs: `docker compose -f dev/docker-compose.yml logs analytics-service`
- Verify S3 credentials are set
- Seed rollup data or run rollup worker

### Export Job Failed

**Check error message**:
```bash
curl http://localhost:8084/analytics/v1/orgs/{orgId}/exports/{jobId}
```

**Common errors**:
- `S3 upload failed`: Check Linode Object Storage credentials
- `Query rollup data failed`: Check database connection and rollup data exists
- `Invalid time range`: Ensure time range is valid and within 31 days

### CSV Download Fails

**Possible causes**:
- Signed URL expired (24-hour expiration)
- Job not in "succeeded" status
- Linode Object Storage access issues

**Solutions**:
- Request new export job to regenerate signed URL
- Verify job status is "succeeded"
- Check Linode Object Storage bucket permissions

## Validation Checklist

- [ ] Export job creation works via API
- [ ] Export worker processes jobs (if S3 configured)
- [ ] CSV files are generated correctly
- [ ] CSV totals reconcile with rollup queries (within 1%)
- [ ] Signed URLs are generated and accessible
- [ ] All granularities (hourly, daily, monthly) work
- [ ] Error handling works for invalid inputs
- [ ] Job listing and filtering work correctly
- [ ] Integration tests pass

## Next Steps

After successful testing:
1. Update `PHASE_STATUS.md` to mark Phase 5 as complete
2. Proceed to Phase 6: Polish & Cross-Cutting Concerns
3. Consider performance testing with larger datasets

