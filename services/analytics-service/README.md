# Analytics Service

Usage analytics and billing service for the AI-AAS platform.

## Overview

The Analytics Service consumes usage data and provides:
- Usage analytics and reporting
- Billing calculations
- Usage aggregation
- Data exports

## Quick Start

```bash
# Start local development environment
make up

# Run locally
go run ./cmd/server

# Build
go build -o analytics ./cmd/server

# Run tests
go test ./...
make test SERVICE=analytics-service
```

## API Endpoints

All endpoints are under `/analytics/v1/`.

### Usage

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/analytics/v1/usage` | Get usage summary |
| GET | `/analytics/v1/usage/detailed` | Get detailed usage records |
| GET | `/analytics/v1/usage/export` | Export usage data |

### Billing

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/analytics/v1/billing/summary` | Get billing summary |
| GET | `/analytics/v1/billing/invoices` | List invoices |

### Status

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/analytics/v1/status/healthz` | Liveness probe |
| GET | `/analytics/v1/status/readyz` | Readiness probe |
| GET | `/metrics` | Prometheus metrics |

## Configuration

Key environment variables:
- `HTTP_PORT` - HTTP port (default: 8084)
- `DATABASE_URL` - PostgreSQL connection string
- `REDIS_URL` - Redis for caching
- `RABBITMQ_URL` - RabbitMQ for event ingestion
- `LOG_LEVEL` - Logging level (default: info)

## Architecture

```
cmd/server/             # Entry point
internal/
├── handlers/           # HTTP handlers
├── service/            # Business logic
├── repository/         # Database access
├── consumer/           # Message queue consumer
└── aggregator/         # Usage aggregation
```

## Data Flow

1. API Router sends usage events to RabbitMQ
2. Consumer ingests events into database
3. Aggregator processes raw events into summaries
4. API serves aggregated data for reporting

## Related Documentation

- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment requirements
- [docs/go-services/api-patterns.md](../../docs/go-services/api-patterns.md) - API conventions
- [docs/go-services/database-patterns.md](../../docs/go-services/database-patterns.md) - Database patterns
