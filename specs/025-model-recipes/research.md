# Research: Model Recipes

**Feature**: 025-model-recipes
**Date**: 2025-12-14

## Technical Decisions

### 1. Recipe Scope: Cluster-scoped vs Namespaced

**Decision**: Cluster-scoped (stored in `ai-model-system` namespace)

**Rationale**:
- Recipes are shared knowledge - same recipe should work across all environments
- Avoids duplication of recipes per namespace
- Operators can still deploy AIModels to any namespace using recipes
- Follows KServe pattern where ClusterServingRuntimes are cluster-scoped

**Alternatives Considered**:
- Namespaced: Would require duplicating recipes per environment, defeating the purpose
- Both: Too complex, unclear which takes precedence

### 2. Recipe Storage Location

**Decision**: GitOps in `infra/model-recipes/` directory

**Rationale**:
- Consistent with GitOps-first principle
- Version controlled with full history
- ArgoCD can sync recipes to cluster
- Easy to review changes via PR

**Structure**:
```
infra/model-recipes/
├── llm/
│   ├── mistral/
│   ├── llama/
│   └── qwen/
├── vision/
│   ├── florence/
│   └── sam/
└── multimodal/
```

### 3. Override Merge Strategy

**Decision**: Deep merge with explicit override semantics

**Rationale**:
- AIModel.spec.overrides deep-merges with recipe
- Explicit `null` can remove a field from recipe
- Arrays are replaced, not merged (consistent with Kubernetes strategic merge)

**Example**:
```yaml
# Recipe has: runtimeArgs.vllm.maxModelLen: 8192
# AIModel overrides: runtimeArgs.vllm.gpuMemoryUtilization: 0.85
# Result: Both maxModelLen AND gpuMemoryUtilization are set
```

### 4. Runtime Support Order

**Decision**: Phase 1 vLLM only, Phase 2 Triton, Phase 3 TGI

**Rationale**:
- vLLM is currently used for all deployed models
- Triton needed for vision models (Florence, SAM)
- TGI is lower priority, only for models that perform better with it

### 5. Recipe Versioning

**Decision**: Git-based versioning (no semver in CRD)

**Rationale**:
- Git provides version history naturally
- Recipe updates are breaking by nature (any change affects deployments)
- Pinning to specific commits can be done via ArgoCD targetRevision
- Simpler than maintaining semver in the CRD

**Alternative Considered**:
- SemVer in CRD metadata: Adds complexity, hard to enforce, still need Git

### 6. Backward Compatibility

**Decision**: AIModel supports both `recipeRef` and inline spec

**Rationale**:
- Existing AIModels with inline specs continue to work
- New AIModels should use `recipeRef`
- Operator applies recipe first, then inline spec as overrides
- Deprecation warning for inline-only AIModels

### 7. Recipe Validation

**Decision**: Validate at apply-time in operator, not admission webhook

**Rationale**:
- Simpler implementation
- Recipes are created by platform operators, not end users
- Validation can be more comprehensive in reconcile loop
- Can validate against cluster state (e.g., GPU availability)

### 8. CLI Command Structure

**Decision**: Add `ai-aas-cli model recipe` subcommand group

**Commands**:
```bash
ai-aas-cli model recipe list [--runtime vllm|triton|tgi] [--task text-generation|image-captioning]
ai-aas-cli model recipe show <name>
ai-aas-cli model recipe validate <file>
ai-aas-cli model deploy create --recipe <name> [-e environment]
```

**Rationale**:
- Follows existing CLI structure (`ai-aas-cli model ...`)
- `recipe` as subcommand group keeps commands organized
- `deploy create --recipe` extends existing deploy command

## Dependencies

### Existing Components to Modify

1. **AI Model Operator** (`operators/ai-model-operator/`)
   - Add ModelRecipe CRD and types
   - Update controller to resolve recipes
   - Add recipe validation

2. **Admin API Service** (`services/admin-api-service/`)
   - Add recipe endpoints (CRUD, list, validate)
   - Add recipe-to-deployment mapping endpoint

3. **CLI** (`services/ai-aas-cli/`)
   - Add `model recipe` subcommands
   - Update `model deploy create` to support `--recipe`

### New Components

1. **ModelRecipe CRD** (`operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`)
2. **Recipe Library** (`infra/model-recipes/`)
3. **ArgoCD Application** (`gitops/clusters/development/apps/model-recipes.yaml`)

## Open Questions Resolved

| Question | Resolution |
|----------|------------|
| Namespaced vs cluster-scoped? | Cluster-scoped in `ai-model-system` |
| How to handle versioning? | Git-based, no semver in CRD |
| Auto-generate from HuggingFace? | Future work (Phase 4) |
| Private model recipes? | Use separate namespace + RBAC (future) |
| Recipe deletion policy? | Prevent deletion if AIModels reference it (see below) |
| Re-reconcile AIModels on recipe change? | Yes, controller watches recipes (see below) |

### 9. Recipe Deletion Policy

**Decision**: Prevent deletion if AIModels reference the recipe

**Rationale**:
- Deleting a recipe that AIModels depend on would break those deployments
- Operator adds finalizer to ModelRecipe when referenced by any AIModel
- Finalizer removed only when no AIModels reference the recipe
- Admin API returns 409 Conflict with list of dependent AIModels

**User Experience**:
```bash
$ kubectl delete modelrecipe llama-7b-vllm
Error: modelrecipe "llama-7b-vllm" is referenced by 3 AIModel(s): [llama-prod, llama-staging, llama-test]
```

### 10. Recipe Update Reconciliation

**Decision**: Re-reconcile AIModels when referenced recipe changes

**Rationale**:
- Recipe changes should propagate to running deployments
- Operator controller watches ModelRecipe resources
- When recipe changes, controller finds AIModels with matching `recipeRef`
- Each dependent AIModel is re-reconciled to apply new recipe values
- Status updated to show recipe version hash for tracking

**Implementation**:
- Controller uses `handler.EnqueueRequestsFromMapFunc` to map recipe changes to AIModels
- Recipe includes `status.lastAppliedHash` to track applied version
- AIModel status shows which recipe version is currently deployed
