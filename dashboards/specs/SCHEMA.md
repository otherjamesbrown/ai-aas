# Dashboard Spec YAML Schema

This document defines the schema for dashboard specification files in the `dashboards/specs/` directory.

## Purpose

Dashboard specs are the **source of truth** for what a dashboard should do. They answer:
- WHO is this dashboard for?
- WHAT questions does it answer?
- WHEN would someone use it?
- HOW should each panel behave?

**Write the spec BEFORE building the dashboard.**

## File Organization

- One YAML file per dashboard (e.g., `benchmark-system-health.yaml`)
- Filename should match the Grafana dashboard name (kebab-case)
- Implementation JSON lives in `infra/k8s/monitoring/dashboards/`

```
dashboards/
  specs/
    SCHEMA.md                        # This file
    benchmark-system-health.yaml     # Spec
    model-performance.yaml           # Spec
    platform-overview.yaml           # Spec

infra/k8s/monitoring/dashboards/
    benchmark-system-health.json     # Implementation
    model-performance.json           # Implementation
    platform-overview.json           # Implementation
```

## Naming Conventions

### Dashboard ID Format

`DB-{PREFIX}-{NNN}` (e.g., `DB-BM-001`)

| Prefix | Category |
|--------|----------|
| `DB-BM-` | Benchmark dashboards |
| `DB-MDL-` | Model performance dashboards |
| `DB-PLT-` | Platform/infrastructure dashboards |
| `DB-SVC-` | Service-specific dashboards |
| `DB-OPS-` | Operations/SRE dashboards |
| `DB-USR-` | User/tenant dashboards |

### Panel ID Format

Kebab-case descriptive name (e.g., `request-success-rate`, `gpu-utilization`)

## Schema Definition

```yaml
# dashboards/specs/{dashboard-name}.yaml

# === DASHBOARD METADATA ===
dashboard:
  id: DB-{PREFIX}-{NNN}              # Required: Unique identifier
  title: string                       # Required: Human-readable title
  status: draft | active | deprecated # Required: Current status
  grafana_uid: string                 # Optional: Grafana dashboard UID (set after creation)

  # Deprecation info (only when status: deprecated)
  deprecated_reason: string           # Why deprecated
  deprecated_by: DB-XXX-NNN           # Replacement dashboard if any

  # WHO is this dashboard for?
  audience:                           # Required
    primary: string                   # Required: Main user role
    secondary: string                 # Optional: Secondary user role

  # WHAT problem does it solve?
  purpose: |                          # Required: Multiline description
    Answer: "What question does this dashboard answer?"
    - Bullet points of specific insights provided
    - Focus on user goals, not technical details

  # WHEN would someone open this?
  use_when:                           # Required: List of scenarios
    - Scenario 1
    - Scenario 2

  # Related dashboards (for navigation)
  related_dashboards:                 # Optional
    - DB-XXX-NNN                      # Link to related dashboard IDs

  # Dependencies
  datasources:                        # Required: What data sources are needed
    - name: Prometheus
      type: prometheus
      required: true
    - name: Loki
      type: loki
      required: false

# === FILTERS (Template Variables) ===
filters:                              # Optional but recommended
  - name: string                      # Required: Variable name (used in queries as $name)
    label: string                     # Required: Display label
    purpose: string                   # Required: What does this filter do?
    query: string                     # Required: PromQL label_values() or static list
    default: string                   # Optional: Default value
    multi: boolean                    # Optional: Allow multiple selection (default: false)
    include_all: boolean              # Optional: Include "All" option (default: true)

# === ROWS (Panel Grouping) ===
rows:                                 # Optional: Define row structure
  - name: string                      # Row title
    collapsed: boolean                # Start collapsed? (default: false)
    purpose: string                   # What panels in this row show

# === PANELS ===
panels:                               # Required: At least one panel
  - id: string                        # Required: Unique panel ID (kebab-case)
    title: string                     # Required: Panel title
    row: string                       # Optional: Which row this panel belongs to
    type: graph | stat | gauge | table | heatmap | logs  # Required

    # WHAT question does this panel answer?
    question: string                  # Required: The question this panel answers

    # Detailed description
    description: |                    # Optional: Longer explanation
      Additional context about what this panel shows
      and how to interpret the data.

    # Required metrics
    metrics:                          # Required: List of metrics used
      - name: string                  # Metric name
        labels:                       # Important labels
          - label1
          - label2
        aggregation: string           # e.g., "rate(5m)", "histogram_quantile(0.95, ...)"

    # PromQL query (reference, may differ slightly in implementation)
    query: string                     # Optional: Example PromQL query

    # How to interpret values
    interpretation:                   # Recommended for stat/gauge panels
      good: string                    # What value range is healthy
      warning: string                 # What value range needs attention
      bad: string                     # What value range is critical

    # Alert thresholds (if panel has alerts)
    alert_thresholds:                 # Optional
      warning: string                 # Warning threshold
      critical: string                # Critical threshold

    # Acceptance criteria - when is this panel "working"?
    acceptance_criteria:              # Required: At least one criterion
      - criterion 1
      - criterion 2

    # What this panel does NOT show
    out_of_scope:                     # Optional: Clarify boundaries
      - What this panel is NOT for
```

