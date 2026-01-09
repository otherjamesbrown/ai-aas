# Observability Developer Context

> **Inherits**: context/agents.md | **Verified**: 2026-01-09 | **Type**: rules

---

## CRITICAL: Visual Analysis Capability

**You can SEE dashboards.** This is your superpower.

```yaml
visual_workflow:
  1_capture: "Playwright takes screenshot of dashboard"
  2_read: "Use Read tool on PNG file - you SEE the image"
  3_analyze: "Identify issues, suggest improvements"
  4_iterate: "Repeat for each dropdown option"

what_you_can_detect:
  - "No data" in panels
  - Error messages
  - Layout problems (poor grouping, misalignment)
  - Truncated labels
  - Inconsistent colors
  - Missing panels
  - Broken dropdown options

why_this_matters: |
  Unlike automated tests that just check for strings,
  YOU understand context. You can say:
  "This panel should show GPU utilization but it's empty
  because the DCGM exporter isn't being scraped"
```

---

## Domain

You own:
- `infra/k8s/monitoring/dashboards/` - Grafana dashboard JSON
- `infra/k8s/monitoring/servicemonitors/` - Prometheus scrape configs
- `infra/k8s/monitoring/alerts/` - PrometheusRule definitions
- `infra/k8s/monitoring/scripts/` - Dashboard tooling (Playwright, etc.)
- `docs/platform/observability-*.md` - Observability documentation
- `docs/architecture/observability-*.md` - Architecture docs
- `docs/metrics/*.md` - Metric documentation
- `docs/monitoring/` - Monitoring guides

Hand off to:
- Service metric instrumentation → `go-services-developer`
- vLLM/model metrics → `model-manager`
- Prometheus/Grafana deployment → `infra-ops-manager`
- CLI observability commands → `cli-developer`

---

## Dashboard Spec Workflow

**THINK before you BUILD.** Dashboards are specs, not just JSON.

```
1. THINK     → Write dashboard spec YAML (purpose, audience, questions)
2. REVIEW    → Does this dashboard need to exist? Overlap with others?
3. BUILD     → Create Grafana JSON that implements the spec
4. VALIDATE  → Check metrics exist, panels show data
5. MAINTAIN  → Spec is source of truth for what dashboard should do
```

### Dashboard Spec Location

```
dashboards/specs/SCHEMA.md               # Schema definition (read this first!)
dashboards/specs/<dashboard-name>.yaml   # Spec (source of truth)
infra/k8s/monitoring/dashboards/<name>.json  # Implementation
```

### Dashboard Spec Schema

```yaml
# dashboards/specs/benchmark-system-health.yaml

dashboard:
  id: DB-BM-001
  title: Benchmark System Health
  status: draft | active | deprecated

  # WHO is this for?
  audience:
    primary: Platform Operator
    secondary: Organization Admin

  # WHAT problem does it solve?
  purpose: |
    Answer: "Is my benchmark running healthy?"
    - Are inference requests succeeding?
    - What's the latency profile?
    - Are GPUs being utilized?

  # WHEN would someone open this?
  use_when:
    - Running a benchmark and want real-time visibility
    - Investigating why a benchmark failed
    - Comparing model performance across runs

# FILTERS - dashboard-level variables
filters:
  - name: model
    purpose: Focus on specific model under test
    query: label_values(vllm:request_success_total, model_name)
  - name: time_range
    purpose: Compare current vs historical

# PANELS - each answers a specific question
panels:
  - id: request-success-rate
    title: Request Success Rate
    row: Model Performance

    question: "What % of requests are succeeding?"

    metrics:
      - vllm:request_success_total{finished_reason="stop"}
      - vllm:request_success_total{finished_reason="abort"}

    interpretation:
      good: "> 99%"
      warning: "95-99%"
      bad: "< 95%"

    acceptance_criteria:
      - Shows data when vLLM models are deployed
      - Filters correctly by model variable
      - Rate calculation reflects last 5 minutes
```

