# Handoff Document: API Router Service - Phase 8 Ready

**Spec**: `006-api-router-service`  
**Phase**: Phase 8 - Polish & Cross-Cutting Concerns  
**Date**: 2025-01-27  
**Status**: 🚀 Ready to Start  
**Previous Phase**: Phase 7 Complete ✅

## Summary

Phase 8 focuses on service-wide hardening, documentation, and release readiness. This phase ensures the API Router Service is production-ready with proper error handling, deployment configurations, testing coverage, and documentation validation.

## Prerequisites Status

### ✅ Phase 1: Setup - Complete
- Service scaffold, build tooling, docker-compose, CI/CD all in place

### ✅ Phase 2: Foundational - Complete
- Config loader with etcd integration
- BoltDB cache for configuration
- Telemetry (OpenTelemetry + zap logger)
- Middleware stack
- Contract generation workflow

### ✅ Phase 3: User Story 1 - Complete
- Authenticated inference routing working
- Request validation and DTOs
- Backend client with forwarding
- Handler pipeline with error handling
- All tests passing (contract + integration)

### ✅ Phase 4: User Story 2 - Complete
- Budget and rate limiting enforcement
- Redis token-bucket limiter
- Budget service client integration
- Audit event emission
- Structured limit error responses

### ✅ Phase 5: User Story 3 - Complete
- Intelligent routing with weighted selection
- Backend health monitoring
- Routing engine with failover
- Admin routing override endpoints
- Routing decision metrics

### ✅ Phase 6: User Story 4 - Complete
- Usage record builder
- Kafka publisher for usage records
- Disk-based buffering
- Usage emission hooks
- Audit lookup endpoint

### ✅ Phase 7: User Story 5 - Complete
- Health and readiness endpoints with component checks
- Comprehensive Prometheus metrics exporters
- Grafana operational dashboard
- Incident response runbooks

## Phase 8 Tasks

### T042: Harden Error Catalog and Response Codes ✅ START HERE
- **File**: `internal/api/errors.go`
- **Status**: ⏳ To Do
- **Purpose**: Create centralized error catalog with consistent response codes
- **Requirements**:
  - Define error codes matching OpenAPI spec
  - Create error response builder
  - Map internal errors to HTTP status codes
  - Ensure consistent error format across all endpoints
  - Include error codes for:
    - Authentication failures (401)
    - Authorization failures (403)
    - Budget/quota exceeded (402)
    - Rate limit exceeded (429)
    - Validation errors (400)
    - Backend errors (502, 503, 504)
    - Routing errors (500, 503)
    - Not found errors (404)
- **Integration Points**:
  - Update `internal/api/public/handler.go` to use error catalog
  - Update `internal/api/public/middleware.go` to use error catalog
  - Update `internal/api/admin/routing_handlers.go` to use error catalog

### T043: Finalize Helm Values and Alert Rules
- **File**: `deployments/helm/api-router-service/values.yaml`
- **Status**: ⏳ To Do
- **Purpose**: Create production-ready Helm chart configuration
- **Requirements**:
  - Default values for all configuration options
  - Environment-specific overrides (development, staging, production)
  - Resource limits and requests
  - Replica counts and autoscaling configuration
  - Service account and RBAC configuration
  - Ingress configuration
  - Prometheus ServiceMonitor configuration
  - Alert rules for:
    - High error rate (> 5% for 5 minutes)
    - High latency (P95 > 3s for 10 minutes)
    - Backend unhealthy (> 5 minutes)
    - High failover rate (> 10/min for 5 minutes)
    - Buffer store size high (> 1000 records)
    - Buffer store age high (> 24 hours)
    - Rate limit denials high (> 100/min for 5 minutes)
- **Files to Create**:
  - `deployments/helm/api-router-service/values.yaml` - Default values
  - `deployments/helm/api-router-service/values-staging.yaml` - Staging overrides
  - `deployments/helm/api-router-service/values-production.yaml` - Production overrides
  - `deployments/helm/api-router-service/templates/alerts.yaml` - Prometheus alert rules
  - `deployments/helm/api-router-service/Chart.yaml` - Chart metadata

### T044: Tune Load Test Scenarios for SLO Coverage
- **File**: `scripts/loadtest.sh`
- **Status**: ⏳ To Do
- **Purpose**: Create load test scenarios that validate SLO requirements
- **Requirements**:
  - Test scenarios covering:
    - Baseline load (normal traffic patterns)
    - Peak load (sustained high RPS)
    - Burst traffic (sudden spike)
    - Backend failure scenarios
    - Rate limit enforcement
    - Budget enforcement
  - Validate SLOs:
    - Latency P95 ≤ 3s (NFR-001)
    - Latency P99 ≤ 5s
    - Error rate < 1%
    - Router overhead ≤ 150ms median (NFR-001)
    - Router overhead ≤ 400ms p95 (NFR-001)
    - Rate limit decision ≤ 5ms (NFR-002)
  - Use `vegeta` or similar load testing tool
  - Generate reports with metrics
