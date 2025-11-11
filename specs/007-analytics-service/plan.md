# Implementation Plan: Analytics Service

**Branch**: `007-analytics-service` | **Date**: 2025-11-11 | **Spec**: `/specs/007-analytics-service/spec.md`  
**Input**: Feature specification from `/specs/007-analytics-service/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Deliver a multi-tenant analytics subsystem that ingests inference usage events from the API Router, deduplicates and aggregates them in TimescaleDB, and exposes REST + CSV export interfaces for cost, reliability, and performance insights. The service will run as a Go microservice with background workers consuming RabbitMQ topics, scheduled aggregation jobs, and read APIs that feed Grafana dashboards and finance exports. Observability, freshness indicators, and data quality checks ensure stakeholders can trust spend and reliability trends within five minutes of ingestion.

## Technical Context

**Language/Version**: Go 1.21, SQL (PostgreSQL 15 + TimescaleDB 2.13), Bash 5.2 for runbooks  
**Primary Dependencies**: RabbitMQ 3.12 streams, `shared/go` libraries (auth, config, observability), `github.com/go-chi/chi/v5`, `github.com/jackc/pgx/v5`, `github.com/pressly/goose` for migrations, Grafana dashboards pipeline, dbt-style SQL transforms under `analytics/transforms`  
**Storage**: Analytics schema within PostgreSQL 15 (Timescale hypertables) for raw events and rollups; Redis 7 for short-lived cache of freshness indicators; S3-compatible object storage for archived CSV exports  
**Testing**: Go unit tests with `go test`, integration tests using Testcontainers (PostgreSQL + RabbitMQ), data validation suites in `analytics/tests` invoked via `scripts/analytics/verify.sh`, contract tests in `tests/analytics/contract` comparing API responses to OpenAPI contracts  
**Target Platform**: Akamai LKE clusters (development/production) with GitHub Actions CI; developers on macOS/Linux via `make analytics-*` targets  
**Project Type**: Backend service with background workers and scheduled jobs; complements shared data pipelines and dashboards  
**Performance Goals**: Sustain 3k events/sec steady with 5x burst tolerance; rollups deliver ≤2s query latency for 7-day windows; freshness indicator ≤5 minutes lag for 95% of partitions; exports generated within 60 seconds for 31-day range  
**Constraints**: All ingestion async via RabbitMQ; no cross-org leakage; dedupe drift <0.5%; GitOps-managed config; only documented REST APIs; background jobs idempotent; budgets and alert thresholds configurable via GitOps  
**Scale/Scope**: 250 orgs, 40 models, 30-day detailed partitions + 13-month summarized retention; initial release focuses on org-level dashboards and finance exports with hooks for per-user extensions

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- **API-First Interfaces**: Plan delivers OpenAPI contracts for ingestion health, aggregates, and export endpoints under `specs/007-analytics-service/contracts/`. Dashboards and CLI rely on these APIs only.  
- **Stateless Microservices & Async Non-Critical Paths**: Analytics service runs stateless pods with configuration in Git; raw events persist in Timescale; aggregation/exports operate asynchronously via RabbitMQ + scheduler.  
- **Security by Default**: Shared auth middleware, per-org RBAC scopes, audit logging, encrypted exports in S3, NetworkPolicies restricting DB/queue access, secrets sourced from sealed secrets.  
- **Declarative Infrastructure & GitOps**: Deployments via Helm/ArgoCD; RabbitMQ routing keys and Timescale hypertables managed through Terraform + migration tooling; dashboards versioned in Git.  
- **Observability, Testing, and Performance SLOs**: OpenTelemetry instrumentation, Prometheus metrics for lag/dedupe/error budgets, synthetic checks for dashboards, data quality tests in CI/CD.  
- **Testing Discipline**: Unit + integration tests (Testcontainers), end-to-end reconciliation in `analytics/tests`, contract tests verifying OpenAPI compliance, nightly load tests for freshness.  
- **Performance & Freshness**: Burst handling via RabbitMQ prefetch/backpressure, Timescale continuous aggregates, and caching of latest snapshots to meet ≤2s latency. No waivers requested.

## Project Structure

### Documentation (this feature)

```text
specs/007-analytics-service/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   ├── analytics-events-openapi.yaml
│   ├── analytics-views-openapi.yaml
│   └── analytics-exports-openapi.yaml
└── tasks.md                # generated via /speckit.tasks
```

### Source Code (repository root)

```text
services/
└── analytics-service/
    ├── cmd/
    │   └── analytics-service/main.go
    ├── internal/
    │   ├── ingestion/
    │   ├── aggregation/
    │   ├── api/
    │   ├── exports/
    │   ├── freshness/
    │   └── config/
    ├── pkg/
    │   └── models/
    ├── Makefile
    ├── go.mod
    └── README.md

analytics/
├── pipelines/
│   ├── ingestion/
│   └── rollups/
├── transforms/
│   ├── hourly_rollup.sql
│   └── daily_rollup.sql
└── tests/
    ├── hourly_rollup.yml
    ├── daily_reconciliation_test.go
    └── reconciliation_test.go

db/
├── migrations/
│   └── analytics/
│       ├── 0001_init.sql
│       ├── 0002_rollups.sql
│       └── 0003_exports.sql
├── seeds/
│   └── analytics/
│       └── sample_usage.sql
└── tools/
    └── migrate/

scripts/
└── analytics/
    ├── run-hourly.sh
    ├── verify.sh
    └── export-refresh.sh

tests/
└── analytics/
    ├── contract/
    │   └── api_contract_test.go
    ├── integration/
    │   └── ingestion_roundtrip_test.go
    └── perf/
        └── freshness_benchmark_test.go
```

**Structure Decision**: Implement analytics as a dedicated Go service under `services/analytics-service` that depends on shared libraries. Timescale SQL artifacts and QA harnesses live under top-level `analytics/` to share pipelines and tests with data engineering tasks. Database migrations and seeds extend the existing `db` layout, while scripts under `scripts/analytics/` orchestrate scheduled jobs and verification. Tests follow repository conventions inside `tests/analytics/` for contract, integration, and performance coverage.

## Complexity Tracking

No constitution violations introduced; architecture follows standard Go microservice pattern with existing analytics pipelines and GitOps tooling.
