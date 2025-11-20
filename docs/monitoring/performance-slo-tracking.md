# Performance Monitoring and SLO Tracking for vLLM Deployments

**Feature**: `010-vllm-deployment` (User Story 3 - Safe operations)
**Last Updated**: 2025-01-27

## Overview

This document defines Service Level Objectives (SLOs) for vLLM model deployments and provides guidance on tracking, monitoring, and maintaining performance targets.

## Service Level Objectives (SLOs)

### Production Environment

| Metric | SLO Target | Measurement Window | Severity if Breached |
|--------|------------|-------------------|---------------------|
| **Availability** | 99.9% | 30 days | Critical |
| **P95 Latency** | < 3 seconds | 5 minutes | High |
| **P99 Latency** | < 6 seconds | 5 minutes | Critical |
| **Error Rate** | < 0.1% | 5 minutes | Critical |
| **Request Success Rate** | > 99.9% | 5 minutes | Critical |
| **Time to First Token (TTFT)** | < 500ms | 5 minutes | Medium |
| **Throughput** | > 10 req/sec per pod | 5 minutes | Medium |

### Staging Environment

| Metric | SLO Target | Measurement Window | Severity if Breached |
|--------|------------|-------------------|---------------------|
| **Availability** | 99.0% | 7 days | High |
| **P95 Latency** | < 3 seconds | 5 minutes | Medium |
| **P99 Latency** | < 6 seconds | 5 minutes | High |
| **Error Rate** | < 1% | 5 minutes | High |
| **Request Success Rate** | > 99% | 5 minutes | High |

### Development Environment

| Metric | SLO Target | Measurement Window | Severity if Breached |
|--------|------------|-------------------|---------------------|
| **Availability** | 95.0% | 7 days | Low |
| **P95 Latency** | < 5 seconds | 5 minutes | Low |
| **Error Rate** | < 5% | 10 minutes | Low |

## Key Performance Indicators (KPIs)

### 1. Availability

**Definition**: Percentage of time the service is operational and accepting requests.

**Calculation**:
```promql
# Availability = (Total Time - Downtime) / Total Time
(
  1 - (
    sum(rate(vllm_errors_total{environment="production"}[30d]))
    /
    sum(rate(vllm_requests_total{environment="production"}[30d]))
  )
) * 100
```

**SLO Targets**:
- Production: 99.9% (43.8 minutes downtime per month)
- Staging: 99.0% (7.2 hours downtime per month)
- Development: 95.0% (36 hours downtime per month)

**Error Budget**:
```
Production monthly error budget: 43.8 minutes
Per day: ~87 seconds
Per hour: ~3.6 seconds
```

### 2. Request Latency

**Definition**: Time from request received to response sent.

**P95 Latency** (95th percentile):
```promql
histogram_quantile(0.95,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le, model)
)
```

**P99 Latency** (99th percentile):
```promql
histogram_quantile(0.99,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le, model)
)
```

**SLO Targets**:
- P95 < 3s: 95% of requests complete within 3 seconds
- P99 < 6s: 99% of requests complete within 6 seconds

**Latency Breakdown**:
- Network latency: ~10-50ms
- Model inference: 1-5s (varies by model size and prompt length)
- Overhead (serialization, deserialization): ~10-20ms

### 3. Error Rate

**Definition**: Percentage of requests that result in errors (4xx, 5xx status codes).

**Calculation**:
```promql
sum(rate(vllm_errors_total{environment="production"}[5m]))
/
sum(rate(vllm_requests_total{environment="production"}[5m]))
```

**SLO Target**: < 0.1% (1 error per 1000 requests)

**Error Categories**:
- 4xx Client Errors: Invalid requests, authentication failures
- 5xx Server Errors: Model crashes, OOM, timeouts

### 4. Throughput

**Definition**: Number of requests processed per second per pod.

**Calculation**:
```promql
sum(rate(vllm_requests_total{environment="production"}[5m])) by (pod)
```

**SLO Target**: > 10 requests/second per pod (varies by model size)

**Expected Throughput by Model Size**:
- 7B models: 20-30 req/sec per GPU
- 13B models: 10-15 req/sec per GPU
- 70B models: 2-5 req/sec per GPU

