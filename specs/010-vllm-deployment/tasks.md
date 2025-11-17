# Tasks: Model Inference Deployment

**Input**: Design documents from `/specs/010-vllm-deployment/`
**Prerequisites**: Kubernetes cluster (LKE) with GPU node pool, Helm 3.x, ArgoCD, PostgreSQL database, kubectl access
**Tests**: Helm chart linting, Kubernetes manifest validation, integration tests (deploy → verify), E2E tests (deployment → registration → routing)
**Organization**: Tasks grouped by setup, foundational work, user stories, then polish; all user story tasks carry `[US#]`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish Helm chart structure, directories, and development tooling

- [ ] T-S010-P01-001 Create Helm chart directory structure in `infra/helm/charts/vllm-deployment/` (Chart.yaml, values.yaml, templates/, values-{env}.yaml files)
- [ ] T-S010-P01-002 [P] Initialize Helm chart metadata in `infra/helm/charts/vllm-deployment/Chart.yaml` (name, version, description, dependencies)
- [ ] T-S010-P01-003 [P] Create base values.yaml with default configuration in `infra/helm/charts/vllm-deployment/values.yaml`
- [ ] T-S010-P01-004 [P] Create environment-specific values files in `infra/helm/charts/vllm-deployment/values-development.yaml`, `values-staging.yaml`, `values-production.yaml`
- [ ] T-S010-P01-005 [P] Create template directory structure in `infra/helm/charts/vllm-deployment/templates/` (deployment.yaml, service.yaml, configmap.yaml, networkpolicy.yaml, servicemonitor.yaml)
- [ ] T-S010-P01-006 [P] Add Helm chart linting to CI workflow in `.github/workflows/vllm-deployment.yml`
- [ ] T-S010-P01-007 [P] Create database migration directory structure in `db/migrations/operational/` for deployment metadata schema changes

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T-S010-P02-008 Create database migration to extend model_registry_entries table in `db/migrations/operational/20250127120000_add_deployment_metadata.up.sql` (add deployment_endpoint, deployment_status, deployment_environment, deployment_namespace, last_health_check_at columns with constraints and indexes)
- [ ] T-S010-P02-009 Create down migration for rollback in `db/migrations/operational/20250127120000_add_deployment_metadata.down.sql`
- [ ] T-S010-P02-010 [P] Create base Helm Deployment template with vLLM container configuration in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (image, resources, nodeSelector for GPU pool, basic pod spec)
- [ ] T-S010-P02-011 [P] Create Kubernetes Service template with predictable naming in `infra/helm/charts/vllm-deployment/templates/service.yaml` (format: {model-name}-{environment}.{namespace}.svc.cluster.local:8000)
- [ ] T-S010-P02-012 [P] Create ConfigMap template for vLLM configuration in `infra/helm/charts/vllm-deployment/templates/configmap.yaml`
- [ ] T-S010-P02-013 [P] Create NetworkPolicy template for API Router → vLLM pod communication in `infra/helm/charts/vllm-deployment/templates/networkpolicy.yaml`
- [ ] T-S010-P02-014 [P] Create ServiceMonitor template for Prometheus metrics scraping in `infra/helm/charts/vllm-deployment/templates/servicemonitor.yaml`
- [ ] T-S010-P02-015 Validate Helm chart with `helm lint` and `helm template --dry-run` for all environments
- [ ] T-S010-P02-016 [P] Setup Testcontainers framework for integration tests in `test/integration/setup.go` (PostgreSQL container, Redis container, test utilities)

**Checkpoint**: Foundation ready — database schema extended, base Helm chart structure in place, proceed to user stories

---

## Phase 3: User Story 1 - Provision reliable inference endpoints (Priority: P1) 🎯 MVP

**Goal**: Deploy model inference engines on GPU nodes with predictable endpoints and readiness checks

**Independent Test**: Deploy a model instance, verify readiness endpoint returns healthy, and test completion endpoint returns valid response within ≤3 seconds

### Tests for User Story 1

