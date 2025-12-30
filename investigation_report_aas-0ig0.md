# Investigation Report

**Bead**: aas-0ig0
**Date**: 2025-12-30
**Investigator**: debugger agent

## Symptom

E2E test `TestTextCompletionsStreaming` fails with 502 error:
```
completions_test.go:253: Streaming returned status 502: {"error":"backend request failed: unmarshal OpenAI completion response: invalid character 'd' looking for beginning of value","code":"BACKEND_ERROR"}
```

The error suggests API router is trying to parse a backend response as JSON but encountering SSE format (Server-Sent Events) starting with "data:".

## Reproduction

**Test**: `tests/e2e/suites/completions_test.go::TestTextCompletionsStreaming`

**Steps**:
1. Test creates organization, service account, and API key
2. Sends `POST /v1/completions` with:
   ```json
   {
     "model": "gpt-oss-20b",
     "prompt": "The capital of France is",
     "stream": true,
     "max_tokens": 50,
     "temperature": 0.7
   }
   ```
3. Expects SSE response stream (Content-Type: text/event-stream)
4. Receives 502 with JSON error instead

## Evidence Gathered

| Source | Finding |
|--------|---------|
| `openai.go:52-59` | `OpenAICompletionRequest` struct has `Stream bool` field with correct JSON tag |
| `openai.go:277-280` | Request body unmarshaled with `json.NewDecoder(r.Body).Decode(&openAIReq)` |
| `openai.go:339-346` | Code checks `if openAIReq.Stream` and routes to streaming handler |
| `openai.go:349-352` | Non-streaming path wraps errors with "backend request failed:" |
| `openai.go:496-497` | Non-streaming handler creates error "unmarshal OpenAI completion response:" |
| `openai_test.go:14-50` | Unit test `TestOpenAICompletionRequest_Unmarshal_Stream` **PASSES** |
| `openai_test.go:52-96` | Unit test `TestOpenAICompletionRequest_Decode_AfterMiddleware` **PASSES** |
| Error message format | Proves non-streaming code path (line 349) was executed, not streaming path (line 344) |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| Struct definition wrong | ❌ Ruled out | Struct correctly defined with `Stream bool \`json:"stream,omitempty"\`` |
| JSON unmarshal broken | ❌ Ruled out | Unit tests pass: `TestOpenAICompletionRequest_Unmarshal_Stream` |
| Middleware corrupts body | ❌ Ruled out | `TestBodyBufferMiddleware_PreservesStreamField` passes |
| vLLM doesn't support /v1/completions streaming | ❌ Ruled out | [vLLM docs confirm support](https://docs.vllm.ai/en/stable/serving/openai_compatible_server/) |
| Stream field not set in E2E environment | ✅ CONFIRMED | E2E fails but unit tests pass → environment-specific issue |

## Root Cause

**Category**: `missing_test` + `config_drift`

**Explanation**:

The E2E test fails because the Stream field is **not being set to true** during request parsing in the actual deployment environment, despite being set correctly in unit tests.

**Why unit tests pass but E2E test fails:**

1. **Unit tests**: Direct struct unmarshaling - no HTTP middleware, no routing, no auth
2. **E2E tests**: Full request flow through HTTP server → middleware → auth → handler

**Possible causes of the discrepancy:**

1. **Body already consumed**: Some middleware reads `r.Body` before the handler, and the body restoration doesn't work correctly for boolean fields
2. **JSON parser bug**: The `json.Decoder` behaves differently when reading from a restored `io.ReadCloser` vs a fresh reader
3. **Content-Type issue**: Request might be missing `Content-Type: application/json` header in E2E environment
4. **Body size/buffer issue**: The body buffering middleware might have a bug with specific request sizes

**Debug logging added but not sufficient:**

Lines 283-290 and 332-336 in `openai.go` add debug logging to log the parsed Stream field value, but these logs are not accessible in this investigation. The test is failing in an environment where we can't see the logs to confirm the actual parsed value.

