# Quickstart: Project Setup & Repository Structure

**Branch**: `000-project-setup`  
**Date**: 2025-11-08  
**Audience**: New contributors and platform engineers

---

## 1. Prerequisites

1. Confirm platform context
   - All infrastructure provisioning targets **Akamai Linode** services:
     - Kubernetes clusters via [Linode Kubernetes Engine (LKE) API](https://techdocs.akamai.com/linode-api/reference/post-lke-cluster)
     - Virtual machines via [Linode Instance API](https://techdocs.akamai.com/linode-api/reference/post-linode-instance)
     - Object storage buckets via [Linode Object Storage API](https://techdocs.akamai.com/linode-api/reference/post-object-storage-bucket)
   - Console/API access should use personal API tokens with required scopes.
2. Install required tooling (macOS/Linux/WSL2):
   ```bash
   ./scripts/setup/bootstrap.sh --check-only
   ```
   - Verifies Go ≥ 1.21, Git, Docker (for local runs), GitHub CLI, `make`
   - On failure, prints installation commands per OS and exits non-zero
3. Authenticate GitHub CLI (needed for `make ci-remote`):
   ```bash
   gh auth login
   ```
4. Configure access to the metrics bucket (optional local validation):
   ```bash
   aws configure sso  # or `mc alias set` for MinIO
   ```

## 2. One-time Setup

```bash
git clone git@github.com:otherjamesbrown/ai-aas.git
cd ai-aas
./scripts/setup/bootstrap.sh   # idempotent; installs tooling where possible
make help                      # discover available tasks
```

Expected outcome:
- Workspace initialized (`go.work` synced)
- `configs/` lint and security configs available
- Help output lists `build`, `test`, `lint`, `check`, `ci-remote`, etc.

Recent bootstrap run (macOS M3, 16 GB RAM, 2025-11-08): **8m 42s** end-to-end with all prerequisites installed. Progress indicators show each phase (`[1/5]` ... `[5/5]`) for transparency.

## 3. Daily Workflow

### Build & Test a Single Service
```bash
cd services/user-org-service
make build
make test
```

### Run Quality Checks Locally
```bash
make check            # from repo root (all services) or service directory
```
- Measure help responsiveness (target < 1s):
  ```bash
  tests/perf/measure_help.sh
  ```

### Generate Metrics Locally (optional)
```bash
make check METRICS=true          # emits JSON under scripts/metrics/output/
```

### Trigger Remote CI (restricted laptops)
```bash
make ci-remote \
  SERVICE=user-org-service \
  REF=feature/my-branch \
  NOTES="Run full pipeline from restricted device"
```
- Wrapper calls `scripts/ci/trigger-remote.sh`
- Outputs GitHub Actions run URL
- Waits for completion (<10 minutes SLA) and prints status summary
- Benchmark cross-service builds when needed:
  ```bash
  tests/perf/build_all_benchmark.sh
  ```

### Mirror Remote CI Locally with `act`
```bash
make ci-local
```
- Uses `scripts/ci/run-local.sh`
- Requires Docker when available; otherwise prints guidance

## 4. Metrics & Observability

- Metrics collector (`scripts/metrics/collector.go`) writes JSON artifacts with build/test duration, status, and metadata.
- Upload task (`scripts/metrics/upload.sh`) pushes results to `s3://ai-aas-build-metrics/YYYY/MM/DD/<run-id>.json`.
- GitHub Actions job `metrics-upload` runs on every `ci`/`ci-remote` completion.
- Scheduled workflow cleans files older than 30 days (configurable).

## 5. Adding a New Service

```bash
make service-new NAME=billing-service
```
- Copies `templates/service.mk` into `services/billing-service/Makefile`
- Adds module entry to `go.work`
- Generates boilerplate README and CI alerts to review automation

Run initial checks:
```bash
cd services/billing-service
make build
make check
make ci-remote SERVICE=billing-service REF=$(git rev-parse HEAD)
```
- Update `configs/tool-versions.mk` if the service introduces new tooling requirements.

## 6. Troubleshooting

| Scenario | Resolution |
|----------|------------|
| Missing Go or Docker | Rerun `./scripts/setup/bootstrap.sh`; follow printed instructions. |
| `make ci-remote` fails authentication | `gh auth status`; ensure required scopes (`workflow`, `repo`). |
| Metrics not uploading | Check `scripts/metrics/upload.sh` logs; verify `METRICS_BUCKET` secret present. |
| Local checks slow | Use `make check SERVICE=<name>` to scope run; verify caching under `~/.cache/golangci-lint`. |
| Workflow not listed in repo | Ensure `.github/workflows/ci-remote.yml` merged; rerun `make ci-remote --help`. |

---

**Next Steps**: After following this quickstart, contributors can review `/docs/contributing.md` for coding standards and run `/speckit.tasks` to view implementation work items tied to this feature.

