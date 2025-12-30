# Investigation Report

**Bead**: aas-6zxj
**Date**: 2025-12-30
**Investigator**: debugger agent

## Symptom

E2E tests `TestTextCompletions` and `TestTextCompletionsStreaming` are failing with error code mismatch:
- **Initially reported**: 502 Bad Gateway
- **Actual error**: 402 Payment Required (TOKEN_QUOTA_EXCEEDED)
- Both text completions tests fail while chat completions tests pass

## Reproduction

Text completion requests return 402:

```bash
curl -X POST https://api.dev.otherjamesbrown.com/v1/completions \
  -H "Authorization: Bearer <test-api-key>" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-oss-20b", "prompt": "Hello", "max_tokens": 10}'

# Response: 402 Payment Required
# Error: "Token quota exceeded for day period"
```

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `api-router-service logs` | All /v1/completions requests return 402 with TOKEN_QUOTA_EXCEEDED |
| `middleware.go:421-505` | TokenQuotaMiddleware blocks requests before handler |
| `handler.go:181` | Route /v1/completions IS registered correctly |
| `openai.go:272-371` | HandleOpenAICompletions handler implementation exists |
| `Redis key` | ratelimit:tokens:eda1ca3a-dbfa-4a48-a591-1b7e7f489e1a:day:1767052800 = **1,000,721** |
| `config.go:55-57` | Default limits: Hourly=100k, **Daily=1M**, Weekly=5M |

### Log Evidence

```json
{
  "level": "info",
  "msg": "request denied",
  "organization_id": "eda1ca3a-dbfa-4a48-a591-1b7e7f489e1a",
  "decision_reason": "TOKEN_QUOTA_EXCEEDED",
  "limit_state": "TOKEN_QUOTA_EXCEEDED",
  "status": 402
}
```

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Route not registered for /v1/completions | ❌ Ruled out | Route registered at handler.go:181 |
| Handler implementation missing | ❌ Ruled out | HandleOpenAICompletions exists at openai.go:272 |
| Backend doesn't support text completions | ❌ Ruled out | Never reaches backend - blocked at middleware |
| Routing policy missing | ❌ Ruled out | Never reaches routing logic |
| **Token quota exceeded** | ✅ **CONFIRMED** | Redis shows 1,000,721 > 1,000,000 daily limit |

## Root Cause

**Category**: `test_infrastructure`

**Explanation**:

The E2E test organization (eda1ca3a-dbfa-4a48-a591-1b7e7f489e1a) has exceeded the daily token quota limit configured in api-router-service.

**Token Usage Analysis**:
- Daily limit: 1,000,000 tokens (RATE_LIMIT_TOKENS_DAILY)
- Current usage: 1,000,721 tokens
- Exceeded by: 721 tokens

**Request Flow**:
1. POST /v1/completions arrives at api-router-service
2. BodyBufferMiddleware parses request ✓
3. AuthContextMiddleware validates API key ✓
4. **TokenQuotaMiddleware checks daily quota** → BLOCKED
5. Returns 402 Payment Required
6. HandleOpenAICompletions handler **never invoked**

**Why text completions fail but chat completions succeed**:
- E2E test suite runs chat completions tests first
- Cumulative token usage from all tests approaches daily limit
- Text completions tests run later in suite
- By the time text completions tests execute, quota is exhausted

**The /v1/completions endpoint implementation is working correctly** - it's simply being blocked by quota enforcement middleware as designed.

## Context Gap Check

- [X] Was this caused by missing context? **YES**

**Context file**: `tests/e2e/README.md` or `tests/e2e/fixtures/organizations.go`
**What was missing**:
- E2E test documentation doesn't mention token quota limits
- Organization fixture doesn't configure unlimited quotas for test orgs
- No guidance on clearing Redis token counters between test runs

**Suggested fix**:
1. Document that E2E tests are subject to token quotas
2. Add fixture option to create orgs with unlimited quotas (set limits to 0 to disable)
3. Add Redis cleanup step to E2E test harness to reset token counters

## Proposed Fix

**Option 1: Disable token quotas for E2E tests** (Recommended)
- Set RATE_LIMIT_TOKENS_DAILY=0 in development environment
- Zero value disables quota checks (see middleware.go:441)
- Allows unlimited E2E test execution

**Option 2: Increase quota limits for development**
- Raise RATE_LIMIT_TOKENS_DAILY to 10,000,000 (10M)
- Still provides some protection against runaway usage
- Requires environment variable change in deployment

**Option 3: Clear Redis counters between test runs**
- Add cleanup step to E2E test harness
- Delete ratelimit:tokens:* keys before test suite
- Ensures fresh quota state for each run

**Option 4: Configure test orgs with unlimited quotas**
- Modify organization fixture to set quota overrides
- Requires Admin API support for per-org quota settings
- Most flexible but requires API changes

**Affected files**:
- `services/api-router-service/deployments/helm/api-router-service/values-development.yaml` - Set RATE_LIMIT_TOKENS_DAILY=0
- OR `tests/e2e/harness/context.go` - Add Redis cleanup step
- OR `tests/e2e/fixtures/organizations.go` - Add quota configuration

**Estimated complexity**: Low (Option 1), Medium (Options 2-3), High (Option 4)

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add E2E test assertion for quota header presence in 402 responses |
| Lint | N/A |
| Context | Document token quota behavior in E2E test README |
| Logging | Quota middleware already logs denial reason - adequate |
| Monitoring | Add alert for repeated 402 responses in development environment |

## Follow-up Beads Created

| Bead | Type | Labels | Purpose |
|------|------|--------|---------|
| aas-8nqa | task | devops | Disable token quotas in development environment |
| aas-8kuq | task | test-infrastructure | Add Redis cleanup to E2E test harness |
| aas-y3d2 | documentation | documentation, context-update | Document token quota behavior for E2E tests |
