# Go Services Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-13 | **Commit**: 24c3e0ee

---

## Domain

You own:
- `services/admin-api-service/` - Model registry, deployments
- `services/api-router-service/` - Inference gateway → vLLM
- `services/analytics-service/` - Usage tracking
- `services/user-org-service/` - Auth, users, orgs, RBAC
- `shared/` - Shared libraries

Hand off to:
- CLI commands → `cli-developer`
- Helm/deployment → `infra-ops-manager`
- Operator CRDs → `operator-developer`
- Frontend → `web-portal-developer`

---

## Key Patterns

```yaml
patterns:
  api_design:
    response_format:
      data: "interface{} (payload)"
      error: "{code, message, details}"
      meta: "{pagination, etc}"
    error_codes: "MODEL_NOT_FOUND, INTERNAL_ERROR, UNAUTHORIZED, etc"
    status_mapping:
      - ErrNotFound → 404
      - ErrUnauthorized → 401
      - ErrValidation → 400
      - Other → 500

  error_handling:
    rule: Always wrap errors with context
    format: "fmt.Errorf(\"failed to X: %w\", err)"
    never:
      - Swallow errors with _
      - Expose internal errors to API response
      - Return raw error.Error() to client

  database:
    rule: Use transactions for multi-step operations
    pattern: "BeginTx → operations → Commit (defer Rollback)"
    migrations: Goose SQL files in migrations/
    never:
      - Manual SQL in code (use migrations)
      - Hardcoded connection strings

  deployment_contract:
    rule: Update DEPLOYMENT.md when changing
    triggers:
      - Environment variables
      - Health endpoint paths
      - Resource requirements
      - Dependencies
      - Ports
    why: Interface with infra-ops-manager

  testing:
    rule: Table-driven tests preferred
    required:
      - Unit tests for business logic
      - Integration tests for API endpoints
    coverage: go test ./... -coverprofile=coverage.out

api_endpoints:
  admin-api-service:  # /v1 prefix
    models:
      - "GET    /models"
      - "POST   /models"
      - "GET    /models/{name}"
      - "DELETE /models/{name}"
      - "POST   /models/{name}/rename"
    cache:
      - "GET    /models/{name}/cache"
      - "POST   /models/{name}/pull"
      - "GET    /models/{name}/pull"
      - "DELETE /models/{name}/pull/{job_id}"
      - "POST   /models/{name}/cache/verify"
    credentials:
      - "GET    /credentials"
      - "POST   /credentials"
      - "POST   /credentials/{type}/test"
      - "DELETE /credentials/{type}"
    deployments:
      - "GET    /deployments"
      - "POST   /deployments"
      - "GET    /deployments/{model}/{env}"
      - "PUT    /deployments/{model}/{env}"
      - "DELETE /deployments/{model}/{env}"
    engines:
      - "GET    /engines"
      - "GET    /engines/{name}"
      - "POST   /engines/{name}/versions"
      - "DELETE /engines/{name}/versions/{version}"
    registry:
      - "POST   /registry/models"
      - "GET    /registry/models"
      - "GET    /registry/models/{name}"
      - "PATCH  /registry/models/{name}"
      - "DELETE /registry/models/{name}"
    routing:
      - "POST   /routing/policies"
      - "GET    /routing/policies"
      - "GET    /routing/policies/{id}"
      - "PATCH  /routing/policies/{id}"
      - "DELETE /routing/policies/{id}"
      - "POST   /routing/policies/{id}/activate"

  user-org-service:  # /v1 prefix
    api_keys:
      - "POST   /orgs/{orgId}/api-keys"
      - "GET    /orgs/{orgId}/api-keys"
      - "PATCH  /orgs/{orgId}/api-keys/{id}"
      - "POST   /orgs/{orgId}/api-keys/{id}/rotate"
      - "DELETE /orgs/{orgId}/api-keys/{id}"
    self:
      - "POST   /organizations/me/api-keys"
      - "GET    /organizations/me/api-keys"
```

---

## Anti-patterns

```go
// WRONG: Swallow errors
result, _ := doSomething()

// WRONG: Expose internal errors
return c.JSON(500, map[string]string{"error": err.Error()})

// WRONG: Manual schema changes
db.Exec("ALTER TABLE models ADD COLUMN new_field TEXT")

// WRONG: Hardcoded config
db, _ := sql.Open("postgres", "host=localhost...")

// WRONG: N+1 queries
for _, user := range users {
    org, _ := repo.GetOrg(user.OrgID)  // Query per user!
}

// WRONG: Not using transactions for multi-table updates
repo.UpdateModel(model)
repo.UpdatePolicy(policy)  // If this fails, model is inconsistent

// WRONG: Forgetting to update DEPLOYMENT.md when adding env var
// infra-ops-manager won't know to add it to Helm values
```

---

## Commands

```bash
# Test
cd services/<service> && go test ./...
go test ./internal/services/... -run TestModelCreate
go test ./... -coverprofile=coverage.out

# Lint
golangci-lint run

# Migrations
goose -dir migrations create add_new_table sql
goose -dir migrations postgres "$DATABASE_URL" up
goose -dir migrations postgres "$DATABASE_URL" down
```

---

## Sources

| Service | Key Code |
|---------|----------|
| admin-api | `internal/api/handlers/models.go`, `internal/repository/` |
| api-router | `internal/router/router.go`, `internal/backends/` |
| user-org | `internal/api/handlers/auth.go`, `internal/services/auth_service.go` |
| analytics | `internal/api/handlers/usage.go`, `internal/aggregation/` |
| shared | `shared/` |
| Structure | Each service: `cmd/*/main.go`, `internal/{api,models,repository,services}/` |

---

## Checklist

Before completing work:
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] API follows REST conventions
- [ ] Errors wrapped with context
- [ ] Migrations are idempotent
- [ ] DEPLOYMENT.md updated if changed env/ports/health
