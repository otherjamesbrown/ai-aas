## Research Summary

### Migration Toolchain & Versioning
- **Decision**: Adopt `golang-migrate` (CLI + Go library) as the primary migration executor with timestamped version directories and paired rollback scripts.
- **Rationale**: Aligns with Go-centric platform toolchain, integrates cleanly with containerized workflows, supports SQL + Go migrations, and exposes hooks for custom telemetry. Actively maintained and widely adopted in cloud-native stacks.
- **Alternatives considered**: Liquibase (powerful diff tooling but heavier JVM dependency, slower spin-up in CI); Flyway (similar capabilities but less flexible for Go embedding and custom telemetry instrumentation).

### Migration Telemetry & Observability
- **Decision**: Emit structured logs and metrics via the existing platform observability SDK, publishing to OpenTelemetry collectors with labels for migration version, status, duration, and affected tables.
- **Rationale**: Reuses platform-standard telemetry stack; enables dashboards/alerts defined in Grafana and supports constitution observability gate. Minimal additional infrastructure required.
- **Alternatives considered**: Custom logging-only solution (insufficient alerting/metrics integration), standalone metrics push gateway (extra component to maintain, limited correlation with existing traces).

### Analytics Warehouse Target
- **Decision**: Target the managed warehouse used by analytics teams (BigQuery-compatible interface) with dbt-style SQL transforms for hourly/daily rollups.
- **Rationale**: Warehouse already provisioned for observability dashboards; dbt-style transforms enable versioned SQL, testing, and documentation. Works with incremental models and partition pruning needed for performance NFRs.
- **Alternatives considered**: Snowflake (higher cost, additional governance setup), self-managed ClickHouse (higher operational burden, diverges from managed-first strategy).

### Seed Data Strategy
- **Decision**: Provide Go-based deterministic seed scripts for operational data (organizations, users, API keys) plus SQL fixtures for analytics usage events, both idempotent via natural keys.
- **Rationale**: Go scripts leverage existing shared libraries (hashing, UUIDs) and allow conditional creation. SQL fixtures easier to reason about for analytics backfills. Idempotency satisfies NFR-006 and supports local replays.
- **Alternatives considered**: Pure SQL seeds (less flexibility for hashed secrets and cascading inserts), CSV imports (additional tooling, harder to enforce idempotency).

### Data Classification & Retention
- **Decision**: Maintain `configs/data-classification.yml` mapping entities/fields to classifications (Restricted, Confidential, Internal) with retention windows and purge strategies enforced by automated lint checks.
- **Rationale**: Centralized config allows schema lint tool to enforce encryption/anonymization, satisfies FR-010/NFR-009, and integrates with security review process.
- **Alternatives considered**: Inline annotations inside SQL files (harder to lint programmatically), external spreadsheet (risk of drift from repo state, fails constitution GitOps expectations).


