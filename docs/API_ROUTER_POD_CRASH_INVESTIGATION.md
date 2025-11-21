# API Router Service Pod Crash Investigation

**Date:** 2025-11-20
**Issue:** Pods crash immediately on startup with no logs after authentication fix was deployed
**Status:** ROOT CAUSE IDENTIFIED

## Summary

After fixing the authentication context bug and deploying via CI, new pods started crashing immediately on startup without producing any logs. Investigation revealed that the issue is related to telemetry initialization blocking due to misconfigured OTLP endpoint.

## Symptoms

1. **Pods Status**: CrashLoopBackOff
2. **Liveness/Readiness Probes**: Failing with "connection refused" on port 8080
3. **Container Logs**: Completely empty - zero logs produced
4. **Process Status**: Binary runs (PID 1 exists) but HTTP server never starts listening
5. **Local Docker Build**: Exhibits same behavior - no logs, process runs but doesn't serve HTTP

## Investigation Steps

### 1. Initial Hypothesis: Docker Image Build Issue
- **Action**: Built Docker image locally using same Dockerfile
- **Result**: Build succeeded, but container exhibited same crash behavior
- **Conclusion**: Not a build issue - runtime issue

### 2. Container Runtime Testing
- **Action**: Ran container locally with minimal environment variables
- **Result**: Binary ran but produced no output, connection reset when accessing port 9090
- **Finding**: Process exists but HTTP server not starting

### 3. Binary Execution Testing
- **Action**: Ran binary directly via `go run` with timeout
- **Result**: Exit code 124 (timeout) - binary ran for 2 seconds without any output
- **Finding**: Binary doesn't crash, but hangs silently without producing logs

### 4. Code Analysis
- **Finding 1**: `main.go` line 77-90 shows config.MustLoad() followed by telemetry.MustInit()
- **Finding 2**: Logger is created INSIDE telemetry.MustInit() - no logger exists before this
- **Finding 3**: If telemetry init fails/hangs, no logs can be produced
- **Finding 4**: MustInit() writes errors to stderr and calls os.Exit(1) on failure

### 5. Environment Variable Analysis
```json
{
  "name": "OTEL_EXPORTER_OTLP_ENDPOINT",
  "value": "localhost:4318"
},
```

- **Critical Finding**: OTLP endpoint points to `localhost:4318`
- **Problem**: Inside container, localhost has no OTLP collector listening
- **Effect**: Telemetry initialization blocks or fails before logger is created

## Root Cause

**Telemetry initialization blocking due to OTLP connection attempt to non-existent localhost:4318**

The initialization sequence is:
1. `config.MustLoad()` - succeeds (all config fields have defaults)
2. `telemetry.MustInit()` - attempts to connect to OTLP exporter at localhost:4318
   - Connection attempt blocks or times out
   - Logger not yet available so no error logs produced
   - If it fails with MustInit(), it writes to stderr and exits with code 1
   - Container restarts, cycle repeats

## Code References

### services/api-router-service/cmd/router/main.go:73-100
```go
func main() {
    ctx := context.Background()

    // Load configuration
    cfg := config.MustLoad()

    // Initialize telemetry
    telemetryCfg := telemetry.Config{
        ServiceName: cfg.ServiceName,
        Environment: cfg.Environment,
        Endpoint:    cfg.TelemetryEndpoint,  // <- localhost:4318
        Protocol:    cfg.TelemetryProtocol,
        Headers:     map[string]string{},
        Insecure:    cfg.TelemetryInsecure,
        LogLevel:    cfg.LogLevel,
    }

    tel := telemetry.MustInit(ctx, telemetryCfg)  // <- BLOCKS HERE
    // ...
    logger := tel.Logger  // <- Never reached if init fails
```

### services/api-router-service/internal/telemetry/telemetry.go:90-97
```go
func MustInit(ctx context.Context, cfg Config) *Telemetry {
    telemetry, err := Init(ctx, cfg)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to initialize telemetry: %v\n", err)
        os.Exit(1)  // <- Silent exit, container restarts
    }
    return telemetry
}
```

## Solution Options

### Option 1: Remove OTLP Endpoint (Quick Fix)
Remove or comment out the `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable in the deployment manifest to use the default value.

**Location**: Helm chart or Kubernetes deployment manifest
**Change**: Remove or set to empty string (will use default)

### Option 2: Point to Actual OTLP Collector
Deploy an OTLP collector service and update the endpoint to point to it.

**Example**: `otel-collector.monitoring.svc.cluster.local:4317`

### Option 3: Make Telemetry Init More Resilient
Update `telemetry.Init()` to not block on OTLP connection, or add connection timeout with fallback.

**Trade-off**: Requires code changes, more robust long-term solution

## Recommended Action

**Immediate**: Add connection timeouts to OTLP client in `shared/go/observability/otel.go`
**Long-term**: Make telemetry initialization non-blocking or add a startup timeout configuration

## Update: Further Investigation

After attempting to fix by:
1. Removing OTEL_EXPORTER_OTLP_ENDPOINT (failed - returns "telemetry endpoint required" error)
2. Setting endpoint to non-existent service (failed - hangs during connection attempt)
3. Increasing liveness probe initialDelaySeconds to 60s (failed - initialization takes >60s)

**Root Cause Confirmed**: The `shared/go/observability/otel.go` buildClient() or otlptrace.New() calls are blocking without timeout when trying to connect to unreachable OTLP endpoints. Even though the code has fallback logic (line 80), the connection attempts take too long (>60 seconds) before timing out.

**Required Fix**: Add connection timeouts to the gRPC/HTTP client configuration in `buildClient()` function

## Related Issues

- **Authentication Fix**: The authentication fix (commit 58cb94d9) is correct and working
- **Routing Policy**: Successfully created in etcd, policy is ready
- **etcd Service**: Deployed and operational

Once the telemetry configuration is fixed, the service should start normally and be able to:
1. Authenticate requests using test API keys
2. Load routing policy from etcd
3. Route requests to vLLM backend

## Testing After Fix

After applying the fix:
1. Deploy updated manifest
2. Verify pod starts successfully: `kubectl get pods -n development`
3. Check logs appear: `kubectl logs -n development <pod-name>`
4. Test health endpoint: `curl http://localhost:8080/v1/status/healthz`
5. Test inference request with test API key

## Lessons Learned

1. **Initialization Order Matters**: Critical services (logging) should initialize before optional services (telemetry exporters)
2. **Fail-Fast vs Resilient**: MustInit() pattern is good for required services, but optional exporters should fail gracefully
3. **Container Debugging**: When containers produce no logs, issue is likely in early initialization before logger creation
4. **Environment Variable Pitfalls**: Empty string values in env vars don't trigger defaults in envconfig
