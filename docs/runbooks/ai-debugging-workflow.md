# AI Coding Assistant Debugging Workflow

---
last_updated: 2025-12-15
document_type: runbook
audience: ai-assistants
spec: 024-logging-observability-improvements
---

This guide provides step-by-step instructions for AI coding assistants (Claude) to debug issues in the AI-AAS platform using the centralized observability stack. This runbook is specifically designed for AI assistants to efficiently diagnose and resolve platform issues.

## Quick Reference

| Component | URL | Purpose |
|-----------|-----|---------|
| Grafana | `https://grafana.dev.otherjamesbrown.com` | Log visualization, dashboards, trace exploration |
| Loki API | `https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range` | Direct log querying |
| ArgoCD | `https://argocd.dev.otherjamesbrown.com` | Deployment status and sync operations |
| Sentry | External SaaS | Frontend error tracking with session replay |

### Quick Command Reference

| Task | Command/Query |
|------|---------------|
| Find all errors | `{namespace="system"} \| json \| level="error"` |
| Search by trace ID | `{namespace="system"} \| json \| trace_id="<id>"` |
| Service real-time logs | `kubectl logs -n system -l app=<service> -f --tail=100` |
| vLLM GPU errors | `{namespace=~"ai-models\|system\|kserve"} \| json \| gpu_error="true"` |
| Model loading status | `{namespace=~"ai-models\|system\|kserve"} \| json \| loading_status=~"loading\|ready\|failed"` |
| Slow requests | `{namespace="system"} \| json \| duration > 1000` |
| Error count by service | `sum(count_over_time({namespace="system"} \| json \| level="error" [1h])) by (service)` |

## Prerequisites

Before debugging, ensure you have:

1. **Kubeconfig access**: `secrets/kubeconfigs/kubeconfig-development.yaml`
2. **kubectl configured**: `export KUBECONFIG=/home/dev/ai-aas-024-observability/secrets/kubeconfigs/kubeconfig-development.yaml`
3. **Access to observability endpoints** (verified via curl or browser)
4. **Basic understanding of LogQL** (Prometheus-like query language for logs)

## Step-by-Step Debug Workflow

This is the systematic approach to debugging any issue in the AI-AAS platform:

### Step 1: Gather Initial Information

Before diving into logs, collect context about the issue:

**Questions to Answer:**
- What service or component is affected? (api-router, user-org-service, analytics, vLLM model, frontend)
- When did the issue start? (timestamp or time range)
- What are the symptoms? (error message, slow response, crash, incorrect behavior)
- Is there a request ID, trace ID, or user ID associated with the issue?
- Is the issue reproducible or intermittent?

**Initial Health Check:**
```bash
# Check pod status for all services
kubectl get pods -n system -o wide

# Check recent events for anomalies
kubectl get events -n system --sort-by='.lastTimestamp' | tail -20

# Check ArgoCD sync status
kubectl get applications -n argocd
```

### Step 2: Check Recent Logs

Start with real-time logs to see current activity:

**Real-time logs via kubectl:**
```bash
# Tail logs for specific service (replace <service> with actual service name)
kubectl logs -n system deployment/<service> -f --tail=100

# Example: API Router service
kubectl logs -n system deployment/api-router-service -f --tail=100

# Example: User Org service
kubectl logs -n system deployment/user-org-service -f --tail=100

# Get logs from previous pod (if crashed)
kubectl logs -n system <pod-name> --previous
```

**Historical search in Grafana:**
1. Navigate to `https://grafana.dev.otherjamesbrown.com`
2. Click **Explore** (compass icon in left sidebar)
3. Select **Loki** as the data source
4. Enter LogQL query:
   ```logql
   {namespace="system", service="<service-name>"} | json | level=~"error|warn"
   ```
5. Adjust time range (default: last 1 hour)

**Direct Loki API query:**
```bash
# Query errors in last 15 minutes
curl -s 'https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range' \
  --data-urlencode 'query={namespace="system", level="error"}' \
  --data-urlencode 'limit=100' \
  --data-urlencode 'start='$(date -u -d '15 minutes ago' +%s)000000000 \
  --data-urlencode 'end='$(date -u +%s)000000000 | jq '.data.result'
```

### Step 3: Find Related Traces

If the issue involves a request flow across multiple services, use distributed tracing:

**Extract trace_id from logs:**
```logql
# Find logs with trace information
{namespace="system", service="<service>"} | json | trace_id != ""
```

**View full trace in Grafana:**
1. Copy the `trace_id` from the log entry
2. Go to Grafana → **Explore** → Select **Tempo** datasource
3. Enter the trace ID in the query field
4. View the complete request flow with timing information

**Alternative: Direct trace search by time range:**
1. Grafana → Explore → Tempo
2. Click **Search** tab
3. Filter by:
   - Service Name: Select service
   - Status: Select `error` for failed requests
   - Duration: Filter by min/max duration
   - Time range: Adjust to incident window

### Step 4: Correlate Across Services

Use trace_id to find logs from all services involved in a request:

**Query all logs for a trace:**
```logql
{namespace="system"} | json | trace_id="<trace_id>"
```

**Trace-to-logs workflow in Grafana:**
1. View trace in Tempo (Step 3)
2. Click on any span in the trace
3. Click **Logs for this span** button
4. Grafana automatically opens Loki with filtered logs

**Correlation by request_id (if trace_id unavailable):**
```logql
{namespace="system"} | json | request_id="<request_id>"
```

### Step 5: Root Cause Analysis