- [ ] T-S010-P03-016 [P] [US1] Create integration test for deployment readiness in `test/integration/vllm_deployment_test.go` using Testcontainers for PostgreSQL/Redis (deploy model, wait for ready, verify /health and /ready endpoints)
- [ ] T-S010-P03-017 [P] [US1] Create integration test for completion endpoint in `test/integration/vllm_completion_test.go` using Testcontainers for PostgreSQL/Redis (POST /v1/chat/completions, verify response within ≤3 seconds)
- [ ] T-S010-P03-018 [P] [US1] Create E2E test for deployment → readiness → completion flow in `test/e2e/vllm_deployment_e2e_test.go`

### Implementation for User Story 1

- [ ] T-S010-P03-019 [US1] Add liveness probe configuration to Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (path: /health, port: 8000, initialDelaySeconds: 60, periodSeconds: 30)
- [ ] T-S010-P03-020 [US1] Add readiness probe configuration to Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (path: /ready, port: 8000, initialDelaySeconds: 30, periodSeconds: 10)
- [ ] T-S010-P03-021 [US1] Configure GPU resource requests and limits in Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (nvidia.com/gpu: 1, memory: 32Gi-48Gi, CPU: 8-16)
- [ ] T-S010-P03-022 [US1] Add nodeSelector and tolerations for GPU node pool in Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (node-type: gpu, toleration for gpu-workload)
- [ ] T-S010-P03-023 [US1] Configure vLLM container environment variables in Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (MODEL_PATH from values, port 8000, host 0.0.0.0)
- [ ] T-S010-P03-024 [US1] Add startup probe for model loading in Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (initialDelaySeconds: 0, periodSeconds: 30, timeoutSeconds: 10, failureThreshold: 20 for 7B-13B models, failureThreshold: 40 for 70B models, document timeout strategy in values.yaml comments)
- [ ] T-S010-P03-024a [US1] Document model initialization timeout strategy in `docs/model-initialization.md` (timeout calculation based on model size, failureThreshold configuration per model size, fallback behavior on timeout, monitoring and alerting for long initialization times)
- [ ] T-S010-P03-025 [US1] Implement predictable endpoint naming in values.yaml and Service template in `infra/helm/charts/vllm-deployment/templates/service.yaml` (format: {model-name}-{environment}.{namespace}.svc.cluster.local:8000)
- [ ] T-S010-P03-026 [US1] Add resource capacity validation in Helm chart (pre-install hook or values validation) to check GPU availability before deployment, with clear error messages for resource exhaustion scenarios
- [ ] T-S010-P03-026a [US1] Implement deployment wait/retry logic in `scripts/deploy-with-retry.sh` (check GPU availability, wait with exponential backoff if unavailable, retry deployment, timeout after max wait period, log wait events)
- [ ] T-S010-P03-027 [US1] Create deployment verification script in `scripts/verify-deployment.sh` (check pod status, health endpoints, test completion)
- [ ] T-S010-P03-028 [US1] Document deployment workflow in `docs/deployment-workflow.md` (helm install, wait for ready, verify endpoints)

**Checkpoint**: At this point, User Story 1 should be fully functional - can deploy a model, verify readiness, and test completion endpoint independently

---

## Phase 4: User Story 2 - Register models for routing (Priority: P2)

**Goal**: Register/de-register model names and backends so API Router can route client requests

**Independent Test**: Deploy a model, register it in model_registry_entries, verify API Router can route requests to the model by name

### Tests for User Story 2

- [ ] T-S010-P04-029 [P] [US2] Create integration test for model registration in `test/integration/model_registration_test.go` using Testcontainers for PostgreSQL (insert into model_registry_entries, verify deployment_endpoint and deployment_status set)
- [ ] T-S010-P04-030 [P] [US2] Create integration test for API Router routing to registered model in `test/integration/routing_test.go` (query model_registry_entries, verify routing decision)
- [ ] T-S010-P04-031 [P] [US2] Create integration test for model deregistration in `test/integration/model_deregistration_test.go` (update deployment_status to 'disabled', verify routing denied)

### Implementation for User Story 2

