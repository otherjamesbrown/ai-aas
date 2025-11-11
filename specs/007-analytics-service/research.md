# Research Log: Analytics Service

**Date**: 2025-11-11  
**Owner**: Analytics service working group  
**Context**: Resolves technical unknowns captured in the implementation plan and prepares Phase 1 design artifacts.

---

## Decision: Event Ingestion & Transport
- **Decision**: Use RabbitMQ 3.12 streams (`analytics.usage.v1`) as the sole ingestion queue. API Router publishes inference usage, latency, and error events as JSON documents complying with `analytics-events-openapi.yaml`. Consumers acknowledge in batches of 500 with idempotent dedupe keys.
- **Rationale**: RabbitMQ is already constitution-mandated for async workloads. Streams provide exactly-once semantics with consumer offsets, satisfy 5x burst tolerance, and integrate cleanly with existing Helm charts.
- **Alternatives considered**:
  - **Direct PostgreSQL writes from API Router**: Rejected due to cross-service coupling and synchronous latency penalties.
  - **Kafka**: Higher operational overhead in LKE, no existing footprint, duplicates RabbitMQ responsibilities.

## Decision: Storage Engine & Schema
- **Decision**: Persist raw events and rollups in TimescaleDB hypertables within the `analytics` schema of PostgreSQL 15. Partition raw events by `occurred_at` (daily chunks) and compress after 7 days; continuous aggregates materialize hourly and daily rollups. Redis 7 caches latest freshness markers.
- **Rationale**: TimescaleDB supports time-series queries with low-latency aggregates. It aligns with constitution technology standards and existing DB toolchain (`db/tools/migrate`). Redis cache keeps freshness endpoint sub-second without overloading Postgres.
- **Alternatives considered**:
  - **ClickHouse**: Excellent at aggregates but adds non-standard datastore with no existing operations playbook.
  - **BigQuery / warehouse offload**: Violates GitOps-first requirement and introduces external billing complexity.

## Decision: Aggregation & Backfill Strategy
- **Decision**: Implement Go workers (`internal/aggregation`) that trigger Timescale continuous aggregates and schedule backfill jobs through CronJobs managed in ArgoCD. Late-arriving events flagged via `ingestion_batches` table and re-run through delta refresh pipelines every 15 minutes.
- **Rationale**: Keeps aggregation logic close to database while allowing reprocessing for late or deduped events. CronJobs are declarative, managed via GitOps, and integrate with existing `scripts/analytics/run-hourly.sh`.
- **Alternatives considered**:
  - **dbt incremental models**: Would require new tooling stack and additional CI/CD investment.
  - **Airflow**: Heavy-weight orchestrator, unnecessary for three pipelines, and not currently deployed.

## Decision: API Surface & Access Patterns
- **Decision**: Expose REST endpoints under `/analytics/v1/` for summaries, reliability metrics, and freshness indicators; all responses JSON with RFC7807 errors. Finance exports initiated via POST to `/analytics/v1/orgs/{orgId}/exports` and delivered as signed S3 URLs. Grafana dashboards consume `/analytics/v1/orgs/{orgId}/usage` and `/analytics/v1/orgs/{orgId}/reliability`.
- **Rationale**: Aligns with constitution API-first mandate, leverages shared Go HTTP stack, and keeps UI/CLI thin. Export flow decouples generation (async job) from delivery while maintaining auditable artifacts.
- **Alternatives considered**:
  - **GraphQL**: Flexible but overkill for known aggregations and adds new gateway surface area.
  - **Direct SQL access**: Violates API-first principle and weakens access control.

## Decision: Security, Observability, & Compliance
- **Decision**: Reuse `shared/go/auth` for JWT + API key verification, enforce org scoping in middleware, and write audit events to `services/analytics-service/internal/audit`. Metrics emitted via OpenTelemetry with spans covering ingestion-to-aggregate latency; dedupe, freshness, and export states exported as Prometheus metrics. Exports are encrypted at rest, signed URLs expire in 24 hours, and `gitleaks`/`trivy` run in CI.
- **Rationale**: Meets constitution security mandates, ensures multi-tenant isolation, and gives reliability teams actionable signals. Instrumentation matches existing observability tooling, easing dashboard creation.
- **Alternatives considered**:
  - **Homegrown auth**: Redundant and risks inconsistency with other services.
  - **Plain logs without metrics**: Slower detection of freshness regressions; violates observability gate.

## Decision: Export Cadence & Retention
- **Decision**: Support on-demand exports plus scheduled monthly generation delivered to an S3 prefix per org (`analytics/exports/{orgId}/{YYYYMM}.csv`). Retain exports for 90 days, after which jobs can regenerate from warehouse tables. Document overrides for orgs requiring extended retention.
- **Rationale**: Balances storage cost with finance needs, provides predictable cadence, and respects 13-month aggregate availability. Scheduled job integrates with GitOps Cron and ensures finance receives artifacts without manual triggers.
- **Alternatives considered**:
  - **Email delivery**: Insecure and complicates auditing.
  - **Infinite retention**: Increases storage costs; redundant because aggregates remain queryable.

## Decision: Future Per-User Drilldowns
- **Decision**: Defer per-user dashboards to follow-on work but design schema with optional `actor_id` column (nullable) and index to support later expansion without rework. Document backlog item in tasks.
- **Rationale**: Keeps current scope focused while preventing schema rewrites when requirement materializes.
- **Alternatives considered**:
  - **Immediate implementation**: Adds complexity without stakeholder commitment.
  - **Ignore entirely**: Risks costly migrations later.

---

All clarifications from the implementation plan are addressed; no outstanding `NEEDS CLARIFICATION` markers remain.

