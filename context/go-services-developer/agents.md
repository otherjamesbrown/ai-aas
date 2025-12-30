# Go Services Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-25 | **Commit**: 084a4b2e

---

## Domain

You own:
- `services/admin-api-service/` - Model registry, deployments
- `services/api-router-service/` - Inference gateway (vLLM HTTP, TRT-LLM gRPC)
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

  triton_tensorrt_llm:
    model_name: "Always use 'ensemble' as model name"
    why: "TRT-LLM pipeline uses ensemble (preprocessing → inference → postprocessing)"
    applies_to:
      http: "/v2/models/ensemble/infer"
      grpc: "ModelInferRequest.ModelName = 'ensemble'"
    routing: "Service selection (which pod) determines which model, not model name"
    input_tensors:
      required: ["text_input", "max_tokens"]
      optional: ["temperature", "top_p", "stop_words"]
      note: "TRT-LLM ensemble rejects requests with unexpected inputs"
    grpc_error_handling:
      rule: "Always check ModelStreamInferResponse.ErrorMessage"
      why: "Backend errors appear in error_message, not gRPC status"
      symptom: "Empty completions (0 tokens) = likely error_message ignored"
      pattern: |
        if resp.ErrorMessage != "" {
          return fmt.Errorf("backend error: %s", resp.ErrorMessage)
        }
    never:
      - "Use policy.Model or user-facing model name for TRT-LLM"
      - "Vary model name based on which model is being called"
      - "Add 'stream' input tensor (streaming is via gRPC StreamInfer, not input)"
      - "Ignore error_message field (causes silent empty responses)"

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
    users:
      - "GET    /orgs/{orgId}/users"
      - "POST   /orgs/{orgId}/users"
      - "GET    /orgs/{orgId}/users/{userId}"
      - "GET    /orgs/{orgId}/users/by-email/{email}"
      - "PATCH  /orgs/{orgId}/users/{userId}"
      - "DELETE /orgs/{orgId}/users/{userId}"
      - "PUT    /orgs/{orgId}/users/{userId}/roles"
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

## Go Module Naming Conventions

