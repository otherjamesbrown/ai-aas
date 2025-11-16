# Local Development Setup Checklist

This document outlines all components, tools, keys, and configuration needed to develop, build, and test the AI-AAS platform locally.

## Required Core Tools

### 1. Go Development Environment
- **Go Version**: `1.24.6` (see `configs/tool-versions.mk`)
- **Toolchain**: `go1.24.10`
- **Installation**:
  - macOS: `brew install go`
  - Linux: Download from https://go.dev/dl/ or use package manager
- **Verify**: `go version` should show `go1.24.6` or compatible

### 2. Git
- **Required**: Git for version control
- **Installation**:
  - macOS: `xcode-select --install` (Command Line Tools)
  - Linux: `sudo apt install git` (or equivalent)

### 3. GNU Make
- **Required**: Version 4.x or later
- **Installation**:
  - macOS: `brew install make`
  - Linux: `sudo apt install build-essential`

### 4. Docker & Docker Compose
- **Required**: For local database and service dependencies
- **Purpose**: 
  - Local PostgreSQL databases
  - Redis, Kafka, RabbitMQ for services
  - Integration testing with testcontainers
- **Installation**:
  - macOS: Docker Desktop from https://www.docker.com/products/docker-desktop
  - Linux: https://docs.docker.com/engine/install/

### 5. GitHub CLI (gh)
- **Required**: For remote CI triggering (`make ci-remote`)
- **Installation**:
  - macOS: `brew install gh`
  - Linux: https://cli.github.com/manual/install_linux
- **Authentication**: `gh auth login` (requires scopes: `repo`, `read:actions`, `workflow`)

## Required Node.js Environment (for TypeScript/Web Components)

### 6. Node.js
- **Version**: `>=20` (see `web/portal/package.json` and `shared/ts/package.json`)
- **Installation**: https://nodejs.org/
- **Verify**: `node --version`

### 7. pnpm
- **Required**: Package manager for TypeScript workspaces
- **Installation**: `npm install -g pnpm` or via package manager
- **Purpose**: Manages dependencies for `web/portal` and `shared/ts`
- **Verify**: `pnpm --version`

### 8. Playwright Browsers
- **Required**: For end-to-end UI testing of the web portal
- **Installation**: After installing TypeScript dependencies, run:
  ```bash
  cd web/portal
  pnpm exec playwright install --with-deps
  ```
- **Purpose**: Installs Chromium, Firefox, and WebKit browsers for E2E tests
- **Verify**: Run `pnpm exec playwright --version` or check that `~/.cache/ms-playwright` exists

## Optional but Recommended Tools

### 9. act (Local GitHub Actions)
- **Version**: `0.2.61` (see `configs/tool-versions.mk`)
- **Purpose**: Run GitHub Actions workflows locally
- **Installation**:
  - macOS: `brew install act`
  - Linux: https://github.com/nektos/act/releases

### 10. AWS CLI or MinIO Client
- **AWS CLI Version**: `2.17.0`
- **MinIO Client**: For S3-compatible storage (metrics uploads)
- **Purpose**: Upload build metrics to S3-compatible storage
- **Installation**:
  - macOS: `brew install awscli` or `brew install minio/stable/mc`
  - Linux: https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html

### 11. Development Database Tools
- **golang-migrate** CLI: For database migrations
- **psql**: PostgreSQL client (usually comes with PostgreSQL)
- **Purpose**: Database schema management and migrations

### 12. Additional Development Tools
- **buf** CLI: For protobuf contract validation (API Router Service)
- **vegeta**: Load testing tool (API Router Service)
- **sqlc**: Generate typed DB accessors (some services)
- **goose**: Database migration tool (alternative to golang-migrate)

## Environment Variables & Configuration

### Database Connection Strings

