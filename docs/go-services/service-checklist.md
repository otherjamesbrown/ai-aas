# Service Automation Checklist

---
last_updated: 2025-12-08
document_type: reference
---

## Overview

Use this checklist when creating a new Go service for the AI-AAS platform.

## Pre-Development

- [ ] Service directory created under `services/<name>/`
- [ ] Service name follows convention (kebab-case, descriptive)

## Build Configuration

- [ ] `Makefile` includes `../../templates/service.mk`
- [ ] `SERVICE_NAME` and version variables defined before the include
- [ ] `go.mod` / `go.sum` created and referenced in `go.work`

## Code Structure

- [ ] `cmd/<name>/main.go` - Entry point
- [ ] `internal/` - Private packages
- [ ] Health endpoints implemented (`/health` or `/healthz`, `/ready` or `/readyz`)
- [ ] Metrics endpoint implemented (`/metrics`)

## Documentation

- [ ] `README.md` explains purpose, API endpoints, quick start
- [ ] `DEPLOYMENT.md` contains deployment contract (health endpoints, env vars, dependencies, resources)

## Quality Gates

- [ ] `make build` succeeds locally
- [ ] `make check` succeeds locally (lint + test)
- [ ] `go test -race ./...` passes
- [ ] Remote CI (`make ci-remote SERVICE=<name>`) completes successfully

## Deployment Readiness

- [ ] Dockerfile created
- [ ] Helm chart created at `deployments/helm/<name>/`
- [ ] ArgoCD Application created (notify infra-ops-manager)
- [ ] Environment-specific values files created

## Final Verification

- [ ] Service deploys successfully to development
- [ ] Health probes pass in Kubernetes
- [ ] Metrics visible in Prometheus
- [ ] Logs visible in Loki

Document completion of this checklist in the PR description.

## Related Documents

- [makefile-customization.md](makefile-customization.md) - Customizing service Makefiles
- Service template: `services/_template/`
