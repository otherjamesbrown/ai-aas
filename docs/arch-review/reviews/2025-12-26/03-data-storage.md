# Theme 3: Data Storage (etcd vs Postgres)

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-jtr2

## Summary

The platform shows mixed implementation patterns with clear separation in some services but critical violations in the CLI. User-org-service has best-in-class transaction handling while CLI bypasses the API layer with direct database access.

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| admin-api-service | 3.5/5 | Good pool config, missing transactions |
| api-router-service | 4/5 | Excellent etcd/BoltDB separation |
| analytics-service | 3.5/5 | TimescaleDB, missing batch optimization |
| user-org-service | 4.5/5 | Best-in-class withTx() pattern |
| ai-model-operator | 4/5 | Pure K8s-native (correct) |
| ai-aas-cli | 1.5/5 | **CRITICAL: Direct DB access** |

**Average Score:** 3.5/5

## Criteria Checklist

- [x] Clear separation: etcd for K8s state, Postgres for app data - **GOOD**
- [ ] Consistent data access patterns - **PARTIAL** (varies by service)
- [x] No mixed storage for same data type - **GOOD**
- [x] Proper migrations for schema changes - **GOOD** (goose)
- [ ] Connection pooling configured - **PARTIAL** (admin-api only)
- [ ] Transaction handling - **PARTIAL** (only user-org-service)

## Critical Issue: CLI Direct Database Access

**File:** `services/ai-aas-cli/internal/admin/deployment.go:102`
```go
db, err := sql.Open("postgres", cfg.DatabaseURL)  // ❌ Direct connection
defer db.Close()
```

**Impact:**
- Violates API-First principle
- No connection pooling
- Business logic duplication
- Database schema coupling in CLI

## Data Distribution

| Service | Postgres | etcd | Redis | Other |
|---------|----------|------|-------|-------|
| admin-api | Models, recipes, audit | - | - | - |
| api-router | - | Routing policies | Rate limits | BoltDB cache |
| analytics | Usage, reliability | - | Freshness cache | S3 exports |
| user-org | Users, orgs, API keys | - | Session cache | - |
| operator | - | K8s CRDs (implicit) | - | - |

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P0 | Remove direct DB access from CLI | cli | High | TBD |
| P1 | Add transaction handling to admin-api | admin-api | Medium | TBD |
| P2 | Make pool sizes configurable | analytics, user-org | Low | TBD |
| P2 | Optimize batch inserts in analytics | analytics | Medium | TBD |

## Files Examined

- `services/ai-aas-cli/internal/admin/deployment.go:102-109` (direct DB)
- `services/user-org-service/internal/storage/postgres/store.go:48-67` (withTx pattern)
- `services/admin-api-service/internal/repository/db.go:35-39` (pool config)
- `services/api-router-service/internal/config/loader.go:32-54` (etcd client)
