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

As a platform engineer, I can configure realistic user behavior patterns including variable think times, conversation context, diverse question strategies, and test types (random short, long, cached) to accurately simulate production workloads.

**Why this priority**: Ensures load tests reflect actual user behavior rather than artificial hammering, providing realistic performance data across different scenarios including KVCache utilization.

**Independent Test**: Can be tested by reviewing generated questions for uniqueness, measuring actual think times between requests, validating conversation context is maintained, and verifying test types produce expected cache behavior.

**Acceptance Scenarios**:

1. **Given** user behavior config with 5-30s think time, **When** simulated users run, **Then** actual think times follow the configured distribution (exponential/gaussian) and requests are properly spaced.
2. **Given** random short test type configuration, **When** users generate questions, **Then** each user produces highly unique, short questions that avoid KVCache hits, enabling measurement of cold-start performance.
3. **Given** cached test type configuration, **When** users generate questions, **Then** questions are designed to hit KVCache (repeated prompts or similar patterns) to model warm cache scenarios and measure cache performance.
4. **Given** long test type configuration, **When** users generate questions, **Then** questions are longer and more complex, using randomly selected documents from the library to test token generation limits and extended inference scenarios.
5. **Given** multi-turn conversations enabled, **When** users ask follow-up questions, **Then** conversation context is maintained and each question builds on previous answers.
6. **Given** continuous looping enabled, **When** a test session completes its configured sets, **Then** the test automatically restarts with a new random seed, ensuring test diversity across loop iterations.

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

### User Story 7 - Cache salt isolation testing (Priority: P3)

As a platform engineer, I can configure and test cache salt isolation to validate multi-tenant security, ensuring that KV cache is properly partitioned by organization or user to prevent timing side-channel attacks.

**Why this priority**: Validates critical security isolation mechanism that prevents cross-tenant cache leakage and timing-based inference attacks in multi-tenant LLM serving environments.

**Independent Test**: Can be tested by configuring different cache salt values (org_id, user_id) and verifying that cache hits only occur within the same salt boundary, with no cross-tenant cache reuse.

**Acceptance Scenarios**:

1. **Given** cache salt configured per organization, **When** two different organizations submit identical prompts, **Then** their requests are treated as distinct cache contexts with no cross-org cache hits, preventing timing side-channel attacks.
2. **Given** cache salt configured per user, **When** two different users in the same organization submit identical prompts, **Then** their requests are isolated with no cross-user cache hits, ensuring strict user-level isolation.
3. **Given** cache salt testing enabled, **When** I review metrics, **Then** I can verify cache hit rates are correctly partitioned by salt value (org_id or user_id) with no cross-boundary cache reuse.
4. **Given** identical prompts with different cache salts, **When** requests are made, **Then** response latencies are consistent (no cache benefit for different salts), validating proper cache partitioning.

---

### Edge Cases

