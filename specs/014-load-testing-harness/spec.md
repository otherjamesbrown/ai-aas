# Feature Specification: Load Testing Harness

**Feature Branch**: `014-load-testing-harness`
**Created**: 2025-01-27
**Status**: Draft
**Input**: User description: "Create a scalable load testing harness that can simulate hundreds to thousands of concurrent users making realistic API calls to the platform. Each test container must bootstrap itself from central configuration, controlling organization count, users per organization, and API keys. The system must support realistic user behavior patterns with configurable think times between requests and unique question generation to avoid cache hits. The harness must be Kubernetes-native and capable of scaling to thousands of concurrent simulated users."

## User Scenarios & Testing *(mandatory)*

<!--
  IMPORTANT: User stories should be PRIORITIZED as user journeys ordered by importance.
  Each user story/journey must be INDEPENDENTLY TESTABLE - meaning if you implement just ONE of them,
  you should still have a viable MVP (Minimum Viable Product) that delivers value.

  Assign priorities (P1, P2, P3, etc.) to each story, where P1 is the most critical.
  Think of each story as a standalone slice of functionality that can be:
  - Developed independently
  - Tested independently
  - Deployed independently
  - Demonstrated to users independently
-->

### User Story 1 - Single organization realistic load test (Priority: P1)

As a platform engineer, I can execute a load test that simulates 10-100 concurrent users within a single organization, with realistic think times and unique questions, to validate platform behavior under normal load.

**Why this priority**: Validates the core platform functionality with a realistic user base before scaling to multi-org scenarios.

**Independent Test**: Can be tested by deploying the load test with a single organization configuration and verifying all simulated users complete their sessions successfully with expected latency and error rates.

**Acceptance Scenarios**:

1. **Given** a load test configuration for 1 org with 50 users, **When** the test executes, **Then** all 50 users bootstrap successfully, receive API keys, and complete their question sessions with <5% error rate.
2. **Given** running load test, **When** I query metrics, **Then** I see real-time latency percentiles (p50, p90, p95, p99), throughput, token usage, and cost metrics per user.
3. **Given** completed load test, **When** I review results, **Then** I can identify performance bottlenecks, error patterns, and resource utilization issues with clear diagnostics.

---

### User Story 2 - Multi-organization scalability test (Priority: P2)

As a platform engineer, I can execute a load test that simulates 2-3 organizations with varying user counts to validate multi-tenancy isolation, budget enforcement, and resource fairness.

**Why this priority**: Ensures platform can handle multiple organizations simultaneously without cross-tenant interference.

**Independent Test**: Can be tested by deploying a multi-org configuration and verifying each organization's users operate independently with proper isolation and budget enforcement.

**Acceptance Scenarios**:

1. **Given** a load test with 3 orgs (10, 20, 30 users each), **When** tests execute concurrently, **Then** each organization's metrics are isolated, budgets are enforced independently, and no cross-tenant interference occurs.
2. **Given** one organization exceeding budget, **When** that org's users hit limits, **Then** only that organization's requests are throttled while other organizations continue normally.
3. **Given** completed multi-org test, **When** I review metrics, **Then** I can compare performance, cost, and behavior across organizations to identify fairness issues.

---

### User Story 3 - Realistic user behavior simulation (Priority: P1)

As a platform engineer, I can configure realistic user behavior patterns including variable think times, conversation context, and diverse question strategies to accurately simulate production workloads.

**Why this priority**: Ensures load tests reflect actual user behavior rather than artificial hammering, providing realistic performance data.

**Independent Test**: Can be tested by reviewing generated questions for uniqueness, measuring actual think times between requests, and validating conversation context is maintained.

**Acceptance Scenarios**:

1. **Given** user behavior config with 5-30s think time, **When** simulated users run, **Then** actual think times follow the configured distribution (exponential/gaussian) and requests are properly spaced.
2. **Given** question strategy configuration, **When** users generate questions, **Then** each user produces unique questions based on their seed (org_id + user_id) with no cache hits across users.
3. **Given** multi-turn conversations enabled, **When** users ask follow-up questions, **Then** conversation context is maintained and each question builds on previous answers.

---

### User Story 4 - Dynamic load patterns (Priority: P3)

As a platform engineer, I can configure dynamic load patterns (ramp-up, sustained, spike, cool-down) to test platform behavior under varying conditions and validate auto-scaling.

**Why this priority**: Ensures platform can handle traffic variations and scale appropriately.

**Independent Test**: Can be tested by executing a multi-phase load pattern and verifying platform behavior during each phase.

**Acceptance Scenarios**:

