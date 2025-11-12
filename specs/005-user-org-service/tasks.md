# Tasks: User & Organization Service

**Input**: Design artifacts in `/specs/005-user-org-service/`  
**Prerequisites**: Updated `spec.md`, finalized `plan.md`, `research.md`, `data-model.md`, `contracts/user-org-service.openapi.yaml`, `quickstart.md`, rebased feature branch.

## Phase 1: Service Scaffolding & Tooling

**Purpose**: Establish project skeleton, local automation, CI hooks, and baseline documentation before building user stories.

- [x] T-S005-P01-001 Create Go module for service (`services/user-org-service/go.mod`, `go.sum`) and root `Makefile` delegating to shared templates (`build`, `test`, `lint`, `opa`, `dredd`, `k6`, `migrate`).  
- [x] T-S005-P01-002 [P] Scaffold binaries under `cmd/admin-api` and `cmd/reconciler` with configuration loading (Viper or envconfig), structured logging (`zerolog`), OpenTelemetry wiring, and health endpoints (`/healthz`, `/readyz`).  
- [x] T-S005-P01-003 [P] Author Kubernetes deployment assets (`configs/helm/Chart.yaml`, templates, values) plus Kustomize overlays per environment; ensure Helm chart exposes necessary secrets, config maps, and service monitors.  
- [x] T-S005-P01-004 Define CI workflows (`.github/workflows/user-org-service.yml`) running Go unit tests, linting, OPA regression, contract validation, and k6 smoke; integrate artifact uploads for OpenAPI schema.  
- [x] T-S005-P01-005 Populate `services/user-org-service/README.md` summarizing architecture, local development commands, SLOs, and links to spec artifacts.  
- [x] T-S005-P01-006 Add database migration framework (`migrations/` with goose/sqlc setup), including Make targets for `migrate`, `rollback`, and schema drift detection.

---

## Phase 2: Identity & Session Lifecycle (Priority: P1, maps to US-001) 🎯 MVP foundation

**Goal**: Deliver interactive auth flows (login, refresh, logout), MFA enforcement, user/org lifecycles, API key issuance/revocation, and audit logging to satisfy US-001 acceptance scenarios.

- [x] T-S005-P02-007 Design and implement Postgres repositories for `orgs`, `users`, `sessions`, `api_keys` per `data-model.md` with optimistic locking and RLS policies; add integration tests using testcontainers.  
- [ ] T-S005-P02-008 Wire authentication service with `ory/fosite` for password + MFA flows, `go-oidc` for IdP federation stubs, and session issuance with Redis caching; expose `/v1/auth/login`, `/refresh`, `/logout`.  
- [ ] T-S005-P02-009 Implement user/org lifecycle handlers (`/v1/orgs`, `/v1/orgs/{slug}/invites`, `/v1/orgs/{slug}/users/...`) including invite expiry, suspension, and recovery; ensure audit events emitted via Kafka.  
- [ ] T-S005-P02-010 [P] Build API key lifecycle (`/v1/orgs/{slug}/service-accounts/.../api-keys`, `/v1/orgs/{slug}/api-keys/{id}`) using Vault Transit for secret material, hashed fingerprints, rotation policies, and revocation propagation to Redis.  
- [ ] T-S005-P02-011 [P] Implement API and CLI flows for credential recovery and lockout handling, including admin approval workflow and audit trails.  
- [ ] T-S005-P02-012 Create end-to-end tests (integration + k6 smoke) exercising login → key issuance → revocation; document manual verification steps in `quickstart.md` and update acceptance test list.  
- [ ] T-S005-P02-013 Update metrics collectors for authentication success/failure, sessions, API key issuance, and revocations; surface dashboards + alerts in `docs/observability/user-org-service-auth.json`.
- [ ] T-S005-P02-014 Implement break-glass support tooling (privileged escalation workflow, multi-party approval, expiring access tokens) with corresponding APIs, alerts, and detailed runbooks to satisfy FR-019.

---

## Phase 3: Authorization, Policy, & Budget Enforcement (Priority: P1, maps to US-002)

**Goal**: Enforce role-based access, budget policies, override workflows, and PDP contracts with deterministic denials and auditing.

