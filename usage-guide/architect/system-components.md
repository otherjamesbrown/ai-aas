# System Components

## Overview

The AIaaS platform consists of three core microservices, each with distinct responsibilities and clear service boundaries. This document describes each component in detail.

## Core Services

### 1. API Router Service

**Purpose**: Single entrypoint for inference requests that handles authentication, routing, rate limiting, and usage tracking.

**Responsibilities**:
- Authenticate API requests using API keys
- Enforce organization budgets and rate limits
- Route requests to appropriate model backends
- Implement intelligent routing with failover
- Track usage for analytics and billing
- Provide operational visibility

**Key Components**:
- **Authentication Layer** (`internal/auth/`): API key validation and HMAC signature verification
- **Rate Limiter** (`internal/limiter/`): Budget enforcement and rate limiting using Redis
- **Routing Engine** (`internal/routing/`): Backend selection, health monitoring, and failover logic
- **Usage Tracker** (`internal/usage/`): Usage record generation and async publishing to RabbitMQ
- **Configuration Cache** (`internal/config/`): Policy and routing configuration caching with BoltDB

**Technology Stack**:
- Go 1.24+
- Redis (rate limiting and caching)
- RabbitMQ (async usage event publishing)
- BoltDB (local configuration cache)

**API Endpoints**:
- `POST /v1/inference` - Main inference endpoint (OpenAI-compatible)
- `GET /v1/status/healthz` - Health check
- `GET /v1/status/readyz` - Readiness check
- `GET /metrics` - Prometheus metrics

**Data Stores**:
- Redis: Rate limit counters, configuration cache
- RabbitMQ: Usage event queue
- BoltDB: Local configuration cache

**Dependencies**:
- User & Organization Service (for API key validation and budget checks)
- Model backends (vLLM or external providers)
- Redis (required for rate limiting)
- RabbitMQ (required for usage tracking)

---

### 2. User & Organization Service

**Purpose**: Manages tenant identity, access control, budgeting, and declarative configuration.

**Responsibilities**:
- User and organization lifecycle management
- API key generation and management
- Role-based access control (RBAC)
- Budget policy definition and enforcement
- Declarative configuration reconciliation
- OAuth2 authentication and authorization
- Audit event generation

**Key Components**:
- **Admin API** (`cmd/admin-api/`): REST endpoints for interactive administration
- **Reconciler** (`cmd/reconciler/`): Git-backed declarative configuration processor
- **Authentication** (`internal/authn/`): OAuth2 provider (Fosite) with MFA support
- **Authorization** (`internal/authz/`): Policy enforcement using OPA/Rego
- **Budget Management** (`internal/budgets/`): Budget policy evaluation and tracking
- **API Key Management** (`internal/apikeys/`): API key lifecycle and validation
- **Declarative Config** (`internal/declarative/`): Git client, diffing, and reconciliation

**Technology Stack**:
- Go 1.24+
- PostgreSQL (primary data store)
- Redis (session caching)
- Kafka (audit event streaming)
- Vault (secrets management)
- OPA (policy engine)

**API Endpoints**:
- `POST /oauth2/token` - OAuth2 token endpoint
- `POST /admin/v1/organizations` - Organization management
- `POST /admin/v1/users` - User management
- `POST /admin/v1/api-keys` - API key management
- `POST /admin/v1/budgets` - Budget management
- `GET /admin/v1/audit/export` - Audit log export

**Data Stores**:
- PostgreSQL: Users, organizations, API keys, budgets, policies
- Redis: OAuth2 session cache
- Kafka: Audit event stream

**Dependencies**:
- PostgreSQL (required)
- Redis (optional, graceful degradation)
- Kafka (for audit events)
- Vault (for secrets)

---

### 3. Analytics Service

**Purpose**: Provides multi-tenant analytics for inference usage, reliability, and cost insights.

**Responsibilities**:
- Ingest usage events from RabbitMQ
- Aggregate usage data in time-series format
- Provide org-level usage and spend visibility
- Track reliability metrics and error rates
- Generate finance-friendly CSV exports
- Maintain data freshness indicators