- **Dependencies**: `vegeta` CLI tool

### T045: Extend Smoke Test Coverage
- **File**: `scripts/smoke.sh`
- **Status**: ⏳ To Do
- **Purpose**: Create comprehensive smoke tests for deployment validation
- **Requirements**:
  - Test all critical endpoints:
    - Health endpoints (`/v1/status/healthz`, `/v1/status/readyz`)
    - Inference endpoint (`/v1/inference`)
    - Admin endpoints (`/v1/admin/routing/*`)
    - Audit endpoint (`/v1/audit/requests/{requestId}`)
    - Metrics endpoint (`/metrics`)
  - Test rate limiting:
    - Verify rate limit enforcement
    - Verify 429 responses
  - Test routing:
    - Verify backend selection
    - Verify failover behavior
  - Test budget enforcement:
    - Verify budget checks (if budget service available)
    - Verify 402 responses
  - Test usage tracking:
    - Verify usage records published (if Kafka available)
    - Verify buffer store (if Kafka unavailable)
  - Exit with non-zero code on failure
  - Provide clear output for CI/CD integration

### T046: Validate Quickstart Instructions
- **File**: `docs/quickstart-validation.md`
- **Status**: ⏳ To Do
- **Purpose**: Validate and document quickstart process
- **Requirements**:
  - Follow quickstart instructions step-by-step
  - Document any issues or gaps
  - Verify all commands work as documented
  - Test on clean environment
  - Capture:
    - Prerequisites verification
    - Setup steps
    - Configuration steps
    - Running the service
    - Testing the service
    - Common issues and solutions
  - Update quickstart documentation if needed
- **Integration Point**: May need to create or update `README.md` quickstart section

## Architecture Overview

### Error Catalog Structure

```
internal/api/errors.go
  ├─ ErrorCode (enum)
  ├─ ErrorResponse (struct)
  ├─ ErrorBuilder (functions)
  └─ Error mapping functions
```

### Helm Chart Structure

```
deployments/helm/api-router-service/
  ├─ Chart.yaml
  ├─ values.yaml
  ├─ values-staging.yaml
  ├─ values-production.yaml
  ├─ templates/
  │   ├─ deployment.yaml
  │   ├─ service.yaml
  │   ├─ ingress.yaml
  │   ├─ servicemonitor.yaml
  │   └─ alerts.yaml
  └─ dashboards/
      └─ api-router.json (already created in Phase 7)
```

## Implementation Strategy

### Step 1: Error Catalog (T042)
- Create `internal/api/errors.go` with error definitions
- Define error codes matching OpenAPI spec
- Create error response builder
- Update all handlers to use error catalog
- Verify consistent error format

### Step 2: Helm Chart (T043)
- Create Helm chart structure
- Define default values
- Create environment-specific overrides
- Add Prometheus alert rules
- Test chart installation

### Step 3: Load Testing (T044)
- Create load test script
- Define test scenarios
- Run tests and validate SLOs
- Document results

### Step 4: Smoke Tests (T045)
- Create smoke test script
- Test all critical endpoints
- Test rate limiting and routing
- Integrate with CI/CD

### Step 5: Quickstart Validation (T046)
- Follow quickstart instructions
- Document issues and solutions
- Update documentation as needed

## Dependencies

### External Tools
- **vegeta**: For load testing (install: `go install github.com/tsenart/vegeta/v2@latest`)
- **Helm**: For Kubernetes deployments (v3.x)
- **kubectl**: For Kubernetes operations

### Internal Dependencies
- All previous phases (1-7) must be complete
- OpenAPI spec for error code definitions
- Prometheus for alert rules
- Grafana for dashboard (already created)

## Configuration

### Error Codes (to define in errors.go)

