# Research: Local Development Environment

**Branch**: `002-local-dev-environment`  
**Date**: 2025-11-10  
**Spec**: `/specs/002-local-dev-environment/spec.md`

## Research Questions & Answers

| Topic | Decision | Rationale | Alternatives Considered |
|-------|----------|-----------|--------------------------|
| Remote workspace provisioning | Use Terraform 1.7 modules that wrap Linode Instance + StackScript and private VLAN attachment | Matches infrastructure spec provider (Linode), keeps infra declarative, StackScript lets us bootstrap base image with Docker, Vector, and security agents in one pass | Pure Linode CLI automation (harder to version), manual VM provisioning (no reproducibility) |
| Remote bootstrap & lifecycle | Ship systemd units to orchestrate Docker Compose stack via `/opt/ai-aas/dev-stack/compose.yaml`; manage via `make remote-*` wrappers invoking `ssh` + `systemctl` | Systemd keeps services resilient to reboots, Compose keeps parity with local workflow, Make targets give unified UX across modes | Using Kubernetes-in-VM (KinD/k3s) (heavier footprint, slower start), running everything via ad-hoc shell scripts (fragile) |
| Local dependency orchestration | Use Docker Compose v2 with a curated `compose.local.yaml` plus `.specify/local/ports.yaml` overrides | Compose is already required for remote stack; reuse reduces cognitive load, `.env.local` works out-of-the-box | Minikube/k3d (adds kubernetes overhead), custom Go supervisors (large lift) |
| Core dependency stack | PostgreSQL 15, Redis 7, NATS 2.x, MinIO (S3-compatible), mock inference service (FastAPI "+uvicorn" image) | Aligns with infrastructure plans and typical platform services; all have official containers, fast startup, cover data/cache/messaging/inference parity | CockroachDB (heavier, slower), RabbitMQ (broader features than needed), AWS S3 (external dependency) |
| Secrets distribution | Use HashiCorp Vault with AppRole to generate short-lived tokens; `make secrets-sync` calls a Go helper (`cmd/secrets-sync`) that writes `.env.linode` & `.env.local` with masked values | Vault already adopted by infrastructure team, AppRole works for both remote and local flows, Go helper integrates with existing tooling patterns | 1Password CLI (does not integrate with automation/TTL), storing secrets in Linode metadata (security risk) |
| Observability path | Install Vector agent in remote workspace to ship systemd & container logs to central Loki stack; expose `make remote-status --json` that queries healthcheck endpoints and publishes metrics via `scripts/dev/status.go` | Vector lightweight, Loki already targeted by observability roadmap, JSON status output feeds metrics pipeline, leverages Go expertise | Filebeat/FluentBit (more config overhead), plain journalctl tailing (no aggregation), Python status script (introduces new runtime) |
| Health check implementation | Build Go binary (`cmd/dev-status`) that polls dependency endpoints (PostgreSQL, Redis, NATS, MinIO, mock inference) and returns structured component states | Go binary embeds nicely in existing Go workspace, compiles for multiple OS targets, easy to add tests | Bash script (harder to test, no JSON formatting), Python (adds dependency) |
| Secure tunnel | Continue using `linode ssh bastion --workspace <name>` with enforced MFA/session recording, plus fail-close TTL (24h) automation | Reuses security tooling from infrastructure spec, audited connections, minimal extra work | Direct SSH with static keys (policy violation), WireGuard mesh (longer lead time) |
| Workspace teardown safety | Cron-based TTL enforcement plus `make remote-destroy` calling Terraform destroy; logs pushed to observability sink with correlation IDs | Guarantees cleanup, auditable actions, integrates with governance requirements | Manual cleanup scripts (error prone), relying solely on developer discipline |
| Mock inference behavior | Provide simple FastAPI service loading canned responses from `/opt/ai-aas/mock-data/` and matching production schema | Fast to implement, replicates schema for happy-path flows, allows future extension for alternate responses | Building gRPC stub (overkill now), hard-coded JSON server (less extensible) |

## Outstanding Follow-ups

- Confirm Vault namespace and AppRole IDs with infrastructure team before wiring `make secrets-sync` script.
- Coordinate with observability spec (`011-observability`) to provision Loki endpoint and API token for Vector.
- Align with security on retention policy for remote workspace session recordings (currently assumed 90 days).
- Validate MinIO bucket naming and credentials align with infrastructure provisioning outputs.