**For Migrations** (`configs/migrate.example.env`):
```bash
# Operational database (PostgreSQL)
DB_URL=postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable

# Analytics warehouse (PostgreSQL-compatible)
ANALYTICS_URL=postgres://analytics:analytics@localhost:6432/ai_aas_warehouse?sslmode=disable

# Migration HMAC key (32-byte secret)
MIGRATION_EMAIL_HASH_KEY=replace-with-32-byte-secret
```

**For Services**:
```bash
# User-Org Service
USER_ORG_DATABASE_URL=postgres://postgres:postgres@localhost:5432/user_org?sslmode=disable

# Analytics Service
DATABASE_URL=postgres://postgres:postgres@localhost:5432/analytics?sslmode=disable
```

### Observability/Telemetry

```bash
# OpenTelemetry endpoint
OTEL_EXPORTER_OTLP_ENDPOINT=https://otel.dev.ai-aas.internal

# OpenTelemetry headers (API key)
OTEL_EXPORTER_OTLP_HEADERS="x-api-key=<token>"

# Optional: Store token in ~/.config/ai-aas/otel.token
```

### Infrastructure Access (for remote operations)

```bash
# Linode API token (for remote workspace provisioning)
LINODE_TOKEN=<your-linode-api-token>

# For metrics uploads (optional)
METRICS_BUCKET=ai-aas-build-metrics
METRICS_ENDPOINT=  # Leave empty for AWS S3
```

### Service-Specific Environment Variables

**API Router Service**:
```bash
SERVICE_NAME=api-router-service
HTTP_PORT=8080
REDIS_ADDR=localhost:6379
KAFKA_BROKERS=localhost:9092
CONFIG_SERVICE_ENDPOINT=http://localhost:8080
```

**Analytics Service**:
```bash
SERVICE_NAME=analytics-service
HTTP_PORT=8084
DATABASE_URL=postgres://postgres:postgres@localhost:5432/analytics?sslmode=disable
REDIS_URL=redis://localhost:6379
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
```

**User-Org Service**:
```bash
SERVICE_NAME=user-org-service
HTTP_PORT=8081
USER_ORG_DATABASE_URL=postgres://postgres:postgres@localhost:5432/user_org?sslmode=disable
```

## Setup Steps

### 1. Clone Repository
```bash
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas
```

### 2. Run Bootstrap Script
The bootstrap script validates prerequisites and initializes the workspace:
```bash
./scripts/setup/bootstrap.sh
```

Or check prerequisites without installing:
```bash
./scripts/setup/bootstrap.sh --check-only
```

### 3. Install TypeScript Dependencies
```bash
# Install shared TypeScript libraries
cd shared/ts && pnpm install && cd ../..

# Install web portal dependencies
cd web/portal && pnpm install && cd ../..

# Or use Make targets
make shared-ts-install
```

### 4. Install Playwright Browsers
```bash
# Navigate to web portal directory
cd web/portal

# Install Playwright browsers (Chromium, Firefox, WebKit)
# This installs system dependencies and browser binaries
pnpm exec playwright install --with-deps

# Verify installation
pnpm exec playwright --version
```

### 5. Initialize Go Workspace
```bash
go work sync
```

### 6. Set Up Environment Files

**For Build/Test**:
```bash
cp configs/build.env.example configs/build.env
# Edit as needed for your environment
```

**For Database Migrations**:
```bash
cp configs/migrate.example.env migrate.env
# Update with your database connection strings
```

### 7. Start Local Databases (if needed)
```bash
# Services may have docker-compose files for local development
# Example for services with dev dependencies:
cd services/api-router-service
make dev-up  # Starts Redis, Kafka, mock backends

cd services/analytics-service
make dev-up  # Starts Postgres (TimescaleDB), Redis, RabbitMQ
```

### 8. Run Database Migrations (if needed)
```bash
# Set environment variables
export DB_URL=postgres://postgres:postgres@localhost:5432/ai_aas?sslmode=disable
export ANALYTICS_URL=postgres://analytics:analytics@localhost:6432/ai_aas_warehouse?sslmode=disable

# Check migration status
make db-migrate-status

# Apply migrations (see scripts/db/apply.sh for options)
scripts/db/apply.sh --env local --component operational --yes
```

