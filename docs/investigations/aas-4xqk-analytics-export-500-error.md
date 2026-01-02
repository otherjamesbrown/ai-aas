# Investigation Report

**Bead**: aas-4xqk
**Date**: 2025-12-30
**Investigator**: debugger agent

> **Note**: The E2E tests referenced in this investigation have been migrated to UC tests in `tests/usecases/`. See `TestUC_ANL_003_AnalyticsExport` for the current implementation.

## Symptom

E2E test `TestAnalyticsExportWorkflow` failing with HTTP 500 Internal Server Error when creating an analytics export job.

**Error Message**:
```
analytics_export_test.go:155: Expected status 202 Accepted, got 500: {"detail":"failed to create export job","status":500,"title":"Internal Server Error"}
```

**Expected Behavior**: POST to `/analytics/v1/orgs/{orgId}/exports` should return 202 Accepted with export job details

**Actual Behavior**: Returns 500 with generic error message "failed to create export job"

## Reproduction

The error occurs when:
1. E2E test creates an organization
2. Test makes inference requests to generate usage data
3. Test attempts to create an export job via POST to analytics service export endpoint
4. Handler validation passes (time range, granularity checks)
5. Database INSERT fails during `repo.CreateExportJob()` call

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `/home/dev/worktrees/develop/tests/e2e/suites/analytics_export_test.go:148-156` | Test POST to export endpoint, expects 202, receives 500 |
| `/home/dev/worktrees/develop/services/analytics-service/internal/api/exports_handler.go:102-111` | Handler catches error from `repo.CreateExportJob()` and returns 500 |
| `/home/dev/worktrees/develop/services/analytics-service/internal/exports/job_repository.go:50-72` | **ROOT CAUSE**: INSERT query line 51-55 uses string values without ENUM type casting |
| `/home/dev/worktrees/develop/db/migrations/analytics/20251127001_exports.sql:19-34` | Schema defines `granularity` and `status` columns as PostgreSQL ENUM types |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Missing required fields in request | ❌ Ruled out | Handler validates all required fields before INSERT (lines 46-79) |
| Database table doesn't exist | ❌ Ruled out | Migration 20251127001_exports.sql creates table with IF NOT EXISTS |
| Missing database permissions | ❌ Ruled out | Service successfully performs SELECT queries on same table (GetExportJob, ListExportJobs) |
| Type mismatch between Go strings and PostgreSQL ENUMs | ✅ CONFIRMED | Schema uses ENUM types, query passes raw strings without casting |

## Root Cause

**Category**: `missing_type_cast`

**Explanation**:

The `analytics.export_jobs` table schema defines two columns as PostgreSQL ENUM types:

```sql
-- From db/migrations/analytics/20251127001_exports.sql
granularity analytics.export_job_granularity NOT NULL DEFAULT 'daily',
status analytics.export_job_status NOT NULL DEFAULT 'pending',
```

Where the ENUM types are defined as:
```sql
CREATE TYPE analytics.export_job_status AS ENUM ('pending', 'running', 'succeeded', 'failed', 'expired');
CREATE TYPE analytics.export_job_granularity AS ENUM ('hourly', 'daily', 'monthly');
```

However, the INSERT query in `job_repository.go` passes string values without explicit type casting:

```sql
-- services/analytics-service/internal/exports/job_repository.go:51-55
INSERT INTO analytics.export_jobs (
    org_id, requested_by, time_range_start, time_range_end, granularity, status
) VALUES ($1, $2, $3, $4, $5, 'pending')
```

PostgreSQL requires explicit casting when inserting string literals or parameters into ENUM columns. The driver (pgx) passes the Go string `req.Granularity` as parameter `$5` with type `text`, not `analytics.export_job_granularity`.

**Evidence**:

The expected PostgreSQL error would be:
```
ERROR: column "granularity" is of type analytics.export_job_granularity but expression is of type text
HINT: You will need to rewrite or cast the expression.
```

or similarly for the `status` column.

**Why this wasn't caught earlier**:
- No unit tests exist for `ExportJobRepository.CreateExportJob()` that run against a real PostgreSQL database
- Integration tests may not have run or may have been skipped
- E2E test suite may not have been run before the analytics export feature was merged

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: `context/go-services-developer/agents.md` or similar testing context

**What was missing**:
1. **Testing pattern**: All repository methods that perform database INSERTs with ENUM types must include type casting
2. **Anti-pattern**: Passing Go strings directly to PostgreSQL ENUM columns without explicit casting
3. **Verification rule**: Repository tests should use real PostgreSQL (not mocks) to catch type mismatches

**Suggested fix to context**:

Add to Go services developer context:

```markdown
## PostgreSQL ENUM Type Handling

### ALWAYS:
- Cast string literals and parameters when inserting into ENUM columns
- Use `::enum_type` syntax: `'value'::analytics.export_job_status`
- Test repository methods against real PostgreSQL database, not mocks

### Example:
```sql
-- CORRECT:
INSERT INTO table (status) VALUES ($1::analytics.status_enum)

-- WRONG:
INSERT INTO table (status) VALUES ($1)  -- Type mismatch error
```

### Anti-pattern:
```go
// WRONG: No type casting
query := "INSERT INTO jobs (status) VALUES ('pending')"

// CORRECT: Explicit cast
query := "INSERT INTO jobs (status) VALUES ('pending'::analytics.job_status)"
```
```

## Proposed Fix

**High-level description**: Add explicit type casting to INSERT query in `ExportJobRepository.CreateExportJob()`

**Affected files**:
- `/home/dev/worktrees/develop/services/analytics-service/internal/exports/job_repository.go` - Update INSERT query with type casts

**Implementation**:

Change the INSERT query from:
```sql
INSERT INTO analytics.export_jobs (
    org_id, requested_by, time_range_start, time_range_end, granularity, status
) VALUES ($1, $2, $3, $4, $5, 'pending')
```

To:
```sql
INSERT INTO analytics.export_jobs (
    org_id, requested_by, time_range_start, time_range_end, granularity, status
) VALUES ($1, $2, $3, $4, $5::analytics.export_job_granularity, 'pending'::analytics.export_job_status)
```

**Estimated complexity**: Low (single-line SQL change)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add repository integration tests that run against real PostgreSQL database |
| Test | Ensure E2E tests run in CI before merge |
| Lint | Add sqlc or similar SQL type checking tool (optional, high effort) |
| Context | Add PostgreSQL ENUM type handling pattern to Go developer context |
| Documentation | Document ENUM type casting requirement in repository README |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| aas-tvvz | bug | go-services-developer | Fix ENUM type casting in CreateExportJob |
| aas-nme0 | task | go-services-developer | Add repository integration tests for export jobs |
| aas-1skp | task | context-maintainer | Update Go context with PostgreSQL ENUM handling pattern |
| aas-d4xr | task | ci-cd | Ensure E2E analytics tests run in CI pipeline |
