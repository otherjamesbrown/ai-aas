---
title: Configure Routing Policies
last_updated: 2025-12-14
document_type: runbook
status: active
---

# Configure Routing Policies for API Router

## Overview

The api-router-service requires routing policies to map model requests to backend inference services (vLLM, KServe InferenceServices). This runbook describes how to configure routing policies.

## Architecture

### Policy Storage

Routing policies are stored in:

1. **PostgreSQL Database** (Primary): `routing_policies` table in Admin API database
2. **etcd** (Optional): Distributed cache for real-time updates
3. **Local BoltDB** (Fallback): `/tmp/api-router-config.db` in api-router pods

### Policy Lookup Flow

```
Request → API Router
  ↓
GetPolicy(org_id, model)
  ↓
PolicyCache → ConfigLoader
  ↓
Check cache → Check etcd → Check Admin API
  ↓
Return policy with backend weights
  ↓
Select backend → Forward request
```

### Backend Registry

The `BACKEND_ENDPOINTS` environment variable maps backend IDs to actual service URLs:

```
BACKEND_ENDPOINTS=backend-id:http://service.namespace.svc.cluster.local:port/path
```

**Example:**
```
BACKEND_ENDPOINTS=mistral-7b-instruct:http://mistral-7b-instruct-v03-predictor-00001-private.staging.svc.cluster.local:8012,unsloth/gpt-oss-20b:http://unsloth-gpt-oss-20b-predictor-00001-private.staging.svc.cluster.local:8012
```

## Policy Schema

```sql
CREATE TABLE routing_policies (
    policy_id          VARCHAR(255) PRIMARY KEY,
    organization_id    VARCHAR(255) NOT NULL,  -- '*' for global
    model              VARCHAR(255) NOT NULL,
    backends           JSONB NOT NULL,         -- [{"backend_id": "...", "weight": 100}]
    fallback_backends  JSONB DEFAULT '[]',
    failover_threshold INTEGER DEFAULT 3,
    enabled            BOOLEAN DEFAULT TRUE,
    version            INTEGER DEFAULT 1,
    metadata           JSONB DEFAULT '{}',
    created_at         TIMESTAMP DEFAULT NOW(),
    updated_at         TIMESTAMP DEFAULT NOW(),
    created_by         VARCHAR(255),
    updated_by         VARCHAR(255),
    deleted_at         TIMESTAMP,
    UNIQUE (organization_id, model) WHERE deleted_at IS NULL
);
```

## Creating Routing Policies

### Option 1: Via Admin API (Recommended)

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  "$ADMIN_API_ENDPOINT/v1/routing/policies" \
  -d '{
    "model": "mistral-7b-instruct-v03",
    "organization_id": "*",
    "backends": [
      {
        "backend_id": "mistral-7b-instruct",
        "weight": 100
      }
    ],
    "failover_threshold": 1,
    "enabled": true
  }'
```

### Option 2: Direct Database Insert

```sql
INSERT INTO routing_policies (
    policy_id,
    organization_id,
    model,
    backends,
    fallback_backends,
    failover_threshold,
    enabled,
    version,
    metadata,
    created_at,
    updated_at,
    created_by,
    updated_by
) VALUES (
    gen_random_uuid()::text,
    '*',  -- Global policy
    'mistral-7b-instruct-v03',
    '[{"backend_id": "mistral-7b-instruct", "weight": 100}]'::jsonb,
    '[]'::jsonb,
    1,
    true,
    1,
    '{"environment": "staging"}'::jsonb,
    NOW(),
    NOW(),
    'system-setup',
    'system-setup'
) ON CONFLICT (organization_id, model) WHERE deleted_at IS NULL
DO UPDATE SET
  backends = EXCLUDED.backends,
  enabled = EXCLUDED.enabled,
  updated_at = NOW(),
  version = routing_policies.version + 1;
```

### Option 3: Using psql Client Pod

```bash
kubectl run -n admin-api-service psql-client \
  --image=postgres:15 --rm -i --restart=Never \
  --command -- psql "$DATABASE_URL" <<'EOF'