**Key Components**:
- **HTTP API Server** (`internal/api/`): REST endpoints for analytics queries
- **Ingestion Pipeline** (`internal/ingestion/`): RabbitMQ consumer with deduplication
- **Aggregation Layer** (`internal/aggregation/`): Rollup workers and TimescaleDB continuous aggregates
- **Storage Layer** (`internal/storage/postgres/`): Usage, reliability, and freshness repositories
- **Export System** (`internal/exports/`): CSV generation and S3 delivery
- **Freshness Cache** (`internal/freshness/`): Redis-backed cache for data freshness

**Technology Stack**:
- Go 1.24+
- PostgreSQL with TimescaleDB extension
- RabbitMQ (event ingestion)
- Redis (freshness cache)
- S3-compatible storage (Linode Object Storage)

**API Endpoints**:
- `GET /analytics/v1/usage` - Usage queries
- `GET /analytics/v1/reliability` - Reliability metrics
- `GET /analytics/v1/exports` - Export job management
- `GET /analytics/v1/status/healthz` - Health check
- `GET /analytics/v1/status/readyz` - Readiness check
- `GET /metrics` - Prometheus metrics

**Data Stores**:
- PostgreSQL (TimescaleDB): Usage events, aggregations, exports
- RabbitMQ: Usage event queue (consumed)
- Redis: Freshness cache
- S3: Export file storage

**Dependencies**:
- PostgreSQL with TimescaleDB (required)
- RabbitMQ (required for ingestion)
- Redis (optional, graceful degradation)
- S3-compatible storage (for exports)

---

## Supporting Infrastructure

### Data Stores

#### PostgreSQL
- **User & Organization Service**: Primary data store for users, orgs, API keys, budgets
- **Analytics Service**: TimescaleDB for time-series usage data and aggregations
- **Deployment**: Separate databases per service (service boundaries)

#### Redis
- **API Router Service**: Rate limiting counters, configuration cache
- **User & Organization Service**: OAuth2 session cache
- **Analytics Service**: Freshness cache
- **Deployment**: Shared Redis cluster with database separation

#### RabbitMQ
- **Purpose**: Async message queue for non-critical operations
- **Usage**: API Router → Analytics Service (usage events)
- **Pattern**: Fire-and-forget with at-least-once delivery

#### TimescaleDB
- **Purpose**: Time-series database extension for PostgreSQL
- **Usage**: Analytics Service for usage aggregations
- **Features**: Continuous aggregates, hypertables, retention policies

### Message Queues

#### RabbitMQ Streams
- **Purpose**: Durable event streaming for usage events
- **Producers**: API Router Service
- **Consumers**: Analytics Service
- **Pattern**: At-least-once delivery with deduplication

#### Kafka
- **Purpose**: Audit event streaming
- **Producers**: User & Organization Service
- **Consumers**: External audit systems
- **Pattern**: Event sourcing for compliance

### Object Storage

#### Linode Object Storage (S3-compatible)
- **Purpose**: Export file storage
- **Usage**: Analytics Service exports CSV files
- **Access**: Pre-signed URLs for secure access

---

## Service Boundaries

### Clear Separation of Concerns

Each service maintains:
- **Independent database**: No shared databases across services
- **Own API surface**: Clear API boundaries
- **Distinct responsibilities**: Single responsibility per service
- **Independent deployment**: Can be deployed and scaled independently

### Communication Patterns

- **Synchronous**: HTTP/REST for real-time operations (API key validation, budget checks)
- **Asynchronous**: RabbitMQ/Kafka for non-critical operations (usage events, audit logs)
- **Caching**: Redis for performance optimization (rate limits, sessions, freshness)

### Data Consistency

- **Strong Consistency**: User & Organization Service (users, API keys, budgets)
- **Eventual Consistency**: Analytics Service (usage aggregations)
- **Idempotency**: All async operations are idempotent

---

## Scalability Considerations

### Horizontal Scaling
- All services are stateless and can scale horizontally
- Load balancing via Kubernetes Service objects
- Database connection pooling per service

### Performance Targets
- **Inference TTFB**: P50 <100ms, P95 <300ms
- **Management API**: P99 <500ms
- **Analytics Queries**: Sub-second for common queries

### Resource Requirements
- **API Router Service**: CPU-intensive (routing logic)
- **User & Organization Service**: Database-intensive (queries)
- **Analytics Service**: Database-intensive (aggregations)

---

## Related Documentation

- [Architecture Overview](./architecture-overview.md) - High-level architecture
- [Service Interactions](./service-interactions.md) - How services communicate
- [Data Flow](./data-flow.md) - Request and data flow patterns