- [ ] T-S005-P03-015 Implement policy engine using embedded OPA (bundle loader, decision cache) with Rego policies for roles/scopes, budget states, override approvals; store bundles under `policies/` with tests.  
- [ ] T-S005-P03-016 [P] Expose `/v1/policy/evaluate` PDP endpoint with signed responses, TTL, obligations metadata, and integrate with internal authorization middleware for admin API.  
- [ ] T-S005-P03-017 Build budget ingestion pipeline: consume `billing.usage` Kafka topic, aggregate spend snapshots, update Postgres budget tables, trigger warn/block thresholds, and emit notifications via shared service.  
- [ ] T-S005-P03-018 Implement budget override APIs (`/v1/budgets/{slug}/overrides`) with dual approval enforcement, SLA timers, and state transitions; add UI/API friendly error codes.  
- [ ] T-S005-P03-019 Extend audit producer to record policy IDs, denial reasons, and override events; add Prometheus metrics for policy decisions and budget alerts.  
- [ ] T-S005-P03-020 Author regression tests covering deny/allow permutations, override expiry, budget oscillation throttling; include conftest playbooks and k6-based denial latency checks.  
- [ ] T-S005-P03-021 Document policy configuration and override process in new `docs/runbooks/user-org-policy.md` and update quickstart budget drill section with CLI/API examples.
- [ ] T-S005-P03-022 Design and implement rate limiting, throttling, and abuse detection for high-risk endpoints (FR-020), including configuration management, telemetry, and automated tests to verify enforcement under load.  
- [ ] T-S005-P03-023 Conduct performance and capacity testing to validate NFR-008 (20k orgs / 500k keys / 10k auth decisions per minute), capturing k6 scenarios, scaling benchmarks, and remediation plans.

---

## Phase 4: Declarative Management & Reconciliation (Priority: P2, maps to US-003)

**Goal**: Deliver Git-backed declarative state reconciliation with drift detection, manual sync APIs, and pause/resume flows.

- [ ] T-S005-P04-024 Implement Git client (`go-git`) with webhook/polling triggers, commit verification, and configuration schema validation; populate declarative revision tables.  
- [ ] T-S005-P04-025 Build reconciliation worker (cmd/reconciler) with job queue from Kafka `identity.reconcile`, performing diff, validation, apply, rollback, and status updates.  
- [ ] T-S005-P04-026 Add APIs (`/v1/declarative/config`, `/v1/declarative/status/{org}`, `/v1/declarative/pause`) for manual sync, status query, and pause/resume with guardrails when interactive changes requested.  
- [ ] T-S005-P04-027 [P] Implement drift detection logic comparing live state vs declared config, producing detailed diff snapshots and raising alerts/webhooks; ensure quickstart drift exercise works end-to-end.  
- [ ] T-S005-P04-028 Extend observability: metrics for reconcile durations, backlog, drift counts; traces linking admin actions to reconciler spans; dashboards `docs/observability/user-org-service-reconcile.json`.  
- [ ] T-S005-P04-029 Author end-to-end tests simulating merge events, conflicting manual changes, reconciliation failure recovery, and ensure results recorded in audit log with diff attachments.

---

## Phase 5: Audit, Compliance, & Reporting (Priority: P2, maps to US-004)

**Goal**: Ensure tamper-evident audit logging, exports, retention, and compliance reporting for identity lifecycle activities.

- [ ] T-S005-P05-030 Harden Kafka audit pipeline: define topic schema, retention policies, partition strategy, retries, and batcher for forwarding to Loki/object storage with Ed25519 signatures.  
- [ ] T-S005-P05-031 Build audit export service (`/v1/audit/export`) producing signed NDJSON/Parquet blobs in S3-compatible storage, with asynchronous job tracker and integrity verification CLI.  
- [ ] T-S005-P05-032 [P] Implement compliance dashboards and scheduled reports for SOC2/ISO controls, integrating with quickstart verification commands.  
- [ ] T-S005-P05-033 Add runbooks for audit backlog handling, export troubleshooting, and compliance handoffs (`docs/runbooks/user-org-audit.md`).  
- [ ] T-S005-P05-034 Conduct disaster recovery drills for audit data (backup/restore), documenting RPO/RTO results and linking to success criteria SC-006/SC-007 evidence.
- [ ] T-S005-P05-035 Automate primary data backups and configuration snapshots (Postgres, Redis metadata, Vault policies) meeting RPO ≤ 15 minutes, with monitoring and alerting for failures (FR-021, NFR-010).  
- [ ] T-S005-P05-036 Execute full disaster-recovery exercises (restore to clean environment, verify data integrity, validate RTO ≤ 60 minutes), capture evidence, and update supporting documentation and runbooks.