- Worker pod crashes during test execution; orchestrator detects and optionally respawns or marks as failed.
- Platform services unavailable during bootstrap; workers retry with exponential backoff and timeout appropriately.
- Budget exhaustion mid-test; affected users stop gracefully and report final state without cascading failures.
- Question generator produces identical questions; seeding strategy ensures uniqueness based on org + user + timestamp (for random_short type), with seed recalculation on each loop iteration.
- KVCache misses when cached test type is configured; question generator uses consistent patterns to ensure cache hits.
- Model targeting mismatch (SLM receives complex query or medium LLM receives simple query); test configuration validation prevents mismatches.
- Network partitions between workers and platform; workers detect and report connectivity issues in metrics.
- Excessive load causing platform degradation; tests honor configured limits (max cost, max error rate) and stop automatically.
- Concurrent tests interfering; namespace isolation and unique test run IDs prevent cross-test contamination.
- Large-scale tests (1000+ users) exceeding K8s resource quotas; clear pre-flight checks and resource estimation.
- KVCache eviction during cached test type; test detects cache misses and reports cache performance metrics.
- Continuous looping causing resource exhaustion; tests respect configured maximum duration and loop count limits.
- Document library unavailable or empty; long test type falls back to generated long-form questions with clear error reporting.
- Document selection out of bounds (requesting document X+1 when only X documents exist); validation ensures random selection stays within valid range (1-X).
- Cache salt misconfiguration (missing or invalid salt value); tests validate salt presence and format, with clear error reporting for security-critical failures.
- Cross-tenant cache leakage despite cache salt; tests detect and report any cache hits across different salt boundaries as security violations.
- Cache salt collision (same salt value used by different tenants); tests validate salt uniqueness or document collisions in test configuration.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Provide a Go-based load test worker that simulates realistic user behavior including variable think times between requests.
- **FR-002**: Provide self-bootstrapping capability where workers read configuration from ConfigMap/Secret and create orgs, users, and API keys.
- **FR-003**: Provide unique question generation strategy using seeded randomness (org_id + user_id) to avoid cache hits.
- **FR-004**: Provide multi-turn conversation support where users maintain context across multiple questions.
- **FR-005**: Provide configurable load patterns (ramp-up, sustained, spike, cool-down) with phase-based user scaling.
- **FR-006**: Provide Kubernetes-native deployment using Jobs with configurable parallelism for horizontal scaling.
- **FR-007**: Provide metrics export to Prometheus Pushgateway with standardized labels (test_run, org_id, user_id, worker_pod, test_type, model_target).
- **FR-008**: Provide real-time observability via Grafana dashboards showing latency, throughput, errors, tokens, and cost.
- **FR-009**: Provide test orchestrator that manages worker lifecycle, aggregates results, and enforces limits (max cost, max errors).
- **FR-010**: Provide configurable cleanup strategy to optionally delete test data or retain for analysis.
- **FR-011**: Provide per-user metrics including questions asked, tokens consumed, cost incurred, errors encountered.
- **FR-012**: Provide test run correlation IDs and detailed logging for debugging failures.
- **FR-013**: Provide pre-flight validation to check platform availability and resource capacity before launching tests.
- **FR-014**: Provide support for testing different API endpoints (chat completions, embeddings, org management).
- **FR-015**: Provide worker pod resource limits and requests appropriate for simulating 10-50 users per pod.
- **FR-016**: Provide test type configuration (random_short, long, cached) to model different KVCache scenarios and system performance characteristics.
- **FR-017**: Provide model targeting configuration to route short queries to SLMs (Small Language Models) and complex queries to medium-sized LLMs, enabling realistic workload modeling.
- **FR-018**: Provide KVCache-aware question generation that can either avoid cache hits (random_short) or intentionally hit cache (cached) based on test type.
- **FR-019**: Provide model selection logic that matches test complexity to appropriate model size (SLM for short queries, medium LLM for complex queries).
- **FR-020**: Provide continuous looping capability where all tests automatically restart after completing their configured sets, with random seeds recalculated on each loop iteration to ensure test diversity.
- **FR-021**: Provide document library integration for long test types, where documents are stored in an object store (S3/MinIO), numbered sequentially (1-X), and randomly selected during test execution to generate realistic long-form queries.
- **FR-022**: Provide cache salt configuration support to test multi-tenant security isolation, where `cache_salt` parameter is passed via `extra_body` in API requests to partition KV cache by organization or user.
- **FR-023**: Provide cache salt isolation validation to verify that cache hits only occur within the same salt boundary, with metrics tracking cache performance per salt value (org_id or user_id).

### Non-Functional Requirements

