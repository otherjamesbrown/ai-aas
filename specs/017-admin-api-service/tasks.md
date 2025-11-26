# Tasks: Admin API Service

**Input**: Design documents from `/specs/017-admin-api-service/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Task Naming Convention

**Format**: `T-S017-P{phase}-{task}`

---

## Phase 1: Setup

**Purpose**: Project initialization and service structure

- [x] T-S017-P01-001 Create service directory structure at services/admin-api-service/ per plan.md
- [x] T-S017-P01-002 Initialize Go module with go.mod at services/admin-api-service/go.mod
- [x] T-S017-P01-003 [P] Create main.go entrypoint at services/admin-api-service/cmd/admin-api/main.go
- [x] T-S017-P01-004 [P] Create config.go with environment variable loading at services/admin-api-service/internal/config/config.go
- [x] T-S017-P01-005 [P] Create Dockerfile at services/admin-api-service/Dockerfile
- [x] T-S017-P01-006 [P] Create README.md at services/admin-api-service/README.md

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure required before ANY user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T-S017-P02-007 Create database migration for routing_policies table at db/migrations/XXX_create_routing_policies.up.sql
- [x] T-S017-P02-008 Create database migration for audit_logs table at db/migrations/XXX_create_audit_logs.up.sql
- [x] T-S017-P02-009 Create database migration for policy_sync_log table at db/migrations/XXX_create_policy_sync_log.up.sql
- [x] T-S017-P02-010 [P] Implement database connection pool using shared/go/dbutil at services/admin-api-service/internal/repository/db.go
- [x] T-S017-P02-011 [P] Implement API key authentication middleware at services/admin-api-service/internal/api/middleware/auth.go
- [x] T-S017-P02-012 [P] Implement rate limiting middleware at services/admin-api-service/internal/api/middleware/ratelimit.go
- [x] T-S017-P02-013 [P] Implement request logging middleware at services/admin-api-service/internal/api/middleware/logging.go
- [x] T-S017-P02-014 Implement chi router setup with middleware chain at services/admin-api-service/internal/api/router.go
- [x] T-S017-P02-015 [P] Implement health check handler at services/admin-api-service/internal/api/handlers/health.go
- [x] T-S017-P02-016 [P] Implement metrics handler (Prometheus) at services/admin-api-service/internal/api/handlers/metrics.go
- [x] T-S017-P02-017 [P] Implement RFC 7807 error response helpers at services/admin-api-service/internal/api/errors.go

**Checkpoint**: Foundation ready - user story implementation can begin

---

## Phase 3: User Story 1 - Model Registry Management (Priority: P1) 🎯 MVP

**Goal**: Register and manage model deployments through secure API without direct database access

**Independent Test**: Deploy service, use curl to POST /v1/registry/models with valid auth and verify 201 response

### Implementation for User Story 1

- [x] T-S017-P03-018 [P] [US1] Create Model repository at services/admin-api-service/internal/repository/models.go
- [x] T-S017-P03-019 [P] [US1] Create Model domain types at services/admin-api-service/internal/domain/model.go
- [x] T-S017-P03-020 [US1] Implement ModelRegistry service at services/admin-api-service/internal/service/registry.go
- [x] T-S017-P03-021 [US1] Implement POST /v1/registry/models handler (register/upsert) at services/admin-api-service/internal/api/handlers/models.go
- [x] T-S017-P03-022 [US1] Implement GET /v1/registry/models handler (list with filtering) at services/admin-api-service/internal/api/handlers/models.go
- [x] T-S017-P03-023 [US1] Implement GET /v1/registry/models/{model_name} handler at services/admin-api-service/internal/api/handlers/models.go
- [x] T-S017-P03-024 [US1] Implement PATCH /v1/registry/models/{model_name} handler at services/admin-api-service/internal/api/handlers/models.go
- [x] T-S017-P03-025 [US1] Implement DELETE /v1/registry/models/{model_name} handler at services/admin-api-service/internal/api/handlers/models.go
- [x] T-S017-P03-026 [US1] Add request validation for model endpoints at services/admin-api-service/internal/api/handlers/models.go
- [x] T-S017-P03-027 [US1] Wire model routes in router at services/admin-api-service/internal/api/router.go

**Checkpoint**: Model registry API fully functional and testable

---

## Phase 4: User Story 2 - Organization and System Management (Priority: P2)

**Goal**: Manage organizations and query system state through API

**Independent Test**: Call POST /v1/organizations with valid auth and verify 201 response with generated UUID

### Implementation for User Story 2

- [x] T-S017-P04-028 [P] [US2] Create Organization repository at services/admin-api-service/internal/repository/organizations.go
- [x] T-S017-P04-029 [P] [US2] Create Organization domain types at services/admin-api-service/internal/domain/organization.go
- [x] T-S017-P04-030 [US2] Implement OrganizationService at services/admin-api-service/internal/service/organizations.go
- [x] T-S017-P04-031 [US2] Implement POST /v1/organizations handler at services/admin-api-service/internal/api/handlers/organizations.go
- [x] T-S017-P04-032 [US2] Implement GET /v1/organizations handler (list with pagination) at services/admin-api-service/internal/api/handlers/organizations.go
- [x] T-S017-P04-033 [US2] Implement GET /v1/organizations/{org_id} handler at services/admin-api-service/internal/api/handlers/organizations.go
- [x] T-S017-P04-034 [US2] Implement PATCH /v1/organizations/{org_id} handler at services/admin-api-service/internal/api/handlers/organizations.go
- [x] T-S017-P04-035 [US2] Add request validation for organization endpoints at services/admin-api-service/internal/api/handlers/organizations.go
- [x] T-S017-P04-036 [US2] Wire organization routes in router at services/admin-api-service/internal/api/router.go

**Checkpoint**: Organization management API fully functional

---

## Phase 5: User Story 3 - Audit Logging and Observability (Priority: P2)

**Goal**: Track all administrative operations through comprehensive audit logs and metrics

**Independent Test**: Perform any operation, then GET /v1/audit-logs and verify entry exists with correct actor/action

### Implementation for User Story 3

- [x] T-S017-P05-036 [P] [US3] Create AuditLog repository at services/admin-api-service/internal/repository/audit.go
- [x] T-S017-P05-037 [P] [US3] Create AuditLog domain types at services/admin-api-service/internal/domain/audit.go
- [x] T-S017-P05-038 [US3] Implement AuditService at services/admin-api-service/internal/service/audit.go
- [x] T-S017-P05-039 [US3] Implement audit logging middleware (intercept all mutating operations) at services/admin-api-service/internal/api/middleware/audit.go
- [x] T-S017-P05-040 [US3] Implement GET /v1/audit-logs handler (query with filters) at services/admin-api-service/internal/api/handlers/audit.go
- [x] T-S017-P05-041 [US3] Add RED metrics (Rate/Errors/Duration) per endpoint at services/admin-api-service/internal/api/middleware/metrics.go
- [x] T-S017-P05-042 [US3] Add database query duration metrics at services/admin-api-service/internal/repository/metrics.go
- [x] T-S017-P05-043 [US3] Wire audit routes in router at services/admin-api-service/internal/api/router.go

**Checkpoint**: Full audit trail and observability in place

---

## Phase 6: Routing Policies (Priority: P3)

**Goal**: Manage routing policies for model traffic distribution

**Specification**: See `routing-policy-api-addition.md` for full API specification (FR-013 through FR-020)

**Independent Test**: Create policy via POST /v1/routing/policies, verify via GET, update weights via PATCH

### Implementation for Routing Policies

- [x] T-S017-P06-044 [P] [US4] Create RoutingPolicy repository at services/admin-api-service/internal/repository/policies.go
- [x] T-S017-P06-045 [P] [US4] Create RoutingPolicy domain types at services/admin-api-service/internal/domain/policy.go
- [x] T-S017-P06-046 [US4] Implement PolicyService at services/admin-api-service/internal/service/policies.go
- [x] T-S017-P06-047 [US4] Implement POST /v1/routing/policies handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-048 [US4] Implement GET /v1/routing/policies handler (list) at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-049 [US4] Implement GET /v1/routing/policies/{policy_id} handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-050 [US4] Implement PATCH /v1/routing/policies/{policy_id} handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-051 [US4] Implement DELETE /v1/routing/policies/{policy_id} handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-052 [US4] Implement POST /v1/routing/policies/{policy_id}/activate handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-053 [US4] Implement POST /v1/routing/policies/{policy_id}/deactivate handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-054 [US4] Implement GET /v1/routing/policies/sync handler (for api-router) at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-055 [US4] Implement POST /v1/routing/policies/validate handler at services/admin-api-service/internal/api/handlers/policies.go
- [x] T-S017-P06-056 [US4] Add backend validation (verify backends exist in model registry) at services/admin-api-service/internal/service/policies.go
- [x] T-S017-P06-057 [US4] Wire policy routes in router at services/admin-api-service/internal/api/router.go

**Checkpoint**: Full routing policy management available

---

## Phase 7: Production Hardening

**Purpose**: Reliability and security improvements

- [x] T-S017-P07-058 Implement graceful shutdown (30s grace period) at services/admin-api-service/cmd/admin-api/main.go
- [x] T-S017-P07-059 [P] Implement circuit breaker for database operations at services/admin-api-service/internal/repository/circuit.go
- [x] T-S017-P07-060 [P] Add security headers middleware at services/admin-api-service/internal/api/middleware/security.go
- [x] T-S017-P07-061 [P] Add OpenTelemetry tracing at services/admin-api-service/internal/api/middleware/tracing.go
- [x] T-S017-P07-062 [P] Create Kubernetes deployment manifest at services/admin-api-service/k8s/deployment.yaml
- [x] T-S017-P07-063 [P] Create Kubernetes service manifest at services/admin-api-service/k8s/service.yaml
- [x] T-S017-P07-064 [P] Create Kubernetes configmap at services/admin-api-service/k8s/configmap.yaml

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation and final validation

- [x] T-S017-P08-065 [P] Create config.example.env at services/admin-api-service/config.example.env
- [x] T-S017-P08-066 [P] Update service README with API documentation at services/admin-api-service/README.md
- [x] T-S017-P08-067 Add service to root Makefile targets (auto-discovered via services/ directory)
- [x] T-S017-P08-068 Run quickstart.md validation (local deployment test)
- [x] T-S017-P08-069 Validate OpenAPI spec matches implementation at specs/017-admin-api-service/contracts/openapi.yaml

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 - BLOCKS all user stories
- **Phases 3-6 (User Stories)**: All depend on Phase 2 completion
- **Phase 7 (Hardening)**: Depends on at least Phase 3 (MVP)
- **Phase 8 (Polish)**: Depends on all desired phases complete

### User Story Dependencies

- **US1 (Model Registry)**: Independent after Phase 2
- **US2 (Organizations)**: Independent after Phase 2
- **US3 (Audit)**: Independent after Phase 2, enhances US1/US2
- **US4 (Policies)**: Independent after Phase 2, references models from US1

### Parallel Opportunities

- Phase 1: T-S017-P01-003 through T-S017-P01-006 can run in parallel
- Phase 2: T-S017-P02-010 through T-S017-P02-017 (marked [P]) can run in parallel
- Phase 3+: All user stories can run in parallel if team capacity allows

---

## Parallel Example: Phase 2

```bash
# Launch all foundational middleware in parallel:
Task: "Implement API key authentication middleware"
Task: "Implement rate limiting middleware"  
Task: "Implement request logging middleware"
Task: "Implement health check handler"
Task: "Implement metrics handler"
Task: "Implement RFC 7807 error response helpers"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational
3. Complete Phase 3: User Story 1 (Model Registry)
4. **STOP and VALIDATE**: Test model registration via curl
5. Deploy to development cluster

### Incremental Delivery

1. Setup + Foundational → Foundation ready
2. Add US1 (Model Registry) → Test → Deploy (MVP!)
3. Add US2 (Organizations) → Test → Deploy
4. Add US3 (Audit) → Test → Deploy
5. Add US4 (Policies) → Test → Deploy
6. Hardening → Production ready

---

## Summary

| Metric | Count |
|--------|-------|
| Total Tasks | 70 |
| Phase 1 (Setup) | 6 |
| Phase 2 (Foundational) | 11 |
| Phase 3 (US1 - Models) | 10 |
| Phase 4 (US2 - Orgs) | 9 |
| Phase 5 (US3 - Audit) | 8 |
| Phase 6 (US4 - Policies) | 14 |
| Phase 7 (Hardening) | 7 |
| Phase 8 (Polish) | 5 |
| Parallel Opportunities | 35 tasks marked [P] |

**Suggested MVP Scope**: Phases 1-3 (27 tasks) delivers model registry API

