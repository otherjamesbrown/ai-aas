# Theme 2: Configuration Management

**Review Date:** 2025-12-26
**Reviewer:** Claude (AI-assisted)
**Epic Bead:** aas-4g29
**Theme Bead:** aas-c2es

## Summary

The platform has functional configuration management with good Helm-based deployment configuration but significant inconsistencies in application-level patterns. Three different config loading patterns are used (manual helpers, envconfig, Viper) creating cognitive overhead for developers.

## Scoring

| Component | Score | Notes |
|-----------|-------|-------|
| admin-api-service | 3/5 | Manual helpers, no docs, good validation |
| api-router-service | 3.5/5 | envconfig, but hardcoded localhost DB default |
| analytics-service | 4/5 | envconfig, good validation |
| user-org-service | 3.5/5 | envconfig, good docs, weak secrets |
| ai-model-operator | 1.2/5 | No code-level config, Helm-only |
| ai-aas-cli | 2.2/5 | Viper (different pattern), plaintext secrets |
| internal (shared) | 1.5/5 | Manual helpers, appears unused |

**Average Score:** 2.7/5

## Criteria Checklist

- [ ] Single source of truth for config - **PARTIAL** (multiple sources with unclear precedence)
- [x] Environment-specific overrides documented - **GOOD** (Helm values-*.yaml)
- [ ] No hardcoded values - **FAIL** (api-router has localhost DB default)
- [ ] Config validation at startup - **PARTIAL** (50% of services)
- [ ] Consistent config loading pattern - **FAIL** (3 different patterns)
- [x] Secrets separated from config - **GOOD** (except CLI)

## Critical Issues

### 1. Three Incompatible Config Patterns

| Service | Pattern | Library |
|---------|---------|---------|
| admin-api | Manual helpers | None |
| api-router, analytics, user-org | envconfig tags | envconfig |
| cli | Viper + env override | Viper |
| operator | Helm only | None |

### 2. Hardcoded Localhost Defaults

**api-router-service** `config.go:26`:
```go
DatabaseURL string `envconfig:"DATABASE_URL" default:"postgres://postgres:postgres@localhost:5432/ai_aas_operational"`
```
This is dangerous - should be `required:"true"` instead.

### 3. CLI Stores Secrets in Plaintext

API keys and HuggingFace tokens stored in `~/.ai-aas-cli.yaml` - security risk on shared systems.

## Remediation Items

| Priority | Issue | Affected Components | Effort | Bead |
|----------|-------|---------------------|--------|------|
| P1 | Standardize on envconfig across all services | admin-api, cli, operator | High | TBD |
| P1 | Remove hardcoded localhost DB default | api-router | Low | TBD |
| P1 | Move CLI secrets to OS credential manager | cli | Medium | TBD |
| P2 | Add Validate() method to all services | api-router, user-org | Low | TBD |
| P2 | Create operator config struct | operator | Medium | TBD |

## Files Examined

- `services/admin-api-service/internal/config/config.go:83-115` (manual helpers)
- `services/api-router-service/internal/config/config.go:26` (hardcoded default)
- `services/ai-aas-cli/internal/config/config.go:25-27` (plaintext secrets)
- `operators/ai-model-operator/deployments/helm/` (no code config)
