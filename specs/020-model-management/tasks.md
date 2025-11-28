# Tasks: Model Management (ai-aas-cli)

**Input**: Design documents from `/specs/020-model-management/`
**Prerequisites**: plan.md ✅, spec.md ✅, research.md ✅, data-model.md ✅, contracts/ ✅

**Tests**: Tests are included per constitution (Test-First principle).

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Task Naming Convention

**Format**: `T-S020-P{phase}-{task}` (Spec 020, Model Management)

---

## Phase 1: Setup

**Purpose**: Project initialization and CLI service structure

- [ ] T-S020-P01-001 Create CLI service directory structure per plan.md in `services/ai-aas-cli/`
- [ ] T-S020-P01-002 Initialize Go module with dependencies in `services/ai-aas-cli/go.mod`
- [ ] T-S020-P01-003 [P] Create main.go entry point in `services/ai-aas-cli/main.go`
- [ ] T-S020-P01-004 [P] Create Makefile with build/test/lint targets in `services/ai-aas-cli/Makefile`
- [ ] T-S020-P01-005 [P] Configure golangci-lint for CLI service in `services/ai-aas-cli/.golangci.yml`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

### Database Migrations

- [ ] T-S020-P02-006 Create model_registry table migration in `db/migrations/20251128_001_create_model_registry.sql`
- [ ] T-S020-P02-007 [P] Create model_cache table migration in `db/migrations/20251128_002_create_model_cache.sql`
- [ ] T-S020-P02-008 [P] Create model_deployments table migration in `db/migrations/20251128_003_create_model_deployments.sql`
- [ ] T-S020-P02-009 [P] Create model_aliases table migration in `db/migrations/20251128_004_create_model_aliases.sql`
- [ ] T-S020-P02-010 [P] Create model_state_history table migration in `db/migrations/20251128_005_create_model_state_history.sql`
- [ ] T-S020-P02-011 [P] Create model_validations table migration in `db/migrations/20251128_006_create_model_validations.sql`
- [ ] T-S020-P02-012 [P] Create platform_credentials table migration in `db/migrations/20251128_007_create_platform_credentials.sql`

### Admin API Extensions

- [ ] T-S020-P02-013 Create models handler package structure in `services/admin-api-service/internal/handlers/models/`
- [ ] T-S020-P02-014 Create models service package structure in `services/admin-api-service/internal/services/models/`
- [ ] T-S020-P02-015 Register model management routes in admin-api-service router
- [ ] T-S020-P02-015A [P] Create OpenAPI contract for model management API in `specs/020-model-management/contracts/admin-api.yaml`

### CLI Core Infrastructure

- [ ] T-S020-P02-016 Create root command with --init, --version, --help in `services/ai-aas-cli/cmd/root.go`
- [ ] T-S020-P02-017 [P] Create output formatter package in `services/ai-aas-cli/internal/output/table.go`
- [ ] T-S020-P02-018 [P] Create JSON output formatter in `services/ai-aas-cli/internal/output/json.go`
- [ ] T-S020-P02-019 [P] Create progress bar helper in `services/ai-aas-cli/internal/output/progress.go`
- [ ] T-S020-P02-020 Create Admin API client base in `services/ai-aas-cli/internal/api/client.go`
- [ ] T-S020-P02-020A [P] Configure shared/go/logging with zap backend in `services/ai-aas-cli/internal/logging/logger.go`

**Checkpoint**: Foundation ready - user story implementation can now begin

---

## Phase 3: US-000 - CLI Initialization (Priority: P1) 🎯 MVP

**Goal**: Platform admins can initialize CLI with API key, endpoint, and credentials

**Independent Test**: Run `ai-aas-cli --init` on fresh install, verify config saved and `config test` passes

### Tests for US-000

- [ ] T-S020-P03-021 [P] [US0] Unit test for PATH detection in `services/ai-aas-cli/internal/config/path_test.go`
- [ ] T-S020-P03-022 [P] [US0] Unit test for config management in `services/ai-aas-cli/internal/config/config_test.go`
- [ ] T-S020-P03-023 [P] [US0] Integration test for init wizard in `services/ai-aas-cli/tests/integration/init_test.go`

