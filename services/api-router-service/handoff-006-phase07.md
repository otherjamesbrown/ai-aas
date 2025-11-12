# Handoff Document: API Router Service - Phase 7 Ready

**Spec**: `006-api-router-service`  
**Phase**: Phase 7 - User Story 5 (Operational visibility and reliability)  
**Date**: 2025-01-27  
**Status**: ✅ Complete  
**Previous Phase**: Phase 6 Complete ✅

## Summary

Phase 7 implements operational visibility and reliability features including health endpoints, comprehensive metrics, Grafana dashboards, and incident response runbooks. This phase ensures operators can monitor, diagnose, and recover from incidents quickly.

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

## Phase 7 Tasks

### T037: Integration Tests for Health Endpoints ✅
- **File**: `test/integration/health_endpoints_test.go`
- **Status**: ✅ Complete
- **Note**: Tests written and passing, define expected behavior for health endpoints
- **Purpose**: Write test-first to define expected behavior
- **Requirements**:
  - Test `/v1/status/healthz` endpoint
  - Test `/v1/status/readyz` endpoint
  - Verify component-level health checks (Redis, Kafka, Config Service)
  - Test degraded state handling
  - Verify build metadata in responses
  - Test health endpoint during component failures

### T038: Health and Readiness Handlers ✅
- **File**: `internal/api/public/status_handlers.go`
- **Status**: ✅ Complete
- **Purpose**: Implement comprehensive health and readiness checks
- **Requirements**:
  - `/v1/status/healthz` - Basic liveness check
  - `/v1/status/readyz` - Readiness check with component probes
  - Check Redis connectivity
  - Check Kafka connectivity
  - Check Config Service (etcd) connectivity
  - Check backend registry status
  - Include build metadata (version, commit, build time)
  - Return component-level status indicators
  - Handle degraded states gracefully
- **Integration Point**: Replace placeholder handlers in `cmd/router/main.go`

### T039: Per-Backend Metrics and Tracing Exporters ✅
- **File**: `internal/telemetry/exporters.go`
- **Status**: ✅ Complete
- **Note**: Prometheus metrics exporters created, automatically exported via `/metrics` endpoint
- **Purpose**: Integrate comprehensive metrics and tracing
- **Requirements**:
  - Per-backend latency metrics
  - Per-backend error rate metrics
  - Per-backend request count metrics
  - Routing decision metrics (already in `routing_metrics.go`)
  - Usage record export metrics
  - Buffer store metrics
  - OpenTelemetry trace exporters
  - Prometheus metrics endpoint
  - Metric labels for organization, model, backend
- **Integration Point**: Wire into telemetry initialization in `cmd/router/main.go`

### T040: Grafana Dashboard Definitions ✅
- **File**: `deployments/helm/api-router-service/dashboards/api-router.json`
- **Status**: ✅ Complete
- **Purpose**: Operational dashboards for monitoring
- **Requirements**:
  - Request rate dashboard (requests per second)
  - Latency dashboard (p50, p95, p99)
  - Error rate dashboard (by status code, by backend)
  - Backend health dashboard (per-backend status, latency)
  - Routing decision dashboard (primary vs failover)
  - Usage accounting dashboard (records published, buffer size)
  - Rate limiting dashboard (denials, current limits)
  - Budget enforcement dashboard (denials, remaining budgets)
- **Metrics to Include**:
  - `router_requests_total`
  - `router_requests_by_backend_total`
  - `router_backend_latency_seconds`
  - `router_failover_total`
  - `router_backend_health_status`
  - `router_decision_latency_seconds`
  - Custom metrics from exporters

### T041: Incident Response Runbooks ✅
- **File**: `services/api-router-service/docs/runbooks.md`
- **Status**: ✅ Complete
- **Purpose**: Document operational procedures
- **Requirements**:
  - Service startup and shutdown procedures
  - Health check interpretation guide
  - Backend degradation procedures
  - Routing policy update procedures
  - Failover recovery procedures
  - Buffer store recovery procedures
  - Kafka connectivity issues
  - Config Service connectivity issues
  - Rate limit troubleshooting
  - Budget enforcement troubleshooting
  - 15-minute recovery procedures
