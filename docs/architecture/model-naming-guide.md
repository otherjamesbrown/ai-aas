# Model Naming Guide

This document explains all the different model naming fields in the AI-AAS platform, how they relate to each other, and how to ensure they stay aligned.

## The Golden Thread: `spec.modelID`

**The `spec.modelID` field in the AIModel CRD is the single source of truth.** All other naming fields are derived from or must align with this value.

```
AIModel.spec.modelID = "unsloth/gpt-oss-20b"  ← GOLDEN THREAD
         ↓
    Everything else derives from this
```

## Complete Naming Field Reference

### 1. AIModel CRD (Kubernetes)

| Field | Purpose | Example | Derivation |
|-------|---------|---------|------------|
| `metadata.name` | K8s resource name | `unsloth-gpt-oss-20b` | Manual (DNS-1123 safe version of modelID) |
| `spec.modelName` | Legacy internal name | `unsloth-gpt-oss-20b` | Usually same as metadata.name |
| `spec.modelID` | **GOLDEN THREAD** - Full HF path | `unsloth/gpt-oss-20b` | Manual - this is the source |
| `spec.externalName` | OpenAI API model name | `gpt-oss-20b` | Auto-derived from modelID if not set |

**Auto-derivation of externalName:**
```
modelID: "unsloth/gpt-oss-20b"  →  externalName: "gpt-oss-20b"
modelID: "meta-llama/Llama-3.1-8B-Instruct"  →  externalName: "Llama-3.1-8B-Instruct"
```

### 2. Database Tables

#### model_registry
| Column | Maps To | Example |
|--------|---------|---------|
| `name` | AIModel metadata.name | `unsloth-gpt-oss-20b` |
| `hf_model_id` | AIModel spec.modelID | `unsloth/gpt-oss-20b` |
| `external_name` | AIModel spec.externalName | `gpt-oss-20b` |

#### routing_policies
| Column | Maps To | Example |
|--------|---------|---------|
| `model` | AIModel spec.modelID | `unsloth/gpt-oss-20b` |
| `external_name` | JOINed from model_registry | `gpt-oss-20b` |

### 3. API Router

| Config | Maps To | Example |
|--------|---------|---------|
| `backends.endpoints` key | spec.modelID | `unsloth/gpt-oss-20b:http://...` |
| Policy lookup | spec.modelID or externalName | Resolves `gpt-oss-20b` → `unsloth/gpt-oss-20b` |

### 4. vLLM Backend

| Argument | Maps To | Example |
|----------|---------|---------|
| `--model` | spec.modelID | `--model=unsloth/gpt-oss-20b` |
| `--served-model-name` | spec.modelID | `--served-model-name=unsloth/gpt-oss-20b` |

## Data Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        AIModel CR (Source of Truth)                         │
│  ┌─────────────────────────────────────────────────────────────────────┐   │
│  │ metadata.name: unsloth-gpt-oss-20b                                  │   │
│  │ spec.modelID: unsloth/gpt-oss-20b        ← GOLDEN THREAD            │   │
│  │ spec.externalName: gpt-oss-20b           ← derived if not set       │   │
│  └─────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
┌──────────────────────┐  ┌─────────────────┐  ┌────────────────────┐
│   model_registry     │  │ InferenceService│  │  vLLM Container    │
│ ─────────────────────│  │ ────────────────│  │ ───────────────────│
│ name: unsloth-gpt-...│  │ name: unsloth-  │  │ --model=unsloth/   │
│ hf_model_id: unsloth/│  │   gpt-oss-20b   │  │   gpt-oss-20b      │
│   gpt-oss-20b        │  │                 │  │ --served-model-    │
│ external_name:       │  │                 │  │   name=unsloth/    │
│   gpt-oss-20b        │  │                 │  │   gpt-oss-20b      │
└──────────────────────┘  └─────────────────┘  └────────────────────┘
          │
          ▼
┌──────────────────────┐
│  routing_policies    │
│ ─────────────────────│
│ model: unsloth/      │  ← Must match hf_model_id
│   gpt-oss-20b        │
│ external_name:       │  ← JOINed from model_registry
│   gpt-oss-20b        │
└──────────────────────┘
          │
          ▼