Identify the first error in the trace or log sequence:

**Analyze error patterns:**
```logql
# Find errors with stack traces
{namespace="system"} | json | level="error" | error != ""

# Count errors by type
sum(count_over_time({namespace="system"} | json | level="error" [1h])) by (msg)

# Find first occurrence of error
{namespace="system"} | json | level="error" | msg=~".*connection refused.*"
```

**Check upstream dependencies:**
- Database connectivity: Check PostgreSQL pod status
- External APIs: Look for timeout or connection errors
- Inter-service communication: Verify service mesh health

**Verify external service health:**
```bash
# Check database pod
kubectl get pods -n system -l app=postgresql

# Check Redis (if applicable)
kubectl get pods -n system -l app=redis

# Check model backend pods
kubectl get pods -n ai-models
kubectl get pods -n kserve
```

### Step 6: Verify Fix

After implementing a fix, verify the issue is resolved:

**Monitor logs for errors:**
```logql
# Real-time error monitoring
{namespace="system", service="<service>"} | json | level="error"
```

**Check service health:**
```bash
# Verify pod is running
kubectl get pods -n system -l app=<service>

# Check pod events
kubectl describe pod -n system <pod-name>

# Test service endpoint
curl -k https://api.dev.otherjamesbrown.com/health
```

**Verify metrics in Grafana:**
- Check Service Logs dashboard for error rate
- Verify request latency has normalized
- Check resource utilization is stable

## Common Debugging Scenarios

### Scenario 1: API Returns 500 Error

**Symptoms:** HTTP 500 response from API endpoint, client reports server error

**Debug Steps:**
1. Get trace_id from error response header or client logs
2. Query Loki for API router logs:
   ```logql
   {namespace="system", service="api-router-service"} | json | trace_id="<id>"
   ```
3. View full trace in Tempo to see which backend service failed
4. Query backend service logs:
   ```logql
   {namespace="system", service="<backend-service>"} | json | trace_id="<id>" | level="error"
   ```
5. Look for error message with stack trace
6. Check if error is in application code or external dependency

**Common Causes:**
- Database connection timeout
- Downstream service unavailable
- Invalid request validation
- Panic/exception in handler code

### Scenario 2: Slow API Response

**Symptoms:** API latency > 5 seconds, timeout warnings, poor user experience

**Debug Steps:**
1. Check request latency in Service Logs dashboard (Grafana)
2. Query for slow requests:
   ```logql
   {namespace="system"} | json | duration > 5000 | line_format "{{.service}} {{.method}} {{.path}} {{.duration}}ms"
   ```
3. Get trace_id from slow request
4. View trace in Tempo to identify slow spans
5. Check downstream service latency:
   ```logql
   {namespace="system", service="<downstream-service>"} | json | trace_id="<id>"
   ```
6. Look for database query time, external API calls, or computational bottlenecks

**Performance Metrics to Check:**
- Database query duration
- Cache hit rate
- CPU/memory utilization of pods
- Network latency between services

### Scenario 3: Model Loading Failure

**Symptoms:** vLLM model pod fails to start, model deployment stuck in "Loading" state

**Debug Steps:**
1. Check model pod status:
   ```bash
   kubectl get pods -n system -l model=<model-name>
   kubectl get pods -n ai-models -l model=<model-name>
   ```
2. Query vLLM logs for loading errors:
   ```logql
   {namespace=~"ai-models|system|kserve"} | json | loading_status="failed"
   ```
3. Look for CUDA/GPU errors:
   ```logql
   {namespace=~"ai-models|system|kserve"} | json | gpu_error="true"
   ```
4. Check pod events for resource issues:
   ```bash
   kubectl describe pod -n system <model-pod-name>
   ```
5. Verify GPU availability:
   ```bash
   # Check GPU nodes
   kubectl get nodes -l node.kubernetes.io/gpu=true

   # Check GPU allocation
   kubectl describe nodes -l node.kubernetes.io/gpu=true | grep -A 10 "Allocated resources"
   ```

**Common Causes:**
- Insufficient GPU memory
- Model not found in registry
- Network timeout downloading model weights
- CUDA driver mismatch
- OOM (Out of Memory) killer

### Scenario 4: Frontend Error

**Symptoms:** User reports error in web portal, React error boundary triggered

**Debug Steps:**
1. Get Sentry event ID from user or error boundary UI
2. Open Sentry dashboard (check `web/portal/src/lib/sentry.ts` for DSN)
3. Search for event by ID or user email
4. Review:
   - Error stack trace
   - Session replay (if available)
   - User actions leading to error (breadcrumbs)
   - Browser and OS information
5. Check if error has corresponding backend trace_id in Sentry context
6. If trace_id exists, correlate with backend logs:
   ```logql
   {namespace="system"} | json | trace_id="<trace_id_from_sentry>"
   ```

**Frontend-specific checks:**
```bash
# Check if frontend logger is configured
cat web/portal/src/lib/logger/index.ts

# Verify Sentry DSN is set
grep VITE_SENTRY_DSN web/portal/.env.development
```

### Scenario 5: Pod Crash Loop

**Symptoms:** Pod repeatedly restarting, high restart count, service unavailable

**Debug Steps:**
1. Check pod status and restart count:
   ```bash
   kubectl get pods -n system -l app=<service> -o wide
   ```
2. Get crash logs from previous pod:
   ```bash
   kubectl logs -n system <pod-name> --previous
   ```
