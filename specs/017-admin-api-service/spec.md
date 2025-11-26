# Feature Specification: Admin API Service

**Feature Branch**: `017-admin-api-service`
**Created**: 2025-11-26
**Status**: Draft
**Input**: User need: "Provide an HTTP API service that admin-cli can communicate with instead of requiring direct database access. Enable secure, audited administrative operations without exposing database credentials to operators."

## Clarifications

### Session 2025-11-26

- Q: Should this service replace all direct database access in admin-cli? → A: Yes. The admin-cli should become a thin client that only makes HTTP requests to the admin-api-service. All database operations move to the service layer.
- Q: What authentication mechanism should be used? → A: Service-to-service authentication using API keys with scoped permissions (admin scope). Keys are managed separately and rotated regularly. Optionally support mTLS for production environments.
- Q: Should this service be publicly accessible or cluster-internal only? → A: Cluster-internal by default (ClusterIP service). Operators use kubectl port-forward for local access. Production can optionally expose via ingress with additional auth layers (OAuth2 proxy, VPN).
- Q: What operations need to be supported initially? → A: Model registry management (register, update, delete, list models), organization management (create, list, query), and health/status endpoints. Additional operations can be added incrementally.

## User Scenarios & Testing *(mandatory)*

### User Story 1 (US-001) - Model Registry Management via API (Priority: P1)

As a platform operator, I can register and manage model deployments through a secure API without direct database access, ensuring proper authentication and audit trails.

**Why this priority**: This is the immediate need discovered when trying to register models. Without this, operators must have direct database credentials which violates security best practices.

**Independent Test**: Can be fully tested by deploying the service and using curl or the admin-cli to perform model registration operations. This delivers immediate value by removing the need for database credentials in operational tooling.

**Acceptance Scenarios**:

1. **[Primary]** **Given** the admin-api-service is running in the cluster, **When** I call `POST /v1/registry/models` with valid authentication and model details (name, endpoint, environment, namespace), **Then** the model is registered in the database, the service returns 201 Created with the model ID, and the operation is logged in audit logs.

2. **[Primary]** **Given** a model already exists for a given environment, **When** I call `POST /v1/registry/models` with the same model name and environment, **Then** the service performs an upsert (updates existing record), returns 200 OK with updated model details, and audit log captures the update operation.

3. **[Primary]** **Given** I need to list all registered models, **When** I call `GET /v1/registry/models?environment=development`, **Then** the service returns 200 OK with JSON array of models filtered by environment, including model name, endpoint, status, namespace, and timestamps.

4. **[Alternate]** **Given** I need to update a model's status or endpoint, **When** I call `PATCH /v1/registry/models/{model-name}?environment=development` with updated fields, **Then** the service updates only the specified fields, returns 200 OK with updated model, and maintains revision history in audit logs.

5. **[Exception]** **Given** I attempt to register a model with invalid data (missing required fields, invalid status enum), **When** the request is processed, **Then** the service returns 400 Bad Request with detailed validation errors in JSON format, and no partial data is written to database.

6. **[Exception]** **Given** I call the API without proper authentication headers, **When** any endpoint is accessed, **Then** the service returns 401 Unauthorized with clear error message indicating authentication is required, and the request is logged for security monitoring.

---

### User Story 2 (US-002) - Organization and System Management (Priority: P2)

As a platform operator, I can manage organizations and query system state through the API for administrative workflows.

**Why this priority**: Enables full administrative capabilities without database access, supporting the broader platform management needs.

**Independent Test**: Can be tested independently by calling organization endpoints and verifying data consistency. Delivers value even without model registry by enabling org management.

**Acceptance Scenarios**:

1. **[Primary]** **Given** I need to create a default organization for testing, **When** I call `POST /v1/organizations` with organization details (slug, display name, plan tier), **Then** the organization is created with generated UUID, service returns 201 Created, and organization ID can be used in subsequent operations.

2. **[Primary]** **Given** I need to query organizations, **When** I call `GET /v1/organizations?limit=50&offset=0`, **Then** the service returns paginated list of organizations with id, slug, display_name, plan_tier, status, created_at, and provides pagination metadata (total, next_offset).

3. **[Primary]** **Given** I need to check service health, **When** I call `GET /healthz` (unauthenticated endpoint), **Then** the service returns 200 OK with JSON showing service status, database connectivity, and version information.

4. **[Alternate]** **Given** I need detailed organization info, **When** I call `GET /v1/organizations/{org-id}`, **Then** the service returns 200 OK with full organization details including statistics (user count, API key count, token usage), and related resources.

5. **[Exception]** **Given** I attempt to create an organization with duplicate slug, **When** the request is processed, **Then** the service returns 409 Conflict with error message indicating slug already exists, and suggests alternative slugs.

---

