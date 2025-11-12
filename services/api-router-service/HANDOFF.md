# Handoff Document: API Router Service (Spec 006)

**Date**: 2025-01-27  
**Status**: Phase 3 Complete ✅, Ready for Phase 4 🚀  
**Branch**: `main` (or current working branch)

## Current Status

### ✅ Completed: Phase 3 - User Story 1 (Route authenticated inference requests)

All 7 tasks complete:
1. **T012 - Contract Tests** ✅ - OpenAPI schema validation
2. **T013 - Integration Tests** ✅ - End-to-end request flow
3. **T014 - Request DTOs** ✅ - Validation logic implemented
4. **T015 - Authentication** ✅ - API key auth (stubbed for dev)
5. **T016 - Backend Client** ✅ - HTTP forwarding with health checks
6. **T017 - Handler Pipeline** ✅ - Complete request flow
7. **T018 - Router Registration** ✅ - Routes registered with middleware

**See**: `handoff-006-phase03.md` for complete details

### ✅ Completed: Phase 2 - Foundational (Blocking Prerequisites)

All 5 foundational tasks are complete:

1. **T007 - Config Service Integration** ✅
   - Implemented etcd client integration in `internal/config/loader.go`
   - Real-time watch stream for configuration updates
   - Cache fallback when etcd unavailable
   - Comprehensive test suite in `internal/config/loader_test.go`
   - **Files**: `internal/config/loader.go`, `internal/config/loader_test.go`, `internal/config/README_TESTING.md`

2. **T008 - BoltDB Cache** ✅
   - Persistent configuration caching implemented
   - **Files**: `internal/config/cache.go`

3. **T009 - Telemetry Bootstrap** ✅
   - OpenTelemetry and zap logger integration complete
   - **Files**: `internal/telemetry/telemetry.go`

4. **T010 - Middleware Stack** ✅
   - Chi router with standard middleware (RequestID, Logger, Recoverer, Timeout)
   - **Files**: `cmd/router/main.go`

5. **T011 - Contract Generation** ✅
   - OpenAPI validation and Go type generation using oapi-codegen
   - CLI tool at `cmd/contracts/main.go`
   - Makefile targets: `make contracts`, `make contracts-validate`, `make contracts-generate`
   - **Files**: `pkg/contracts/generate.go`, `cmd/contracts/main.go`, `pkg/contracts/README.md`

### ✅ Completed: Phase 7 - User Story 5 (Operational visibility and reliability)

All 5 tasks complete:
1. **T037 - Health Endpoint Tests** ✅ - Integration tests for health endpoints
2. **T038 - Status Handlers** ✅ - Health and readiness handlers with component probes
3. **T039 - Metrics Exporters** ✅ - Prometheus metrics exporters
4. **T040 - Grafana Dashboard** ✅ - Operational dashboard with 11 panels
5. **T041 - Runbooks** ✅ - Incident response runbooks

**See**: `handoff-006-phase07.md` for complete details

### 📋 Next Steps: Phase 8 - Polish & Cross-Cutting Concerns

**Goal**: Service-wide hardening, documentation, and release readiness

**Tasks to complete**:
- [ ] T042 [P] Harden error catalog and response codes
- [ ] T043 [P] Finalize Helm values and alert rules
- [ ] T044 [P] Tune load test scenarios for SLO coverage
- [ ] T045 [P] Extend smoke test coverage
- [ ] T046 Validate quickstart instructions

**See**: `handoff-006-phase08.md` for detailed Phase 8 handoff document

### 📋 Previous: Phase 4 - User Story 2 (Enforce budgets and safe usage)

**Goal**: Enforce budget, quota, and rate limits with auditable denials (Priority: P1)

**Tasks to complete**:
- [ ] T019 [P] [US2] Add integration test for quota exhaustion denial
- [ ] T020 [P] [US2] Implement budget service client integration
- [ ] T021 [P] [US2] Implement Redis token-bucket limiter wrapper
- [ ] T022 [P] [US2] Attach rate-limit and budget middleware to request pipeline
- [ ] T023 [US2] Emit audit events for deny and queue outcomes
- [ ] T024 [US2] Produce structured limit error responses and metrics

**Independent Test**: Simulate limit exhaustion and confirm HTTP 402/429 responses plus durable audit records

**See**: `handoff-006-phase04.md` for detailed Phase 4 handoff document

## Key Context & Decisions