3. Check Loki for logs before crash:
   ```logql
   {namespace="system", pod=~"<pod-name>.*"} | json
   ```
4. Review pod events:
   ```bash
   kubectl describe pod -n system <pod-name>
   ```
5. Check resource limits:
   ```bash
   kubectl get pod -n system <pod-name> -o jsonpath='{.spec.containers[0].resources}'
   ```

**Common Causes:**
- Application panic on startup
- Failed liveness/readiness probes
- OOM (Out of Memory)
- Missing environment variables or secrets
- Database connection failure on startup

### Scenario 6: Database Connection Issues

**Symptoms:** Connection refused, timeout errors, pool exhausted warnings

**Debug Steps:**
1. Search for database errors:
   ```logql
   {namespace="system"} |= "database" |= "error" | json
   ```
2. Check database pod health:
   ```bash
   kubectl get pods -n system -l app=postgresql
   kubectl logs -n system -l app=postgresql --tail=50
   ```
3. Verify database service:
   ```bash
   kubectl get svc -n system postgresql
   ```
4. Test database connectivity from service pod:
   ```bash
   kubectl exec -n system deployment/<service> -- nc -zv postgresql 5432
   ```
5. Check connection pool metrics in Grafana

**Common Causes:**
- Database pod not running
- Connection pool exhausted
- Invalid credentials
- Network policy blocking traffic
- Database migrations failed

### Scenario 7: Authentication Failures

**Symptoms:** 401 Unauthorized, token expired, JWT validation failed

**Debug Steps:**
1. Search for auth-related logs:
   ```logql
   {namespace="system", service="user-org-service"} | json | level=~"warn|error" | msg=~".*auth.*"
   ```
2. Look for `auth_failure_reason` in structured logs
3. Check JWT token validity and expiration:
   - Extract token from request
   - Decode JWT payload (use jwt.io)
   - Verify expiration time (`exp` claim)
4. Verify API key if applicable:
   ```bash
   # Check API key status via CLI
   ai-aas-cli apikey list
   ```

**Common Causes:**
- Expired JWT token
- Invalid API key
- User session expired
- Clock skew between services
- Missing authentication header

### Scenario 8: Performance Degradation

**Symptoms:** General slowness, increased latency across services, resource exhaustion

**Debug Steps:**
1. Check Service Logs dashboard in Grafana for anomalies
2. Query for slow requests:
   ```logql
   {namespace="system"} | json | duration > 1000
   ```
3. Check resource utilization:
   ```bash
   # CPU and memory usage
   kubectl top pods -n system
   kubectl top nodes
   ```
4. Review Prometheus metrics in Grafana:
   - CPU/memory usage by pod
   - Request rate and latency
   - Database connection pool usage
5. Look for log volume spikes:
   ```logql
   sum(rate({namespace="system"}[5m])) by (service)
   ```

**Performance Metrics to Monitor:**
- Request latency (p95, p99)
- Error rate
- Throughput (requests per second)
- CPU and memory utilization
- Database query time

## LogQL Query Patterns

LogQL is the query language for Loki, similar to PromQL for Prometheus.

### Basic Query Structure

```logql
{<label_selector>} |= "<string_filter>" | json | <field_filter>
```

- `{label_selector}`: Filter by indexed labels (namespace, service, pod)
- `|= "text"`: Search for string in log line (case-sensitive)
- `| json`: Parse JSON logs and extract fields
- `field_filter`: Filter by extracted JSON fields

### Basic Queries

**All errors in system namespace:**
```logql
{namespace="system"} | json | level="error"
```

**Specific service errors:**
```logql
{namespace="system", service="api-router-service"} | json | level="error"
```

**Search by message content:**
```logql
{namespace="system"} | json | msg=~".*connection refused.*"
```

**Multiple label filters:**
```logql
{namespace="system", service="user-org-service", level="error"}
```

**Case-insensitive search:**
```logql
{namespace="system"} |~ "(?i)error"
```

### Field Filtering

**Filter by numeric field:**
```logql
{namespace="system"} | json | duration > 1000
{namespace="system"} | json | status >= 500
```

**Filter by string field:**
```logql
{namespace="system"} | json | method="POST"
{namespace="system"} | json | path=~"/v1/.*"
```

**Multiple field filters:**
```logql
{namespace="system"} | json | level="error" | status >= 500
```

**Filter by trace ID:**
```logql
{namespace="system"} | json | trace_id="4bf92f3577b34da6a3ce929d0e0e4736"
```

### Aggregation Queries

**Error count by service (last hour):**
```logql
sum(count_over_time({namespace="system"} | json | level="error" [1h])) by (service)
```

**Log volume rate (logs per second):**
```logql
sum(rate({namespace="system"}[5m])) by (service)
```

**Unique error messages:**
```logql
count by (msg) (rate({namespace="system"} | json | level="error" [1h]))
```

**Average request duration:**
```logql
avg_over_time({namespace="system"} | json | unwrap duration [5m]) by (service)
```

### Advanced Queries

**Slow requests with context:**
```logql
{namespace="system"} | json | duration > 1000 | line_format "{{.service}} {{.method}} {{.path}} {{.duration}}ms"
```

**Errors with stack traces:**
```logql
{namespace="system"} | json | level="error" | error != ""
```

**Logs in time window:**
```logql
{namespace="system"} | json | level="error" | __timestamp__ > 1633000000000000000
```

**Pattern extraction:**
```logql
{namespace="system"} | json | regexp "user_id=(?P<user>[0-9]+)"
```