- **NFR-001**: Worker pods must be lightweight (<512Mi memory, <500m CPU per pod).
- **NFR-002**: Bootstrap phase must complete within 60 seconds per worker pod.
- **NFR-003**: Metrics export must not introduce >100ms overhead per request.
- **NFR-004**: System must support scaling to 1000+ concurrent simulated users across 20-50 worker pods.
- **NFR-005**: Question generation must produce unique questions with <0.1% collision rate across 100k questions.
- **NFR-006**: Configuration changes must not require rebuilding container images (use ConfigMaps).
- **NFR-007**: Test results must be exportable to S3/MinIO for long-term retention and analysis.
- **NFR-008**: Worker failures must not cause cascading failures in other workers or platform services.
- **NFR-009**: Continuous looping must support infinite loops (until stopped) or configurable maximum loop count/duration, with seed recalculation overhead <10ms per loop iteration.
- **NFR-010**: Document library access must have <500ms latency for document retrieval from object store, with support for at least 10,000 documents (numbered 1-10000).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Single-org load test (50 users) completes successfully with <5% error rate and reports metrics within 10 minutes.
- **SC-002**: Multi-org load test (3 orgs, 60 total users) completes with proper isolation and independent budget enforcement.
- **SC-003**: Question uniqueness: 99.9%+ of questions are unique across all users in a 1000-user test run (for random_short test type).
- **SC-004**: Think time accuracy: Actual think times are within ±10% of configured distribution parameters.
- **SC-005**: Worker self-bootstrap: 95%+ of workers successfully initialize without manual intervention.
- **SC-006**: Metrics availability: All defined metrics (latency, throughput, tokens, cost, errors, cache hits/misses) are queryable in Prometheus within 30 seconds of test start.
- **SC-007**: Scalability: System can support 1000 concurrent simulated users with stable performance and <10% error rate.
- **SC-008**: Observability: Grafana dashboards provide real-time visibility into test progress and platform health during load tests, including test type and model target breakdowns.
- **SC-009**: Cost control: Tests automatically stop when configured cost limit is reached with <5% overspend.
- **SC-010**: Cleanup: Test data is cleanly removed (or retained based on config) within 5 minutes of test completion.
- **SC-011**: Cache hit accuracy: Cached test type achieves >80% KVCache hit rate when configured with repeated prompts.
- **SC-012**: Cache miss accuracy: Random short test type achieves <5% KVCache hit rate, ensuring cold-start performance measurement.
- **SC-013**: Model targeting: Short queries are routed to SLMs and complex queries to medium LLMs with >95% accuracy based on query complexity.
- **SC-014**: Test type distribution: Configured test type distribution (e.g., 40% random_short, 30% long, 30% cached) is followed within ±5% variance.
- **SC-015**: Continuous looping: Tests automatically restart after completing configured sets, with random seeds recalculated on each loop iteration, maintaining test diversity across loops.
- **SC-016**: Document library integration: Long test type successfully retrieves and uses randomly selected documents from object store (S3/MinIO) with <1% retrieval failure rate.
- **SC-017**: Cache salt isolation: When cache salt is configured, identical prompts with different salt values (different orgs/users) show no cross-boundary cache hits, with cache hit rates correctly partitioned by salt value.
- **SC-018**: Cache salt security validation: Response latencies for identical prompts with different cache salts are consistent (no cache benefit), confirming proper cache partitioning and preventing timing side-channel attacks.

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
│  │    (with loops)  │  │    (with loops)  │  │    (with loops)│ │
│  │ 6. Export metrics│  │ 6. Export metrics│  │ 6. Export      │ │
│  │ 7. Cleanup       │  │ 7. Cleanup       │  │ 7. Cleanup     │ │
│  │                  │  │                  │  │                │ │
│  │ Per-user sim:    │  │ Per-user sim:    │  │ Per-user sim:  │ │
│  │ - Get API key    │  │ - Get API key    │  │ - Get API key  │ │
│  │ - Generate Q     │  │ - Generate Q     │  │ - Generate Q   │ │
│  │   (select doc    │  │   (select doc    │  │   (select doc  │ │
│  │    for long)     │  │    for long)     │  │    for long)   │ │
│  │ - Send request   │  │ - Send request   │  │ - Send request │ │
│  │ - Measure latency│  │ - Measure latency│  │ - Measure      │ │
│  │ - Think (wait)   │  │ - Think (wait)   │  │ - Think (wait) │ │
│  │ - Loop (repeat)  │  │ - Loop (repeat)  │  │ - Loop (repeat)│ │
│  │   with new seed  │  │   with new seed  │  │   with new seed│ │
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
│                    ┌────────────▼──────────────┐                 │
│                    │  Object Store (S3/MinIO) │                 │
│                    │                           │                 │
│                    │  - Document Library      │                 │
│                    │  - Numbered Documents     │                 │
│                    │    (1, 2, ..., X)        │                 │
│                    │  - Random Selection      │                 │
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
│  │     │   * Test Type Selector          │    │        │
│  │     │     (random_short/long/cached)  │    │        │
│  │     │   * Model Target Selector       │    │        │
│  │     │     (SLM vs medium LLM)        │    │        │
│  │     │ - HTTP Client (keep-alive)      │    │        │
│  │     │ - Metrics Collector             │    │        │
│  │     │ - Think Time Manager            │    │        │
│  │     │ - Cache Hit Tracker             │    │        │
│  │     │                                 │    │        │
│  │     │ Outer Loop (Continuous):        │    │        │
│  │     │   - Recalculate random seed     │    │        │
│  │     │   - Reset session state         │    │        │
│  │     │                                 │    │        │
│  │     │ Inner Loop (Session):           │    │        │
│  │     │   1. Select test type           │    │        │
│  │     │   2. Select target model        │    │        │
│  │     │   3. Generate question          │    │        │
│  │     │      (unique/cached/long)       │    │        │
│  │     │      * For long: select random  │    │        │
│  │     │        doc from library (1-X)   │    │        │
│  │     │   4. Build HTTP request         │    │        │
│  │     │      * Include cache_salt in     │    │        │
│  │     │        extra_body (org_id/      │    │        │
│  │     │        user_id)                 │    │        │
│  │     │   5. Send to API Router         │    │        │
│  │     │   6. Measure latency            │    │        │
│  │     │   7. Detect cache hit/miss      │    │        │
│  │     │   8. Record tokens/cost         │    │        │
│  │     │   9. Wait (think time)          │    │        │
│  │     │  10. Repeat until session done  │    │        │
│  │     │  11. Restart outer loop         │    │        │
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
│  │  (Recalculated on each loop iteration)     │        │
│  └────────────────────────────────────────────┘        │
│                                                         │
│  ┌────────────────────────────────────────────┐        │
│  │  Document Library Client                   │        │
│  │                                             │        │
│  │  - Object Store (S3/MinIO)                 │        │
│  │  - Document enumeration (1, 2, ..., X)     │        │
│  │  - Random document selection                │        │
│  │  - Document content retrieval              │        │
│  │  - Used for long test type                 │        │
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
- **Cache Salt Security**: Cache salt configuration ensures multi-tenant isolation, preventing timing side-channel attacks. Tests validate that cache hits only occur within the same salt boundary, with violations reported as security events.