### 5. Time to First Token (TTFT)

**Definition**: Time from request start to first token generation.

**Calculation**:
```promql
histogram_quantile(0.95,
  sum(rate(vllm_time_to_first_token_seconds_bucket{environment="production"}[5m])) by (le, model)
)
```

**SLO Target**: P95 < 500ms

**Impact**: Affects perceived responsiveness for streaming responses.

## Monitoring Dashboards

### Grafana Dashboard: vLLM Deployment Status

**Location**: `docs/dashboards/vllm-deployment-dashboard.json`

**Key Panels**:
1. **Pod Availability Gauge**: Real-time pod readiness percentage
2. **Pod Status Timeline**: Running/Pending/Failed pods over time
3. **Request Latency**: P50/P95/P99 latency trends
4. **Request Rate & Error Rate**: Traffic volume and error percentage
5. **GPU Utilization**: GPU usage percentage per pod
6. **GPU Memory Usage**: VRAM consumption vs. total
7. **CPU Usage**: CPU utilization per pod
8. **Memory Usage**: System memory consumption
9. **Pod Details Table**: Pod status, restarts, and node placement

**Access**: https://grafana.example.com/d/vllm-deployment-status

### Custom SLO Dashboard

Create a dedicated SLO tracking dashboard:

```json
{
  "title": "vLLM SLO Tracking",
  "panels": [
    {
      "title": "Availability (30-day)",
      "targets": [
        {
          "expr": "(1 - (sum(rate(vllm_errors_total{environment=\"production\"}[30d])) / sum(rate(vllm_requests_total{environment=\"production\"}[30d])))) * 100"
        }
      ],
      "thresholds": [
        { "value": 99.9, "color": "green" },
        { "value": 99.0, "color": "yellow" },
        { "value": 0, "color": "red" }
      ]
    },
    {
      "title": "Error Budget Remaining",
      "targets": [
        {
          "expr": "43.8 - (sum(rate(vllm_errors_total{environment=\"production\"}[30d])) * 60 * 60 * 24 * 30)"
        }
      ]
    },
    {
      "title": "P95 Latency vs SLO",
      "targets": [
        {
          "expr": "histogram_quantile(0.95, sum(rate(vllm_request_duration_seconds_bucket{environment=\"production\"}[5m])) by (le))"
        }
      ],
      "thresholds": [
        { "value": 3, "color": "red", "line": true }
      ]
    }
  ]
}
```

## Prometheus Queries

### Availability Queries

**Uptime percentage (last 30 days)**:
```promql
(
  1 - (
    sum(rate(vllm_errors_total{environment="production"}[30d]))
    /
    sum(rate(vllm_requests_total{environment="production"}[30d]))
  )
) * 100
```

**Downtime minutes (last 30 days)**:
```promql
(
  sum(rate(vllm_errors_total{environment="production"}[30d]))
  /
  sum(rate(vllm_requests_total{environment="production"}[30d]))
) * 60 * 60 * 24 * 30
```

**Error budget consumed**:
```promql
# Total budget: 43.8 minutes per month
# Consumed = downtime minutes
(
  sum(rate(vllm_errors_total{environment="production"}[30d]))
  /
  sum(rate(vllm_requests_total{environment="production"}[30d]))
) * 60 * 60 * 24 * 30
```

**Error budget remaining**:
```promql
43.8 - (
  (
    sum(rate(vllm_errors_total{environment="production"}[30d]))
    /
    sum(rate(vllm_requests_total{environment="production"}[30d]))
  ) * 60 * 60 * 24 * 30
)
```

### Latency Queries

**P50/P95/P99 latency**:
```promql
# P50
histogram_quantile(0.50,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le, model)
)

# P95
histogram_quantile(0.95,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le, model)
)

# P99
histogram_quantile(0.99,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le, model)
)
```

**Latency trend (compare to 1 hour ago)**:
```promql
histogram_quantile(0.95,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le)
)
/
histogram_quantile(0.95,
  sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m] offset 1h)) by (le)
)
```

