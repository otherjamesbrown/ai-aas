# Automation Checklist: Project Setup

Use this checklist when rolling automation patterns into downstream features.

## Make Targets

- [ ] Root `Makefile` includes target descriptions (`make help` lists new targets).
- [ ] New service targets depend on shared template hooks (no duplicated logic).
- [ ] `check` target integrates fmt, lint, security, and tests for new scope.
- [ ] Metrics collection is triggered with accurate status metadata.

## CI Workflows

- [ ] GitHub Actions workflow yaml stored under `.github/workflows/`.
- [ ] Reusable workflows are leveraged when building/test matrices expand.
- [ ] Remote (`ci-remote.yml`) stays aligned with standard `ci.yml`.
- [ ] Metrics upload step runs with `if: always()` to capture failures.

## Documentation

- [ ] Quickstart updated with any new commands or prerequisites.
- [ ] Runbooks/troubleshooting entries cover common failure modes.
- [ ] `llms.txt` links to new automation assets.
- [ ] Release notes template captures automation-impacting changes.

## Tooling & Scripts

- [ ] Scripts declare `set -euo pipefail` and print usage.
- [ ] Tool versions sourced from `configs/tool-versions.mk`.
- [ ] Service generator updates templates and `go.work`.
- [ ] Local CI wrapper (`scripts/ci/run-local.sh`) supports caching and architecture overrides.