### User Story 3 (US-003) - Audit Logging and Observability (Priority: P2)

As a security engineer, I can track all administrative operations through comprehensive audit logs and metrics for compliance and troubleshooting.

**Why this priority**: Security and compliance requirement. All privileged operations must be audited with actor identity, timestamp, and outcome.

**Independent Test**: Can be tested by performing operations and verifying audit log entries are created with correct fields. Delivers compliance value independently.

**Acceptance Scenarios**:

1. **[Primary]** **Given** any successful API operation is performed, **When** the operation completes, **Then** an audit log entry is created with actor (API key ID or subject), action, target (resource ID/name), timestamp, outcome (success), and request metadata (IP, user-agent).

2. **[Primary]** **Given** an API operation fails (validation error, authorization failure), **When** the failure occurs, **Then** an audit log entry is created with failure outcome, error details, and the attempted operation is captured for security review.

3. **[Primary]** **Given** I need to query audit logs, **When** I call `GET /v1/audit-logs?actor={actor-id}&from={timestamp}&to={timestamp}`, **Then** the service returns paginated audit log entries filtered by parameters, in reverse chronological order.

4. **[Alternate]** **Given** the service is processing requests, **When** metrics are collected, **Then** Prometheus metrics are exposed on `/metrics` endpoint including request count by endpoint/status, request duration histograms, database query duration, and active connections.

5. **[Exception]** **Given** database write fails during audit log creation, **When** the failure occurs, **Then** the service logs error to stderr, emits metric for audit_log_failure, and continues processing (does not block main operation), but alerts monitoring system.

---

### Edge Cases

1. **Database Connection Loss**
   - **Given** the service loses database connectivity mid-request, **When** operation is attempted, **Then** service returns 503 Service Unavailable with retry-after header, implements connection retry with exponential backoff, and health endpoint reflects degraded state.

2. **Concurrent Model Registration**
   - **Given** two operators register the same model simultaneously, **When** both requests reach the service, **Then** database unique constraint prevents duplicate, one request succeeds with 201, the other receives 409 Conflict, and both operations are audited.

3. **Large Result Sets**
   - **Given** listing models returns thousands of results, **When** query is executed, **Then** service implements cursor-based pagination (default page size 100), streams results to avoid memory exhaustion, and provides next cursor token in response.

4. **API Key Rotation**
   - **Given** admin-cli is using an API key that gets rotated, **When** old key is revoked, **Then** service returns 401 Unauthorized with clear message indicating key is invalid/revoked, and provides guidance on obtaining new key.

5. **Request Validation**
   - **Given** client sends malformed JSON or invalid field types, **When** request is parsed, **Then** service returns 400 Bad Request with detailed field-level validation errors in standard JSON problem format (RFC 7807).

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide RESTful HTTP API endpoints for model registry operations (POST /v1/registry/models, GET /v1/registry/models, PATCH /v1/registry/models/{name}, DELETE /v1/registry/models/{name}).
- **FR-002**: Provide organization management endpoints (POST /v1/organizations, GET /v1/organizations, GET /v1/organizations/{id}, PATCH /v1/organizations/{id}).
- **FR-003**: Provide health check endpoint (GET /healthz) for Kubernetes liveness/readiness probes, including database connectivity check.
- **FR-004**: Provide metrics endpoint (GET /metrics) exposing Prometheus-format metrics for request rates, durations, error rates, and database statistics.
- **FR-005**: Provide audit logging for all mutating operations (POST, PATCH, DELETE) with actor, action, target, timestamp, outcome, and request metadata.
- **FR-006**: Provide API key authentication for all endpoints except /healthz and /metrics (which use separate auth mechanisms or are unauthenticated).
- **FR-007**: Provide request validation with detailed error messages for malformed requests, missing required fields, and invalid enum values.
- **FR-008**: Provide pagination support for list endpoints using cursor-based or offset-based pagination with configurable page sizes (default 100, max 500).
- **FR-009**: Provide filtering support for list endpoints using query parameters (e.g., ?environment=development, ?status=ready).
- **FR-010**: Provide idempotent operations for resource creation (POST endpoints should support upsert behavior based on natural keys like model_name + environment).
- **FR-011**: Provide structured error responses following RFC 7807 Problem Details format with type, title, status, detail, and instance fields.
- **FR-012**: Provide API versioning using URL path prefix (/v1/) to support future backward-incompatible changes.

### Non-Functional Requirements

**Performance:**
- **NFR-001**: API endpoints respond within 200ms at p95 for single-record operations under normal load (database responding within 50ms).
- **NFR-002**: List endpoints stream results and respond with first page within 500ms at p95 for result sets up to 10,000 records.
- **NFR-003**: Service handles concurrent requests up to 100 req/s with graceful degradation (rate limiting) beyond that threshold.
- **NFR-004**: Database connection pool maintains 10-50 connections with automatic scaling based on load and proper timeout/cleanup on idle connections.

