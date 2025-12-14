# API Router Service

Main API gateway for the AI-AAS platform.

## Overview

The API Router Service is the primary entry point for all inference requests. It handles:
- Request routing to model backends
- Authentication and authorization
- Rate limiting
- Budget enforcement
- Usage tracking

## Quick Start

```bash
# Start local development environment
make up

# Run locally
go run ./cmd/router

# Build
go build -o router ./cmd/router

# Run tests
go test ./...
make test SERVICE=api-router-service
```

## API Endpoints

### Inference

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/chat/completions` | OpenAI-compatible chat endpoint |
| POST | `/v1/completions` | OpenAI-compatible completions endpoint |

### Status

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/status/healthz` | Liveness probe |
| GET | `/v1/status/readyz` | Readiness probe |
| GET | `/v1/chat/completions/health` | Unauthenticated health check for validation tools (e.g., guidellm) |
| GET | `/v1/models` | List available models |
| GET | `/metrics` | Prometheus metrics |

## Authentication

Requests require an API key via:
- Header: `Authorization: Bearer <api-key>`
- Header: `X-API-Key: <api-key>`

## Configuration

Key environment variables:
- `HTTP_PORT` - HTTP port (default: 8080)
- `REDIS_ADDRESS` - Redis for rate limiting
- `KAFKA_BROKERS` - Kafka for usage records
- `CONFIG_SERVICE_ENDPOINT` - etcd for configuration
- `BACKEND_ENDPOINTS` - Comma-separated backend URLs
- `LOG_LEVEL` - Logging level (default: info)

## Architecture

```
cmd/router/             # Entry point
internal/
├── handlers/           # HTTP handlers
├── router/             # Request routing logic
├── auth/               # Authentication
├── ratelimit/          # Rate limiting
├── budget/             # Budget enforcement
└── middleware/         # HTTP middleware
```

## Request Flow

1. Request arrives at `/v1/chat/completions`
2. Authentication middleware validates API key
3. Rate limiter checks request limits
4. Budget service verifies spending limits
5. Router selects backend based on model/policy
6. Request forwarded to vLLM backend
7. Response streamed back to client
8. Usage record sent to Kafka

## Related Documentation

- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment requirements
- [docs/go-services/api-patterns.md](../../docs/go-services/api-patterns.md) - API conventions
- [docs/platform/routing-policies.md](../../docs/platform/routing-policies.md) - Routing configuration
