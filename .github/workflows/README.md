# CI/CD Workflows

This directory contains the GitHub Actions workflows for the AI-AAS platform.

## Main Workflows

*   **`ci.yml`**: This is the main Continuous Integration (CI) workflow. It is triggered on every push to the `main` branch and on every pull request. It discovers all the services in the `services/` directory and then builds and tests each service in parallel. It also runs a linting job and a security scan.
*   **`ci-remote.yml`**: This workflow is used to run the CI pipeline on a remote environment. It is triggered manually and requires a git revision and a service name as input.
*   **`e2e-tests.yml`**: This workflow runs the end-to-end tests. It is triggered manually.
*   **`reusable-build.yml`**: This is a reusable workflow that builds or tests a single service. It is used by the `ci.yml` workflow.

## Service-Specific Workflows

The following workflows are service-specific and are triggered on changes within the respective service directories:

*   `analytics-service.yml`
*   `api-router-service.yml`
*   `user-org-service.yml`
*   `web-portal.yml`

These workflows are largely redundant with the main `ci.yml` workflow and will be removed in the future.

## Testing Workflows

*   `e2e.yml`: Runs end-to-end tests against live environments.
*   `nightly-e2e.yml`: Scheduled nightly end-to-end tests across development and staging environments.
*   `failure-mode-tests.yml`: Validates model deployment failure modes and CLI observability output. Runs weekly on schedule and on-demand. Includes both Go integration tests and shell-based test harness.

## Other Workflows

*   `api-router-validation.yml`: Validates the API router configuration.
*   `db-guardrails.yml`: Checks for dangerous database migrations.
*   `dev-environment-ci.yml`: Runs CI for the development environment.
*   `infra-availability.yml`: Checks the availability of the infrastructure.
*   `infra-terraform.yml`: Runs terraform to deploy infrastructure changes.
*   `shared-libraries-ci.yml`: Runs CI for the shared libraries.
*   `shared-libraries-release.yml`: Creates a release for the shared libraries.