## Example

```yaml
# dashboards/specs/benchmark-system-health.yaml

dashboard:
  id: DB-BM-001
  title: Benchmark System Health
  status: active
  grafana_uid: benchmark-system-health

  audience:
    primary: Platform Operator
    secondary: Organization Admin

  purpose: |
    Answer: "Is my benchmark running healthy?"
    - Are inference requests succeeding or failing?
    - What's the latency profile (TTFT, ITL, E2E)?
    - Are GPUs being utilized efficiently?
    - Is throughput meeting expectations?

  use_when:
    - Running a benchmark and want real-time visibility
    - Investigating why a benchmark failed or underperformed
    - Comparing model performance across different runs
    - Validating model deployment before production

  related_dashboards:
    - DB-MDL-001  # Model Performance Deep Dive
    - DB-PLT-001  # Platform Overview

  datasources:
    - name: Prometheus
      type: prometheus
      required: true

filters:
  - name: model
    label: Model
    purpose: Filter all panels to show data for a specific model
    query: label_values(vllm:request_success_total, model_name)
    default: All
    multi: false
    include_all: true

  - name: environment
    label: Environment
    purpose: Switch between staging and production views
    query: label_values(up, environment)
    default: staging
    multi: false
    include_all: false

rows:
  - name: Request Performance
    collapsed: false
    purpose: Overall request success/failure metrics

  - name: Latency Breakdown
    collapsed: false
    purpose: Detailed latency metrics (TTFT, ITL, E2E)

  - name: Throughput
    collapsed: false
    purpose: Token generation and request throughput

  - name: GPU Infrastructure
    collapsed: true
    purpose: GPU utilization and hardware metrics

panels:
  # === Request Performance Row ===
  - id: request-success-rate
    title: Request Success Rate
    row: Request Performance
    type: stat

    question: "What percentage of inference requests are succeeding?"

    metrics:
      - name: vllm:request_success_total
        labels: [model_name, finished_reason]
        aggregation: rate(5m)

    query: |
      sum(rate(vllm:request_success_total{finished_reason="stop"}[5m])) /
      sum(rate(vllm:request_success_total[5m])) * 100

    interpretation:
      good: "> 99%"
      warning: "95-99%"
      bad: "< 95%"

    acceptance_criteria:
      - Shows percentage when vLLM models are deployed and receiving traffic
      - Filters correctly when model variable is changed
      - Updates in real-time (15s refresh)

  - id: error-rate
    title: Error Rate
    row: Request Performance
    type: stat

    question: "What percentage of requests are failing?"

    metrics:
      - name: vllm:request_success_total
        labels: [model_name, finished_reason]
        aggregation: rate(5m)

    query: |
      sum(rate(vllm:request_success_total{finished_reason="abort"}[5m])) /
      sum(rate(vllm:request_success_total[5m])) * 100

    interpretation:
      good: "< 1%"
      warning: "1-5%"
      bad: "> 5%"

    acceptance_criteria:
      - Shows 0% when no errors occurring
      - Spikes visible when errors occur
      - Breakdown by model available via filter

  # === Latency Row ===
  - id: time-to-first-token
    title: Time to First Token (P95)
    row: Latency Breakdown
    type: stat

    question: "How long until users see the first token?"

    description: |
      TTFT is critical for user-perceived latency. Users waiting more than
      2 seconds for the first token will perceive the system as slow.

    metrics:
      - name: vllm:time_to_first_token_seconds_bucket
        labels: [model_name]
        aggregation: histogram_quantile(0.95, rate(...[5m]))

    query: |
      histogram_quantile(0.95,
        sum(rate(vllm:time_to_first_token_seconds_bucket{model_name=~"$model"}[5m])) by (le)
      )

    interpretation:
      good: "< 500ms"
      warning: "500ms - 2s"
      bad: "> 2s"

    acceptance_criteria:
      - Shows latency in human-readable format (ms or s)
      - P95 calculation is accurate
      - Responds to model filter

  - id: inter-token-latency
    title: Inter-Token Latency (P95)
    row: Latency Breakdown
    type: stat

    question: "How fast are tokens being generated after the first one?"

    metrics:
      - name: vllm:inter_token_latency_seconds_bucket
        labels: [model_name]
        aggregation: histogram_quantile(0.95, rate(...[5m]))

    interpretation:
      good: "< 50ms"
      warning: "50-100ms"
      bad: "> 100ms"

    acceptance_criteria:
      - Shows latency when streaming is active
      - Lower values indicate faster token generation

  # === Throughput Row ===
  - id: generation-throughput
    title: Generation Throughput
    row: Throughput
    type: stat

    question: "How many tokens per second are being generated?"

    metrics:
      - name: vllm:generation_tokens_total
        labels: [model_name]
        aggregation: rate(5m)

    query: |
      sum(rate(vllm:generation_tokens_total{model_name=~"$model"}[5m]))

    interpretation:
      good: "Depends on model size"
      warning: "Below baseline"
      bad: "Near zero"

    acceptance_criteria:
      - Shows tokens/sec when inference is running
      - Zero when no active requests (expected)
      - Scales with concurrent requests

  # === GPU Row ===
  - id: gpu-utilization
    title: GPU Utilization
    row: GPU Infrastructure
    type: gauge

    question: "How efficiently are GPUs being used?"

    metrics:
      - name: DCGM_FI_DEV_GPU_UTIL
        labels: [gpu, instance]
        aggregation: avg

    interpretation:
      good: "70-95%"
      warning: "< 70% (underutilized) or > 95% (saturated)"
      bad: "0% (idle) or 100% (bottleneck)"

    acceptance_criteria:
      - Shows percentage for each GPU
      - Updates every 15 seconds
      - Works when DCGM exporter is running

    out_of_scope:
      - GPU memory utilization (separate panel)
      - Per-process GPU usage
```

