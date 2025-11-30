# Tasks: User-Level Model Access Control

**Input**: Design documents from `/specs/022-user-model-access-control/`
**Prerequisites**: plan.md ✓, spec.md ✓, research.md ✓, data-model.md ✓, contracts/ ✓

**Tests**: Integration tests included as this feature extends critical auth path (per constitution: testing gate).

**Organization**: Tasks grouped by user story to enable independent implementation and testing.

## Task Naming Convention

**Format**: `T-S022-P{phase}-{task}`

- **Spec Number**: 022 (user-model-access-control)
- **Phase Number**: Two-digit phase (01, 02, 03...)
- **Task Number**: Three-digit sequential (001, 002, 003...)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Database migrations and project structure

- [X] T-S022-P01-001 Create database migration file `services/user-org-service/migrations/sql/000004_user_model_access.sql` (up)
- [X] T-S022-P01-002 Create database rollback (down) in same file (goose format)
- [X] T-S022-P01-003 [P] Add feature flag `USER_MODEL_ACCESS_ENABLED` to `configs/environments/development.yaml`
- [X] T-S022-P01-004 [P] Add feature flag to `configs/environments/production.yaml`

**Checkpoint**: Database schema ready, feature flag configured

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Repository layer that all user stories depend on

**⚠️ CRITICAL**: No API or router work can begin until this phase is complete

- [X] T-S022-P02-005 Create model access repository interface in `services/user-org-service/internal/storage/postgres/model_access.go`
- [X] T-S022-P02-006 Implement `GetUserModelAccess(ctx, orgID, userID)` method
- [X] T-S022-P02-007 Implement `SetUserAccessMode(ctx, orgID, userID, mode)` method
- [X] T-S022-P02-008 Implement `CreateModelGrant(ctx, orgID, userID, modelName, grantedBy, expiresAt)` method
- [X] T-S022-P02-009 Implement `DeleteModelGrant(ctx, orgID, userID, modelName)` method
- [X] T-S022-P02-010 Implement `ListModelGrants(ctx, orgID, userID)` method
- [X] T-S022-P02-011 Implement `CheckModelAccess(ctx, orgID, userID, modelName)` method (hot path query)
- [X] T-S022-P02-012 Implement `GrantAllCurrentModels(ctx, orgID, userID, grantedBy)` method
- [ ] T-S022-P02-013 Implement `GetAvailableModels(ctx, orgID)` method to query routing_policies for org's available models (deferred - routing_policies table in different schema)
- [ ] T-S022-P02-014 [P] Create repository unit tests in `services/user-org-service/internal/storage/postgres/model_access_test.go`

**Checkpoint**: Repository layer complete and tested - API implementation can now begin

---

## Phase 3: User Story 1 - Model Access API (Priority: P1) 🎯 MVP

**Goal**: Enable org admins to view and manage user access modes via REST API

**Independent Test**: `curl GET /api/v1/orgs/{org}/users/{user}/model-access` returns access mode and grants

### Tests for User Story 1

- [ ] T-S022-P03-015 [P] [US1] Integration test for GET model-access in `services/user-org-service/internal/httpapi/modelaccess/handlers_test.go`
- [ ] T-S022-P03-016 [P] [US1] Integration test for PUT access mode in `services/user-org-service/internal/httpapi/modelaccess/handlers_test.go`

### Implementation for User Story 1

- [X] T-S022-P03-017 [US1] Create handler struct and constructor in `services/user-org-service/internal/httpapi/modelaccess/handlers.go`
- [X] T-S022-P03-018 [US1] Implement `GetUserModelAccess` handler (GET /orgs/{org_id}/users/{user_id}/model-access)
- [X] T-S022-P03-019 [US1] Implement `SetUserAccessMode` handler (PUT /orgs/{org_id}/users/{user_id}/model-access/mode)
- [X] T-S022-P03-020 [US1] Add org admin permission check middleware
- [X] T-S022-P03-021 [US1] Register routes in `services/user-org-service/cmd/admin-api/main.go`
- [X] T-S022-P03-022 [US1] Add request/response DTOs with JSON tags and validation

**Checkpoint**: Admin can view user access mode and set restricted/auto_grant via API

---

## Phase 4: User Story 2 - Grant Management API (Priority: P1)

**Goal**: Enable org admins to grant/revoke specific model access via REST API

**Independent Test**: 
1. `POST /grants` with model name creates grant
2. `DELETE /grants/{model}` removes grant
3. User with grant can access model; without grant cannot

### Tests for User Story 2

- [ ] T-S022-P04-023 [P] [US2] Integration test for POST grant in `services/user-org-service/internal/httpapi/modelaccess/handlers_test.go`
- [ ] T-S022-P04-024 [P] [US2] Integration test for DELETE grant in `services/user-org-service/internal/httpapi/modelaccess/handlers_test.go`
- [ ] T-S022-P04-025 [P] [US2] Integration test for POST grant-all-current in `services/user-org-service/internal/httpapi/modelaccess/handlers_test.go`

### Implementation for User Story 2

