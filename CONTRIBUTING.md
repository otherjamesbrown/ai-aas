# Contributing to AI-AAS

Welcome! This repository follows a spec-first workflow. Every feature is defined in `specs/` with associated plans, research, and tasks. Please read this guide before opening a pull request.

## 1. Onboarding Checklist

1. Review the [Project Setup Spec](./specs/000-project-setup/spec.md) and [Plan](./specs/000-project-setup/plan.md).
2. Run the bootstrap script:
   ```bash
   ./scripts/setup/bootstrap.sh --check-only
   ```
3. Authenticate the GitHub CLI (`gh auth login`) to enable `make ci-remote`.
4. Configure Linode API access as described in `docs/platform/linode-access.md`.
5. Skim the [Quickstart](./specs/000-project-setup/quickstart.md) for daily workflows.

Record onboarding notes or blockers under `docs/troubleshooting/` where appropriate.

## 2. Development Workflow

- Use feature branches named after the spec ID, e.g. `feature/000-project-setup`.
- Follow Test-Driven Development when implementing tasks from `tasks.md`.
- Keep documentation up to date: README, quickstart, runbooks, and llms.txt.
- Add or update automation tests in `tests/` when modifying automation behavior.

### Commands

- `make help` — discover available targets.
- `make check` — run fmt, lint, security, test checks before committing.
- `make ci-local` — run GitHub Actions locally via `act`.
- `make ci-remote` — trigger the remote pipeline when local resources are limited.

## 3. Pull Requests

1. Ensure `make check` and relevant tests pass locally.
2. Provide context that links to spec and tasks (e.g. `Fixes T041, T045`).
3. Include screenshots or logs for automation-heavy changes when useful.
4. Reference updates to metrics or runbooks when applicable.
5. Request a review from the platform engineering team.

Use the pull request template under `.github/PULL_REQUEST_TEMPLATE.md`.

## 4. Coding Standards

- Go code must pass `gofmt`, `golangci-lint`, and `gosec`.
- All production code must include robust comments:
  - Provide package/type/function doc comments that follow the language’s docstring conventions (`// Foo ...` for Go, `/** ... */` for TypeScript, etc.).
  - Add explanatory comments for non-obvious control flow, data transformations, and cross-service interactions—include the “why”, not just the “what”.
  - Link to relevant specs, tickets, or runbooks when implementing guardrails, feature flags, or operational workarounds.
  - Remove stale comments as behavior changes; reviewers should block merges when comment coverage or accuracy falls short.
- Scripts should use `set -euo pipefail` and include usage documentation.
- Documentation should prefer Markdown with task IDs where applicable.

## 5. Release Process

- Update `docs/release-notes/template.md` when introducing automation changes.
- Coordinate with infrastructure specs before altering metrics retention or CI infrastructure.
- Tag releases using semantic versioning (`vX.Y.Z`) via GitHub releases.

Thanks for contributing and keeping the automation stack healthy!