**Multi-line log search:**
```logql
{namespace="system"} | json | msg=~"(?s).*panic.*stack trace.*"
```

### vLLM-Specific Queries

**GPU errors:**
```logql
{namespace=~"ai-models|system|kserve"} | json | gpu_error="true"
```

**Model loading events:**
```logql
{namespace=~"ai-models|system|kserve"} | json | loading_status=~"loading|ready|failed"
```

**Model by name:**
```logql
{namespace=~"ai-models|system|kserve"} | json | model_name="llama-2-7b-hf"
```

**CUDA errors:**
```logql
{namespace=~"ai-models|system|kserve"} |~ "CUDA" |~ "error"
```

### Formatting Output

**Custom line format:**
```logql
{namespace="system"} | json | line_format "{{.timestamp}} [{{.level}}] {{.service}}: {{.msg}}"
```

**Extract specific fields:**
```logql
{namespace="system"} | json | line_format "{{.request_id}} {{.duration}}ms"
```

**Pretty JSON output:**
```logql
{namespace="system"} | json | line_format "{{toJson .}}"
```

## kubectl Commands for Log Access

While Grafana and Loki are preferred, kubectl provides direct log access:

### Basic Log Commands

**Tail logs for deployment:**
```bash
kubectl logs -n system deployment/<service-name> -f --tail=100
```

**Get logs from specific pod:**
```bash
kubectl logs -n system <pod-name> -f
```

**Get logs from previous pod (if crashed):**
```bash
kubectl logs -n system <pod-name> --previous
```

**Get logs from specific container (multi-container pod):**
```bash
kubectl logs -n system <pod-name> -c <container-name>
```

### Filtering and Searching

**Search logs for error keyword:**
```bash
kubectl logs -n system deployment/api-router-service --tail=1000 | grep -i error
```

**Show logs with timestamps:**
```bash
kubectl logs -n system deployment/api-router-service --timestamps
```

**Get logs since time:**
```bash
kubectl logs -n system deployment/api-router-service --since=1h
kubectl logs -n system <pod-name> --since=2h
kubectl logs -n system <pod-name> --since-time=2024-01-15T10:00:00Z
```

### Multi-Pod Queries

**Get logs from all pods matching label:**
```bash
kubectl logs -n system -l app=api-router-service -f --tail=50
```

**Get logs from all pods and prefix with pod name:**
```bash
kubectl logs -n system -l app=api-router-service --prefix=true
```

### Advanced Usage

**Stream logs from multiple pods:**
```bash
# Use stern (third-party tool, if available)
stern -n system api-router
```

**Export logs to file:**
```bash
kubectl logs -n system deployment/api-router-service --tail=10000 > api-router-logs.txt
```

**Follow logs with context:**
```bash
kubectl logs -n system -l app=user-org-service -f --tail=100 | grep -A 3 -B 3 "error"
```

## Grafana Dashboard Navigation

The AI-AAS platform includes pre-built dashboards for debugging.

### Accessing Dashboards

1. Navigate to `https://grafana.dev.otherjamesbrown.com`
2. Login with admin credentials (retrieve password from Kubernetes secret)
3. Click **Dashboards** icon in left sidebar (four squares)
4. Browse dashboards or use search

### Key Dashboards for Debugging

#### 1. Service Logs Dashboard

**Purpose:** Monitor log volume, error rates, and view service logs

**Key Panels:**
- **Log Volume by Service**: Identify services with high log output
- **Error Rate**: Real-time error count by service
- **Log Viewer**: Interactive log exploration with filtering
- **Error Breakdown**: Top error messages by frequency

**How to Use:**
1. Open Service Logs dashboard
2. Select service from dropdown variable
3. Adjust time range (top-right)
4. Click on log lines to expand details
5. Click "Explore" to open in Loki with full context

#### 2. Inference Backends Dashboard

**Purpose:** Monitor vLLM model health, GPU errors, loading status

**Key Panels:**
- **Model Loading Status**: Current state of all models
- **GPU Errors**: Count and details of GPU-related errors
- **Model Performance**: Throughput and latency metrics
- **CUDA Events**: CUDA errors and warnings

**How to Use:**
1. Open Inference Backends dashboard
2. Select model from dropdown (if available)
3. Check "Model Loading Status" panel for failures
4. Review "GPU Errors" panel for hardware issues
5. Correlate errors with performance metrics

#### 3. Request Tracing Dashboard

**Purpose:** Visualize request flow, trace correlation, latency breakdown

**Key Panels:**
- **Request Flow Graph**: Service dependency topology
- **Trace Latency Heatmap**: Distribution of request durations
- **Error Traces**: List of failed requests with trace IDs
- **Service Latency Breakdown**: Time spent in each service

**How to Use:**
1. Open Request Tracing dashboard
2. Select time range around incident
3. Click on trace ID in "Error Traces" panel
4. View full trace in Tempo
5. Use "Logs for this span" to see detailed logs

#### 4. Fleet Overview Dashboard

**Purpose:** Executive view of entire AI fleet, model health

**Key Panels:**
- **Total Models Deployed**: Count of active models
- **GPU Utilization**: Fleet-wide GPU usage
- **Total Throughput**: Aggregate tokens/sec
- **Generation Throughput by Model**: Per-model performance

**How to Use:**
- Quick health check of all models
- Identify underperforming models
- Monitor capacity and utilization

### Dashboard Features

**Time Range Selection:**
- Top-right corner: Select preset ranges (Last 5m, 15m, 1h, 6h, 24h, 7d)
- Custom range: Click time range → "Absolute time range"