## Observability

### Metrics

**Worker-level metrics:**
- `loadtest_worker_status{test_run, worker_pod, phase}` - Worker state (bootstrapping, running, completed, failed)
- `loadtest_worker_users_active{test_run, worker_pod}` - Number of active user simulations
- `loadtest_worker_orgs_created{test_run, worker_pod}` - Organizations successfully created

**User-level metrics:**
- `loadtest_user_requests_total{test_run, org_id, user_id, status, test_type, model_target, cache_salt}` - Total requests (counter)
- `loadtest_user_latency_seconds{test_run, org_id, user_id, test_type, model_target, cache_salt, quantile}` - Request latency (summary)
- `loadtest_user_tokens_total{test_run, org_id, user_id, type, test_type, model_target, cache_salt}` - Tokens consumed (counter)
- `loadtest_user_cost_usd{test_run, org_id, user_id, test_type, model_target, cache_salt}` - Cost incurred (gauge)
- `loadtest_user_errors_total{test_run, org_id, user_id, error_type, test_type, model_target, cache_salt}` - Errors encountered (counter)
- `loadtest_user_cache_hits_total{test_run, org_id, user_id, test_type, cache_salt}` - KVCache hits (counter)
- `loadtest_user_cache_misses_total{test_run, org_id, user_id, test_type, cache_salt}` - KVCache misses (counter)
- `loadtest_user_cache_hit_ratio{test_run, org_id, user_id, test_type, cache_salt}` - Cache hit ratio (gauge, 0.0-1.0)
- `loadtest_user_cache_salt_violations_total{test_run, org_id, user_id, cache_salt}` - Cache salt isolation violations (counter, security metric)

**Test-level metrics:**
- `loadtest_run_duration_seconds{test_run}` - Total test duration
- `loadtest_run_cost_total_usd{test_run}` - Total cost across all users
- `loadtest_run_requests_per_second{test_run}` - Aggregate throughput
- `loadtest_run_error_rate{test_run}` - Aggregate error rate

### Dashboards

