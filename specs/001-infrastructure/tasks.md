# Tasks: Infrastructure Provisioning

**Input**: Design documents from `/specs/001-infrastructure/`  
**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish repository scaffolding, tooling versions, and automation entry points.

- [ ] T001 Create infrastructure directory tree (`infra/terraform/`, `infra/helm/`, `infra/argo/`, `infra/tests/`, `scripts/infra/`, `infra/secrets/`) with placeholder `README.md` files describing responsibilities.
- [ ] T002 [P] Document infrastructure layout and ownership in `infra/README.md` referencing new directories.
- [ ] T003 [P] Configure Terraform remote backend (`infra/terraform/backend.hcl`) targeting Linode Object Storage and add state policy JSON under `infra/terraform/state-policy.json`.
- [ ] T004 [P] Author `infra/terraform/Makefile` with `plan/apply/destroy/drift/state-pull` targets consuming `ENV` variable and invoking shared scripts.
- [ ] T005 Update `configs/tool-versions.mk` and root `Makefile` to pin Terraform, Helm, Terratest, `tfsec`, `tflint`, and expose `make infra-plan/apply`.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Shared provider configuration, security scanning, and automation scripts required before user story work.

- [ ] T006 Define shared provider configuration (`infra/terraform/environments/_shared/providers.tf`, `versions.tf`, `locals.tf`) with Linode/Kubernetes providers, backend data sources, and tagging conventions.
- [ ] T007 [P] Add Terraform linting and security tooling (`infra/terraform/.tflint.hcl`, `.terraform.lock.hcl`, `tfsec` config) and wire checks into `.github/workflows/infra-terraform.yml`.
- [ ] T008 Create infrastructure automation scripts (`scripts/infra/plan.sh`, `apply.sh`, `drift-detect.sh`, `rollback.sh`, `validate.sh`, `common.sh`) implementing plan/apply approval gates, Slack notifications, and drift detection CLI.
- [ ] T009 [P] Scaffold Terratest harness (`tests/infra/terratest/go.mod`, `go.sum`, `helpers/linode_client.go`, `suite_test.go`) with test utilities for provisioning smoke tests.
- [ ] T010 Establish GitHub Actions workflow (`.github/workflows/infra-terraform.yml`) executing plan, tfsec, tflint, Terratest, drift detection scheduling, and enforcing manual approval for production applies.

---

## Phase 3: User Story 1 - Stand up foundational environments safely (Priority: P1) 🎯 MVP

**Goal**: Provision development, staging, production, and system environments with isolated resources and automated rollback.  
**Independent Test**: Run `make infra-plan/apply ENV=development` followed by Terratest smoke to confirm namespaces, node pools, and access succeed for each environment.

- [ ] T011 [US1] Implement `infra/terraform/modules/lke-cluster` (variables, main, outputs) creating LKE clusters, node pools, and kubeconfig outputs with validations for GPU pools and multi-AZ.
- [ ] T012 [P] [US1] Implement `infra/terraform/modules/network` defining firewalls, Calico NetworkPolicies, ingress/egress allowlists, and DNS records.
- [ ] T013 [P] [US1] Implement `infra/terraform/modules/secrets-bootstrap` provisioning Linode secrets manager items, Sealed Secrets jobs, and rotation metadata outputs.
- [ ] T014 [US1] Implement `infra/terraform/modules/observability` deploying kube-prometheus-stack, Loki, Tempo Helm releases with environment overlays stored under `infra/helm/values/<env>.yaml`.
- [ ] T015 [US1] Implement `infra/terraform/modules/argo-bootstrap` installing ArgoCD, ApplicationSet definitions, and project RBAC.
- [ ] T016 [US1] Compose environment stacks (`infra/terraform/environments/{development,staging,production,system}/main.tf`) wiring modules, environment-specific variables, quotas, and outputs.
- [ ] T017 [P] [US1] Add environment variable files and documentation (`infra/terraform/environments/{env}/variables.tf`, `README.md`) describing inputs, outputs, and quotas.
- [ ] T018 [US1] Configure shared data store endpoints and service endpoints module (`infra/terraform/modules/data-services`) exposing per-environment PostgreSQL/Redis connection info and outputs consumed by handoff contracts.
- [ ] T019 [US1] Extend Terratest suite (`tests/infra/terratest/environments_test.go`) to create ephemeral cluster, verify namespaces, quotas, access, and teardown with Linode API mocks where possible.
- [ ] T020 [US1] Implement state snapshot and backup automation (`scripts/infra/state-backup.sh`, GitHub Actions scheduled job) storing Terraform state versions and recording metadata in Object Storage.
- [ ] T021 [US1] Update documentation (`docs/platform/infrastructure-overview.md`, `specs/001-infrastructure/quickstart.md`) with provisioning workflow, timelines, and validation commands for each environment.
- [ ] T022 [P] [US1] Implement synthetic availability probes (`tests/infra/synthetics/availability_probe.go`) hitting Kubernetes control plane `/readyz` endpoints for each environment.
- [ ] T023 [US1] Add GitHub Actions workflow `.github/workflows/infra-availability.yml` with Grafana alert routing that enforces the 99.5% availability SLO and notifies `#platform-infra` on breach.
- [ ] T024 [P] [US1] Define capacity quotas and load validation (`infra/terraform/modules/lke-cluster/quotas.tf`, `tests/infra/perf/capacity_test.go`) to prove support for 30 services per environment and document results in `docs/platform/infrastructure-overview.md`.

