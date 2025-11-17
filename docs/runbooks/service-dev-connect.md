# Service Development Connectivity Runbook

**Last Updated**: 2025-01-20  
**Applies To**: Service developers connecting services to local/remote dev stacks

## Overview

This runbook explains how to configure services to connect to the local or remote development stack, run example services, and troubleshoot connectivity issues.

## Prerequisites

### Local Development
- Local dev stack running (`make up`)
- Service source code available
- Go 1.24+ or Node.js 20+ (depending on service language)

### Remote Development
- Remote workspace provisioned (`make remote-provision`)
- Remote stack running (`make remote-up`)
- SSH access to workspace

## Quick Start

### 1. Start Dev Stack

**Local:**
```bash
make up
make status JSON=true  # Verify all components are healthy
```

**Remote:**
```bash
make remote-provision WORKSPACE_NAME=dev-jab WORKSPACE_OWNER=jab
make remote-up WORKSPACE_HOST=<ip>
make remote-status WORKSPACE_HOST=<ip> JSON=true
```

### 2. Configure Service

Use the service configuration template to create your environment file:

```bash
# Copy template
cp configs/dev/service-example.env.tpl .env.local

# Edit and customize
# For local mode, ports default from .specify/local/ports.yaml
# For remote mode, update hostnames to workspace IP
```

### 3. Run Example Service

**Local:**
```bash
./scripts/dev/examples/run_sample_service.sh service-template --mode local
```

**Remote:**
```bash
./scripts/dev/examples/run_sample_service.sh service-template \
  --mode remote \
  --host <workspace-ip>
```

### 4. Run Smoke Test

```bash
./tests/dev/service_happy_path.sh --service service-template
```

## Configuration Templates

### Environment File Template

The template (`configs/dev/service-example.env.tpl`) provides placeholders for:

- **Database**: `DATABASE_DSN`, `DATABASE_MAX_IDLE_CONNS`, etc.
- **Redis**: `REDIS_ADDR`, `REDIS_PASSWORD`, `REDIS_DB`
- **NATS**: `NATS_URL`, `NATS_HTTP_ADDR`
- **MinIO**: `MINIO_ENDPOINT`, `MINIO_ACCESS_KEY`, `MINIO_SECRET_KEY`
- **Mock Inference**: `MOCK_INFERENCE_ENDPOINT`
- **Observability**: `OTEL_EXPORTER_OTLP_ENDPOINT`, etc.

### Local Mode Configuration

For local development, the template uses:
- `localhost` for all hostnames
- Ports from `.specify/local/ports.yaml` (configurable via environment variables)
- Default credentials (safe for local only)

Example local configuration:
```bash
DATABASE_DSN=postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable
REDIS_ADDR=localhost:6379
NATS_URL=nats://localhost:4222
MINIO_ENDPOINT=http://localhost:9000
MOCK_INFERENCE_ENDPOINT=http://localhost:8000
DEV_MODE=local
```

### Remote Mode Configuration

For remote workspace, update hostnames:
```bash
DATABASE_DSN=postgres://postgres:postgres@<workspace-ip>:5432/ai_aas?sslmode=disable
REDIS_ADDR=<workspace-ip>:6379
NATS_URL=nats://<workspace-ip>:4222
MINIO_ENDPOINT=http://<workspace-ip>:9000
MOCK_INFERENCE_ENDPOINT=http://<workspace-ip>:8000
DEV_MODE=remote
```

Or use the `run_sample_service.sh` script with `--host` flag (auto-configures).

## Port Mappings

Default port mappings (from `.specify/local/ports.yaml`):

| Service | Default Port | Environment Variable | Override |
|---------|-------------|---------------------|----------|
| PostgreSQL | 5432 | `POSTGRES_PORT` | Set if conflict |
| Redis | 6379 | `REDIS_PORT` | Set if conflict |
| NATS (client) | 4222 | `NATS_CLIENT_PORT` | Set if conflict |
| NATS (HTTP) | 8222 | `NATS_HTTP_PORT` | Set if conflict |
| MinIO (API) | 9000 | `MINIO_API_PORT` | Set if conflict |
| MinIO (Console) | 9001 | `MINIO_CONSOLE_PORT` | Set if conflict |
| Mock Inference | 8000 | `MOCK_INFERENCE_PORT` | Set if conflict |

To override ports, set environment variables:
```bash
export POSTGRES_PORT=5433
export REDIS_PORT=6380
make up  # Stack will use custom ports
```

## Connection Examples

### Go Service Example

