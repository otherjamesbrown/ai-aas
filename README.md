# AI-AAS Platform

[![CI](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml/badge.svg)](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci.yml)
[![Remote CI](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci-remote.yml/badge.svg)](https://github.com/otherjamesbrown/ai-aas/actions/workflows/ci-remote.yml)
[![Docs](https://img.shields.io/badge/docs-quickstart-blue)](./specs/000-project-setup/quickstart.md)

Spec-driven repository for an inference-as-a-service platform built on Akamai Linode infrastructure. This repo provides the automation scaffolding, documentation, and tooling required to onboard new contributors and operate Go-based services with consistent build, test, and CI workflows.

> 📚 Start with the [specification](./specs/000-project-setup/spec.md) and [implementation plan](./specs/000-project-setup/plan.md) for context.

---

## Using the Platform

**Are you looking to use the AIaaS platform?** Start with the **[Usage Guide](./usage-guide/README.md)** for role-based documentation:

- **[Getting Started](./usage-guide/getting-started.md)** - First-time setup and onboarding
- **[Architects](./usage-guide/architect/README.md)** - System architecture and component design
- **[Developers](./usage-guide/developer/README.md)** - API integration and development workflows
- **[System Administrators](./usage-guide/system-admin/README.md)** - Platform administration and infrastructure
- **[Operations](./usage-guide/operations/README.md)** - Day-to-day operations and monitoring
- **[Organization Administrators](./usage-guide/org-admin/README.md)** - Organization and user management
- **[Security](./usage-guide/security/README.md)** - Security architecture and incident response
- **[Finance](./usage-guide/finance/README.md)** - Budget management and billing
- **[Managers](./usage-guide/manager/README.md)** - Team oversight and reporting
- **[Analysts](./usage-guide/analyst/README.md)** - Usage analytics and insights
- **[FAQ](./usage-guide/FAQ.md)** - Frequently asked questions
- **[Troubleshooting](./usage-guide/troubleshooting.md)** - Common issues and solutions

---

## Quickstart

```bash
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas
./scripts/setup/bootstrap.sh
make help
```

### Prerequisites

The bootstrap script verifies required and optional tooling. See `specs/000-project-setup/quickstart.md` for detailed installation instructions.

**Required:**
- Go 1.24+
- Git
- GNU Make 4.x
- Docker (with Compose v2 plugin)
- GitHub CLI (`gh`)

**Optional (recommended for local/remote dev environment):**
- Terraform (for remote workspace provisioning)
- Vault (for secrets management)
- Linode CLI (for remote workspace operations)
- `act` (for local GitHub Actions testing)
- AWS CLI (for S3-compatible storage)
- MinIO Client (`mc`)

For local development environment setup, see `specs/002-local-dev-environment/quickstart.md`.

### Database Schemas & Migrations

- Review `specs/003-database-schemas/quickstart.md` before running migration tooling.
- Copy `configs/migrate.example.env` to `migrate.env` (or set environment variables in CI) and update connection strings, telemetry headers, and component selector.
- Migration scripts expect `db/` and `analytics/` directories provisioned via `/speckit.tasks` Phase 1; see `make db-migrate-status` for validation commands.

---

## Repository Layout

| Path | Description |
|------|-------------|
| `Makefile` | Root automation entrypoint (build, test, check, ci-local, ci-remote). |
| `configs/` | Shared lint/security/tooling configuration, including `tool-versions.mk`. |
| `scripts/` | Setup, CI, metrics, and service generator scripts. |
| `services/` | Service implementations. `_template/` provides scaffolding for new services. |
| `usage-guide/` | **Platform usage documentation organized by persona** (architects, developers, admins, etc.). |
| `docs/` | Runbooks, platform guides, troubleshooting references for operations and development. |
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

### Branching Strategy

- `main` is the long-lived integration branch; all feature work must merge back into `main`.
- Create feature branches per spec or workstream using the numeric prefix convention (e.g., `002-local-dev-environment-plan`).
- Branch directly from `main`. If temporarily stacking work atop another in-flight branch, plan to rebase onto `main` before opening the final PR.
- Keep branches focused on a single spec or deliverable to simplify review and history.

---

## Resources

### Platform Usage
- **[Usage Guide](./usage-guide/README.md)** - Complete platform usage documentation by role
- **[Getting Started](./usage-guide/getting-started.md)** - First-time user onboarding
- **[FAQ](./usage-guide/FAQ.md)** - Frequently asked questions
- **[Troubleshooting](./usage-guide/troubleshooting.md)** - Common issues and solutions

### Development & Operations
- [Quickstart](./specs/000-project-setup/quickstart.md) - Developer setup and prerequisites
- [Local Dev Environment](./specs/002-local-dev-environment/quickstart.md) - Local and remote workspace setup
- [Troubleshooting Guides](./docs/troubleshooting/) - Development and operations troubleshooting
- [Linode Access Guide](./docs/platform/linode-access.md) - Infrastructure access
- [Remote CI Runbook](./docs/runbooks/ci-remote.md) - CI/CD operations
- [llms.txt index](./llms.txt) - LLM context index