---

## Phase 4: User Story 2 - Application teams can deploy a sample service (Priority: P2)

**Goal**: Enable application teams to deploy a sample service using documented handoff and observe successful health checks.  
**Independent Test**: Run sample service deployment via ArgoCD workflow, confirm `/healthz` reachable, metrics emitted, and quickstart instructions succeed end-to-end.

- [ ] T025 [US2] Create sample service Helm chart (`infra/helm/charts/sample-service/`) with configurable image, replicas, ingress, service account, and metrics endpoints plus README.
- [ ] T026 [P] [US2] Define ArgoCD ApplicationSet (`infra/argo/applications/sample-service.yaml`) templating per-environment deployment with sync waves and toggle for production manual approval.
- [ ] T027 [US2] Implement integration tests (`tests/infra/integration/sample_service_deploy_test.go`) deploying chart, validating ingress availability, and tearing down resources.
- [ ] T028 [P] [US2] Add GitHub Actions workflow (`.github/workflows/sample-service-smoke.yml`) executing integration tests against development cluster on demand.
- [ ] T029 [US2] Update quickstart, handoff contract, and access docs (`specs/001-infrastructure/quickstart.md`, `specs/001-infrastructure/contracts/environment-access.md`, `contracts/service-handoff-openapi.yaml`) with deployment steps, observability links, and troubleshooting notes.

---

## Phase 5: User Story 3 - Secure-by-default posture (Priority: P3)

**Goal**: Enforce default isolation, secrets hygiene, and auditability across environments.  
**Independent Test**: Attempt unauthorized cross-environment access, rotate secrets via documented flow, and verify alerts/logs capture events with automated notification within 5 minutes.

- [ ] T030 [US3] Author baseline NetworkPolicies (`infra/helm/charts/network-policies/templates/*.yaml`) enforcing namespace isolation, required egress, and documented override annotations.
- [ ] T031 [P] [US3] Implement RBAC bundles (`infra/helm/charts/access-packages/`) generating roles/bindings per AccessPolicy and integrate with access package generator script.
- [ ] T032 [P] [US3] Build secrets rotation workflow (`scripts/infra/secrets/{sync.sh,rotate.sh}`, `infra/secrets/bundles/*.yaml`) and schedule rotation pipeline (`.github/workflows/infra-secrets-rotation.yml`).
- [ ] T033 [US3] Extend network Terratest suite (`tests/infra/terratest/network_policies_test.go`) to attempt cross-environment traffic and assert default deny behavior with evidence captured in CI artifacts.
- [ ] T034 [US3] Configure observability alerts and dashboards (`infra/helm/charts/observability-stack/templates/alerts/*.yaml`, `dashboards/access-overview.json`) covering secret freshness, drift, and access anomalies.
- [ ] T035 [US3] Implement audit log shipping (`infra/helm/charts/logging/templates/loki-promtail.yaml`, `scripts/infra/audit-export.sh`) tagging events with environment/actor metadata and forwarding to Loki/Tempo.
- [ ] T036 [US3] Update security documentation (`docs/platform/access-control.md`, `docs/platform/observability-guide.md`, `docs/runbooks/infrastructure-troubleshooting.md`) with rotation cadence, alert responses, and audit procedures.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Ensure documentation, runbooks, and automation align; finalize compliance.

- [ ] T037 [P] Refresh runbooks (`docs/runbooks/infrastructure-rollback.md`, `docs/runbooks/infrastructure-troubleshooting.md`) with screenshots, command samples, and incident templates.
- [ ] T038 [P] Validate quickstart end-to-end by executing provisioning + sample deploy on fresh Linode account, capturing timings for success criteria.
- [ ] T039 Perform repository-wide linting (`make lint`, `terraform fmt`, `helm lint`) and ensure GitHub Actions pass for all new pipelines.
- [ ] T040 Schedule automated rollback drill workflow (`.github/workflows/infra-rollback-drill.yml`) and log quarterly execution results in `docs/runbooks/infrastructure-rollback.md`.
- [ ] T041 Conduct constitution gate checklist (`/speckit.analyze` follow-up) documenting compliance notes in `specs/001-infrastructure/plan.md` or follow-up log.

---

## Dependencies & Execution Order

- Phase 1 must complete before Phase 2.  
- Phase 2 must complete before any user story phases.  
- User Story phases (3–5) may begin after Phase 2; US1 should complete before US2/US3 to supply cluster artifacts.  
- Phase 6 runs after targeted user stories reach "ready" state.

### Parallel Opportunities

- Tasks marked `[P]` operate on different files (e.g., Terraform modules vs. documentation) and can run concurrently.  
- After Phase 2, teams can pursue US2 and US3 in parallel once US1 establishes baseline resources, subject to coordination on shared modules.

## Implementation Strategy

- **MVP**: Deliver Phase 1–3 to provision environments and validate smoke tests.  
- **Incremental Delivery**: Layer US2 (sample service deploy) next, followed by US3 (security hardening).  
- **Polish**: Execute Phase 6 tasks to finalize documentation, linting, and compliance checks.

