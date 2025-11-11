# Shared Libraries Sample Service

This directory contains reference implementations for Go and TypeScript services that will adopt the shared libraries feature.

- `go/`: Go-based sample service (Chi router) with `/healthz` endpoint.
- `ts/`: TypeScript/Node.js sample service (Fastify) with `/healthz` endpoint.
- `otel/`: Local OpenTelemetry Collector stack for telemetry experiments.
- `scripts/`: Utility scripts (smoke tests, upgrade checks) populated in later phases.

Use `docker compose -f otel/docker-compose.yml up` to spin up the collector.

