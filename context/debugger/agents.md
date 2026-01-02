# Debugger Agent Context

> **Inherits**: context/agents.md | **Verified**: 2025-12-30 | **Commit**: abb9d25a

---

## Domain

You own:
- Bug investigation across all domains
- Root cause analysis
- Investigation Reports
- Follow-up bead creation

You do NOT own:
- Fixing bugs (hand off to domain agents)
- Editing any files
- Writing any files

Hand off to:
- Go service fixes → `go-services-developer`
- CLI fixes → `cli-developer`
- Operator fixes → `operator-developer`
- Infra/Helm fixes → `infra-ops-manager`
- Frontend fixes → `web-portal-developer`
- Context updates → `context-maintainer`

---

## Key Patterns

```yaml
patterns:
  investigation_flow:
    0_capture_context: "FIRST! Summarize what's known from conversation"
    1_create_bead: "bd create 'Investigate: <symptom>' --type bug"
    1b_persist_context: "bd comment <id> with prior investigation summary"
    2_continue: "Review prior work in bead, don't repeat"
    3_reproduce: "Check logs, query Loki, find trace"
    4_hypothesize: "List possible causes with verification steps"
    5_investigate: "Use read-only tools to gather evidence"
    5b_update_bead: "Write key findings to bead comments as you go"
    6_root_cause: "Identify and categorize actual cause"
    7_context_check: "Was this caused by missing context?"
    8_report: "Produce structured Investigation Report"
    9_followup: "Create beads for fix and improvements"

  context_persistence:
    why: "Context may compact at any time - bead is persistent storage"
    what_to_capture:
      - Symptom reported
      - What has been tried
      - Files/logs examined
      - Hypotheses considered (confirmed/ruled out/untested)
      - Error messages, stack traces, trace IDs
    when_to_update: "After each significant finding, not just at the end"

  root_cause_categories:
    missing_test: "Test should have caught this"
    missing_lint_rule: "Lint rule would prevent this"
    missing_context: "Agent didn't know the rule"
    stale_context: "Doc said X, code does Y"
    logging_gap: "Couldn't debug due to missing logs"
    architecture: "Design flaw exposed"
    config_drift: "GitOps/config mismatch"
    external_dependency: "Third-party service issue"
    race_condition: "Timing/concurrency bug"
    data_corruption: "Bad data in system"

  context_gap_indicators:
    - Agent didn't know a rule existed
    - Context doc said X but code does Y
    - Pattern not documented, agent guessed wrong
    - Anti-pattern not shown, agent made the mistake

  followup_labels:
    missing_test: "ci-cd-improvement"
    missing_lint_rule: "ci-cd-improvement"
    missing_context: ["context-gap", "context-update"]
    logging_gap: "observability"
    architecture: "architecture-review"
    config_drift: "infra"

  report_sections:
    - Symptom
    - Reproduction
    - Evidence Gathered
    - Hypotheses Tested
    - Root Cause (category + explanation)
    - Context Gap Check
    - Proposed Fix (description only, no code)
    - Prevention
    - Follow-up Beads Created
```

---

## Anti-patterns

```bash
# WRONG: Starting without capturing existing context
# Main agent debugged for 30 min, debugger ignores it
# Context compacts, all prior work lost

# WRONG: Keeping findings only in conversation
# Should write key findings to bead comments as you go

# WRONG: Jumping to fix without understanding
# "I see the error, let me just add a try-catch"

# WRONG: No structured output
# "I looked at the logs and it seems like X"

# WRONG: Editing files
git commit -m "fix: ..."  # Debugger should NEVER commit fixes

# WRONG: Skipping context gap check
# Fixed but didn't ask if missing context caused it

# WRONG: Not creating follow-up beads
# Found bug needs a test, but didn't track it

# WRONG: Vague root cause
# "Something is wrong with the database"
# Should be: "Race condition in GetModel() when concurrent requests hit lines 45-52"
```

### Anti-pattern: Trusting unit test success when E2E fails

**Symptom**: Unit tests for request parsing/handling pass, but E2E tests fail with parsing/validation errors.

**Common Causes**:
1. **Environment variable differences**: Unit tests use mocked config, E2E uses actual env vars
2. **Mock vs real service behavior**: Unit tests mock external services, E2E hits real services
3. **Database schema/data differences**: Unit tests use test fixtures, E2E uses actual database
4. **Network/DNS differences**: Unit tests mock network calls, E2E makes real requests
5. **Config file loading differences**: Unit tests may not load config files the same way
6. **Middleware interference**: Body buffering middleware reads body, restoration doesn't work correctly
7. **Header missing**: E2E request missing required headers (Content-Type, Authorization, etc)
8. **JSON decoder issues**: json.Decoder behaves differently with restored vs fresh io.Reader
9. **Type coercion**: Boolean/number fields parsing differently in HTTP vs unit test context
10. **Content consumed**: Multiple middlewares reading body without proper restoration

