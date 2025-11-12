# API Router Service

The API Router Service provides a single inference entrypoint that authenticates requests, enforces organization budgets and quotas, selects the appropriate model backend, and returns responses reliably.

## Overview

This service routes inference requests to model backends while providing:
- Authentication and authorization via API keys
- Budget and quota enforcement
- Intelligent routing with failover
- Usage tracking and accounting
- Operational visibility

## Prerequisites

- Go 1.24+
- GNU Make 4.x
- Docker & Docker Compose (for local development)
- `buf` CLI (for contract validation)

## Quick Start

### Local Development

1. **Install dependencies**:
   ```bash
   go mod download
   ```

2. **Start dependencies**:
   ```bash
   make dev-up
   ```
   This starts Redis, Kafka, and mock backend services in Docker.

3. **Build and run**:
   ```bash
   make build
   make run
   ```
   The service will start on `http://localhost:8080`.

4. **Check health**:
   ```bash
   curl http://localhost:8080/v1/status/healthz
   curl http://localhost:8080/v1/status/readyz
   ```

5. **Test the service**:
   ```bash
   # Run smoke tests
   ./scripts/smoke.sh
   
   # Or send a test request
   curl -X POST http://localhost:8080/v1/inference \
     -H 'X-API-Key: dev-test-key' \
     -H 'Content-Type: application/json' \
     -d '{
       "request_id": "550e8400-e29b-41d4-a716-446655440000",
       "model": "gpt-4o",
       "payload": "Hello, world!"
     }'
   ```

### Configuration

Copy the sample configuration files:
```bash
cp configs/router.sample.yaml configs/router.yaml
cp configs/policies.sample.yaml configs/policies.yaml
```

Configure via environment variables (see `internal/config/config.go` for all options):
- `SERVICE_NAME`: Service identifier (default: `api-router-service`)
- `HTTP_PORT`: HTTP server port (default: `8080`)
- `REDIS_ADDR`: Redis address for rate limiting
- `KAFKA_BROKERS`: Kafka broker addresses
- `CONFIG_SERVICE_ENDPOINT`: Config Service endpoint
- `OTEL_EXPORTER_OTLP_ENDPOINT`: OpenTelemetry endpoint

## Development

### Build
```bash
make build
```

### Test
```bash
make test          # Unit tests
make contract-test # Contract validation
make integration-test # Integration tests (requires docker-compose)
```

### Lint and Format
```bash
make check  # Runs fmt, lint, security, and test
make fmt    # Format code
make lint   # Run golangci-lint
```

### Contracts
```bash
make contracts  # Validate and generate contracts
```

### Load Testing
```bash
# Run load tests (requires vegeta)
./scripts/loadtest.sh baseline
./scripts/loadtest.sh peak
./scripts/loadtest.sh all
```

### Smoke Testing
```bash
# Run comprehensive smoke tests
./scripts/smoke.sh

# Custom options
./scripts/smoke.sh --url http://localhost:8080 --verbose
```

## Troubleshooting

### Service Won't Start

- **Port already in use**: Check if port 8080 is available:
  ```bash
  lsof -i :8080
  ```
- **Dependencies not running**: Ensure Docker services are up:
  ```bash
  make dev-up
  docker compose -f dev/docker-compose.yml ps
  ```

### Health Checks Fail

- **Service not responding**: Check service logs for errors
- **Components showing as degraded**: Verify dependencies:
  ```bash
  redis-cli -h localhost -p 6379 ping
  docker compose -f dev/docker-compose.yml exec kafka kafka-broker-api-versions --bootstrap-server localhost:9092
  ```

### Authentication Errors (401/403)

- **Missing API key**: Ensure `X-API-Key` header is included
- **Invalid API key**: Verify API key format matches expected format
- **Service not configured**: Check authentication configuration

### Rate Limiting Not Working

- **Redis unavailable**: Rate limiting requires Redis. Check:
  ```bash
  redis-cli -h localhost -p 6379 ping
  ```
- **Service degraded**: Service will operate without rate limiting if Redis is unavailable

For more detailed troubleshooting, see `docs/quickstart-validation.md`.

## Project Structure

```
services/api-router-service/
├── cmd/router/          # Main application entrypoint
├── internal/
│   ├── auth/           # Authentication (API keys, HMAC)
│   ├── config/         # Configuration loading and caching
│   ├── limiter/        # Rate limiting and budget enforcement
│   ├── routing/        # Backend selection and routing logic
│   ├── telemetry/      # OpenTelemetry and logging
│   ├── usage/          # Usage tracking and export
│   └── admin/          # Admin API endpoints
├── pkg/contracts/      # Generated contract types
├── configs/            # Configuration files
├── dev/                # Local development docker-compose
├── test/               # Test suites
└── deployments/        # Helm charts and deployment configs
```

## Status

**Phase 1 & 2 Complete**: Foundation infrastructure is in place:
- ✅ Service scaffold and directory structure
- ✅ Go module initialization
- ✅ Makefile with build/test targets
- ✅ Sample configuration files
- ✅ Docker Compose development harness
- ✅ GitHub Actions CI/CD workflow
- ✅ Configuration loader with BoltDB cache
- ✅ Telemetry bootstrap (OpenTelemetry + zap)
- ✅ HTTP server with middleware and health endpoints
- ✅ Contract generation workflow

**Next Steps**: Implement user stories (Phase 3+):
- User Story 1: Route authenticated inference requests
- User Story 2: Enforce budgets and safe usage
- User Story 3: Intelligent routing and fallback
- User Story 4: Accurate usage accounting
- User Story 5: Operational visibility

## References

- Specification: `specs/006-api-router-service/spec.md`
- Tasks: `specs/006-api-router-service/tasks.md`
- Contracts: `specs/006-api-router-service/contracts/`

