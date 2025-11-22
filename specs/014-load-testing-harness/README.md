# Spec 014: Load Testing Harness

**Status**: Draft
**Created**: 2025-01-27
**Owner**: Platform Team

## Overview

This specification defines a comprehensive load testing harness for the AI-AAS platform. The system simulates hundreds to thousands of realistic concurrent users to evaluate platform performance, measure real-world LLM metrics (TTFT, TPS), and validate scalability.

### Key Objectives

1. **Realistic User Simulation**: Simulate authentic user behavior with configurable think times, diverse question strategies, and multi-turn conversations
2. **LLM Performance Measurement**: Capture critical metrics like Time to First Token (TTFT), Tokens per Second (TPS), and cost efficiency
3. **Scalability Validation**: Test platform behavior from 10 to 1000+ concurrent users
4. **Multi-Tenancy Testing**: Validate organization isolation, budget enforcement, and resource fairness
5. **Unified Observability**: Combined dashboards showing both test cluster and inference cluster metrics for holistic performance analysis

## Key Features

### Self-Bootstrapping Workers
- Each worker pod automatically creates organizations, users, and API keys
- No manual setup required
- Configuration-driven via Kubernetes ConfigMaps

### Realistic Behavior
- **Variable Think Times**: Base time ± variance (e.g., 5s ± 2s) with multiple distributions
- **Unique Questions**: Seeded question generation ensuring >99% uniqueness
- **Multi-Turn Conversations**: Contextual follow-up questions maintaining conversation state
- **Diverse Strategies**: Mixed, technical, historical, geographical, hypothetical question types

### Dynamic Load Patterns
- **Ramp-Up**: Gradually increase from 10 to 1000+ users
- **Sustained Load**: Maintain constant user count for soak testing
- **Spike**: Instant load bursts for stress testing
- **Multi-Phase**: Sequential phases (warm-up, ramp, sustain, cool-down)

### Comprehensive Metrics

#### Standard Metrics
- Request latency (p50, p90, p95, p99)
- Throughput (requests/second)
- Error rates by type
- Token usage (prompt, completion, total)
- Cost tracking per user/org

#### LLM-Specific Metrics
- **Time to First Token (TTFT)**: Perceived latency
- **Tokens per Second (TPS)**: Generation speed
- **Backend Saturation**: Queue time, requests in flight
- **GPU Utilization**: Memory usage, efficiency
- **Cost Efficiency**: TPS per dollar, cost per 1M tokens

### Unified Observability

The load testing harness integrates with the platform's central Prometheus/Grafana stack, providing:

- **Combined Dashboards**: Single view of test cluster (load generators) and inference cluster (vLLM backends)
- **Correlated Metrics**: Link test requests to backend performance
- **Real-Time Monitoring**: Watch both sides simultaneously during tests
- **Historical Comparison**: Compare test runs over time

## Architecture