**Refresh Interval:**
- Top-right corner: Auto-refresh (off, 5s, 10s, 30s, 1m, 5m)

**Variables (Dropdowns):**
- Top of dashboard: Select service, environment, model
- Variables filter all panels dynamically

**Panel Actions:**
- Click panel title → "Explore": Opens query in Explore view
- Click panel title → "View JSON": See raw query
- Click on graph: Drill down to specific time

### Explore Mode

**Loki Exploration:**
1. Click **Explore** (compass icon)
2. Select **Loki** datasource
3. Enter LogQL query or use query builder
4. Click **Run query**
5. Expand log lines for details
6. Click **Tempo** button next to trace_id to view trace

**Tempo Exploration:**
1. Click **Explore** → **Tempo**
2. Enter trace ID or use search
3. View trace timeline with spans
4. Click on span → "Logs for this span" to see related logs
5. Use "Service Graph" to see topology

### Derived Fields (Trace-to-Logs Linking)

Loki datasource is configured with derived fields to extract trace_id:

- **Field Name**: trace_id
- **Regex**: `"trace_id":"([a-f0-9]{32})"`
- **Link**: Opens trace in Tempo

**How it works:**
1. View logs in Grafana
2. If log contains trace_id, a **Tempo** button appears
3. Click button to jump to trace in Tempo

## Trace Correlation Workflow

Distributed tracing links requests across multiple services.

### Understanding Traces

**Trace Components:**
- **Trace**: Complete request journey (unique trace_id)
- **Span**: Single operation (e.g., HTTP request, database query)
- **Parent-child relationship**: Spans nested to show call hierarchy

**Trace Context Propagation:**
- `trace_id`: Unique identifier for entire request
- `span_id`: Unique identifier for specific operation
- `parent_span_id`: Links span to parent operation

### Finding Traces

**From Error Response:**
1. Check HTTP response headers for `X-Trace-Id`
2. Use trace_id to search Tempo

**From Logs:**
1. Query logs with error context:
   ```logql
   {namespace="system"} | json | level="error"
   ```
2. Extract trace_id from log entry
3. Search Tempo with trace_id

**From Time Range:**
1. Grafana → Explore → Tempo → Search
2. Set service name and time range
3. Filter by status (error) or duration

### Analyzing Traces

**View Trace Timeline:**
1. Click on trace in search results
2. See hierarchical span view with timing
3. Identify long-running spans (potential bottlenecks)
4. Look for spans with errors (red color)

**Span Details:**
- Click on span to see attributes
- Check tags: http.method, http.status_code, error
- Review events: exceptions, logs
- View process info: service name, version

**Trace-to-Logs Navigation:**
1. Click on span with error
2. Click "Logs for this span" button
3. Grafana opens Loki with filtered query:
   ```logql
   {namespace="system"} | json | trace_id="<trace_id>" | span_id="<span_id>"
   ```

### Correlating Across Services

**Multi-Service Request:**
1. View trace in Tempo to see all services involved
2. Identify first span with error
3. Use "Logs for this span" to see detailed error message
4. Check parent spans for request context
5. Check child spans for downstream failures

**Example Trace Flow:**
```
api-router-service (200ms)
  ├─ user-org-service (50ms)
  │   └─ database query (30ms)
  └─ analytics-service (150ms) ← ERROR HERE
      └─ redis connection (TIMEOUT)
```

## vLLM / Inference Backend Debugging

vLLM pods have special logging considerations due to mixed log formats.

### Common vLLM Issues

#### 1. Model Loading Failure

**Symptoms:**
- Pod stuck in "ContainerCreating" or "CrashLoopBackOff"
- Model not available in API

**Queries:**
```logql
# Check loading status
{namespace=~"ai-models|system|kserve"} | json | loading_status="failed"

# Look for download errors
{namespace=~"ai-models|system|kserve"} |~ "download" |~ "error"

# Check Hugging Face API errors
{namespace=~"ai-models|system|kserve"} |~ "huggingface" |~ "error"
```

**kubectl Checks:**
```bash
# Pod status
kubectl get pods -n system -l model=<model-name>

# Pod events
kubectl describe pod -n system <model-pod>

# Init container logs (if downloading weights)
kubectl logs -n system <model-pod> -c download-weights
```

#### 2. CUDA / GPU Errors

**Symptoms:**
- GPU unavailable errors
- CUDA out of memory
- Model inference fails

**Queries:**
```logql
# GPU errors
{namespace=~"ai-models|system|kserve"} | json | gpu_error="true"

# CUDA errors
{namespace=~"ai-models|system|kserve"} |~ "CUDA" |~ "error"

# Out of memory errors
{namespace=~"ai-models|system|kserve"} |~ "OOM"
{namespace=~"ai-models|system|kserve"} |~ "out of memory"
```

**GPU Health Checks:**
```bash
# Check GPU nodes
kubectl get nodes -l node.kubernetes.io/gpu=true

# Check GPU allocation
kubectl describe nodes -l node.kubernetes.io/gpu=true | grep -A 10 "Allocated resources"

# Exec into pod and check nvidia-smi
kubectl exec -n system <model-pod> -- nvidia-smi
```

**Grafana Metrics:**
- Open Inference Backends dashboard
- Check GPU Utilization panel
- Check GPU Memory Usage panel

#### 3. Inference Performance Issues

**Symptoms:**
- Slow response times
- High latency
- Low throughput

