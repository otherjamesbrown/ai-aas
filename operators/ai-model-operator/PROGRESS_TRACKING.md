# Download Progress Tracking Implementation

## Overview

This implementation adds download progress tracking to the AIModel operator, populating the `status.Progress` field during the `Downloading` phase.

## Implementation Details

### 1. Progress Parser (`parseDownloadProgress`)

**Location**: `controllers/aimodel_controller.go`

Parses common download progress patterns from pod logs:

- **Percentage**: `45%`, `60%|`, `45%|████▌`
- **Downloaded/Total**: `(1.2GB/2.4GB)`, `2.25G/5.0G`, `(1073741824/2147483648)`
- **ETA**: `ETA: 5m30s`, `[02:30<01:40]`, `<00:45`

**Supported Log Formats**:
- HuggingFace Hub CLI: `model.safetensors: 45%|████▌     | 2.25G/5.0G [00:30<00:45, 61.2MB/s]`
- tqdm progress bars: `Fetching 15 files: 60%|████████  | 9/15 [02:30<01:40]`
- Simple percentage: `Downloading: 50% (1.2GB/2.4GB) ETA: 5m30s`

### 2. Pod Log Reader (`getPodLogs`)

**Location**: `controllers/aimodel_controller.go`

Reads the last 50 lines from the storage-initializer init container logs.

**Error Handling**:
- Returns empty string on error (non-critical to reconciliation)
- Logs errors at debug level (V(1))
- Gracefully handles missing Clientset (returns empty)

### 3. Integration with Reconciliation Loop

**Location**: `controllers/aimodel_controller.go:2305-2325` in `determineGranularPhase`

When the storage-initializer init container is running:
1. Reads recent logs from the container
2. Parses progress information
3. Updates `aiModel.Status.Progress` with extracted data
4. Updates status message with percentage info

**Example Status Messages**:
- `"Downloading: 45%"` - When percentage is available
- `"Downloading: 45% (2.25G / 5.0G)"` - When both percentage and bytes are available
- `"Downloading model artifacts"` - Fallback when no progress info available

### 4. Reconciler Configuration

**Location**: `main.go:129-142`

Added:
- `Clientset` field to reconciler for pod log reading
- `Config` field for REST configuration
- Initialization of Kubernetes clientset from REST config

## Architecture

```
Reconcile Loop
    ↓
determineGranularPhase()
    ↓
storage-initializer is Running?
    ↓
getPodLogs(pod, "storage-initializer", 50)
    ↓
parseDownloadProgress(logs)
    ↓
Update aiModel.Status.Progress
    {
      Percentage: 45,
      Downloaded: "2.25G / 5.0G",
      ETA: "00:45"
    }
```

## Testing

**Test File**: `controllers/aimodel_controller_test.go`

**Test Coverage**:
- 14 test cases covering various log formats
- Edge cases (0%, 100%, invalid percentages)
- Multiple log line parsing
- Missing/incomplete progress information

**Run Tests**:
```bash
cd operators/ai-model-operator
go test ./controllers -run TestParseDownloadProgress -v
```

## Limitations

### 1. Log Format Dependency

The implementation relies on parsing log output, which varies by storage initializer implementation:

- **KServe storage-initializer**: May not emit detailed progress
- **Custom storage-initializer**: Would need to emit compatible log formats

### 2. No Direct Storage Initializer Control

We don't control the storage-initializer container (it's part of KServe), so we can't:
- Force it to emit progress information
- Guarantee a specific log format
- Ensure progress updates at regular intervals

### 3. Best-Effort Approach

Progress tracking is **best-effort**:
- If logs don't contain progress info, progress remains unpopulated
- If log reading fails, reconciliation continues normally
- Errors are logged at debug level to avoid noise

## Future Enhancements

### Option 1: Custom Progress Monitor Sidecar

Create a sidecar container that:
1. Monitors download directory size
2. Compares to expected model size (from spec)
3. Reports progress to Admin API
4. Operator fetches progress from Admin API

**Pros**:
- Independent of log format
- More reliable progress tracking
- Can calculate download speed

**Cons**:
- Requires Admin API changes
- Adds complexity to pod spec
- Need to know expected model size upfront

### Option 2: Custom Storage Initializer

Replace KServe's storage-initializer with our own that:
- Emits standardized progress logs
- Writes progress to shared volume
- Uses well-defined format

**Pros**:
- Full control over progress reporting
- Can optimize for our use case

**Cons**:
- Maintenance burden
- Need to keep up with KServe changes
- More complex deployment

### Option 3: Status API

Add a status endpoint to storage-initializer container:
- HTTP endpoint that returns JSON progress
- Operator polls endpoint during download

**Pros**:
- Clean API contract
- No log parsing required

**Cons**:
- Requires modifying storage-initializer
- Network overhead

## Deployment

No additional deployment steps required beyond normal operator upgrade:

1. The Clientset is initialized automatically in `main.go`
2. Progress tracking activates when logs contain progress information
3. Falls back gracefully when logs don't contain progress

## Monitoring

**Operator Logs** (when progress tracking fails):
```
V(1) Failed to get pod logs for progress tracking: <error>
V(1) Failed to read pod logs for progress tracking: <error>
```

**AIModel Status** (when progress tracking succeeds):
```yaml
status:
  phase: Downloading
  message: "Downloading: 45% (2.25G / 5.0G)"
  progress:
    percentage: 45
    downloaded: "2.25G / 5.0G"
    eta: "00:45"
```

## Related Issues

- Epic: aas-j1q4 "Improve Model Deployment Observability & Operator Experience"
- Bead: aas-oag6x "Add download progress tracking to AIModel status"
- Dependency: aas-e4mnd "Add granular deployment phases to AIModel status"
