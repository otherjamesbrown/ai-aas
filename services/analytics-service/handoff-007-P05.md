# Handoff Document: Analytics Service - Phase 5 Ready

**Spec**: `007-analytics-service`  
**Phase**: Phase 5 - User Story 3 (Finance-friendly reporting)  
**Date**: 2025-01-27  
**Status**: 🚀 Ready to Start  
**Previous Phase**: Phase 4 Complete ✅

## Summary

Phase 5 implements finance-friendly CSV exports with S3 delivery, job lifecycle management, and reconciliation validation. This phase enables finance stakeholders to self-serve month-to-date cost exports with audit trails and retention controls.

## Prerequisites Status

### ✅ Phase 1: Setup - Complete
- Service scaffold, build tooling, docker-compose, CI/CD all in place
- `services/analytics-service/cmd/analytics-service/main.go`
- `services/analytics-service/dev/docker-compose.yml` (Postgres, Redis, RabbitMQ)

### ✅ Phase 2: Foundational - Complete
- Config loader with validation (`internal/config/config.go`)
- HTTP server bootstrap with chi router (`internal/api/server.go`)
- TimescaleDB migrations (`db/migrations/analytics/20251112001_init.up.sql`)
- Observability instrumentation (`internal/observability/telemetry.go`)
- Rollup tables migration (`db/migrations/analytics/20251116001_rollups.up.sql`)

### ✅ Phase 3: User Story 1 - Complete
- Deduplicated persistence pipeline (`internal/ingestion/processor.go`)
- Rollup worker (`internal/aggregation/rollup_worker.go`)
- Usage API handler (`internal/api/usage_handler.go`)
- Redis-backed freshness cache (`internal/freshness/cache.go`)
- Grafana dashboards (`dashboards/analytics/usage.json`)
- Integration tests (`tests/analytics/integration/usage_visibility_test.go`)

### ✅ Phase 4: User Story 2 - Complete
- Reliability repository (`internal/storage/postgres/reliability_repository.go`)
- Reliability API handler (`internal/api/reliability_handler.go`)
- Prometheus alerts (`dashboards/alerts/analytics-service.yaml`)
- Incident exporter (`internal/exports/incident_exporter.go`)
- Incident response runbook (`docs/runbooks/analytics-incident-response.md`)
- Reliability integration tests (`tests/analytics/integration/reliability_incident_test.go`)

## Phase 5 Tasks

### T-S007-P05-022: Export Job Repository and Migration ⏳ START HERE
- **Files**: 
  - `services/analytics-service/internal/exports/job_repository.go`
  - `db/migrations/analytics/20251127001_exports.up.sql`
  - `db/migrations/analytics/20251127001_exports.down.sql`
- **Status**: ⏳ To Do
- **Purpose**: Create database schema and repository for export job lifecycle management
- **Requirements**:
  - Create `analytics.export_jobs` table per data model spec
  - Columns: `job_id`, `org_id`, `requested_by`, `time_range_start`, `time_range_end`, `granularity`, `status`, `output_uri`, `checksum`, `row_count`, `initiated_at`, `completed_at`, `error_message`
  - Status enum: `pending`, `running`, `succeeded`, `failed`, `expired`
  - Granularity enum: `hourly`, `daily`, `monthly`
  - Indexes: `(org_id, initiated_at DESC)`, partial index on `(status)` WHERE `status IN ('pending', 'running')`
  - Repository methods: `CreateExportJob()`, `GetExportJob()`, `ListExportJobs()`, `UpdateExportJobStatus()`, `SetExportJobOutput()`
- **Dependencies**: None (foundational migration)

### T-S007-P05-023: Export Worker Pipeline
- **File**: `services/analytics-service/internal/exports/job_runner.go`
- **Status**: ⏳ To Do
- **Purpose**: Background worker that processes export jobs and generates CSVs
- **Requirements**:
  - Poll for pending export jobs
  - Query rollup tables based on granularity (hourly/daily/monthly)
  - Generate CSV with proper formatting
  - Calculate SHA-256 checksum
  - Upload to S3 via delivery adapter
  - Update job status and metadata
  - Handle errors gracefully (mark job as failed)
  - Support concurrent job processing (configurable workers)
- **Dependencies**: T-S007-P05-022 (job repository), T-S007-P05-025 (S3 delivery)
- **Integration Point**: Start worker in `cmd/analytics-service/main.go`

