# Data Flow

## Overview

This document describes how data flows through the AIaaS platform, from client requests to analytics processing and exports.

## Request Flow

### Inference Request Flow

```
┌─────────┐
│ Client  │
└────┬────┘
     │ HTTPS
     │ POST /v1/inference
     │ X-API-Key: <key>
     ▼
┌─────────────────┐
│ NGINX Ingress   │
│ (TLS Termination)
└────┬────────────┘
     │ HTTP
     ▼
┌─────────────────────────────────┐
│      API Router Service          │
│                                  │
│  1. Extract API Key              │
│  2. Validate API Key ────────────┼───► User & Org Service
│     (sync HTTP call)             │     GET /admin/v1/api-keys/{id}/validate
│                                  │     Response: {valid, budget_remaining}
│  3. Check Rate Limits ───────────┼───► Redis
│     (INCR rate_limit:{key_id})   │     Counter check
│                                  │
│  4. Select Backend ──────────────┼───► Routing Engine
│     (health check, weights)      │     Backend selection
│                                  │
│  5. Route Request ───────────────┼───► Model Backend
│     (HTTP/gRPC)                  │     POST /v1/chat/completions
│                                  │     Response: {completion, tokens}
│                                  │
│  6. Publish Usage Event ─────────┼───► RabbitMQ
│     (async, fire-and-forget)     │     Queue: usage_events
│                                  │
│  7. Return Response              │
└────┬─────────────────────────────┘
     │ HTTP Response
     ▼
┌─────────┐
│ Client  │
└─────────┘
```

### Admin Request Flow

```
┌─────────┐
│ Admin   │
└────┬────┘
     │ HTTPS
     │ POST /admin/v1/api-keys
     │ Authorization: Bearer <token>
     ▼
┌─────────────────┐
│ NGINX Ingress   │
└────┬────────────┘
     │ HTTP
     ▼
┌─────────────────────────────────┐
│   User & Organization Service    │
│                                  │
│  1. Authenticate Token           │
│     (OAuth2 validation)          │
│                                  │
│  2. Authorize Action              │
│     (RBAC middleware)            │
│                                  │
│  3. Process Request               │
│     (create API key)             │
│                                  │
│  4. Store in PostgreSQL ─────────┼───► PostgreSQL
│     (INSERT INTO api_keys)        │     Transaction
│                                  │
│  5. Publish Audit Event ──────────┼───► Kafka
│     (async)                       │     Topic: audit_events
│                                  │
│  6. Return Response              │
└────┬─────────────────────────────┘
     │ HTTP Response
     ▼
┌─────────┐
│ Admin   │
└─────────┘
```

## Usage Event Flow

### Event Ingestion Flow

```
┌─────────────────────────────────┐
│      API Router Service         │
│                                  │
│  Usage Event Created:           │
│  {                               │
│    request_id: UUID              │
│    organization_id: UUID          │
│    api_key_id: UUID              │
│    model: string                 │
│    tokens_used: int              │
│    latency_ms: int               │
│    timestamp: timestamp          │
│  }                               │
└────┬─────────────────────────────┘
     │ AMQP
     │ Publish to queue
     ▼
┌─────────────────┐
│   RabbitMQ      │
│   Queue:        │
│   usage_events  │
└────┬────────────┘
     │ AMQP
     │ Consume
     ▼
┌─────────────────────────────────┐
│      Analytics Service          │
│                                  │
│  1. Consume Event                │
│     (RabbitMQ consumer)          │
│                                  │
│  2. Deduplicate                  │
│     (check request_id)           │
│                                  │
│  3. Store Raw Event ────────────┼───► PostgreSQL
│     (INSERT INTO usage_events)   │     TimescaleDB hypertable
│                                  │
│  4. Update Freshness ───────────┼───► Redis
│     (SET freshness:{org_id})     │     TTL: 1 hour
│                                  │
│  5. Trigger Aggregation          │
│     (async rollup worker)        │
└─────────────────────────────────┘
```

### Aggregation Flow

```
┌─────────────────────────────────┐
│      Analytics Service          │
│                                  │
│  Rollup Worker (scheduled)      │
│                                  │
│  1. Query Raw Events ───────────┼───► PostgreSQL
│     (SELECT * FROM usage_events)│     TimescaleDB
│     WHERE time > last_rollup    │
│                                  │
│  2. Aggregate by:                │
│     - Organization               │
│     - Model                      │
│     - Time bucket (hour/day)    │
│                                  │
│  3. Store Aggregations ──────────┼───► PostgreSQL
│     (INSERT INTO aggregations)   │     Continuous aggregates
│                                  │
│  4. Update Freshness ───────────┼───► Redis
│     (SET freshness:{org_id})     │
└─────────────────────────────────┘
```

## Analytics Query Flow

### Usage Query Flow

