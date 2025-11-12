# Analytics Service

The Analytics Service provides multi-tenant analytics for inference usage, ingesting events from the API Router, aggregating them in TimescaleDB, and exposing REST + CSV export interfaces for cost, reliability, and performance insights.

## Overview

This service provides:
- Usage event ingestion from RabbitMQ
- Time-series aggregation and rollups
- Org-level usage and spend visibility
- Reliability and error insights
- Finance-friendly CSV exports
- Freshness indicators and data quality checks

## Prerequisites

- Go 1.24+
- GNU Make 4.x
- Docker & Docker Compose (for local development)
- PostgreSQL 15+ with TimescaleDB extension
- RabbitMQ 3.12+ with streams support
- Redis 7+ (for freshness cache)

## Quick Start

### Local Development

1. **Start dependencies**:
   ```bash
   make dev-up
   ```
   This starts Postgres (with TimescaleDB), Redis, and RabbitMQ.

2. **Run migrations**:
   ```bash
   # Apply analytics schema migrations
   make migrate-up
   ```

3. **Build and run**:
   ```bash
   make build
   make run
   ```
   The service will start on `http://localhost:8084`.

4. **Check health**:
   ```bash
   curl http://localhost:8084/analytics/v1/status/healthz
   curl http://localhost:8084/analytics/v1/status/readyz
   ```

### Configuration

Configure via environment variables (see `internal/config/config.go` for all options):
- `SERVICE_NAME`: Service identifier (default: `analytics-service`)
- `HTTP_PORT`: HTTP server port (default: `8084`)
- `DATABASE_URL`: PostgreSQL connection string
- `REDIS_URL`: Redis connection string
- `RABBITMQ_URL`: RabbitMQ connection string
- `S3_ENDPOINT`: S3-compatible object storage endpoint
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry endpoint

## Development

### Build
```bash
make build
```

### Test
```bash
make test          # Unit tests
make integration-test # Integration tests (requires docker-compose)
```

### Lint and Format
```bash
make check  # Runs fmt, lint, security, and test
make fmt    # Format code
make lint   # Run golangci-lint
```

## Project Structure

```
services/analytics-service/
├── cmd/analytics-service/  # Main application entrypoint
├── internal/
│   ├── api/               # HTTP handlers and routing
│   ├── config/            # Configuration loading
│   ├── ingestion/         # RabbitMQ consumer for events
│   ├── aggregation/       # Rollup workers and queries
│   ├── exports/           # CSV export generation and S3 delivery
│   ├── freshness/         # Freshness tracking and cache
│   └── observability/     # OpenTelemetry and logging
├── pkg/models/            # Shared data models
├── dev/                   # Local development docker-compose
└── test/                  # Test suites
```

## Status

**Phase 1: Setup** - In Progress
- ✅ Service scaffold and directory structure
- ✅ Go module initialization
- ✅ Makefile with build/test targets
- ⏳ Docker Compose development harness
- ⏳ Build orchestration registration

**Next Steps**: 
- Phase 2: Foundational infrastructure (config, server, ingestion skeleton)
- Phase 3: User Story 1 - Org-level usage and spend visibility (MVP)
- Phase 4: User Story 2 - Reliability and error insights
- Phase 5: User Story 3 - Finance-friendly reporting

## References

- Specification: `specs/007-analytics-service/spec.md`
- Tasks: `specs/007-analytics-service/tasks.md`
- Contracts: `specs/007-analytics-service/contracts/`
- Quickstart: `specs/007-analytics-service/quickstart.md`