### T-S007-P05-024: Export Management API Handler
- **File**: `services/analytics-service/internal/api/exports_handler.go`
- **Status**: ⏳ To Do
- **Purpose**: HTTP handlers for export job management per OpenAPI contract
- **Requirements**:
  - `POST /analytics/v1/orgs/{orgId}/exports` - Create export job
    - Validate time range (max 31 days)
    - Validate granularity
    - Create job with status `pending`
    - Return 202 Accepted with job details
  - `GET /analytics/v1/orgs/{orgId}/exports` - List export jobs
    - Support status filter query parameter
    - Return paginated list
  - `GET /analytics/v1/orgs/{orgId}/exports/{jobId}` - Get job status
    - Return job details including status, output_uri (if ready)
  - `GET /analytics/v1/orgs/{orgId}/exports/{jobId}/download` - Get signed URL
    - Return 302 redirect to signed S3 URL
    - Only for jobs with status `succeeded`
- **Dependencies**: T-S007-P05-022 (job repository)
- **OpenAPI Reference**: `specs/007-analytics-service/contracts/analytics-exports-openapi.yaml`

### T-S007-P05-025: S3 Delivery Adapter
- **File**: `services/analytics-service/internal/exports/s3_delivery.go`
- **Status**: ⏳ To Do
- **Purpose**: S3-compatible object storage adapter for CSV delivery
- **Requirements**:
  - Upload CSV files to S3 with org-specific prefixes: `analytics/exports/{org_id}/{job_id}.csv`
  - Generate signed URLs with 24-hour expiration
  - Support S3-compatible storage (AWS S3, MinIO, etc.)
  - Handle upload errors gracefully
  - Calculate and return checksum
  - Support configurable bucket (per-org override or default)
- **Dependencies**: AWS SDK or MinIO client
- **Configuration**: S3 credentials and endpoint from config (`S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_REGION`)

### T-S007-P05-026: Reconciliation Integration Test
- **File**: `tests/analytics/integration/export_reconciliation_test.go`
- **Status**: ⏳ To Do
- **Purpose**: Validate that export CSV totals reconcile with rollup aggregates
- **Requirements**:
  - Seed test data with known totals
  - Create export job via API
  - Wait for job completion
  - Download CSV and parse
  - Compare CSV totals with rollup query totals
  - Verify reconciliation within 1% tolerance
  - Test all granularities (hourly, daily, monthly)
  - Test model filtering
- **Dependencies**: T-S007-P05-022, T-S007-P05-023, T-S007-P05-024, T-S007-P05-025
- **Test Pattern**: Similar to `usage_visibility_test.go` and `reliability_incident_test.go`

### T-S007-P05-027: Finance Documentation
- **File**: `docs/metrics/report.md`
- **Status**: ⏳ To Do
- **Purpose**: Document export process, retention policy, and finance workflows
- **Requirements**:
  - Export process overview
  - How to request exports via API
  - Export formats and schemas
  - Reconciliation procedures
  - Retention policy (24-hour signed URLs, 13-month data retention)
  - Monthly export workflows
  - Troubleshooting guide
- **Dependencies**: All export functionality complete

## Architecture Overview

### Export Job Lifecycle

```
1. Client Request
   POST /analytics/v1/orgs/{orgId}/exports
   ↓
2. Create Job (Status: pending)
   INSERT INTO analytics.export_jobs
   ↓
3. Export Worker Polls
   SELECT * FROM analytics.export_jobs WHERE status = 'pending'
   ↓
4. Process Job (Status: running)
   UPDATE status = 'running'
   ↓
5. Generate CSV
   Query rollup tables → Generate CSV → Calculate checksum
   ↓
6. Upload to S3
   Upload CSV to s3://bucket/analytics/exports/{org_id}/{job_id}.csv
   Generate signed URL (24h expiration)
   ↓
7. Complete Job (Status: succeeded)
   UPDATE status = 'succeeded', output_uri, checksum, row_count
   ↓
8. Client Retrieval
   GET /analytics/v1/orgs/{orgId}/exports/{jobId}
   GET /analytics/v1/orgs/{orgId}/exports/{jobId}/download → 302 redirect
```

### Key Components

1. **Export Job Repository** (`internal/exports/job_repository.go`)
   - Database operations for export jobs
   - Job lifecycle management
   - Status transitions

2. **Export Worker** (`internal/exports/job_runner.go`)
   - Background worker polling for pending jobs
   - CSV generation from rollup tables
   - S3 upload coordination
   - Error handling and retry logic

