# AI-AAS Platform

[![CI](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml/badge.svg)](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml)
[![Remote CI](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci-remote.yml/badge.svg)](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci-remote.yml)
[![Docs](https://img.shields.io/badge/docs-quickstart-blue)](./specs/000-project-setup/quickstart.md)

Spec-driven repository for an inference-as-a-service platform built on Akamai Linode infrastructure. This repo provides the automation scaffolding, documentation, and tooling required to onboard new contributors and operate Go-based services with consistent build, test, and CI workflows.

> 📚 Start with the [specification](./specs/000-project-setup/spec.md) and [implementation plan](./specs/000-project-setup/plan.md) for context.

---

## Quickstart

```bash
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas
./scripts/setup/bootstrap.sh
make help
```

Consult `specs/000-project-setup/quickstart.md` for prerequisite tooling, remote CI instructions, and troubleshooting guidance.

---

## Repository Layout

| Path | Description |
|------|-------------|
| `Makefile` | Root automation entrypoint (build, test, check, ci-local, ci-remote). |
| `configs/` | Shared lint/security/tooling configuration, including `tool-versions.mk`. |
| `scripts/` | Setup, CI, metrics, and service generator scripts. |
| `services/` | Service implementations. `_template/` provides scaffolding for new services. |
| `docs/` | Runbooks, platform guides, troubleshooting references. |
| `specs/` | Feature specifications, plans, research, and derived task lists. |
| `tests/` | Performance and check fixtures executed via Make targets. |

---

## Key Commands

- `make help` — Discover available automation targets with descriptions.
- `make build SERVICE=<name>` — Build a specific service (`SERVICE=all` for every service).
- `make test SERVICE=<name>` — Run Go tests.
- `make check` — Run fmt, lint, security, and test checks (supports `METRICS=true`).
- `make ci-local` — Execute GitHub Actions workflow locally via `act`.
- `make ci-remote SERVICE=<name> REF=<branch>` — Trigger the remote pipeline from restricted machines.
- `make service-new NAME=<service>` — Generate a new service skeleton from `_template`.

---

## Metrics & Observability

- Metrics collector (`scripts/metrics/collector.go`) emits JSON artifacts consumable by dashboards.
- Upload helper (`scripts/metrics/upload.sh`) sends artifacts to S3-compatible storage.
- See `docs/metrics/policy.md` for retention and lifecycle rules.

---

## Contributing

- Run `./scripts/setup/bootstrap.sh --check-only` before submitting changes.
- Add new services with `make service-new NAME=<service>`, then update Go modules via `go work use`.
- Reference `CONTRIBUTING.md` for coding standards and code review expectations.
- Open issues using repository templates under `.github/ISSUE_TEMPLATE/`.

---

## Resources

- [Quickstart](./specs/000-project-setup/quickstart.md)
- [Troubleshooting Guides](./docs/troubleshooting/)
- [Linode Access Guide](./docs/platform/linode-access.md)
- [Remote CI Runbook](./docs/runbooks/ci-remote.md)
- [llms.txt index](./llms.txt)