**Grafana dashboards:**
1. **Load Test Overview** - Real-time test progress, active users, throughput, error rate
2. **Performance Analysis** - Latency percentiles, response times, bottleneck identification, breakdown by test type and model target
3. **Cost Tracking** - Cost per org, per user, total spend, budget utilization, cost by test type and model
4. **Error Analysis** - Error breakdown by type, affected users, correlation with load phases
5. **Cache Performance** - KVCache hit/miss rates by test type, cache utilization, cache performance impact on latency, breakdown by cache salt
6. **Model Performance Comparison** - SLM vs medium LLM performance metrics, latency comparison, throughput comparison
7. **Cache Salt Isolation** - Cache salt isolation validation, cross-salt cache hit violations, isolation effectiveness by salt strategy (org_id vs user_id), security violation alerts

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

## Test Type Configuration

### Test Types

The load testing harness supports three test types to model different scenarios:

#### 1. Random Short (`random_short`)
- **Purpose**: Measure cold-start performance and avoid KVCache hits
- **Characteristics**:
  - Highly unique, short questions (<50 tokens)
  - Each question is generated with maximum randomness
  - Seeding ensures uniqueness: `hash(org_id + user_id + timestamp + random)`
  - Designed to bypass KVCache to measure uncached performance
- **Use Cases**:
  - Baseline performance measurement
  - Cold-start latency analysis
  - Cache miss scenario modeling
- **Expected Cache Hit Rate**: <5%

#### 2. Long (`long`)
- **Purpose**: Test extended inference scenarios and token generation limits
- **Characteristics**:
  - Complex, multi-part questions (>200 tokens)
  - Extended prompts requiring detailed responses
  - Uses randomly selected documents from a document library stored in an object store (S3/MinIO)
  - Documents are numbered sequentially (1, 2, ..., X) in a single folder
  - Random document selection: generates random number between 1-X to select document
  - Tests token generation limits and extended inference with realistic document content
- **Use Cases**:
  - Token generation capacity testing
  - Extended inference performance
  - Memory and resource utilization under long queries
  - Realistic document-based query scenarios
- **Expected Cache Hit Rate**: Variable (depends on prompt similarity and document reuse)

#### 3. Cached (`cached`)
- **Purpose**: Model warm cache scenarios and measure KVCache performance
- **Characteristics**:
  - Repeated or similar prompts designed to hit KVCache
  - Consistent question patterns across users
  - Measures warm cache performance and cache utilization
- **Use Cases**:
  - Cache performance measurement
  - Warm cache latency analysis
  - Cache utilization optimization
- **Expected Cache Hit Rate**: >80%

### Cache Salt Isolation

The harness supports cache salt configuration to test and validate multi-tenant security isolation in LLM serving frameworks.

#### Purpose and Security

The `cache_salt` parameter is a critical security mechanism used in LLM serving frameworks (such as vLLM OpenAI-compatible API) to manage security and isolation in multi-tenant environments:

1. **Security Mitigation**: The primary role of `cache_salt` is to act as a defense mechanism against **timing side-channel attacks**. These attacks exploit the performance benefits of shared KV (Key-Value) cache reuse across different users to infer sensitive prompt content based on variations in response latency (Time-to-First-Token).

2. **Cache Partitioning**: The process, known as **Cache Salting**, involves **injecting a unique, user-specific or tenant-specific value (the "salt")** into the internal hashing calculation used for KV cache blocks. This hash calculation determines if cached context can be reused: `BlockHash = Hash(TokenIDs + ParentHash + Salt)`.

3. **Isolation Effect**: By including the salt in the hash, cache reuse is strictly **restricted to requests providing the same salt**. This ensures that even if two different users submit identical prompts, the system treats their contexts as distinct sequences if their salts differ, effectively partitioning the cache and preventing cross-tenant leakage.

4. **Implementation**: The `cache_salt` parameter is passed as an extra field in the API request, within the **`extra_body`** parameter of the client, to transmit the required isolation identity to the serving engine. The specific salt value chosen (e.g., Organization ID or User ID) dictates the level of desired sharing versus strict isolation within the platform.

#### Cache Salt Strategies

- **Organization-level (`org_id`)**: All users within the same organization share cache, providing performance benefits while maintaining org-level isolation.
- **User-level (`user_id`)**: Strict user-level isolation, where each user has their own cache partition, providing maximum security but reduced cache efficiency.
- **Custom**: Allows specifying custom salt values for specialized testing scenarios.

#### Testing Scenarios

- **Isolation Validation**: Verify that identical prompts with different cache salts show no cross-boundary cache hits.
- **Security Testing**: Confirm that response latencies are consistent for identical prompts with different salts (no cache benefit), preventing timing side-channel attacks.
- **Performance Impact**: Measure the performance trade-off between isolation level (org vs user) and cache efficiency.