- [ ] T-S010-P04-032 [US2] Extend admin-cli with model registration command in `services/admin-cli/internal/commands/registry.go` (register model with endpoint, environment, status)
- [ ] T-S010-P04-033 [US2] Implement model registration logic in `services/admin-cli/internal/commands/registry.go` (insert/update model_registry_entries with deployment_endpoint, deployment_status='ready', deployment_environment, deployment_namespace)
- [ ] T-S010-P04-034 [US2] Implement model deregistration logic in `services/admin-cli/internal/commands/registry.go` (update deployment_status to 'disabled' or delete entry)
- [ ] T-S010-P04-035 [US2] Add model enable/disable commands in `services/admin-cli/internal/commands/registry.go` (update deployment_status between 'ready' and 'disabled')
- [ ] T-S010-P04-036 [US2] Extend API Router Service to query model_registry_entries for routing in `services/api-router-service/internal/routing/registry.go` (query WHERE deployment_status='ready' AND model_name=$1)
- [ ] T-S010-P04-037 [US2] Add Redis caching for model registry queries in `services/api-router-service/internal/routing/registry.go` (TTL: 2 minutes, invalidate on status changes)
- [ ] T-S010-P04-038 [US2] Implement routing decision logic using deployment_endpoint in `services/api-router-service/internal/routing/engine.go` (construct request URL from deployment_endpoint)
- [ ] T-S010-P04-039 [US2] Add error handling for disabled models in `services/api-router-service/internal/routing/engine.go` (return 404 or 503 with clear error message when deployment_status='disabled')
- [ ] T-S010-P04-040 [US2] Create post-deployment registration hook script in `scripts/register-model.sh` (automatically register model after successful deployment)
- [ ] T-S010-P04-041 [US2] Document registration workflow in `docs/registration-workflow.md` (deploy → register → verify routing)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - can deploy models and register them for routing

---

## Phase 5: User Story 3 - Safe operations and environment separation (Priority: P3)

**Goal**: Separate environments and perform safe rollouts with clear status and minimal disruption

**Independent Test**: Deploy to staging, validate, promote to production, verify same configuration and success criteria

### Tests for User Story 3

- [ ] T-S010-P05-042 [P] [US3] Create integration test for environment separation in `test/integration/environment_separation_test.go` using Testcontainers for PostgreSQL (deploy to different namespaces, verify isolation)
- [ ] T-S010-P05-043 [P] [US3] Create integration test for rollback in `test/integration/rollback_test.go` (deploy, rollback to previous revision, verify previous version serving)
- [ ] T-S010-P05-044 [P] [US3] Create integration test for promotion workflow in `test/integration/promotion_test.go` (deploy to staging, validate, promote to production)
- [ ] T-S010-P05-045 [P] [US3] Create integration test for status inspection in `test/integration/status_inspection_test.go` (query deployment status, identify failing components)

### Implementation for User Story 3

- [ ] T-S010-P05-046 [US3] Configure namespace isolation in values files in `infra/helm/charts/vllm-deployment/values-development.yaml`, `values-staging.yaml`, `values-production.yaml` (namespace: system or environment-specific)
- [ ] T-S010-P05-047 [US3] Add environment labels and selectors to Deployment template in `infra/helm/charts/vllm-deployment/templates/deployment.yaml` (environment: {development|staging|production})
- [ ] T-S010-P05-048 [US3] Implement Helm rollback workflow documentation in `docs/rollback-workflow.md` (helm history, helm rollback, verify previous version, troubleshooting common rollback failures, rollback decision criteria, rollback validation steps)
- [ ] T-S010-P05-048a [US3] Create comprehensive rollout guidance in `docs/rollout-workflow.md` (pre-deployment checks, deployment steps, validation gates, rollback triggers, status monitoring, common issues and resolutions)
- [ ] T-S010-P05-049 [US3] Create rollback script in `scripts/rollback-deployment.sh` (helm rollback, update model_registry_entries status, verify)
- [ ] T-S010-P05-050 [US3] Implement promotion workflow script in `scripts/promote-deployment.sh` (validate staging, deploy to production with same config, verify)
- [ ] T-S010-P05-051 [US3] Add ArgoCD Application manifests for GitOps in `argocd-apps/vllm-{model}-{environment}.yaml` (sync policy: auto for dev/staging, manual for production)
- [ ] T-S010-P05-052 [US3] Create status inspection command in `services/admin-cli/internal/commands/deployment.go` (query deployment status, pod status, health check status)
- [ ] T-S010-P05-053 [US3] Implement deployment status aggregation in `services/admin-cli/internal/commands/deployment.go` (combine Helm status, Kubernetes pod status, database status)
- [ ] T-S010-P05-054 [US3] Add health check status monitoring in `services/admin-cli/internal/commands/deployment.go` (query last_health_check_at from model_registry_entries)
- [ ] T-S010-P05-055 [US3] Create runbook for partial failure remediation in `docs/runbooks/partial-failure-remediation.md` (identify failing components, remediation steps, rollback procedures)
- [ ] T-S010-P05-056 [US3] Add validation gates for promotion in `scripts/promote-deployment.sh` (health check passing, completion test successful, metrics within SLO)
- [ ] T-S010-P05-057 [US3] Document environment separation strategy in `docs/environment-separation.md` (namespace isolation, NetworkPolicies, resource quotas)

