# Admin API Service

---
last_updated: 2025-12-25
---

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
| POST | `/v1/models/{name}/rename` | Rename a model and optionally migrate cache |

#### Model Rename Endpoint

**POST `/v1/models/{name}/rename`**

Renames a model in the registry and optionally migrates its cached files in S3.

Request body:
```json
{
  "new_name": "mistral-7b-instruct-v03",
  "migrate_cache": true
}
```

Response:
```json
{
  "old_name": "mistral-7b-instruct-v0.3",
  "new_name": "mistral-7b-instruct-v03",
  "cache_migrated": true,
  "cache_size_bytes": 27000000000,
  "cache_file_count": 42
}
```

Validation:
- New name must match KServe naming regex: `[a-z]([-a-z0-9]*[a-z0-9])?`
- Model must not have active deployments
- New name must not already exist

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

## Rate Limiting

The Admin API implements traffic class differentiation for rate limiting:

- **Data Plane (User Traffic)**: Standard rate limit (configurable via `RATE_LIMIT_PER_MIN`)
- **Control Plane (Operator Traffic)**: 5x higher rate limit

Traffic is classified based on:
1. `X-Client-Type` header: Set to `operator` or `control-plane` for operator traffic
2. `User-Agent` header: Checked for `ai-model-operator` or `kubernetes-operator` patterns

This prevents operator control plane traffic (deployment status updates) from starving user API requests during high-frequency reconciliation loops.

Rate limit responses include:
- HTTP 429 status
- `X-RateLimit-Class` header indicating which traffic class was limited
- `Retry-After` header with seconds to wait

## Configuration

See `config.example.env` for all configuration options.

Key environment variables:
- `DATABASE_URL` - PostgreSQL connection string
- `API_KEY_HASH_SALT` - Salt for API key hashing
- `USER_ORG_SERVICE_URL` - URL of user-org-service
- `PORT` - HTTP port (default: 8080)
- `LOG_LEVEL` - Logging level (default: info)
- `RATE_LIMIT_PER_MIN` - Base rate limit for data plane traffic (default: 100)

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
