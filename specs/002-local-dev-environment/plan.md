# Implementation Plan: Local Development Environment

**Branch**: `002-local-dev-environment` | **Date**: 2025-11-10 | **Spec**: `/specs/002-local-dev-environment/spec.md`
**Input**: Feature specification from `/specs/002-local-dev-environment/spec.md`

**Note**: Generated via `/speckit.plan`.

## Summary

Deliver a reproducible developer environment that defaults to secure, remote Linode workspaces while preserving local parity. Terraform + StackScript provision an ephemeral VM, bootstrap Docker Compose dependencies (PostgreSQL, Redis, NATS, MinIO, mock inference), and install observability agents. Make-based lifecycle commands remain identical across remote and local workflows, backed by Go utilities for secrets synchronization (GitHub repository environment secrets via `gh api`) and health status reporting. Documentation and automation include data-classification/retention guardrails plus telemetry scripts that measure remote/local startup performance so requirements stay verifiable end to end.

## Technical Context

**Language/Version**: Terraform 1.7, Go 1.21 (CLI utilities), Bash 5.x, Docker Compose v2, GNU Make 4.x  
**Primary Dependencies**: Linode Terraform provider, StackScript (cloud-init), GitHub CLI (`gh`) with fine-grained PAT for secret fetch, Vector + Loki for log shipping, PostgreSQL 15, Redis 7, NATS 2.x, MinIO, FastAPI mock inference container  
**Storage**: Ephemeral Linode block storage volumes for remote workspace; local Docker volumes; MinIO bucket for artifact parity  
**Testing**: Go unit tests for `cmd/dev-status` and `cmd/secrets-sync`; bash-based smoke tests (`make status --json`, `make remote-status --json`); Terraform plan validation via `terraform validate` and integration smoke via CI  
**Target Platform**: Remote: Linode VM with private VLAN + bastion tunnel; Local: macOS/Linux developer machines with Docker/WSL2  
**Project Type**: Developer tooling + infrastructure automation (no runtime services)  
**Performance Goals**: Remote stack healthy within 5 minutes; local stack healthy within 5 minutes; status command returns in <10 seconds; lifecycle commands complete (reset/stop) within 3 minutes  
**Constraints**: Enforce SSO+MFA, 24-hour TTL, audit logging; secrets never persist in VCS; parity between remote/local command outputs; logs/metrics land in observability pipeline  
**Scale/Scope**: Support up to 30 concurrent workspaces, 10–20 services connecting via shared dependencies, growth path for additional dependencies (additive containers)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

- Constitution document is currently placeholder (awaiting governance ratification). No enforceable gates provided; flagging as **NEEDS CLARIFICATION** for platform governance team.  
- Interim compliance posture: plan honors API-first (CLI/JSON contracts), security (Vault, MFA tunnel), observability (Vector/Loki, JSON status), and testing (Go unit + smoke tests). No waivers requested.

## Project Structure

### Documentation (this feature)

```text
specs/002-local-dev-environment/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── dev-environment-contracts.md
├── checklists/
│   └── requirements.md
└── spec.md
```

### Source Code (repository root)

```text
infra/terraform/
└── modules/
    └── dev-workspace/            # Terraform module for Linode workspace + StackScript

scripts/
└── dev/
    ├── remote_provision.sh            # wraps terraform apply/destroy
    ├── remote_lifecycle.sh            # ssh/systemd helpers (up/status/reset/logs)
    ├── status_transform.py            # optional jq helpers for CLI output (if needed)
    ├── measure_remote_startup.sh      # CI helper verifying remote startup + status latency SLAs
    └── measure_local_startup.sh       # CI helper verifying local startup + status latency SLAs

cmd/
├── dev-status/                   # Go module emitting component health JSON
└── secrets-sync/                 # Go module hydrating .env.* from GitHub secrets (gh api)

configs/
└── log-redaction.yaml            # redact patterns referenced by CLI and Vector

.dev/
└── compose/                      # docker compose specs (remote/local parity)
    ├── compose.base.yaml
    ├── compose.remote.yaml
    └── compose.local.yaml

.specify/local/
└── ports.yaml                    # developer overrides for host ports

.github/workflows/
└── dev-environment-ci.yml        # CI smoke tests for plan/commands (new)

docs/
└── platform/
    └── data-classification.md    # Data classification and retention guidelines for dev environments
```

**Structure Decision**: Extend existing monorepo tooling with dedicated `cmd/` binaries and `scripts/dev/` helpers to preserve Make-based UX. Terraform module lives alongside infrastructure code for reuse. Docker Compose definitions reside under `.dev/compose/` to keep mode-specific overrides organized while sharing core service definitions. CI workflow validates commands without deploying real workspaces by using `terraform plan` and mocked Vault responses. New measurement scripts report remote/local startup timing directly into the CI workflow, and data-classification guidance under `docs/platform/data-classification.md` informs both provisioning defaults and security reviews.