- [X] T-S022-P04-026 [US2] Implement `GrantModelAccess` handler (POST /orgs/{org_id}/users/{user_id}/model-access/grants)
- [X] T-S022-P04-027 [US2] Implement `RevokeModelAccess` handler (DELETE /orgs/{org_id}/users/{user_id}/model-access/grants/{model_name})
- [X] T-S022-P04-028 [US2] Implement `ListUserModelGrants` handler (GET /orgs/{org_id}/users/{user_id}/model-access/grants)
- [X] T-S022-P04-029 [US2] Implement `GrantAllCurrentModels` handler (POST /orgs/{org_id}/users/{user_id}/model-access/grants/all-current)
- [X] T-S022-P04-030 [US2] Add grant expiration validation (expires_at must be future if provided)
- [X] T-S022-P04-031 [US2] Add conflict handling for duplicate grants (return 409)

**Checkpoint**: Admin can grant specific models, grant all current models, and revoke grants via API

---

## Phase 5: User Story 3 - Router Enforcement (Priority: P2)

**Goal**: API Router enforces user-level model access on inference requests

**Independent Test**:
1. User with `auto_grant` mode can access any model
2. User with `restricted` mode + grant can access granted model
3. User with `restricted` mode + no grant receives 403

### Tests for User Story 3

- [ ] T-S022-P05-032 [P] [US3] Integration test for auto_grant access in `services/api-router-service/test/integration/model_access_test.go`
- [ ] T-S022-P05-033 [P] [US3] Integration test for restricted access with grant in `services/api-router-service/test/integration/model_access_test.go`
- [ ] T-S022-P05-034 [P] [US3] Integration test for restricted access denied in `services/api-router-service/test/integration/model_access_test.go`

### Implementation for User Story 3

- [X] T-S022-P05-035 [US3] Extend `AuthenticatedContext` struct with `ModelAccessMode` and `GrantedModels` in `services/api-router-service/internal/auth/authenticator.go`
- [X] T-S022-P05-036 [US3] Update `validateAPIKey` to include model access in auth response
- [X] T-S022-P05-037 [US3] Extend user-org-service `/v1/auth/validate-api-key` response with model access fields
- [X] T-S022-P05-038 [US3] Create `ModelAccessMiddleware` in `services/api-router-service/internal/api/public/middleware.go`
- [X] T-S022-P05-039 [US3] Implement model extraction from request body (model field parsing) - uses BodyBufferMiddleware
- [X] T-S022-P05-040 [US3] Implement access check logic (auto_grant bypasses, restricted checks grants)
- [X] T-S022-P05-041 [US3] Add 403 error response with clear message for access denied
- [ ] T-S022-P05-042 [US3] Register middleware in middleware chain (after AuthContext, before RateLimit) in `services/api-router-service/cmd/router/main.go`
- [X] T-S022-P05-043 [US3] Add feature flag check to skip middleware when disabled (featureEnabled parameter)

**Checkpoint**: Inference requests are enforced based on user-level model access

---

## Phase 6: User Story 4 - CLI Commands (Priority: P2)

**Goal**: Platform admins can manage user model access via CLI

**Independent Test**: `ai-aas-cli user model-access show/grant/revoke/set-mode` commands work end-to-end

### Implementation for User Story 4

- [X] T-S022-P06-044 [US4] Create CLI client for model access API in `services/ai-aas-cli/internal/client/userorg/model_access.go`
- [X] T-S022-P06-045 [US4] Create `model-access` command group in `services/ai-aas-cli/internal/admin/user_model_access.go`
- [X] T-S022-P06-046 [US4] Implement `show` subcommand (--org-id, --user-id/--email)
- [X] T-S022-P06-047 [US4] Implement `set-mode` subcommand (--mode restricted|auto_grant)
- [X] T-S022-P06-048 [US4] Implement `grant` subcommand (--model, --expires-in optional)
- [X] T-S022-P06-049 [US4] Implement `revoke` subcommand (--model)
- [X] T-S022-P06-050 [US4] Implement `list` subcommand (list all grants)
- [X] T-S022-P06-051 [US4] Implement `grant-all` subcommand
- [X] T-S022-P06-052 [US4] Add `--email` flag resolution to user ID
- [X] T-S022-P06-053 [US4] Add formatted table output for `list` and `show` commands

**Checkpoint**: CLI provides full model access management capability

---

## Phase 7: User Story 5 - Migration & Rollout (Priority: P3)

**Goal**: Smooth rollout with migration tooling for existing orgs

**Independent Test**: `ai-aas-cli user model-access migrate-existing` correctly grants all current models to existing users

### Implementation for User Story 5

- [X] T-S022-P07-054 [US5] Implement `migrate` CLI command (--org-id, --mode, --models)
- [X] T-S022-P07-055 [US5] Add batch processing for migrate (iterate all users in org)
- [X] T-S022-P07-056 [US5] Add --dry-run flag to preview migration changes
- [X] T-S022-P07-057 [US5] Add progress output for migration (X of Y users processed)
- [ ] T-S022-P07-058 [US5] Add audit logging for grant changes via existing audit queue (deferred)