## Verification Steps

### 1. Check All Tools
```bash
./scripts/setup/bootstrap.sh --check-only
```

### 2. Verify Go Setup
```bash
go version
make version  # Shows all pinned tool versions
```

### 3. Build Shared Libraries
```bash
make shared-build  # Builds Go and TypeScript shared libraries
```

### 4. Test Shared Libraries
```bash
make shared-test   # Tests Go and TypeScript shared libraries
```

### 5. Build a Service
```bash
make build SERVICE=hello-service
```

### 6. Run Tests
```bash
make test SERVICE=hello-service
make check SERVICE=hello-service  # Format, lint, security, and test
```

### 7. Verify TypeScript Setup
```bash
cd web/portal
pnpm build
pnpm test
```

### 8. Verify Playwright Setup
```bash
cd web/portal
# Check Playwright version
pnpm exec playwright --version

# Run E2E tests (requires dev server running)
pnpm test:e2e
# Or run with UI mode
pnpm test:e2e:ui
```

## Common Development Workflows

### Building a Service
```bash
make build SERVICE=<service-name>
# or from service directory:
cd services/<service-name>
make build
```

### Running Tests
```bash
make test SERVICE=<service-name>
# or
make check SERVICE=<service-name>  # Includes formatting, linting, security scans
```

### Running Local CI
```bash
make ci-local  # Uses `act` to run GitHub Actions locally
```

### Triggering Remote CI
```bash
make ci-remote SERVICE=<service-name> REF=<branch-name>
```

### Generating a New Service
```bash
make service-new NAME=<service-name>
```

## Port Requirements

Ensure these ports are available (or configure alternatives):
- `5432`: PostgreSQL (operational database)
- `6432`: PostgreSQL (analytics warehouse)
- `6379`: Redis
- `8080`: API Router Service
- `8081`: User-Org Service
- `8084`: Analytics Service
- `9092`: Kafka
- `5672`: RabbitMQ

## Troubleshooting

### Missing Tools
Run the bootstrap script to check what's missing:
```bash
./scripts/setup/bootstrap.sh --check-only
```

### Go Module Issues
```bash
go work sync
go mod download  # In each service directory
```

### TypeScript/Package Issues
```bash
# Clean and reinstall
rm -rf node_modules pnpm-lock.yaml
cd shared/ts && pnpm install
cd ../../web/portal && pnpm install
```

### Database Connection Issues
- Verify Docker containers are running
- Check connection strings in environment variables
- Ensure PostgreSQL is accessible at configured host/port

### GitHub CLI Authentication
```bash
gh auth login
# Select: GitHub.com
# Select: HTTPS or SSH
# Select scopes: repo, read:actions, workflow
```

## Next Steps

1. Review the [Quickstart Guide](specs/000-project-setup/quickstart.md)
2. Check [Troubleshooting Guide](docs/troubleshooting/)
3. Explore service-specific READMEs in `services/`
4. Review [Contributing Guidelines](CONTRIBUTING.md)

## Summary Checklist

- [ ] Go 1.24.6+ installed
- [ ] Git installed
- [ ] GNU Make 4.x+ installed
- [ ] Docker & Docker Compose installed
- [ ] GitHub CLI installed and authenticated
- [ ] Node.js 20+ installed
- [ ] pnpm installed
- [ ] Repository cloned
- [ ] Bootstrap script run successfully
- [ ] Go workspace synced (`go work sync`)
- [ ] TypeScript dependencies installed (`make shared-ts-install`)
- [ ] Playwright browsers installed (`cd web/portal && pnpm exec playwright install --with-deps`)
- [ ] Environment files created (build.env, migrate.env if needed)
- [ ] Docker containers started (if using local databases)
- [ ] Verified with `make check` or `make build SERVICE=<name>`