**Queries:**
```logql
# Slow inference requests
{namespace=~"ai-models|system|kserve"} | json | duration > 5000

# Queue depth issues
{namespace=~"ai-models|system|kserve"} |~ "queue" |~ "full"
```

**Grafana Metrics:**
- Open Per-Model Performance dashboard
- Check Time to First Token (TTFT)
- Check GPU Cache Usage
- Check Request Queue Status

**Performance Metrics:**
```bash
# vLLM metrics endpoint
kubectl port-forward -n system <model-pod> 8000:8000
curl http://localhost:8000/metrics | grep vllm
```

#### 4. Model Configuration Issues

**Symptoms:**
- Invalid model parameters
- Trust_remote_code errors
- Tokenizer errors

**Queries:**
```logql
# Configuration errors
{namespace=~"ai-models|system|kserve"} |~ "config" |~ "error"

# Trust_remote_code issues
{namespace=~"ai-models|system|kserve"} |~ "trust_remote_code"

# Tokenizer errors
{namespace=~"ai-models|system|kserve"} |~ "tokenizer" |~ "error"
```

**Check Model Deployment:**
```bash
# KServe InferenceService
kubectl get inferenceservice -n system <model-name>

# Model ConfigMap
kubectl get configmap -n system <model-name>-config -o yaml
```

### vLLM Log Formats

**JSON Format (Structured):**
```json
{
  "timestamp": "2024-01-15T10:30:45Z",
  "level": "info",
  "message": "Model loaded successfully",
  "model_name": "llama-2-7b-hf",
  "loading_status": "ready",
  "gpu_id": 0
}
```

**Plain Text (Unstructured):**
```
Loading model weights from /models/llama-2-7b-hf
Model llama-2-7b-hf loaded successfully
CUDA device 0: NVIDIA GeForce RTX 4090
```

**Mixed Format Queries:**
```logql
# Use |~ for plain text search
{namespace=~"ai-models|system|kserve"} |~ "Model.*loaded"

# Use | json for structured logs
{namespace=~"ai-models|system|kserve"} | json | loading_status="ready"

# Combine both
{namespace=~"ai-models|system|kserve"} | json | level="error" | model_name=~".*llama.*"
```

### vLLM Debugging Checklist

- [ ] Check pod status and events
- [ ] Verify GPU allocation and availability
- [ ] Check model loading logs
- [ ] Verify model configuration (trust_remote_code, tensor_parallel_size)
- [ ] Check CUDA errors in logs
- [ ] Monitor GPU memory usage
- [ ] Check inference performance metrics (TTFT, throughput)
- [ ] Verify KServe InferenceService status
- [ ] Check Inference Backends dashboard in Grafana

## Frontend Error Investigation (Sentry)

Frontend errors are tracked in Sentry with session replay capabilities.

### Accessing Sentry

**Sentry Configuration:**
- Location: `web/portal/src/lib/sentry.ts`
- DSN: Configured via `VITE_SENTRY_DSN` environment variable
- Features: Error capture, session replay, performance monitoring

**Access Sentry Dashboard:**
1. Get Sentry DSN from environment configuration
2. Extract organization and project from DSN
3. Login to Sentry dashboard (sentry.io or self-hosted)

### Finding Frontend Errors

**From User Report:**
1. User provides Sentry event ID (shown in ErrorBoundary)
2. Search Sentry by event ID
3. View error details and session replay

**From Error List:**
1. Navigate to Issues in Sentry
2. Filter by:
   - Environment (development, staging, production)
   - Time range
   - Error type or message
3. Sort by frequency or recency

### Analyzing Frontend Errors

**Error Details:**
- **Stack Trace**: JavaScript call stack at error
- **Breadcrumbs**: User actions leading to error
- **Context**: User info, browser, OS, session ID
- **Tags**: Custom tags (component, feature, user_id)

**Session Replay:**
1. Click "Replay" tab in error details
2. Watch video of user session
3. See console logs and network requests
4. Identify exact user actions causing error

**Breadcrumbs:**
- Navigation: Route changes
- User interactions: Clicks, form submissions
- Console logs: Logger library output
- Network: API requests and responses

### Correlating with Backend

**Get Backend trace_id:**
1. Check error context in Sentry for `trace_id` tag
2. Frontend logger attaches trace_id from sessionStorage
3. If available, query backend logs:
   ```logql
   {namespace="system"} | json | trace_id="<trace_id_from_sentry>"
   ```

**Frontend-to-Backend Flow:**
1. User action triggers API request
2. Frontend logger creates/retrieves trace_id
3. API request includes trace_id in header
4. Backend logs include trace_id
5. Error captured by Sentry includes trace_id

**End-to-End Debugging:**
1. User reports error → Get Sentry event ID
2. View error in Sentry → Extract trace_id
3. Query backend logs with trace_id
4. View full trace in Tempo
5. Identify if error originated in frontend or backend

### Common Frontend Issues

#### 1. React Rendering Errors

**Symptoms:** Component fails to render, ErrorBoundary triggered

**Sentry Analysis:**
- Check stack trace for component name
- Review breadcrumbs for props/state changes
- Watch session replay for user actions

#### 2. API Request Failures

**Symptoms:** Network errors, 4xx/5xx responses, timeouts

**Sentry Analysis:**
- Check network breadcrumbs for request details
- Review response status and error message
- Correlate with backend logs using trace_id

#### 3. State Management Errors

**Symptoms:** Invalid state, Redux errors, context issues

