# Handoff Document: Analytics Service - Phase 6 Ready

**Spec**: `007-analytics-service`  
**Phase**: Phase 6 - Polish & Cross-Cutting Concerns  
**Date**: 2025-01-27  
**Status**: 🚀 Ready to Start  
**Previous Phase**: Phase 5 Complete ✅

## Summary

Phase 6 focuses on hardening security, performance, and documentation across all user stories. This phase implements RBAC middleware, audit logging, performance benchmarks, and finalizes documentation to ensure the analytics service is production-ready.

## Prerequisites Status

### ✅ Phase 1: Setup - Complete
- Service scaffold, build tooling, docker-compose, CI/CD all in place

### ✅ Phase 2: Foundational - Complete
- Config loader with validation
- HTTP server bootstrap with chi router
- TimescaleDB migrations
- Observability instrumentation

### ✅ Phase 3: User Story 1 - Complete
- Usage visibility API
- Rollup workers
- Freshness cache
- Integration tests

### ✅ Phase 4: User Story 2 - Complete
- Reliability API
- Incident exporter
- Prometheus alerts
- Runbooks

### ✅ Phase 5: User Story 3 - Complete
- Export job lifecycle management
- CSV generation and S3 delivery
- Reconciliation validation
- Finance documentation

## Phase 6 Tasks

### T-S007-P06-028: RBAC Middleware & Audit Logging ⏳ START HERE
- **Files**: 
  - `services/analytics-service/internal/middleware/rbac.go`
  - `services/analytics-service/internal/audit/logger.go`
- **Status**: ⏳ To Do
- **Purpose**: Secure all analytics endpoints with role-based access control and audit logging
- **Requirements**:
  - Integrate `shared/go/auth` middleware for RBAC enforcement
  - Define role-based policies for analytics endpoints:
    - `analytics:usage:read` - View usage data
    - `analytics:reliability:read` - View reliability metrics
    - `analytics:exports:create` - Create export jobs
    - `analytics:exports:read` - View export jobs
    - `analytics:exports:download` - Download export files
  - Implement audit logging for all API operations
  - Log actor, action, resource, and outcome (allowed/denied)
  - Support both header-based and token-based actor extraction
  - Apply middleware to all analytics API routes
- **Dependencies**: `shared/go/auth` package
- **Integration Point**: Add middleware to `internal/api/server.go`

### T-S007-P06-029: Performance Benchmarks
- **File**: `tests/analytics/perf/freshness_benchmark_test.go`
- **Status**: ⏳ To Do
- **Purpose**: Establish performance baselines and document thresholds
- **Requirements**:
  - Benchmark rollup query performance (hourly, daily, monthly)
  - Benchmark CSV generation performance for various data volumes
  - Benchmark freshness cache operations
  - Document performance thresholds:
    - Rollup queries: < 100ms for 7-day range
    - CSV generation: < 5s for 1M rows
    - Freshness cache: < 1ms for lookups
  - Capture results in benchmark test file
- **Dependencies**: None (can run independently)

### T-S007-P06-030: Documentation Finalization
- **Files**: 
  - `specs/007-analytics-service/quickstart.md`
  - `docs/runbooks/analytics-incident-response.md`
- **Status**: ⏳ To Do
- **Purpose**: Ensure documentation reflects actual implementation
- **Requirements**:
  - Update quickstart guide with:
    - Correct API endpoints and examples
    - Actual configuration options
    - Step-by-step setup instructions
    - Example requests/responses
  - Update runbook with:
    - Actual alert names and thresholds
    - Real troubleshooting steps
    - Correct command examples
    - Actual dashboard locations
- **Dependencies**: None (can be done in parallel)

### T-S007-P06-031: Knowledge Artifact Updates
- **Files**: 
  - `llms.txt`
  - `docs/specs-progress.md`
- **Status**: ⏳ To Do
- **Purpose**: Update knowledge artifacts with analytics service information
- **Requirements**:
  - Add analytics service links to `llms.txt`
  - Update `docs/specs-progress.md` with Phase 6 completion status
  - Include links to:
    - Service documentation
    - API contracts
    - Runbooks
    - Dashboards
- **Dependencies**: None (can be done in parallel)

## Architecture Overview

### RBAC Middleware Integration

```
HTTP Request
    ↓
RequestID Middleware
    ↓
RealIP Middleware
    ↓
RBAC Middleware (NEW)
    ├─ Extract Actor (from headers/token)
    ├─ Check Authorization (via Policy Engine)
    ├─ Log Audit Event
    └─ Allow/Deny Request
    ↓
Logger Middleware
    ↓
Recoverer Middleware
    ↓
API Handler
```

### Audit Logging Flow

```
API Request
    ↓
RBAC Middleware
    ├─ Extract Actor Info
    ├─ Determine Action (method:path)
    ├─ Check Authorization
    └─ Create Audit Event
    ↓
Audit Logger
    ├─ Format Event (JSON)
    ├─ Include: actor, action, resource, outcome, timestamp
    └─ Write to Log Stream
```

