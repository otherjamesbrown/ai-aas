# Service Automation Checklist

- [ ] Service directory created under `services/<name>/`.
- [ ] `Makefile` includes `../../templates/service.mk`.
- [ ] `SERVICE_NAME` and version variables defined before the include.
- [ ] `README.md` explains build/test commands and dependencies.
- [ ] `go.mod` / `go.sum` updated and referenced in `go.work`.
- [ ] `make build` and `make check` succeed locally.
- [ ] Remote CI (`make ci-remote SERVICE=<name>`) completes successfully.
- [ ] Metrics JSON produced and uploaded for at least one CI run.

Document completion of this checklist in the corresponding service spec or PR description.