**Checkpoint**: All user stories should now be independently functional - can deploy, register, and manage models across environments with safe rollouts

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T-S010-P06-058 [P] Add comprehensive Helm chart documentation in `infra/helm/charts/vllm-deployment/README.md` (values reference, deployment examples, troubleshooting)
- [ ] T-S010-P06-059 [P] Create Grafana dashboard for model deployment status in `docs/dashboards/vllm-deployment-dashboard.json` (deployment status, health checks, resource utilization)
- [ ] T-S010-P06-060 [P] Add Prometheus alerts for deployment failures in `infra/helm/charts/vllm-deployment/templates/alerts.yaml` (pod crash, health check failures, GPU resource exhaustion)
- [ ] T-S010-P06-061 [P] Implement health check service (optional) in `services/health-check-service/` to periodically update last_health_check_at in model_registry_entries
- [ ] T-S010-P06-062 [P] Add performance monitoring and SLO tracking in `docs/slo-tracking.md` (deployment time, completion latency, registration propagation)
- [ ] T-S010-P06-063 [P] Create troubleshooting guide in `docs/troubleshooting.md` (common issues, debugging steps, recovery procedures)
- [ ] T-S010-P06-064 [P] Add integration with existing observability stack in `infra/helm/charts/vllm-deployment/templates/servicemonitor.yaml` (Prometheus scraping, Loki log collection)
- [ ] T-S010-P06-065 [P] Update architecture documentation in `docs/platform/deployment-architecture.md` (vLLM deployment patterns, GitOps workflow)
- [ ] T-S010-P06-066 [P] Add security hardening (NetworkPolicies review, RBAC, secrets management) in `infra/helm/charts/vllm-deployment/templates/networkpolicy.yaml` and `templates/rbac.yaml`
- [ ] T-S010-P06-067 [P] Create quickstart validation script in `scripts/validate-quickstart.sh` (run quickstart.md steps, verify all operations work)
- [ ] T-S010-P06-068 [P] Add load testing scenarios in `test/load/vllm_load_test.go` (concurrent deployments, multiple model instances, resource contention)
- [ ] T-S010-P06-069 [P] Document best practices in `docs/best-practices.md` (model sizing, resource allocation, deployment patterns)
- [ ] T-S010-P06-070 [P] Add cost tracking and optimization guidance in `docs/cost-optimization.md` (GPU utilization, model sharing strategies)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3+)**: All depend on Foundational phase completion
  - User Story 1 (P1): Can start after Foundational - No dependencies on other stories
  - User Story 2 (P2): Can start after Foundational - Depends on User Story 1 for deployed models to register
  - User Story 3 (P3): Can start after Foundational - Depends on User Story 1 for deployments to manage
- **Polish (Phase 6)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Requires User Story 1 deployments to register, but can be developed in parallel
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Requires User Story 1 deployments to manage, but can be developed in parallel

### Within Each User Story