1. **Given** a ramp-up pattern (10→1000 users over 10min), **When** test executes, **Then** users are added gradually according to the pattern and platform scales appropriately.
2. **Given** a spike pattern (100→1000 users instantly), **When** spike occurs, **Then** platform handles the load without catastrophic failures and error rates remain acceptable.
3. **Given** sustained load phase, **When** running at peak capacity, **Then** platform maintains consistent performance metrics and no resource exhaustion occurs.

---

### User Story 5 - Self-bootstrapping test workers (Priority: P2)

As a platform engineer, I can deploy test worker pods that automatically bootstrap themselves by reading central configuration, creating their assigned organization, users, and API keys without manual intervention.

**Why this priority**: Enables true scalability and reduces operational overhead for running large-scale tests.

**Independent Test**: Can be tested by deploying worker pods with only a ConfigMap reference and verifying they self-initialize completely.

**Acceptance Scenarios**:

1. **Given** a worker pod deployed with config reference, **When** pod starts, **Then** it reads config, creates its assigned organization, users, API keys, and begins simulation without errors.
2. **Given** worker pod failures during bootstrap, **When** errors occur, **Then** pod logs clearly indicate the failure point, retries appropriately, and reports status to orchestrator.
3. **Given** test completion, **When** workers finish, **Then** they publish final metrics, optionally clean up test data (based on config), and terminate gracefully.

---

### User Story 6 - Comprehensive metrics and observability (Priority: P2)

As a platform engineer, I can observe real-time and historical metrics from load tests including latency distributions, throughput, error rates, token usage, cost, and resource utilization.

**Why this priority**: Provides actionable insights to identify bottlenecks and optimize platform performance.

**Independent Test**: Can be tested by executing a load test and verifying all expected metrics are collected, exported, and visualizable in Grafana.

**Acceptance Scenarios**:

1. **Given** a running load test, **When** I view Grafana dashboards, **Then** I see real-time metrics for active users, requests/sec, latency percentiles, error rate, and cost rate.
2. **Given** completed load test, **When** I query Prometheus, **Then** I can retrieve historical data for all metrics labeled by test_run, org_id, user_id, and worker_pod.
3. **Given** performance anomalies, **When** I drill into metrics, **Then** I can correlate latency spikes with specific organizations, users, or backend models to identify root causes.

---

### Edge Cases

- Worker pod crashes during test execution; orchestrator detects and optionally respawns or marks as failed.
- Platform services unavailable during bootstrap; workers retry with exponential backoff and timeout appropriately.
- Budget exhaustion mid-test; affected users stop gracefully and report final state without cascading failures.
- Question generator produces identical questions; seeding strategy ensures uniqueness based on org + user + timestamp.
- Network partitions between workers and platform; workers detect and report connectivity issues in metrics.
- Excessive load causing platform degradation; tests honor configured limits (max cost, max error rate) and stop automatically.
- Concurrent tests interfering; namespace isolation and unique test run IDs prevent cross-test contamination.
- Large-scale tests (1000+ users) exceeding K8s resource quotas; clear pre-flight checks and resource estimation.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide a Go-based load test worker that simulates realistic user behavior including variable think times between requests.
- **FR-002**: Provide self-bootstrapping capability where workers read configuration from ConfigMap/Secret and create orgs, users, and API keys.
- **FR-003**: Provide unique question generation strategy using seeded randomness (org_id + user_id) to avoid cache hits.
- **FR-004**: Provide multi-turn conversation support where users maintain context across multiple questions.
- **FR-005**: Provide configurable load patterns (ramp-up, sustained, spike, cool-down) with phase-based user scaling.
- **FR-006**: Provide Kubernetes-native deployment using Jobs with configurable parallelism for horizontal scaling.
- **FR-007**: Provide metrics export to Prometheus Pushgateway with standardized labels (test_run, org_id, user_id, worker_pod).
- **FR-008**: Provide real-time observability via Grafana dashboards showing latency, throughput, errors, tokens, and cost.
- **FR-009**: Provide test orchestrator that manages worker lifecycle, aggregates results, and enforces limits (max cost, max errors).
- **FR-010**: Provide configurable cleanup strategy to optionally delete test data or retain for analysis.
- **FR-011**: Provide per-user metrics including questions asked, tokens consumed, cost incurred, errors encountered.
- **FR-012**: Provide test run correlation IDs and detailed logging for debugging failures.
- **FR-013**: Provide pre-flight validation to check platform availability and resource capacity before launching tests.
- **FR-014**: Provide support for testing different API endpoints (chat completions, embeddings, org management).
- **FR-015**: Provide worker pod resource limits and requests appropriate for simulating 10-50 users per pod.