**Reliability:**
- **NFR-005**: Service implements graceful shutdown on SIGTERM, completing in-flight requests (up to 30s grace period) before terminating.
- **NFR-006**: Database operations implement retry logic with exponential backoff (max 3 attempts) for transient failures (connection timeouts, deadlocks).
- **NFR-007**: Service implements circuit breaker pattern for database access, opening circuit after 5 consecutive failures and attempting recovery after 30s.
- **NFR-008**: Service survives database connection loss, returning 503 errors and automatically reconnecting when database becomes available.

**Security:**
- **NFR-009**: API keys are validated on every request using constant-time comparison to prevent timing attacks.
- **NFR-010**: API keys are stored hashed in database (bcrypt or argon2) and never logged in plaintext. Only last 4 characters shown in audit logs.
- **NFR-011**: All responses include security headers (X-Content-Type-Options: nosniff, X-Frame-Options: DENY, Content-Security-Policy).
- **NFR-012**: Service supports TLS for production deployments with automatic certificate renewal via cert-manager.
- **NFR-013**: Rate limiting is applied per API key (100 req/min default) to prevent abuse and protect database from overload.
- **NFR-014**: Audit logs capture client IP address (respecting X-Forwarded-For header) for security investigations and access pattern analysis.

**Observability:**
- **NFR-015**: All errors are logged with structured logging (JSON format) including request ID, endpoint, error type, and stack trace for debugging.
- **NFR-016**: Request/response payloads are logged at debug level (sanitizing sensitive fields like API keys, credentials) for troubleshooting.
- **NFR-017**: Metrics include RED metrics (Rate, Errors, Duration) per endpoint, plus database-specific metrics (connection pool utilization, query duration).
- **NFR-018**: Service emits OpenTelemetry traces for distributed tracing, with spans for HTTP handlers, database queries, and external service calls.

**Maintainability:**
- **NFR-019**: Service uses shared database access patterns from shared/dbutil package for consistent error handling and retry logic.
- **NFR-020**: API schema is defined using OpenAPI 3.0 specification with automatic validation and documentation generation.
- **NFR-021**: Service includes comprehensive integration tests that run against real PostgreSQL database (using testcontainers) to validate database interactions.
- **NFR-022**: Service configuration uses environment variables with sensible defaults and clear documentation (see config.example.env).

## API Design

### Authentication

All endpoints except `/healthz` require authentication using API Key in header:
```
Authorization: Bearer <api-key>
```

### Model Registry Endpoints

#### Register/Update Model
```http
POST /v1/registry/models
Content-Type: application/json

{
  "model_name": "gpt-oss-20b",
  "deployment_endpoint": "gpt-oss-20b-predictor-00001-private.development.svc.cluster.local:80",
  "deployment_environment": "development",
  "deployment_namespace": "development",
  "deployment_status": "ready",
  "revision": 1,
  "deployment_target": "managed",
  "cost_per_1k_tokens": 0.0015,
  "metadata": {
    "gpu_type": "a100",
    "model_size": "20B"
  }
}

Response 201 Created:
{
  "model_id": "uuid",
  "model_name": "gpt-oss-20b",
  "deployment_endpoint": "...",
  "deployment_environment": "development",
  "deployment_status": "ready",
  "created_at": "2025-11-26T10:00:00Z",
  "updated_at": "2025-11-26T10:00:00Z"
}
```

#### List Models
```http
GET /v1/registry/models?environment=development&status=ready&limit=100&offset=0

Response 200 OK:
{
  "models": [
    {
      "model_id": "uuid",
      "model_name": "gpt-oss-20b",
      "deployment_endpoint": "...",
      "deployment_environment": "development",
      "deployment_status": "ready",
      "deployment_namespace": "development",
      "created_at": "2025-11-26T10:00:00Z",
      "updated_at": "2025-11-26T10:00:00Z"
    }
  ],
  "pagination": {
    "total": 1,
    "limit": 100,
    "offset": 0,
    "next_offset": null
  }
}
```

#### Get Model
```http
GET /v1/registry/models/{model_name}?environment=development

Response 200 OK:
{
  "model_id": "uuid",
  "model_name": "gpt-oss-20b",
  "deployment_endpoint": "...",
  "deployment_environment": "development",
  "deployment_status": "ready",
  "deployment_namespace": "development",
  "revision": 1,
  "cost_per_1k_tokens": 0.0015,
  "metadata": {...},
  "created_at": "2025-11-26T10:00:00Z",
  "updated_at": "2025-11-26T10:00:00Z"
}
```