### Implementation for US-000

- [ ] T-S020-P03-024 [P] [US0] Implement PATH detection logic in `services/ai-aas-cli/internal/config/path.go`
- [ ] T-S020-P03-025 [P] [US0] Implement config file management in `services/ai-aas-cli/internal/config/config.go`
- [ ] T-S020-P03-026 [US0] Implement interactive init wizard in `services/ai-aas-cli/internal/config/init.go`
- [ ] T-S020-P03-027 [US0] Implement config show command in `services/ai-aas-cli/cmd/config.go`
- [ ] T-S020-P03-028 [US0] Implement config set command in `services/ai-aas-cli/cmd/config.go`
- [ ] T-S020-P03-029 [US0] Implement config test command in `services/ai-aas-cli/cmd/config.go`
- [ ] T-S020-P03-030 [US0] Implement config check-path command in `services/ai-aas-cli/cmd/config.go`
- [ ] T-S020-P03-031 [US0] Wire --init flag to init wizard in root command

**Checkpoint**: CLI can be initialized and configured independently

---

## Phase 4: US-001 - Model Registry & Discovery (Priority: P1)

**Goal**: Platform admins can register models and view their state across all layers

**Independent Test**: Run `model add`, `model list`, `model info` - verify registry operations work

### Tests for US-001

- [ ] T-S020-P04-032 [P] [US1] Contract test for POST /models in `services/ai-aas-cli/tests/contract/models_test.go`
- [ ] T-S020-P04-033 [P] [US1] Contract test for GET /models in `services/ai-aas-cli/tests/contract/models_test.go`
- [ ] T-S020-P04-034 [P] [US1] Unit test for HF license detection in `services/ai-aas-cli/internal/huggingface/license_test.go`

### Admin API Implementation for US-001

- [ ] T-S020-P04-035 [P] [US1] Implement model registry service in `services/admin-api-service/internal/services/models/registry.go`
- [ ] T-S020-P04-036 [P] [US1] Implement POST /models handler in `services/admin-api-service/internal/handlers/models/add.go`
- [ ] T-S020-P04-037 [P] [US1] Implement GET /models handler in `services/admin-api-service/internal/handlers/models/list.go`
- [ ] T-S020-P04-038 [P] [US1] Implement GET /models/{name} handler in `services/admin-api-service/internal/handlers/models/get.go`
- [ ] T-S020-P04-039 [P] [US1] Implement DELETE /models/{name} handler in `services/admin-api-service/internal/handlers/models/delete.go`

### CLI Implementation for US-001

- [ ] T-S020-P04-040 [P] [US1] Implement HF Hub API client in `services/ai-aas-cli/internal/huggingface/client.go`
- [ ] T-S020-P04-041 [P] [US1] Implement license/gating detection in `services/ai-aas-cli/internal/huggingface/license.go`
- [ ] T-S020-P04-042 [US1] Implement model add command in `services/ai-aas-cli/cmd/model/add.go`
- [ ] T-S020-P04-043 [US1] Implement model list command in `services/ai-aas-cli/cmd/model/list.go`
- [ ] T-S020-P04-044 [US1] Implement model info command in `services/ai-aas-cli/cmd/model/info.go`
- [ ] T-S020-P04-045 [US1] Implement model remove command in `services/ai-aas-cli/cmd/model/remove.go`
- [ ] T-S020-P04-046 [US1] Implement model status command in `services/ai-aas-cli/cmd/model/status.go`

**Checkpoint**: Models can be registered, listed, and queried independently

---

## Phase 5: US-002 - Credentials Management (Priority: P1)

**Goal**: Platform admins can securely store HF tokens and S3 credentials

**Independent Test**: Run `credentials set hf-token`, `credentials test hf-token` - verify token validated

### Tests for US-002

- [ ] T-S020-P05-047 [P] [US2] Contract test for credentials endpoints in `services/ai-aas-cli/tests/contract/credentials_test.go`
- [ ] T-S020-P05-048 [P] [US2] Integration test for credentials workflow in `services/ai-aas-cli/tests/integration/credentials_test.go`

