# Implementation Plan: Model Management

**Branch**: `020-model-management` | **Date**: 2025-11-28 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/020-model-management/spec.md`

## Summary

Implement `ai-aas-cli`, a comprehensive CLI tool for model lifecycle management. The CLI enables platform admins to manage models from HuggingFace through object storage caching to Kubernetes deployment, with full validation, audit trails, and library management (enable/disable) capabilities.

**Core capabilities:**
- CLI initialization with PATH detection and credential setup
- Model registry with HuggingFace integration and license handling
- Object storage caching for fast deployments (~5 min vs 30+ min cold starts)
- KServe-based deployment with validation and health checks
- Enable/disable library management for capacity optimization
- Aliases for version abstraction and A/B testing

## Technical Context

**Language/Version**: Go 1.21+ (aligns with existing admin-cli codebase)
**Primary Dependencies**:
- `cobra` - CLI framework (existing pattern)
- `huggingface_hub` Go client or REST API wrapper
- `aws-sdk-go-v2` - S3-compatible object storage
- `client-go` - Kubernetes API interactions
- `pgx/v5` - PostgreSQL driver
- `viper` - Configuration management

**Storage**: PostgreSQL (existing platform DB) + S3-compatible object storage (Linode)
**Testing**: Go standard testing + testify + mockery for mocks
**Target Platform**: Linux/macOS/Windows CLI binary (cross-compiled)
**Project Type**: CLI tool extending existing services architecture
**Performance Goals**:
- `model list` < 1 second for 100 models
- `model pull` progress updates every 5 seconds
- `model validate` < 30 seconds per model
- Multipart upload for files > 100MB

**Constraints**:
- Must integrate with existing admin-api-service (017)
- HF tokens encrypted at rest (AES-256)
- All destructive operations require `--confirm` or `--force`
- Resume support for interrupted downloads

**Scale/Scope**:
- 10-50 models in registry
- 5-10 concurrent deployments per environment
- 3 environments (development, staging, production)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Status | Notes |
|------|--------|-------|
| API-First | ✅ PASS | CLI calls Admin API; OpenAPI contract in `contracts/admin-api.yaml`; CLI is thin client with no business logic |
| Statelessness | ✅ PASS | CLI stores only local config; all persistent state in PostgreSQL via admin-api-service; cache in Redis |
| Async Non-Critical | ✅ PASS | CLI commands are user-initiated synchronous operations; audit logging via admin-api is async |
| Security | ✅ PASS | Credentials encrypted (AES-256) server-side; audit log for all operations; no secrets in Git; HF tokens never logged |
| GitOps/Declarative | ✅ PASS | Deployments use InferenceService manifests applied via KServe; production deployments via ArgoCD |
| Observability | ✅ PASS | CLI uses `shared/go/logging`; admin-api has health/ready/metrics endpoints; audit trail in DB |
| Testing | ✅ PASS | Unit tests (≥80% coverage target), integration tests with Testcontainers, E2E lifecycle test |
| Performance | ✅ PASS | SLOs defined: `model list` <1s, `model validate` <30s, multipart upload >100MB |

**No violations requiring justification.**

## Project Structure

### Documentation (this feature)

```text
specs/020-model-management/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output
│   └── admin-api.yaml   # OpenAPI additions for model management
└── tasks.md             # Phase 2 output
```

### Source Code (repository root)

```text
services/ai-aas-cli/
├── cmd/
│   ├── root.go              # Root command, --init, --version, --help
│   ├── config.go            # config show/set/test/check-path
│   ├── credentials.go       # credentials set/list/test/delete
│   └── model/
│       ├── add.go           # model add
│       ├── list.go          # model list
│       ├── info.go          # model info
│       ├── pull.go          # model pull
│       ├── cache.go         # model cache list/verify/delete/gc
│       ├── deploy.go        # model deploy
│       ├── undeploy.go      # model undeploy
│       ├── enable.go        # model enable/disable
│       ├── library.go       # model library
│       ├── swap.go          # model swap
│       ├── validate.go      # model validate
│       ├── update.go        # model update/check-updates/pin/unpin
│       ├── alias.go         # model alias create/list/update/delete
│       ├── logs.go          # model logs/events/describe
│       ├── test.go          # model test
│       └── history.go       # model history
├── internal/
│   ├── config/
│   │   ├── config.go        # Configuration management
│   │   ├── path.go          # PATH detection logic
│   │   └── init.go          # Init wizard
│   ├── api/
│   │   └── client.go        # Admin API client
│   ├── huggingface/
│   │   ├── client.go        # HF Hub API client
│   │   ├── download.go      # Model download with resume
│   │   └── license.go       # License/gating detection
│   ├── storage/
│   │   ├── s3.go            # S3 operations (multipart upload)
│   │   └── manifest.go      # Manifest generation/verification
│   ├── kubernetes/
│   │   ├── client.go        # K8s client wrapper
│   │   ├── inference.go     # InferenceService operations
│   │   └── wait.go          # Wait for ready helpers
│   ├── registry/
│   │   └── client.go        # Registry DB operations via API
│   ├── validation/
│   │   ├── validator.go     # Validation framework
│   │   ├── registry.go      # Registry checks
│   │   ├── cache.go         # Cache checks
│   │   ├── deployment.go    # Deployment checks
│   │   ├── endpoint.go      # Endpoint checks
│   │   └── router.go        # Router checks
│   └── output/
│       ├── table.go         # Table formatter
│       ├── json.go          # JSON formatter
│       └── progress.go      # Progress bar
├── main.go
├── go.mod
├── go.sum
└── Makefile