### Model Targeting

The harness supports intelligent model targeting to route queries to appropriate model sizes:

#### Small Language Models (SLMs)
- **Target Queries**: Short, simple queries
- **Characteristics**:
  - Fast response times
  - Low token usage
  - Suitable for simple Q&A, classification, short completions
- **Example Models**: phi-3-mini, gemma-2b, qwen2-0.5b
- **Query Characteristics**:
  - <100 tokens input
  - Single-turn conversations
  - Factual questions
  - Simple classification tasks

#### Medium Language Models
- **Target Queries**: Complex, multi-step queries
- **Characteristics**:
  - Higher token usage
  - Reasoning capabilities
  - Suitable for complex tasks, multi-step reasoning, extended conversations
- **Example Models**: llama-3-8b, mistral-7b, qwen2-7b
- **Query Characteristics**:
  - >100 tokens input
  - Multi-turn conversations
  - Reasoning tasks
  - Complex problem-solving

### Configuration Example

```yaml
test_types:
  distribution:
    random_short: 0.4  # 40% of requests
    long: 0.3          # 30% of requests
    cached: 0.3        # 30% of requests

model_targeting:
  enabled: true
  slm_models:
    - "phi-3-mini"
    - "gemma-2b"
  medium_llm_models:
    - "llama-3-8b"
    - "mistral-7b"
  
  routing_rules:
    - query_complexity: "simple"  # <100 tokens, single-turn
      target: "slm"
    - query_complexity: "complex"  # >100 tokens, multi-turn
      target: "medium_llm"

cache_strategy:
  random_short:
    avoid_cache: true
    uniqueness_seed: "org_id + user_id + timestamp + random"
  cached:
    ensure_cache_hit: true
    repeat_patterns: true
    cache_key_consistency: true
  long:
    cache_behavior: "natural"  # No specific cache targeting

# Continuous looping configuration
looping:
  enabled: true  # All tests automatically loop
  max_loops: null  # null = infinite, or set max number of loops
  max_duration: "24h"  # Maximum total test duration across all loops
  seed_recalculation: true  # Recalculate random seed on each loop iteration

# Document library for long test type
document_library:
  enabled: true
  storage:
    type: "s3"  # or "minio"
    endpoint: "s3.amazonaws.com"  # or MinIO endpoint
    bucket: "load-test-documents"
    folder: "documents"  # Single folder containing numbered documents
    access_key: "${S3_ACCESS_KEY}"  # From Secret
    secret_key: "${S3_SECRET_KEY}"  # From Secret
  document_count: 100  # Total number of documents (1-X)
  selection_strategy: "random"  # Random selection between 1 and document_count

# Cache salt configuration for multi-tenant security isolation
cache_salt:
  enabled: true
  strategy: "org_id"  # Options: "org_id", "user_id", "custom"
  # When strategy is "org_id": uses organization ID as salt
  # When strategy is "user_id": uses user ID as salt (stricter isolation)
  # When strategy is "custom": uses custom_salt_value
  custom_salt_value: null  # Only used when strategy is "custom"
  validation:
    enabled: true  # Validate cache isolation (no cross-salt cache hits)
    report_violations: true  # Report security violations if cross-salt cache hits detected
  # Cache salt is passed via extra_body parameter in API requests:
  # extra_body: { "cache_salt": "<org_id or user_id or custom>" }
```

## Future Enhancements

- **Dynamic question strategies**: Support for custom question templates
- **Recorded session replay**: Capture real user sessions and replay them
- **Geo-distributed load**: Run workers from multiple regions
- **Model comparison**: A/B testing different models under same load
- **Auto-tuning**: Automatically find optimal load levels
- **Chaos engineering**: Inject failures during load tests
- **Cost optimization**: Recommend configuration changes to reduce cost
- **Advanced cache modeling**: Simulate cache eviction, cache warming strategies
- **Model-specific test profiles**: Predefined test profiles optimized for specific model types
- **Document library management**: UI/CLI for managing document library (upload, enumerate, validate)
- **Adaptive looping**: Dynamic loop duration based on test performance metrics
- **Document caching**: Local caching of frequently accessed documents to reduce object store latency
