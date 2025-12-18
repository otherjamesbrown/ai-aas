# Implementation Plan: Model Recipes

**Feature Branch**: `025-model-recipes`
**Date**: 2025-12-14
**Spec**: [spec.md](./spec.md)

## Summary

Implement a ModelRecipe CRD that captures known-good baseline configurations for AI model deployments. AIModel instances reference recipes and can override settings for experimentation. This enables consistent deployments, hardware testing, and runtime comparison (vLLM vs Triton vs TGI).

## Technical Context

- **Language**: Go 1.22
- **Framework**: Kubebuilder for operator, Chi for Admin API
- **Dependencies**:
  - `sigs.k8s.io/controller-runtime` (operator)
  - Existing Admin API client in `internal/api`
- **Storage**: PostgreSQL (recipes table), Kubernetes (CRDs)
- **Testing**: Go testing, envtest for operator

## Constitution Compliance

| Principle | Compliance |
|-----------|------------|
| API-First | Admin API endpoints first, CLI uses API client |
| GitOps-First | Recipes stored in Git, synced via ArgoCD |
| CLI-First | All operations available via CLI |

## Project Structure

### Files to Create

```
operators/ai-model-operator/
├── api/v1alpha1/
│   └── modelrecipe_types.go          # New CRD types
├── internal/
│   └── recipe/
│       ├── resolver.go               # Recipe resolution logic
│       └── merger.go                 # Deep merge for overrides

services/admin-api-service/
├── internal/
│   ├── handlers/
│   │   └── recipes.go                # Recipe HTTP handlers
│   ├── repository/
│   │   └── recipes.go                # Recipe database ops
│   └── service/
│       └── recipes.go                # Recipe business logic

services/ai-aas-cli/
├── cmd/
│   └── model/
│       └── recipe/
│           ├── list.go               # List recipes
│           ├── show.go               # Show recipe details
│           └── validate.go           # Validate recipe file

infra/model-recipes/
├── llm/
│   └── mistral/
│       └── mistral-7b-instruct-v03.yaml
├── vision/
└── README.md

gitops/clusters/development/apps/
└── model-recipes.yaml                # ArgoCD Application
```

### Files to Modify

```
operators/ai-model-operator/
├── api/v1alpha1/
│   ├── aimodel_types.go              # Add RecipeRef, Overrides
│   └── zz_generated.deepcopy.go      # Regenerate
├── internal/
│   └── controller/
│       └── aimodel_controller.go     # Use recipe resolver

services/admin-api-service/
├── internal/
│   └── router/
│       └── router.go                 # Add recipe routes
├── cmd/
│   └── migrate/
│       └── migrations/               # Add recipes table

services/ai-aas-cli/
├── cmd/
│   └── model/
│       ├── deploy.go                 # Add --recipe flag
│       └── model.go                  # Add recipe subcommand
```

## Implementation Phases

### Phase 1: Recipe CRD and Operator Support (P1)

**Goal**: Define ModelRecipe CRD and update operator to resolve recipes

1. Create `ModelRecipe` CRD types
2. Update `AIModel` CRD with `recipeRef` and `overrides`
3. Implement recipe resolver in operator
4. Implement deep merge for overrides
5. Update operator controller to use recipes
6. Add validation for recipes

### Phase 2: Admin API and Database (P1)

**Goal**: Add recipe storage and REST API

1. Create recipes database migration
2. Implement recipe repository (CRUD)
3. Add recipe HTTP handlers
4. Add recipe routes to router
5. Implement recipe-to-deployment mapping

### Phase 3: CLI Commands (P1)

**Goal**: Add recipe management CLI commands

1. Add `model recipe list` command
2. Add `model recipe show` command
3. Add `model recipe validate` command
4. Update `model deploy create` with `--recipe` flag

### Phase 4: Recipe Library and GitOps (P2)

**Goal**: Create initial recipe library and ArgoCD sync

1. Create recipe library directory structure
2. Create recipes for existing models (gpt-oss-20b, mistral-7b)
3. Create ArgoCD Application for recipes
4. Migrate existing AIModels to use recipes

### Phase 5: Triton Runtime Support (P2)

**Goal**: Add Triton support for vision models

1. Add Triton InferenceService builder to operator
2. Create Triton-specific recipe validation
3. Create vision model recipes (Florence)
4. Test Triton deployments

### Phase 6: TGI Runtime Support (P3)

**Goal**: Add TGI support

1. Add TGI container builder to operator
2. Create TGI-specific recipe validation
3. Create TGI recipes for compatible models
4. Test TGI deployments

## API Endpoints

### Admin API

```
GET    /api/v1/recipes                     # List all recipes
GET    /api/v1/recipes/:name               # Get recipe by name
POST   /api/v1/recipes                     # Create recipe
PUT    /api/v1/recipes/:name               # Update recipe
DELETE /api/v1/recipes/:name               # Delete recipe
POST   /api/v1/recipes/:name/validate      # Validate recipe
GET    /api/v1/recipes/:name/deployments   # List deployments using recipe
```

### Request/Response Examples

**List Recipes**
```json
GET /api/v1/recipes?runtime=vllm

{
  "recipes": [
    {
      "name": "mistral-7b-instruct-v03",
      "displayName": "Mistral 7B Instruct v0.3",
      "modelID": "mistralai/Mistral-7B-Instruct-v0.3",
      "runtime": "vllm",
      "deploymentCount": 3
    }
  ]
}
```

**Get Recipe**
```json
GET /api/v1/recipes/mistral-7b-instruct-v03

{
  "name": "mistral-7b-instruct-v03",
  "displayName": "Mistral 7B Instruct v0.3",
  "modelID": "mistralai/Mistral-7B-Instruct-v0.3",
  "runtime": "vllm",
  "resources": {
    "gpu": {"vendor": "nvidia", "count": 1, "minMemoryGB": 16},
    "cpu": {"requests": "4", "limits": "8"},
    "memory": {"requests": "16Gi", "limits": "24Gi"}
  },
  "runtimeArgs": {
    "vllm": {
      "dtype": "auto",
      "maxModelLen": 8192,
      "gpuMemoryUtilization": "0.9",
      "tokenizerMode": "mistral"
    }
  }
}
```

## Testing Strategy

### Unit Tests
- Recipe type validation
- Deep merge logic
- Recipe resolver
- CLI command parsing

### Integration Tests
- Operator reconciles AIModel with recipe
- Admin API CRUD operations
- CLI commands work end-to-end

### E2E Tests
- Deploy model using recipe
- Override recipe settings
- Recipe validation catches errors

## Risks and Mitigations

| Risk | Mitigation |
|------|------------|
| Breaking existing AIModels | Backward compatible - inline spec still works |
| Complex merge logic | Well-defined merge rules, comprehensive tests |
| Recipe drift from actual deployments | Recipes are source of truth, GitOps enforces |

## Dependencies

- Existing AIModel CRD and operator
- Admin API service
- CLI framework
- ArgoCD for GitOps

## Success Criteria

1. Can deploy a model using `ai-aas-cli model deploy create --recipe <name>`
2. Can override recipe settings in AIModel
3. Existing AIModels continue to work
4. Recipes are version controlled in Git
5. ArgoCD syncs recipes to cluster
