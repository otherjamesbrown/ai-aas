# Feature Specification: API Router Service

**Feature Branch**: `006-api-router-service`  
**Created**: 2025-11-07  
**Status**: Draft  
**Input**: User description: "Provide a single inference entrypoint that authenticates requests, enforces organization budgets and quotas, selects the appropriate model backend, and returns responses reliably. Include usage tracking and clear health/status for operations. The outcome is predictable latency, accurate accounting, and safe routing under load."

## Clarifications

### Session 2025-11-10

- **Q:** How will organizations authenticate?  
  **A:** All clients authenticate via signed API keys issued per organization plus optional request-level HMAC. OAuth is out of scope for this release.
- **Q:** What routing controls do operations need day one?  
  **A:** Weighted routing by model, emergency kill switches per backend, and the ability to mark a backend as “degraded” for auto-failover.
- **Q:** Is there a preferred accounting cadence?  
  **A:** Near-real-time (≤1 minute lag) usage updates into shared telemetry so finance dashboards stay current.
- **Q:** Which workloads must be supported initially?  
  **A:** Text completions (synchronous responses) with payloads ≤64 KB. Streaming responses and multimodal payloads are future work.

## Assumptions

- Organization hierarchy, billing accounts, and quota definitions already exist in shared services (`003-database-schemas`, `004-shared-libraries`).
- The router owns ingress authentication, but downstream model services still verify request signatures before execution.
- Routing policies are supplied by the configuration service and refreshable without redeploys.
- Usage accounting pushes into the centralized telemetry pipeline defined in `005-usage-analytics`.
- Latency targets assume models are deployed within the same region as the router; cross-region hops are out of scope.

## User Scenarios & Testing *(mandatory)*

### User Story 1 (US-001) - Route authenticated inference requests (Priority: P1)

As a client, I can send a request with valid credentials and receive a completion from the selected model backend.

**Why this priority**: Delivers the core product capability to end users.

**Independent Test**: Issue a request with valid credentials and observe a successful completion payload with router-computed latency metrics.

**Acceptance Scenarios**:

1. **[Primary]** **Given** a valid credential and model name, **When** the client submits a request, **Then** the router validates the credential, selects the configured backend, forwards the request, and returns the completion in the documented response schema.  
2. **[Exception]** **Given** an expired or revoked API key, **When** the client submits a request, **Then** the router responds with HTTP 401 including an error code and does not contact any backend.  
3. **[Alternate]** **Given** a backend flagged as degraded, **When** a request targets that backend, **Then** the router applies failover routing rules and returns the alternate backend response while surfacing a warning header.

---

### User Story 2 (US-002) - Enforce budgets and safe usage (Priority: P1)

As an organization admin, I want budget and quota rules applied consistently so spend is controlled and fair use is enforced.

**Why this priority**: Prevents overruns and maintains service quality.

**Independent Test**: Simulate requests that reach or exceed limits and observe denial/throttling plus durable audit records.

**Acceptance Scenarios**:

1. **[Primary]** **Given** an organization that has exhausted its monthly budget, **When** a new request is made, **Then** the router rejects it with HTTP 402, includes remaining limit context, and records the denial in the usage log.  
2. **[Exception]** **Given** burst traffic that exceeds rate limits, **When** clients continue sending, **Then** the router enforces token-bucket throttling with Retry-After guidance without overloading backends.  
3. **[Recovery]** **Given** limits are reset via admin action, **When** the next request arrives, **Then** the router authorizes it and clears any temporary lockouts for that organization.

---

### User Story 3 (US-003) - Intelligent routing and fallback (Priority: P2)

As a platform engineer, I can configure routing strategies and observe decisions so we can balance load and recover from backend failures automatically.

**Why this priority**: Ensures continuity of service when any backend degrades.

**Independent Test**: Apply routing configuration fixtures and simulate backend outages while validating logs and metrics capture routing rationale.

**Acceptance Scenarios**:

1. **[Primary]** **Given** multiple backends for a model with weights configured, **When** requests arrive, **Then** the router distributes traffic per weight and records decision metadata.  
2. **[Exception]** **Given** the primary backend returns consecutive errors above the failover threshold, **When** new requests arrive, **Then** the router shifts traffic to secondary backends and emits alerts.  
3. **[Recovery]** **Given** the primary backend recovers, **When** health checks succeed for the warm-up window, **Then** the router gradually restores normal weights without dropping requests.