3. **S3 Delivery Adapter** (`internal/exports/s3_delivery.go`)
   - S3 client wrapper
   - Signed URL generation
   - Org-specific prefix handling
   - Upload error handling

4. **Exports API Handler** (`internal/api/exports_handler.go`)
   - HTTP handlers for export management
   - Request validation
   - Job status retrieval
   - Signed URL redirection

5. **Incident Exporter** (`internal/exports/incident_exporter.go`) ✅ Already exists
   - Can be reused/adapted for export worker CSV generation

## Implementation Strategy

### Step 1: Database Schema (T022)
```bash
# Create migration file
touch db/migrations/analytics/20251127001_exports.up.sql
touch db/migrations/analytics/20251127001_exports.down.sql

# Define export_jobs table schema
# - Follow data model spec exactly
# - Include all required columns and indexes
# - Test migration up/down
```

### Step 2: Repository Layer (T022)
```bash
# Create repository
touch services/analytics-service/internal/exports/job_repository.go

# Implement methods:
# - CreateExportJob(ctx, req) → (jobID, error)
# - GetExportJob(ctx, orgID, jobID) → (ExportJob, error)
# - ListExportJobs(ctx, orgID, status) → ([]ExportJob, error)
# - UpdateExportJobStatus(ctx, jobID, status) → error
# - SetExportJobOutput(ctx, jobID, uri, checksum, rowCount) → error
```

### Step 3: S3 Delivery Adapter (T025) - Can run in parallel
```bash
# Create S3 adapter
touch services/analytics-service/internal/exports/s3_delivery.go

# Implement:
# - UploadCSV(ctx, orgID, jobID, csvData) → (signedURL, checksum, error)
# - GenerateSignedURL(ctx, key) → (url, error)
# - Use AWS SDK or MinIO client
```

### Step 4: Export Worker (T023)
```bash
# Create worker
touch services/analytics-service/internal/exports/job_runner.go

# Implement:
# - Worker struct with config (store, s3Delivery, logger)
# - Start() method with polling loop
# - processJob() method:
#   - Query rollup tables based on granularity
#   - Generate CSV (reuse incident_exporter logic)
#   - Upload to S3
#   - Update job status
# - Handle errors and retries
```

### Step 5: API Handler (T024)
```bash
# Create handler
touch services/analytics-service/internal/api/exports_handler.go

# Implement handlers:
# - CreateExportJob(w, r)
# - ListExportJobs(w, r)
# - GetExportJob(w, r)
# - GetExportDownloadUrl(w, r) → 302 redirect
# - Register routes in server.go
```

### Step 6: Integration Test (T026)
```bash
# Create test
touch tests/analytics/integration/export_reconciliation_test.go

# Test cases:
# - TestExportJobCreation
# - TestExportJobCompletion
# - TestCSVReconciliation
# - TestSignedURLGeneration
# - TestJobStatusFiltering
```

### Step 7: Documentation (T027)
```bash
# Update finance docs
touch docs/metrics/report.md

# Document:
# - Export API usage
# - Reconciliation procedures
# - Retention policies
# - Monthly workflows
```

## Dependencies

### External Services
- **S3-compatible Storage**: For CSV file storage (AWS S3, MinIO, etc.)
  - Already configured in `config.go` with `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`, `S3_REGION`
  - Can use MinIO for local development (add to docker-compose if needed)

### Internal Dependencies
- **PostgreSQL/TimescaleDB**: For export_jobs table and rollup queries
- **Rollup Tables**: `analytics_hourly_rollups`, `analytics_daily_rollups` (already exist)
- **Incident Exporter**: Can reuse CSV generation logic from `internal/exports/incident_exporter.go`

### Go Dependencies (to add)
```go
// For S3 operations
github.com/aws/aws-sdk-go-v2/service/s3
github.com/aws/aws-sdk-go-v2/config
// OR
github.com/minio/minio-go/v7

// For checksum calculation
crypto/sha256
encoding/hex
```

## Configuration

### Environment Variables (already in config.go)
```bash
# S3 Configuration (already exists)
S3_ENDPOINT=http://localhost:9000  # MinIO for local dev
S3_ACCESS_KEY=minioadmin
S3_SECRET_KEY=minioadmin
S3_BUCKET=analytics-exports
S3_REGION=us-east-1

# Export Worker Configuration (to add)
EXPORT_WORKER_INTERVAL=30s         # Poll interval
EXPORT_WORKER_CONCURRENCY=2        # Concurrent jobs
EXPORT_SIGNED_URL_TTL=24h          # Signed URL expiration
```