### Non-Functional Requirements

- **NFR-001**: Worker pods must be lightweight (<512Mi memory, <500m CPU per pod).
- **NFR-002**: Bootstrap phase must complete within 60 seconds per worker pod.
- **NFR-003**: Metrics export must not introduce >100ms overhead per request.
- **NFR-004**: System must support scaling to 1000+ concurrent simulated users across 20-50 worker pods.
- **NFR-005**: Question generation must produce unique questions with <0.1% collision rate across 100k questions.
- **NFR-006**: Configuration changes must not require rebuilding container images (use ConfigMaps).
- **NFR-007**: Test results must be exportable to S3/MinIO for long-term retention and analysis.
- **NFR-008**: Worker failures must not cause cascading failures in other workers or platform services.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Single-org load test (50 users) completes successfully with <5% error rate and reports metrics within 10 minutes.
- **SC-002**: Multi-org load test (3 orgs, 60 total users) completes with proper isolation and independent budget enforcement.
- **SC-003**: Question uniqueness: 99.9%+ of questions are unique across all users in a 1000-user test run.
- **SC-004**: Think time accuracy: Actual think times are within ±10% of configured distribution parameters.
- **SC-005**: Worker self-bootstrap: 95%+ of workers successfully initialize without manual intervention.
- **SC-006**: Metrics availability: All defined metrics (latency, throughput, tokens, cost, errors) are queryable in Prometheus within 30 seconds of test start.
- **SC-007**: Scalability: System can support 1000 concurrent simulated users with stable performance and <10% error rate.
- **SC-008**: Observability: Grafana dashboards provide real-time visibility into test progress and platform health during load tests.
- **SC-009**: Cost control: Tests automatically stop when configured cost limit is reached with <5% overspend.
- **SC-010**: Cleanup: Test data is cleanly removed (or retained based on config) within 5 minutes of test completion.

## Architecture Overview

### Component Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Control Plane                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌──────────────────────┐      ┌──────────────────────┐        │
│  │  LoadTest ConfigMap  │      │  Test Orchestrator   │        │
│  │  (YAML Config)       │─────▶│  (Go Binary)         │        │
│  │                      │      │                      │        │
│  │  - Organizations     │      │  - Validates config  │        │
│  │  - Users per org     │      │  - Creates K8s Jobs  │        │
│  │  - API keys          │      │  - Monitors progress │        │
│  │  - Load pattern      │      │  - Aggregates metrics│        │
│  │  - User behavior     │      │  - Enforces limits   │        │
│  │  - Metrics config    │      │  - Manages cleanup   │        │
│  │  - Cleanup policy    │      └──────────┬───────────┘        │
│  └──────────────────────┘                 │                     │
│                                            │                     │
└────────────────────────────────────────────┼─────────────────────┘
                                             │
                                             │ Creates K8s Jobs
                                             │
┌────────────────────────────────────────────▼─────────────────────┐
│                     Data Plane (Workers)                          │
├───────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────────────┐  ┌──────────────────┐  ┌────────────────┐ │
│  │ Worker Pod 1     │  │ Worker Pod 2     │  │ Worker Pod N   │ │
│  │ (Go + embedded   │  │ (Go + embedded   │  │ (Go + embedded │ │
│  │  question gen)   │  │  question gen)   │  │  question gen) │ │
│  │                  │  │                  │  │                │ │
│  │ Lifecycle:       │  │ Lifecycle:       │  │ Lifecycle:     │ │
│  │ 1. Read config   │  │ 1. Read config   │  │ 1. Read config │ │
│  │ 2. Bootstrap org │  │ 2. Bootstrap org │  │ 2. Bootstrap   │ │
│  │ 3. Create users  │  │ 3. Create users  │  │ 3. Create users│ │
│  │ 4. Create keys   │  │ 4. Create keys   │  │ 4. Create keys │ │
│  │ 5. Simulate users│  │ 5. Simulate users│  │ 5. Simulate    │ │
│  │ 6. Export metrics│  │ 6. Export metrics│  │ 6. Export      │ │
│  │ 7. Cleanup       │  │ 7. Cleanup       │  │ 7. Cleanup     │ │
│  │                  │  │                  │  │                │ │
│  │ Per-user sim:    │  │ Per-user sim:    │  │ Per-user sim:  │ │
│  │ - Get API key    │  │ - Get API key    │  │ - Get API key  │ │
│  │ - Generate Q     │  │ - Generate Q     │  │ - Generate Q   │ │
│  │ - Send request   │  │ - Send request   │  │ - Send request │ │
│  │ - Measure latency│  │ - Measure latency│  │ - Measure      │ │
│  │ - Think (wait)   │  │ - Think (wait)   │  │ - Think (wait) │ │
│  │ - Repeat         │  │ - Repeat         │  │ - Repeat       │ │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬───────┘ │
│           │                     │                      │         │
│           └─────────────────────┴──────────────────────┘         │
│                                 │                                │
│                    ┌────────────▼──────────────┐                 │
│                    │  Metrics Pushgateway      │                 │
│                    │  (Prometheus)             │                 │
│                    │                           │                 │
│                    │  Labels:                  │                 │
│                    │  - test_run_id            │                 │
│                    │  - org_id                 │                 │
│                    │  - user_id                │                 │
│                    │  - worker_pod             │                 │
│                    │  - phase                  │                 │
│                    └───────────────────────────┘                 │
│                                                                   │
└───────────────────────────────────────────────────────────────────┘
                                 │
                                 │ API Requests
                                 │
                    ┌────────────▼──────────────┐
                    │  Platform Under Test      │
                    │                           │
                    │  - API Router Service     │
                    │  - User/Org Service       │
                    │  - Budget Service         │
                    │  - Analytics Service      │
                    │  - vLLM Backends          │
                    └───────────────────────────┘
