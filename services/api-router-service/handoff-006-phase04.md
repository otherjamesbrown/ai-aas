# Handoff Document: API Router Service - Phase 4 Ready

**Spec**: `006-api-router-service`  
**Phase**: Phase 4 - User Story 2 (Enforce budgets and safe usage)  
**Date**: 2025-01-27  
**Status**: 🚀 Ready to Start  
**Previous Phase**: Phase 3 Complete ✅

## Summary

Phase 4 implements budget enforcement, quota management, and rate limiting with auditable denials. This phase adds critical safety controls to prevent over-spending and ensure fair usage across organizations.

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

## Phase 4 Tasks

### T019: Integration Test for Quota Exhaustion ✅ START HERE
- **File**: `test/integration/limiter_budget_test.go`
- **Status**: ⏳ To Do
- **Purpose**: Write test-first to define expected behavior
- **Requirements**:
  - Simulate quota exhaustion scenario
  - Verify HTTP 402 (Payment Required) response
  - Verify HTTP 429 (Too Many Requests) for rate limits
  - Confirm audit records are emitted
  - Test both budget and rate limit denials

### T020: Budget Service Client Integration
- **File**: `internal/limiter/budget_client.go`
- **Status**: ⏳ To Do
- **Purpose**: Client for checking budgets/quotas
- **Requirements**:
  - Check organization budget before processing request
  - Check quota limits (daily/monthly)
  - Handle budget service unavailability gracefully
  - Return structured budget/quota status
- **Dependencies**: Budget service API (may need to stub initially)

### T021: Redis Token-Bucket Rate Limiter
- **File**: `internal/limiter/rate_limiter.go`
- **Status**: ⏳ To Do
- **Purpose**: Per-organization and per-key rate limiting
- **Requirements**:
  - Redis-backed token bucket implementation
  - Configurable RPS (requests per second) and burst size
  - Per-organization limits
  - Per-API-key limits
  - Thread-safe operations
- **Dependencies**: Redis (already in docker-compose)

### T022: Rate-Limit and Budget Middleware
- **File**: `internal/api/public/middleware.go`
- **Status**: ⏳ To Do
- **Purpose**: Attach limiter checks to request pipeline
- **Requirements**:
  - Middleware that runs before handler
  - Check rate limits first (fast fail)
  - Check budget/quota second
  - Return appropriate HTTP status codes (402, 429)
  - Include retry-after headers
- **Integration Point**: Add to middleware stack in `cmd/router/main.go`

### T023: Audit Event Emission
- **File**: `internal/usage/audit_logger.go`
- **Status**: ⏳ To Do
- **Purpose**: Emit audit events for deny/queue outcomes
- **Requirements**:
  - Log budget denial events
  - Log rate limit denial events
  - Include request context (org, key, model, tokens)
  - Emit to Kafka (or logger fallback)
  - Structured event format
- **Dependencies**: Kafka (already in docker-compose)

### T024: Structured Limit Error Responses
- **File**: `internal/telemetry/limits.go`
- **Status**: ⏳ To Do
- **Purpose**: Produce structured error responses and metrics
- **Requirements**:
  - `LimitErrorResponse` struct matching OpenAPI spec
  - Include limit type (budget, quota, rate_limit)
  - Include current usage vs limit
  - Include retry-after information
  - Emit Prometheus metrics for limit denials
- **OpenAPI Reference**: See `LimitErrorResponse` schema in contracts

## Architecture Overview

### Request Flow with Limits

```
HTTP Request
  ↓
[Authentication Middleware] ✅ Phase 3
  ↓
[Rate Limit Middleware] ⏳ Phase 4
  ├─ Check per-org rate limit
  ├─ Check per-key rate limit
  └─ Return 429 if exceeded
  ↓
[Budget Middleware] ⏳ Phase 4
  ├─ Check organization budget
  ├─ Check quota limits
  └─ Return 402 if exceeded
  ↓
[Handler: Routing & Forwarding] ✅ Phase 3
  ↓
[Audit Logger] ⏳ Phase 4
  └─ Emit usage/denial events
```