services/ai-aas-cli/tests/
├── unit/
│   ├── config_test.go
│   ├── huggingface_test.go
│   └── validation_test.go
├── integration/
│   ├── init_test.go
│   ├── model_workflow_test.go
│   └── credentials_test.go
└── e2e/
    └── full_lifecycle_test.go

# Database migrations (extends existing)
db/migrations/
├── 20251128_001_create_model_registry.sql
├── 20251128_002_create_model_cache.sql
├── 20251128_003_create_model_deployments.sql
├── 20251128_004_create_model_aliases.sql
├── 20251128_005_create_model_state_history.sql
├── 20251128_006_create_model_validations.sql
└── 20251128_007_create_platform_credentials.sql

# Admin API extensions
services/admin-api-service/
├── internal/handlers/
│   └── models/              # New model management handlers
└── internal/services/
    └── models/              # New model management service
```

**Structure Decision**: Follows existing services architecture pattern. CLI is a standalone service in `services/ai-aas-cli/` with clear separation between commands (`cmd/`), business logic (`internal/`), and tests. Database migrations extend the existing migration set. Admin API gets new endpoints for model operations.

## Implementation Phases

### Phase 1: Foundation (P1 Stories)
- US-000: CLI Initialization
- US-001: Model Registry & Discovery
- US-002: Credentials Management
- US-003: Model Caching

**Delivers**: Working CLI that can init, register models, store credentials, and cache models to object storage.

### Phase 2: Deployment & Operations (P2 Stories)
- US-004: Model Deployment
- US-005: Model Validation & Audit
- US-008: Model Enable/Disable (Library Management)

**Delivers**: Full deployment lifecycle with validation and library management.

### Phase 3: Advanced Features (P3 Stories)
- US-006: Model Updates
- US-007: Troubleshooting
- US-009: Model Aliases

**Delivers**: Update workflows, debugging tools, and operational convenience features.

## Complexity Tracking

> No constitution violations. Standard architecture following existing patterns.

| Decision | Rationale |
|----------|-----------|
| Separate CLI service | Follows existing pattern (admin-cli → ai-aas-cli rename) |
| Via Admin API | CLI doesn't access DB directly; goes through admin-api-service |
| Local config file | Standard CLI pattern; credentials stored server-side via API |
