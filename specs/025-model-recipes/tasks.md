# Tasks: Model Recipes

**Feature**: `025-model-recipes`
**Generated**: 2025-12-14
**Source**: plan.md, spec.md

## Phase 1: Setup & Prerequisites

- [ ] T001 [P] Create recipe package directory structure in `operators/ai-model-operator/internal/recipe/`
- [ ] T002 [P] Create recipe library directory structure at `infra/model-recipes/`

## Phase 2: ModelRecipe CRD (blocks Phase 3-4)

- [ ] T003 Define ModelRecipe CRD types in `operators/ai-model-operator/api/v1alpha1/modelrecipe_types.go`
- [ ] T004 Add RecipeRef and Overrides to AIModel in `operators/ai-model-operator/api/v1alpha1/aimodel_types.go`
- [ ] T005 Run `make generate` to regenerate deepcopy in `operators/ai-model-operator/`
- [ ] T006 Run `make manifests` to generate CRD YAML in `operators/ai-model-operator/`

## Phase 3: Operator Recipe Support

### Tests
- [ ] T007 Write unit tests for recipe resolver in `operators/ai-model-operator/internal/recipe/resolver_test.go`
- [ ] T008 Write unit tests for deep merge in `operators/ai-model-operator/internal/recipe/merger_test.go`

### Implementation
- [ ] T009 Implement recipe resolver in `operators/ai-model-operator/internal/recipe/resolver.go`
- [ ] T010 Implement deep merge for overrides in `operators/ai-model-operator/internal/recipe/merger.go`
- [ ] T011 Implement recipe validation in `operators/ai-model-operator/internal/recipe/validator.go`
- [ ] T012 Update AIModel controller to use recipe resolver in `operators/ai-model-operator/internal/controller/aimodel_controller.go`

## Phase 4: Admin API

### Database
- [ ] T013 Create recipes table migration in `services/admin-api-service/migrations/`

### Tests
- [ ] T014 [P] Write unit tests for recipe repository in `services/admin-api-service/internal/repository/recipes_test.go`
- [ ] T015 [P] Write unit tests for recipe service in `services/admin-api-service/internal/service/recipes_test.go`
- [ ] T016 [P] Write unit tests for recipe handlers in `services/admin-api-service/internal/handlers/recipes_test.go`

### Implementation
- [ ] T017 Implement recipe repository in `services/admin-api-service/internal/repository/recipes.go`
- [ ] T018 Implement recipe service in `services/admin-api-service/internal/service/recipes.go`
- [ ] T019 Implement recipe HTTP handlers in `services/admin-api-service/internal/handlers/recipes.go`
- [ ] T020 Add recipe routes to router in `services/admin-api-service/internal/router/router.go`
- [ ] T037 Implement recipe deployments endpoint in `services/admin-api-service/internal/handlers/recipes.go` (GET /api/v1/recipes/:name/deployments)

## Phase 5: CLI Commands

### Tests
- [ ] T021 [P] Write tests for recipe list command in `services/ai-aas-cli/cmd/model/recipe/list_test.go`
- [ ] T022 [P] Write tests for recipe show command in `services/ai-aas-cli/cmd/model/recipe/show_test.go`
- [ ] T023 [P] Write tests for recipe validate command in `services/ai-aas-cli/cmd/model/recipe/validate_test.go`

### Implementation
- [ ] T024 Add recipe subcommand to model in `services/ai-aas-cli/cmd/model/model.go`
- [ ] T025 Implement recipe list command in `services/ai-aas-cli/cmd/model/recipe/list.go`
- [ ] T026 Implement recipe show command in `services/ai-aas-cli/cmd/model/recipe/show.go`
- [ ] T027 Implement recipe validate command in `services/ai-aas-cli/cmd/model/recipe/validate.go`
- [ ] T028 Add --recipe flag to deploy create in `services/ai-aas-cli/cmd/model/deploy.go`

## Phase 6: Recipe Library & GitOps

- [ ] T029 Create gpt-oss-20b recipe in `infra/model-recipes/llm/openai/gpt-oss-20b.yaml`
- [ ] T030 Create mistral-7b recipe in `infra/model-recipes/llm/mistral/mistral-7b-instruct-v03.yaml`
- [ ] T031 Create ArgoCD Application in `gitops/clusters/development/apps/model-recipes.yaml`
- [ ] T032 Create README for recipe library in `infra/model-recipes/README.md`

## Phase 7: Integration & Polish

- [ ] T033 Write integration tests for operator recipe resolution in `operators/ai-model-operator/internal/controller/aimodel_controller_test.go`
- [ ] T034 Write E2E test for deploying model with recipe
- [ ] T035 Update operator README with recipe documentation in `operators/ai-model-operator/README.md`
- [ ] T036 Update CLI README with recipe commands in `services/ai-aas-cli/README.md`

## Dependencies

```
T003 → T004 → T005 → T006 (CRD definition chain)
T006 → T007, T008 (CRD must exist before tests)
T007, T008 → T009, T010 (tests before implementation)
T009, T010 → T011 → T012 (resolver/merger before validator before controller)
T013 → T014, T015, T016 (migration before tests)
T014, T015, T016 → T017, T018, T019 (tests before implementation)
T017 → T018 → T019 → T020 → T037 (repo → service → handler → router → deployment mapping)
T020 → T021, T022, T023 (API must exist before CLI tests)
T021, T022, T023 → T024, T025, T026, T027, T028 (tests before implementation)
T006 → T029, T030, T031 (CRD must exist before recipes)
T012, T020, T028 → T033, T034 (all components before integration tests)
```

## Summary

| Phase | Tasks | Parallel |
|-------|-------|----------|
| Setup | T001-T002 | 2 |
| CRD | T003-T006 | 0 |
| Operator | T007-T012 | 2 |
| Admin API | T013-T020, T037 | 3 |
| CLI | T021-T028 | 3 |
| GitOps | T029-T032 | 4 |
| Integration | T033-T036 | 0 |

**Total**: 37 tasks
**Parallel opportunities**: 14 tasks can run concurrently (within phases)