```
┌───────────────────────────────────────────────────────────────┐
│                  Central Observability                        │
│               (Shared Prometheus + Grafana)                   │
├───────────────────────────────────────────────────────────────┤
│                                                               │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │          Combined Load Test Dashboard                   │ │
│  ├─────────────────────────────────────────────────────────┤ │
│  │                                                         │ │
│  │  Test Cluster              │  Inference Cluster        │ │
│  │  ─────────────              │  ────────────────        │ │
│  │  • Active users             │  • vLLM pod status       │ │
│  │  • Requests/sec             │  • GPU utilization       │ │
│  │  • Think times              │  • TTFT distribution     │ │
│  │  • Questions generated      │  • TPS per backend       │ │
│  │  • Cost tracking            │  • Queue depths          │ │
│  │                             │  • Memory usage          │ │
│  │  Correlated View:                                      │ │
│  │  • Request latency (test) ←→ Processing time (vLLM)   │ │
│  │  • Error rate (test) ←→ Backend errors (vLLM)         │ │
│  │  • Token usage (test) ←→ Token throughput (vLLM)      │ │
│  └─────────────────────────────────────────────────────────┘ │
│                                                               │
└───┬───────────────────────────────────────────────────┬───────┘
    │                                                   │
    │ Metrics                                  Metrics  │
    │                                                   │
┌───▼─────────────────┐                    ┌───────────▼──────┐
│   Test Cluster      │                    │ Inference Cluster│
│   (Load Testing)    │                    │ (vLLM Backends)  │
├─────────────────────┤                    ├──────────────────┤
│                     │                    │                  │
│ ┌─────────────────┐ │   API Requests    │ ┌──────────────┐ │
│ │ Worker Pod 1    │─┼──────────────────▶│ │ vLLM Pod 1   │ │
│ │ - Org A (20u)   │ │                    │ │ (gpt-4o)     │ │
│ └─────────────────┘ │                    │ └──────────────┘ │
│                     │                    │                  │
│ ┌─────────────────┐ │                    │ ┌──────────────┐ │
│ │ Worker Pod 2    │─┼──────────────────▶│ │ vLLM Pod 2   │ │
│ │ - Org B (15u)   │ │                    │ │ (gpt-3.5)    │ │
│ └─────────────────┘ │                    │ └──────────────┘ │
│                     │                    │                  │
│ ┌─────────────────┐ │                    │                  │
│ │ Worker Pod N    │─┼──────────────────▶│      ...         │
│ │ - Org Z (18u)   │ │                    │                  │
│ └─────────────────┘ │                    │                  │
└─────────────────────┘                    └──────────────────┘
```

## Directory Structure

```
specs/014-load-testing-harness/
├── README.md                          # This file
├── spec.md                            # Complete specification
├── data-model.md                      # Data structures and schemas
├── tasks.md                           # Implementation task breakdown
├── PROGRESS.md                        # Implementation progress tracking
│
├── checklists/
│   └── requirements.md                # Requirements completion checklist
│
├── contracts/
│   ├── load-test-config.yaml          # Configuration schema with examples
│   ├── worker-metrics.yaml            # Standard metrics contract
│   └── llm-performance-metrics.yaml   # LLM-specific metrics (TTFT, TPS)
│
└── diagrams/
    ├── architecture.svg               # System architecture diagram
    ├── worker-lifecycle.svg           # Worker pod lifecycle
    └── observability-flow.svg         # Metrics flow diagram
```

## Quick Start

### Example: Single Organization Test

```yaml
# tests/load/configs/single-org-test.yaml
apiVersion: loadtest.ai-aas.dev/v1
kind: LoadTestScenario
metadata:
  name: single-org-50-users
  namespace: load-testing
spec:
  organizations:
    count: 1
    namingPattern: "loadtest-org-{index}"

  users:
    perOrg: { min: 50, max: 50, distribution: "uniform" }
    namingPattern: "user-{org}-{index}"

  apiKeys:
    perUser: 1

  userBehavior:
    questionsPerSession: { min: 10, max: 30, distribution: "gaussian" }
    thinkTimeSeconds: { base: 5, variance: 2, distribution: "uniform", min: 1, max: 10 }
    questionStrategies:
      - { name: "mixed", weight: 100 }
    sessionDuration: { min: "5m", max: "15m" }
    modelPreferences:
      - { model: "gpt-4o", weight: 100 }

  loadPattern:
    type: "steady"
    phases:
      - { name: "test", duration: "10m", targetActiveUsers: 50 }

  workers:
    replicas: 3
    usersPerWorker: 17
    resources:
      requests: { cpu: "500m", memory: "512Mi" }
      limits: { cpu: "1000m", memory: "1Gi" }

  targets:
    apiRouterUrl: "http://api-router-service.development.svc.cluster.local:8080"
    userOrgUrl: "http://user-org-service.development.svc.cluster.local:8081"

  metrics:
    pushgatewayUrl: "http://prometheus-pushgateway.observability.svc:9091"
    exportInterval: "10s"
    capture: ["latency_percentiles", "token_usage", "cost_per_user", "error_rate"]

  limits:
    maxCostUsd: 10.00
    maxErrorRate: 0.05
    maxDuration: "15m"

  cleanup:
    onCompletion: true
    retainMetrics: true
```