---

### User Story 4 (US-004) - Accurate, timely usage accounting (Priority: P2)

As finance and analytics stakeholders, we can consume near-real-time usage records grouped by organization, model, and API key to reconcile billing and monitor adoption.

**Why this priority**: Accurate usage data enables billing, alerts, and forecasting.

**Independent Test**: Generate requests with known token counts and verify emitted records land in telemetry within the SLA window.

**Acceptance Scenarios**:

1. **[Primary]** **Given** successful inference requests, **When** usage exporters run, **Then** they publish records with organization, API key, model, token usage, cost, and latency metadata within 60 seconds.  
2. **[Exception]** **Given** telemetry pipeline backpressure, **When** exporters cannot publish, **Then** records are buffered with at-least-once guarantees and operators are alerted.  
3. **[Alternate]** **Given** a disputed request ID, **When** finance queries the audit API, **Then** they receive trace metadata including timestamps, routing decisions, and limit status.

---

### User Story 5 (US-005) - Operational visibility and reliability (Priority: P3)

As an operator, I can view health/status, track usage, and identify errors quickly to keep service reliable.

**Why this priority**: Enables rapid diagnosis and stable operations under load.

**Independent Test**: Inspect status endpoints, synthetic checks, and correlate usage counters with client requests.

**Acceptance Scenarios**:

1. **[Primary]** **Given** the service is healthy, **When** I hit `/healthz` and `/readyz`, **Then** they return HTTP 200 with component-level indicators and build metadata.  
2. **[Exception]** **Given** downstream latency spikes, **When** I review metrics dashboards, **Then** I observe per-backend latency, error percentages, and routed request counts highlighting the issue.  
3. **[Recovery]** **Given** an incident, **When** I follow the runbook, **Then** I can drain traffic, toggle routing policies, and confirm recovery via dashboards within 15 minutes.

---

### Edge Cases

- Backend unavailability or intermittent timeouts; balance retry versus fail-fast without double-billing.  
- Model name conflicts or missing mappings; return deterministic error codes and remediation guidance.  
- Budget checks racing with concurrent requests; ensure atomic limit enforcement with idempotent request IDs.  
- Large payloads or streaming behavior impacting fairness; enforce payload size and rate gating.  
- Requests arriving precisely at budget reset boundaries; confirm correct rollover handling.  
- Telemetry or usage exporter outages; ensure buffered writes and eventual reconciliation.  
- Misconfigured routing policies; validate on load and fail safe to default routing with warnings.  
- Health endpoint abuse; rate-limit and authenticate sensitive diagnostics.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide credential validation for both interactive users and programmatic clients, including key revocation checks and HMAC signature verification.  
- **FR-002**: Provide request schema validation (model, prompt, parameters) with descriptive errors before contacting any backend.  
- **FR-003**: Provide routing engine that selects backends based on configured weights, health status, and organization-level policies.  
- **FR-004**: Provide budget and quota enforcement with consistent deny/queue behavior and auditable decision metadata.  
- **FR-005**: Provide per-organization and per-key rate limiting with configurable token-bucket policies.  
- **FR-006**: Provide usage accounting (counts, tokens, cost) and emit structured records via telemetry exporters within 60 seconds.  
- **FR-007**: Provide idempotency handling for client-supplied request IDs to avoid double-billing during retries.  
- **FR-008**: Provide configurable timeout, retry, and backpressure policies to protect both router and backends.  
- **FR-009**: Provide admin APIs or configuration hooks to update routing policies, rate limits, and kill switches without redeploy.  
- **FR-010**: Provide health, readiness, and diagnostics endpoints exposing aggregated backend status and router self-checks.  
- **FR-011**: Provide structured logging with correlation IDs linking ingress requests to backend calls and usage records.  
- **FR-012**: Provide alerting hooks (metrics, events) for routing failures, quota breaches, exporter backlog, and auth anomalies.  
- **FR-013**: Provide deterministic error responses with machine-readable codes covering auth failures, quota violations, routing errors, and backend faults.  
- **FR-014**: Provide audit trail APIs allowing finance and compliance teams to query request history by organization or request ID.

### Key Entities

