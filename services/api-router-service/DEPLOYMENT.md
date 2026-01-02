# api-router-service Deployment Specification

---
last_updated: 2026-01-01
maintained_by: go-services-developer
consumed_by: infra-ops-manager
---

## Overview

This document defines the deployment requirements for the api-router-service. The infra-ops-manager agent reads this file when deploying or updating the service.

**If you change this service in ways that affect deployment, you MUST update this file.**

## Health Endpoints

| Endpoint | Type | Expected Response | Notes |
|----------|------|-------------------|-------|
| `/v1/status/healthz` | Liveness | 200 OK | Basic process health |
| `/v1/status/readyz` | Readiness | 200 OK when ready | Dependencies checked |
| `/metrics` | Metrics | Prometheus format | Port 8080 |

**Note**: Readiness probe may need higher timeout (10s) and failure threshold (6) due to multiple backend health checks.

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | API traffic and metrics |

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `BACKEND_ENDPOINTS` | Comma-separated backend URLs | `http://vllm-1:8000,http://vllm-2:8000` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | HTTP listen port | `8080` |
| `SERVICE_NAME` | Service identifier | `api-router-service` |
| `ENVIRONMENT` | Environment name | `development` |
| `LOG_LEVEL` | Logging level | `info` |
| `MODEL_ACCESS_ENABLED` | Enable model access control | `false` |
| `REDIS_ADDRESS` | **Redis for rate limiting + API key cache invalidation** | `redis-service:6379` |
| `REDIS_PASSWORD` | Redis password | (empty) |
| `REDIS_DB` | Redis database number | `0` |
| `KAFKA_BROKERS` | Kafka broker addresses | `kafka-service:9092` |
| `KAFKA_TOPIC` | Usage records topic | `usage.records.v1` |
| `KAFKA_AUDIT_TOPIC` | Audit events topic | `audit.router` |
| `CONFIG_SERVICE_ENDPOINT` | etcd endpoint | `etcd-service:2379` |
| `CONFIG_WATCH_ENABLED` | Enable config watching | `true` |
| `RATE_LIMIT_DEFAULT_RPS` | Default rate limit | `100` |
| `RATE_LIMIT_BURST_SIZE` | Rate limit burst | `200` |
| `DEFAULT_BACKEND_TIMEOUT` | Backend request timeout | `90s` |
| `OTEL_ENABLED` | Enable OpenTelemetry | `false` |
| `OTEL_COLLECTOR_ENDPOINT` | OTEL collector URL | (empty) |

## Resource Requirements

| Resource | Request | Limit |
|----------|---------|-------|
| CPU | 100m | 500m |
| Memory | 128Mi | 512Mi |

## Dependencies

### Required

| Dependency | Purpose | Notes |
|------------|---------|-------|
| vLLM backends | Model inference | Configure via `BACKEND_ENDPOINTS` |

### Optional (Recommended)

| Dependency | Purpose | Notes |
|------------|---------|-------|
| Redis 7+ | Rate limiting, API key cache invalidation | **CRITICAL**: Required for immediate API key revocation. Without Redis, revoked keys remain valid for up to 1 minute (cache TTL). Also required for rate limiting. |
| Kafka | Usage tracking | Required for usage analytics |
| etcd | Dynamic config | Required for config hot-reload |

## Secrets

| Secret Name | Key | Purpose |
|-------------|-----|---------|
| `api-router-secrets` | `redis-password` | Redis authentication (if enabled) |
| `api-router-secrets` | `kafka-brokers` | Kafka connection (if using secrets) |

## Helm Chart Location

```
services/api-router-service/deployments/helm/api-router-service/
```

## Container Image

```
ghcr.io/otherjamesbrown/api-router-service:latest
```

## Replicas

- Development: 2
- Staging: 2
- Production: 3+ (with HPA)

## Autoscaling

HPA enabled:
- Min replicas: 2
- Max replicas: 10
- Target CPU: 70%
- Target Memory: 80%

## Ingress

Ingress enabled by default:
- Host: `api.{domain}`
- TLS: Required (cert-manager)
- Force SSL redirect: true

## Notes for infra-ops-manager

1. This is the main public-facing service - ingress is critical
2. Backend endpoints must be configured before service starts
3. Service gracefully handles missing optional dependencies (Redis, Kafka, etcd)
4. Pod disruption budget ensures at least 1 replica available during updates
5. Readiness probe timeout should be increased if many backends configured
6. Service is stateless - safe for rolling updates
