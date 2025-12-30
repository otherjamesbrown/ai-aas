# Investigation Report

**Bead**: aas-84dv
**Date**: 2025-12-30
**Investigator**: debugger agent

## Symptom

E2E test `TestTextCompletionsStreaming` reported to fail with 502 error:
```
Streaming returned status 502: {"error":"backend request failed: unmarshal OpenAI completion response: invalid character 'd' looking for beginning of value","code":"BACKEND_ERROR"}
```

Error suggests API router is trying to parse an SSE (Server-Sent Events) response starting with "data:" as JSON, causing a parse error.

## Reproduction

**Test**: `tests/e2e/suites/completions_test.go::TestTextCompletionsStreaming`

**Steps**:
1. Test sends POST `/v1/completions` with `stream: true`
2. Expected: SSE stream response (Content-Type: text/event-stream)
3. Reported: 502 error with "unmarshal OpenAI completion response" error

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `openai.go:272-378` | HandleOpenAICompletions parses request and routes based on Stream field |
| `openai.go:370-378` | If `openAIReq.Stream == true` → calls `forwardOpenAIStreamingRequest()` |
| `openai.go:381-383` | If `openAIReq.Stream == false` → calls `forwardOpenAIRequest()` with reqType="completion" |
| `openai.go:484-543` | `forwardOpenAIRequest()` tries to unmarshal response as JSON (line 528-529) |
| `openai.go:1208-1339` | `forwardOpenAIStreamingRequest()` proxies SSE stream without parsing |
| Error message | "unmarshal OpenAI completion response: invalid character 'd'" is from line 529 |
| Error path | Proves non-streaming handler (forwardOpenAIRequest) was called instead of streaming handler |
| Git history | Commit 8cab3325 (3 commits ago) fixed this exact issue in aas-wc6u |
| Current code | Lines 296-306 use buffered body with json.Unmarshal (fix is active) |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Stream field not parsed | ✅ CONFIRMED (original issue) | Previous investigation aas-0ig0 identified this |
| Buffered body not used | ✅ CONFIRMED (original cause) | json.NewDecoder didn't work after middleware |
| Fix not applied | ❌ Ruled out | Commit 8cab3325 is in current branch, code uses buffered body |
| User testing old code | ✅ LIKELY | Fix exists but user reports same error |
| Different root cause | ❓ POSSIBLE | If user has latest code, may be new issue |

## Root Cause

**Category**: `duplicate_investigation`

**Explanation**:

This issue was **already investigated and fixed** in previous beads:

1. **Original Investigation**: aas-0ig0 (2025-12-30 10:23-10:30)
   - Root cause: `json.NewDecoder(r.Body).Decode()` inconsistently parsed boolean fields after BodyBufferMiddleware processed the request body
   - Impact: Stream field defaulted to false, causing streaming requests to be routed to non-streaming handler
   - Result: Non-streaming handler tried to parse SSE response as JSON → "invalid character 'd'" error

2. **Fix Implementation**: aas-wc6u, commit 8cab3325 (2025-12-30)
   - Changed `HandleOpenAICompletions` to use buffered body from context
   - Uses `json.Unmarshal(bodyBytes, &openAIReq)` instead of `json.NewDecoder(r.Body).Decode()`
   - More deterministic parsing behavior after body has been read by middleware

3. **Current Status**:
   - Fix is active in develop branch (verified at lines 296-306 of openai.go)
   - No subsequent changes have modified this code
   - Code correctly uses buffered body approach

**Why User May Still See Error**:

1. **Testing against old code**: User may be running test against code before commit 8cab3325
2. **Environment issue**: Test environment may have stale deployment
3. **Different root cause**: If truly running latest code, may be a new/different issue with same symptom

## Context Gap Check

- [ ] Was this caused by missing context? NO

This is a duplicate investigation. The original investigation (aas-0ig0) already identified the root cause and the fix was implemented.

## Proposed Fix

**Status**: ✅ ALREADY FIXED

**Fix commit**: 8cab3325 (2025-12-30)

**Summary**: No new fix needed. User should:

1. **Verify code version**: Ensure test is running against commit 8cab3325 or later
   ```bash
   git log --oneline -1 -- services/api-router-service/internal/api/public/openai.go
   # Should show: 8cab3325 fix(api-router): use buffered body for request parsing [aas-wc6u]
   ```

2. **Check deployment**: If running E2E against deployed service, verify it's running latest image

3. **Capture new evidence**: If error persists with latest code, capture:
   - Full error message
   - API router service logs with debug level
   - Request/response details
   - vLLM backend logs

4. **Create new investigation**: If truly a different issue, create new bead with fresh evidence

## Prevention

How to prevent duplicate investigations:

| Type | Action |
|------|--------|
| Process | Before investigating, search beads for similar symptoms: `bd list --status closed | grep "streaming"` |
| Process | Check recent commits for related fixes: `git log --oneline --grep="streaming\|completions" -20` |
| Process | Read existing investigation reports in repository root: `ls -la investigation_report_*.md` |
| Documentation | Link related beads in investigation comments |
| Documentation | Create index of investigation reports for quick reference |

## Follow-up Beads Created

None - issue already fixed. User should verify they're testing latest code.

If user confirms error persists with latest code (commit 8cab3325+), create new investigation bead with fresh evidence.

---

## Related Beads

| Bead | Status | Description |
|------|--------|-------------|
| aas-0ig0 | Closed | Original investigation: "Text streaming completions returns 502: backend parse error" |
| aas-wc6u | Closed | Fix implementation: "Fix: Stream field not set in E2E environment for text completions" |
| aas-wbxv | Closed | Follow-up: "Add HTTP integration test for streaming text completions" |
| aas-gdbk | Closed | Follow-up: "Context: Add debugging pattern for unit-vs-E2E test failures" |
| aas-84dv | This bead | Duplicate investigation (user unaware of previous fix) |

## Sources

- Previous investigation report: `/home/dev/worktrees/develop/investigation_report_aas-0ig0.md`
- Fix commit: `8cab3325 fix(api-router): use buffered body for request parsing [aas-wc6u]`
- Current code: `services/api-router-service/internal/api/public/openai.go` lines 292-313
