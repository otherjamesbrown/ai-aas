# Investigation Report

**Bead**: ai-aas-8v77
**Date**: 2025-12-14
**Investigator**: debugger agent

## Symptom

GuideLLM runner benchmarks fail with zero requests after deploying the API key fix from ai-aas-caxh. The runner.go was updated to read OPENAI_API_KEY from environment and pass it to the guidellm subprocess, but benchmarks still report zero successful requests.

**Error logs**:
```
level=ERROR msg="benchmark completed with ZERO requests - possible validation failure"
```

## Reproduction

The issue occurs on every benchmark run:
1. Pod `guidellm-runner-859597d69d-vqqxb` in namespace `monitoring`
2. Runs every 5 minutes against targets in development environment
3. Both unsloth-gpt-oss-20b and mistral-7b targets fail with zero requests
4. OPENAI_API_KEY environment variable is correctly set in the pod

## Evidence Gathered

| Source | Finding |
|--------|---------|
| Pod environment | OPENAI_API_KEY is set correctly (length 50 chars) |
| /home/dev/guidellm-runner/internal/runner/runner.go:112-119 | Code correctly reads OPENAI_API_KEY and appends to cmd.Env |
| Manual guidellm execution | Backend validation SUCCEEDS, but requests get 401 Unauthorized |
| /tmp/test3/benchmarks.json | Shows `"error": "Client error '401 Unauthorized' for url 'https://api.dev.otherjamesbrown.com/v1/chat/completions'"` |
| /usr/local/lib/python3.11/site-packages/guidellm/backends/openai.py:132-145 | httpx.AsyncClient created WITHOUT any headers |
| /usr/local/lib/python3.11/site-packages/guidellm/backends/openai.py:256-263 | Requests use `headers=request.arguments.headers` from request object, NOT from environment |
| Manual test with api_key in backend_kwargs | TypeError: `OpenAIHTTPBackend.__init__() got an unexpected keyword argument 'api_key'` |

## Hypotheses Tested

| Hypothesis | Verdict | Evidence |
|------------|---------|----------|
| API key environment variable not set | ❌ Ruled out | `kubectl exec` confirms OPENAI_API_KEY is present |
| Fix not deployed to running container | ❌ Ruled out | runner.go code shows fix is present (lines 112-119) |
| Network connectivity issue | ❌ Ruled out | Backend validation succeeds, proving network works |
| Invalid data parameter | ⚠️ Initial red herring | `type=emulated` was invalid, but JSON array format also fails with 401 |
| guidellm doesn't use OPENAI_API_KEY | ✅ CONFIRMED | Source code shows no usage of OPENAI_API_KEY environment variable |

## Root Cause

**Category**: `external_dependency`

**Explanation**:

The guidellm library's `OpenAIHTTPBackend` class does NOT automatically read the `OPENAI_API_KEY` environment variable or add it as a Bearer token to HTTP requests.

The backend creates an httpx.AsyncClient without any default headers (openai.py:132-145), and all requests use headers from the request object itself (`headers=request.arguments.headers` at line 259), not from the environment.

While the runner.go fix correctly passes OPENAI_API_KEY to the guidellm subprocess's environment, guidellm simply ignores it because it doesn't have code to:
1. Read OPENAI_API_KEY from os.environ
2. Add it as an Authorization: Bearer header to requests

The docstring example showing `api_key="your-api-key"` is misleading - attempting to pass api_key in backend_kwargs results in `TypeError: OpenAIHTTPBackend.__init__() got an unexpected keyword argument 'api_key'`.

**Evidence**:
- Manual execution with OPENAI_API_KEY set: Gets 401 Unauthorized
- Source code inspection: No environment variable reading code
- TypeError when trying to pass api_key parameter
- Requests fail with 401 even though manual curl with Bearer token succeeds

## Context Gap Check

- [X] Was this caused by missing context? **NO**

This is not a context gap. The issue is a limitation/incompatibility of the guidellm library with OpenAI-style Bearer token authentication. The runner.go fix was correct based on common practice (many OpenAI-compatible tools read OPENAI_API_KEY from environment), but guidellm specifically does not support this pattern.

## Proposed Fix

**Affected files**:
- `/home/dev/guidellm-runner/internal/runner/runner.go` - needs to inject Authorization header into guidellm request

**High-level description**:

Since guidellm doesn't support OPENAI_API_KEY environment variable, we need to pass the API key as a request header. Options:

### Option 1: Use request-level customization (RECOMMENDED)
Modify runner.go to construct the guidellm benchmark command with custom headers:
- Add `--request-headers '{"Authorization": "Bearer <api-key>"}'` to the guidellm command
- OR use guidellm's request customization mechanism if available

### Option 2: Patch guidellm backend
- Fork guidellm or create a local patch
- Modify OpenAIHTTPBackend to read OPENAI_API_KEY and inject into client headers
- Requires maintaining custom guidellm fork/patch

### Option 3: Use a local proxy
- Run a proxy (like nginx or envoy) that adds Authorization header
- Point guidellm at localhost proxy
- Proxy forwards to actual API with auth header added

### Option 4: Check if guidellm has auth configuration
- Review full guidellm documentation for auth mechanisms
- May have a config file or different approach not found in source inspection

**Estimated complexity**: Medium

The implementation will require:
1. Research guidellm's full API/CLI options for custom headers
2. Modify buildArgs() in runner.go to include auth header
3. Test with manual guidellm execution
4. Rebuild Docker image and deploy

## Prevention

How to prevent this class of bug in future:

| Type | Action |
|------|--------|
| Test | Add integration test that verifies guidellm can authenticate with API key |
| Lint | N/A - this is an external dependency compatibility issue |
| Context | Document that guidellm doesn't support OPENAI_API_KEY env var |
| Logging | Add debug logging in runner.go showing full guidellm command being executed |
| Documentation | Add guidellm auth requirements to guidellm-runner README |

## Follow-up Beads Created

| Bead | Type | Assigned To | Purpose |
|------|------|-------------|---------|
| (to be created) | bug | go-services-developer | Fix guidellm authentication by injecting Authorization header |
| (to be created) | task | go-services-developer | Add integration test for guidellm auth |
| (to be created) | task | go-services-developer | Add debug logging for guidellm command execution |
