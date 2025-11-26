# Research: Admin API Service

**Feature**: 017-admin-api-service
**Date**: 2025-11-26

## Technology Decisions

### 1. HTTP Router

**Decision**: chi router
**Rationale**: Already used in api-router-service, lightweight, idiomatic Go, middleware support
**Alternatives considered**:
- gin: More opinionated, heavier, not used elsewhere in codebase
- gorilla/mux: Deprecated, chi is spiritual successor
- net/http ServeMux: Too basic for middleware composition

### 2. Database Access Pattern

**Decision**: Repository pattern with shared/go/dbutil
**Rationale**: Consistent with existing services, provides retry logic, connection pooling
**Alternatives considered**:
- Direct database/sql: No retry logic, manual connection management
- GORM: ORM overhead unnecessary, not used in codebase
- sqlx: Additional dependency, dbutil already provides needed abstractions

### 3. API Key Storage

**Decision**: Argon2id hashing (via shared/go/auth)
**Rationale**: Memory-hard, resistant to GPU attacks, recommended for new applications
**Alternatives considered**:
- bcrypt: Slower for legitimate users, less resistant to modern attacks
- scrypt: Good but Argon2 is newer standard
- SHA256: Not suitable for password/key hashing

### 4. Rate Limiting

**Decision**: Token bucket per API key using in-memory store with Redis fallback
**Rationale**: Simple, effective, Redis enables distributed rate limiting across replicas
**Alternatives considered**:
- Fixed window: Allows burst at window boundaries
- Sliding log: Memory intensive
- External rate limiter (Kong, Envoy): Over-engineering for internal service

### 5. Audit Logging

**Decision**: Structured JSON logs to stdout + database audit_logs table
**Rationale**: Logs for real-time monitoring, database for queryable audit trail
**Alternatives considered**:
- Only database: Loses logs if DB unavailable
- Only stdout: Not queryable for compliance
- External audit service: Additional infrastructure

### 6. Policy Sync Mechanism

**Decision**: HTTP polling (30s interval) for MVP, SSE for production
**Rationale**: Polling is simple to implement and debug; SSE provides real-time updates without WebSocket complexity
**Alternatives considered**:
- WebSocket: More complex, bidirectional not needed
- gRPC streaming: Would require new protocol, HTTP sufficient
- Message queue (NATS/Kafka): Over-engineering for this use case

### 7. Error Response Format

**Decision**: RFC 7807 Problem Details
**Rationale**: Industry standard, provides structured errors with type URIs
**Alternatives considered**:
- Custom format: Non-standard, harder for clients
- Plain text: Not machine-parseable
- GraphQL errors: Not using GraphQL

## Existing Codebase Patterns

### Authentication Middleware
Found in `shared/go/middleware/auth.go` - provides API key validation middleware that can be reused.

### Database Connection
Found in `shared/go/dbutil/` - provides connection pooling, retry logic, health checks.

### Structured Logging
Found in `shared/go/logging/` - provides zap-based structured logging with request context.

### Metrics
Found in `shared/go/metrics/` - provides Prometheus client helpers and standard RED metrics.

### Health Checks
Pattern in `services/api-router-service/internal/health/` - standard /healthz and /readyz endpoints.

## Database Schema Additions

### Required Tables

1. **routing_policies** - New table for routing policy storage (schema in routing-policy-api-addition.md)
2. **policy_sync_log** - New table for tracking api-router sync operations
3. **audit_logs** - May need to add if not existing (check migrations)

### Existing Tables Used

- **model_registry** - Model registration data (existing)
- **organizations** - Organization data (existing)
- **api_keys** - API key storage (existing)

## Security Requirements

1. **API Key Validation**: Constant-time comparison to prevent timing attacks
2. **TLS**: Required in production via cert-manager
3. **Headers**: X-Content-Type-Options, X-Frame-Options, CSP
4. **Rate Limiting**: Per API key to prevent abuse
5. **Audit**: All mutating operations logged with actor identity
6. **Input Validation**: All inputs validated, SQL injection prevented via parameterized queries

## Performance Considerations

1. **Connection Pool**: 10-50 connections, scale based on load
2. **Query Optimization**: Indexes on frequently filtered columns
3. **Pagination**: Cursor-based for large result sets
4. **Caching**: Consider Redis for frequently accessed policies
5. **Graceful Shutdown**: 30s grace period for in-flight requests

## Integration Points

1. **admin-cli**: Primary client, will be updated to use HTTP instead of direct DB
2. **api-router-service**: Consumes routing policies via sync endpoint
3. **Kubernetes**: Health probes, ConfigMaps, Secrets
4. **Prometheus**: Metrics scraping
5. **Grafana**: Dashboard for monitoring