```
┌─────────┐
│ Client  │
└────┬────┘
     │ HTTPS
     │ GET /analytics/v1/usage
     │ ?org_id=...&start=...&end=...
     │ Authorization: Bearer <token>
     ▼
┌─────────────────┐
│ NGINX Ingress   │
└────┬────────────┘
     │ HTTP
     ▼
┌─────────────────────────────────┐
│      Analytics Service          │
│                                  │
│  1. Authenticate & Authorize     │
│     (RBAC middleware)            │
│                                  │
│  2. Check Freshness ────────────┼───► Redis
│     (GET freshness:{org_id})     │     Cache check
│                                  │
│  3. Query Aggregations ──────────┼───► PostgreSQL
│     (SELECT FROM aggregations)   │     TimescaleDB
│     WHERE org_id = ...           │     Continuous aggregates
│     AND time BETWEEN ...          │
│                                  │
│  4. Return Results               │
└────┬─────────────────────────────┘
     │ HTTP Response (JSON)
     ▼
┌─────────┐
│ Client  │
└─────────┘
```

### Export Flow

```
┌─────────┐
│ Client  │
└────┬────┘
     │ HTTPS
     │ POST /analytics/v1/exports
     │ {org_id, start_date, end_date}
     ▼
┌─────────────────────────────────┐
│      Analytics Service          │
│                                  │
│  1. Create Export Job            │
│     (INSERT INTO export_jobs)    │
│                                  │
│  2. Process Export (async)       │
│     (Export Worker)              │
│                                  │
│     a. Query Data ──────────────┼───► PostgreSQL
│        (SELECT FROM aggregations)│     TimescaleDB
│                                  │
│     b. Generate CSV              │
│        (format data)             │
│                                  │
│     c. Upload to S3 ─────────────┼───► S3 (Linode)
│        (PUT object)              │     Object Storage
│                                  │
│     d. Update Job Status         │
│        (UPDATE export_jobs)       │
│                                  │
│  3. Return Job ID               │
└────┬─────────────────────────────┘
     │ HTTP Response
     │ {job_id, status: "processing"}
     ▼
┌─────────┐
│ Client  │
│ (polls) │
└────┬────┘
     │ GET /analytics/v1/exports/{job_id}
     ▼
┌─────────────────────────────────┐
│      Analytics Service          │
│                                  │
│  Return Job Status:              │
│  {                               │
│    status: "completed"           │
│    download_url: "https://..."   │
│  }                               │
└─────────────────────────────────┘
```

## Data Storage Patterns

### Time-Series Data (Analytics Service)

**Storage**: PostgreSQL with TimescaleDB

**Structure**:
- **Hypertables**: Raw usage events partitioned by time
- **Continuous Aggregates**: Pre-aggregated hourly/daily rollups
- **Retention Policies**: Automatic data retention (configurable)

**Query Pattern**:
- Recent data: Query continuous aggregates (fast)
- Historical data: Query hypertables (slower but complete)
- Freshness cache: Redis for quick freshness checks

### Relational Data (User & Organization Service)

**Storage**: PostgreSQL

**Structure**:
- **Users**: User accounts and profiles
- **Organizations**: Tenant organizations
- **API Keys**: API key credentials (SHA-256 hashed)
- **Budgets**: Budget policies and tracking
- **Roles**: RBAC roles and permissions

**Query Pattern**:
- Primary key lookups (fast)
- Foreign key joins (indexed)
- Transactional consistency (ACID)

### Cached Data

**Storage**: Redis

**Patterns**:
- **Rate Limits**: Counters with sliding window
- **Sessions**: OAuth2 tokens with TTL
- **Configuration**: Routing policies with TTL
- **Freshness**: Last update timestamps with TTL

**Eviction**: TTL-based expiration

## Data Consistency Models

### Strong Consistency

**User & Organization Service**:
- Synchronous operations
- ACID transactions
- Immediate consistency

**Use Cases**:
- API key creation
- Budget updates
- User management

### Eventual Consistency

**Analytics Service**:
- Asynchronous processing
- Idempotent operations
- Eventual consistency acceptable

**Use Cases**:
- Usage aggregations
- Export generation
- Freshness indicators

### Caching Consistency

**Redis Caches**:
- TTL-based expiration
- Cache invalidation on updates
- Eventual consistency acceptable

**Use Cases**:
- Rate limit counters
- Configuration cache
- Session cache

## Data Retention

### Usage Events

- **Raw Events**: 90 days (configurable)
- **Aggregations**: 7 years (for compliance)
- **Retention Policy**: Automatic via TimescaleDB

### Audit Events

- **Kafka Topic**: 7 years retention
- **Compliance**: Required for audit trails

### Exports

- **S3 Storage**: 90 days (configurable)
- **Pre-signed URLs**: 24 hours validity

## Related Documentation

- [System Components](./system-components.md) - Component details
- [Service Interactions](./service-interactions.md) - Communication patterns
- [Architecture Overview](./architecture-overview.md) - High-level architecture