**Sentry Analysis:**
- Check breadcrumbs for state mutations
- Review error message for state details
- Watch session replay for interaction flow

### Sentry Configuration

**Sensitive Data Scrubbing:**
```typescript
// web/portal/src/lib/sentry.ts
beforeSend(event, hint) {
  // Remove Authorization headers
  if (event.request?.headers) {
    delete event.request.headers['Authorization'];
  }

  // Scrub password fields
  if (event.request?.data) {
    scrubPasswords(event.request.data);
  }

  return event;
}
```

**Session Replay Settings:**
- Sample Rate: 100% on errors
- Mask All Text: false (enable for PII compliance)
- Block All Media: false

**Performance Monitoring:**
- Sample Rate: 10% (production)
- Traces Sample Rate: 10%

### Frontend Debugging Checklist

- [ ] Get Sentry event ID from user or logs
- [ ] View error details in Sentry dashboard
- [ ] Check stack trace for error location
- [ ] Review session replay (if available)
- [ ] Analyze breadcrumbs for user actions
- [ ] Extract trace_id from Sentry context
- [ ] Correlate with backend logs using trace_id
- [ ] Verify API endpoints and responses
- [ ] Check browser and OS compatibility
- [ ] Review console logs in session replay

## Log Field Reference

The platform uses structured JSON logs with standardized fields.

### Common Log Fields

| Field | Description | Example |
|-------|-------------|---------|
| `timestamp` | ISO 8601 timestamp | `2024-01-15T10:30:45Z` |
| `level` | Log level | `info`, `warn`, `error`, `debug` |
| `msg` | Log message | `request_completed` |
| `service` | Service name | `api-router-service` |
| `trace_id` | OpenTelemetry trace ID | `4bf92f3577b34da6a3ce929d0e0e4736` |
| `span_id` | OpenTelemetry span ID | `00f067aa0ba902b7` |
| `request_id` | Request correlation ID | `req-abc123` |
| `method` | HTTP method | `POST`, `GET` |
| `path` | Request path | `/v1/inference` |
| `status` | HTTP status code | `200`, `500` |
| `duration` | Request duration | `150` (milliseconds) |
| `error` | Error message | `connection refused` |
| `stack` | Stack trace | `panic: runtime error...` |

### Service-Specific Fields

**User Org Service:**
- `user_id`: User identifier
- `org_id`: Organization identifier
- `role`: User role (admin, user)

**API Router Service:**
- `backend`: Backend service name
- `model`: Model name
- `endpoint`: Backend endpoint URL

**Analytics Service:**
- `event_type`: Analytics event type
- `user_id`: User identifier
- `session_id`: Session identifier

### vLLM-Specific Fields

| Field | Description | Example |
|-------|-------------|---------|
| `model_name` | Model identifier | `llama-2-7b-hf` |
| `gpu_id` | GPU index | `0`, `1` |
| `loading_status` | Model loading state | `loading`, `ready`, `failed` |
| `gpu_error` | GPU error flag | `true`, `false` |
| `tokens_generated` | Number of tokens | `128` |
| `inference_time` | Inference duration (ms) | `850` |

## Automated Debug Script

Use this script to quickly gather debug information for a service:

```bash
#!/bin/bash
# debug-service.sh - Quick service debugging
# Usage: ./debug-service.sh <service-name> [namespace] [kubeconfig]

SERVICE=${1:-api-router-service}
NAMESPACE=${2:-system}
KUBECONFIG_PATH=${3:-secrets/kubeconfigs/kubeconfig-development.yaml}

export KUBECONFIG=$KUBECONFIG_PATH

echo "========================================="
echo "Debug Report for: $SERVICE"
echo "Namespace: $NAMESPACE"
echo "Time: $(date)"
echo "========================================="
echo ""

echo "=== Pod Status ==="
kubectl get pods -n $NAMESPACE -l app=$SERVICE -o wide
echo ""

echo "=== Recent Events ==="
POD_NAME=$(kubectl get pods -n $NAMESPACE -l app=$SERVICE -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -n "$POD_NAME" ]; then
  kubectl get events -n $NAMESPACE \
    --field-selector involvedObject.name=$POD_NAME \
    --sort-by='.lastTimestamp' 2>/dev/null | tail -10
else
  echo "No pods found for service: $SERVICE"
fi
echo ""

echo "=== Recent Logs (last 50 lines) ==="
if [ -n "$POD_NAME" ]; then
  kubectl logs -n $NAMESPACE -l app=$SERVICE --tail=50
else
  echo "No pods found for service: $SERVICE"
fi
echo ""

echo "=== Resource Usage ==="
kubectl top pod -n $NAMESPACE -l app=$SERVICE 2>/dev/null || echo "Metrics not available"
echo ""

echo "=== Service Endpoint ==="
kubectl get svc -n $NAMESPACE -l app=$SERVICE -o wide
echo ""

echo "========================================="
echo "Debug report complete"
echo "========================================="
```

**Save and Use:**
```bash
# Save script
cat > debug-service.sh << 'EOF'
[paste script above]
EOF

# Make executable
chmod +x debug-service.sh

# Run for specific service
./debug-service.sh api-router-service system

# Run for vLLM model
./debug-service.sh llama-2-7b ai-models
```

## Troubleshooting Tips

### No Logs Appearing in Grafana

**Check Promtail:**
```bash
# Verify Promtail is running
kubectl get pods -n system -l app=promtail

# Check Promtail logs
kubectl logs -n system -l app=promtail --tail=50

# Verify Promtail configuration
kubectl get configmap -n system promtail-config -o yaml
```