-- Your SQL here
EOF
```

## Backend ID Alignment

**CRITICAL**: The `backend_id` in routing policies MUST match the backend ID in `BACKEND_ENDPOINTS`.

### Check Current Backend Endpoints

```bash
kubectl get deployment -n $NAMESPACE api-router-service-$ENV-api-router-service \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="BACKEND_ENDPOINTS")].value}'
```

Example output:
```
mistral-7b-instruct:http://...:8012,unsloth/gpt-oss-20b:http://...:8012
```

Then create policies with matching backend_ids:
- Policy for `mistral-7b-instruct-v03` → backend_id: `mistral-7b-instruct`
- Policy for `unsloth/gpt-oss-20b` → backend_id: `unsloth/gpt-oss-20b`

## Policy Sync

The api-router-service syncs policies from the Admin API at a configurable interval:

```yaml
env:
  - name: ADMIN_API_ENDPOINT
    value: http://admin-api-service-$ENV.admin-api-service.svc.cluster.local:8080
  - name: ADMIN_API_SYNC_INTERVAL
    value: 30s
```

Policies are automatically synced every 30 seconds (default).

## Organization-Specific vs Global Policies

### Policy Lookup Order

1. **Organization-specific**: `organization_id = <actual org id>`
2. **Global fallback**: `organization_id = '*'`

### Best Practice

Create **both** global and org-specific policies:

```sql
-- Global policy (fallback for all orgs)
INSERT INTO routing_policies (...) VALUES ('*', 'model-name', ...);

-- Org-specific policy (if needed)
INSERT INTO routing_policies (...) VALUES ('org-uuid', 'model-name', ...);
```

## Verification

### 1. Check policies in database

```sql
SELECT policy_id, organization_id, model, backends, enabled
FROM routing_policies
WHERE deleted_at IS NULL
ORDER BY organization_id, model;
```

### 2. Check api-router logs

```bash
kubectl logs -n $NAMESPACE -l app.kubernetes.io/name=api-router-service --tail=100 | grep policy
```

**Success indicators:**
- No "no routing policy found" errors
- Backend health checks appearing in logs
- `backend marked as degraded/healthy` messages

### 3. Test inference request

```bash
curl -X POST \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  "$API_ROUTER_ENDPOINT/v1/chat/completions" \
  -d '{
    "model": "mistral-7b-instruct-v03",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

## Troubleshooting

### Error: "no routing policy configured"

**Symptoms:**
```json
{"level":"warn","msg":"no routing policy found","org_id":"...","model":"..."}
{"level":"warn","msg":"request error","status":500,"code":"ROUTING_ERROR"}
```

**Causes:**
1. No policy exists for the model + organization combination
2. Policy exists but backend_id doesn't match BACKEND_ENDPOINTS
3. api-router hasn't synced policies yet (wait up to 30s)

**Solutions:**
1. Create policy for the specific organization_id (not just global '*')
2. Verify backend_id matches: `kubectl get deployment ... -o yaml | grep BACKEND_ENDPOINTS`
3. Wait for sync interval or restart api-router pods

### Error: "backend not found"

**Symptoms:**
```
{"level":"error","msg":"backend not found","backend_id":"..."}
```

**Cause:** Backend ID in policy doesn't exist in BACKEND_ENDPOINTS

**Solution:** Update policy or BACKEND_ENDPOINTS to match

### Backends showing as unhealthy (502)

**Symptoms:**
```
{"level":"warn","msg":"backend marked as unhealthy","backend_id":"...","error":"status 502"}
```

**Cause:** This is NOT a routing policy issue - the vLLM/KServe pods are not ready

**Solution:** Investigate pod status and readiness (different issue, likely ai-aas-e6e3 or similar)

## Related Beads

- `ai-aas-f3ju`: Configure routing policies in staging (this implementation)
- `ai-aas-e6e3`: vLLM pod stability issues
- `ai-aas-fkm6`: guidellm-runner benchmarks failing

## See Also

- `/services/api-router-service/internal/config/loader.go` - Policy loading logic
- `/services/api-router-service/internal/routing/policy_cache.go` - Policy caching
- `/services/admin-api-service/internal/domain/policy.go` - Policy domain model
- `/services/admin-api-service/internal/repository/policies.go` - Policy repository
