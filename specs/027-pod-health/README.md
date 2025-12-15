# Spec 027: CLI Pod Health Command

## Overview

Add `ai-aas-cli pod health` command to retrieve model pod health information via the Admin API, enabling operators to quickly diagnose infrastructure issues.

## Motivation

Currently, diagnosing pod health issues requires direct `kubectl` access and knowledge of Kubernetes internals. This feature exposes pod health through the CLI, providing:

- Quick visibility into model pod status
- Restart counts and failure reasons
- Resource usage and node placement
- Actionable next steps for troubleshooting

## Scope

- **Pods**: Model pods only (vLLM/InferenceService pods) by default
- **Command**: `ai-aas-cli pod health` - new top-level command group
- **Display**: Full health details (restarts, container states, resources, node info)

---

## API Specification

### Endpoint: `GET /v1/pods/health`

**Authentication**: Required (Bearer token)

**Query Parameters:**

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `namespace` | string | "system" | Kubernetes namespace to query |
| `model_name` | string | "" | Filter by model name (InferenceService name) |
| `environment` | string | "" | Filter by environment label |
| `include_healthy` | bool | true | Include healthy pods in results |

**Response (200 OK):**

```json
{
  "pods": [
    {
      "name": "gpt-oss-20b-predictor-00001-deployment-abc123",
      "namespace": "system",
      "model_name": "gpt-oss-20b",
      "inferenceservice": "gpt-oss-20b-development",
      "phase": "Running",
      "ready": true,
      "node": "lke-pool-abc123",
      "restart_count": 2,
      "last_restart_time": "2025-12-14T10:30:00Z",
      "last_termination": {
        "reason": "OOMKilled",
        "exit_code": 137,
        "message": "Container exceeded memory limit",
        "started_at": "2025-12-14T10:00:00Z",
        "finished_at": "2025-12-14T10:30:00Z"
      },
      "containers": [
        {
          "name": "kserve-container",
          "state": "running",
          "ready": true,
          "restart_count": 2,
          "resources": {
            "requests": {"cpu": "4", "memory": "32Gi", "nvidia.com/gpu": "1"},
            "limits": {"cpu": "8", "memory": "64Gi", "nvidia.com/gpu": "1"}
          }
        }
      ],
      "conditions": [
        {"type": "Ready", "status": "True", "reason": "", "message": ""},
        {"type": "ContainersReady", "status": "True", "reason": "", "message": ""}
      ],
      "age_seconds": 86400,
      "created_at": "2025-12-13T10:30:00Z"
    }
  ],
  "summary": {
    "total_pods": 3,
    "healthy_pods": 2,
    "unhealthy_pods": 1,
    "total_restarts": 5,
    "pods_with_restarts": 2
  }
}
```

**Error Responses:**

| Status | Description |
|--------|-------------|
| 401 | Unauthorized - missing or invalid API key |
| 500 | Internal error - Kubernetes client failure |

---

## CLI Specification

### Command: `ai-aas-cli pod health`

**Flags:**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--namespace` | `-n` | string | "system" | Kubernetes namespace to query |
| `--model` | `-m` | string | "" | Filter by model name |
| `--environment` | `-e` | string | "" | Filter by environment |
| `--unhealthy-only` | | bool | false | Show only unhealthy pods |
| `--details` | | bool | false | Show detailed container info |
| `--format` | `-f` | string | "table" | Output format (table, json) |

**Examples:**

```bash
# Show all model pod health (default namespace: system)
ai-aas-cli pod health

# Show health for a specific model
ai-aas-cli pod health --model gpt-oss-20b

# Show only unhealthy pods
ai-aas-cli pod health --unhealthy-only

# Show detailed container information
ai-aas-cli pod health --details

# Filter by environment
ai-aas-cli pod health --environment development

# Output as JSON
ai-aas-cli pod health -f json
```

**Table Output:**

```
Pod Health Summary
==================
Total: 3 | Healthy: 2 | Unhealthy: 1 | Total Restarts: 5

POD                                         MODEL         STATUS    READY  RESTARTS      AGE   NODE
gpt-oss-20b-predictor-00001-deployment-...  gpt-oss-20b   Running   Yes    0             2d    lke-pool-abc
mistral-7b-predictor-00001-deployment-...   mistral-7b    Running   Yes    2 (3h ago)    5d    lke-pool-def
llama-3-8b-predictor-00001-deployment-...   llama-3-8b    NotReady  No     3 (10m ago)   1h    lke-pool-ghi

Next steps:
  ai-aas-cli model troubleshoot logs <model>    # View pod logs
  ai-aas-cli model troubleshoot events <model>  # Check K8s events
```

**Detailed Output (with `--details`):**

```
POD                                         MODEL         STATUS    READY  RESTARTS      AGE   NODE          LAST TERM
gpt-oss-20b-predictor-00001-deployment-...  gpt-oss-20b   Running   Yes    0             2d    lke-pool-abc  -
llama-3-8b-predictor-00001-deployment-...   llama-3-8b    NotReady  No     3 (10m ago)   1h    lke-pool-ghi  OOMKilled (exit 137)
```

---

## Implementation

### Files to Create

**Admin API (`services/admin-api-service/`):**

| File | Purpose |
|------|---------|
| `internal/handlers/pods/types.go` | Response types (PodHealth, HealthSummary, ContainerHealth, etc.) |
| `internal/handlers/pods/handler.go` | HTTP handler for GET /v1/pods/health |
| `internal/service/pods.go` | Business logic service |

**CLI (`services/ai-aas-cli/`):**

| File | Purpose |
|------|---------|
| `internal/pod/client.go` | API client for pod health endpoint |
| `cmd/pod/pod.go` | Parent pod command group |
| `cmd/pod/health.go` | Health subcommand implementation |

**RBAC (`services/admin-api-service/deployments/helm/`):**

| File | Purpose |
|------|---------|
| `admin-api-service/templates/rbac.yaml` | ClusterRole for pod access |

### Files to Modify

| File | Changes |
|------|---------|
| `services/admin-api-service/internal/kubernetes/client.go` | Add ListModelPodHealth() method |
| `services/admin-api-service/internal/api/router.go` | Register /v1/pods/health route |
| `services/ai-aas-cli/cmd/ai-aas-cli/root.go` | Register pod command group |
| `services/admin-api-service/deployments/helm/admin-api-service/values.yaml` | Add rbac.create setting |

### RBAC Requirements

The Admin API service account requires:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: admin-api-service-pod-reader
rules:
  - apiGroups: [""]
    resources: ["pods"]
    verbs: ["get", "list", "watch"]
  - apiGroups: ["serving.kserve.io"]
    resources: ["inferenceservices"]
    verbs: ["get", "list", "watch"]
```

---

## Testing

### Unit Tests

- Handler tests with mock K8s client
- Service tests with various pod states
- CLI output formatting tests

### Integration Tests

- API endpoint with test cluster
- CLI command with mock API server

### Manual E2E Tests

1. Deploy model with known restart behavior
2. Run `ai-aas-cli pod health` and verify output
3. Test filters (--model, --unhealthy-only)
4. Verify JSON output matches schema

---

## Implementation Sequence

1. Admin API - K8s Client extension
2. Admin API - Types and Handler
3. Admin API - Service layer
4. Admin API - Router integration
5. RBAC configuration
6. CLI - Client
7. CLI - Commands
8. CLI - Root integration
9. Testing

---

## Related

- `ai-aas-cli model troubleshoot logs` - View pod logs
- `ai-aas-cli model troubleshoot events` - View Kubernetes events
- `ai-aas-cli status` - Platform health overview
