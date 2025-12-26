# Architecture Review Methodology

Detailed process for conducting architecture reviews.

## Review Phases

### Phase 1: Discovery (Read-Only)

For each theme, run an exploratory analysis across all components:

1. **Identify Components**: List all services, operators, and CLI tools
2. **Analyze Each Component**: Check adherence to theme criteria
3. **Document Findings**: Use theme template
4. **Score Components**: Apply rubric (1-5)
5. **Checkpoint**: Present findings to user before continuing

**Components to Review:**
- `services/admin-api-service`
- `services/api-router-service`
- `services/analytics-service`
- `services/user-org-service`
- `operators/ai-model-operator`
- `cmd/ai-aas-cli`
- `internal/*` (shared libraries)

### Phase 2: Prioritization

After all themes reviewed:

1. **Aggregate Scores**: Create summary matrix
2. **Identify Gaps**: Components scoring 1-2 on any theme
3. **Risk Assessment**: Categorize by impact and effort
4. **Prioritize**: Order by risk/effort matrix

| | Low Effort | High Effort |
|---|---|---|
| **Low Risk** | Do First | Schedule |
| **High Risk** | Plan Carefully | Major Initiative |

### Phase 3: Remediation Planning

1. **Group by Type**: Batch similar changes across components
2. **Create Beads**: One per remediation item
3. **Define Dependencies**: Link related beads
4. **Document in remediation.md**: With bead references

## Theme Review Criteria

### 1. Code Structure & Reuse

**Check For:**
- [ ] Consistent package organization across services
- [ ] Shared code in `internal/` properly extracted
- [ ] No duplicated code between services
- [ ] Functions < 50 lines (guideline)
- [ ] Clear separation of concerns
- [ ] Consistent naming conventions

**Files to Examine:**
- `services/*/internal/` structure
- `internal/` shared packages
- Function sizes and complexity

### 2. Configuration Management

**Check For:**
- [ ] Single source of truth for config
- [ ] Environment-specific overrides documented
- [ ] No hardcoded values
- [ ] Config validation at startup
- [ ] Consistent config loading pattern
- [ ] Secrets separated from config

**Files to Examine:**
- `*/config/` directories
- Environment variable usage
- Helm `values*.yaml` files
- `.env` files and references

### 3. Data Storage (etcd vs Postgres)

**Check For:**
- [ ] Clear separation: etcd for K8s state, Postgres for app data
- [ ] Consistent data access patterns
- [ ] No mixed storage for same data type
- [ ] Proper migrations for schema changes
- [ ] Connection pooling configured

**Files to Examine:**
- Database client initialization
- Repository patterns
- etcd client usage
- Migration files

### 4. Logging & Observability

**Check For:**
- [ ] Structured JSON logging
- [ ] Consistent log levels (debug/info/warn/error)
- [ ] Trace ID propagation
- [ ] Request ID in all logs
- [ ] No sensitive data in logs
- [ ] Prometheus metrics exposed
- [ ] Health/ready endpoints

**Files to Examine:**
- Logger initialization
- Log statements across services
- Metrics registration
- `/health` and `/ready` handlers

### 5. Error Handling

**Check For:**
- [ ] Consistent error types
- [ ] Error wrapping with context
- [ ] Proper error propagation
- [ ] Client-facing error format
- [ ] No swallowed errors
- [ ] Retryable vs non-retryable distinction

**Files to Examine:**
- Error type definitions
- Error handling in handlers
- Error responses in API

### 6. Security Practices

**Check For:**
- [ ] Input validation on all endpoints
- [ ] SQL injection prevention (parameterized queries)
- [ ] No secrets in code
- [ ] Auth middleware applied consistently
- [ ] RBAC properly scoped
- [ ] Rate limiting configured
- [ ] TLS for all external connections

**Files to Examine:**
- Input validation code
- Database query construction
- Auth middleware
- Secret references

### 7. API Design

**Check For:**
- [ ] Consistent REST conventions
- [ ] Proper HTTP status codes
- [ ] Versioned endpoints (if applicable)
- [ ] Consistent error response format
- [ ] OpenAPI specs maintained
- [ ] Pagination implemented consistently

**Files to Examine:**
- Route definitions
- Handler implementations
- Response structures
- OpenAPI/Swagger files

### 8. Kubernetes Patterns

**Check For:**
- [ ] Health probes on all deployments
- [ ] Resource requests/limits defined
- [ ] Service accounts with minimal permissions
- [ ] ConfigMaps/Secrets used appropriately
- [ ] Helm charts follow consistent structure
- [ ] Labels and selectors consistent

**Files to Examine:**
- Helm chart templates
- values.yaml files
- ArgoCD Application definitions

### 9. Testing Strategy

**Check For:**
- [ ] Unit tests for business logic
- [ ] Integration tests for APIs
- [ ] E2E tests for critical paths
- [ ] Consistent test patterns
- [ ] Mocking done consistently
- [ ] CI runs tests automatically

**Files to Examine:**
- `*_test.go` files
- Test utilities and helpers
- CI configuration
- E2E test scripts

## Checkpoints

After each theme review, pause and present:

```
═══════════════════════════════════════════════════
 Theme $N: $THEME_NAME - Review Complete
═══════════════════════════════════════════════════

 Components Reviewed: $COUNT

 Scores:
   - admin-api-service:    4/5
   - api-router-service:   3/5
   - ...

 Key Findings:
   - Finding 1
   - Finding 2

 Critical Issues (Score 1-2):
   - Issue requiring immediate attention

 Continue to next theme? [Y/n]
═══════════════════════════════════════════════════
```

## Comparing to Previous Reviews

When a previous review exists:

1. Load previous `summary.md`
2. Extract scores per theme per component
3. Calculate deltas
4. Flag regressions (score decreased)
5. Highlight improvements (score increased)

Include comparison in new summary:

```markdown
| Component | Theme | Previous | Current | Delta |
|-----------|-------|----------|---------|-------|
| api-router | Logging | 3 | 4 | ↑ +1 |
| admin-api | Security | 4 | 3 | ↓ -1 |
```
