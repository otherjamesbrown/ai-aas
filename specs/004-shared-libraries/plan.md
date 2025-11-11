# Implementation Plan: Shared Libraries & Conventions

**Branch**: `004-development` | **Date**: 2025-11-11 | **Spec**: `/specs/004-shared-libraries/spec.md`  
**Input**: Feature specification from `/specs/004-shared-libraries/spec.md`

**Note**: Generated via `/speckit.plan` workflow (manual execution).

## Summary

Deliver a set of reusable Go and TypeScript libraries plus scaffolding assets that standardize authentication, authorization, configuration, logging, metrics, tracing, data access, and error handling. The solution will publish semver packages through internal registries, instrument telemetry with OpenTelemetry, enforce policy evaluation via OPA bundles, and ship upgrade checklists, sample services, observability dashboards, and pilot adoption playbooks so new or existing services can onboard in under 60 minutes while reducing duplicated boilerplate by at least 30%.

## Technical Context

**Language/Version**: Go 1.21+, TypeScript 5.x (Node.js 20 LTS)  
**Primary Dependencies**: OpenTelemetry SDKs (Go/TS), OPA (Rego) policy bundles, `zap` (`slog` adaptor) for Go logging sinks, `pino` for Node logging sinks, `cobra` CLI for sample tooling, `dotenv` for config fallbacks  
**Storage**: No persistent storage introduced; relies on existing platform telemetry backends and identity providers  
**Testing**: Go: `go test` with contract/integration suites; TypeScript: `vitest` + `tsd` for type assertions; shared sample service integration tests via GitHub Actions matrix  
**Target Platform**: Kubernetes-hosted services running on Linux AMD64/ARM64; developer workstations (macOS, Linux, WSL2)  
**Project Type**: Polyglot library packages with sample service scaffold and documentation artifacts  
**Performance Goals**: Shared middleware adds ≤5% latency overhead at p95 under 500 RPS; telemetry export completes within 200 ms flush window; auth decisions evaluated in <5 ms at p95  
**Constraints**: Must operate without mandatory external network calls (graceful degradation if telemetry or policy distribution offline); configuration loader must boot with defaults without filesystem reliance; libraries must not require privileged OS access  
**Scale/Scope**: Designed for 8–12 services in first wave, supporting up to 30 services; policy bundles cover 5 core roles with extensibility; telemetry volume sized for 10k RPS across adopters

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Constitution file contains placeholders (`.specify/memory/constitution.md`); until governance fills it, use platform gates defined in the specs upgrade playbook (API-first, Security, Observability, Reliability, Governance).  
- **API-first**: Libraries expose language-agnostic contracts (error schema, config shapes) and sample OpenAPI fragments; no service-level API changes introduced.  
- **Security**: Authorization middleware integrates OPA bundles, enforces least privilege, audit logs, and secrets handling via environment/Vault providers.  
- **Observability**: OpenTelemetry instrumentation, default dashboards, required log fields, and alerting recommendations satisfy observability gate.  
- **Reliability**: Graceful degradation for telemetry sinks, health/readiness checks, and config validation meet reliability expectations.  
- **Governance**: Semver versioning, upgrade checklists, and compatibility tests align with change-management expectations.  
No violations identified; revisit after design artifacts are generated to ensure dashboards/checklists fully cover observability.

## Project Structure

### Documentation (this feature)

```text
specs/004-shared-libraries/
├── plan.md              # This file (/speckit.plan workflow output)
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by plan workflow)
```

### Source Code (repository root)
```text
shared/
├── go/
│   ├── auth/                  # Authorization middleware, policy loader
│   ├── config/                # Env/Vault configuration helpers
│   ├── observability/         # Logging, metrics, tracing instrumentation
│   ├── errors/                # Standardized error types & encoders
│   ├── dataaccess/            # Health checks, DB helpers, connection guards
│   └── internal/              # Shared utils (kept private)
├── ts/
│   ├── auth/
│   ├── config/
│   ├── observability/
│   ├── errors/
│   ├── dataaccess/
│   └── internal/
├── policies/
│   └── bundles/               # Rego bundles & distribution metadata
├── dashboards/
│   ├── grafana/               # JSON dashboards for logs/metrics/traces
│   └── alerts/                # Alertmanager/SLO templates
└── samples/
    └── service-template/      # Reference service demonstrating library usage

tests/
├── go/
│   ├── unit/
│   ├── contract/
│   ├── integration/
│   └── perf/
├── ts/
│   ├── unit/
│   ├── contract/
│   ├── integration/
│   └── perf/
└── samples/
    └── e2e/                   # End-to-end tests against sample service

docs/
├── adoption/                   # Pilot plans and measured outcomes
├── perf/                       # Benchmark reports
└── runbooks/                   # Operational guides for shared libraries
```

**Structure Decision**: Introduce a top-level `shared/` directory housing the Go and TypeScript packages, shared policy bundles, observability dashboard assets, and a sample service under `samples/service-template`. Language-specific tests remain co-located under `tests/` to preserve go/ts tooling conventions, while sample E2E flows validate cross-language integration. Documentation artifacts live within `specs/004-shared-libraries/` per template.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| _None_ | – | – |