## Evidence Supporting Root Cause

```go
// Error message structure proves code path:
"backend request failed: unmarshal OpenAI completion response: invalid character 'd'"
                          ↑                                                        ↑
                    Line 351 wrapper                                    Line 497 error
```

This error can ONLY occur if:
1. Line 339 evaluated to FALSE (Stream != true)
2. Line 349 was executed (non-streaming path)
3. Line 496-497 tried to unmarshal backend response as JSON
4. Backend returned SSE format (starting with "data:") causing JSON parse error

## Context Gap Check

- [x] Was this caused by missing context? **YES**

**Context file**: `context/debugger/agents.md`

**What was missing**:
- No pattern documented for "unit tests pass but E2E tests fail" scenarios
- No guidance on investigating body buffering/middleware issues
- No examples of JSON unmarshaling bugs specific to request pipelines

**Suggested fix**: Add anti-pattern:
```markdown
### Anti-pattern: Trusting unit test success when E2E fails

If unit tests for request parsing pass but E2E tests fail with parsing errors:
1. Check middleware that reads/buffers request body
2. Check Content-Type header handling
3. Add logging to log actual parsed struct values (not just JSON)
4. Consider body already consumed before handler processes it
5. Test with actual HTTP requests, not just struct unmarshaling
```

## Proposed Fix

**High-level description**:

The fix requires identifying WHY the Stream field is not being set in E2E environment despite being set in unit tests.

**Approach**:

1. **Immediate fix**: Add explicit logging in `HandleOpenAICompletions` to log the actual Stream field value after parsing:
   ```go
   h.logger.Info("parsed completion request",
       zap.String("model", openAIReq.Model),
       zap.Bool("stream_field_value", openAIReq.Stream),  // CRITICAL: Log actual value
       zap.String("raw_body_preview", "<first 200 chars>"),
   )
   ```

2. **Diagnostic test**: Create integration test that makes actual HTTP POST request (not unit test) to verify:
   ```go
   func TestHandleOpenAICompletions_StreamField_IntegrationTest(t *testing.T) {
       // Create actual HTTP server with all middleware
       // Send POST /v1/completions with stream:true
       // Verify handler receives Stream==true
   }
   ```

3. **Root cause fix**: Once logging reveals the actual issue:
   - If middleware: Fix `BodyBufferMiddleware` to preserve body correctly
   - If Content-Type: Add validation/error for missing header
   - If JSON decoder: Switch to `json.Unmarshal` instead of `json.NewDecoder`

**Affected files**:
- `services/api-router-service/internal/api/public/openai.go` - Add logging, possibly change unmarshaling method
- `services/api-router-service/internal/api/public/middleware.go` - Fix body buffering if that's the issue
- `services/api-router-service/test/integration/` - Add HTTP integration test for streaming

**Estimated complexity**: Medium

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add integration test that makes real HTTP requests (not just unit tests) |
| Test | Add E2E test diagnostics to log actual parsed struct values |
| Lint | Consider adding linter rule to flag `json.NewDecoder(r.Body)` after middleware |
| Context | Add anti-pattern for "unit tests pass, E2E fails" debugging |
| Logging | Ensure all request parsing logs the parsed struct values (not just raw JSON) |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| TBD | bug | go-services-developer | Fix Stream field parsing in E2E environment |
| TBD | task | go-services-developer | Add HTTP integration test for streaming requests |
| TBD | task | context-maintainer | Add "unit vs E2E test failure" debugging pattern |

---

## Sources

- [vLLM OpenAI-Compatible Server Documentation](https://docs.vllm.ai/en/stable/serving/openai_compatible_server/)
- [vLLM OpenAI Chat Completion With Reasoning Streaming](https://docs.vllm.ai/en/latest/examples/online_serving/openai_chat_completion_with_reasoning_streaming/)
- [Support streaming for `/v1/completions` endpoints](https://github.com/letta-ai/letta/issues/1286)