### Config Structure (to add to `config.go`)
```go
type Config struct {
    // ... existing fields ...
    
    // Export Worker
    ExportWorkerInterval  time.Duration `envconfig:"EXPORT_WORKER_INTERVAL" default:"30s"`
    ExportWorkerConcurrency int `envconfig:"EXPORT_WORKER_CONCURRENCY" default:"2"`
    ExportSignedURLTTL   time.Duration `envconfig:"EXPORT_SIGNED_URL_TTL" default:"24h"`
}
```

## Testing Strategy

### Unit Tests
- Export job repository (CRUD operations)
- S3 delivery adapter (upload, signed URL generation)
- CSV generation logic
- Job status transitions

### Integration Tests
- End-to-end export job creation → completion → download
- CSV reconciliation with rollup totals
- Signed URL generation and expiration
- Error handling (S3 failures, job failures)
- Concurrent job processing

### Manual Testing
```bash
# Start dependencies (add MinIO if needed)
make dev-up

# Run migrations
make migrate-up

# Start service
make run

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

# Download CSV (when ready)
curl -L http://localhost:8084/analytics/v1/orgs/{orgId}/exports/{jobId}/download
```

## Key Files and References

### Existing Files (Reference)
- `services/analytics-service/internal/exports/incident_exporter.go` - CSV generation logic to reuse
- `services/analytics-service/internal/storage/postgres/store.go` - Database connection pattern
- `services/analytics-service/internal/api/usage_handler.go` - API handler pattern
- `services/analytics-service/internal/aggregation/rollup_worker.go` - Worker pattern
- `tests/analytics/integration/usage_visibility_test.go` - Test setup pattern

### OpenAPI Contract
- `specs/007-analytics-service/contracts/analytics-exports-openapi.yaml` - Complete API specification

### Data Model
- `specs/007-analytics-service/data-model.md` - Export jobs table specification

### Tasks Reference
- `specs/007-analytics-service/tasks.md` - Phase 5 task breakdown

## Implementation Notes

### CSV Generation
- Reuse logic from `incident_exporter.go` but adapt for rollup tables
- For `hourly` granularity: Query `analytics_hourly_rollups`
- For `daily` granularity: Query `analytics_daily_rollups`
- For `monthly` granularity: Aggregate from daily rollups
- Include columns: `bucket_start`, `organization_id`, `model_id`, `request_count`, `tokens_total`, `error_count`, `cost_total`

### S3 Path Structure
- Format: `analytics/exports/{org_id}/{job_id}.csv`
- Supports org-specific prefixes for access control
- Signed URLs expire after 24 hours (configurable)

### Job Status Transitions
- `pending` → `running` (when worker picks up job)
- `running` → `succeeded` (on successful completion)
- `running` → `failed` (on error)
- `succeeded` → `expired` (after signed URL expiration, via cleanup job)

### Reconciliation Validation
- Compare CSV totals with rollup query totals
- Tolerance: within 1% difference
- Validate: invocations, tokens, cost estimates
- Test across all granularities

### Error Handling
- S3 upload failures → mark job as `failed`, store error message
- CSV generation failures → mark job as `failed`
- Invalid time ranges → return 400 Bad Request
- Job not found → return 404 Not Found

## Parallel Opportunities

Tasks marked `[P]` can run in parallel:
- **T022** (Repository) and **T025** (S3 Adapter) can be implemented concurrently
- **T026** (Integration Test) can start once T022-T025 interfaces exist
- **T027** (Documentation) can be written alongside implementation

## Success Criteria

Phase 5 is complete when:
1. ✅ Export jobs can be created via API
2. ✅ Export worker processes jobs and generates CSVs
3. ✅ CSVs are uploaded to S3 with signed URLs
4. ✅ CSV totals reconcile with rollup queries (within 1%)
5. ✅ Integration tests pass
6. ✅ Finance documentation is complete

## Next Steps After Phase 5

- **Phase 6**: Polish & Cross-Cutting Concerns
  - RBAC middleware
  - Performance benchmarks
  - Documentation updates
  - Knowledge artifact updates

## Questions or Issues?

- Check `services/analytics-service/PHASE_STATUS.md` for current status
- Review `specs/007-analytics-service/` for detailed specifications
- See `docs/runbooks/analytics-incident-response.md` for operational procedures