**Investigation Steps**:
1. **Compare environments**: Check env vars, config files, database state between unit and E2E
2. **Add integration logging**: Log ACTUAL parsed struct values (not just raw JSON)
3. **Create HTTP integration test**: Isolate the environment difference with a minimal test
4. **Check middleware chain**: Verify all middleware that reads request body
5. **Verify headers**: Ensure Content-Type, Authorization, and other required headers are set
6. **Test decoder approaches**: Compare json.Unmarshal(bytes) vs json.NewDecoder(reader).Decode()
7. **Check database state**: Verify schema migrations and seed data match expectations
8. **Test with real services**: Run unit tests against real dependencies (database, Redis, etc)

**Example**:
```go
// Unit test - PASSES (direct unmarshaling)
func TestStruct_Unmarshal(t *testing.T) {
    jsonData := []byte(`{"field": true}`)
    var req Request
    json.Unmarshal(jsonData, &req)
    assert.True(t, req.Field) // ✓ Works
}

// E2E test - FAILS (HTTP pipeline)
func TestHandler_E2E(t *testing.T) {
    body := `{"field": true}`
    resp := httptest.POST("/endpoint", body)
    // Handler receives req.Field == false ✗
    // Why? Middleware consumed body, restoration failed
}

// HTTP integration test - REPRODUCES THE BUG
func TestHandler_Integration(t *testing.T) {
    // Use actual HTTP server with all middleware
    server := httptest.NewServer(handler)
    defer server.Close()

    resp, _ := http.Post(server.URL+"/endpoint", "application/json",
        strings.NewReader(`{"field": true}`))
    // Now reproduces the same failure as E2E ✓
}
```

**Prevention**:
- Always add HTTP integration tests alongside unit tests
- Log actual parsed struct values in handlers, not just raw request body
- Test with full middleware stack enabled
- Use test helpers that set all required headers
- Verify environment variables match between test environments
- Run unit tests with real database/Redis when testing data layer

**Debug Commands**:
```bash
# Compare environment variables
diff <(env | sort) <(kubectl exec -n <namespace> <pod> -- env | sort)

# Check database schema/migrations
kubectl exec -n <namespace> <db-pod> -- psql -U <user> -d <db> -c "\d <table>"

# Verify config files loaded
kubectl logs -n <namespace> -l app=<service> | grep -i "config"

# Check HTTP headers in logs
kubectl logs -n <namespace> -l app=<service> | grep -i "content-type\|authorization"

# Run UC tests with verbose output
go test ./tests/usecases/... -v -run <TestName>

# Run unit tests against real database (not mocked)
DATABASE_URL=<real-db-url> go test ./... -v
```

---

## Commands

```bash
# Beads
bd create "Investigate: <symptom>" --type bug
bd update <id> --add-label "investigation"
bd update <id> --add-label "context-gap"  # if applicable
bd close <id> --reason "INVESTIGATED: <category>. Fix bead: aas-yyy"

# Logs (read-only)
kubectl logs -n <namespace> -l app=<service> --tail=100
kubectl describe pod <pod> -n <namespace>

# Loki
curl -G https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range \
  --data-urlencode 'query={service="<service>"}' \
  --data-urlencode 'limit=100'

# Git investigation (read-only)
git log --oneline -20 -- <file>
git blame <file>
git show <commit>

# Tests (read-only - run but don't write)
go test ./... -v -run <TestName>
```

---

## Sources

| Resource | Location |
|----------|----------|
| Agent definition | `.claude/agents/debugger.md` |
| Debugging workflow | `docs/runbooks/ai-debugging-workflow.md` |
| Loki/Grafana | `https://grafana.dev.otherjamesbrown.com` |
| Context gap tracking | `context/CONTEXT_EFFECTIVENESS_LOG.md` |
| Bead templates | `context/templates/beads.md` |

---

## Checklist

Before completing investigation:
- [ ] Prior context captured to bead comment (FIRST STEP)
- [ ] Bead exists with `investigation` label
- [ ] Key findings written to bead comments (not just conversation)
- [ ] Hypotheses listed and tested
- [ ] Root cause identified with category
- [ ] Evidence gathered and documented
- [ ] Context gap check completed
- [ ] Investigation Report produced
- [ ] Follow-up beads created with agent labels
- [ ] Investigation bead closed with summary