### Deploy and Run

```bash
# 1. Apply configuration
kubectl apply -f tests/load/configs/single-org-test.yaml

# 2. Create worker jobs (via orchestrator)
./bin/load-test-orchestrator start \
  --config single-org-50-users \
  --namespace load-testing

# 3. Monitor progress
./bin/load-test-orchestrator status \
  --config single-org-50-users \
  --follow

# 4. View in Grafana
# Navigate to: Dashboards → Load Testing → Combined View
# Select test run: single-org-50-users
```

## Key Metrics to Monitor

### During Test Execution

**Test Cluster (Load Generator) Metrics:**
- Active users: `loadtest_run_active_users`
- Requests/second: `loadtest_run_requests_per_second`
- Error rate: `loadtest_run_error_rate`
- Cost: `loadtest_run_cost_total_usd`

**Inference Cluster (vLLM) Metrics:**
- Time to First Token (p95): `loadtest_llm_ttft_summary_milliseconds{quantile="0.95"}`
- Tokens per Second (avg): `avg(loadtest_llm_tokens_per_second)`
- Backend saturation: `loadtest_llm_backend_saturation_ratio`
- GPU memory: `loadtest_llm_gpu_memory_utilization_percent`

**Correlated Metrics:**
- User latency vs backend processing time
- Test errors vs backend errors
- Question rate vs backend throughput

### Post-Test Analysis

1. **Performance Summary**
   - p50, p90, p95, p99 latencies
   - Average TPS by model
   - Total cost and cost per user

2. **Quality Metrics**
   - Error rate breakdown
   - Finish reason distribution
   - Question uniqueness percentage

3. **Resource Efficiency**
   - TPS per dollar
   - GPU utilization
   - Backend saturation patterns

4. **Comparison**
   - Performance vs baseline
   - Model comparison (TTFT, TPS, cost)
   - Trend analysis across test runs

## Configuration Highlights

### Think Time with Variance

The harness supports realistic think times with configurable variance:

```yaml
# Option 1: Base ± Variance (Recommended)
thinkTimeSeconds:
  base: 5           # Center value
  variance: 2       # ±2 seconds
  distribution: "uniform"
  min: 1            # Hard minimum
  max: 10           # Hard maximum

# Result: Think times range from 3-7s (base ± variance),
#         but never below 1s or above 10s

# Option 2: Explicit Range
thinkTimeSeconds:
  min: 3
  max: 7
  distribution: "exponential"

# Result: Think times range from 3-7s with exponential distribution
#         (most users quick, some slower)
```

### Question Strategies

```yaml
questionStrategies:
  - name: "mixed"        # Diverse questions
    weight: 50
  - name: "technical"    # Programming/tech questions
    weight: 30
  - name: "historical"   # Year-based questions
    weight: 20
```

### Load Patterns

```yaml
# Ramp-up pattern
loadPattern:
  type: "ramp-up"
  phases:
    - name: "warm-up"
      duration: "2m"
      targetActiveUsers: 10

    - name: "ramp"
      duration: "10m"
      targetActiveUsers: 1000
      rampStrategy: "linear"

    - name: "sustain"
      duration: "30m"
      targetActiveUsers: 1000

    - name: "cool-down"
      duration: "5m"
      targetActiveUsers: 100
```

## Observability Strategy

### Prometheus Metrics

All metrics are pushed to the **central** Prometheus instance (not separate per cluster):

```
Load Test Workers  ──┐
                     ├──▶ Prometheus Pushgateway ──▶ Central Prometheus
vLLM Backends     ──┘

Central Prometheus serves:
├── Test cluster metrics (loadtest_*)
├── Inference cluster metrics (vllm_*, gpu_*)
└── Platform metrics (api_router_*, budget_*, etc.)
```

