# analytics-service Deployment Specification

---
last_updated: 2025-12-08
maintained_by: go-services-developer
consumed_by: infra-ops-manager
---

## Overview

This document defines the deployment requirements for the analytics-service. The infra-ops-manager agent reads this file when deploying or updating the service.

**If you change this service in ways that affect deployment, you MUST update this file.**

## Health Endpoints

| Endpoint | Type | Expected Response | Notes |
|----------|------|-------------------|-------|
| `/analytics/v1/status/healthz` | Liveness | 200 OK | Basic process health |
| `/analytics/v1/status/readyz` | Readiness | 200 OK when ready | Database connection verified |
| `/metrics` | Metrics | Prometheus format | Port 8084 |

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8084 | HTTP | API traffic and metrics |

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/analytics?sslmode=require` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | HTTP listen port | `8084` |
| `SERVICE_NAME` | Service identifier | `analytics-service` |
| `ENVIRONMENT` | Environment name | `development` |
| `LOG_LEVEL` | Logging level | `info` |
| `REDIS_URL` | Redis for caching | (empty) |
| `RABBITMQ_URL` | RabbitMQ for events | (empty) |
| `RABBITMQ_STREAM` | Event stream name | `usage.events.v1` |
| `RABBITMQ_CONSUMER` | Consumer group name | `analytics-service` |
| `OTEL_ENABLED` | Enable OpenTelemetry | `false` |
| `OTEL_COLLECTOR_ENDPOINT` | OTEL collector URL | (empty) |
| `S3_ENABLED` | Enable S3 exports | `false` |
| `S3_ENDPOINT` | S3-compatible endpoint | (empty) |
| `S3_BUCKET` | Export bucket name | `analytics-exports` |

## Resource Requirements

| Resource | Request | Limit |
|----------|---------|-------|
| CPU | 100m | 500m |
| Memory | 128Mi | 512Mi |

## Dependencies

### Required

| Dependency | Purpose | Notes |
|------------|---------|-------|
| PostgreSQL 14+ | Analytics database | Separate from other services |

### Optional (Recommended)

| Dependency | Purpose | Notes |
|------------|---------|-------|
| Redis 7+ | Query caching | Improves report performance |
| RabbitMQ | Event ingestion | Required for real-time analytics |
| S3-compatible | Data exports | For large data exports |

## Secrets

| Secret Name | Key | Purpose |
|-------------|-----|---------|
| `analytics-secrets` | `database-url` | PostgreSQL connection string |
| `analytics-secrets` | `redis-url` | Redis connection (if enabled) |
| `analytics-secrets` | `rabbitmq-url` | RabbitMQ connection (if enabled) |
| `analytics-secrets` | `s3-access-key` | S3 access key (if enabled) |
| `analytics-secrets` | `s3-secret-key` | S3 secret key (if enabled) |

## Helm Chart Location

```
services/analytics-service/deployments/helm/analytics-service/
```

## Container Image

```
ghcr.io/otherjamesbrown/analytics-service:latest
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

Ingress enabled:
- Path: `/analytics`
- Host: `api.{domain}` (shared with api-router)
- TLS: Required (cert-manager)

## Notes for infra-ops-manager

1. Analytics database should be separate from main application database
2. Service can start without RabbitMQ but won't receive real-time events
3. Database migrations run automatically at startup
4. Heavy aggregation queries may require resource increases in production
5. Consider read replicas for database if query load is high
6. Service is stateless - safe for rolling updates