```go
package main

import (
    "context"
    "database/sql"
    
    _ "github.com/lib/pq"
    "github.com/ai-aas/shared-go/config"
)

func main() {
    ctx := context.Background()
    cfg := config.MustLoad(ctx)
    
    // Connect to database
    db, err := sql.Open("postgres", cfg.Database.DSN)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()
    
    // Verify connection
    if err := db.PingContext(ctx); err != nil {
        log.Fatal(err)
    }
    
    // Use database...
}
```

### TypeScript Service Example

```typescript
import { Pool } from 'pg';
import { createClient } from 'redis';

// Database connection
const pool = new Pool({
  connectionString: process.env.DATABASE_DSN || 
    'postgres://postgres:postgres@localhost:5432/ai_aas',
});

// Redis connection
const redis = createClient({
  url: process.env.REDIS_ADDR || 'redis://localhost:6379',
});

await redis.connect();
```

## Testing Connectivity

### Verify Stack Health

**Local:**
```bash
make status JSON=true
# or
make dev-status MODE=local JSON=true
```

**Remote:**
```bash
make remote-status WORKSPACE_HOST=<ip> JSON=true
# or
make dev-status MODE=remote HOST=<ip> JSON=true
```

### Test Individual Components

**PostgreSQL:**
```bash
psql "${DATABASE_DSN}" -c "SELECT 1"
# or via Docker
docker exec -i dev-postgres psql -U postgres -d ai_aas -c "SELECT 1"
```

**Redis:**
```bash
redis-cli -h localhost -p 6379 ping
# or via Docker
docker exec -i dev-redis redis-cli ping
```

**NATS:**
```bash
curl http://localhost:8222/healthz
# or via Docker
docker exec -i dev-nats wget -q -O- http://localhost:8222/healthz
```

**MinIO:**
```bash
curl http://localhost:9000/minio/health/live
# or via Docker
docker exec -i dev-minio curl -f http://localhost:9000/minio/health/live
```

**Mock Inference:**
```bash
curl http://localhost:8000/health
curl -X POST http://localhost:8000/v1/completions \
  -H "Content-Type: application/json" \
  -d '{"prompt": "test", "max_tokens": 10}'
```

### Run Integration Smoke Test

```bash
# Full stack verification
./tests/dev/service_happy_path.sh

# With verbose output
./tests/dev/service_happy_path.sh --verbose

# Test specific service
./tests/dev/service_happy_path.sh --service service-template
```

## Troubleshooting

### Port Conflicts

**Symptom**: Service fails to start, port already in use

**Solution**:
1. Run `make diagnose` to detect conflicts
2. Override port via environment variable:
   ```bash
   export POSTGRES_PORT=5433
   make up
   ```
3. Update service configuration to use new port

### Connection Refused

**Symptom**: Service cannot connect to database/cache/messaging

**Possible Causes**:
1. Dev stack not running
2. Wrong hostname/port in configuration
3. Firewall blocking connections

**Solution**:
1. Verify stack is running: `make status`
2. Check ports: `make diagnose`
3. Verify connectivity: `nc -z localhost <port>`
4. Check service logs: `make logs COMPONENT=<service>`

### Database Connection Errors

**Symptom**: `pq: password authentication failed` or `connection refused`

**Solution**:
1. Verify database is running:
   ```bash
   docker ps | grep postgres
   make status | grep postgres
   ```
2. Check connection string format:
   ```bash
   echo $DATABASE_DSN
   # Should be: postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable
   ```
3. Test connection directly:
   ```bash
   psql "${DATABASE_DSN}" -c "SELECT version()"
   ```

### Redis Connection Errors

**Symptom**: `dial tcp: connection refused` or `NOAUTH Authentication required`

**Solution**:
1. Verify Redis is running: `make status | grep redis`
2. Check if password is required:
   ```bash
   # Default: no password
   redis-cli -h localhost -p 6379 ping
   ```
3. Update connection string if password is set

### NATS Connection Errors

**Symptom**: `nats: no servers available for connection`

**Solution**:
1. Verify NATS is running: `make status | grep nats`
2. Check NATS health: `curl http://localhost:8222/healthz`
3. Verify port: `nc -z localhost 4222`
4. Check connection URL format: `nats://localhost:4222`

### MinIO Connection Errors

**Symptom**: `dial tcp: connection refused` or `Invalid Access Key`

**Solution**:
1. Verify MinIO is running: `make status | grep minio`
2. Check MinIO health: `curl http://localhost:9000/minio/health/live`
3. Verify credentials:
   ```bash
   # Default: minioadmin / minioadmin
   echo $MINIO_ACCESS_KEY
   echo $MINIO_SECRET_KEY
   ```
