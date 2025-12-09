# Product Requirements Document: GitOps AI Model Management

**Feature ID**: `023-model-gitops`
**Status**: Draft
**Owner**: Platform Engineering

## 1. Overview

This feature introduces a **GitOps-based workflow** for managing the lifecycle of AI models on the AI-AAS platform. By introducing a Kubernetes Operator (`ai-model-operator`) and a Custom Resource Definition (`AIModel`), we shift from an imperative, CLI-driven model registration process to a declarative, version-controlled approach.

## 2. Problem Statement

Currently, deploying a new model involves multiple disjointed steps:
1.  Manually running `admin-cli` to register the model in a Postgres database.
2.  Manually or script-based downloading of model weights to Object Storage.
3.  Manually creating/applying a KServe `InferenceService` manifest.

This process is error-prone, lacks a single source of truth, and makes it difficult to audit changes or rollback versions. There is no guarantee that the registered model matches the deployed infrastructure.

## 3. Architecture

The solution is a **Kubernetes Operator** that reconciles the state of `AIModel` resources.

### 3.1 Components

1.  **AIModel CRD**: A high-level abstraction defining the model's metadata, source (HuggingFace), storage location (S3/MinIO), and serving configuration (vLLM/KServe).
2.  **AI Model Operator**: A controller that:
    *   Watches `AIModel` resources.
    *   Orchestrates the download of model artifacts to Object Storage via a Kubernetes Job.
    *   Manages the lifecycle of the underlying KServe `InferenceService`.
    *   Updates the model status (Downloading, Ready, Failed).
3.  **Model Downloader**: A specialized container image responsible for efficiently syncing artifacts from HuggingFace Hub to S3-compatible storage.

### 3.2 Workflow

1.  **Engineer** commits an `AIModel` YAML file to the git repository.
2.  **ArgoCD** syncs the manifest to the Kubernetes cluster.
3.  **Operator** detects the new resource.
4.  **Operator** checks if artifacts exist in the specified S3 bucket.
    *   *If missing*: Launches a **Downloader Job**.
    *   *If present*: Proceeds to serving.
5.  **Operator** creates/updates the KServe `InferenceService` pointing to the S3 artifacts.
6.  **KServe** spins up the vLLM pod(s).
7.  **Operator** updates the `AIModel` status to `Ready` with the inference endpoint.

### 3.3 Environment Strategy

We will follow a **Directory-Based Strategy** within a single GitOps repository (monorepo), consistent with the existing platform architecture.

*   **Structure**:
    ```text
    gitops/
      clusters/
        development/
          apps/
            ai-models/
              llama-2-7b.yaml
              mistral-7b.yaml
        staging/
          apps/
            ai-models/
              llama-2-7b.yaml
        production/
          apps/
            ai-models/
              llama-2-7b.yaml
    ```
*   **Promotion Flow**:
    1.  **Dev**: Engineer adds `llama-2-7b.yaml` to `gitops/clusters/development/apps/ai-models/`.
    2.  **Stage**: After testing, copy the file to `gitops/clusters/staging/...`.
    3.  **Prod**: Finally, promote to `gitops/clusters/production/...`.
*   **Kustomize**: We can optionally use Kustomize overlays if we need environment-specific overrides (e.g., `minReplicas: 1` in Dev vs `minReplicas: 3` in Prod), but for simplicity, full manifest copies are often clearer for CRDs.

## 4. User Stories

