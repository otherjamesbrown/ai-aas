# Remediation Backlog

**Review Date:** 2025-12-26
**Epic Bead:** aas-4g29

## P0 - Critical (Do Immediately)

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|
| Remove direct database access from CLI | Data Storage | cli | High | aas-7odh | Open |
| Move CLI secrets to OS credential manager | Security | cli | Medium | aas-od8s | Open |
| Add 40+ unit tests to analytics-service | Testing | analytics | High | aas-c8hg | Open |

### Details

**aas-7odh: Remove direct database access from CLI**
- File: `services/ai-aas-cli/internal/admin/deployment.go:102`
- Issue: `sql.Open()` bypasses Admin API
- Fix: Use Admin API client instead of direct DB connection
- Impact: Critical - violates API-First principle

**aas-od8s: Move CLI secrets to OS credential manager**
- File: `services/ai-aas-cli/internal/config/config.go:25-27`
- Issue: API keys stored in plaintext `~/.ai-aas-cli.yaml`
- Fix: Use keychain/credentials.json or require env vars
- Impact: Critical - security risk on shared systems

**aas-c8hg: Add unit tests to analytics-service**
- Current: Only 4 test functions for finance-critical service
- Target: 40+ tests covering aggregation, export, validation
- Impact: Critical - no safety net for refactoring

## P1 - High Priority (This Sprint)

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|
| Extract API key validation to shared library | Code | admin-api, api-router, user-org | Medium | aas-cssz | Open |
| Consolidate HTTP response utilities to shared | Code | all services | Medium | aas-wd41 | Open |
| Implement HMAC signature verification | Security | api-router | Medium | aas-65bd | Open |
| Add HPA/PDB templates to user-org-service | K8s | user-org | Low | aas-uo28 | Open |
| Apply security context to all Helm charts | K8s | api-router, analytics, user-org, operator | Medium | aas-toe9 | Open |
| Migrate admin-api to shared logging library | Logging | admin-api | Low | aas-cq98 | Open |
| Replace http.Error() with JSON responses | API | user-org | Medium | aas-tki6 | Open |

### Details

**aas-cssz: Extract API key validation**
- 250+ lines duplicated across 3 services
- Create `shared/go/auth/apikey/client.go`
- Single source of truth for validation logic

**aas-wd41: Consolidate HTTP utilities**
- Move admin-api's `httputil/response.go` to shared
- Standardize WriteJSON, WriteError across platform

**aas-65bd: HMAC verification**
- File: `api-router/internal/auth/authenticator.go:300`
- Issue: Uses `stub-secret` placeholder
- Fix: Fetch actual API key secret from user-org-service

**aas-uo28: User-org HPA/PDB**
- Values exist but no templates
- Copy from admin-api-service as baseline

**aas-toe9: Security context**
- Only admin-api has `runAsNonRoot`, `readOnlyRootFilesystem`
- Apply to all service Helm charts

**aas-cq98: Admin-api logging**
- Replace `zap.NewProduction()` with shared logging library
- 2-line change for trace context, sampling, service name

**aas-tki6: User-org JSON errors**
- Replace `http.Error(w, msg, status)` with structured JSON
- Match error format from admin-api-service

## P2 - Medium Priority (Next Sprint)

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|
| Standardize error codes in shared library | Code | admin-api, api-router | Low | aas-r9ot | Open |
| Decompose user-org auth middleware (442 lines) | Code | user-org | Medium | aas-i18t | Open |
| Add request logger middleware to user-org | Logging | user-org | Low | - | Open |
| Add worker metrics to analytics | Logging | analytics | Medium | - | Open |
| Enable testcontainers in admin-api | Testing | admin-api | Medium | - | Open |
| Standardize on envconfig across all services | Config | admin-api, cli, operator | High | - | Open |
| Remove hardcoded localhost DB default | Config | api-router | Low | - | Open |

## P3 - Low Priority (Backlog)

| Issue | Theme | Components | Effort | Bead | Status |
|-------|-------|------------|--------|------|--------|
| Extract Admin API client to shared | Code | operator, cli | Low | - | Open |
| Standardize package organization | Code | admin-api | Low | - | Open |
| Add startup probe to analytics | K8s | analytics | Low | - | Open |
| Complete CLI E2E tests | Testing | cli | Medium | - | Open |
| Add coverage reporting to CI | Testing | all | Low | - | Open |
| Implement standard pagination response | API | all | Medium | - | Open |

## Execution Strategy

### Week 1: P0 Security & Testing
1. **CLI Security** (aas-7odh, aas-od8s)
   - Remove direct DB access, implement credential manager
   - Run regression tests

2. **Analytics Tests** (aas-c8hg)
   - Create test infrastructure
   - Add 20 unit tests for aggregation logic
   - Add 20 unit tests for export logic

### Week 2: P1 Shared Libraries
3. **Extract API key validation** (aas-cssz)
   - Create shared package
   - Migrate admin-api, api-router, user-org
   - Run integration tests

4. **Consolidate HTTP utilities** (aas-wd41)
   - Move to shared
   - Update all services

### Week 3: P1 Infrastructure
5. **Helm chart hardening** (aas-uo28, aas-toe9)
   - Add templates, security context
   - Deploy to dev, validate

6. **Logging & API fixes** (aas-cq98, aas-tki6)
   - Quick wins, low risk

### Week 4: P1 Security & Cleanup
7. **HMAC verification** (aas-65bd)
   - Implement, test with webhooks

8. **Error handling standardization** (aas-r9ot, aas-i18t)
   - Begin middleware decomposition

## Tracking

View all remediation beads:
```bash
bd list --label=arch-review --label=remediation
```

View by priority:
```bash
bd list --label=remediation --priority=0  # P0 Critical
bd list --label=remediation --priority=1  # P1 High
bd list --label=remediation --priority=2  # P2 Medium
```