- Tests MUST be written and FAIL before implementation
- Helm chart templates before deployment scripts
- Core deployment functionality before registration/routing integration
- Basic functionality before advanced features (rollback, promotion)
- Story complete before moving to next priority

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel (T-S010-P01-002 through T-S010-P01-007)
- All Foundational tasks marked [P] can run in parallel within Phase 2 (T-S010-P02-010 through T-S010-P02-014)
- Once Foundational phase completes:
  - User Story 1 tests can run in parallel (T-S010-P03-016, T-S010-P03-017, T-S010-P03-018)
  - User Story 1 implementation tasks can proceed sequentially (some can be parallelized)
- User Story 2 can start development in parallel with User Story 1 completion (tests can be written)
- User Story 3 can start development in parallel with User Story 1 completion (tests can be written)
- All Polish tasks marked [P] can run in parallel (T-S010-P06-058 through T-S010-P06-070)

---

## Parallel Example: User Story 1

```bash
# Launch all tests for User Story 1 together:
Task: T-S010-P03-016 - Integration test for deployment readiness
Task: T-S010-P03-017 - Integration test for completion endpoint
Task: T-S010-P03-018 - E2E test for deployment flow

# Launch foundational Helm templates in parallel:
Task: T-S010-P02-010 - Base Deployment template
Task: T-S010-P02-011 - Service template
Task: T-S010-P02-012 - ConfigMap template
Task: T-S010-P02-013 - NetworkPolicy template
Task: T-S010-P02-014 - ServiceMonitor template
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (Helm chart structure)
2. Complete Phase 2: Foundational (Database migration, base Helm chart)
3. Complete Phase 3: User Story 1 (Deploy model, verify readiness, test completion)
4. **STOP and VALIDATE**: Test User Story 1 independently
   - Deploy a test model (e.g., llama-7b)
   - Verify /health and /ready endpoints
   - Test /v1/chat/completions endpoint
   - Verify response time ≤3 seconds
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test independently → Deploy/Demo (MVP!)
   - Can deploy models to GPU nodes
   - Can verify readiness
   - Can test completions
3. Add User Story 2 → Test independently → Deploy/Demo
   - Can register models for routing
   - API Router can route to deployed models
4. Add User Story 3 → Test independently → Deploy/Demo
   - Can manage environments separately
   - Can perform rollbacks
   - Can promote between environments
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (Helm chart, deployment, readiness)
   - Developer B: User Story 2 (Registration, API Router integration) - can start after US1 has basic deployment
   - Developer C: User Story 3 (Rollback, promotion, status) - can start after US1 has basic deployment
3. Stories complete and integrate independently

---

## Task Summary

- **Total Tasks**: 75 (5 new tasks added from remediation)
- **Phase 1 (Setup)**: 7 tasks
- **Phase 2 (Foundational)**: 9 tasks (added Testcontainers setup)
- **Phase 3 (User Story 1 - P1)**: 15 tasks (3 tests + 12 implementation, added timeout docs and retry logic)
- **Phase 4 (User Story 2 - P2)**: 12 tasks (3 tests + 9 implementation)
- **Phase 5 (User Story 3 - P3)**: 18 tasks (4 tests + 14 implementation, added rollout guidance)
- **Phase 6 (Polish)**: 14 tasks

### Independent Test Criteria

- **User Story 1**: Deploy model instance → Verify readiness → Test completion endpoint returns valid response within ≤3 seconds
- **User Story 2**: Deploy model → Register in model_registry_entries → Verify API Router routes requests to model by name
- **User Story 3**: Deploy to staging → Validate → Promote to production → Verify same configuration and success criteria

### Suggested MVP Scope

**MVP = Phase 1 + Phase 2 + Phase 3 (User Story 1 only)**

This delivers:
- Helm chart for vLLM deployment
- Database schema for deployment metadata
- Ability to deploy models to GPU nodes
- Readiness and health checks
- Test completion endpoint

Total: 31 tasks for MVP (includes Testcontainers setup and enhanced edge case handling)

---

## Notes

- [P] tasks = different files, no dependencies - can run in parallel
- [US1], [US2], [US3] labels map tasks to specific user stories for traceability
- Each user story should be independently completable and testable
- Verify tests fail before implementing
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence
- Helm chart validation should run in CI before merging
- Database migrations must be tested in development environment first
- All Kubernetes resources must follow naming conventions for predictable endpoints

