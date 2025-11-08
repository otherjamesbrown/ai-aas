# Research: Project Setup & Repository Structure

**Branch**: `000-project-setup`  
**Date**: 2025-11-08  
**Spec**: `/specs/000-project-setup/spec.md`

## Research Questions & Answers

| Topic | Decision | Rationale | Alternatives Considered |
|-------|----------|-----------|--------------------------|
| CI/CD platform | Use GitHub Actions with reusable workflow templates (`ci.yml`, `ci-remote.yml`) | Native integration with repo hosting, supports push + manual dispatch, secrets management, matrix builds | CircleCI (additional cost, less integrated), self-hosted runners (ops overhead) |
| Remote execution trigger | Provide `make ci-remote` that calls `scripts/ci/trigger-remote.sh` (GitHub CLI) | Contributors on restricted machines can run full pipeline remotely in <10 min | Asking developers to run `gh workflow run` manually (less discoverable) |
| Local CI parity | Use `act` wrapper (`scripts/ci/run-local.sh`) with containerized GHA | Mirrored environment ensures local failures match remote | Custom Docker Compose pipeline (higher maintenance) |
| Linting & formatting | `golangci-lint` plus `gofmt`/`goimports`, configs stored under `configs/` | Industry-standard tooling, fast, configurable | Running individual linters manually (less consistent), Build tags |
| Security scanning | `gosec` integrated into `make check` and workflows | Catches common Go security issues before PR | Manual review only (insufficient), alternate scanners (Bandit not Go-focused) |
| Go workspace | `go.work` spanning all services | Simplifies multi-module development, aligns with FR-010 | GOPATH mode (deprecated), single module (doesn't scale to 10–20 services) |
| Metrics storage | S3-compatible bucket (`ai-aas-build-metrics`) via `scripts/metrics/upload.sh` | Durable, queryable JSON artifacts, works with AWS & MinIO | GitHub Artifacts (not query friendly), relational DB (overkill) |
| Metrics schema | JSON files per run with fields: service, command, start/end, duration, status, commits, environment | Enables trend analysis, satisfies NFR-016–NFR-018 | Plain text logs (poor structure) |
| Template distribution | `templates/service.mk` included by per-service Makefiles | Single source of truth for targets, easy extension | Copy-paste per service (drifts), heavy task runners (mage, just) |
| Environment detection | Setup scripts detect macOS/Linux/WSL, install prerequisites or exit with instructions | Keeps onboarding <10 minutes, consistent error handling | Manual READMEs for each OS (more toil) |

## Outstanding Follow-ups

- Confirm whether organization already has an S3 bucket or if MinIO should be provisioned locally (handoff to infrastructure spec `001`).
- Governance team must finalize constitution principles and gates; current document is placeholder.