**Check Loki:**
```bash
# Verify Loki is healthy
kubectl get pods -n system -l app=loki

# Check Loki logs
kubectl logs -n system -l app=loki --tail=50

# Test Loki API
curl -k https://loki.dev.otherjamesbrown.com/ready
```

**Check Pod Annotations:**
```bash
# Verify logging is enabled for pod
kubectl get pods -n system -o yaml | grep -A 3 "logging.enabled"

# Should see: logging.enabled: "true"
```

### Logs Not in JSON Format

**Issue:** Logs appear as plain text, field extraction not working

**Solution:**
- Verify service is using `shared/go/logging` package
- Check `LOG_FORMAT` environment variable is not set to "text"
- Review service logs for JSON output
- Update service to use structured logging

```bash
# Check service environment variables
kubectl get deployment -n system <service> -o jsonpath='{.spec.template.spec.containers[0].env}'
```

### Cannot Find Specific Request

**Expand Time Range:**
- Grafana: Adjust time picker (top-right)
- Logs retained for 14 days (development)

**Verify Request ID:**
- Check if request ID is correct
- Try searching by user_id or org_id instead

**Check Service Restart:**
```bash
# Check pod age
kubectl get pods -n system -l app=<service> -o wide

# If pod recently restarted, logs before restart are in Loki only
```

### Grafana Dashboard Not Loading

**Check Ingress:**
```bash
# Verify ingress exists
kubectl get ingress -n system grafana

# Check ingress status
kubectl describe ingress -n system grafana
```

**Check TLS Certificate:**
```bash
# Verify certificate
kubectl get certificate -n system grafana-tls

# Check certificate status
kubectl describe certificate -n system grafana-tls
```

**Check Grafana Pods:**
```bash
# Verify Grafana is running
kubectl get pods -n system -l app=grafana

# Check Grafana logs
kubectl logs -n system -l app=grafana --tail=50
```

**Port-Forward (Fallback):**
```bash
# Direct access to Grafana
kubectl port-forward -n system svc/grafana 3000:3000

# Access at: http://localhost:3000
```

### High Cardinality Warnings

**Issue:** Loki complains about too many unique label combinations

**Solution:**
- Avoid using high-cardinality labels (request IDs, user IDs)
- Use field extraction instead of labels
- Review Promtail relabeling configuration

```bash
# Check Loki ingester metrics
kubectl port-forward -n system svc/loki 3100:3100
curl http://localhost:3100/metrics | grep loki_ingester_streams
```

## Integration with CI/CD

For automated testing and debugging in CI pipelines:

```yaml
# .github/workflows/debug-on-failure.yml
name: Debug on Test Failure

on:
  workflow_run:
    workflows: ["CI"]
    types:
      - completed

jobs:
  debug:
    runs-on: ubuntu-latest
    if: ${{ github.event.workflow_run.conclusion == 'failure' }}
    steps:
      - name: Query Loki for Errors
        run: |
          # Wait for logs to be ingested
          sleep 30

          # Query for errors in test time window
          ERROR_COUNT=$(curl -s -k "https://loki.dev.otherjamesbrown.com/loki/api/v1/query" \
            --data-urlencode 'query=count_over_time({namespace="system"} |= "error" [5m])' \
            | jq '.data.result[0].value[1] // "0"')

          echo "Found $ERROR_COUNT errors in logs"

          if [ "$ERROR_COUNT" -gt "0" ]; then
            echo "Fetching error logs..."
            curl -s -k "https://loki.dev.otherjamesbrown.com/loki/api/v1/query_range" \
              --data-urlencode 'query={namespace="system"} |= "error"' \
              --data-urlencode 'limit=20' | jq
          fi

      - name: Create Issue on Failure
        if: failure()
        run: |
          # Create GitHub issue or Slack notification
          echo "CI failure detected. Check Grafana for details."
```

## Related Documentation

### Architecture & Design
- [Observability Architecture](../architecture/observability-architecture.md) - Detailed architecture, data flows, components
- [Infrastructure Overview](../platform/infrastructure-overview.md) - Overall platform architecture

### Operations & Access
- [Environment Access Guide](../platform/environment-access.md) - Credentials, endpoints, kubeconfig
- [Observability Guide](../platform/observability-guide.md) - Observability stack overview
- [vLLM Observability](../platform/vllm-observability.md) - Inference backend monitoring

### Deployment & Troubleshooting
- [Deploy to Environments](./deploy-to-environments.md) - GitOps deployment workflow
- [Infrastructure Troubleshooting](./infrastructure-troubleshooting.md) - General infrastructure issues
- [Partial Failure Remediation](./partial-failure-remediation.md) - Handling deployment failures

### Specifications
- [Spec 024: Logging & Observability](../../specs/024-logging-observability-improvements/spec.md) - Feature specification
- [Spec 024: Architecture](../../specs/024-logging-observability-improvements/architecture.md) - Technical design

## Support

For additional help:

1. **Check Prometheus targets**: Verify metrics scraping is working
2. **Review pod logs**: Use kubectl logs for real-time debugging
3. **Verify metrics endpoint**: Test service /metrics endpoint
4. **Consult team documentation**: Check team wiki or internal docs
5. **Escalate to on-call**: If critical issue, page on-call engineer

---

**Last Updated**: 2025-12-15
**Version**: 2.1
**Maintained By**: AI-AAS Platform Team
**Spec**: 024-logging-observability-improvements
