# user-org-service Deployment Specification

---
last_updated: 2026-01-02
maintained_by: go-services-developer
consumed_by: infra-ops-manager
---

## Overview

This document defines the deployment requirements for the user-org-service. The infra-ops-manager agent reads this file when deploying or updating the service.

**If you change this service in ways that affect deployment, you MUST update this file.**

## Health Endpoints

| Endpoint | Type | Expected Response | Notes |
|----------|------|-------------------|-------|
| `/health` | Liveness | 200 OK | Basic process health |
| `/ready` | Readiness | 200 OK when ready | Database connection verified |
| `/metrics` | Metrics | Prometheus format | Port 8081 |

## Ports

| Port | Protocol | Purpose |
|------|----------|---------|
| 8081 | HTTP | API traffic and metrics |

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/userorg?sslmode=require` |
| `JWT_SECRET` | Secret for JWT signing | (secret, min 32 chars) |
| `API_KEY_HASH_SALT` | Salt for API key hashing | (secret) |

### Optional

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_PORT` | HTTP listen port | `8081` |
| `ENVIRONMENT` | Environment name | `development` |
| `LOG_LEVEL` | Logging level | `info` |
| `DB_MAX_CONNS` | Max database connections | `50` |
| `DB_MIN_CONNS` | Min database connections | `10` |
| `JWT_EXPIRY` | JWT token expiry | `24h` |
| `REFRESH_TOKEN_EXPIRY` | Refresh token expiry | `168h` (7 days) |

## Resource Requirements

| Resource | Request | Limit |
|----------|---------|-------|
| CPU | 100m | 500m |
| Memory | 128Mi | 512Mi |

## Dependencies

### Required

| Dependency | Purpose | Notes |
|------------|---------|-------|
| PostgreSQL 14+ | User/org database | Dedicated database recommended |

### Optional

None.

## Secrets

| Secret Name | Key | Purpose |
|-------------|-----|---------|
| `user-org-secrets` | `database-url` | PostgreSQL connection string |
| `user-org-secrets` | `jwt-secret` | JWT signing secret |
| `user-org-secrets` | `api-key-hash-salt` | API key hashing salt |

## Helm Chart Location

```
services/user-org-service/configs/helm/
```

**Note**: This service uses a non-standard location (`configs/helm/` instead of `deployments/helm/`). The Helm chart includes templates for deployment, service, and ingress.

## Container Image

```
ghcr.io/otherjamesbrown/ai-aas/user-org-service:latest
```

## Replicas

- Development: 2
- Staging: 2
- Production: 3+

## Autoscaling

Not currently enabled. Service is stateless and can be horizontally scaled.

## Ingress

Ingress configuration:
- Host: `user-org.{domain}` or internal only
- TLS: Required if external
- Consider keeping internal-only for security

## Notes for infra-ops-manager

1. **Critical service** - other services depend on this for authentication
2. Database migrations run automatically at startup
3. JWT_SECRET must be consistent across all replicas
4. API_KEY_HASH_SALT must match admin-api-service for key validation
5. Consider dedicated database for isolation
6. Service is stateless - safe for rolling updates
7. Downtime affects all authenticated services
