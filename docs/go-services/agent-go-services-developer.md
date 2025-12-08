# Go Services Developer - Document Map

---
last_updated: 2025-12-08
document_type: reference
purpose: Navigation index for go-services-developer AI agent
---

## Quick Navigation

This document provides a map of all Go services documentation to help the go-services-developer agent quickly locate relevant information.

## Your Services

You are responsible for four Go services:

| Service | Purpose | Location |
|---------|---------|----------|
| admin-api-service | Administrative API for platform management | `services/admin-api-service/` |
| analytics-service | Usage analytics and billing | `services/analytics-service/` |
| api-router-service | API gateway and request routing | `services/api-router-service/` |
| user-org-service | User and organization management | `services/user-org-service/` |

## Document Index

### Cross-Cutting Documentation (in docs/go-services/)

| Document | Purpose | Use When |
|----------|---------|----------|
| [api-patterns.md](api-patterns.md) | REST conventions, response formats, versioning | Implementing new endpoints |
| [error-handling.md](error-handling.md) | Error types, wrapping, structured errors | Handling/returning errors |
| [testing-guide.md](testing-guide.md) | Unit tests, integration tests, mocking | Writing tests |
| [database-patterns.md](database-patterns.md) | SQL patterns, migrations, connection handling | Database operations |
| [service-checklist.md](service-checklist.md) | New service creation checklist | Creating a new service |
| [makefile-customization.md](makefile-customization.md) | Makefile hooks and overrides | Customizing build process |

### Service-Specific Documentation (in services/<name>/)

| Service | README | DEPLOYMENT |
|---------|--------|------------|
| admin-api-service | [README.md](../../services/admin-api-service/README.md) | [DEPLOYMENT.md](../../services/admin-api-service/DEPLOYMENT.md) |
| analytics-service | [README.md](../../services/analytics-service/README.md) | [DEPLOYMENT.md](../../services/analytics-service/DEPLOYMENT.md) |
| api-router-service | [README.md](../../services/api-router-service/README.md) | [DEPLOYMENT.md](../../services/api-router-service/DEPLOYMENT.md) |
| user-org-service | [README.md](../../services/user-org-service/README.md) | [DEPLOYMENT.md](../../services/user-org-service/DEPLOYMENT.md) |

## Documentation Ownership

### What You Own (READ + WRITE)

1. **`docs/go-services/*.md`** - Cross-cutting patterns and guides
2. **`services/<name>/README.md`** - Service overview, API endpoints, development guide
3. **`services/<name>/DEPLOYMENT.md`** - Deployment requirements (interface with infra-ops-manager)

### What You Read Only

1. **`docs/platform/*`** - Infrastructure documentation (owned by infra-ops-manager)
2. **Helm charts** - Reference for deployment config, but don't modify

## The DEPLOYMENT.md Contract

**IMPORTANT**: The `DEPLOYMENT.md` file in each service is the interface between you and the infra-ops-manager agent.

When you change a service in ways that affect deployment, you MUST update `DEPLOYMENT.md`:
- New environment variables
- Changed health endpoints
- New dependencies (databases, Redis, Kafka, etc.)
- Changed resource requirements
- New ports

The infra-ops-manager agent reads these files when deploying services.

## Key Source-of-Truth Locations

### Code Structure

| Information | Location |
|-------------|----------|
| Service entry points | `services/<name>/cmd/` |
| Internal packages | `services/<name>/internal/` |
| Shared libraries | `pkg/` |
| API clients | `internal/api/`, `internal/registry/` |

### Configuration

| Information | Location |
|-------------|----------|
| Example configs | `services/<name>/config.example.env` |
| Helm values (reference only) | `services/<name>/deployments/helm/<name>/values.yaml` |

### Tests

| Information | Location |
|-------------|----------|
| Unit tests | `services/<name>/**/*_test.go` |
| Integration tests | `services/<name>/tests/` |
| E2E tests | `tests/e2e/` |

## Common Tasks - Where to Look

### Adding a New API Endpoint

1. Check patterns: [api-patterns.md](api-patterns.md)
2. Implement in service: `services/<name>/internal/handlers/`
3. Update README: `services/<name>/README.md`
4. Add tests: `services/<name>/**/*_test.go`

### Debugging Service Issues

1. Check service README for known issues: `services/<name>/README.md`
2. Review error handling patterns: [error-handling.md](error-handling.md)
3. Check logs (refer to `docs/platform/observability-guide.md` for log queries)

### Changing Deployment Requirements

1. Update `services/<name>/DEPLOYMENT.md`
2. Notify that Helm chart may need updating (infra-ops-manager's responsibility)

### Adding Database Migrations

1. Check patterns: [database-patterns.md](database-patterns.md)
2. Add migration: `services/<name>/migrations/`
3. Update DEPLOYMENT.md if new environment variables needed

## Quality Checklist

Before completing any task:

- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Linting passes: `golangci-lint run`
- [ ] README.md updated if API changes
- [ ] DEPLOYMENT.md updated if deployment requirements change

## Related Runbooks

| Runbook | Location | Purpose |
|---------|----------|---------|
| Service dev connect | `docs/runbooks/service-dev-connect.md` | Local development setup |

## Services Quick Reference

### admin-api-service

- **Port**: 8080
- **Health**: `/healthz`, `/readyz`
- **Metrics**: `/metrics`
- **Base path**: `/v1/`

### api-router-service

- **Port**: 8080
- **Health**: `/v1/status/healthz`, `/v1/status/readyz`
- **Metrics**: `/metrics`
- **Base path**: `/v1/`

### analytics-service

- **Port**: 8084
- **Health**: `/analytics/v1/status/healthz`, `/analytics/v1/status/readyz`
- **Metrics**: `/metrics`
- **Base path**: `/analytics/v1/`

### user-org-service

- **Port**: 8081
- **Health**: `/health`, `/ready`
- **Metrics**: `/metrics`
- **Base path**: `/v1/`