4. Access MinIO console: `http://localhost:9001`

### Mock Inference Errors

**Symptom**: `connection refused` or `404 Not Found`

**Solution**:
1. Verify mock inference is running: `make status | grep mock`
2. Check health endpoint: `curl http://localhost:8000/health`
3. Test completion endpoint:
   ```bash
   curl -X POST http://localhost:8000/v1/completions \
     -H "Content-Type: application/json" \
     -d '{"prompt": "test", "max_tokens": 10}'
   ```

### Remote Workspace Connectivity

**Symptom**: Cannot connect to remote workspace services

**Solution**:
1. Verify workspace is running: `make remote-status WORKSPACE_HOST=<ip>`
2. Check SSH connectivity: `ssh root@<ip> "echo ok"`
3. Verify ports are accessible (may require SSH tunnel)
4. Check firewall rules on workspace

### Service Build Errors

**Symptom**: Service fails to build or run

**Solution**:
1. Check Go/Node version requirements
2. Verify dependencies:
   ```bash
   # Go
   go mod download
   go mod verify
   
   # Node
   npm install  # or pnpm install
   ```
3. Check for missing environment variables
4. Review service logs for specific errors

## Best Practices

### Environment File Management

1. **Never commit `.env.*` files**: Ensure `.gitignore` includes `.env.local`, `.env.linode`, `.env.*`
2. **Use templates**: Start with `configs/dev/service-example.env.tpl`
3. **Separate configs**: Use `.env.local` for local, `.env.linode` for remote
4. **Load before running**: Source environment file before starting service

### Connection String Security

1. **Local development**: Default credentials are acceptable (isolated stack)
2. **Remote workspace**: Use secrets sync (`make remote-secrets`)
3. **Production**: Never use dev credentials; use secrets management

### Service Lifecycle

1. **Start stack first**: Always start dev stack before service
2. **Verify health**: Check `make status` before connecting service
3. **Clean shutdown**: Stop service gracefully before stopping stack
4. **Reset when needed**: Use `make reset` to clear corrupted state

### Debugging

1. **Enable verbose logging**: Set `LOG_LEVEL=debug` in service environment
2. **Check component logs**: `make logs COMPONENT=<name>`
3. **Use diagnose**: `make diagnose` for port conflicts and config issues
4. **Test incrementally**: Start with health checks, then add functionality

## Example Workflow

### Complete Local Development Setup

```bash
# 1. Start local stack
make up
make status JSON=true

# 2. Sync secrets (if using GitHub secrets)
make remote-secrets

# 3. Create service environment file
cp configs/dev/service-example.env.tpl .env.local
# Edit .env.local as needed

# 4. Load environment
source .env.local

# 5. Run service
./scripts/dev/examples/run_sample_service.sh service-template --mode local

# 6. Test connectivity
./tests/dev/service_happy_path.sh --service service-template

# 7. When done
make stop
```

### Complete Remote Development Setup

```bash
# 1. Provision workspace
make remote-provision WORKSPACE_NAME=dev-jab WORKSPACE_OWNER=jab

# 2. Note workspace IP from output, then start stack
make remote-up WORKSPACE_HOST=<ip>
make remote-status WORKSPACE_HOST=<ip> JSON=true

# 3. Sync secrets
make remote-secrets

# 4. Create service environment file
cp configs/dev/service-example.env.tpl .env.linode
# Edit .env.linode with workspace IP

# 5. Run service
./scripts/dev/examples/run_sample_service.sh service-template \
  --mode remote --host <ip>

# 6. When done
make remote-destroy WORKSPACE_NAME=dev-jab WORKSPACE_OWNER=jab
```

## References

- **Endpoints & URLs**: `docs/platform/endpoints-and-urls.md` - Complete endpoint configuration guide
- **Configuration Template**: `configs/dev/service-example.env.tpl`
- **Example Runner**: `scripts/dev/examples/run_sample_service.sh`
- **Smoke Test**: `tests/dev/service_happy_path.sh`
- **Port Mappings**: `.specify/local/ports.yaml`
- **Local Stack Commands**: `make help` (see Local Dev Environment section)
- **Remote Stack Commands**: `make help` (see Dev Environment section)
- **Shared Libraries**: `shared/README.md`
- **Service Template**: `samples/service-template/README.md`

## Support

For additional help:
1. Check troubleshooting section above
2. Review service-specific documentation in `services/<service>/README.md`
3. Consult shared libraries documentation: `shared/README.md`
4. Review quickstart: `specs/002-local-dev-environment/quickstart.md`

