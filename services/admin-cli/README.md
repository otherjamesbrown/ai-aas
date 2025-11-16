# Admin CLI

The `admin-cli` is a command-line interface for administrators to manage the platform.

## Purpose

This CLI provides a convenient way to perform administrative tasks, such as creating users, managing organizations, and configuring services.

## Running the CLI

To run the CLI, you can use the following command:

```bash
go run ./services/admin-cli/cmd/cli --help
```

## Running Tests

To run the tests for this service, use the following command:

```bash
make test SERVICE=admin-cli
```