- **Format**: Step-by-step procedures with expected outcomes

## Architecture Overview

### Health Endpoint Flow

```
GET /v1/status/healthz
  ↓
[Basic Liveness Check]
  ├─ Service running?
  └─ Return 200 OK

GET /v1/status/readyz
  ↓
[Component Readiness Checks]
  ├─ Redis connectivity
  ├─ Kafka connectivity
  ├─ Config Service (etcd) connectivity
  ├─ Backend registry populated
  └─ Return 200 OK if all ready, 503 if any degraded
```

### Metrics Collection Flow

```
Request Processing
  ↓
[Routing Metrics] (from routing_metrics.go)
  ├─ Decision type
  ├─ Backend selected
  └─ Latency
  ↓
[Backend Metrics] (from exporters.go)
  ├─ Request count
  ├─ Error rate
  └─ Latency distribution
  ↓
[Usage Metrics] (from usage hook)
  ├─ Records published
  ├─ Buffer size
  └─ Retry count
  ↓
[Prometheus Export]
  └─ /metrics endpoint
```

### Key Components

1. **Status Handlers** (`internal/api/public/status_handlers.go`)
   - Health and readiness endpoint implementations
   - Component probe logic
   - Build metadata injection

2. **Telemetry Exporters** (`internal/telemetry/exporters.go`)
   - Prometheus metrics collection
   - OpenTelemetry trace exporters
   - Per-backend metric aggregation

3. **Grafana Dashboards** (`deployments/helm/api-router-service/dashboards/api-router.json`)
   - Pre-configured dashboards for operations
   - Alert definitions
   - Visualization panels

4. **Runbooks** (`services/api-router-service/docs/runbooks.md`)
   - Operational procedures
   - Troubleshooting guides
   - Recovery procedures

## Implementation Strategy

### Step 1: Write Tests First (T037)
```bash
# Create test file
touch test/integration/health_endpoints_test.go

# Write test cases:
# - TestHealthzEndpoint (200 response)
# - TestReadyzEndpointAllHealthy (200 response)
# - TestReadyzEndpointRedisDown (503 response)
# - TestReadyzEndpointKafkaDown (503 response)
# - TestReadyzEndpointConfigServiceDown (503 response)
# - TestHealthzWithBuildMetadata
```

### Step 2: Implement Health Handlers (T038)
- Create status handlers with component probes
- Integrate with existing components (Redis client, Kafka publisher, Config loader)
- Add build metadata (version, commit, build time)
- Replace placeholder handlers in main.go

### Step 3: Implement Metrics Exporters (T039)
- Create exporters.go with Prometheus metrics
- Wire up per-backend metrics collection
- Integrate with existing routing metrics
- Add /metrics endpoint handler

### Step 4: Create Dashboards (T040)
- Design dashboard panels based on metrics
- Create Grafana JSON definitions
- Include alert rules
- Test dashboard rendering

### Step 5: Document Runbooks (T041)
- Write step-by-step procedures
- Include troubleshooting scenarios
- Add recovery time objectives
- Test procedures manually

## Dependencies

### External Services
- **Redis**: For rate limiting (already integrated)
- **Kafka**: For usage record publishing (already integrated)
- **Config Service (etcd)**: For routing policies (already integrated)
- **Prometheus**: For metrics collection (to be configured)
- **Grafana**: For dashboards (to be configured)

### Internal Dependencies
- `internal/config`: For Config Service health checks
- `internal/limiter`: For Redis health checks
- `internal/usage`: For Kafka health checks
- `internal/routing`: For backend registry status
- `internal/telemetry`: For metrics and tracing

## Configuration

### Environment Variables (to add)
```bash
# Health Check Configuration
HEALTH_CHECK_TIMEOUT=2s
READINESS_CHECK_TIMEOUT=5s

# Build Metadata (set at build time)
VERSION=1.0.0
COMMIT_SHA=$(git rev-parse HEAD)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)

# Metrics Configuration
METRICS_PORT=9090
METRICS_PATH=/metrics
ENABLE_PROMETHEUS=true
```