---

## Phase 6: Service Accounts & Automation Safety (Priority: P3, maps to US-005)

**Goal**: Enable scoped service accounts, automated credential rotation, anomaly detection, and emergency revocation.

- [ ] T-S005-P06-037 Implement service account lifecycle APIs (`/v1/orgs/{slug}/service-accounts`) with role assignments, metadata, rotation schedules, and webhook notifications for rotations.  
- [ ] T-S005-P06-038 Build automated rotation scheduler (cron job or CI workflow) invoking rotation API, enforcing policy guardrails, and updating metrics; integrate with alerts on failure.  
- [ ] T-S005-P06-039 Add anomaly detection hooks to flag unusual usage (rate spikes, geo anomalies) and auto quarantine keys with configurable thresholds.  
- [ ] T-S005-P06-040 [P] Extend quickstart with automation scenarios, emergency revocation drills, and integration guidance for downstream services; update llms context accordingly.  
- [ ] T-S005-P06-041 Develop integration tests verifying rotation success, failure remediation paths, and emergency revocation propagation under load.
- [ ] T-S005-P06-042 Implement localization and accessibility support (EN + secondary language content, email templates, UI copy, WCAG 2.1 AA validation) and document translation workflow (NFR-017).

---

## Phase 7: Cross-Cutting Resilience, Observability, & Documentation

**Purpose**: Final polish across resilience, SLO monitoring, support readiness, and governance alignment.

- [ ] T-S005-P07-043 Execute chaos experiments (latency injection for dependencies) to validate SC-007; document mitigations and ensure retries/circuit-breaking tuned appropriately.  
- [ ] T-S005-P07-044 [P] Finalize dashboards/alerts for auth latency, reconciliation backlog, budget denials, audit pipeline lag; ensure SLO burn rate alerts wired to incident channels.  
- [ ] T-S005-P07-045 Produce support runbooks (drift triage, budget override escalation, MFA recovery) and training deck for platform operations.  
- [ ] T-S005-P07-046 Update `llms.txt` with links to spec, plan, quickstart, data model, contracts, observability dashboards, and runbooks for AI agents.  
- [ ] T-S005-P07-047 Prepare rollout checklist (staged deploy, canary strategy, feature flag toggles, rollback plan) and obtain security/compliance sign-off.  
- [ ] T-S005-P07-048 Run `/speckit.analyze` and resolve findings; capture constitution compliance summary in `plan.md` and mark tasks complete.
- [ ] T-S005-P07-049 Design and validate multi-region failover and degraded read-only mode (FR-022), including replication strategy, failover automation, observability, and chaos drills with documented outcomes.  
- [ ] T-S005-P07-050 Define API versioning and backward-compatibility policy (NFR-018), implement schema negotiation/tests, and document change-management process for downstream consumers.

---

## Dependencies & Execution Order

- Complete Phase 1 before implementing user stories (Phases 2–6).  
- Phases 2 & 3 (US-001/US-002) form MVP and must complete before declarative (Phase 4) or audit/reporting (Phase 5).  
- Phase 4 requires Vault, Kafka, and policy groundwork from Phases 2–3.  
- Phase 5 depends on audit streams established in Phases 2–3; Phase 6 builds on service account + key infrastructure from earlier phases.  
- Phase 7 runs after user story features achieve acceptance but can begin documentation tasks in parallel.

### Parallel Opportunities

- Tasks marked `[P]` may proceed concurrently when they touch separate components (e.g., API dev vs. infrastructure docs).  
- After Phase 1, teams can split between auth lifecycle (Phase 2) and policy enforcement (Phase 3) as long as shared database migrations are coordinated.  
- Declarative reconciliation (Phase 4) and audit reporting (Phase 5) can overlap once policy/audit pipelines are stable.

## Delivery Strategy

- **MVP**: Deliver Phases 1–3 to unlock interactive governance, budgets, and authorization for first tenants.  
- **Incremental Enhancements**: Layer declarative management (Phase 4) and audit/compliance (Phase 5) to support GitOps + regulatory requirements.  
- **Automation & Polish**: Complete service account automation (Phase 6) and resilience/observability (Phase 7) before GA; finalize documentation and compliance artifacts.  
- Track progress via success criteria (SC-001 – SC-008) and update dashboards/runbooks as features land.

