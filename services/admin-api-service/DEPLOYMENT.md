# admin-api-service Deployment Specification

---
last_updated: 2025-12-08
maintained_by: go-services-developer
consumed_by: infra-ops-manager
---

## Overview

This document defines the deployment requirements for the admin-api-service. The infra-ops-manager agent reads this file when deploying or updating the service.

**If you change this service in ways that affect deployment, you MUST update this file.**

## Health Endpoints

| Endpoint | Type | Expected Response | Notes |
|----------|------|-------------------|-------|
| `/healthz` | Liveness | 200 OK | Basic process health |
| `/readyz` | Readiness | 200 OK when ready | Database connection verified |
| `/metrics` | Metrics | Prometheus format | Port 8080 |

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8080 | HTTP | API traffic and metrics |

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=require` |
| `API_KEY_HASH_SALT` | Salt for hashing API keys | (secret) |
| `USER_ORG_SERVICE_URL` | URL of user-org-service | `http://user-org-service.namespace.svc.cluster.local:8081` |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `8080` |
| `ENVIRONMENT` | Environment name | `development` |
| `LOG_LEVEL` | Logging level | `info` |
| `DB_MAX_CONNS` | Max database connections | `50` |
| `DB_MIN_CONNS` | Min database connections | `10` |
| `RATE_LIMIT_PER_MIN` | Rate limit per minute | `100` |
| `METRICS_ENABLED` | Enable Prometheus metrics | `true` |
| `TRACING_ENABLED` | Enable OpenTelemetry tracing | `false` |

## Resource Requirements

| Resource | Request | Limit |
|----------|---------|-------|
| CPU | 100m | 500m |
| Memory | 128Mi | 512Mi |

## Dependencies

### Required

| Dependency | Purpose | Notes |
|------------|---------|-------|
| PostgreSQL 14+ | Primary database | Requires `DATABASE_URL` |
| user-org-service | User/org validation | Requires `USER_ORG_SERVICE_URL` |

### Optional

None.

## Secrets

| Secret Name | Key | Purpose |
|-------------|-----|---------|
| `admin-api-secrets` | `database-url` | PostgreSQL connection string |
| `admin-api-secrets` | `api-key-hash-salt` | API key hashing salt |

## Helm Chart Location

```
services/admin-api-service/deployments/helm/admin-api-service/
```

## Container Image

```
ghcr.io/otherjamesbrown/ai-aas/admin-api-service:latest
```

## Replicas

- Development: 2
- Staging: 2
- Production: 3+

## Autoscaling

Not currently enabled. Service is stateless and can be horizontally scaled.

## Ingress

Ingress is disabled by default (internal service). Enable if external access needed.

## Notes for infra-ops-manager

1. Database migrations run automatically at startup
2. Service requires database to be available before starting (readiness probe will fail otherwise)
3. Worker process is embedded - no separate deployment needed
4. Service is stateless - safe to roll with no drain time