### Architecture Decisions
- **Config Service**: Using etcd (default endpoint: `localhost:2379`)
- **Contract Generation**: Using `oapi-codegen` (not openapi-generator)
- **Testing Strategy**: Tests work with/without etcd (graceful fallback)
- **Cache Strategy**: BoltDB for local persistence, etcd for distributed config

### Important Files & Locations

**Core Implementation**:
- `services/api-router-service/cmd/router/main.go` - Main entrypoint
- `services/api-router-service/internal/config/` - Config loading & caching
- `services/api-router-service/internal/auth/authenticator.go` - Authentication (has TODOs for user-org-service integration)
- `services/api-router-service/internal/routing/backend_client.go` - Backend client (basic implementation exists)
- `services/api-router-service/internal/api/public/handler.go` - Public API handlers (has TODOs)

**Specification**:
- `specs/006-api-router-service/spec.md` - Feature specification
- `specs/006-api-router-service/tasks.md` - Task breakdown
- `specs/006-api-router-service/contracts/api-router.openapi.yaml` - OpenAPI contract

**Testing**:
- `services/api-router-service/internal/config/loader_test.go` - Config loader tests
- `services/api-router-service/test/contract/` - Contract tests (empty, needs T012)
- `services/api-router-service/test/integration/` - Integration tests (empty, needs T013)

### Dependencies & Prerequisites

**Required Tools**:
- `oapi-codegen`: `go install github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen@latest`
- `etcd`: For Config Service (optional, has fallback)
- `spectral`: Optional for better OpenAPI validation

**Service Dependencies**:
- **user-org-service**: For API key validation (currently stubbed in `authenticator.go`)
- **Config Service (etcd)**: For routing policies (has cache fallback)
- **Redis**: For rate limiting (not yet implemented)
- **Kafka**: For usage tracking (not yet implemented)

### Known TODOs & Incomplete Work

**In `internal/auth/authenticator.go`**:
- TODO: Replace stub API key validation with real user-org-service integration
- TODO: Implement HMAC signature verification
- TODO: Implement API key revocation/expiration checks

**In `internal/api/public/handler.go`**:
- TODO: Implement weighted backend selection
- TODO: Implement failover logic
- TODO: Get backend URI from config (currently hardcoded)

**In `cmd/router/main.go`**:
- TODO: Register admin routes (`/v1/admin/*`)
- TODO: Add Prometheus metrics handler
- TODO: Implement readiness checks for Redis, Kafka, config service

### Testing Status

✅ **Working**:
- Config loader tests (cache fallback, etcd integration when available)
- Contract validation
- Basic compilation

⏳ **Needs Implementation**:
- Contract tests (T012)
- Integration tests (T013)
- End-to-end inference routing tests

### How to Continue

1. **Start with T012**: Create contract test for `POST /v1/inference`
   - Location: `test/contract/inference_contract_test.go`
   - Should validate request/response schemas match OpenAPI spec

2. **Then T013**: Create integration test
   - Location: `test/integration/inference_success_test.go`
   - Should test full happy path: auth → routing → backend → response

3. **Then T014-T018**: Implement the actual functionality
   - Follow TDD: tests first, then implementation
   - Use the OpenAPI spec as the source of truth

### Useful Commands

```bash
# Generate contracts from OpenAPI spec
make contracts

# Validate OpenAPI spec only
make contracts-validate

# Run config loader tests
go test ./internal/config -v

# Run the service locally
make run

# Start dev dependencies (Redis, Kafka, etcd)
make dev-up
```

### Notes for Next Agent

- The codebase compiles and basic infrastructure is in place
- Config Service integration is production-ready with fallback
- Authentication is stubbed - needs real user-org-service integration
- Backend routing logic is simplified - needs weighted selection and failover
- Contract generation is ready - can generate Go types from OpenAPI spec
- Follow the task order in `specs/006-api-router-service/tasks.md`
- Each user story task has `[US#]` tag for tracking
- Priority tasks are marked with `[P]`

## Questions to Resolve

1. **User-org-service integration**: What's the actual endpoint/API for validating API keys?
2. **Backend configuration**: How are backend URIs stored in Config Service?
3. **Mock backends**: Should we use the docker-compose mock backends for testing?

---

**Ready to proceed with Phase 4: User Story 2 (Enforce budgets and safe usage)**

**📄 Detailed Phase 4 Handoff**: See `handoff-006-phase04.md` for complete implementation guide

