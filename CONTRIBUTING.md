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
*   Ensure that the CI checks pass on your pull request.

## Key Makefile Commands

*   `make check`: Run all checks (format, lint, security, test).
*   `make build`: Build the services.
*   `make test`: Run unit tests.
*   `make up`: Start the local development environment.
*   `make stop`: Stop the local development environment.
*   `make logs`: View logs from the local development stack.