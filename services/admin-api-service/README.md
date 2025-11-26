# Admin API Service

Internal HTTP API for admin-cli to manage model registry, organizations, and routing policies.

## Quick Start

```bash
# Copy and configure environment
cp config.example.env .env
# Edit .env with your database credentials

# Run locally
go run ./cmd/admin-api

# Build
go build -o admin-api ./cmd/admin-api
```

## API Endpoints

### Model Registry
- `POST /v1/registry/models` - Register a model
- `GET /v1/registry/models` - List models
- `GET /v1/registry/models/{name}` - Get model by name
- `PATCH /v1/registry/models/{name}` - Update model
- `DELETE /v1/registry/models/{name}` - Delete model

### Organizations
- `POST /v1/organizations` - Create organization
- `GET /v1/organizations` - List organizations
- `GET /v1/organizations/{id}` - Get organization
- `PATCH /v1/organizations/{id}` - Update organization

### Routing Policies
- `POST /v1/routing/policies` - Create policy
- `GET /v1/routing/policies` - List policies
- `GET /v1/routing/policies/{id}` - Get policy
- `PATCH /v1/routing/policies/{id}` - Update policy
- `DELETE /v1/routing/policies/{id}` - Delete policy
- `POST /v1/routing/policies/{id}/activate` - Activate policy
- `POST /v1/routing/policies/{id}/deactivate` - Deactivate policy
- `GET /v1/routing/policies/sync` - Sync endpoint for api-router
- `POST /v1/routing/policies/validate` - Validate policy config

### Audit & System
- `GET /v1/audit-logs` - Query audit logs
- `GET /healthz` - Liveness probe
- `GET /readyz` - Readiness probe
- `GET /metrics` - Prometheus metrics

## Authentication

All `/v1/*` endpoints require API key authentication via `X-API-Key` header.

## Configuration

See `config.example.env` for all configuration options.

## Kubernetes Deployment

```bash
kubectl apply -f k8s/
```