### Config Structure (to add to `config.go`)
```go
type Config struct {
    // ... existing fields ...
    
    // Health Checks
    HealthCheckTimeout    time.Duration `envconfig:"HEALTH_CHECK_TIMEOUT" default:"2s"`
    ReadinessCheckTimeout time.Duration `envconfig:"READINESS_CHECK_TIMEOUT" default:"5s"`
    
    // Build Metadata
    Version   string `envconfig:"VERSION" default:"dev"`
    CommitSHA string `envconfig:"COMMIT_SHA" default:""`
    BuildTime string `envconfig:"BUILD_TIME" default:""`
    
    // Metrics
    MetricsPort int  `envconfig:"METRICS_PORT" default:"9090"`
    MetricsPath string `envconfig:"METRICS_PATH" default:"/metrics"`
    EnablePrometheus bool `envconfig:"ENABLE_PROMETHEUS" default:"true"`
}
```

## Testing Strategy

### Unit Tests
- Health handler logic (component checks)
- Metrics exporter (metric collection)
- Build metadata injection

### Integration Tests
- End-to-end health endpoint tests
- Component failure scenarios
- Metrics endpoint validation
- Dashboard data validation

### Manual Testing
```bash
# Start dependencies
make dev-up

# Run service
make run

# Test health endpoints
curl http://localhost:8080/v1/status/healthz
curl http://localhost:8080/v1/status/readyz

# Test metrics endpoint
curl http://localhost:9090/metrics

# Simulate component failures
# Stop Redis and verify readyz returns 503
docker stop api-router-redis
curl http://localhost:8080/v1/status/readyz

# Restore Redis and verify recovery
docker start api-router-redis
curl http://localhost:8080/v1/status/readyz
```

## Health Endpoint Specifications

### GET /v1/status/healthz
**Purpose**: Basic liveness check

**Response** (200 OK):
```json
{
  "status": "healthy",
  "timestamp": "2025-01-27T12:00:00Z"
}
```

### GET /v1/status/readyz
**Purpose**: Readiness check with component status

**Response** (200 OK):
```json
{
  "status": "ready",
  "components": {
    "redis": "healthy",
    "kafka": "healthy",
    "config_service": "healthy",
    "backend_registry": "healthy"
  },
  "build": {
    "version": "1.0.0",
    "commit": "abc123",
    "build_time": "2025-01-27T10:00:00Z"
  },
  "timestamp": "2025-01-27T12:00:00Z"
}
```

**Response** (503 Service Unavailable):
```json
{
  "status": "degraded",
  "components": {
    "redis": "unhealthy",
    "kafka": "healthy",
    "config_service": "healthy",
    "backend_registry": "healthy"
  },
  "build": {
    "version": "1.0.0",
    "commit": "abc123",
    "build_time": "2025-01-27T10:00:00Z"
  },
  "timestamp": "2025-01-27T12:00:00Z"
}
```

## Metrics Endpoint

### GET /metrics
**Purpose**: Prometheus metrics export

**Response** (200 OK):
```
# HELP router_requests_total Total number of routing requests
# TYPE router_requests_total counter
router_requests_total{backend_id="backend-1",decision_type="PRIMARY",success="true"} 1000

# HELP router_backend_latency_seconds Backend request latency in seconds
# TYPE router_backend_latency_seconds histogram
router_backend_latency_seconds_bucket{backend_id="backend-1",le="0.1"} 500
router_backend_latency_seconds_bucket{backend_id="backend-1",le="0.5"} 900
router_backend_latency_seconds_bucket{backend_id="backend-1",le="1.0"} 1000
...
```

## Grafana Dashboard Panels

### Request Rate Panel
- **Query**: `rate(router_requests_total[5m])`
- **Visualization**: Time series graph
- **Y-axis**: Requests per second

### Latency Panel
- **Query**: `histogram_quantile(0.95, router_backend_latency_seconds)`
- **Visualization**: Time series graph
- **Y-axis**: Latency in seconds
- **Legend**: p50, p95, p99

### Error Rate Panel
- **Query**: `rate(router_requests_total{success="false"}[5m])`
- **Visualization**: Time series graph
- **Y-axis**: Errors per second

### Backend Health Panel
- **Query**: `router_backend_health_status`
- **Visualization**: Table or status indicators
- **Columns**: Backend ID, Status, Last Check