#### Update Model
```http
PATCH /v1/registry/models/{model_name}?environment=development
Content-Type: application/json

{
  "deployment_status": "degraded",
  "deployment_endpoint": "new-endpoint:80"
}

Response 200 OK:
{
  "model_id": "uuid",
  "model_name": "gpt-oss-20b",
  "deployment_status": "degraded",
  "deployment_endpoint": "new-endpoint:80",
  "updated_at": "2025-11-26T11:00:00Z"
}
```

#### Delete Model
```http
DELETE /v1/registry/models/{model_name}?environment=development

Response 204 No Content
```

### Organization Endpoints

#### Create Organization
```http
POST /v1/organizations
Content-Type: application/json

{
  "slug": "acme-corp",
  "display_name": "ACME Corporation",
  "plan_tier": "enterprise",
  "budget_limit_tokens": 1000000000
}

Response 201 Created:
{
  "organization_id": "uuid",
  "slug": "acme-corp",
  "display_name": "ACME Corporation",
  "plan_tier": "enterprise",
  "status": "active",
  "created_at": "2025-11-26T10:00:00Z"
}
```

#### List Organizations
```http
GET /v1/organizations?limit=50&offset=0

Response 200 OK:
{
  "organizations": [...],
  "pagination": {
    "total": 10,
    "limit": 50,
    "offset": 0
  }
}
```

### System Endpoints

#### Health Check
```http
GET /healthz

Response 200 OK:
{
  "status": "healthy",
  "version": "v1.0.0",
  "database": "connected",
  "timestamp": "2025-11-26T10:00:00Z"
}
```

#### Metrics
```http
GET /metrics

Response 200 OK (Prometheus format):
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="POST",endpoint="/v1/registry/models",status="201"} 42
...
```

## Implementation Plan

### Phase 1: Core Service & Model Registry (Week 1)
- Set up Go service structure using shared libraries
- Implement database connection management with connection pooling
- Implement model registry endpoints (POST, GET, PATCH, DELETE)
- Add API key authentication middleware
- Add structured logging and basic metrics
- Create Kubernetes deployment manifests (Deployment, Service, ConfigMap)

### Phase 2: Organization Management & Audit (Week 2)
- Implement organization endpoints (POST, GET)
- Implement audit logging for all operations
- Add request validation middleware with detailed error responses
- Add pagination support for list endpoints
- Add filtering support (query parameters)
- Create integration tests using testcontainers

### Phase 3: Production Hardening (Week 3)
- Implement rate limiting per API key
- Add circuit breaker for database operations
- Implement graceful shutdown
- Add OpenTelemetry tracing
- Add comprehensive error handling and recovery
- Performance testing and optimization
- Security review (TLS, headers, key management)

### Phase 4: Admin-CLI Integration (Week 4)
- Update admin-cli to use HTTP client instead of database client
- Remove database dependencies from admin-cli
- Update admin-cli commands to call API endpoints
- Add error handling and retry logic in admin-cli
- Update documentation and runbooks
- End-to-end testing of full workflow

## Security Considerations

1. **API Key Management**: Keys stored hashed in database, transmitted over TLS, rotated regularly, scoped to admin operations only.

2. **Network Access**: Service deployed as ClusterIP by default, only accessible within cluster. Operators use kubectl port-forward. Production can add ingress with OAuth2 proxy.

3. **Audit Logging**: All operations logged with actor identity, captures who did what when for compliance and security investigations.

4. **Input Validation**: All inputs validated against schema, SQL injection prevented using parameterized queries, no dynamic SQL generation.

5. **Rate Limiting**: Per-key rate limits prevent abuse and protect database from overload attacks.

6. **Database Credentials**: Service uses single service account with minimal required permissions (INSERT, UPDATE, DELETE, SELECT on specific tables only).

## Migration Strategy

1. **Deploy admin-api-service** alongside existing admin-cli without disrupting current workflows.

2. **Create admin API keys** for operators and store in secrets management system (Kubernetes secrets or vault).

3. **Update admin-cli** to support both database mode (legacy) and API mode (new) using feature flag or environment variable.

4. **Gradual rollout**: Operators switch to API mode as admin-api-service proves stable in development environment.

5. **Remove database access**: Once all operators migrated, revoke database credentials from admin-cli config and remove database dependencies.

6. **Deprecation**: Mark direct database access as deprecated, remove in next major version after 3 month transition period.

## Success Criteria

1. **Security**: Operators no longer require direct database credentials for administrative operations.

2. **Auditability**: All administrative operations are logged with actor identity and captured in audit logs queryable via API.

3. **Reliability**: Service achieves 99.9% uptime in production with graceful handling of database failures.

4. **Performance**: API operations complete within 200ms p95 latency, supporting operational workflows without delays.

5. **Adoption**: 100% of administrative operations migrate from direct database access to API-based operations within 3 months of release.