### Admin API Implementation for US-002

- [ ] T-S020-P05-049 [P] [US2] Implement credentials service with encryption in `services/admin-api-service/internal/services/models/credentials.go`
- [ ] T-S020-P05-050 [P] [US2] Implement POST /credentials handler in `services/admin-api-service/internal/handlers/models/credentials.go`
- [ ] T-S020-P05-051 [P] [US2] Implement GET /credentials handler in `services/admin-api-service/internal/handlers/models/credentials.go`
- [ ] T-S020-P05-052 [P] [US2] Implement POST /credentials/{type}/test handler in `services/admin-api-service/internal/handlers/models/credentials.go`

### CLI Implementation for US-002

- [ ] T-S020-P05-053 [US2] Implement credentials set command in `services/ai-aas-cli/cmd/credentials.go`
- [ ] T-S020-P05-054 [US2] Implement credentials list command in `services/ai-aas-cli/cmd/credentials.go`
- [ ] T-S020-P05-055 [US2] Implement credentials test command in `services/ai-aas-cli/cmd/credentials.go`
- [ ] T-S020-P05-056 [US2] Implement credentials delete command in `services/ai-aas-cli/cmd/credentials.go`

**Checkpoint**: Credentials can be stored and validated independently

---

## Phase 6: US-003 - Model Caching (Priority: P1)

**Goal**: Platform admins can download models from HF to object storage for fast deployments

**Independent Test**: Run `model pull`, `model cache list`, `model cache verify` - verify files in S3

### Tests for US-003

- [ ] T-S020-P06-057 [P] [US3] Unit test for HF download with resume in `services/ai-aas-cli/internal/huggingface/download_test.go`
- [ ] T-S020-P06-058 [P] [US3] Unit test for S3 multipart upload in `services/ai-aas-cli/internal/storage/s3_test.go`
- [ ] T-S020-P06-059 [P] [US3] Unit test for manifest operations in `services/ai-aas-cli/internal/storage/manifest_test.go`

### Admin API Implementation for US-003

- [ ] T-S020-P06-060 [P] [US3] Implement cache service in `services/admin-api-service/internal/services/models/cache.go`
- [ ] T-S020-P06-061 [P] [US3] Implement POST /models/{name}/pull handler in `services/admin-api-service/internal/handlers/models/pull.go`
- [ ] T-S020-P06-062 [P] [US3] Implement GET /models/{name}/cache handler in `services/admin-api-service/internal/handlers/models/cache.go`
- [ ] T-S020-P06-063 [P] [US3] Implement POST /models/{name}/cache/verify handler in `services/admin-api-service/internal/handlers/models/cache.go`

### CLI Implementation for US-003

- [ ] T-S020-P06-064 [P] [US3] Implement HF model download with resume in `services/ai-aas-cli/internal/huggingface/download.go`
- [ ] T-S020-P06-065 [P] [US3] Implement S3 client with multipart upload in `services/ai-aas-cli/internal/storage/s3.go`
- [ ] T-S020-P06-066 [P] [US3] Implement manifest generation/verification in `services/ai-aas-cli/internal/storage/manifest.go`
- [ ] T-S020-P06-067 [US3] Implement model pull command with progress in `services/ai-aas-cli/cmd/model/pull.go`
- [ ] T-S020-P06-068 [US3] Implement model cache list command in `services/ai-aas-cli/cmd/model/cache.go`
- [ ] T-S020-P06-069 [US3] Implement model cache verify command in `services/ai-aas-cli/cmd/model/cache.go`
- [ ] T-S020-P06-070 [US3] Implement model cache delete command in `services/ai-aas-cli/cmd/model/cache.go`
- [ ] T-S020-P06-071 [US3] Implement model cache gc command in `services/ai-aas-cli/cmd/model/cache.go`

**Checkpoint**: Models can be pulled from HF and cached in S3 independently

---

## Phase 7: US-004 - Model Deployment (Priority: P2)

