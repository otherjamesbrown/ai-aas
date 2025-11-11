# Implementation Plan: API Router Service

**Branch**: `006-api-router-service` | **Date**: 2025-11-11 | **Spec**: `/specs/006-api-router-service/spec.md`  
**Input**: Feature specification from `/specs/006-api-router-service/spec.md`

**Note**: Generated via `/speckit.plan` and enriched with research in `/specs/006-api-router-service/research.md`.

## Summary

Deliver a resilient inference ingress that authenticates API requests, enforces organization spend controls, routes traffic intelligently across model backends, and emits near-real-time usage telemetry. The router will reuse shared authentication, configuration, observability, and limiter libraries, expose admin controls for routing overrides, and integrate with the existing usage analytics pipeline to keep finance dashboards current.

## Technical Context

**Language/Version**: Go 1.21, GNU Make 4.x, Protocol Buffers v3.23  
**Primary Dependencies**: Shared service framework (`004-shared-libraries`), `go-redis/v9` (rate limiter), `grpc-go`, `chi` HTTP router, OpenTelemetry SDK/collector, `sarama` Kafka client, `zap` logging  
**Storage**: Redis Cluster (Elasticache) for limiter state, Kafka topic `usage.records.v1` for telemetry egress, Config Service watch cache persisted to local disk (BoltDB), no direct relational storage  
**Testing**: `go test ./...`, contract tests via `buf` + golden fixtures, integration tests with docker-compose harness for Redis/Kafka, load tests using `vegeta`  
**Target Platform**: Kubernetes (staging + production clusters), GitHub Actions CI, developer laptops (macOS/Linux, WSL2)  
**Project Type**: Backend service exposing REST + gRPC endpoints with shared admin CLI support  
**Performance Goals**: ≤150 ms router overhead, ≤400 ms p95 latency, 1k RPS sustained with linear horizontal scale, telemetry export lag ≤60 s  
**Constraints**: Enforce budget/quota atomically, preserve idempotency across retries, Zero Trust mutual TLS for admin APIs, configuration propagation ≤30 s, failover to secondary backends ≤30 s  
**Scale/Scope**: Supports 200+ organizations, 10 concurrent model backends, burst handling up to 2k RPS, 24-hour buffered telemetry retention

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Constitution v1.4.0 gates satisfied:
  - **API-First**: External inference, status, and admin surfaces documented in OpenAPI; internal gRPC contracts versioned.  
  - **Security**: API key + HMAC validation, mutual TLS admin channel, encrypted secrets, and audited config updates meet Zero Trust guardrails.  
  - **Observability**: RED metrics, traces, structured logs, and Kafka usage exports deliver mandatory signals; dashboards/runbooks scheduled in deliverables.  
  - **Reliability/Resilience**: Automated failover, backpressure, limiter safeguards, and blue/green deployment path uphold resilience gate.  
  - **Testing**: Unit, contract, integration, and chaos drills planned; idempotency + limiter correctness covered by load tests.  
  - **Performance**: Latency and throughput targets align with success criteria; benchmarking and regression alerts defined.  
No waivers required; any future change raising new components must revisit gates.

## Project Structure

### Documentation (this feature)

```text
specs/006-api-router-service/
├── spec.md
├── plan.md               # this file
├── research.md           # Phase 0 output
├── data-model.md         # Phase 1 output
├── quickstart.md         # Phase 1 output
├── contracts/            # OpenAPI + event contracts
│   ├── api-router.openapi.yaml
│   └── usage-record.schema.yaml
└── tasks.md              # curated + speckit.tasks overlay
```

### Source Code (repository root)

```text
services/api-router-service/
├── cmd/router/                 # main binary wiring HTTP+gRPC servers
├── internal/
│   ├── auth/                   # API key + HMAC validation adapters
│   ├── limiter/                # Redis-backed rate/budget enforcement
│   ├── routing/                # policy evaluation, backend selection, failover
│   ├── usage/                  # accounting pipeline, Kafka producer
│   ├── admin/                  # config overrides, diagnostics handlers
│   └── telemetry/              # RED metrics, tracing, logging glue
├── pkg/contracts/              # generated protobuf + OpenAPI artifacts
├── configs/
│   ├── router.sample.yaml      # bootstrap configuration
│   └── policies.sample.yaml    # routing policy examples
├── deployments/
│   └── helm/api-router-service/  # chart + values for staging/prod
├── scripts/
│   ├── smoke.sh                # post-deploy smoke tests
│   └── loadtest.sh             # vegeta harness for latency/limiter checks
└── test/
    ├── integration/            # docker-compose harness (Redis, Kafka, mock backends)
    ├── contract/               # OpenAPI + protobuf golden tests
    └── chaos/                  # failover + limiter race drills
```

**Structure Decision**: Service lives under `services/api-router-service` to align with existing monorepo pattern; shared libraries remain in `libs/`. Generated contracts stored in `pkg/contracts` to keep clients in sync. Deployment assets co-locate with service for GitOps. Tests follow `internal` component boundaries to keep ownership clear and accelerate targeted runs.

## Complexity Tracking

No constitution violations introduced. Planned components (Redis limiter, Kafka exporter, Config Service integration) reuse existing platform capabilities and avoid bespoke infrastructure.