┌──────────────────────┐
│    API Router        │
│ ─────────────────────│
│ /v1/models returns:  │
│   id: gpt-oss-20b    │  ← Uses external_name
│                      │
│ Chat completions:    │
│   model: gpt-oss-20b │  ← Maps to unsloth/gpt-oss-20b
└──────────────────────┘
```

## How Alignment is Maintained

### Automatic Sync (Operator → Admin API)

The operator sends `modelID` and `externalName` to the Admin API on **every reconciliation**:

```go
// On every status update, not just creation
r.AdminAPIClient.UpdateDeploymentStatus(ctx, aiModel.Name, environment, adminapi.DeploymentStatus{
    Status:       deploymentStatus,
    ModelID:      aiModel.Spec.ModelID,      // ← Always sent
    ExternalName: deriveExternalName(aiModel), // ← Always sent
    ...
})
```

The Admin API then updates `model_registry` if the values differ:

```go
if req.ModelID != "" && model.HFModelID != req.ModelID {
    s.updateModelHFModelID(ctx, model.ID, req.ModelID)
}
if req.ExternalName != "" && model.ExternalName != req.ExternalName {
    s.updateModelExternalName(ctx, model.ID, req.ExternalName)
}
```

### SQL JOINs (routing_policies → model_registry)

Routing policies get `external_name` via JOIN:

```sql
SELECT rp.*, m.external_name
FROM routing_policies rp
LEFT JOIN model_registry m ON rp.model = m.hf_model_id
```

This ensures routing policies always have the current `external_name` from the model registry.

## What Can Go Wrong

### 1. Stale model_registry Data
**Symptom:** Old deployments have wrong `hf_model_id` or `external_name`
**Fix:** Trigger operator reconciliation: `kubectl annotate aimodel <name> reconcile-trigger=$(date +%s) --overwrite`

### 2. Routing Policy Not Found
**Symptom:** "no routing policy configured" error
**Cause:** `routing_policies.model` doesn't match `model_registry.hf_model_id`
**Fix:** Ensure routing policy uses full HF path (e.g., `unsloth/gpt-oss-20b`)

### 3. External Name Collision
**Symptom:** Two models with same external_name
**Cause:** Auto-derivation creates duplicates (e.g., `org1/model` and `org2/model` both derive to `model`)
**Fix:** Set explicit `spec.externalName` with unique values

### 4. vLLM Model Mismatch
**Symptom:** Backend returns different model name in response
**Cause:** `--served-model-name` doesn't match routing policy
**Fix:** Ensure vLLM's `--served-model-name` equals `spec.modelID`

## Validation Checklist

When adding a new model, verify alignment:

```bash
# 1. Check AIModel CR
kubectl get aimodel <name> -o jsonpath='{.spec.modelID}'
kubectl get aimodel <name> -o jsonpath='{.spec.externalName}'

# 2. Check model_registry (via Admin API)
curl -H "Authorization: Bearer $ADMIN_API_KEY" \
  "$ADMIN_API/v1/models" | jq '.[] | {name, hf_model_id, external_name}'

# 3. Check routing policy exists
# Policy.model should equal spec.modelID

# 4. Check /v1/models returns external_name
curl -H "X-API-Key: $API_KEY" \
  "$API_ROUTER/v1/models" | jq '.data[].id'

# 5. Test chat completion with external_name
curl -X POST -H "X-API-Key: $API_KEY" \
  "$API_ROUTER/v1/chat/completions" \
  -d '{"model":"<external_name>","messages":[...]}'
```

## Best Practices

### 1. Always Set `spec.modelID` to Full HF Path
```yaml
spec:
  modelID: unsloth/gpt-oss-20b  # GOOD - full path
  # modelID: gpt-oss-20b        # BAD - missing org
```

### 2. Use Explicit `externalName` for User-Friendly Names
```yaml
spec:
  modelID: meta-llama/Llama-3.1-8B-Instruct
  externalName: llama-3.1-8b  # User-friendly name
```

### 3. Create Routing Policies with Full Path
```yaml
# routing policy
model: unsloth/gpt-oss-20b  # Must match spec.modelID
```

### 4. Configure API Router Backends with Full Path
```yaml
backends:
  endpoints: "unsloth/gpt-oss-20b:http://..."  # Must match spec.modelID
```

## Summary

| What Users See | What Internal Systems Use | Source of Truth |
|----------------|---------------------------|-----------------|
| `/v1/models` returns `gpt-oss-20b` | Routing uses `unsloth/gpt-oss-20b` | `spec.modelID` |
| Chat request: `model: gpt-oss-20b` | Maps to `unsloth/gpt-oss-20b` | `spec.modelID` |
| Response: `model: unsloth/gpt-oss-20b` | vLLM's served-model-name | `spec.modelID` |

**The golden thread is `spec.modelID`.** Everything else derives from it or must align with it.
