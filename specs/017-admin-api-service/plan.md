# Implementation Plan: Admin API Service

**Branch**: `017-admin-api-service` | **Date**: 2025-11-26 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/017-admin-api-service/spec.md`

## Summary

Provide an HTTP API service that admin-cli can communicate with instead of requiring direct database access. The service enables secure, audited administrative operations for model registry management, organization management, and routing policy configuration. Built as a Go microservice using existing shared libraries, deployed as a Kubernetes ClusterIP service.

## Technical Context

**Language/Version**: Go 1.21+ (consistent with existing services)
**Primary Dependencies**: net/http, chi router, shared/go/dbutil, shared/go/middleware, OpenTelemetry, Prometheus client
**Storage**: PostgreSQL (existing platform database)
**Testing**: Go testing + testcontainers for integration tests
**Target Platform**: Kubernetes (Linux containers)
**Project Type**: Go microservice (following services/_template pattern)
**Performance Goals**: 200ms p95 for single-record operations, 100 req/s sustained
**Constraints**: <200ms p95 latency, graceful shutdown within 30s, rate limiting 100 req/min per API key
**Scale/Scope**: Internal admin operations, ~10-50 concurrent admin users

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

The constitution template is generic. Based on project conventions from CONTRIBUTING.md and existing services:

| Gate | Status | Notes |
|------|--------|-------|
| Follows microservice pattern | ✅ PASS | Uses services/_template structure |
| Uses shared libraries | ✅ PASS | Uses shared/go/* packages |
| Has comprehensive tests | ✅ PASS | Unit + integration tests planned |
| Has OpenAPI spec | ✅ PASS | contracts/ will contain OpenAPI 3.0 |
| Follows REST conventions | ✅ PASS | Standard REST with versioned paths |
| Has observability | ✅ PASS | Prometheus metrics + OpenTelemetry tracing |

## Project Structure

### Documentation (this feature)

```text
specs/017-admin-api-service/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (OpenAPI specs)
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
services/admin-api-service/
├── cmd/
│   └── admin-api/
│       └── main.go           # Service entrypoint
├── internal/
│   ├── api/
│   │   ├── handlers/         # HTTP handlers
│   │   │   ├── models.go
│   │   │   ├── organizations.go
│   │   │   ├── policies.go
│   │   │   ├── audit.go
│   │   │   └── health.go
│   │   ├── middleware/       # Auth, rate limiting, logging
│   │   └── router.go
│   ├── service/              # Business logic
│   │   ├── registry.go
│   │   ├── organizations.go
│   │   ├── policies.go
│   │   └── audit.go
│   ├── repository/           # Database access
│   │   ├── models.go
│   │   ├── organizations.go
│   │   ├── policies.go
│   │   └── audit.go
│   └── config/
│       └── config.go
├── k8s/
│   ├── deployment.yaml
│   ├── service.yaml
│   └── configmap.yaml
├── Dockerfile
├── go.mod
├── go.sum
└── README.md

tests/
├── integration/
│   └── admin-api/
│       ├── models_test.go
│       ├── organizations_test.go
│       └── policies_test.go
```

**Structure Decision**: Standard Go microservice following existing patterns in services/api-router-service and services/user-org-service.

## Complexity Tracking

No violations requiring justification. Design follows existing patterns and uses shared infrastructure.