```

### Worker Pod Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    Worker Pod                           │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌────────────────────────────────────────────┐        │
│  │  Main Process (Go)                         │        │
│  │                                             │        │
│  │  1. Config Loader                           │        │
│  │     - Read ConfigMap                        │        │
│  │     - Parse YAML                            │        │
│  │     - Validate parameters                   │        │
│  │                                             │        │
│  │  2. Bootstrap Manager                       │        │
│  │     - Create organization (User/Org API)    │        │
│  │     - Create N users                        │        │
│  │     - Generate API keys for each user       │        │
│  │     - Store credentials in memory           │        │
│  │                                             │        │
│  │  3. User Simulator (goroutine per user)    │        │
│  │     ┌─────────────────────────────────┐    │        │
│  │     │ User Goroutine 1                │    │        │
│  │     │ - Question Generator (embedded) │    │        │
│  │     │ - HTTP Client (keep-alive)      │    │        │
│  │     │ - Metrics Collector             │    │        │
│  │     │ - Think Time Manager            │    │        │
│  │     │                                 │    │        │
│  │     │ Loop:                           │    │        │
│  │     │   1. Generate unique question   │    │        │
│  │     │   2. Build HTTP request         │    │        │
│  │     │   3. Send to API Router         │    │        │
│  │     │   4. Measure latency            │    │        │
│  │     │   5. Record tokens/cost         │    │        │
│  │     │   6. Wait (think time)          │    │        │
│  │     │   7. Repeat until session done  │    │        │
│  │     └─────────────────────────────────┘    │        │
│  │     ... (User 2, User 3, ... User N)       │        │
│  │                                             │        │
│  │  4. Metrics Exporter                        │        │
│  │     - Aggregate per-user metrics            │        │
│  │     - Push to Pushgateway every 10s         │        │
│  │     - Export final results on completion    │        │
│  │                                             │        │
│  │  5. Cleanup Manager                         │        │
│  │     - Wait for all users to complete        │        │
│  │     - Optionally delete org/users/keys      │        │
│  │     - Export final metrics/logs             │        │
│  │     - Exit gracefully                       │        │
│  └────────────────────────────────────────────┘        │
│                                                         │
│  ┌────────────────────────────────────────────┐        │
│  │  Embedded Question Generator (Go port)     │        │
│  │                                             │        │
│  │  - Historical questions                     │        │
│  │  - Mathematical questions                   │        │
│  │  - Geographical questions                   │        │
│  │  - Hypothetical questions                   │        │
│  │  - Technical questions                      │        │
│  │  - Mixed strategy                           │        │
│  │                                             │        │
│  │  Seeding: hash(org_id + user_id + time)    │        │
│  └────────────────────────────────────────────┘        │
│                                                         │
└─────────────────────────────────────────────────────────┘
```

### Load Pattern Management

