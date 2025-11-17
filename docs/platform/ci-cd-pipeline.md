# CI/CD Pipeline

This document provides a comprehensive overview of the CI/CD pipeline, Git strategy, and automated testing for the AI-as-a-Service platform.

## Philosophy

Our approach to CI/CD is guided by the principles of GitOps. The `main` branch is the single source of truth, and all changes to the application and its infrastructure are managed through Git. We use a combination of GitHub Actions for Continuous Integration and ArgoCD for Continuous Deployment.

## Git Strategy

### Branch and Tag Strategy

*   **`main` branch**: The `main` branch represents the latest version of the application and automatically deploys to the **development environment**.
*   **Feature branches**: All new features and bug fixes are developed on feature branches. These branches are created from `main` and should have a descriptive name (e.g., `feature/new-billing-endpoint`).
*   **Pull Requests**: When a feature is complete, a pull request is opened to merge the feature branch into `main`. All pull requests must pass the CI pipeline before they can be merged.
*   **Release Tags**: Production deployments are triggered by creating Git tags following semantic versioning (e.g., `v1.0.0`, `v1.1.0`, `v1.2.3`).

### Environment Promotion Flow

```
Feature Branch → main (development) → tag vX.Y.Z (production)
```

1. **Development**: Merge feature branch to `main` → auto-deploys to development
2. **Production**: Create release tag from `main` → manually sync production apps to tag

## Continuous Integration (CI)

Our CI pipeline is powered by GitHub Actions and consists of multiple workflows:

### Go Services CI (`.github/workflows/ci.yml`)

The main CI pipeline for Go microservices is triggered on every push to `main` and on every pull request. It consists of the following stages:

1.  **Setup & Discover Services**: This stage checks out the code and dynamically discovers all the microservices in the `services/` directory.
2.  **Build**: This stage compiles the code for each microservice to ensure that it is buildable.
3.  **Test**: This stage runs the unit and integration tests for each microservice.
4.  **Lint**: This stage runs a static analysis tool (`golangci-lint`) to check for code quality and style issues.
5.  **Metrics Upload**: This stage generates and archives build metrics.

### Web Portal CI (`.github/workflows/web-portal.yml`)

The web portal CI pipeline runs independently and is triggered when changes are made to `web/portal/` or `shared/ts/`. It consists of the following stages:

1.  **Lint**: Runs ESLint to check for code quality and style issues.
2.  **Unit Tests**: Runs Vitest unit tests to verify component and hook functionality.
3.  **E2E Tests**: Runs Playwright end-to-end tests to verify user workflows (including critical paths like sign-in).
4.  **Build**: Builds and pushes Docker images **only if all tests pass**.

**Critical**: The build stage depends on all test stages passing. This ensures that broken code cannot be deployed, even if it compiles successfully.

### Required Secrets

The web portal CI pipeline requires the following GitHub repository secrets:

#### GHCR_TOKEN

**Purpose**: Authentication for pushing Docker images to GitHub Container Registry (GHCR).

**Setup**:
1. Create a GitHub Personal Access Token (Classic) at https://github.com/settings/tokens/new
2. Grant the following scopes:
   - `repo` (Full control of private repositories)
   - `write:packages` (Upload packages to GitHub Package Registry)
   - `read:packages` (Download packages from GitHub Package Registry)
   - `delete:packages` (Optional, for cleanup)
3. Add the token to repository secrets:
   ```bash
   gh secret set GHCR_TOKEN
   # Paste the token when prompted
   ```

**Validation**:
```bash
# Verify the secret is set
gh secret list | grep GHCR_TOKEN

# Test GHCR authentication locally
echo $GHCR_TOKEN | docker login ghcr.io -u <username> --password-stdin
```

**Troubleshooting**: If the workflow fails with "denied: denied" during Docker login:
- Token may be expired or revoked - create a new one
- Token may be missing required scopes - ensure `write:packages` is enabled
- See `docs/troubleshooting/ci.md` for detailed resolution steps

## Automated Testing Strategy

We employ a multi-layered testing strategy to ensure the quality and reliability of our platform.