```go
const (
    // Authentication errors
    ErrCodeUnauthorized = "UNAUTHORIZED"
    ErrCodeInvalidAPIKey = "INVALID_API_KEY"
    
    // Authorization errors
    ErrCodeForbidden = "FORBIDDEN"
    
    // Validation errors
    ErrCodeInvalidRequest = "INVALID_REQUEST"
    ErrCodeMissingField = "MISSING_FIELD"
    
    // Rate limiting
    ErrCodeRateLimitExceeded = "RATE_LIMIT_EXCEEDED"
    
    // Budget/quota
    ErrCodeBudgetExceeded = "BUDGET_EXCEEDED"
    ErrCodeQuotaExceeded = "QUOTA_EXCEEDED"
    
    // Backend errors
    ErrCodeBackendUnavailable = "BACKEND_UNAVAILABLE"
    ErrCodeBackendTimeout = "BACKEND_TIMEOUT"
    ErrCodeBackendError = "BACKEND_ERROR"
    
    // Routing errors
    ErrCodeNoBackendAvailable = "NO_BACKEND_AVAILABLE"
    ErrCodeRoutingError = "ROUTING_ERROR"
    
    // Not found
    ErrCodeNotFound = "NOT_FOUND"
    ErrCodeRequestNotFound = "REQUEST_NOT_FOUND"
    
    // Internal errors
    ErrCodeInternalError = "INTERNAL_ERROR"
    ErrCodeServiceUnavailable = "SERVICE_UNAVAILABLE"
)
```

### Helm Values Structure (to create)

```yaml
# values.yaml
replicaCount: 2
image:
  repository: api-router-service
  tag: latest
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 8080

ingress:
  enabled: false
  className: nginx
  annotations: {}
  hosts: []

resources:
  limits:
    cpu: 500m
    memory: 512Mi
  requests:
    cpu: 100m
    memory: 128Mi

autoscaling:
  enabled: true
  minReplicas: 2
  maxReplicas: 10
  targetCPUUtilizationPercentage: 70

config:
  redis:
    addr: redis-service:6379
  kafka:
    brokers: kafka-service:9092
  configService:
    endpoint: etcd-service:2379
```

## Testing Strategy

### Load Testing
- Use `vegeta` for HTTP load testing
- Test scenarios:
  - Baseline: 100 RPS sustained
  - Peak: 1000 RPS sustained
  - Burst: 2000 RPS for 30 seconds
- Validate SLOs:
  - Latency percentiles
  - Error rates
  - Router overhead

### Smoke Testing
- Run after deployment
- Test all critical paths
- Verify health endpoints
- Test inference routing
- Verify metrics export

## Key Files Reference

### Existing (Phases 1-7)
- `internal/api/public/handler.go` - Main handler (needs error catalog integration)
- `internal/api/public/middleware.go` - Middleware (needs error catalog integration)
- `internal/api/admin/routing_handlers.go` - Admin handlers (needs error catalog integration)
- `specs/006-api-router-service/contracts/api-router.openapi.yaml` - OpenAPI spec (error code reference)

### To Create (Phase 8)
- `internal/api/errors.go` - Error catalog
- `deployments/helm/api-router-service/Chart.yaml` - Helm chart metadata
- `deployments/helm/api-router-service/values.yaml` - Default Helm values
- `deployments/helm/api-router-service/values-staging.yaml` - Staging overrides
- `deployments/helm/api-router-service/values-production.yaml` - Production overrides
- `deployments/helm/api-router-service/templates/alerts.yaml` - Prometheus alert rules
- `scripts/loadtest.sh` - Load test script
- `scripts/smoke.sh` - Smoke test script
- `docs/quickstart-validation.md` - Quickstart validation notes

## Success Criteria

Phase 8 is complete when:
- ✅ T042: Error catalog created and integrated into all handlers
- ✅ T043: Helm chart created with values and alert rules
- ✅ T044: Load test scenarios validate SLO requirements
- ✅ T045: Smoke tests cover all critical endpoints
- ✅ T046: Quickstart instructions validated and documented
- ✅ All error responses are consistent
- ✅ Helm chart can be deployed to Kubernetes
- ✅ Load tests pass SLO validation
- ✅ Smoke tests pass in CI/CD

## Next Steps After Phase 8

Phase 8 is the final phase! After completion:
- Service is production-ready
- Ready for staging deployment
- Ready for production deployment
- Documentation complete
- Testing coverage complete

## Useful Commands

```bash
# Load testing with vegeta
echo "POST http://localhost:8080/v1/inference" | \
  vegeta attack -rate=100 -duration=60s | \
  vegeta report

# Smoke testing
./scripts/smoke.sh

# Helm chart installation
helm install api-router-service ./deployments/helm/api-router-service \
  -f ./deployments/helm/api-router-service/values-staging.yaml \
  -n api-router

# Check error responses
curl -v http://localhost:8080/v1/inference \
  -H "Content-Type: application/json" \
  -d '{"invalid": "request"}'
```

## Questions to Resolve

1. **Error Codes**: Confirm error codes match OpenAPI spec exactly?
2. **Helm Chart**: Are there existing Helm chart templates to follow?
3. **Load Testing**: What are the exact SLO targets to validate?
4. **Smoke Tests**: Should smoke tests run in CI/CD or manually?
5. **Quickstart**: Is there an existing quickstart document to validate?

---

**Ready to start Phase 8! Begin with T042 (error catalog) to establish consistent error handling across the service.**

