# Service Interactions

## Overview

This document describes how the three core services interact with each other and with supporting infrastructure to fulfill platform functionality.

## Interaction Patterns

### Synchronous Interactions

Synchronous interactions are used for real-time operations that require immediate responses.

#### 1. API Router → User & Organization Service

**Purpose**: Authenticate API keys and check budgets

**Flow**:
```
Client Request → API Router
  ↓
API Router validates API key (HTTP call to User & Org Service)
  ↓
User & Org Service checks:
  - API key validity
  - Organization status
  - Budget availability
  ↓
Response: {valid: true/false, budget_remaining: X}
  ↓
API Router proceeds or rejects request
```

**Endpoint**: `GET /admin/v1/api-keys/{key_id}/validate`
**Method**: HTTP/REST
**Timeout**: 500ms
**Retry**: No (fail fast)
**Caching**: API key metadata cached in Redis (TTL: 5 minutes)

**Error Handling**:
- Service unavailable: Reject request (503)
- Invalid API key: Reject request (401)
- Budget exceeded: Reject request (429)

#### 2. API Router → Model Backends

**Purpose**: Route inference requests to model backends

**Flow**:
```
API Router receives inference request
  ↓
Routing engine selects backend:
  - Health check status
  - Routing policy weights
  - Load balancing
  ↓
HTTP/gRPC request to selected backend
  ↓
Backend processes inference
  ↓
Response returned to API Router
  ↓
API Router returns to client
```

**Protocol**: HTTP/gRPC (OpenAI-compatible)
**Timeout**: 30s (configurable)
**Retry**: Yes (with failover to alternate backends)
**Health Checks**: Continuous monitoring (every 10s)

**Error Handling**:
- Backend unavailable: Failover to alternate backend
- Timeout: Failover to alternate backend
- All backends unavailable: Return 503

### Asynchronous Interactions

Asynchronous interactions are used for non-critical operations that don't block the request path.

#### 1. API Router → RabbitMQ → Analytics Service

**Purpose**: Track usage events for analytics

**Flow**:
```
API Router processes inference request
  ↓
Usage event created:
  - Organization ID
  - API Key ID
  - Model
  - Tokens used
  - Latency
  - Timestamp
  ↓
Event published to RabbitMQ (fire-and-forget)
  ↓
Request continues (not blocked)
  ↓
Analytics Service consumes from RabbitMQ
  ↓
Event processed and stored in PostgreSQL
```

**Queue**: `usage_events`
**Pattern**: At-least-once delivery
**Deduplication**: Analytics Service deduplicates by request_id
**Retry**: RabbitMQ retries on failure
**Idempotency**: Events are idempotent (deduplicated)

**Error Handling**:
- RabbitMQ unavailable: Log warning, continue (graceful degradation)
- Consumer failure: RabbitMQ retries, dead-letter queue
- Duplicate events: Deduplicated by Analytics Service

#### 2. User & Organization Service → Kafka

**Purpose**: Audit event streaming

**Flow**:
```
User & Org Service processes admin operation
  ↓
Audit event created:
  - User ID
  - Action
  - Resource
  - Timestamp
  - Result
  ↓
Event published to Kafka
  ↓
Operation continues (not blocked)
  ↓
External audit systems consume from Kafka
```

**Topic**: `audit_events`
**Pattern**: Event sourcing
**Retention**: 7 years (compliance)
**Consumers**: External audit systems

### Caching Interactions

Caching is used to improve performance and reduce load on services.

#### 1. API Router ↔ Redis

**Purpose**: Rate limiting and configuration caching

**Patterns**:
- **Rate Limiting**: Increment counters per API key
- **Configuration Cache**: Cache routing policies and backend configs
- **TTL**: 5 minutes for configs, sliding window for rate limits

**Operations**:
- `INCR` for rate limit counters
- `GET/SET` for configuration cache
- `EXPIRE` for TTL management

#### 2. User & Organization Service ↔ Redis

**Purpose**: OAuth2 session caching

**Patterns**:
- **Session Storage**: OAuth2 tokens and refresh tokens
- **TTL**: Token expiration time
- **Graceful Degradation**: Falls back to database if Redis unavailable