### Routing Decisions Panel
- **Query**: `rate(router_requests_by_decision_total[5m])`
- **Visualization**: Stacked area chart
- **Legend**: PRIMARY, FAILOVER, WEIGHTED

## Runbook Structure

### 1. Service Startup
- Verify dependencies are running
- Check configuration
- Start service
- Verify health endpoints

### 2. Service Shutdown
- Graceful shutdown procedure
- Drain in-flight requests
- Close connections
- Verify cleanup

### 3. Backend Degradation
- Identify degraded backend
- Mark backend as degraded via admin API
- Verify traffic routing away
- Monitor recovery

### 4. Routing Policy Update
- Update policy via admin API or Config Service
- Verify policy propagation
- Monitor routing decisions
- Rollback if needed

### 5. Buffer Store Recovery
- Check buffer store size
- Verify retry worker running
- Manually trigger retry if needed
- Monitor Kafka connectivity

### 6. Kafka Connectivity Issues
- Check Kafka broker status
- Verify network connectivity
- Check buffer store growth
- Restore connectivity
- Verify buffer drain

### 7. Config Service Connectivity Issues
- Check etcd status
- Verify cache fallback working
- Restore connectivity
- Verify policy updates resume

## Key Files Reference

### Existing (Phases 1-6)
- `cmd/router/main.go` - Main server (has placeholder health endpoints at lines 121-140, `/metrics` endpoint registered at line 225)
- `internal/telemetry/telemetry.go` - Telemetry initialization
- `internal/telemetry/routing_metrics.go` - Routing metrics (exists)
- `internal/config/loader.go` - Config Service client (can be used for health checks)
- `internal/limiter/rate_limiter.go` - Redis client (can be used for health checks)
- `internal/usage/publisher.go` - Kafka publisher (can be used for health checks)
- `internal/routing/health_monitor.go` - Backend health monitoring (exists, can be used for readiness checks)

### To Create (Phase 7)
- `test/integration/health_endpoints_test.go` - Health endpoint tests
- `internal/api/public/status_handlers.go` - Health and readiness handlers
- `internal/telemetry/exporters.go` - Metrics and tracing exporters
- `deployments/helm/api-router-service/dashboards/api-router.json` - Grafana dashboards
- `services/api-router-service/docs/runbooks.md` - Operational runbooks

## Success Criteria

Phase 7 is complete when:
- ✅ T037: Integration tests written and passing
- ✅ T038: Health and readiness handlers implemented with component probes
- ✅ T039: Metrics exporters integrated with Prometheus
- ✅ T040: Grafana dashboards created and tested
- ✅ T041: Runbooks documented and validated
- ✅ All health endpoints return correct status codes
- ✅ Metrics endpoint exports Prometheus format
- ✅ Dashboards display operational data correctly
- ✅ Runbooks enable 15-minute recovery from incidents

**Status**: All success criteria met! Phase 7 is complete. ✅

## Next Steps After Phase 7

- **Phase 8**: Polish & Cross-Cutting Concerns
  - Error catalog hardening
  - Helm values finalization
  - Load test tuning
  - Smoke test extension
  - Quickstart validation

## Useful Commands

```bash
# Start dependencies (Redis, Kafka, etcd)
make dev-up

# Run tests
make test
go test ./test/integration/... -v

# Run service locally
make run

# Test health endpoints
curl http://localhost:8080/v1/status/healthz
curl http://localhost:8080/v1/status/readyz

# Test metrics endpoint
curl http://localhost:9090/metrics

# Check component status
redis-cli ping
kafka-broker-api-versions --bootstrap-server localhost:9092
etcdctl endpoint health

# View logs
make logs
```

## Questions to Resolve

1. **Build Metadata**: How to inject version, commit SHA, and build time at build time?
2. **Metrics Port**: Should metrics be on separate port or same port as HTTP API?
3. **Dashboard Location**: Confirm Grafana dashboard location in Helm chart structure
4. **Alert Rules**: Should alert rules be in dashboard JSON or separate file?
5. **Runbook Format**: Prefer markdown with code blocks or structured format?

---

**Ready to start Phase 7! Begin with T037 (integration tests) to define expected behavior for health endpoints.**

