# API Router Service Fix - Session Summary

**Date:** 2025-11-20
**Branch:** feat/vllm-gpu-deployment
**Status:** PARTIALLY COMPLETE - Authentication fixed, OTLP collector deployment needed

## Session Overview

This session continued from a previous conversation where work was being done on authentication and routing for the API Router service. The primary goals were to fix authentication issues and enable end-to-end request routing.

## Problems Addressed

### 1. Authentication Context Bug (FIXED ✅)
**Issue:** API requests failing with "authentication required" even with valid test API keys

**Root Cause:** Context key type mismatch in middleware
- Middleware stored auth context with typed constant: `authContextKey` (type `contextKey`)
- Handler retrieved with raw string: `"auth_context"`
- In Go, these don't match: `contextKey("auth_context")` ≠ `"auth_context"`

**Fix Applied:**
```go
// File: services/api-router-service/internal/api/public/middleware.go:264
// Changed from:
ctx := context.WithValue(r.Context(), authContextKey, authCtx)
// To:
ctx := context.WithValue(r.Context(), "auth_context", authCtx)
```

**Verification:** After fix, authentication passed and request proceeded to routing stage

**Commit:** 58cb94d9 - Pushed to main branch

### 2. Routing Policy Configuration (FIXED ✅)
**Issue:** After authentication fix, requests failed with "no routing policy configured"

**Root Cause:** No etcd service existed, CONFIG_SERVICE_ENDPOINT pointed to non-existent "etcd-service:2379"

**Fix Applied:**
1. Deployed etcd service to development namespace
2. Created Kubernetes job to seed routing policy into etcd
3. Successfully stored global policy for gpt-oss-20b model → vllm-gpt-oss-20b backend

**Files Created:**
- `/tmp/etcd-service.yaml` - etcd deployment and service
- `/tmp/seed-routing-policy.yaml` - Job to seed initial routing policy

**Verification:** Policy successfully created at `/api-router/policies/global-gpt-oss-20b`

### 3. Pod Crash Issue (IN PROGRESS ⚠️)
**Issue:** After committing authentication fix, CI-built Docker image causes pods to crash with no logs

**Investigation Process:**
1. Initially suspected LOG_LEVEL=debug - ruled out (one pod was running with it)
2. Suspected Docker image build issue - ruled out (local build had same problem)
3. Found pods produce ZERO logs and HTTP server never starts
4. Process runs (PID 1) but doesn't serve on port 8080