**Goal**: Platform admins can deploy cached models to Kubernetes with KServe

**Independent Test**: Run `model deploy`, watch status, verify pod running and inference working

### Tests for US-004

- [ ] T-S020-P07-072 [P] [US4] Unit test for InferenceService generation in `services/ai-aas-cli/internal/kubernetes/inference_test.go`
- [ ] T-S020-P07-073 [P] [US4] Integration test for deployment workflow in `services/ai-aas-cli/tests/integration/model_workflow_test.go`

### Admin API Implementation for US-004

- [ ] T-S020-P07-074 [P] [US4] Implement deployment service in `services/admin-api-service/internal/services/models/deployment.go`
- [ ] T-S020-P07-075 [P] [US4] Implement POST /deployments handler in `services/admin-api-service/internal/handlers/models/deploy.go`
- [ ] T-S020-P07-076 [P] [US4] Implement GET /deployments handler in `services/admin-api-service/internal/handlers/models/deploy.go`
- [ ] T-S020-P07-077 [P] [US4] Implement DELETE /deployments/{id} handler in `services/admin-api-service/internal/handlers/models/deploy.go`

### CLI Implementation for US-004

- [ ] T-S020-P07-078 [P] [US4] Implement K8s client wrapper in `services/ai-aas-cli/internal/kubernetes/client.go`
- [ ] T-S020-P07-079 [P] [US4] Implement InferenceService operations in `services/ai-aas-cli/internal/kubernetes/inference.go`
- [ ] T-S020-P07-080 [P] [US4] Implement wait-for-ready helpers in `services/ai-aas-cli/internal/kubernetes/wait.go`
- [ ] T-S020-P07-081 [US4] Implement model deploy command in `services/ai-aas-cli/cmd/model/deploy.go`
- [ ] T-S020-P07-082 [US4] Implement model undeploy command in `services/ai-aas-cli/cmd/model/undeploy.go`
- [ ] T-S020-P07-083 [US4] Implement model restart command in `services/ai-aas-cli/cmd/model/deploy.go`
- [ ] T-S020-P07-084 [US4] Implement model scale command in `services/ai-aas-cli/cmd/model/deploy.go`

**Checkpoint**: Models can be deployed to K8s and managed independently

---

## Phase 8: US-005 - Model Validation & Audit (Priority: P2)

**Goal**: Platform admins can validate models across all layers and diagnose issues

**Independent Test**: Run `model validate` - verify all checks pass/fail appropriately with remediation

### Tests for US-005

- [ ] T-S020-P08-085 [P] [US5] Unit test for validation framework in `services/ai-aas-cli/internal/validation/validator_test.go`
- [ ] T-S020-P08-086 [P] [US5] Unit test for individual check implementations in `services/ai-aas-cli/internal/validation/checks_test.go`

### Admin API Implementation for US-005

- [ ] T-S020-P08-087 [P] [US5] Implement validation service in `services/admin-api-service/internal/services/models/validation.go`
- [ ] T-S020-P08-088 [US5] Implement POST /models/{name}/validate handler in `services/admin-api-service/internal/handlers/models/validate.go`

### CLI Implementation for US-005

- [ ] T-S020-P08-089 [P] [US5] Implement validation framework in `services/ai-aas-cli/internal/validation/validator.go`
- [ ] T-S020-P08-090 [P] [US5] Implement registry checks in `services/ai-aas-cli/internal/validation/registry.go`
- [ ] T-S020-P08-091 [P] [US5] Implement cache checks in `services/ai-aas-cli/internal/validation/cache.go`
- [ ] T-S020-P08-092 [P] [US5] Implement deployment checks in `services/ai-aas-cli/internal/validation/deployment.go`
- [ ] T-S020-P08-093 [P] [US5] Implement endpoint checks in `services/ai-aas-cli/internal/validation/endpoint.go`
- [ ] T-S020-P08-094 [P] [US5] Implement router checks in `services/ai-aas-cli/internal/validation/router.go`
- [ ] T-S020-P08-095 [US5] Implement model validate command in `services/ai-aas-cli/cmd/model/validate.go`

