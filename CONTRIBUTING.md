# Contributing

We welcome contributions from everyone. Here are some guidelines to follow.

## Development Workflow

1.  **Fork the repository** and create your branch from `main`.
2.  **Set up the development environment**: Run `make bootstrap` to install all the necessary tools.
3.  **Start the local development stack**: Run `make up` to start all the required services.
4.  **Make your changes**: Implement your feature or bug fix.
5.  **Ensure all checks pass**: Run `make check` to run formatting, linting, security scans, and tests.
6.  **Commit your changes**: Write a clear and concise commit message.
7.  **Push to your fork** and submit a pull request.

## Coding Style

*   **Go**: We follow the standard Go formatting (`gofmt`). The `make fmt` command will format your code automatically.
*   **TypeScript**: We use Prettier and ESLint for formatting and linting. The `make shared-ts-check` command will check your code.

## Testing

*   **Unit Tests**: All new features and bug fixes should be accompanied by unit tests. Run `make test` to run the tests for a specific service, or `make test SERVICE=all` to run all tests.
*   **End-to-End Tests**: For larger features, consider adding end-to-end tests. These are located in the `e2e-test` directory.

## Pull Requests

*   Your pull request should have a clear title and description.
*   The description should explain the "why" behind your changes.
*   If your pull request addresses an issue, link to it in the description.
*   Ensure that the CI checks pass on your pull request. For more information on our CI/CD pipeline, please see the [CI/CD Pipeline document](./docs/platform/ci-cd-pipeline.md).

## Key Makefile Commands

*   `make check`: Run all checks (format, lint, security, test).
*   `make build`: Build the services.
*   `make test`: Run unit tests.
*   `make up`: Start the local development environment.
*   `make stop`: Stop the local development environment.
*   `make logs`: View logs from the local development stack.

## Tooling Version Management

This repository centralizes all automation tool versions in `configs/tool-versions.mk`.  
Teams MUST update that manifest whenever bumping the supported toolchain.

## Version Bump Process

1. Edit `configs/tool-versions.mk`, updating the desired version constants with inline comments if needed.
2. Run affected Make targets locally to ensure compatibility (e.g., `make check`, `make ci-local`).
3. Update quickstart or troubleshooting docs if workflows change.
4. Capture the change in release notes (or project changelog) along with rationale.
5. Submit PR including:
   - Manifest change
   - Any lockfile or dependency updates (e.g., `go.sum`)
   - Verification logs from Step 2
6. After merge, notify contributors in the communication channel so they can upgrade locally.

Automation tasks (e.g., `make check`) must read versions from the manifest—hardcoding versions in scripts is prohibited.

