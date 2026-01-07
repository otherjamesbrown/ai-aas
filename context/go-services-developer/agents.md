# Go Services Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-14 | **Commit**: 5b8479c4

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

    postgresql_enums:
      rule: ALWAYS cast strings when inserting into ENUM columns
      pattern: "$1::schema.enum_type"
      example:
        wrong: "INSERT INTO exports (status) VALUES ($1)"
        correct: "INSERT INTO exports (status) VALUES ($1::analytics.export_status)"
      why: "Go strings don't auto-cast to PostgreSQL ENUMs - causes 500 errors"
      testing: "Test against real PostgreSQL, not just mocks"
      related: aas-4xqk

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

  auth_middleware:
    location: "services/user-org-service/internal/httpapi/middleware/auth.go"
    context_keys:
      UserIDKey: "auth.user_id"
      OrgIDKey: "auth.org_id"
      SessionKey: "auth.session"
    middleware:
      RequireAuth:
        purpose: "Validates Bearer tokens (API key or OAuth)"
        returns_401: "Missing, invalid, or expired token"
        extracts: "UserID, OrgID, Scopes into context"
    usage_pattern: |
      // In route registration:
      router.With(middleware.RequireAuth(rt, logger)).Get("/protected", handler)

      // In handler - extract authenticated user:
      userID := r.Context().Value(middleware.UserIDKey).(uuid.UUID)
      orgID := r.Context().Value(middleware.OrgIDKey).(uuid.UUID)
    auth_types:
      api_key: "Hashed and validated against api_keys table"
      oauth: "Fosite token validation against oauth_sessions"
    multi_tenant:
      rule: "Always use orgID from context for data isolation"
      pattern: "WHERE org_id = $orgID"

model_registration_flow:
  description: "How models flow from AIModel CRs to api-router"
  diagram: |
    AIModel CR (ai-aas-config repo)
        ↓ ai-model-operator watches
    InferenceService (KServe creates)
        ↓ operator syncs status to Admin API
    Admin API deployment record (database)
        ↓ api-router queries on startup/refresh
    Backend routing (in-memory registry)

  steps:
    1_aimodel_created:
      trigger: "AIModel CR applied to cluster"
      actor: "ai-model-operator"
      action: "Creates InferenceService, syncs to Admin API"
      files:
        - "operators/ai-model-operator/controllers/aimodel_controller.go"

    2_deployment_synced:
      trigger: "InferenceService becomes Ready"
      actor: "ai-model-operator"
      action: "POST /v1/deployments to Admin API"
      data: "model name, endpoint URL, status, environment"
      files:
        - "operators/ai-model-operator/internal/adminapi/client.go"

    3_router_discovers:
      trigger: "api-router startup or refresh interval"
      actor: "api-router ModelRegistry"
      action: "GET /v1/deployments?status=ready from Admin API"
      files:
        - "services/api-router-service/internal/registry/model_registry.go"

    4_request_routed:
      trigger: "Client request to /v1/chat/completions"
      actor: "api-router"
      action: "Look up model in registry, proxy to InferenceService"
      files:
        - "services/api-router-service/internal/api/public/openai.go"

  error_scenarios:
    model_not_found:
      cause: "Model not in Admin API deployment records"
      check: "GET /v1/deployments?model=<name>"
      fix: "Verify AIModel exists and InferenceService is Ready"

    stale_registry:
      cause: "api-router hasn't refreshed since deployment"
      check: "kubectl logs api-router | grep 'registry refresh'"
      fix: "Wait for refresh interval or restart api-router"

    endpoint_mismatch:
      cause: "Deployment record has wrong endpoint URL"
      check: "Compare InferenceService URL with deployment record"
      fix: "Operator should sync - check operator logs"

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

## API Scope Authorization Patterns

**CRITICAL**: Distinguish between org-scoped and platform-scoped admin keys.

```yaml
scope_types:
  org:admin:
    use_case: "Org-specific admin operations"
    behavior: "Can only access resources in key's own organization"
    example: "Org dashboard, tenant-specific management"

  admin:
    use_case: "Platform-wide admin operations"
    behavior: "Can access ANY organization (bypasses org membership)"
    example: "Bootstrap keys, cross-tenant tools, platform administration"

  "*":
    use_case: "Wildcard admin (same as admin)"
    behavior: "Platform-wide access"

two_layer_authorization:
  description: "Handlers implement two authorization checks"
  layer_1: "Middleware checks scopes (RequireAdminScope)"
  layer_2: "Handler checks org access (requireOrgAccess)"
  logic:
    - "admin/* scope → bypass org membership check"
    - "org:admin scope → require authOrgID == targetOrgID"
```

**Anti-pattern**:
```go
// WRONG: Bootstrap key with org-scoped admin
// This key can only manage its own org, not perform cross-org operations
apiKey := &APIKey{
    Scopes: []string{"org:admin"},  // TOO RESTRICTIVE for bootstrap
}

// CORRECT: Bootstrap key with platform admin
apiKey := &APIKey{
    Scopes: []string{"admin"},  // Can access any org
}
```

