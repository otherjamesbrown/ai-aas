# Quickstart: Admin API Service

## Prerequisites

- Go 1.21+
- PostgreSQL 14+ (or Docker)
- kubectl configured for cluster access

## Local Development

### 1. Start Database

```bash
# Using Docker
docker run -d --name admin-api-db \
  -e POSTGRES_USER=admin \
  -e POSTGRES_PASSWORD=admin \
  -e POSTGRES_DB=aiaas \
  -p 5432:5432 \
  postgres:14

# Run migrations
make db-migrate
```

### 2. Configure Environment

```bash
cd services/admin-api-service
cp config.example.env .env

# Edit .env with your settings:
# DATABASE_URL=postgres://admin:admin@localhost:5432/aiaas?sslmode=disable
# PORT=8080
# LOG_LEVEL=debug
```

### 3. Run Service

```bash
# From repo root
make run SERVICE=admin-api-service

# Or directly
cd services/admin-api-service
go run cmd/admin-api/main.go
```

### 4. Test Endpoints

```bash
# Health check (no auth)
curl http://localhost:8080/healthz

# Register a model (requires API key)
curl -X POST http://localhost:8080/v1/registry/models \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model_name": "test-model",
    "deployment_endpoint": "test-model.development.svc.cluster.local:80",
    "deployment_environment": "development",
    "deployment_namespace": "development",
    "deployment_status": "ready"
  }'

# List models
curl http://localhost:8080/v1/registry/models?environment=development \
  -H "Authorization: Bearer YOUR_API_KEY"
```

## Kubernetes Deployment

### 1. Create API Key Secret

```bash
# Generate API key
API_KEY=$(openssl rand -base64 32)

# Create secret
kubectl create secret generic admin-api-keys \
  --from-literal=admin-key=$API_KEY \
  -n development
```

### 2. Deploy Service

```bash
# Apply manifests
kubectl apply -f services/admin-api-service/k8s/ -n development

# Verify deployment
kubectl get pods -l app=admin-api-service -n development
```

### 3. Access Service

```bash
# Port forward for local access
kubectl port-forward svc/admin-api-service 8080:8080 -n development

# Test
curl http://localhost:8080/healthz
```

## Running Tests

```bash
# Unit tests
make test SERVICE=admin-api-service

# Integration tests (requires Docker for testcontainers)
make test-integration SERVICE=admin-api-service
```

## API Reference

See [contracts/openapi.yaml](./contracts/openapi.yaml) for full API specification.

### Key Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/healthz` | GET | Health check (no auth) |
| `/metrics` | GET | Prometheus metrics (no auth) |
| `/v1/registry/models` | POST | Register/update model |
| `/v1/registry/models` | GET | List models |
| `/v1/organizations` | POST | Create organization |
| `/v1/organizations` | GET | List organizations |
| `/v1/routing/policies` | POST | Create routing policy |
| `/v1/routing/policies` | GET | List policies |
| `/v1/routing/policies/sync` | GET | Sync endpoint for api-router |
| `/v1/audit-logs` | GET | Query audit logs |

## Troubleshooting

### Database Connection Failed

```bash
# Check database is running
docker ps | grep postgres

# Check connection string
echo $DATABASE_URL
```

### Authentication Failed

```bash
# Verify API key is set
kubectl get secret admin-api-keys -o yaml

# Check logs
kubectl logs -l app=admin-api-service -n development
```

### Service Not Responding

```bash
# Check pod status
kubectl get pods -l app=admin-api-service -n development

# Check events
kubectl describe pod -l app=admin-api-service -n development
```