```yaml
module_naming:
  repository_root: "github.com/otherjamesbrown/ai-aas"
  
  service_modules:
    pattern: "github.com/otherjamesbrown/ai-aas/services/{service-name}"
    examples:
      - "github.com/otherjamesbrown/ai-aas/services/admin-api-service"
      - "github.com/otherjamesbrown/ai-aas/services/api-router-service"
      - "github.com/otherjamesbrown/ai-aas/services/user-org-service"
      - "github.com/otherjamesbrown/ai-aas/services/analytics-service"
    location: "services/{service-name}/go.mod"
    
  cli_module:
    pattern: "github.com/otherjamesbrown/ai-aas/services/ai-aas-cli"
    location: "services/ai-aas-cli/go.mod"
    
  shared_module:
    pattern: "github.com/ai-aas/shared-go"
    location: "shared/go/go.mod"
    note: "Different prefix - uses ai-aas not otherjamesbrown/ai-aas"
    
  import_patterns:
    internal_imports:
      rule: "Use full module path + internal package"
      example: |
        import "github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
        import "github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/repository"
      never: |
        import "internal/domain"  // WRONG: Missing module path
        
    shared_imports:
      rule: "Use shared module with replace directive"
      go_mod_replace: |
        replace github.com/ai-aas/shared-go => ../../shared/go
      example: |
        import "github.com/ai-aas/shared-go/middleware"
        import "github.com/ai-aas/shared-go/observability"
      never: |
        import "shared/go/middleware"  // WRONG: Not a valid import path
        
    cross_service_imports:
      rule: "Generally avoid - services should be independent"
      exception: "CLI may import service packages for client libraries"
      pattern: |
        import "github.com/otherjamesbrown/ai-aas/services/admin-api-service/pkg/client"
      
  common_pitfalls:
    module_path_mismatch:
      symptom: "go build fails with 'package X is not in std'"
      cause: "go.mod module name doesn't match import paths"
      fix: |
        1. Check go.mod: module github.com/otherjamesbrown/ai-aas/services/{service}
        2. Verify imports use full path: github.com/otherjamesbrown/.../internal/...
        3. Run: go mod tidy
        
    wrong_shared_path:
      symptom: "Cannot find package github.com/ai-aas/shared-go"
      cause: "Missing replace directive in service's go.mod"
      fix: |
        Add to service's go.mod:
        replace github.com/ai-aas/shared-go => ../../shared/go
        
    relative_imports:
      symptom: "import './internal/domain' not supported"
      cause: "Using relative imports instead of module-based"
      fix: "Always use full module path, even for same-module imports"
      
  verification_commands:
    check_module: "head -5 go.mod  # Verify module name"
    check_imports: "go list -f '{{.ImportPath}}' ./...  # List all packages"
    fix_dependencies: "go mod tidy  # Clean up go.mod and go.sum"
    verify_build: "go build ./...  # Ensure all packages compile"
    check_shared: "go list -m github.com/ai-aas/shared-go  # Verify replace works"
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

// WRONG: Module path doesn't match directory structure
// In services/admin-api-service/go.mod:
module github.com/admin-api-service  // Missing repo prefix!

// CORRECT:
module github.com/otherjamesbrown/ai-aas/services/admin-api-service

// WRONG: Relative import path
import "./internal/domain"
import "../shared/go/middleware"

// CORRECT: Use full module path
import "github.com/otherjamesbrown/ai-aas/services/admin-api-service/internal/domain"
import "github.com/ai-aas/shared-go/middleware"

// WRONG: Import from another service's internal package
import "github.com/otherjamesbrown/ai-aas/services/user-org-service/internal/auth"

// CORRECT: Services expose public APIs, not internal packages
// Either call via HTTP API or extract to shared if truly shared logic

// WRONG: Missing replace directive for shared module
// Service compiles locally but fails in CI/Docker
import "github.com/ai-aas/shared-go/middleware"
// go.mod has no "replace" line

// CORRECT: Add replace directive to service's go.mod
replace github.com/ai-aas/shared-go => ../../shared/go

// WRONG: Libraries that download data at runtime
// Kubernetes pods have no internet access - this WILL fail in production!
import "github.com/pkoukk/tiktoken-go"
tiktoken.GetEncoding("cl100k_base")  // Downloads BPE vocab from internet

// CORRECT: Embed data in binary or pre-download in Dockerfile
// Option 1: Use offline loader with embedded data
import "github.com/pkoukk/tiktoken-go-loader"
tiktoken.SetBpeLoader(loader.NewOfflineLoader())

// Option 2: Pre-download in Dockerfile and set cache dir
// ENV TIKTOKEN_CACHE_DIR=/app/tiktoken-cache
// RUN python -c "import tiktoken; tiktoken.get_encoding('cl100k_base')"

// WRONG: Any library that fetches remote resources at runtime
// - ML model weights downloaded on first use
// - Config fetched from remote URL without fallback
// - License validation that phones home
// All will fail in network-isolated pods!

// WRONG: Using user-facing model name for TensorRT-LLM backends
// Triton returns "model not found" because TRT-LLM only exposes "ensemble"
grpcReq.ModelName = policy.Model  // e.g., "meta-llama/Llama-3.1-8B-Instruct"
httpPath := fmt.Sprintf("/v2/models/%s/infer", policy.Model)

// CORRECT: Always use "ensemble" for TRT-LLM Triton backends
// The pod selection (routing) determines which model, not the model name param
grpcReq.ModelName = "ensemble"
httpPath := "/v2/models/ensemble/infer"
// TRT-LLM uses "ensemble" because it chains: preprocessing → inference → postprocessing
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

# Module verification
head -5 go.mod  # Verify module name
go list -f '{{.ImportPath}}' ./...  # List all packages
go mod tidy  # Clean up go.mod and go.sum
go build ./...  # Ensure all packages compile
go list -m github.com/ai-aas/shared-go  # Verify replace works
```

---

## Sources

| Service | Key Code |
|---------|----------|
| admin-api | `internal/api/handlers/models.go`, `internal/repository/` |
| api-router | `internal/api/public/openai.go`, `internal/adapter/triton/` |
| user-org | `internal/api/handlers/auth.go`, `internal/services/auth_service.go` |
| analytics | `internal/api/handlers/usage.go`, `internal/aggregation/` |
| shared | `shared/` |
| Structure | Each service: `cmd/*/main.go`, `internal/{api,models,repository,services}/` |

**Reference Docs:**
- Inference routing: `docs/architecture/inference-routing.md` (vLLM vs TRT-LLM, routing policies, gRPC)

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
- [ ] Module path matches import paths
- [ ] Shared module has replace directive in go.mod