### Grafana Dashboards

**Combined Load Test Dashboard** (Primary):
- Left side: Test cluster metrics
- Right side: Inference cluster metrics
- Bottom: Correlated views
- Filters: test_run_id, model, backend_id

**Individual Dashboards**:
1. Load Test Overview (test cluster focus)
2. LLM Performance Analysis (inference cluster focus)
3. Cost Tracking (both clusters)
4. Error Analysis (both clusters)

### Alert Rules

```yaml
# Alert if TTFT degrades
- alert: HighTimeToFirstToken
  expr: loadtest_llm_ttft_summary_milliseconds{quantile="0.95"} > 2000
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High TTFT detected ({{ $value }}ms)"

# Alert if backend saturated
- alert: BackendSaturated
  expr: loadtest_llm_backend_saturation_ratio > 0.9
  for: 5m
  labels:
    severity: critical
  annotations:
    summary: "Backend {{ $labels.backend_id }} saturated"

# Alert if test cost limit approaching
- alert: TestCostApproachingLimit
  expr: loadtest_limit_cost_utilization_ratio > 0.8
  for: 1m
  labels:
    severity: warning
  annotations:
    summary: "Test cost at {{ $value | humanizePercentage }} of limit"
```

## Implementation Phases

### Phase 1: Foundation (MVP)
- Single org support (10-50 users)
- Basic question generation
- Core metrics (latency, throughput, errors)
- ConfigMap-driven configuration
- Duration: 2-3 weeks

### Phase 2: Multi-Org & Scalability
- Multi-org support (2-3 orgs)
- Dynamic load patterns
- LLM performance metrics (TTFT, TPS)
- Grafana dashboards
- Cleanup automation
- Duration: 2-3 weeks

### Phase 3: Production-Ready
- 1000+ user scalability
- Test orchestrator
- Results export (S3/MinIO)
- Cost/error limit enforcement
- Production hardening
- Duration: 3-4 weeks

### Phase 4: Advanced Features (Future)
- Anomaly detection
- Performance baselines
- CI/CD integration
- A/B testing support
- Duration: 2-3 weeks

**Total Timeline**: 9-13 weeks

## Success Criteria

✅ **Functionality**
- Single org, 50 users completes with <5% error rate
- Multi-org test (3 orgs) with proper isolation
- Question uniqueness >99.9%
- Think time accuracy within ±10%
- Worker self-bootstrap >95% success

✅ **Performance**
- 1000 concurrent users with <10% error rate
- Worker memory <512Mi per pod
- Bootstrap time <60s
- Metrics available within 30s

✅ **LLM Metrics**
- TTFT p50 <500ms for gpt-4o
- Average TPS >40 for gpt-4o
- Backend saturation measurable
- Cost efficiency calculable

✅ **Observability**
- Combined dashboard functional
- Real-time metrics visible
- Historical comparison available
- Correlated metrics useful

## Related Documentation

- **Specification**: [`spec.md`](./spec.md) - Detailed requirements
- **Data Model**: [`data-model.md`](./data-model.md) - Schemas and structures
- **Tasks**: [`tasks.md`](./tasks.md) - Implementation breakdown
- **Contracts**: [`contracts/`](./contracts/) - API contracts
- **Progress**: [`PROGRESS.md`](./PROGRESS.md) - Implementation tracking

## Questions?

For questions or discussions about this specification:
1. Review the detailed [`spec.md`](./spec.md)
2. Check [`tasks.md`](./tasks.md) for implementation details
3. Consult [`data-model.md`](./data-model.md) for data structures
4. See example configurations in [`contracts/load-test-config.yaml`](./contracts/load-test-config.yaml)

---

**Status**: Ready for Review
**Next Step**: Stakeholder approval and Phase 1 implementation
