# Admin API Service

Internal HTTP API for platform administration.

## Overview

The Admin API Service provides administrative endpoints for the AI-AAS platform. It is used by the `admin-cli` to manage:
- Model registry
- Organizations
- Routing policies
- Audit logs

## Quick Start

```bash
# Copy and configure environment
cp config.example.env .env
# Edit .env with your database credentials

# Run locally
go run ./cmd/admin-api

# Build
go build -o admin-api ./cmd/admin-api

# Run tests
go test ./...
```

## API Endpoints

All endpoints are versioned under `/v1/` and require API key authentication via `X-API-Key` header.

### Model Registry

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/registry/models` | Register a model |
| GET | `/v1/registry/models` | List models |
| GET | `/v1/registry/models/{name}` | Get model by name |
| PATCH | `/v1/registry/models/{name}` | Update model |
| DELETE | `/v1/registry/models/{name}` | Delete model |

### Organizations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/organizations` | Create organization |
| GET | `/v1/organizations` | List organizations |
| GET | `/v1/organizations/{id}` | Get organization |
| PATCH | `/v1/organizations/{id}` | Update organization |

### Routing Policies

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/routing/policies` | Create policy |
| GET | `/v1/routing/policies` | List policies |
| GET | `/v1/routing/policies/{id}` | Get policy |
| PATCH | `/v1/routing/policies/{id}` | Update policy |
| DELETE | `/v1/routing/policies/{id}` | Delete policy |
| POST | `/v1/routing/policies/{id}/activate` | Activate policy |
| POST | `/v1/routing/policies/{id}/deactivate` | Deactivate policy |
| GET | `/v1/routing/policies/sync` | Sync endpoint for api-router |
| POST | `/v1/routing/policies/validate` | Validate policy config |

### System

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/v1/audit-logs` | Query audit logs |
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | Readiness probe |
| GET | `/metrics` | Prometheus metrics |

## Configuration

See `config.example.env` for all configuration options.

Key environment variables:
- `DATABASE_URL` - PostgreSQL connection string
- `API_KEY_HASH_SALT` - Salt for API key hashing
- `USER_ORG_SERVICE_URL` - URL of user-org-service
- `PORT` - HTTP port (default: 8080)
- `LOG_LEVEL` - Logging level (default: info)

## Architecture

```
cmd/admin-api/          # Entry point
internal/
├── handlers/           # HTTP handlers
├── service/            # Business logic
├── repository/         # Database access
└── middleware/         # HTTP middleware
```

## Related Documentation

- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment requirements
- [docs/go-services/api-patterns.md](../../docs/go-services/api-patterns.md) - API conventions
