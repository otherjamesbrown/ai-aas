# User & Organization Service

User and organization management for the AI-AAS platform.

## Overview

The User-Org Service is the source of truth for:
- User accounts
- Organizations
- API keys
- Authentication and authorization

## Quick Start

```bash
# Start local development environment
make up

# Run locally
go run ./cmd/server

# Build
go build -o user-org-service ./cmd/server

# Run tests
go test ./...
make test SERVICE=user-org-service
```

## API Endpoints

All endpoints are versioned under `/v1/`.

### Users

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/users` | Create user |
| GET | `/v1/users` | List users |
| GET | `/v1/users/{id}` | Get user |
| PATCH | `/v1/users/{id}` | Update user |
| DELETE | `/v1/users/{id}` | Delete user |

### Organizations

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/organizations` | Create organization |
| GET | `/v1/organizations` | List organizations |
| GET | `/v1/organizations/{id}` | Get organization |
| PATCH | `/v1/organizations/{id}` | Update organization |
| DELETE | `/v1/organizations/{id}` | Delete organization |

### API Keys

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/apikeys` | Create API key |
| GET | `/v1/apikeys` | List API keys |
| GET | `/v1/apikeys/{id}` | Get API key |
| DELETE | `/v1/apikeys/{id}` | Revoke API key |
| POST | `/v1/apikeys/validate` | Validate API key |

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/v1/auth/login` | User login |
| POST | `/v1/auth/logout` | User logout |
| POST | `/v1/auth/refresh` | Refresh token |

### Status

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/health` | Liveness probe |
| GET | `/ready` | Readiness probe |
| GET | `/metrics` | Prometheus metrics |

## Configuration

Key environment variables:
- `HTTP_PORT` - HTTP port (default: 8081)
- `DATABASE_URL` - PostgreSQL connection string
- `JWT_SECRET` - Secret for JWT signing
- `API_KEY_HASH_SALT` - Salt for API key hashing
- `LOG_LEVEL` - Logging level (default: info)

## Architecture

```
cmd/server/             # Entry point
internal/
├── handlers/           # HTTP handlers
├── service/            # Business logic
├── repository/         # Database access
├── auth/               # Authentication logic
└── middleware/         # HTTP middleware
migrations/             # Database migrations
```

## Related Documentation

- [DEPLOYMENT.md](DEPLOYMENT.md) - Deployment requirements
- [docs/go-services/api-patterns.md](../../docs/go-services/api-patterns.md) - API conventions
- [docs/go-services/database-patterns.md](../../docs/go-services/database-patterns.md) - Database patterns