- **RouteRequest**: Represents an inbound inference request including API key, organization, requested model, payload metadata, and idempotency token.  
- **RoutingPolicy**: Configurable rules (weights, failover thresholds, allow/deny lists) applied per organization or model.  
- **BackendEndpoint**: Individual model deployment with health state, capacity, latency SLOs, and connection details.  
- **UsageRecord**: Accounting artifact with request identifiers, token usage, cost calculations, routing decision, and limit status.  
- **BudgetConstraint**: Pre-computed monthly or daily spend/token caps with enforcement state and reset cadence.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Valid authenticated requests succeed with end-to-end response in ≤3 seconds for 95% of calls under planned load.  
- **SC-002**: Invalid or unauthorized requests are denied 100% of the time with a clear message and without backend contact.  
- **SC-003**: Budget and quota violations are enforced consistently; at least three independent test scenarios confirm correct behavior.  
- **SC-004**: Health/status endpoints respond in ≤1 second under normal conditions and reflect degraded states during backend outages.  
- **SC-005**: Usage counters reconcile to within 1% of client-side request counts over a 24-hour window.

## Non-Functional Requirements

**Performance**  
- **NFR-001**: Router introduces ≤150 ms median overhead per request and ≤400 ms p95 under planned load.  
- **NFR-002**: Rate limiting decisions occur in ≤5 ms to avoid impacting latency budgets.  
- **NFR-003**: Telemetry export pipeline processes 10k records per minute without backlog.

**Reliability & Availability**  
- **NFR-004**: Router maintains 99.9% availability with zero single points of failure; HA deployment and rolling upgrades are supported.  
- **NFR-005**: Failover to secondary backends completes within 30 seconds of detecting a primary outage.  
- **NFR-006**: At-least-once delivery for usage records with safeguards against duplicates.

**Security & Compliance**  
- **NFR-007**: Secrets and API keys are encrypted at rest and in transit; in-memory exposure is limited to the signed duration.  
- **NFR-008**: All admin or configuration changes are authenticated, authorized, and audited.  
- **NFR-009**: Error responses exclude sensitive backend details while remaining actionable for clients.

**Observability**  
- **NFR-010**: Emit RED metrics (Rate, Errors, Duration) per route and per backend, compatible with the central monitoring stack.  
- **NFR-011**: Trace spans link ingress, routing decision, backend call, and exporter publish for ≥95% of requests.  
- **NFR-012**: Alerting thresholds defined for latency, error rate, quota enforcement failures, and exporter backlog.

**Scalability & Capacity**  
- **NFR-013**: Support sustained 1k RPS with linear scale-out via horizontal replicas and sharded rate limiters.  
- **NFR-014**: Configuration propagation to router replicas occurs in ≤30 seconds after change.  
- **NFR-015**: Storage for buffered usage records retains 24 hours of traffic without data loss.

**Operability**  
- **NFR-016**: Health endpoints and dashboards provide enough detail to diagnose 80% of incidents without SSH access.  
- **NFR-017**: Runbooks cover routing policy updates, backend failover, and exporter backlog clearance.  
- **NFR-018**: Deployment artifacts integrate with shared automation (Makefile, CI) to enable blue/green rollouts.

## Traceability Matrix

| User Story | Primary Requirements | Supporting NFRs | Notes |
|------------|---------------------|-----------------|-------|
| US-001 | FR-001, FR-002, FR-003, FR-013 | NFR-001, NFR-007, NFR-010 | Core ingress path and auth. |
| US-002 | FR-004, FR-005, FR-007, FR-014 | NFR-002, NFR-006, NFR-012 | Limit enforcement and auditability. |
| US-003 | FR-003, FR-008, FR-009, FR-010, FR-012 | NFR-004, NFR-005, NFR-010, NFR-016 | Routing intelligence and resilience. |
| US-004 | FR-006, FR-007, FR-011, FR-012, FR-014 | NFR-003, NFR-006, NFR-011, NFR-015 | Usage accounting and telemetry. |
| US-005 | FR-008, FR-010, FR-011, FR-012 | NFR-004, NFR-010, NFR-012, NFR-016, NFR-018 | Operational visibility tooling. |

## Out of Scope

- Support for streaming responses, multimodal payloads, or long-running operations (deferred to future specs).  
- Self-service API key issuance or OAuth flows (handled by identity platform).  
- Automatic model provisioning or scaling decisions (owned by model-serving teams).  
- Direct billing calculations or invoice generation (handled by finance systems consuming usage data).  
- Regional replication beyond the primary deployment region.
