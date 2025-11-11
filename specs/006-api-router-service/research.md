# Research: API Router Service

## Findings

- Decision: Implement the router in Go 1.21 using the shared service framework from `004-shared-libraries` (HTTP server, middleware, observability).  
  - Rationale: Keeps the service aligned with the platform’s Go-first stack, reuses vetted auth/observability components, and satisfies latency goals with minimal runtime overhead.  
  - Alternatives considered: Node.js (higher latency under load, additional runtime for ops), Rust (performance benefit but lacking shared library coverage and slower team ramp-up).

- Decision: Back per-organization and per-key rate limiting with a Redis Cluster (Elasticache) accessed via the shared limiter library.  
  - Rationale: Redis offers predictable sub-millisecond operations, built-in replication, and existing ops support; limiter library already abstracts Redis tokens.  
  - Alternatives considered: In-memory token buckets (no horizontal scale or HA), DynamoDB/Spanner counters (higher latency, cost, and implementation complexity).

- Decision: Consume routing policy configuration via the centralized Config Service (gRPC streaming + watch API) with local cache and validation hooks.  
  - Rationale: Config Service provides audited updates, rollback, and staged rollouts; streaming watch keeps routers in sync within 30 seconds.  
  - Alternatives considered: S3/object-store polling (slower convergence, no audit trail), direct etcd access (duplicate infrastructure ownership).

- Decision: Emit usage records to the `usage.records.v1` Kafka topic defined by `005-usage-analytics`, using the shared telemetry publisher with disk-backed buffering.  
  - Rationale: Kafka topic already feeds finance/analytics pipelines; shared publisher handles retries/backoff and ensures at-least-once delivery.  
  - Alternatives considered: Direct writes to PostgreSQL (risk of coupling to finance schema, harder scaling), SQS queue (insufficient throughput for 1k RPS with large payloads).

- Decision: Provide admin control surface via gRPC/REST endpoints protected by mutual TLS, exposing operations for routing overrides, limiter adjustments, and health diagnostics.  
  - Rationale: Keeps control channel consistent with other services, supports automation (CLI) and dashboards, and allows role-based authorization via shared auth middleware.  
  - Alternatives considered: Manual config edits (slow incident response), bespoke CLI over SSH (breaks Zero Trust posture).
