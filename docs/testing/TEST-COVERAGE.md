# Test Coverage Tracking

This document tracks test coverage across the AI-AAS platform. It serves as the centralized reference for understanding what is tested, what needs testing, and how to run tests.

## Quick Reference

### Run Smoke Tests Against Dev Cluster

```bash
cd tests/e2e
export ADMIN_API_KEY=$(grep MASTER_ADMIN_API_KEY ../../secrets/env/.env | cut -d= -f2)
go test -v ./suites -run TestSmokeEndToEnd -timeout 10m
```

### Check Platform Health (CLI)

```bash
ai-aas-cli status
```

## Test Coverage Matrix

### Core Platform Flows

| Flow | Unit Tests | Integration | E2E | Smoke | Notes |
|------|------------|-------------|-----|-------|-------|
| Health Check | - | - | `resilience_test.go` | `smoke_test.go` | All services |
| Models Available | - | - | - | `smoke_test.go` | GET /v1/models |
| Organization CRUD | `user-org-service` | - | `happy_path_test.go` | `smoke_test.go` | Create, read, update, delete |
| User Invite/Create | `user-org-service` | - | `happy_path_test.go` | `smoke_test.go` | Invite flow |
| API Key Lifecycle | `user-org-service` | - | `happy_path_test.go` | `smoke_test.go` | Create, validate, revoke |
| Inference Request | - | - | `happy_path_test.go` | `smoke_test.go` | Chat completions |
| Token Usage | - | - | - | `smoke_test.go` | Verify usage tracked |
| Budget Enforcement | - | - | `budget_test.go` | - | Limits and alerts |
| Audit Logging | - | - | `audit_test.go` | - | Request audit trail |
| RBAC/Auth | `server_test.go` | - | `auth_test.go` | - | Role-based access |

### Service-Level Tests

| Service | Unit | Integration | E2E | Coverage Target |
|---------|------|-------------|-----|-----------------|
| user-org-service | `*_test.go` | - | `cmd/e2e-test/` | 80% |
| api-router-service | `*_test.go` | `test/integration/` | - | 80% |
| analytics-service | `*_test.go` | `test/integration/` | - | 80% |
| admin-api-service | `*_test.go` | - | - | 80% |
| web-portal | `*.test.tsx` | - | Playwright | 80% |

### Infrastructure Tests

| Component | Tests | Location | Notes |
|-----------|-------|----------|-------|
| Terraform | Terratest | `tests/infra/terratest/` | Infra validation |
| Kubernetes | - | - | TODO: Add k8s resource tests |
| Helm Charts | - | - | TODO: Add helm test hooks |
| CI/CD | - | `.github/workflows/` | Workflow validation |

## Test Environments

### Local Development

```bash
# Start services locally
make dev

# Run unit tests
make test SERVICE=all

# Run service-specific E2E
cd services/user-org-service && make e2e-test-local
```

### Development Cluster (Remote)

```bash
# Via public internet
cd tests/e2e
export ADMIN_API_KEY=your-key
make test-dev-internet

# Via port-forward
make test-dev-remote
```

### CI/CD

Tests run automatically on:
- Pull requests to `main` or `develop`
- Push to `main`

## Smoke Test Checklist

The smoke test (`TestSmokeEndToEnd`) verifies:

- [ ] API Router health check passes
- [ ] Models endpoint returns available models
- [ ] Organization can be created
- [ ] User can be created in organization
- [ ] API key can be created
- [ ] Inference request completes (if backend available)
- [ ] Token usage is tracked (if analytics available)

## Adding New Tests

### Unit Tests

Add co-located with source code:
- Go: `*_test.go` next to implementation
- TypeScript: `*.test.ts` or `*.test.tsx`

### E2E Tests

Add to `tests/e2e/suites/`:
1. Follow existing patterns in `happy_path_test.go`
2. Use fixtures for resource management
3. Ensure cleanup in `defer ctx.Cleanup()`

### Smoke Tests

Add to `tests/e2e/suites/smoke_test.go`:
1. Should be fast (< 1 minute)
2. Should be read-only where possible
3. Should skip gracefully if dependencies unavailable

## Known Issues

### Playwright Authentication Hang

Playwright E2E tests hang during OAuth login due to incompatibility with Fosite library. Workaround: Use API-based authentication.

See: `tests/README.md#playwright-authentication-hangs-known-issue`

### Backend Model Availability

Inference tests may fail if vLLM backends are not running or models are still loading. Tests gracefully skip when backends are unavailable.

## Test Metrics

| Metric | Target | Current | Notes |
|--------|--------|---------|-------|
| Unit test coverage | 80% | TBD | Run `make coverage` |
| E2E pass rate | 100% | TBD | CI dashboard |
| Smoke test duration | < 2 min | TBD | `TestSmokeEndToEnd` |

## Related Documentation

- [Test README](../../tests/README.md) - Detailed test organization
- [E2E Test Harness](../../tests/e2e/README.md) - E2E framework documentation
- [E2E Spec](../../specs/012-e2e-tests/spec.md) - E2E test specification
- [User-Org E2E](../../services/user-org-service/docs/e2e-testing.md) - Service-specific E2E

## Changelog

| Date | Change | Author |
|------|--------|--------|
| 2025-11-29 | Initial test coverage tracking document | Claude |
| 2025-11-29 | Added smoke_test.go with models and usage verification | Claude |