**Checkpoint**: Existing orgs can be migrated without breaking existing user access

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, observability, and final polish

- [ ] T-S022-P08-059 [P] Add Prometheus metrics for model access checks (hit/miss/denied) in `services/api-router-service/internal/api/public/model_access.go`
- [ ] T-S022-P08-060 [P] Add structured logging for grant changes in `services/user-org-service/internal/httpapi/modelaccess/handlers.go`
- [ ] T-S022-P08-061 [P] Create runbook `docs/runbooks/user-model-access.md` with troubleshooting guide
- [ ] T-S022-P08-062 Update API documentation with new endpoints
- [ ] T-S022-P08-063 Add cache metrics (TTL hits, misses) for auth context caching
- [ ] T-S022-P08-064 Run quickstart.md validation end-to-end
- [ ] T-S022-P08-065 Security review: verify org admin permission checks on all endpoints

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1: Setup ────────────────────┐
                                   │
Phase 2: Foundational ◄────────────┘
         (Repository layer)
              │
              ▼
     ┌────────┴────────┐
     │                 │
     ▼                 ▼
Phase 3: US1       Phase 4: US2
(Access Mode API)  (Grant API)
     │                 │
     └────────┬────────┘
              │
              ▼
        Phase 5: US3
     (Router Enforcement)
              │
              ▼
        Phase 6: US4
        (CLI Commands)
              │
              ▼
        Phase 7: US5
     (Migration Tooling)
              │
              ▼
        Phase 8: Polish
```

### User Story Dependencies

| Story | Depends On | Can Start After |
|-------|------------|-----------------|
| US1 (Access Mode API) | Foundational (Phase 2) | Repository complete |
| US2 (Grant API) | Foundational (Phase 2) | Repository complete |
| US3 (Router Enforcement) | US1 + US2 | Both APIs complete |
| US4 (CLI Commands) | US1 + US2 | Both APIs complete |
| US5 (Migration Tooling) | US4 | CLI foundation complete |

### Parallel Opportunities

**Phase 1 (Setup)**: All 4 tasks can run in parallel
- T-S022-P01-001, T-S022-P01-002, T-S022-P01-003, T-S022-P01-004

**Phase 2 (Foundational)**: Repository test can run after all methods implemented
- Methods T-S022-P02-006 through T-S022-P02-013 are sequential (same file)
- T-S022-P02-014 tests can run parallel to method implementation

**Phase 3 (US1) + Phase 4 (US2)**: Can run in parallel after Phase 2
- US1 tests (T-S022-P03-015, T-S022-P03-016) parallel
- US2 tests (T-S022-P04-023, T-S022-P04-024, T-S022-P04-025) parallel

**Phase 5 (US3)**: Tests can run in parallel
- T-S022-P05-032, T-S022-P05-033, T-S022-P05-034 parallel

**Phase 8 (Polish)**: Multiple independent tasks
- T-S022-P08-059, T-S022-P08-060, T-S022-P08-061 parallel

---

## Parallel Example: User Story 1 + 2

```bash
# After Phase 2 (Foundational) completes, launch in parallel:

# Team Member A: User Story 1
Task: "Integration test for GET model-access"
Task: "Implement GetUserModelAccess handler"
Task: "Implement SetUserAccessMode handler"

# Team Member B: User Story 2
Task: "Integration test for POST grant"
Task: "Implement GrantModelAccess handler"
Task: "Implement RevokeModelAccess handler"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2 + 3)

1. Complete Phase 1: Setup (migrations, feature flag)
2. Complete Phase 2: Foundational (repository layer)
3. Complete Phase 3: User Story 1 (access mode API)
4. Complete Phase 4: User Story 2 (grant API)
5. Complete Phase 5: User Story 3 (router enforcement)
6. **STOP and VALIDATE**: Test access control end-to-end with feature flag ON
7. Deploy to development with feature flag OFF
8. Enable feature flag for specific test org
9. Validate in staging before production

### Incremental Delivery

| Milestone | Phases | Deliverable |
|-----------|--------|-------------|
| Foundation | 1 + 2 | Database ready, repository tested |
| API Ready | 3 + 4 | Admin can manage grants via API |
| **MVP Complete** | 5 | Inference enforces user-level access |
| CLI Complete | 6 | Admin can manage via CLI |
| Production Ready | 7 + 8 | Migration tooling, docs, observability |

### Rollback Plan

If issues in production:
1. Set `USER_MODEL_ACCESS_ENABLED=false` in config
2. ModelAccessMiddleware skips checks
3. All users revert to org-level access (current behavior)
4. No data loss - grants remain in database for re-enablement

---

## Notes

- [P] tasks = different files, no dependencies
- [US#] label maps task to user story for traceability
- Each user story is independently testable after its phase completes
- Feature flag allows gradual rollout without risk
- 30-second cache TTL means revocations take up to 30s to take effect
- Verify tests fail before implementing (TDD approach)
- Commit after each task or logical group