**Handler pattern**:
```go
// In handlers after RequireAdminScope middleware:
func (h *Handler) CreateAPIKey(c echo.Context) error {
    authOrgID := c.Get("orgID").(string)
    targetOrgID := c.Param("orgId")
    scopes := c.Get("scopes").([]string)

    // Platform admin can access any org
    if !hasAdminScope(scopes) && authOrgID != targetOrgID {
        return c.JSON(403, map[string]string{"error": "forbidden"})
    }
    // ... proceed with operation
}
```

Related: aas-q02h investigation

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

// WRONG: Changing field type from string to interface{} without nil-safety
// JSON unmarshaling behavior changes: string never nil, interface{} can be nil
type Message struct {
    Content interface{} `json:"content"`  // Was string before multimodal
}

func processContent(msg Message) string {
    return msg.Content.(string)  // PANIC if content is nil!
}

// CORRECT: Normalize nil after unmarshal OR check before type assertion
func processContent(msg Message) string {
    if msg.Content == nil {
        return ""
    }
    if s, ok := msg.Content.(string); ok {
        return s
    }
    // Handle other types (array for multimodal)
    return ""
}

// BETTER: Normalize immediately after JSON unmarshal
func normalizeMessage(msg *Message) {
    if msg.Content == nil {
        msg.Content = ""  // Normalize nil to empty string
    }
}

// WHY: string fields get "" (empty string) from JSON null
//      interface{} fields get nil from JSON null
// Related: aas-88gl investigation (P1 bug)

// WRONG: Using BackendRegistry for user-facing model lists
// BackendRegistry is static config from BACKEND_ENDPOINTS env var
// It shows ALL configured backends, not just deployed/enabled ones
func (h *Handler) ListModels(c echo.Context) error {
    backends := h.backendRegistry.List()  // Returns static config!
    return c.JSON(200, backends)  // Exposes disabled/undeployed models
}

// CORRECT: Use ModelRegistry for user-facing model lists
// ModelRegistry reflects actual deployment state from Admin API
func (h *Handler) ListModels(c echo.Context) error {
    models, err := h.modelRegistry.ListEnabled(ctx)  // Only enabled + ready
    if err != nil {
        return h.handleError(c, err)
    }
    return c.JSON(200, models)
}

// CONTEXT:
// - BackendRegistry: Static configuration, set at startup from env vars
// - ModelRegistry: Dynamic state from Admin API, reflects actual deployments
// - User-facing endpoints (/v1/models) should ONLY show models users can use
// Related: aas-e80 investigation

// WRONG: Authorization denial without logging
func RequireAdminScope(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        scopes := c.Get("scopes").([]string)
        if !hasAdminScope(scopes) {
            return c.JSON(403, map[string]string{"error": "forbidden"})
            // No logging! Silent failures are impossible to debug
        }
        return next(c)
    }
}

// CORRECT: Log authorization denials for audit trail
func RequireAdminScope(next echo.HandlerFunc) echo.HandlerFunc {
    return func(c echo.Context) error {
        scopes := c.Get("scopes").([]string)
        orgID := c.Get("orgID")
        keyID := c.Get("keyID")

        if !hasAdminScope(scopes) {
            log.Warn().
                Str("keyID", keyID.(string)).
                Str("orgID", orgID.(string)).
                Strs("scopes", scopes).
                Str("required", "admin").
                Str("path", c.Path()).
                Msg("authorization denied: insufficient scope")
            return c.JSON(403, map[string]string{"error": "forbidden"})
        }
        return next(c)
    }
}

// WHY LOGGING MATTERS:
// - Debug E2E test failures: "did the middleware even run?"
// - Security audit: track who tried to access what
// - Root cause analysis: correlate with API key creation
// Related: aas-k25w investigation

// WRONG: Chi router.Route() subrouter with nested routes
// Subrouter captures ALL requests matching prefix, preventing child routes from working
func RegisterRoutes(router chi.Router) {
    router.Route("/v1/orgs", func(r chi.Router) {
        r.Post("/", handler.CreateOrg)
        r.Get("/", handler.ListOrgs)
        r.Get("/{orgId}", handler.GetOrg)
    })
    // This DELETE will return 405! Subrouter above captures /v1/orgs/*
    router.Delete("/v1/orgs/{orgId}/users/{userId}", handler.DeleteUser)
}

// CORRECT: Register routes directly on parent router (no subrouter)
func RegisterRoutes(router chi.Router) {
    // Direct registration allows Chi to route to most specific match
    router.Post("/v1/orgs", handler.CreateOrg)
    router.Get("/v1/orgs", handler.ListOrgs)
    router.Get("/v1/orgs/{orgId}", handler.GetOrg)
    // Now this works! No subrouter to capture the request first
    router.Delete("/v1/orgs/{orgId}/users/{userId}", handler.DeleteUser)
}

// RULE: Avoid router.Route() when you have nested paths registered elsewhere
// Chi subrouters capture based on prefix, not specific path matching
// Related: aas-fa9g investigation
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