### Key Components

1. **Rate Limiter** (`internal/limiter/rate_limiter.go`)
   - Redis token bucket implementation
   - Per-organization and per-key buckets
   - Configurable refill rates

2. **Budget Client** (`internal/limiter/budget_client.go`)
   - HTTP client for budget service
   - Budget/quota checking
   - Graceful degradation

3. **Middleware** (`internal/api/public/middleware.go`)
   - Chi middleware for rate limiting
   - Chi middleware for budget checking
   - Error response formatting

4. **Audit Logger** (`internal/usage/audit_logger.go`)
   - Kafka producer for audit events
   - Structured event format
   - Fallback to logger

5. **Telemetry** (`internal/telemetry/limits.go`)
   - Prometheus metrics
   - Structured error responses
   - Limit state tracking

## Implementation Strategy

### Step 1: Write Tests First (T019)
```bash
# Create test file
touch test/integration/limiter_budget_test.go

# Write test cases:
# - TestRateLimitExceeded (429 response)
# - TestBudgetExceeded (402 response)
# - TestQuotaExceeded (402 response)
# - TestAuditEventEmitted
```

### Step 2: Implement Core Components (T020, T021)
- Start with rate limiter (T021) - simpler, Redis already available
- Then budget client (T020) - may need to stub budget service
- Both can be implemented in parallel

### Step 3: Wire Middleware (T022)
- Create middleware functions
- Add to middleware stack in `main.go`
- Test with integration tests

### Step 4: Add Observability (T023, T024)
- Implement audit logger
- Add structured error responses
- Add Prometheus metrics

## Dependencies

### External Services
- **Redis**: For rate limiting (already in docker-compose)
- **Kafka**: For audit events (already in docker-compose)
- **Budget Service**: For budget/quota checks (may need to stub initially)

### Internal Dependencies
- `internal/config`: For rate limit configuration
- `internal/telemetry`: For metrics and logging
- `internal/auth`: For organization/key context

## Configuration

### Environment Variables (to add)
```bash
# Rate Limiting
RATE_LIMIT_REDIS_ADDR=localhost:6379
RATE_LIMIT_DEFAULT_RPS=100
RATE_LIMIT_BURST_SIZE=200

# Budget Service
BUDGET_SERVICE_ENDPOINT=http://localhost:8082
BUDGET_SERVICE_TIMEOUT=2s

# Audit/Kafka
KAFKA_BROKERS=localhost:9092
KAFKA_AUDIT_TOPIC=audit.router
```

### Config Structure (to add to `config.go`)
```go
type Config struct {
    // ... existing fields ...
    
    // Rate Limiting
    RateLimitRedisAddr string `envconfig:"RATE_LIMIT_REDIS_ADDR"`
    RateLimitDefaultRPS int `envconfig:"RATE_LIMIT_DEFAULT_RPS" default:"100"`
    RateLimitBurstSize int `envconfig:"RATE_LIMIT_BURST_SIZE" default:"200"`
    
    // Budget Service
    BudgetServiceEndpoint string `envconfig:"BUDGET_SERVICE_ENDPOINT"`
    BudgetServiceTimeout time.Duration `envconfig:"BUDGET_SERVICE_TIMEOUT" default:"2s"`
}
```

## Testing Strategy

### Unit Tests
- Rate limiter logic (token bucket refill, check)
- Budget client (request/response parsing)
- Middleware (error response formatting)

### Integration Tests
- End-to-end rate limit enforcement
- End-to-end budget enforcement
- Audit event emission
- Error response format validation

### Manual Testing
```bash
# Start dependencies
make dev-up

# Run service
make run

# Test rate limiting (send many requests quickly)
for i in {1..150}; do
  curl -X POST http://localhost:8080/v1/inference \
    -H "X-API-Key: dev-test-key" \
    -H "Content-Type: application/json" \
    -d '{"request_id":"'$(uuidgen)'","model":"gpt-4o","payload":"test"}'
done

# Should see 429 responses after rate limit exceeded
```