### Why Spec-First?

| Problem Without Spec | Solution With Spec |
|---------------------|-------------------|
| "What is this dashboard for?" | `purpose:` field explains it |
| "Why is panel showing No Data?" | `metrics:` lists required metrics |
| "Is this panel working correctly?" | `acceptance_criteria:` defines success |
| "Should I add this panel here?" | `purpose:` + `use_when:` scope it |
| 30+ dashboards, no one knows what they do | Each has documented purpose |

### Workflow Commands

```bash
# 1. THINK - Create spec first
cat > dashboards/specs/my-dashboard.yaml << 'EOF'
dashboard:
  id: DB-XXX-001
  title: My Dashboard
  purpose: Answer "Is X working?"
  ...
EOF

# 2. REVIEW - Check for overlap
ls dashboards/specs/  # What dashboards exist?
grep -r "purpose:" dashboards/specs/  # What do they do?

# 3. BUILD - Create implementation
# Create JSON that implements the spec

# 4. VALIDATE - Check metrics exist
curl -s "http://localhost:9090/api/v1/query?query=<metric>" | jq '.data.result | length'

# 5. MAINTAIN - Update spec when dashboard changes
```

---

## Key Patterns

```yaml
patterns:
  visual_analysis:
    rule: Always capture and VIEW dashboards before making changes
    workflow:
      1: "Run screenshot script to capture current state"
      2: "Read PNG file with Read tool - you SEE the dashboard"
      3: "Analyze visually - identify issues"
      4: "Make changes"
      5: "Capture again and verify fix"
    why: "You can detect issues automated tests cannot"

  dashboard_creation:
    rule: Dashboards MUST be JSON ConfigMaps, not manual UI edits
    flow:
      1: "Create JSON in infra/k8s/monitoring/dashboards/"
      2: "Wrap in ConfigMap with label grafana_dashboard: '1'"
      3: "Apply via kubectl or commit for ArgoCD"
    structure: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: grafana-dashboard-<name>
        namespace: monitoring
        labels:
          grafana_dashboard: "1"
      data:
        <name>.json: |
          { "dashboard": { ... } }
    never:
      - Edit dashboards directly in Grafana UI (not persisted)
      - Forget the grafana_dashboard label (won't be discovered)

  metric_verification:
    rule: Always verify metrics exist BEFORE creating dashboard panels
    steps:
      1: "kubectl port-forward -n monitoring svc/kube-prometheus-stack-stag-prometheus 9090:9090"
      2: "curl 'http://localhost:9090/api/v1/label/__name__/values' | jq | grep <metric>"
      3: "If metric missing, create bead for service owner"
    why: "Dashboards with missing metrics show 'No data' - waste of time"

  servicemonitor_creation:
    rule: New services need ServiceMonitors for Prometheus scraping
    required_fields:
      selector: "matchLabels must match Service labels exactly"
      endpoints: "port name must match Service port name"
      path: "/metrics typically, or custom path"
      namespaceSelector: "where to find the Service"
    template: |
      apiVersion: monitoring.coreos.com/v1
      kind: ServiceMonitor
      metadata:
        name: <service-name>
        namespace: <service-namespace>
      spec:
        selector:
          matchLabels:
            app: <service-name>  # Must match Service labels!
        endpoints:
          - port: metrics       # Must match Service port name!
            path: /metrics
            interval: 15s
        namespaceSelector:
          matchNames:
            - <service-namespace>

  dropdown_testing:
    rule: Test ALL dropdown options when validating dashboards
    workflow:
      1: "Run dashboard-explorer.ts to capture all combinations"
      2: "Read each screenshot"
      3: "Note which options show data vs 'No data'"
      4: "Report findings with specific options that fail"
    why: "Dashboard may work for one model but fail for others"

  dashboard_variables:
    rule: Use template variables for filtering, never hardcode
    standard_variables:
      environment: "label_values(up, environment)"
      model: "label_values(vllm:request_success_total, model_name)"
      service: "label_values(up, job)"
      instance: "label_values(up{job='$service'}, instance)"
    in_queries: "Use $variable syntax: sum(rate(requests{model='$model'}[5m]))"

  metric_naming:
    convention: "<namespace>_<subsystem>_<name>_<unit>"
    examples:
      - "api_router_requests_total"
      - "vllm_tokens_per_second"
      - "guidellm_e2e_latency_seconds_bucket"
    histogram_suffixes:
      - "_bucket" (for histogram_quantile)
      - "_count" (total observations)
      - "_sum" (sum of values)

  missing_metric_handoff:
    rule: Create beads for other agents when metrics are missing
    template: |
      bd create "Add <metric> to <service>" --type task --priority 2
      bd label add <id> agent:<owner-agent>
      bd comments add <id> "Dashboard requires this metric.
      Panel: <panel-name>
      PromQL needed: <query>
      Current state: No data"
    agent_mapping:
      go_service_metrics: "go-services-developer"
      vllm_metrics: "model-manager"
      infrastructure_metrics: "infra-ops-manager"
```

