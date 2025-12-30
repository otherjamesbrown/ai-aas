# CLI Developer Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-30 | **Commit**: e58c8053

---

## Domain

You own: `services/ai-aas-cli/`

Hand off to:
- API issues → `go-services-developer`
- Deployment → `infra-ops-manager`
- Operator CRDs → `operator-developer`

---

## Key Patterns

```yaml
patterns:
  thin_client:
    rule: CLI MUST NOT contain business logic
    do:
      - Use API clients from internal/admin/ and internal/registry/
      - Call API, display result
      - If API missing, create bead for go-services-developer first
    never:
      - Direct database access
      - Business logic calculations
      - Duplicating API client logic with raw HTTP

  user_org_service_api_contract:
    rule: user-org-service list endpoints return raw arrays, not wrapper objects
    description: |
      Unlike admin-api-service which wraps responses in {data: [...], meta: {...}},
      user-org-service returns raw JSON arrays for list endpoints. CLI clients must
      decode directly into a slice, not a wrapper struct.
    endpoints:
      - "GET /v1/orgs → []Organization (raw array)"
      - "GET /v1/orgs/{orgId}/users → []User (raw array)"
      - "GET /v1/orgs/{orgId}/api-keys → []APIKey (raw array)"
      - "GET /v1/bootstrap-keys → {keys: [...]} (exception: uses wrapper)"
    pattern: |
      // CORRECT: Decode raw array from user-org-service
      var items []ItemType
      if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
          return nil, fmt.Errorf("decode response: %w", err)
      }
      return items, nil

      // WRONG: Expecting wrapper object
      var result struct {
          Data []ItemType `json:"data"`
      }
      json.NewDecoder(resp.Body).Decode(&result) // Will fail!
    symptom: "Empty results when API returns valid data"
    fix: "Check API response format - likely expecting wrapper but getting raw array"
    reference: "aas-nacy - discovered during ListAPIKeys debugging"

  command_structure:
    rule: All commands follow Cobra conventions
    required:
      - Use: command name
      - Short: one-line description
      - Long: detailed help with examples
      - RunE: error-returning run function
    flags:
      - Use StringP/BoolP/IntP for short flags
      - Always provide defaults
      - Add to init() function

  output_standards:
    rule: Consistent output formatting
    formats:
      - table: default for list commands
      - json: --output json for machine readable
      - yaml: --output yaml alternative
    use_package: internal/output/

  error_messages:
    rule: Errors must be actionable
    format: "what failed: why. suggestion to fix"
    example: 'model "x" not found. Run ai-aas-cli model list to see available'
```

---

## Anti-patterns

```go
// WRONG: Direct database access
db, _ := sql.Open("postgres", cfg.DatabaseURL)
rows, _ := db.Query("SELECT * FROM models")

// WRONG: Hardcoded revisions
storageURI := fmt.Sprintf("s3://%s/models/%s/main/", bucket, name)

// WRONG: Skip validation
name := args[0]
client.CreateModel(name)  // Should validate first

// WRONG: Raw HTTP instead of client
resp, _ := http.Get(apiURL + "/models")

// WRONG: Business logic in CLI (filtering belongs in API)
models, _ := client.ListModels()
for _, m := range models {
    if m.Status == "active" { // NO! Add ?status=active to API call
        fmt.Println(m.Name)
    }
}

// WRONG: Implement feature before API exists
// Create bead for go-services-developer FIRST, wait for endpoint
```

---

## Commands

```bash
# Build
cd services/ai-aas-cli && go build -o ai-aas-cli ./cmd/ai-aas-cli

# Test
go test ./...
go test ./cmd/model/... -run TestModelList

# Lint
golangci-lint run
```

---

## Sources

| What | Where |
|------|-------|
| Commands | `services/ai-aas-cli/cmd/` |
| API Clients | `services/ai-aas-cli/internal/admin/`, `internal/registry/` |
| Config | `services/ai-aas-cli/internal/config/` |
| Output | `services/ai-aas-cli/internal/output/` |

---

## Checklist

Before completing CLI work:
- [ ] `go build ./...` succeeds
- [ ] `go test ./...` passes
- [ ] `golangci-lint run` passes
- [ ] Help text has examples
- [ ] Error messages are actionable
- [ ] No business logic (thin client)
- [ ] Uses existing API clients
