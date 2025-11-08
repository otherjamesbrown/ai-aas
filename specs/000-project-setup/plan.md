# Implementation Plan: Project Setup & Repository Structure

**Branch**: `000-project-setup` | **Date**: 2025-11-08 | **Spec**: `/specs/000-project-setup/spec.md`
**Input**: Feature specification from `/specs/000-project-setup/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Establish a standardized monorepo scaffold that onboards contributors quickly, enforces consistent build/test quality checks across 10–20 Go services, and supports both local and remote CI execution. The solution provides a root Makefile, Go workspace (`go.work`), reusable GitHub Actions workflows with push and manual `ci-remote` triggers, shared editor/tooling configs, and a metrics collector that emits build/test telemetry to S3-compatible storage.

## Technical Context

**Language/Version**: Go 1.21, GNU Make 4.x, GitHub Actions runner images  
**Primary Dependencies**: `golangci-lint`, `gofmt`/`goimports`, `gosec`, `act`, AWS CLI or MinIO client for S3-compatible metrics upload  
**Storage**: S3-compatible object storage bucket (`s3://ai-aas-build-metrics` or MinIO) for build/test metrics artifacts  
**Testing**: `go test ./...` executed via Make targets and GitHub Actions jobs; optional `act` locally  
**Target Platform**: Developer workstations (macOS, Linux, Windows via WSL2/Git Bash) and GitHub Actions hosted runners  
**Project Type**: Polyglot monorepo anchored around multiple Go services with shared automation layer  
**Performance Goals**: Setup < 10 minutes, `make check` < 3 minutes per service, remote CI round-trip < 10 minutes, `make build-all` < 30 minutes across 20 services  
**Constraints**: Remote `ci-remote` workflow must mirror local checks; automation must tolerate restricted laptops without Docker/Go; commands provide progress feedback; metrics capture must add ≤5s overhead  
**Scale/Scope**: 10–20 services today, scalable automation templates for additional services, multi-module Go workspace

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Constitution v1.4.0 is ratified. This plan satisfies each gate:
  - **API-First**: Automation delivered via CLI/Make; no new service APIs required.
  - **Stateless & GitOps**: Repository automation keeps infra declarative (Terraform/Helm/ArgoCD) without runtime state drift.
  - **Security**: Linting, `gosec`, GitHub Actions, and secret-handling patterns are specified.
  - **Observability**: Metrics collector + S3 storage, CI telemetry, and quickstart guidance cover required signals.
  - **Testing**: Make targets, CI workflows, and remote parity ensure checks across environments.
  - **Performance**: NFRs enforce timing goals (setup, check, build-all) with benchmarking tasks.
No waivers needed; downstream features must continue to uphold these gates.

## Project Structure

### Documentation (this feature)

```text
specs/000-project-setup/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
└── tasks.md            # generated later by /speckit.tasks
```

### Source Code (repository root)

```text
.
├── Makefile                     # root orchestration targets (build-all, test-all, check, ci-remote)
├── go.work                      # Go workspace definition spanning all services
├── .editorconfig                # shared editor settings
├── .github/
│   └── workflows/
│       ├── ci.yml               # push/PR pipeline
│       └── ci-remote.yml        # workflow_dispatch entry for `make ci-remote`
├── scripts/
│   ├── setup/
│   │   └── bootstrap.sh         # validates prerequisites, installs tooling
│   ├── ci/
│   │   ├── run-local.sh         # wraps `act` execution and local mirrors
│   │   └── trigger-remote.sh    # invoked by `make ci-remote`
│   └── metrics/
│       ├── collector.go         # emits JSON metrics
│       └── upload.sh            # pushes telemetry to S3-compatible storage
├── configs/
│   ├── golangci.yml             # lint configuration
│   └── gosec.toml               # security scanner config
└── services/
    ├── user-org-service/
    │   └── Makefile             # includes shared automation template
    ├── api-router-service/
    │   └── Makefile
    ├── analytics-service/
    │   └── Makefile
    └── ... (additional services consuming shared templates)
```

**Structure Decision**: Retain a single repository with shared automation assets under `scripts/`, `.github/workflows/`, and `configs/`. Each service owns a lightweight Makefile that includes `../templates/service.mk` to inherit standard targets. Root Makefile coordinates cross-service operations and remote dispatch. Metrics tooling lives under `scripts/metrics/` to keep observability concerns centralized.

## Complexity Tracking

No constitution violations introduced. Template-based automation and workflows keep complexity manageable; no additional justification required.