## OpenAPI Contract

### LimitErrorResponse Schema
Reference: `specs/006-api-router-service/contracts/api-router.openapi.yaml`

```yaml
LimitErrorResponse:
  type: object
  required:
    - error
    - code
    - limit_type
  properties:
    error:
      type: string
    code:
      type: string
      enum: [BUDGET_EXCEEDED, QUOTA_EXCEEDED, RATE_LIMIT_EXCEEDED]
    limit_type:
      type: string
      enum: [budget, quota, rate_limit]
    current_usage:
      type: number
    limit:
      type: number
    retry_after:
      type: integer
      description: Seconds until retry allowed
```

## Key Files Reference

### Existing (Phase 3)
- `internal/api/public/handler.go` - Handler pipeline (add middleware before this)
- `internal/auth/authenticator.go` - Provides org/key context
- `cmd/router/main.go` - Middleware stack registration
- `internal/config/config.go` - Configuration structure

### To Create (Phase 4)
- `internal/limiter/rate_limiter.go` - Redis token bucket limiter
- `internal/limiter/budget_client.go` - Budget service client
- `internal/api/public/middleware.go` - Rate limit and budget middleware
- `internal/usage/audit_logger.go` - Audit event emission
- `internal/telemetry/limits.go` - Limit error responses and metrics
- `test/integration/limiter_budget_test.go` - Integration tests

## Research & Design Decisions

### Rate Limiting Strategy
- **Decision**: Redis token bucket (from research.md)
- **Rationale**: Predictable sub-millisecond operations, horizontal scaling
- **Implementation**: Use existing Redis from docker-compose

### Budget Service Integration
- **Decision**: HTTP client with timeout
- **Rationale**: Budget service is separate service (may not exist yet)
- **Fallback**: Stub implementation for development/testing

### Audit Event Format
- **Decision**: Kafka topic `audit.router` (or similar)
- **Rationale**: Centralized audit pipeline
- **Fallback**: Logger when Kafka unavailable

## Known Considerations

1. **Budget Service**: May not exist yet - implement stub first, then real integration
2. **Redis Connection**: Already configured in docker-compose, reuse existing connection
3. **Error Response Format**: Must match OpenAPI spec exactly
4. **Metrics**: Add Prometheus metrics for observability
5. **Performance**: Rate limit checks should be fast (< 1ms)

## Success Criteria

Phase 4 is complete when:
- ✅ T019: Integration tests written and passing
- ✅ T020: Budget client implemented (stub OK for now)
- ✅ T021: Redis rate limiter implemented and tested
- ✅ T022: Middleware integrated into request pipeline
- ✅ T023: Audit events emitted for denials
- ✅ T024: Structured error responses match OpenAPI spec
- ✅ All tests passing
- ✅ Manual testing confirms 402/429 responses

## Next Steps After Phase 4

- **Phase 5**: User Story 3 - Intelligent routing and fallback (P2)
- **Phase 6**: User Story 4 - Usage accounting and export (P2)
- **Phase 7**: User Story 5 - Operational visibility (P3)

## Useful Commands

```bash
# Start dependencies (Redis, Kafka)
make dev-up

# Run tests
make test
go test ./test/integration/... -v

# Run service locally
make run

# Check Redis
redis-cli
> KEYS *
> GET rate_limit:org:*

# Check Kafka topics
kafka-console-consumer --bootstrap-server localhost:9092 --topic audit.router
```

## Questions to Resolve

1. **Budget Service API**: What's the actual endpoint/format? (May need to stub initially)
2. **Rate Limit Configuration**: Per-org defaults? Per-key overrides?
3. **Audit Topic Name**: Confirm topic name for audit events
4. **Retry-After Calculation**: How to calculate retry-after for rate limits?

---

**Ready to start Phase 4! Begin with T019 (integration test) to define expected behavior.**