**Checkpoint**: Full-stack validation works independently

---

## Phase 9: US-008 - Model Enable/Disable (Priority: P2)

**Goal**: Platform admins can enable/disable models for capacity management without losing cache

**Independent Test**: Run `model disable`, verify undeployed but cached, `model enable`, verify quick redeploy

### Tests for US-008

- [ ] T-S020-P09-096 [P] [US8] Integration test for enable/disable workflow in `services/ai-aas-cli/tests/integration/library_test.go`

### Admin API Implementation for US-008

- [ ] T-S020-P09-097 [P] [US8] Implement state history service in `services/admin-api-service/internal/services/models/state.go`
- [ ] T-S020-P09-098 [P] [US8] Implement POST /deployments/{id}/enable handler in `services/admin-api-service/internal/handlers/models/enable.go`
- [ ] T-S020-P09-099 [P] [US8] Implement POST /deployments/{id}/disable handler in `services/admin-api-service/internal/handlers/models/enable.go`

### CLI Implementation for US-008

- [ ] T-S020-P09-100 [US8] Implement model enable command in `services/ai-aas-cli/cmd/model/enable.go`
- [ ] T-S020-P09-101 [US8] Implement model disable command in `services/ai-aas-cli/cmd/model/enable.go`
- [ ] T-S020-P09-102 [US8] Implement model library command in `services/ai-aas-cli/cmd/model/library.go`
- [ ] T-S020-P09-103 [US8] Implement model swap command in `services/ai-aas-cli/cmd/model/swap.go`
- [ ] T-S020-P09-104 [US8] Implement model history command in `services/ai-aas-cli/cmd/model/history.go`

**Checkpoint**: Library management works independently

---

## Phase 10: US-006 - Model Updates (Priority: P3)

**Goal**: Platform admins can check for and apply model updates with controlled rollout

**Independent Test**: Run `model check-updates`, `model update` - verify new version deployed

### Tests for US-006

- [ ] T-S020-P10-105 [P] [US6] Unit test for update checking in `services/ai-aas-cli/internal/huggingface/updates_test.go`

### CLI Implementation for US-006

- [ ] T-S020-P10-106 [P] [US6] Implement HF update checking in `services/ai-aas-cli/internal/huggingface/updates.go`
- [ ] T-S020-P10-107 [US6] Implement model check-updates command in `services/ai-aas-cli/cmd/model/update.go`
- [ ] T-S020-P10-108 [US6] Implement model update command in `services/ai-aas-cli/cmd/model/update.go`
- [ ] T-S020-P10-109 [US6] Implement model pin command in `services/ai-aas-cli/cmd/model/update.go`
- [ ] T-S020-P10-110 [US6] Implement model unpin command in `services/ai-aas-cli/cmd/model/update.go`

**Checkpoint**: Update workflows work independently

---

## Phase 11: US-007 - Troubleshooting (Priority: P3)

**Goal**: Platform admins can diagnose model issues using integrated commands

**Independent Test**: Run `model logs`, `model events`, `model test` - verify output

### Tests for US-007

- [ ] T-S020-P11-111 [P] [US7] Integration test for logs/events/describe commands in `services/ai-aas-cli/tests/integration/troubleshooting_test.go`

### CLI Implementation for US-007

- [ ] T-S020-P11-112 [P] [US7] Implement model logs command in `services/ai-aas-cli/cmd/model/logs.go`
- [ ] T-S020-P11-113 [P] [US7] Implement model events command in `services/ai-aas-cli/cmd/model/logs.go`
- [ ] T-S020-P11-114 [P] [US7] Implement model describe command in `services/ai-aas-cli/cmd/model/logs.go`
- [ ] T-S020-P11-115 [US7] Implement model test command in `services/ai-aas-cli/cmd/model/test.go`

**Checkpoint**: Troubleshooting commands work independently

---

## Phase 12: US-009 - Model Aliases (Priority: P3)

**Goal**: Platform admins can create aliases for models

**Independent Test**: Run `model alias create`, `model info <alias>` - verify resolution

### Tests for US-009