| ID | As a... | I want to... | So that... |
|----|---------|--------------|------------|
| **US-1** | Platform Engineer | Define AI models as Kubernetes manifests in Git | I can manage models using the same GitOps workflow as the rest of the infrastructure. |
| **US-2** | Data Scientist | Add a new model by opening a Pull Request | I don't need CLI access or complex permissions to deploy a model. |
| **US-3** | System | Automatically download model weights to Object Storage | I don't have to manually manage large file transfers or worry about disk space on my laptop. |
| **US-4** | Operator | See the status of a model download in Kubernetes | I know if a model is ready to serve or if the download failed. |
| **US-6** | Data Scientist | Deploy a Vision model using a different runtime (e.g., Triton) | I can support multi-modal use cases beyond just LLMs. |
| **US-7** | Data Scientist | Deploy the same model twice with different parameters (e.g., quantization, context length) | I can A/B test configurations side-by-side. |
| **US-8** | Platform Engineer | Target specific GPU hardware (e.g., A100 vs T4) for a model | I can optimize cost and performance for different model sizes. |
| **US-9** | Operator | Temporarily disable a model without deleting the config | I can stop incurring costs for a model that isn't currently needed but keep its config and cache ready. |

## 5. Requirements

### 5.1 Functional Requirements

*   **FR-1**: The system MUST support downloading models from public and private HuggingFace repositories.
*   **FR-2**: The system MUST support authentication to HuggingFace via Kubernetes Secrets.
*   **FR-3**: The system MUST cache model artifacts in S3-compatible Object Storage to prevent re-downloading on pod restarts.
*   **FR-4**: The system MUST automatically create and manage a KServe `InferenceService` for each `AIModel`.
*   **FR-5**: The `AIModel` status MUST reflect the detailed state of the download (e.g., "Downloading", "DownloadFailed") and serving (e.g., "Starting", "Ready").
*   **FR-6**: The system MUST support configuring engine-specific parameters (e.g., CLI args, env vars) via the CRD to enable A/B testing of configurations.
*   **FR-7**: The system MUST support different inference engines (e.g., vLLM, TGI, Triton) by allowing the user to specify the engine type and optional container image override.
*   **FR-8**: The system MUST support an `enabled` flag in the CRD. When false, the `InferenceService` should be deleted (stopping costs), but the `AIModel` resource and cached artifacts MUST remain.
*   **FR-9**: The system MUST support `nodeSelector` and `tolerations` in the CRD to allow targeting specific hardware (e.g., `accelerator: nvidia-a100`).
*   **FR-10**: The system SHOULD support scale-to-zero configuration. If `minReplicas` is 0, the model should consume no GPU resources when idle.

### 5.2 Non-Functional Requirements

*   **NFR-1**: **Reliability**: The downloader job must handle transient network failures with retries.
*   **NFR-2**: **Security**: HF Tokens must be stored in Kubernetes Secrets and never exposed in plain text logs.
*   **NFR-3**: **Observability**: The Operator must emit metrics for download duration, success/failure rates, and reconciliation errors.
*   **NFR-4**: **Efficiency**: The downloader should use concurrent downloads where possible to saturate available bandwidth.

## 6. Edge Cases & Failure Modes

*   **EC-1: Invalid HF Token**: The downloader job will fail with a 401/403. The Operator should capture this and update the `AIModel` status to `Failed` with a clear message.
*   **EC-2: Insufficient Storage**: If the Object Store or local ephemeral storage is full, the download will fail. The system should report "DiskPressure" or similar errors.
*   **EC-3: Model Too Large for GPU**: If the user requests a 70B model on a 24GB GPU, the vLLM pod will crash-loop. The Operator should reflect the KServe failure state.
*   **EC-4: Network Interruption**: If the download is interrupted, the job should retry. Ideally, the downloader should support resumable downloads (future improvement).
*   **EC-5: Configuration Drift**: If someone manually edits the KServe `InferenceService`, the Operator should revert the changes to match the `AIModel` spec (reconciliation).

## 7. Out of Scope

*   **Fine-tuning**: This spec covers inference serving only. Training/Fine-tuning pipelines are out of scope.
*   **Model Evaluation**: Automated evaluation of model quality is a separate workflow.
*   **Multi-Cluster Federation**: The operator manages models within a single cluster context.

## 8. Future Improvements

*   **Resumable Downloads**: Support resuming interrupted downloads to save bandwidth.
*   **P2P Distribution**: Use Dragonfly or similar P2P protocols for distributing weights to nodes.
*   **Model Quantization**: Auto-quantize models during the ingestion phase.