## Validation

Use the linter to validate dashboard spec files:

```bash
./scripts/lint-dashboard-specs.sh
```

The linter checks:
- ID format matches `DB-{PREFIX}-{NNN}` pattern
- Required fields present
- Panel IDs are unique within dashboard
- Referenced metrics use valid PromQL patterns
- Filters have valid queries

## Workflow

```
1. THINK     → Write spec YAML first (this file format)
2. REVIEW    → Check for overlap: ls dashboards/specs/ && grep -r "purpose:" dashboards/specs/
3. BUILD     → Create Grafana JSON implementing the spec
4. VALIDATE  → Verify metrics exist, panels show data
5. MAINTAIN  → Update spec when dashboard changes
```

## Mapping Spec to JSON

| Spec Field | Grafana JSON Field |
|------------|-------------------|
| `dashboard.title` | `.dashboard.title` |
| `dashboard.grafana_uid` | `.dashboard.uid` |
| `filters[].name` | `.dashboard.templating.list[].name` |
| `filters[].query` | `.dashboard.templating.list[].query` |
| `rows[].name` | `.dashboard.panels[].title` (type: row) |
| `panels[].title` | `.dashboard.panels[].title` |
| `panels[].query` | `.dashboard.panels[].targets[].expr` |

## Related Documentation

- [Use Case Schema](../../usecases/SCHEMA.md) - Similar pattern for feature specs
- [Observability Developer Context](../../context/observability-developer/agents.md) - Agent workflow
- [Dashboard JSON Examples](../../infra/k8s/monitoring/dashboards/) - Implementation files
