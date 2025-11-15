# Implementation Plan: Admin CLI

**Branch**: `009-admin-cli` | **Date**: 2025-11-08 | **Spec**: `/specs/009-admin-cli/spec.md`  
**Input**: Feature specification from `/specs/009-admin-cli/spec.md`

**Note**: Generated via `/speckit.plan` workflow (manual execution).

## Summary

Deliver a command-line tool for platform administrators to perform privileged operations quickly and safely: bootstrap the first admin, manage organizations and users, rotate credentials, trigger syncs, and export reports. The solution provides a Go-based CLI using Cobra for command parsing, structured output (JSON/CSV/table), dry-run and confirmation safeguards, audit logging, and health checks for service dependencies. The outcome is reliable, scriptable operations with predictable exit codes and clear error messages suitable for automation and CI/CD integration.

## Technical Context

**Language/Version**: Go 1.24+, Cobra CLI framework v1.7+, Go 1.24+ standard library  
**Primary Dependencies**: `cobra` for command parsing, `viper` for configuration management, REST client libraries for service API consumption, JSON/CSV encoders for structured output, progress indicator libraries for long-running operations  
**Storage**: Configuration stored in `~/.admin-cli/config.yaml` or environment variables; no persistent storage introduced (CLI is stateless; uses service APIs for data)  
**Testing**: `go test ./...` with unit tests for command parsing, API client mocking, and output formatting; integration tests against local service instances  
**Target Platform**: Developer/admin workstations (Linux, macOS, Windows via WSL2/Git Bash); single Go binary distribution  
**Project Type**: Standalone CLI tool consumed by platform administrators and operators; co-located in monorepo under `services/admin-cli/`  
**Performance Goals**: Bootstrap completes in ≤2 minutes, single org/user operations in ≤5 seconds, batch operations process 100 items in ≤2 minutes, exports complete at ≤1 minute per 10k rows, help command responds in <1 second  
**Constraints**: Must call existing service APIs (user-org-service, analytics-service); no direct database access in normal operations; CLI must be scriptable (non-interactive mode, predictable exit codes, structured output); all privileged operations must emit audit logs  
**Scale/Scope**: Single binary CLI tool supporting bootstrap, org/user/key lifecycle, credential rotation, sync triggers, and exports; designed for administrator use cases, not end-user consumption

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Constitution v1.4.0 is ratified. This plan satisfies each gate:
  - **API-First**: CLI exclusively consumes existing REST APIs from user-org-service and analytics-service; no new API endpoints required. CLI is a thin client with no business logic.
  - **Stateless & Boundaries**: CLI is stateless; all data operations go through service APIs. No in-process state across operations; state managed by backend services.
  - **Async Non-Critical**: Export operations may trigger async jobs through analytics-service; CLI monitors progress but does not block on long-running operations.
  - **Security by Default**: All privileged operations emit audit logs with user identity, timestamp, command, and outcomes. Credentials masked in logs. Break-glass recovery requires explicit authentication. CLI validates authentication tokens before operations.
  - **Declarative Ops & GitOps**: CLI supports declarative batch operations via file input (JSON/YAML), enabling GitOps workflows. Dry-run mode enables preview before applying changes.
  - **Observability**: All operations emit structured logs (JSON format option) suitable for log aggregation. Long-running operations emit progress events. Audit logs capture operation duration and outcomes for correlation with service-side logs.
  - **Testing**: Unit tests for command parsing and output formatting; integration tests against local service instances; no DB mocks (uses service APIs).
  - **Performance**: Explicit SLO targets documented (bootstrap ≤2min, operations ≤5s, batch ≤2min for 100 items, exports ≤1min per 10k rows, help <1s).

No waivers required; implementation must adhere to these checkpoints.

## Project Structure

### Documentation (this feature)

```text
specs/009-admin-cli/
├── plan.md              # This file (/speckit.plan workflow output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output (if needed)
├── quickstart.md        # Phase 1 output (completed)
├── contracts/           # Phase 1 output (OpenAPI fragments if new endpoints needed)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by plan workflow)
```

### Source Code (repository root)

```text
services/admin-cli/
├── cmd/
│   └── admin-cli/
│       └── main.go              # CLI entrypoint, root command setup
├── internal/
│   ├── commands/                # Cobra command implementations
│   │   ├── bootstrap.go         # Bootstrap command (US-001)
│   │   ├── org.go               # Org lifecycle commands (US-002)
│   │   ├── user.go              # User lifecycle commands (US-002)
│   │   ├── credentials.go       # Credential rotation commands (US-001)
│   │   ├── sync.go              # Sync trigger/status commands (US-002)
│   │   └── export.go            # Export commands (US-003)
│   ├── client/                  # API client packages
│   │   ├── userorg/             # user-org-service API client
│   │   │   ├── client.go        # REST client implementation
│   │   │   ├── types.go         # Request/response types
│   │   │   └── auth.go          # Token validation with clock skew tolerance
│   │   └── analytics/           # analytics-service API client
│   │       ├── client.go
│   │       └── types.go
│   ├── config/                  # Configuration management
│   │   ├── config.go            # Config loading (viper-based)
│   │   └── defaults.go          # Default values
│   ├── output/                  # Output formatting
│   │   ├── table.go             # Table formatter
│   │   ├── json.go              # JSON formatter
│   │   └── csv.go               # CSV formatter
│   ├── audit/                   # Audit logging
│   │   └── logger.go            # Audit log emitter
│   ├── progress/                # Progress indicators
│   │   └── indicator.go         # Progress bar/events for long-running ops
│   └── health/                  # Health checks
│       └── checker.go           # Service dependency health checks
├── Makefile                     # Build/test/lint targets
├── go.mod                       # Go module definition
└── README.md                    # Service documentation
```

**Structure Decision**: CLI lives under `services/admin-cli/` following monorepo service pattern, with clear separation between commands (Cobra handlers), API clients (service integrations), configuration (viper-based), output formatting (table/JSON/CSV), audit logging, and progress indicators. Commands are organized by user story (bootstrap, org/user lifecycle, credentials, sync, export) for maintainability.

## Complexity Tracking

No constitution violations introduced. CLI is a thin client consuming existing service APIs; complexity is limited to command parsing, API client implementation, and output formatting. No additional justification required.