- [ ] T-S020-P12-120 [P] [US9] Contract test for alias endpoints in `services/ai-aas-cli/tests/contract/aliases_test.go`

### Admin API Implementation for US-009

- [ ] T-S020-P12-121 [P] [US9] Implement alias service in `services/admin-api-service/internal/services/models/alias.go`
- [ ] T-S020-P12-122 [P] [US9] Implement alias handlers in `services/admin-api-service/internal/handlers/models/alias.go`

### CLI Implementation for US-009

- [ ] T-S020-P12-123 [US9] Implement model alias create command in `services/ai-aas-cli/cmd/model/alias.go`
- [ ] T-S020-P12-124 [US9] Implement model alias list command in `services/ai-aas-cli/cmd/model/alias.go`
- [ ] T-S020-P12-125 [US9] Implement model alias update command in `services/ai-aas-cli/cmd/model/alias.go`
- [ ] T-S020-P12-126 [US9] Implement model alias delete command in `services/ai-aas-cli/cmd/model/alias.go`

**Checkpoint**: Alias management works independently

---

## Phase 13: Polish & Cross-Cutting Concerns

**Purpose**: Documentation, cleanup, and final polish

### Documentation

- [ ] T-S020-P13-127 [P] Review and update `docs/` folder with ai-aas-cli documentation
- [ ] T-S020-P13-128 [P] Create `docs/guides/ai-aas-cli-guide.md` user guide
- [ ] T-S020-P13-129 [P] Update `docs/runbooks/` with model management runbooks
- [ ] T-S020-P13-130 [P] Update existing docs that reference admin-cli to ai-aas-cli
- [ ] T-S020-P13-131 [P] Add CLI command reference to `docs/platform/ai-aas-cli-reference.md`
- [ ] T-S020-P13-132 Update README.md with ai-aas-cli installation and usage

### Testing & Quality

- [ ] T-S020-P13-133 [P] Create E2E test for full lifecycle in `services/ai-aas-cli/tests/e2e/full_lifecycle_test.go`
- [ ] T-S020-P13-134 [P] Performance benchmark for model list (<1s), validate (<30s) operations in `services/ai-aas-cli/tests/perf/benchmark_test.go`
- [ ] T-S020-P13-135 Run all tests with ≥80% coverage gate (`go test -cover`) and fix failures
- [ ] T-S020-P13-136 Run quickstart.md validation manually

### Build & Distribution

- [ ] T-S020-P13-137 [P] Add cross-compilation targets to Makefile
- [ ] T-S020-P13-138 [P] Create release workflow for CLI binaries
- [ ] T-S020-P13-139 Update root Makefile with ai-aas-cli targets

---

## Dependencies & Execution Order

### Phase Dependencies

```
Phase 1 (Setup) ─────────────────────────────────────────┐
                                                          │
Phase 2 (Foundational) ◄──────────────────────────────────┘
    │
    ├── DB Migrations (parallel)
    ├── Admin API Structure (parallel)
    └── CLI Core Infrastructure (parallel)
    │
    ▼ BLOCKS ALL USER STORIES
    │
    ├── Phase 3 (US-000 Init) ────┐
    ├── Phase 4 (US-001 Registry) │ P1 Stories
    ├── Phase 5 (US-002 Creds)    │ (can parallelize)
    └── Phase 6 (US-003 Cache) ───┘
        │
        ▼ US-003 must complete before US-004
        │
        ├── Phase 7 (US-004 Deploy) ──┐
        ├── Phase 8 (US-005 Validate) │ P2 Stories
        └── Phase 9 (US-008 Enable)───┘
            │
            ▼ Deployment required for P3 features
            │
            ├── Phase 10 (US-006 Updates) ─┐
            ├── Phase 11 (US-007 Troubleshoot) │ P3 Stories
            └── Phase 12 (US-009 Aliases) ─────┘
                │
                ▼
            Phase 13 (Polish)
```

### User Story Dependencies

