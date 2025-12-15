# Data Model: AIModel Custom Resource (CRD)

This document defines the data structure for the `AIModel` Custom Resource, which is the primary interface for the GitOps-based AI model management feature.

## Entity: AIModel

The `AIModel` entity represents a single AI model and its desired state in the cluster.

### Spec (Desired State)

| Field | Type | Description | Required | Example |
|---|---|---|---|---|
| `source` | `string` | The URL to the model artifacts, typically a HuggingFace repository URL. | Yes | `"meta-llama/Llama-2-7b-chat-hf"` |
| `hfTokenSecretName` | `string` | The name of the Kubernetes Secret containing the HuggingFace token. | No | `"huggingface-token"` |
| `enabled` | `boolean` | A flag to enable or disable the model. If false, the serving instance is torn down. | No (defaults to `true`) | `true` |
| `serving` | `ServingConfig` | Configuration for the model serving runtime. | Yes | |
| `hardware` | `HardwareConfig` | Configuration for hardware scheduling. | No | |

### ServingConfig (Nested)

| Field | Type | Description | Required | Example |
|---|---|---|---|---|
| `engine` | `string` | The inference engine to use (e.g., `vllm`, `triton`). | Yes | `"vllm"` |
| `image` | `string` | (Optional) Override the default container image for the serving engine. | No | `"nvcr.io/nvidia/tritonserver:23.10-py3"` |
| `parameters` | `map[string]string` | Engine-specific parameters, passed as environment variables or CLI arguments. | No | `{"QUANTIZATION": "GPTQ"}` |
| `minReplicas` | `integer` | The minimum number of replicas for the serving instance. `0` enables scale-to-zero. | No (defaults to `1`) | `1` |

### HardwareConfig (Nested)

| Field | Type | Description | Required | Example |
|---|---|---|---|---|
| `nodeSelector` | `map[string]string` | K8s node selector for targeting specific hardware. | No | `{"accelerator": "nvidia-a100"}` |
| `tolerations` | `[]Toleration` | K8s tolerations for scheduling on tainted nodes. | No | |

### Status (Observed State)

| Field | Type | Description |
|---|---|---|
| `conditions` | `[]Condition` | A list of conditions representing the resource's state (e.g., `Type: Ready`, `Status: True`). |
| `phase` | `string` | A high-level summary of the current state (e.g., `Downloading`, `Ready`, `Failed`). |
| `inferenceEndpoint` | `string` | The URL of the deployed model's inference endpoint, once it is ready. |
| `lastFailureMessage`| `string` | If the last reconciliation failed, this field contains the error message. |