**SLO compliance (% of time P95 < 3s)**:
```promql
(
  count_over_time(
    (histogram_quantile(0.95,
      sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le)
    ) < 3)[7d:]
  )
  /
  count_over_time(
    histogram_quantile(0.95,
      sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le)
    )[7d:]
  )
) * 100
```

### Throughput Queries

**Requests per second (per model)**:
```promql
sum(rate(vllm_requests_total{environment="production"}[5m])) by (model)
```

**Requests per second (per pod)**:
```promql
sum(rate(vllm_requests_total{environment="production"}[5m])) by (pod)
```

**Throughput trend (7-day average)**:
```promql
avg_over_time(
  sum(rate(vllm_requests_total{environment="production"}[5m]))[7d:]
)
```

### Error Rate Queries

**Error rate percentage**:
```promql
(
  sum(rate(vllm_errors_total{environment="production"}[5m]))
  /
  sum(rate(vllm_requests_total{environment="production"}[5m]))
) * 100
```

**Error breakdown by type**:
```promql
sum(rate(vllm_errors_total{environment="production"}[5m])) by (error_type)
```

**Errors per minute**:
```promql
sum(rate(vllm_errors_total{environment="production"}[5m])) * 60
```

## SLO Burn Rate

**Concept**: How quickly you're consuming your error budget.

### Burn Rate Calculation

**1x burn rate** = Using error budget at exactly the target rate
**2x burn rate** = Using error budget twice as fast (budget exhausted in 15 days)
**10x burn rate** = Using error budget 10x faster (budget exhausted in 3 days)

**Formula**:
```promql
# Current error rate / SLO error rate
(
  sum(rate(vllm_errors_total{environment="production"}[1h]))
  /
  sum(rate(vllm_requests_total{environment="production"}[1h]))
)
/
0.001  # 0.1% SLO target
```

### Multi-Window Burn Rate Alerts

**Fast burn (1 hour window, 14.4x burn rate)**:
```promql
(
  sum(rate(vllm_errors_total{environment="production"}[1h]))
  /
  sum(rate(vllm_requests_total{environment="production"}[1h]))
) > (14.4 * 0.001)
```
**Impact**: Exhausts 2% of monthly budget in 1 hour

**Slow burn (6 hour window, 6x burn rate)**:
```promql
(
  sum(rate(vllm_errors_total{environment="production"}[6h]))
  /
  sum(rate(vllm_requests_total{environment="production"}[6h]))
) > (6 * 0.001)
```
**Impact**: Exhausts 5% of monthly budget in 6 hours

## Performance Baselines

### Establish Baselines

**Step 1: Collect baseline data (7 days minimum)**:
```bash
# Export P95 latency over 7 days
promtool query range \
  --start=$(date -d '7 days ago' +%s) \
  --end=$(date +%s) \
  --step=5m \
  'histogram_quantile(0.95, sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le))' \
  > baseline_p95_latency.json
```

**Step 2: Calculate baseline statistics**:
```python
import json
import statistics

with open('baseline_p95_latency.json') as f:
    data = json.load(f)

values = [float(v[1]) for v in data['data']['result'][0]['values']]

print(f"Mean: {statistics.mean(values):.3f}s")
print(f"Median: {statistics.median(values):.3f}s")
print(f"Std Dev: {statistics.stdev(values):.3f}s")
print(f"P95: {statistics.quantiles(values, n=20)[18]:.3f}s")
```

**Step 3: Set SLO based on baseline + margin**:
```
If baseline P95 = 2.1s, set SLO = 3.0s (42% margin)
If baseline error rate = 0.05%, set SLO = 0.1% (2x margin)
```

### Performance Regression Detection

**Compare current performance to baseline**:
```promql
# Latency regression (current vs baseline)
(
  histogram_quantile(0.95,
    sum(rate(vllm_request_duration_seconds_bucket{environment="production"}[5m])) by (le)
  )
  /
  2.1  # baseline P95
) > 1.2  # 20% regression threshold
```

## Load Testing and Capacity Planning

### Load Test Scenarios