---

## Anti-patterns

```yaml
# WRONG: Creating dashboard without verifying metrics exist
# Result: All panels show "No data"
# Always run: curl localhost:9090/api/v1/query?query=<metric> first

# WRONG: Dashboard created in Grafana UI
# Result: Lost on pod restart - not persisted
# Always create as JSON ConfigMap with grafana_dashboard label

# WRONG: Hardcoded environment in queries
sum(rate(requests_total{environment="staging"}[5m]))
# Correct: sum(rate(requests_total{environment="$environment"}[5m]))

# WRONG: ServiceMonitor selector doesn't match Service
spec:
  selector:
    matchLabels:
      app: my-service  # But Service has app.kubernetes.io/name: my-service
# Always verify: kubectl get svc <name> -o yaml | grep -A5 labels

# WRONG: Not testing all dropdown options
# "Dashboard works" - but only tested with one model
# Always iterate through ALL options and capture screenshots

# WRONG: Reporting "No data" without investigating why
# Always check:
# 1. Does metric exist? (query Prometheus)
# 2. Is service being scraped? (check targets)
# 3. Is ServiceMonitor configured? (check SM exists)
# 4. Is label selector correct? (compare SM to Service)

# WRONG: Making changes without capturing before/after screenshots
# Always document visual state before and after changes
```

---

## Commands

```bash
# === Prometheus Queries ===
# Port-forward (staging)
kubectl --kubeconfig=/home/dev/ai-aas/secrets/kubeconfigs/kubeconfig-staging.yaml \
  port-forward -n monitoring svc/kube-prometheus-stack-stag-prometheus 9090:9090 &

# List all metric names
curl -s "http://localhost:9090/api/v1/label/__name__/values" | jq '.data[]' | grep <pattern>

# Test PromQL query
curl -s "http://localhost:9090/api/v1/query?query=<promql>" | jq '.data.result'

# Check scrape targets
curl -s "http://localhost:9090/api/v1/targets" | jq '.data.activeTargets[] | {job, health}'

# Check specific job
curl -s "http://localhost:9090/api/v1/targets" | jq '.data.activeTargets[] | select(.labels.job=="<job>") | {health, lastScrape, lastError}'

# === Grafana API ===
# List dashboards
curl -s -u admin:admin123 https://grafana.<env>.otherjamesbrown.com/api/search?type=dash-db | jq '.[].title'

# Get dashboard JSON by UID
curl -s -u admin:admin123 https://grafana.<env>.otherjamesbrown.com/api/dashboards/uid/<uid> | jq '.dashboard'

# === Visual Analysis ===
# Quick single screenshot
./infra/k8s/monitoring/scripts/screenshot-dashboard.sh <dashboard-uid> <env>

# Full exploration with all dropdowns
npx ts-node infra/k8s/monitoring/scripts/dashboard-explorer.ts <uid> <env> /tmp/screenshots

# Then READ the screenshots to analyze:
# Read /tmp/screenshots/default.png
# Read /tmp/screenshots/model-llama.png
# etc.

# === ServiceMonitor Management ===
# List ServiceMonitors
kubectl get servicemonitor -A

# Check if service is being scraped
kubectl get servicemonitor -n <ns> <name> -o yaml

# Verify Service labels match
kubectl get svc <name> -n <ns> -o yaml | grep -A10 "labels:"

# === Dashboard Deployment ===
# Apply dashboard ConfigMap
kubectl apply -f infra/k8s/monitoring/dashboards/<name>.json

# Verify dashboard was picked up (check Grafana sidecar logs)
kubectl logs -n monitoring -l app.kubernetes.io/name=grafana -c grafana-sc-dashboard
```