**Root Cause Identified:**
Telemetry initialization in `shared/go/observability/otel.go` blocks for >60 seconds trying to connect to unreachable OTLP endpoints:
- Default endpoint: `localhost:4317` (doesn't exist in container)
- Deployment had: `OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4318`
- Connection attempts timeout very slowly (>60 seconds)
- Even with fallback logic, liveness probe kills pod before initialization completes

**Attempted Fixes:**
1. ❌ Remove OTEL_EXPORTER_OTLP_ENDPOINT env var → Failed: "telemetry endpoint required" error
2. ❌ Set endpoint to `otel-collector.monitoring.svc.cluster.local:4317` → Failed: Still blocks trying to connect
3. ❌ Increase liveness probe initialDelaySeconds to 60s → Failed: Initialization takes >60s

**Solution in Progress:**
Deploy OTLP collector service to accept telemetry connections

## Files Modified

### Code Changes (Committed)
1. **services/api-router-service/internal/api/public/middleware.go**
   - Line 264: Fixed context key to use raw string
   - Lines 259-261: Added debug logging for authentication success

2. **services/api-router-service/internal/auth/authenticator.go**
   - Line 130: Added debug logging for stub validator usage

### Infrastructure Files (Created, Not Committed)
3. **/tmp/etcd-service.yaml** - etcd deployment for routing policies
4. **/tmp/seed-routing-policy.yaml** - Job to seed initial routing policy
5. **/tmp/otel-collector.yaml** - OTLP collector deployment (created, not yet applied)

### Documentation Files (Created)
6. **/home/dev/ai-aas/docs/API_ROUTER_POD_CRASH_INVESTIGATION.md** - Comprehensive investigation document
7. **/home/dev/ai-aas/docs/SESSION_SUMMARY_API_ROUTER_FIX.md** - This file

## Key Technical Discoveries

### 1. Go Context Keys Must Match Exactly
Context keys must match by both type AND value:
```go
type contextKey string
const authContextKey contextKey = "auth_context"

// These are NOT equal:
contextKey("auth_context") != "auth_context"
```

### 2. Debug Logging Revealed the Bug
Adding structured debug logs at key points showed:
- Authentication succeeded in middleware
- Handler immediately failed with "authentication required"
- Same timestamp → context propagation issue

### 3. OTLP Client Has Long Connection Timeouts
The OpenTelemetry OTLP client in `shared/go/observability/otel.go`:
- Requires non-empty endpoint (can't disable)
- Takes >60 seconds to timeout on unreachable endpoints
- Has fallback logic but timeouts are too long
- Blocks service startup before logger is available

## Current State

### ✅ Working
- Authentication fix committed and pushed (commit 58cb94d9)
- etcd service deployed and running
- Routing policy created in etcd (`/api-router/policies/global-gpt-oss-20b`)
- CI build successful

### ⚠️ Blocked
- API Router pods crash on startup
- Telemetry initialization blocks for >60 seconds
- No logs produced (logger created after telemetry init)
- Service cannot serve requests

### 📋 Next Steps

**Immediate (Option 1 - Recommended):**
1. Apply `/tmp/otel-collector.yaml` to deploy OTLP collector
2. Update API Router deployment to use `otel-collector.monitoring.svc.cluster.local:4317`
3. Restart API Router pods
4. Verify service starts and serves requests
5. Test end-to-end inference request flow

**Alternative (Option 2 - Code Fix):**
1. Add connection timeouts to `shared/go/observability/otel.go` buildClient()
2. Commit, build new image, deploy
3. Restart pods

**Alternative (Option 3 - Disable OTLP):**
1. Modify `shared/go/observability/otel.go` to allow empty endpoint
2. Use degraded provider immediately
3. Commit, build, deploy

## Test Scenarios Ready

Once service is running, test with:

```bash
# Port forward to service
kubectl port-forward -n development svc/api-router-service-development-api-router-service 8080:8080

# Test inference request with test API key
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: <YOUR_TEST_API_KEY>" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-oss-20b",
    "messages": [{"role": "user", "content": "Hello!"}]
  }'
```

**Expected Flow:**
1. API Router authenticates request using stub validator (test-* prefix)
2. Loads routing policy from etcd (global-gpt-oss-20b)
3. Routes to vLLM backend (vllm-gpt-oss-20b.system.svc.cluster.local:8000)
4. Returns response from vLLM

## Environment Details

- **Cluster:** Linode LKE development cluster
- **Namespace:** development
- **Services:**
  - etcd-service:2379 (running)
  - api-router-service (crashlooping - waiting for OTLP collector)
  - vllm-gpt-oss-20b.system:8000 (running)
  - user-org-service (running)

## Important Context for Next Session

1. **Authentication fix is correct and committed** - Don't revert this
2. **Routing policy is in etcd** - Don't need to recreate
3. **Issue is ONLY telemetry initialization** - Not authentication or routing
4. **OTLP collector manifest is ready** - Just needs to be applied
5. **After OTLP fix, test end-to-end** - Authentication + Routing + Inference

## Commands to Resume

```bash
# 1. Deploy OTLP collector
kubectl apply -f /tmp/otel-collector.yaml

# 2. Update API Router to use collector
kubectl set env deployment/api-router-service-development-api-router-service \
  -n development \
  OTEL_EXPORTER_OTLP_ENDPOINT="otel-collector.monitoring.svc.cluster.local:4317"

# 3. Wait for rollout
kubectl rollout status deployment/api-router-service-development-api-router-service -n development

# 4. Check pod status
kubectl get pods -n development -l app.kubernetes.io/name=api-router-service

# 5. Check logs
kubectl logs -n development -l app.kubernetes.io/name=api-router-service --tail=50

# 6. Test request (if pods are ready)
kubectl port-forward -n development svc/api-router-service-development-api-router-service 8080:8080 &
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "X-API-Key: test-vllm-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "gpt-oss-20b", "messages": [{"role": "user", "content": "Hello!"}]}'
```

## References

- Investigation Document: `/home/dev/ai-aas/docs/API_ROUTER_POD_CRASH_INVESTIGATION.md`
- Middleware Fix: `services/api-router-service/internal/api/public/middleware.go:264`
- Observability Code: `shared/go/observability/otel.go`
- OTLP Collector Manifest: `/tmp/otel-collector.yaml`
- etcd Service Manifest: `/tmp/etcd-service.yaml`
- Routing Policy Seed Job: `/tmp/seed-routing-policy.yaml`
