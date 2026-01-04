# Routing Policy Semantics

## Overview

Routing policies control which organizations can access which models via the inference API. This document explains the semantic behavior of policies, particularly wildcard policies.

## Policy Resolution

When checking if an organization has access to a model, the system uses the following logic:

```sql
SELECT EXISTS(
  SELECT 1 FROM routing_policies
  WHERE (organization_id = $org_id OR organization_id = '*')
  AND model = $model_name
  AND deleted_at IS NULL
)
```

**Key behavior**: Access is granted if EITHER:
1. An org-specific policy exists (`organization_id = <uuid>`)
2. A wildcard policy exists (`organization_id = '*'`)

## Wildcard Policies

### What They Are

A wildcard routing policy has `organization_id = '*'` (literal asterisk string). This grants access to ALL organizations for the specified model.

### Use Cases

| Use Case | Description |
|----------|-------------|
| Platform-wide models | Default models available to all organizations |
| New deployments | Auto-created policies for newly deployed models |
| Open access tier | Models that don't require per-org access control |

### Creation

Wildcard policies are created in two ways:

1. **Automatic**: When a model is deployed via the operator or Admin API, a wildcard policy is auto-created
2. **Manual**: Via CLI with `--global` flag:
   ```bash
   ai-aas-cli routing policy create --global --model gpt-4 --backends backend-1:100
   ```

### Important: Wildcard Grants Universal Access

Once a wildcard policy exists for a model, ALL organizations can access that model. There is no way to "exclude" specific organizations while keeping the wildcard.

**To restrict access to a model:**
1. Do NOT create a wildcard policy
2. Create org-specific policies only for authorized organizations

```yaml
# This grants access to ALL orgs:
organization_id: "*"
model: "premium-model"

# This grants access to ONLY org-123:
organization_id: "org-123-uuid"
model: "premium-model"
```

## Policy Precedence

When resolving which backend to route to (not just access check), the API Router uses this fallback chain:

1. **Org-specific policy** - Highest priority
2. **Wildcard policy** - Fallback
3. **Registry discovery** - Last resort (ephemeral)

This means org-specific policies can override routing (e.g., premium tier to dedicated backends) while still allowing wildcard for access.

## Access Check vs Routing

| Check | Query | Purpose |
|-------|-------|---------|
| `HasModelAccess` | org OR wildcard | Binary yes/no for access control |
| `GetPolicy` | org-first, then wildcard | Which backend to route to |

The `HasModelAccess` function (used by benchmarks, model registry) returns `true` if ANY matching policy exists. It does not distinguish between org-specific and wildcard.

## Code Reference

The access check is implemented in:
- **Repository**: `services/admin-api-service/internal/repository/policies.go:HasModelAccess()`
- **Usage**: Benchmark target creation, model validation

## Related Documentation

- [Platform Routing Policies](../platform/routing-policies.md) - CLI commands and workflows
- [Inference Routing Architecture](./inference-routing.md) - Backend routing details
- [Model Access Control](../platform/model-access-control.md) - Full access control guide