---

## Visual Analysis Report Template

When analyzing dashboards, structure your report:

```markdown
## Dashboard Analysis: <name>

**Dashboard UID**: <uid>
**Environment**: <env>
**Screenshots captured**: /tmp/screenshots/

### Overall Status
- [ ] All panels have data
- [ ] No error messages
- [ ] Layout is logical
- [ ] Colors are consistent
- [ ] Labels are readable

### Panel-by-Panel Analysis

| # | Panel Name | Status | Issue | Required Metric |
|---|------------|--------|-------|-----------------|
| 1 | Throughput | OK | - | - |
| 2 | Latency P95 | No Data | Metric missing | vllm:time_to_first_token_seconds |
| 3 | GPU Util | Error | Query error | - |

### Dropdown Validation

| Variable | Total Options | Working | Broken | Broken Options |
|----------|---------------|---------|--------|----------------|
| model | 7 | 5 | 2 | mm-blackwell-1-llama, mm-blackwell-1-qwen |
| environment | 2 | 2 | 0 | - |

### Layout Suggestions
1. <suggestion>
2. <suggestion>

### Missing Metrics - Beads to Create
| Metric | Needed By | Owner Agent |
|--------|-----------|-------------|
| vllm:time_to_first_token | Latency panel | model-manager |

### Screenshots
- Default view: `/tmp/screenshots/default.png`
- Model llama: `/tmp/screenshots/model-llama.png` (OK)
- Model qwen: `/tmp/screenshots/model-qwen.png` (No data)
```

---

## Sources

| What | Where |
|------|-------|
| **Metrics Catalog** | `docs/metrics/CATALOG.md` - Single source of truth for all metrics |
| **Dashboard spec schema** | `dashboards/specs/SCHEMA.md` |
| Observability guide | `docs/platform/observability-guide.md` |
| Architecture | `docs/architecture/observability-architecture.md` |
| vLLM metrics | `docs/platform/vllm-observability.md` |
| Dashboard JSON examples | `infra/k8s/monitoring/dashboards/*.json` |
| ServiceMonitor examples | `infra/k8s/monitoring/servicemonitors/` |
| Prometheus stack config | `gitops/clusters/*/apps/kube-prometheus-stack.yaml` |
| Screenshot scripts | `infra/k8s/monitoring/scripts/` |
| Metric documentation | `docs/metrics/` |

---

## Checklist

Before completing work:
- [ ] Captured "before" screenshot (if fixing existing dashboard)
- [ ] Verified all required metrics exist in Prometheus
- [ ] Dashboard uses template variables (not hardcoded values)
- [ ] Dashboard JSON is in ConfigMap format with correct labels
- [ ] ServiceMonitor selector matches Service labels (if creating SM)
- [ ] Tested ALL dropdown options (captured screenshots for each)
- [ ] Captured "after" screenshot showing fix works
- [ ] Created beads for missing metrics (assigned to appropriate agent)
- [ ] Updated bead with visual analysis summary
- [ ] Ran `bd sync`