## Role-Based Access Policies

### Policy Definitions

```go
// analytics:usage:read - View usage and cost data
"GET:/analytics/v1/orgs/{orgId}/usage" -> ["analytics:usage:read", "admin"]

// analytics:reliability:read - View reliability metrics
"GET:/analytics/v1/orgs/{orgId}/reliability" -> ["analytics:reliability:read", "admin"]

// analytics:exports:create - Create export jobs
"POST:/analytics/v1/orgs/{orgId}/exports" -> ["analytics:exports:create", "admin"]

// analytics:exports:read - View export jobs
"GET:/analytics/v1/orgs/{orgId}/exports" -> ["analytics:exports:read", "admin"]
"GET:/analytics/v1/orgs/{orgId}/exports/{jobId}" -> ["analytics:exports:read", "admin"]

// analytics:exports:download - Download export files
"GET:/analytics/v1/orgs/{orgId}/exports/{jobId}/download" -> ["analytics:exports:download", "admin"]
```

## Implementation Steps

### Step 1: Create RBAC Middleware Wrapper
1. Create `internal/middleware/rbac.go`
2. Wrap `shared/go/auth.Middleware` with analytics-specific configuration
3. Define policy engine with analytics role mappings
4. Configure actor extractor (support both header and token-based)

### Step 2: Create Audit Logger
1. Create `internal/audit/logger.go`
2. Implement structured audit logging
3. Log format: JSON with actor, action, resource, outcome, timestamp
4. Integrate with observability logger

### Step 3: Integrate Middleware
1. Update `internal/api/server.go`
2. Add RBAC middleware to middleware stack
3. Apply to all analytics API routes
4. Exclude health/readiness endpoints from RBAC

### Step 4: Performance Benchmarks
1. Create `tests/analytics/perf/freshness_benchmark_test.go`
2. Implement benchmark tests for:
   - Rollup queries
   - CSV generation
   - Freshness cache operations
3. Document thresholds and results

### Step 5: Documentation Updates
1. Review and update `specs/007-analytics-service/quickstart.md`
2. Review and update `docs/runbooks/analytics-incident-response.md`
3. Ensure all examples and commands are accurate

### Step 6: Knowledge Artifacts
1. Update `llms.txt` with analytics service links
2. Update `docs/specs-progress.md` with Phase 6 status

## Configuration

### RBAC Configuration

```go
// Policy engine configuration
policies := map[string][]string{
    "GET:/analytics/v1/orgs/{orgId}/usage": {"analytics:usage:read", "admin"},
    "GET:/analytics/v1/orgs/{orgId}/reliability": {"analytics:reliability:read", "admin"},
    "POST:/analytics/v1/orgs/{orgId}/exports": {"analytics:exports:create", "admin"},
    "GET:/analytics/v1/orgs/{orgId}/exports": {"analytics:exports:read", "admin"},
    "GET:/analytics/v1/orgs/{orgId}/exports/{jobId}": {"analytics:exports:read", "admin"},
    "GET:/analytics/v1/orgs/{orgId}/exports/{jobId}/download": {"analytics:exports:download", "admin"},
}
```

### Audit Logging Configuration

- Log level: INFO for allowed actions, WARN for denied actions
- Log format: Structured JSON
- Fields: actor.subject, actor.roles, action, resource, outcome, timestamp, request_id

## Testing Strategy

### RBAC Testing
- Unit tests for policy engine
- Integration tests for middleware
- Test cases:
  - Valid roles → allow
  - Invalid roles → deny (403)
  - Missing actor → deny (403)
  - Admin role → allow all

### Audit Logging Testing
- Verify audit events are logged
- Verify correct fields are captured
- Verify log format is correct

### Performance Benchmark Testing
- Run benchmarks with various data volumes
- Document results
- Compare against thresholds

## Success Criteria

Phase 6 is complete when:
1. ✅ All analytics endpoints are protected by RBAC middleware
2. ✅ Audit logging captures all API operations
3. ✅ Performance benchmarks are documented with thresholds
4. ✅ Quickstart guide is accurate and complete
5. ✅ Runbook reflects actual implementation
6. ✅ Knowledge artifacts are updated

## Parallel Opportunities

Tasks marked `[P]` can run in parallel:
- **T028** (RBAC/Audit) and **T030** (Documentation) can be implemented concurrently
- **T031** (Knowledge Artifacts) can be updated alongside other tasks

## Next Steps After Phase 6

- **Production Deployment**: Service is ready for production deployment
- **Monitoring**: Set up production monitoring and alerting
- **Scaling**: Plan for horizontal scaling if needed

## Questions or Issues?

- Check `services/analytics-service/PHASE_STATUS.md` for current status
- Review `specs/007-analytics-service/` for detailed specifications
- See `docs/runbooks/analytics-incident-response.md` for operational procedures