**Operations**:
- `SET` for session storage
- `GET` for session retrieval
- `DEL` for session invalidation

#### 3. Analytics Service ↔ Redis

**Purpose**: Freshness cache

**Patterns**:
- **Freshness Indicators**: Last update time per organization
- **TTL**: 1 hour
- **Sync**: Periodically synced with database

**Operations**:
- `SET` for freshness updates
- `GET` for freshness checks
- `EXPIRE` for TTL management

## Service Dependencies

### API Router Service Dependencies

**Required**:
- Redis (rate limiting) - Service degrades without Redis
- RabbitMQ (usage events) - Service continues without RabbitMQ (logs warning)
- User & Organization Service (API key validation) - Request rejected if unavailable

**Optional**:
- Model backends (routing) - Service returns 503 if no backends available

### User & Organization Service Dependencies

**Required**:
- PostgreSQL (data store) - Service cannot start without PostgreSQL
- OAuth2 provider (authentication) - Service cannot start without OAuth2

**Optional**:
- Redis (session cache) - Graceful degradation to database
- Kafka (audit events) - Service continues without Kafka (logs warning)
- Vault (secrets) - Service continues without Vault (uses environment variables)

### Analytics Service Dependencies

**Required**:
- PostgreSQL with TimescaleDB (data store) - Service cannot start without PostgreSQL
- RabbitMQ (event ingestion) - Service cannot ingest without RabbitMQ

**Optional**:
- Redis (freshness cache) - Graceful degradation to database queries
- S3 (exports) - Export jobs fail if S3 unavailable

## Communication Protocols

### HTTP/REST

**Used For**:
- API Router → User & Organization Service (API key validation)
- Client → API Router (inference requests)
- Client → User & Organization Service (admin operations)
- Client → Analytics Service (analytics queries)

**Standards**:
- OpenAPI 3.0 specifications
- RFC7807 error semantics
- JSON request/response bodies

### Message Queues

**RabbitMQ**:
- Protocol: AMQP 0.9.1
- Pattern: At-least-once delivery
- Use Case: Usage events (API Router → Analytics)

**Kafka**:
- Protocol: Kafka protocol
- Pattern: Event sourcing
- Use Case: Audit events (User & Org Service → External)

### gRPC

**Used For**:
- API Router → Model Backends (optional, if backend supports)
- Future: Service-to-service communication

## Data Consistency Models

### Strong Consistency

**User & Organization Service**:
- Users, organizations, API keys, budgets
- Synchronous operations
- ACID transactions

### Eventual Consistency

**Analytics Service**:
- Usage aggregations
- Asynchronous processing
- Idempotent operations

### Caching Consistency

**Redis Caches**:
- TTL-based expiration
- Cache invalidation on updates
- Eventual consistency acceptable

## Error Handling Patterns

### Fail Fast

**API Router → User & Organization Service**:
- No retries
- Immediate rejection if service unavailable
- Returns 503 to client

### Retry with Backoff

**API Router → Model Backends**:
- Exponential backoff
- Maximum 3 retries
- Failover to alternate backend

### Idempotent Retries

**RabbitMQ Consumers**:
- Idempotent operations
- Deduplication by request_id
- Dead-letter queue for failures

### Graceful Degradation

**Optional Dependencies**:
- Service continues operating
- Logs warnings
- Falls back to alternative (e.g., database instead of Redis)

## Performance Considerations

### Latency Optimization

- **Caching**: Redis caches reduce database load
- **Async Processing**: Non-critical work off critical path
- **Connection Pooling**: Reuse database connections
- **Batch Processing**: Analytics aggregations in batches

### Throughput Optimization

- **Horizontal Scaling**: Stateless services scale horizontally
- **Load Balancing**: Kubernetes Service objects
- **Queue Buffering**: RabbitMQ buffers usage events
- **Parallel Processing**: Multiple consumers for queues

## Related Documentation

- [System Components](./system-components.md) - Component details
- [Architecture Overview](./architecture-overview.md) - High-level architecture
- [Data Flow](./data-flow.md) - Request and data flow patterns

