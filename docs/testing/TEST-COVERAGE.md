# Test Coverage Tracking

This document tracks test coverage across the AI-AAS platform. It serves as the centralized reference for understanding what is tested, what needs testing, and how to run tests.

## Quick Reference

### Run UC Tests Against Dev Cluster

> **Security Note**: The export command below may save the API key in your shell history.
> To avoid this, prefix with a space (` export ...`) or use `read -s` to input interactively.

```bash
cd tests/usecases

# Set required environment variables
export AI_AAS_API_ENDPOINT=https://admin.dev.otherjamesbrown.com
export AI_AAS_API_KEY=$(grep MASTER_ADMIN_API_KEY ../../secrets/env/.env | cut -d= -f2)
export AI_AAS_API_ROUTER_URL=https://api.dev.otherjamesbrown.com

# Run all UC tests
go test ./... -v -timeout 10m

# Run specific domain
go test ./... -v -run "UC_INF"
```

### Check Platform Health (CLI)

```bash
ai-aas-cli status
```

## Test Structure

Tests are organized by type:

| Location | Type | Description |
|----------|------|-------------|
| `tests/usecases/` | UC (Use Case) tests | End-to-end user journey validation |
| `services/<name>/*_test.go` | Unit tests | Service-level unit tests |
| `services/<name>/tests/` | Integration tests | Service integration tests |
| `tests/go/` | Shared Go tests | Contract, unit, integration, perf |

## UC Test Domains

| Domain | Tests | Description |
|--------|-------|-------------|
| UC_INF | 4 | Inference flow (chat completions) |
| UC_ANL | 4 | Analytics and usage tracking |
| UC_RSL | 5 | Resilience (rate limits, failover) |
| UC_USR | 7 | User management |
| UC_KEY | 4 | API key lifecycle |
| UC_ORG | 2 | Organization management |
| UC_MDL | 2 | Model information |
| UC_MLC | 5 | Model lifecycle (deploy, scale) |
| UC_RCP | 4 | Recipes |
| UC_RTG | 3 | Routing configuration |
| UC_PLH | 4 | Platform health |
| UC_BM | 8 | Benchmarking |
| UC_AUTH | 2 | Authentication |
| UC_AUD | 2 | Audit logging |
| UC_USG | 4 | Usage reporting |
| Contract_* | 8 | CLI-API contract tests |

## Test Coverage Matrix

### Core Platform Flows

| Flow | Unit Tests | Integration | UC Tests | Notes |
|------|------------|-------------|----------|-------|
| Health Check | - | - | `UC_PLH_*` | All services |
| Models Available | - | - | `UC_MDL_*` | GET /v1/models |
| Organization CRUD | `user-org-service` | - | `UC_ORG_*` | Create, read, update, delete |
| User Invite/Create | `user-org-service` | - | `UC_USR_*` | User management |
| API Key Lifecycle | `user-org-service` | - | `UC_KEY_*` | Create, validate, revoke |
| Inference Request | - | - | `UC_INF_*` | Chat completions |
| Token Usage | - | - | `UC_ANL_*` | Usage tracking |
| Rate Limiting | - | - | `UC_RSL_001` | Rate limit enforcement |
| Audit Logging | - | - | `UC_AUD_*` | Request audit trail |

### Service-Level Tests

| Service | Unit | Integration | Coverage Target |
|---------|------|-------------|-----------------|
| user-org-service | `*_test.go` | - | 80% |
| api-router-service | `*_test.go` | `test/integration/` | 80% |
| analytics-service | `*_test.go` | `test/integration/` | 80% |
| admin-api-service | `*_test.go` | - | 80% |
| web-portal | `*.test.tsx` | Playwright | 80% |

### Infrastructure Tests

| Component | Tests | Location | Notes |
|-----------|-------|----------|-------|
| Terraform | Terratest | `tests/infra/terratest/` | Infra validation |
| Kubernetes | - | - | TODO: Add k8s resource tests |
| Helm Charts | - | - | TODO: Add helm test hooks |
| CI/CD | - | `.github/workflows/` | Workflow validation |

## Running Tests

### UC Tests (Recommended)

```bash
# All UC tests
cd tests/usecases
go test ./... -v

# Specific domain
go test ./... -v -run "UC_INF"

# Contract tests only
go test ./... -v -run "Contract"

# Skip tests requiring vLLM backend
go test ./... -v -short
```

### Unit Tests

```bash
# All services
make test SERVICE=all

# Specific service
cd services/api-router-service && go test ./...
```

### CI/CD

Tests run automatically on:
- Pull requests to `main` or `develop`
- Push to `main`
- Nightly UC tests via `nightly-uc.yml`

## Adding New Tests

### Unit Tests

Add co-located with source code:
- Go: `*_test.go` next to implementation
- TypeScript: `*.test.ts` or `*.test.tsx`

### UC Tests

Add to `tests/usecases/`:
1. Follow existing patterns in domain test files
2. Use fixtures from `fixtures_test.go` for resource management
3. Name tests as `TestUC_<DOMAIN>_<NUMBER>_<Description>`
4. Include acceptance criteria subtests

Example:
```go
func TestUC_INF_001_EndToEndChatCompletion(t *testing.T) {
    skipIfNoPlatformCLI(t)
    skipIfNoVLLMBackend(t)

    t.Run("AC-01: successful request", func(t *testing.T) {
        // Test implementation
    })
}
```

### Contract Tests

Add to `tests/usecases/contract_test.go`:
1. Verify CLI can parse API responses
2. Name as `TestContract_<Entity>_<Behavior>`

## Known Issues

### Backend Model Availability

Inference tests may fail if vLLM backends are not running or models are still loading. Tests gracefully skip when backends are unavailable via `skipIfNoVLLMBackend()`.

### Playwright Authentication

Web portal Playwright tests may hang during OAuth login. Workaround: Use API-based authentication.

## Test Metrics

| Metric | Target | Current | Notes |
|--------|--------|---------|-------|
| Unit test coverage | 80% | TBD | Run `make coverage` |
| UC test pass rate | 100% | TBD | CI dashboard |

## Related Documentation

- [UC Test Infrastructure](../../tests/usecases/UC_TEST_INFRASTRUCTURE.md) - UC test framework
- [Use Case Schema](../../usecases/SCHEMA.md) - UC YAML specification

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2025-11-29 | Initial test coverage tracking document | Claude |
| 2026-01-02 | Migrated from E2E to UC test structure | Claude |