**Scenario 1: Sustained Load Test**:
```bash
# Use Locust or K6 for load testing
k6 run --vus 100 --duration 30m load-test-vllm.js

# Monitor during test:
watch -n 5 "kubectl top pods -n system | grep production"
```

**Scenario 2: Spike Test**:
```bash
# Gradual ramp to 500 VUs, hold, then drop
k6 run --stage 5m:100,10m:500,5m:100,5m:0 spike-test-vllm.js
```

**Scenario 3: Soak Test**:
```bash
# 24-hour sustained load
k6 run --vus 50 --duration 24h soak-test-vllm.js
```

### Capacity Planning Metrics

**Requests per GPU**:
```promql
sum(rate(vllm_requests_total{environment="production"}[5m])) by (pod)
/
sum(kube_pod_container_resource_requests{resource="nvidia_com_gpu", pod=~".*-production-.*"}) by (pod)
```

**GPU utilization headroom**:
```promql
100 - avg(nvidia_gpu_utilization{pod=~".*-production-.*"})
```

**Recommended scaling threshold**: Scale up when GPU utilization > 80% sustained for 10 minutes

## SLO Review Process

### Weekly Review

**Metrics to review**:
- [ ] Availability: Did we meet 99.9% uptime?
- [ ] Error budget: How much budget consumed? How much remaining?
- [ ] Latency trends: Any degradation compared to baseline?
- [ ] Error rate: Any spikes or anomalies?
- [ ] Incident count: How many SLO violations?

**Action items**:
- Identify root causes of SLO violations
- Update runbooks based on incidents
- Adjust alert thresholds if needed

### Monthly Review

**SLO Report Template**:
```markdown
# vLLM SLO Report - [Month Year]

## Summary
- Availability: [X]% (Target: 99.9%)
- Error Budget: [X] minutes consumed / 43.8 minutes total
- SLO Violations: [Count]

## Key Incidents
1. [Date] - [Description] - [Duration] - [Impact]

## Trends
- Latency: [Improving/Stable/Degrading]
- Error rate: [Improving/Stable/Degrading]
- Throughput: [Increasing/Stable/Decreasing]

## Action Items
- [ ] [Action item 1]
- [ ] [Action item 2]
```

### Quarterly Review

**Strategic questions**:
1. Are our SLOs still appropriate?
2. Do we need to adjust targets based on business needs?
3. What infrastructure changes are needed to improve SLOs?
4. Should we add new SLO metrics?

## Alerting Strategy

### Alert Prioritization

**Critical (Page oncall)**:
- Availability < 99% for > 5 minutes
- P95 latency > 6s for > 5 minutes
- Error rate > 5% for > 5 minutes
- All pods down

**High (Slack alert)**:
- Availability < 99.5% for > 10 minutes
- P95 latency > 3s for > 10 minutes
- Error rate > 1% for > 10 minutes
- Error budget < 20% remaining

**Medium (Email notification)**:
- P95 latency > 3s for > 30 minutes
- Error rate > 0.5% for > 30 minutes
- Burn rate > 2x for > 1 hour

### Alert Fatigue Prevention

**Principles**:
1. Alert on symptoms, not causes
2. Every alert must be actionable
3. Use multi-window burn rate alerts to reduce noise
4. Group related alerts (1 alert for 10 pods down, not 10 alerts)

## Tools and Integrations

### Prometheus
- Metrics collection and storage
- PromQL queries for SLO calculations
- Alert rule evaluation

### Grafana
- Visualization dashboards
- SLO tracking panels
- Annotations for deployments

### Alertmanager
- Alert routing and grouping
- Integration with PagerDuty, Slack, email
- Alert silencing and inhibition

### SLO Tools
- **Sloth**: Generate SLO rules and alerts from SLO definitions
- **Pyrra**: SLO dashboard and alerting
- **Nobl9**: SLO platform (commercial)

## See Also

- [vLLM Deployment Dashboard](../dashboards/vllm-deployment-dashboard.json)
- [Prometheus Alert Rules](./vllm-alerts.yaml)
- [Rollback Workflow](../rollback-workflow.md)
- [Partial Failure Remediation](../runbooks/partial-failure-remediation.md)