| Story | Depends On | Notes |
|-------|------------|-------|
| US-000 | Foundational | Can start immediately after Phase 2 |
| US-001 | Foundational | Can run parallel with US-000 |
| US-002 | Foundational | Can run parallel with US-000, US-001 |
| US-003 | US-002 | Needs credentials for HF/S3 access |
| US-004 | US-003 | Needs cached model to deploy |
| US-005 | US-004 | Validation checks deployment |
| US-008 | US-004 | Enable/disable affects deployment |
| US-006 | US-003 | Updates require caching |
| US-007 | US-004 | Troubleshooting requires deployment |
| US-009 | US-001 | Aliases reference registry |

### Parallel Opportunities

**Within Phase 2 (Foundational)**:
- All 7 DB migrations can run in parallel
- Admin API structure tasks can run in parallel
- Output formatters can run in parallel

**P1 Stories (after Foundational)**:
- US-000 and US-001 can run in parallel
- US-002 can run in parallel with US-000, US-001
- US-003 must wait for US-002 (credentials)

**P2 Stories (after US-003)**:
- US-004, US-005, US-008 can mostly run in parallel
- Some validation checks depend on deployment existing

**P3 Stories (after US-004)**:
- US-006, US-007, US-009 can run in parallel

---

## Parallel Example: Phase 2 Migrations

```bash
# All migrations in parallel:
T-S020-P02-006 Create model_registry migration
T-S020-P02-007 Create model_cache migration  
T-S020-P02-008 Create model_deployments migration
T-S020-P02-009 Create model_aliases migration
T-S020-P02-010 Create model_state_history migration
T-S020-P02-011 Create model_validations migration
T-S020-P02-012 Create platform_credentials migration
```

## Parallel Example: US-001 Models

```bash
# Parallel model tasks within US-001:
T-S020-P04-040 Implement HF Hub API client
T-S020-P04-041 Implement license/gating detection

# Parallel command implementations:
T-S020-P04-042 model add command
T-S020-P04-043 model list command
T-S020-P04-044 model info command
```

---

## Implementation Strategy

### MVP First (US-000 + US-001 + US-002 + US-003)

1. Complete Phase 1: Setup (5 tasks)
2. Complete Phase 2: Foundational (15 tasks)
3. Complete Phase 3: US-000 CLI Init (11 tasks)
4. Complete Phase 4: US-001 Registry (15 tasks)
5. Complete Phase 5: US-002 Credentials (10 tasks)
6. Complete Phase 6: US-003 Caching (15 tasks)
7. **STOP and VALIDATE**: Test full caching workflow
8. **MVP COMPLETE**: CLI can init, register, cache models

### Incremental Delivery

| Milestone | Stories | Value Delivered |
|-----------|---------|-----------------|
| MVP | US-000, US-001, US-002, US-003 | Init, registry, credentials, caching |
| v1.0 | + US-004 | Full deployment lifecycle |
| v1.1 | + US-005, US-008 | Validation, library management |
| v1.2 | + US-006, US-007, US-009 | Updates, troubleshooting, aliases |

---

## Summary

| Metric | Count |
|--------|-------|
| **Total Tasks** | 141 |
| **Phase 1 (Setup)** | 5 |
| **Phase 2 (Foundational)** | 17 |
| **US-000 (Init)** | 11 |
| **US-001 (Registry)** | 15 |
| **US-002 (Credentials)** | 10 |
| **US-003 (Caching)** | 15 |
| **US-004 (Deployment)** | 13 |
| **US-005 (Validation)** | 11 |
| **US-008 (Enable/Disable)** | 9 |
| **US-006 (Updates)** | 6 |
| **US-007 (Troubleshooting)** | 5 |
| **US-009 (Aliases)** | 7 |
| **Phase 13 (Polish)** | 13 |
| **Parallel Opportunities** | 70 tasks marked [P] |

---

## Notes

- [P] tasks = different files, no dependencies, can run simultaneously
- [USx] label maps task to specific user story
- Each user story checkpoint = independently testable increment
- Commit after each task or logical group
- Run `make test` after each phase
- Documentation tasks (T-S020-P13-122 to T-S020-P13-127) ensure `/docs` is updated

