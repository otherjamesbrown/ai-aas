# Implementation Plan: User-Level Model Access Control

**Branch**: `022-user-model-access-control` | **Date**: 2025-11-30 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/022-user-model-access-control/spec.md`

## Summary

Add fine-grained per-user model access control within organizations. Currently, model access is controlled at the organization level via routing policies—this enhancement enables org admins to restrict specific users to specific models, grant access to all current models (snapshot), or enable auto-grant mode for all current and future models.

## Technical Context

**Language/Version**: Go 1.21+ (existing platform standard)  
**Primary Dependencies**: chi router, pgx v5 (PostgreSQL), Redis 7+ (caching)  
**Storage**: PostgreSQL 15+ (primary), Redis 7+ (auth context cache with 30s TTL)  
**Testing**: Go testing + testcontainers (no DB mocks per constitution)  
**Target Platform**: Kubernetes (LKE)  
**Project Type**: Microservices extension (user-org-service + api-router-service)  
**Performance Goals**: Auth check latency <10ms additional overhead (in critical inference path)  
**Constraints**: 30-second cache TTL for access grants; cache invalidation on grant changes  
**Scale/Scope**: ~100 models, ~1000 users per org, ~100 orgs

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Design Check ✅ PASSED (2025-11-30)

| Gate | Status | Implementation |
|------|--------|----------------|
| **API-First** | ✅ PASS | OpenAPI contracts in `/contracts/`, CLI commands are thin clients calling user-org-service REST API |
| **Statelessness** | ✅ PASS | All state in PostgreSQL (grants, access modes) and Redis (30s cache); no in-memory state |
| **Async Non-Critical** | ✅ PASS | Access checks are synchronous (critical path); audit logging is async via existing queue |
| **Security** | ✅ PASS | Only org admins can manage grants (existing RBAC); grants in database; no secrets in code |
| **GitOps/Declarative** | ✅ PASS | Feature flag `USER_MODEL_ACCESS_ENABLED` for gradual rollout; migrations in `db/migrations/` |
| **Observability** | ✅ PASS | Access check metrics, grant change audit logs, cache hit/miss metrics |
| **Testing** | ✅ PASS | Unit tests for access logic, integration tests with testcontainers, E2E for grant workflow |
| **Performance** | ✅ PASS | <10ms overhead via Redis cache (30s TTL); include grants in auth response to avoid extra round-trip |

### Post-Design Check ✅ PASSED (2025-11-30)

| Gate | Status | Verification |
|------|--------|--------------|
| **API-First** | ✅ PASS | OpenAPI 3.1 spec generated in `contracts/openapi.yaml` with 6 endpoints |
| **Statelessness** | ✅ PASS | Data model uses existing PostgreSQL; 2 new tables with proper FKs; Redis 30s cache |
| **Async Non-Critical** | ✅ PASS | Access check is sync (hot path); grant changes use existing audit queue |
| **Security** | ✅ PASS | Org admin check via existing RBAC; no new secrets; grants auditable |
| **GitOps/Declarative** | ✅ PASS | Migration SQL provided; feature flag documented |
| **Observability** | ✅ PASS | Metrics endpoints existing; cache hit/miss trackable via existing patterns |
| **Testing** | ✅ PASS | Integration test patterns documented in quickstart.md; testcontainers approach |
| **Performance** | ✅ PASS | Cache strategy documented in research.md; <10ms additional latency |

## Project Structure

### Documentation (this feature)

```text
specs/022-user-model-access-control/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (OpenAPI specs)
└── tasks.md             # Phase 2 output (created by /speckit.tasks)
```

### Source Code (repository root)

```text
# Database migrations
db/migrations/operational/
├── 20251130001_user_model_access.up.sql
└── 20251130001_user_model_access.down.sql

# User-Org Service extensions
services/user-org-service/
├── internal/
│   ├── httpapi/
│   │   └── modelaccess/
│   │       ├── handlers.go        # REST API handlers
│   │       └── handlers_test.go
│   └── storage/
│       └── postgres/
│           ├── model_access.go    # Repository layer
│           └── model_access_test.go
└── migrations/sql/
    └── 000XXX_user_model_access.sql

# API Router Service extensions
services/api-router-service/
├── internal/
│   ├── auth/
│   │   └── authenticator.go       # Extend AuthenticatedContext
│   └── api/public/
│       └── middleware.go          # Add ModelAccessMiddleware
└── test/integration/
    └── model_access_test.go

# CLI extensions
services/ai-aas-cli/
└── cmd/user/
    └── model_access.go            # CLI commands
```

**Structure Decision**: Extend existing microservices (user-org-service, api-router-service, ai-aas-cli) rather than creating new services. Access control logic lives in user-org-service; enforcement in api-router-service.

## Complexity Tracking

> No constitution violations requiring justification.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | - | - |
