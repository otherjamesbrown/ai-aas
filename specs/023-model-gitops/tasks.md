# Implementation Tasks: GitOps-Managed AI Models

This document breaks down the implementation of the GitOps-Managed AI Models feature into actionable, dependency-ordered tasks.

## Implementation Strategy

The implementation is phased to deliver value incrementally. The MVP (Minimum Viable Product) is User Story 1, which establishes the core reconciliation loop for the operator. Subsequent user stories add functionality like automated artifact handling and status reporting. Each user story is designed to be an independently testable and deployable unit of work.

## Phase 1: Project Setup & CRD Definition
*Initial setup for the AI Model Operator project.*

- [ ] T001 Initialize the Go operator project structure in `services/ai-model-operator/` using `operator-sdk`.
- [ ] T002 Define the `AIModel` API types (`spec` and `status`) in `services/ai-model-operator/api/v1alpha1/aimodel_types.go` based on `data-model.md`.
- [ ] T003 Generate the Custom Resource Definition (CRD) manifests using `make manifests`.
- [ ] T004 Create the Helm chart structure in `infra/helm/charts/ai-model-operator/`.
- [ ] T005 [P] Add initial Helm templates for `Deployment`, `ServiceAccount`, and `Role`/`RoleBinding` in `infra/helm/charts/ai-model-operator/templates/`.

## Phase 2: User Story 1 - Declarative Model Management
*Implement the core reconciliation logic to manage vLLM `Deployment` resources.*

**Goal**: A Platform Engineer can manage a model's lifecycle by committing an `AIModel` manifest to Git.

**Independent Test**:
1. Create an `AIModel` manifest.
2. `kubectl apply` the manifest.
3. Verify that a corresponding Kubernetes `Deployment` for vLLM is created.
4. Update a field in the `AIModel` manifest (e.g., `minReplicas`).
5. `kubectl apply` the updated manifest.
6. Verify the `Deployment` is updated accordingly.
7. `kubectl delete` the `AIModel` manifest.
8. Verify the `Deployment` is deleted.

### Tasks
- [ ] T006 [US1] Implement the initial `Reconcile` loop in `services/ai-model-operator/internal/controller/aimodel_controller.go` to fetch the `AIModel` resource.
- [ ] T007 [US1] Add a finalizer to the `AIModel` resource to handle cleanup logic upon deletion.
- [ ] T008 [US1] Implement logic to create a vLLM `Deployment` if it doesn't exist, based on the `AIModel` spec.
- [ ] T009 [US1] Implement logic to update the `Deployment` if the `AIModel` spec changes.
- [ ] T010 [US1] Implement the cleanup logic in the finalizer to delete the `Deployment` when the `AIModel` is deleted.
- [ ] T011 [US1] Implement logic to handle the `enabled: false` flag by deleting the `Deployment`.
- [ ] T012 [P] [US1] Write controller unit tests for the vLLM `Deployment` lifecycle in `services/ai-model-operator/internal/controller/aimodel_controller_test.go`.

## Phase 3: User Story 3 - Automated Model Artifact Handling
*Implement the model downloader job.*

**Goal**: The system automatically downloads model artifacts to object storage when a new model is defined.

**Independent Test**:
1. Create an `AIModel` manifest pointing to a public HuggingFace model.
2. `kubectl apply` the manifest.
3. Verify that a Kubernetes `Job` is created to download the model.
4. Verify the `Job` completes successfully.
5. Check the object storage bucket to confirm the model artifacts are present.

### Tasks
- [ ] T013 [US3] Design the logic within the `Reconcile` loop to check if model artifacts exist in S3.
- [ ] T014 [US3] Implement logic to create a Kubernetes `Job` to download the model if artifacts are missing.
- [ ] T015 [US3] Create a simple downloader script and package it into a container image in `services/model-downloader/`.
- [ ] T016 [US3] Pass necessary information (source URL, S3 endpoint, credentials) to the downloader `Job`.
- [ ] T017 [US3] Implement logic to check the status of the downloader `Job` (succeeded/failed) in the `Reconcile` loop.
- [ ] T018 [P] [US3] Write controller unit tests for the downloader `Job` lifecycle.

## Phase 4: User Story 4 - Model Deployment Visibility
*Implement status reporting for the `AIModel` resource.*

**Goal**: An Operator can see the real-time status of a model deployment by inspecting the `AIModel` resource.

**Independent Test**:
1. Create a new `AIModel` manifest.
2. `kubectl apply` it and immediately run `kubectl get aimodel <name> -w`.
3. Verify the `status.phase` changes from empty to `Downloading`.
4. After the downloader `Job` succeeds, verify the `status.phase` changes to `Ready`.
5. Verify `status.inferenceEndpoint` is populated once the `Deployment` is ready and the `Service` is available.

### Tasks
- [ ] T019 [US4] Implement logic to update the `AIModel.status.phase` to `Downloading` when the downloader `Job` is created.
- [ ] T020 [US4] Implement logic to update the `AIModel.status.phase` to `Failed` if the downloader `Job` fails, and populate `lastFailureMessage`.
- [ ] T021 [US4] Implement logic to watch the vLLM `Deployment` status and update the `AIModel.status.phase` to `Ready` when it becomes available.
- [ ] T022 [US4] Populate the `AIModel.status.inferenceEndpoint` field from the created Kubernetes `Service` for the vLLM deployment.
- [ ] T023 [P] [US4] Write controller unit tests for status updates.

## Phase 5: Polish & Cross-Cutting Concerns
*Finalize implementation with logging, metrics, and documentation.*

- [ ] T024 [P] Integrate structured logging (`zap`) throughout the controller logic, conforming to the constitution.
- [ ] T025 [P] Add Prometheus metrics for reconciliation latency, error counts, and total managed models.
- [ ] T026 [P] Update the `README.md` for the `ai-model-operator` service with detailed usage instructions.
- [ ] T027 [P] Test the Helm chart deployment and upgrade process.

## Dependencies

- **User Story 1** is the foundational MVP and has no dependencies on other stories.
- **User Story 3** depends on User Story 1 (the core reconciliation loop must exist).
- **User Story 4** depends on User Story 1 and 3 (it reports the status of artifacts managed by them).
- **User Story 2** is a process-only story and requires no new code, but depends on all other stories being complete to be fully effective.

## Parallel Execution

- Within each phase, tasks marked with `[P]` can be worked on in parallel.
- **Phase 1** (Setup) can be done in parallel with initial work on **Phase 2** (US1).
- The downloader container image (**T015**) can be developed in parallel with the controller logic in other phases.
