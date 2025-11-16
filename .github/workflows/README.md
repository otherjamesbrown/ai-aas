# GitHub Actions Workflows

This directory contains all the GitHub Actions workflows for the AI-as-a-Service platform.

## Main Workflows

*   **`ci.yml`**: The main Continuous Integration (CI) pipeline. It is triggered on pushes to `main` and on all pull requests. It discovers all services and then builds, tests, and lints them.
*   **`ci-remote.yml`**: A workflow that allows for the remote triggering of the CI pipeline. This is useful for running CI on a specific branch from a restricted environment.
*   **`reusable-build.yml`**: A reusable workflow that provides a standardized way to build and test the Go services.

## Service-Specific Workflows

*   **`api-router-service.yml`**: A dedicated CI/CD pipeline for the `api-router-service`.
*   **`user-org-service.yml`**: A dedicated CI/CD pipeline for the `user-org-service`.
*   **`web-portal.yml`**: A dedicated CI/CD pipeline for the `web-portal` frontend application. It runs linting, unit tests, and end-to-end tests.
*   **`shared-libraries-ci.yml`**: A CI pipeline for the shared libraries in the `shared/` directory.
*   **`shared-libraries-release.yml`**: A workflow for creating releases of the shared libraries.

## Infrastructure and Environment Workflows

*   **`infra-terraform.yml`**: A workflow for managing the infrastructure with Terraform.
*   **`infra-availability.yml`**: A workflow that runs periodically to check the availability of the infrastructure.
*   **`dev-environment-ci.yml`**: A CI pipeline for the development environment.
*   **`db-guardrails.yml`**: A workflow that enforces guardrails on the database.
