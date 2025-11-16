# CI/CD Pipeline

This document provides a comprehensive overview of the CI/CD pipeline, Git strategy, and automated testing for the AI-as-a-Service platform.

## Philosophy

Our approach to CI/CD is guided by the principles of GitOps. The `main` branch is the single source of truth, and all changes to the application and its infrastructure are managed through Git. We use a combination of GitHub Actions for Continuous Integration and ArgoCD for Continuous Deployment.

## Git Strategy

*   **`main` branch**: The `main` branch represents the latest stable version of the application. All development work is ultimately merged into this branch.
*   **Feature branches**: All new features and bug fixes are developed on feature branches. These branches are created from `main` and should have a descriptive name (e.g., `feature/new-billing-endpoint`).
*   **Pull Requests**: When a feature is complete, a pull request is opened to merge the feature branch into `main`. All pull requests must pass the CI pipeline before they can be merged.

## Continuous Integration (CI)

Our CI pipeline is powered by GitHub Actions and is defined in the `.github/workflows/ci.yml` file. The pipeline is triggered on every push to `main` and on every pull request.

The CI pipeline consists of the following stages:

1.  **Setup & Discover Services**: This stage checks out the code and dynamically discovers all the microservices in the `services/` directory.
2.  **Build**: This stage compiles the code for each microservice to ensure that it is buildable.
3.  **Test**: This stage runs the unit and integration tests for each microservice.
4.  **Lint**: This stage runs a static analysis tool (`golangci-lint`) to check for code quality and style issues.
5.  **Metrics Upload**: This stage generates and archives build metrics.

## Automated Testing Strategy

We employ a multi-layered testing strategy to ensure the quality and reliability of our platform.

*   **Unit Tests**: These tests verify the functionality of individual components in isolation. They are located alongside the source code and are run as part of the `make test` command.
*   **Integration Tests**: These tests verify the interaction between different components of the system. They are also run as part of the `make test` command and use Docker to spin up dependencies like databases and message queues.
*   **End-to-End (E2E) Tests**: These tests verify the functionality of the entire system from the user's perspective.
*   **Performance Tests**: These tests measure the performance and scalability of the system. They are located in the `tests/perf` directory.
*   **Infrastructure Tests**: These tests verify the correctness of our infrastructure-as-code. They are located in the `tests/infra` directory.

## Continuous Deployment (CD) with ArgoCD

We use ArgoCD to automate the deployment of our application to our Kubernetes clusters. ArgoCD is a declarative, GitOps continuous delivery tool for Kubernetes.

*   **GitOps Repository**: This repository serves as the single source of truth for our application's desired state. The `gitops/clusters/` directory contains the Kubernetes manifests for each of our environments (`development`, `staging`, and `production`).
*   **ArgoCD Application**: We have an ArgoCD application configured to monitor the `gitops/clusters/` directory in this repository.
*   **Deployment Process**: When a pull request is merged into `main`, the following happens:
    1.  The CI pipeline runs and builds new container images for the services that have changed.
    2.  The new container image tags are updated in the Kubernetes manifests in the `gitops/clusters/` directory.
    3.  ArgoCD detects that the state of the repository has changed.
    4.  ArgoCD automatically syncs the changes to the appropriate Kubernetes cluster, deploying the new version of the application.