```
Phase-based scaling:

Time:     0m    2m    5m    15m   45m   50m   55m
          │     │     │     │     │     │     │
Users:    │     │     │     │     │     │     │
1000─     │     │     │     ┌─────┴─────┐     │
          │     │     │    /│           │\    │
 500─     │     │    /│   / │           │ \   │
          │     │   / │  /  │           │  \  │
 100─     │     │  /  │ /   │           │   \ │
          │    /│ /   │/    │           │    \│
  10─  ───┴───/─┴/────┴─────┴───────────┴─────┴───

     Warm-up Ramp  Sustain  Cool-down

Phase 1: Warm-up (0-2m)
  - Start with 10 users
  - Validate platform responsiveness

Phase 2: Ramp-up (2m-15m)
  - Linearly add users to 1000
  - Monitor error rates
  - Stop if thresholds exceeded

Phase 3: Sustained Load (15m-45m)
  - Maintain 1000 active users
  - Collect steady-state metrics
  - Validate stability

Phase 4: Cool-down (45m-55m)
  - Gradually reduce to 100 users
  - Allow platform to recover
  - Validate no stuck resources
```

## Data Model

See `data-model.md` for detailed schema definitions including:
- LoadTestConfig structure
- WorkerState tracking
- Metrics schema
- Organization/User/APIKey models

## API Contracts

See `contracts/` directory for:
- `load-test-config.yaml` - ConfigMap schema
- `orchestrator-api.yaml` - Orchestrator REST API
- `worker-metrics.yaml` - Metrics format specification

## Implementation Tasks

See `tasks.md` for phased implementation plan.

## Security Considerations

- **API Key Management**: Worker pods receive API keys for their created users. Keys are stored in memory only, never logged or persisted.
- **Namespace Isolation**: Load tests run in dedicated namespace (`load-testing`) with NetworkPolicies restricting access.
- **Resource Limits**: Worker pods have strict CPU/memory limits to prevent resource exhaustion.
- **Cleanup**: Test data (orgs, users, keys) is deleted by default unless retention is explicitly configured.
- **Cost Controls**: Hard limits on max cost per test run to prevent runaway spending.
- **Access Control**: Only authorized users can create/execute load tests (RBAC enforced).

## Observability

### Metrics

**Worker-level metrics:**
- `loadtest_worker_status{test_run, worker_pod, phase}` - Worker state (bootstrapping, running, completed, failed)
- `loadtest_worker_users_active{test_run, worker_pod}` - Number of active user simulations
- `loadtest_worker_orgs_created{test_run, worker_pod}` - Organizations successfully created

**User-level metrics:**
- `loadtest_user_requests_total{test_run, org_id, user_id, status}` - Total requests (counter)
- `loadtest_user_latency_seconds{test_run, org_id, user_id, quantile}` - Request latency (summary)
- `loadtest_user_tokens_total{test_run, org_id, user_id, type}` - Tokens consumed (counter)
- `loadtest_user_cost_usd{test_run, org_id, user_id}` - Cost incurred (gauge)
- `loadtest_user_errors_total{test_run, org_id, user_id, error_type}` - Errors encountered (counter)

**Test-level metrics:**
- `loadtest_run_duration_seconds{test_run}` - Total test duration
- `loadtest_run_cost_total_usd{test_run}` - Total cost across all users
- `loadtest_run_requests_per_second{test_run}` - Aggregate throughput
- `loadtest_run_error_rate{test_run}` - Aggregate error rate

### Dashboards

**Grafana dashboards:**
1. **Load Test Overview** - Real-time test progress, active users, throughput, error rate
2. **Performance Analysis** - Latency percentiles, response times, bottleneck identification
3. **Cost Tracking** - Cost per org, per user, total spend, budget utilization
4. **Error Analysis** - Error breakdown by type, affected users, correlation with load phases

### Logging

**Structured logs (JSON):**
- Bootstrap events (org creation, user creation, key generation)
- Request/response details (with correlation IDs)
- Error details with stack traces
- Metrics export confirmation
- Cleanup operations

## Testing Strategy

### Unit Tests
- Question generator uniqueness validation
- Think time distribution accuracy
- Metrics calculation correctness
- Configuration parsing and validation

### Integration Tests
- Worker bootstrap against real User/Org Service
- API request flow against real API Router
- Metrics export to Pushgateway
- Cleanup operations

### End-to-End Tests
- Single-org load test (10 users, 5 minutes)
- Multi-org load test (3 orgs, 30 users, 10 minutes)
- Ramp-up pattern validation
- Cost limit enforcement
- Error threshold enforcement

## Future Enhancements

- **Dynamic question strategies**: Support for custom question templates
- **Recorded session replay**: Capture real user sessions and replay them
- **Geo-distributed load**: Run workers from multiple regions
- **Model comparison**: A/B testing different models under same load
- **Auto-tuning**: Automatically find optimal load levels
- **Chaos engineering**: Inject failures during load tests
- **Cost optimization**: Recommend configuration changes to reduce cost