*   **Unit Tests**: These tests verify the functionality of individual components in isolation. They are located alongside the source code and are run as part of the `make test` command.
*   **Integration Tests**: These tests verify the interaction between different components of the system. They are also run as part of the `make test` command and use Docker to spin up dependencies like databases and message queues.
*   **End-to-End (E2E) Tests**: These tests verify the functionality of the entire system from the user's perspective.
*   **Performance Tests**: These tests measure the performance and scalability of the system. They are located in the `tests/perf` directory.
*   **Infrastructure Tests**: These tests verify the correctness of our infrastructure-as-code. They are located in the `tests/infra` directory.

## Continuous Deployment (CD) with ArgoCD

We use ArgoCD to automate the deployment of our application to our Kubernetes clusters. ArgoCD is a declarative, GitOps continuous delivery tool for Kubernetes.

*   **GitOps Repository**: This repository serves as the single source of truth for our application's desired state. The `gitops/clusters/` directory contains the Kubernetes manifests for each of our environments (`development` and `production`).
*   **ArgoCD Application**: We have ArgoCD applications configured to monitor this repository for changes.

### Deployment Process by Environment

#### Development Environment

**Trigger**: Merge to `main` branch

1.  The CI pipeline runs and **validates all tests pass** (unit, integration, E2E, linting).
2.  **Only if tests pass**, new container images are built for the services that have changed.
3.  ArgoCD detects that the `main` branch has changed.
4.  ArgoCD **automatically syncs** the changes to the development Kubernetes cluster.

**Configuration**: Development apps watch `main` branch with `automated` sync enabled.

#### Production Environment

**Trigger**: Git tag creation (e.g., `v1.0.0`)

1.  Create a release tag using the release script: `./scripts/dev/release.sh v1.0.0`
2.  CI pipeline builds and pushes production Docker images tagged with the version.
3.  ArgoCD detects the new tag is available.
4.  **Manual sync required**: Run `argocd app sync <app-name> --revision v1.0.0`
5.  ArgoCD deploys the tagged version to the production Kubernetes cluster.

**Configuration**: Production apps watch `v*` tags with manual sync (`automated: null`).

### Creating a Production Release

Use the provided release script to create and push a release tag:

```bash
# Create a release
./scripts/dev/release.sh v1.0.0 "Initial production release"

# The script will:
# 1. Validate version format (vMAJOR.MINOR.PATCH)
# 2. Check you're on main branch
# 3. Show what's included in the release
# 4. Create and push an annotated Git tag
# 5. Provide next steps for deployment

# After the tag is pushed, manually sync production:
argocd app sync user-org-service-production --revision v1.0.0
```

**Semantic Versioning Guidelines:**
- **MAJOR** (v2.0.0): Breaking changes, incompatible API changes
- **MINOR** (v1.1.0): New features, backwards-compatible
- **PATCH** (v1.0.1): Bug fixes, backwards-compatible

**Important**: ArgoCD validates Kubernetes resources and deployment health, but **does not run application tests**. All testing must pass in CI before code reaches ArgoCD. This ensures that broken functionality (like a broken sign-in button) cannot be deployed, even if the code compiles successfully.

## Branch Protection

To ensure broken code cannot be merged, configure branch protection rules for the `main` branch:

1. Go to repository Settings → Branches
2. Add branch protection rule for `main`
3. Enable "Require status checks to pass before merging"
4. Select required checks:
   - `lint` (web-portal)
   - `test` (web-portal)
   - `test-e2e` (web-portal)
   - Other service CI checks as needed

This ensures that PRs cannot be merged unless all CI checks pass, preventing broken code from reaching production.

## GitOps & ArgoCD

- GitOps repository structure resides under `gitops/`.
- Bootstrap ArgoCD per cluster with `./scripts/gitops/bootstrap_argocd.sh <environment> <kube-context>`.
- Register this repository using `argocd repo add` and sync `platform-<env>-infrastructure` applications after each promotion.
- Customize `gitops/templates/argocd-values.yaml` (ingress, service type, RBAC) and rerun the bootstrap script to apply changes.
